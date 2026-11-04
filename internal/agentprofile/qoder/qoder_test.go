package qoder

import (
	"encoding/json"
	"slices"
	"testing"

	ap "github.com/rainchan/webmux/internal/agentprofile"
	"github.com/rainchan/webmux/internal/agentprofile/agenttest"
)

func TestContract(t *testing.T) {
	agenttest.Run(t, Agent, agenttest.Fixture{
		Signals:      ap.Signals{ProgramApp: "qoder", RootCwd: "/p"},
		Identity:     ap.Identity{Agent: Name, Cwd: "/p"},
		SteeringInit: json.RawMessage(`{"steering":{"supported":true}}`),
	})
	if got := ap.ReapplyOnLoad(Agent); !slices.Equal(got, []string{"model", "effort"}) {
		t.Fatalf("reapply options = %q", got)
	}
}

func TestProcessExecutable(t *testing.T) {
	for _, base := range []string{"qoder", "qodercli", "qodercli-1.1.81", "qoder-2.0.0"} {
		if !Agent.Profile().RunsProcess(base) {
			t.Fatalf("native process %q not recognized", base)
		}
	}
	for _, base := range []string{"node", "qodercn", "qoderclicn"} {
		if Agent.Profile().RunsProcess(base) {
			t.Fatalf("unrelated process %q recognized", base)
		}
	}
}

func TestACPArgv(t *testing.T) {
	registry := ap.NewRegistry(Agent)
	cmd, err := registry.Parse([]string{"qoder", "--dangerously-skip-permissions"})
	if err != nil {
		t.Fatal(err)
	}
	acp, _ := cmd.ACPArgv()
	if !slices.Equal(acp, []string{"qoder", "--acp"}) {
		t.Fatalf("ACPArgv = %q, want [qoder --acp]", acp)
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
			argv: []string{"qoder", "--model", "gpt-4", "--reasoning-effort", "high"},
			seeds: []ap.Seed{
				{ConfigID: "model", Value: "gpt-4", Arg: "--model gpt-4"},
				{ConfigID: "effort", Value: "high", Arg: "--reasoning-effort high"},
			},
		},
		{
			name:  "yolo maps to bypass_permissions",
			argv:  []string{"qoder", "--yolo"},
			seeds: []ap.Seed{{ConfigID: "mode", Value: "bypass_permissions", Arg: "--yolo"}},
		},
		{
			name:  "dangerously-skip-permissions",
			argv:  []string{"qoder", "--dangerously-skip-permissions"},
			seeds: []ap.Seed{{ConfigID: "mode", Value: "bypass_permissions", Arg: "--dangerously-skip-permissions"}},
		},
		{
			name: "permission mode aliases",
			argv: []string{"qoder", "--permission-mode", "acceptEdits"},
			seeds: []ap.Seed{
				{ConfigID: "mode", Value: "accept_edits", Arg: "--permission-mode acceptEdits"},
			},
		},
		{
			name:        "unknown permission mode",
			argv:        []string{"qoder", "--permission-mode", "invalid"},
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
