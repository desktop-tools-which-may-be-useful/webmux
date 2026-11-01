package codex

import (
	"encoding/json"
	"testing"

	"github.com/rainchan/webmux/internal/agentprofile"
	"github.com/rainchan/webmux/internal/agentprofile/agenttest"
)

func TestContract(t *testing.T) {
	agenttest.Run(t, Agent, agenttest.Fixture{
		Signals:      agentprofile.Signals{ProgramApp: Name, RootCwd: "/p"},
		Identity:     agentprofile.Identity{Agent: Name, Cwd: "/p"},
		SteeringInit: json.RawMessage(`{"steering":{"supported":true}}`),
	})
}
