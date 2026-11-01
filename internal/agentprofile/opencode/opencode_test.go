package opencode

import (
	"testing"

	"github.com/rainchan/webmux/internal/agentprofile"
	"github.com/rainchan/webmux/internal/agentprofile/agenttest"
)

func TestContract(t *testing.T) {
	agenttest.Run(t, Agent, agenttest.Fixture{
		Signals:  agentprofile.Signals{ProgramApp: Name, ProgramChildID: "ses_e7fb", RootCwd: "/p"},
		Identity: agentprofile.Identity{Agent: Name, AgentSessionID: "ses_e7fb", Cwd: "/p"},
	})
}
