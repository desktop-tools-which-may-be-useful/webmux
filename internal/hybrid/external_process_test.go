package hybrid

import (
	"testing"

	"github.com/rainchan/webmux/internal/agentprofile"
	"github.com/rainchan/webmux/internal/session"
)

func TestSelectDeclaredAgentProcess(t *testing.T) {
	agent := agentprofile.Spec{
		ProcessExecutables: []string{"acme-native"},
		Commands: []agentprofile.CommandSpec{{
			Match:    []string{"isolate-run", "acme", "work"},
			ACP:      []string{"acme-bridge", "serve"},
			Resume:   []string{"isolate-run", "acme", "work", "resume"},
			Launcher: "sandbox-root",
		}},
	}.Agent("acme")
	cmd, err := agentprofile.NewRegistry(agent).Parse([]string{"isolate-run", "acme", "work"})
	if err != nil {
		t.Fatal(err)
	}

	root := session.ProcessFacts{PID: 10, Executable: "sandbox-root", Cwd: "/launcher"}
	native := session.ProcessFacts{PID: 11, Executable: "acme-native", Cwd: "/agent"}
	unrelated := session.ProcessFacts{PID: 12, Executable: "helper", Cwd: "/helper"}

	tests := []struct {
		name      string
		processes []session.ProcessFacts
		wantOK    bool
		wantPID   int
		wantCwd   string
	}{
		{name: "native root", processes: []session.ProcessFacts{{PID: 20, Executable: "acme-native", Cwd: "/root"}}, wantOK: true, wantPID: 20, wantCwd: "/root"},
		{name: "launcher with no native child", processes: []session.ProcessFacts{root, unrelated}},
		{name: "launcher with one native child", processes: []session.ProcessFacts{root, unrelated, native}, wantOK: true, wantPID: 11, wantCwd: "/agent"},
		{name: "launcher with two native children", processes: []session.ProcessFacts{root, native, session.ProcessFacts{PID: 13, Executable: "acme-native", Cwd: "/other"}}},
		{name: "undeclared root", processes: []session.ProcessFacts{{PID: 30, Executable: "other-launcher", Cwd: "/other"}, native}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got, ok := selectAgentProcess(cmd, test.processes)
			if ok != test.wantOK || got.PID != test.wantPID || got.Cwd != test.wantCwd {
				t.Fatalf("selectAgentProcess() = %+v, %v; want PID %d cwd %q, %v", got, ok, test.wantPID, test.wantCwd, test.wantOK)
			}
		})
	}
}
