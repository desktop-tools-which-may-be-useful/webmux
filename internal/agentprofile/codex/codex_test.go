package codex

import (
	"encoding/json"
	"slices"
	"testing"

	ap "github.com/rainchan/webmux/internal/agentprofile"
	"github.com/rainchan/webmux/internal/agentprofile/agenttest"
)

func TestContract(t *testing.T) {
	agenttest.Run(t, Agent, agenttest.Fixture{
		Signals:      ap.Signals{ProgramApp: Name, RootCwd: "/p"},
		Identity:     ap.Identity{Agent: Name, Cwd: "/p"},
		SteeringInit: json.RawMessage(`{"steering":{"supported":true}}`),
	})
	if !ap.ResumesSessions(Agent) {
		t.Fatal("Codex must support resuming sessions")
	}
	if got := ap.ReapplyOnLoad(Agent); !slices.Equal(got, []string{"model", "reasoning_effort", "mode"}) {
		t.Fatalf("reapply options = %q", got)
	}
}

func TestLockedSessions(t *testing.T) {
	paths := []string{"/home/u/.codex/thread-writer-locks/01968f88-1111-7000-8000-000000000001.lock"}
	got := (adapter{}).LockedSessions(paths, func(string) string { return "" }, "/home/u")
	if !slices.Equal(got, []string{"01968f88-1111-7000-8000-000000000001"}) {
		t.Fatalf("LockedSessions = %q", got)
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
			name: "model double-quoted effort and sandbox mode",
			argv: []string{"codex", "--model", "o3", "-c", `model_reasoning_effort="high"`, "--sandbox", "workspace-write"},
			seeds: []ap.Seed{
				{ConfigID: "model", Value: "o3", Arg: "--model o3"},
				{ConfigID: "reasoning_effort", Value: "high", Arg: `-c model_reasoning_effort="high"`},
				{ConfigID: "mode", Value: "workspace-write", Arg: "-s workspace-write"},
			},
		},
		{
			name: "single-quoted effort and mode priority",
			argv: []string{
				"codex",
				"-c", "model_reasoning_effort = 'medium'",
				"-s", "read-only",
				"--approve-for-me",
				"--dangerously-bypass-approvals-and-sandbox",
			},
			seeds: []ap.Seed{
				{ConfigID: "reasoning_effort", Value: "medium", Arg: "-c model_reasoning_effort = 'medium'"},
				{ConfigID: "mode", Value: "agent-full-access", Arg: "--dangerously-bypass-approvals-and-sandbox"},
			},
		},
		{
			name: "unquoted effort and approve-for-me priority over read-only",
			argv: []string{"codex", "-c", "model_reasoning_effort=low", "-s", "read-only", "--approve-for-me"},
			seeds: []ap.Seed{
				{ConfigID: "reasoning_effort", Value: "low", Arg: "-c model_reasoning_effort=low"},
				{ConfigID: "mode", Value: "agent", Arg: "--approve-for-me"},
			},
		},
		{
			name: "unsupported config approval sandbox and chat-only flags",
			argv: []string{
				"codex",
				"-c", "other=1",
				"--ask-for-approval", "untrusted",
				"-s", "custom-sandbox",
				"--profile", "dev",
				"--search",
			},
			unsupported: []string{
				"-c other=1",
				"--ask-for-approval untrusted",
				"-s custom-sandbox",
				"--profile dev",
				"--search",
			},
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
