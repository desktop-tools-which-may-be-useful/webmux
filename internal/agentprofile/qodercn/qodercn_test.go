package qodercn

import (
	"encoding/json"
	"slices"
	"testing"

	ap "github.com/rainchan/webmux/internal/agentprofile"
	"github.com/rainchan/webmux/internal/agentprofile/agenttest"
)

func TestContract(t *testing.T) {
	agenttest.Run(t, Agent, agenttest.Fixture{
		Signals:      ap.Signals{ProgramApp: "qoder-cli-cn", RootCwd: "/p"},
		Identity:     ap.Identity{Agent: Name, Cwd: "/p"},
		SteeringInit: json.RawMessage(`{"steering":{"supported":true}}`),
	})
	if got := ap.ReapplyOnLoad(Agent); !slices.Equal(got, []string{"model", "effort"}) {
		t.Fatalf("reapply options = %q", got)
	}
}

func TestProcessExecutable(t *testing.T) {
	for _, base := range []string{"qodercn", "qoderclicn", "qoder-cn", "qoderclicn-1.1.81"} {
		if !profile.RunsProcess(base) {
			t.Fatalf("native process %q not recognized", base)
		}
	}
	for _, base := range []string{"node", "qoder", "qodercli"} {
		if profile.RunsProcess(base) {
			t.Fatalf("unrelated process %q recognized", base)
		}
	}
}

func TestACPArgv(t *testing.T) {
	registry := ap.NewRegistry(Agent)
	cmd, err := registry.Parse([]string{"qodercn", "--dangerously-skip-permissions"})
	if err != nil {
		t.Fatal(err)
	}
	acp, _ := cmd.ACPArgv()
	if !slices.Equal(acp, []string{"qodercn", "--acp"}) {
		t.Fatalf("ACPArgv = %q, want [qodercn --acp]", acp)
	}
}

func TestChatConfig(t *testing.T) {
	tests := []struct {
		name        string
		argv        []string
		seeds       []ap.Seed
		unsupported []string
	}{
		{
			name: "model and effort",
			argv: []string{"qodercn", "--model", "qwen-max", "--reasoning-effort", "high"},
			seeds: []ap.Seed{
				{ConfigID: "model", Value: "qwen-max", Arg: "--model qwen-max"},
				{ConfigID: "effort", Value: "high", Arg: "--reasoning-effort high"},
			},
		},
		{
			name:  "yolo maps to bypass_permissions",
			argv:  []string{"qodercn", "--yolo"},
			seeds: []ap.Seed{{ConfigID: "mode", Value: "bypass_permissions", Arg: "--yolo"}},
		},
		{
			name:  "dangerously-skip-permissions",
			argv:  []string{"qodercn", "--dangerously-skip-permissions"},
			seeds: []ap.Seed{{ConfigID: "mode", Value: "bypass_permissions", Arg: "--dangerously-skip-permissions"}},
		},
		{
			name: "permission mode aliases",
			argv: []string{"qodercn", "--permission-mode", "bypassPermissions"},
			seeds: []ap.Seed{
				{ConfigID: "mode", Value: "bypass_permissions", Arg: "--permission-mode bypassPermissions"},
			},
		},
		{
			name: "plan mode",
			argv: []string{"qodercn", "--permission-mode", "plan"},
			seeds: []ap.Seed{
				{ConfigID: "mode", Value: "plan", Arg: "--permission-mode plan"},
			},
		},
		{
			name:        "unknown permission mode",
			argv:        []string{"qodercn", "--permission-mode", "invalid"},
			unsupported: []string{"--permission-mode invalid"},
		},
	}
	registry := ap.NewRegistry(Agent)
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cmd, err := registry.Parse(tt.argv)
			if err != nil {
				t.Fatal(err)
			}
			seeds, unsupported := cmd.ChatConfig()
			if !slices.Equal(seeds, tt.seeds) || !slices.Equal(unsupported, tt.unsupported) {
				t.Fatalf("ChatConfig = %+v %q, want %+v %q", seeds, unsupported, tt.seeds, tt.unsupported)
			}
		})
	}
}

func TestSteeringUsesPromptRequired(t *testing.T) {
	steering := Agent.(ap.Steerer)
	if got := string(steering.SteeringMeta()); got != `{"steering":{"idleBehavior":"promptRequired"}}` {
		t.Fatalf("steering metadata = %s", got)
	}
}
