package agy

import (
	"slices"
	"testing"

	ap "github.com/rainchan/webmux/internal/agentprofile"
	"github.com/rainchan/webmux/internal/agentprofile/agenttest"
)

func TestContract(t *testing.T) {
	const sid = "89c9a933-60f6-46ad-977b-a8daf22f8e4d"
	agenttest.Run(t, Agent, agenttest.Fixture{
		Signals:  ap.Signals{ProgramApp: "antigravity", ProgramChildID: sid, RootCwd: "/p"},
		Identity: ap.Identity{Agent: Name, AgentSessionID: sid, Cwd: "/p"},
	})
	if got := ap.ReapplyOnLoad(Agent); !slices.Equal(got, []string{"model", "thought_level"}) {
		t.Fatalf("reapply options = %q", got)
	}
	if !ap.ResumesSessions(Agent) {
		t.Fatal("agy must support session/resume")
	}
}
