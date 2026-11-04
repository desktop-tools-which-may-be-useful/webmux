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
		if !Agent.Profile().RunsProcess(base) {
			t.Fatalf("native process %q not recognized", base)
		}
	}
	for _, base := range []string{"node", "qoder", "qodercli"} {
		if Agent.Profile().RunsProcess(base) {
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
