package claudecode

import (
	"encoding/json"
	"slices"
	"strings"
	"testing"

	ap "github.com/rainchan/webmux/internal/agentprofile"
	"github.com/rainchan/webmux/internal/agentprofile/agenttest"
)

func TestContract(t *testing.T) {
	agenttest.Run(t, Agent, agenttest.Fixture{
		Signals:      ap.Signals{ProgramApp: "claude-code", RootCwd: "/p"},
		Identity:     ap.Identity{Agent: Name, Cwd: "/p"},
		SteeringInit: json.RawMessage(`{"steering":{"supported":true}}`),
	})
	if Agent.Profile().StartupQuiet != 0 {
		t.Fatal("Claude startup must not hide its initial busy state")
	}
	if got := ap.ReapplyOnLoad(Agent); !slices.Equal(got, []string{"model", "effort"}) {
		t.Fatalf("reapply options = %q", got)
	}
}

func TestProcessExecutable(t *testing.T) {
	for _, base := range []string{"claude", "2.1.316", "3.0.0"} {
		if !profile.RunsProcess(base) {
			t.Fatalf("native process %q not recognized", base)
		}
	}
	for _, base := range []string{"node", "2.1", "2.1.x", "claude-agent-acp"} {
		if profile.RunsProcess(base) {
			t.Fatalf("unrelated process %q recognized", base)
		}
	}
	if profile.RunsAs("2.1.316") {
		t.Fatal("versioned native filename became a launch command")
	}
}

func TestResumeArgv(t *testing.T) {
	registry := ap.NewRegistry(Agent)
	cmd, err := registry.Parse([]string{
		"claude", "--model=opus", "--effort", "high", "--add-dir", "one", "two",
		"--allowed-tools", "Bash", "Read", "--bare", "--image", "a.png", "修 bug",
	})
	if err != nil {
		t.Fatal(err)
	}
	got, dropped := cmd.ResumeArgv("01a18715-8504-7020-9f3d-257900000000")
	want := []string{
		"claude", "--model=opus", "--effort", "high", "--add-dir", "one", "two",
		"--allowed-tools", "Bash", "Read", "--bare", "--resume",
		"01a18715-8504-7020-9f3d-257900000000",
	}
	if !slices.Equal(got, want) {
		t.Fatalf("ResumeArgv = %q, want %q", got, want)
	}
	if !slices.Equal(dropped, []string{"--image a.png", "修 bug"}) {
		t.Fatalf("dropped = %q", dropped)
	}
	acp, unsupported := cmd.ACPArgv()
	if !slices.Equal(acp, []string{"claude-agent-acp"}) {
		t.Fatalf("ACPArgv = %q", acp)
	}
	if !slices.Contains(unsupported, "--add-dir one two") || !slices.Contains(unsupported, "--allowed-tools Bash Read") {
		t.Fatalf("variadic options not reported whole: %q", unsupported)
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
			name: "model effort and manual",
			argv: []string{"claude", "--model", "sonnet", "--effort=high", "--permission-mode", "manual"},
			seeds: []ap.Seed{
				{ConfigID: "model", Value: "sonnet", Arg: "--model sonnet"},
				{ConfigID: "effort", Value: "high", Arg: "--effort=high"},
				{ConfigID: "mode", Value: "default", Arg: "--permission-mode manual"},
			},
		},
		{
			name:  "dangerous permissions win",
			argv:  []string{"claude", "--permission-mode", "plan", "--dangerously-skip-permissions"},
			seeds: []ap.Seed{{ConfigID: "mode", Value: "bypassPermissions", Arg: "--dangerously-skip-permissions"}},
		},
		{
			name:        "unknown permission and chat-only option",
			argv:        []string{"claude", "--permission-mode", "dontAsk", "--fallback-model", "haiku"},
			unsupported: []string{"--fallback-model haiku", "--permission-mode dontAsk"},
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

func TestForceSwitchNote(t *testing.T) {
	note := ap.ForceSwitchNote(Agent)
	for _, part := range []string{"cancel", "后台", "等待受管后代退出"} {
		if !strings.Contains(note, part) {
			t.Fatalf("force switch note %q lacks %q", note, part)
		}
	}
}
