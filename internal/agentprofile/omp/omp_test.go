package omp

import (
	"encoding/json"
	"testing"

	"github.com/rainchan/webmux/internal/agentprofile"
	"github.com/rainchan/webmux/internal/agentprofile/agenttest"
)

func TestContract(t *testing.T) {
	agenttest.Run(t, Agent, agenttest.Fixture{
		Signals:      agentprofile.Signals{WarpAgent: Name, WarpPluginVersion: "18.13.1", WarpSessionID: "01a", WarpCwd: "/p", ProgramApp: Name, RootCwd: "/other"},
		Identity:     agentprofile.Identity{Agent: Name, Version: "18.13.1", AgentSessionID: "01a", Cwd: "/p"},
		SteeringInit: json.RawMessage(`{"steering":{"supported":true}}`),
	})
}
