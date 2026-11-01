// Package agenttest is the contract every agent adapter passes: each
// adapter subpackage runs Run with its own fixture, so a new agent gets the
// same checks as the existing ones.
package agenttest

import (
	"encoding/json"
	"slices"
	"testing"

	"github.com/rainchan/webmux/internal/agentprofile"
)

// Fixture is what an adapter's own terminal and ACP side produce.
type Fixture struct {
	// Signals come from the agent's terminal UI; Identity is what the
	// adapter must make of them.
	Signals  agentprofile.Signals
	Identity agentprofile.Identity
	// SteeringInit is an initialize result _meta that declares steering;
	// required for a Steerer.
	SteeringInit json.RawMessage
}

// Run checks adapter a against the contract hybrid sessions rely on.
func Run(t *testing.T, a agentprofile.Agent, f Fixture) {
	t.Helper()
	p := a.Profile()
	registry := agentprofile.NewRegistry(a)
	exe := p.Name
	if len(p.Executables) > 0 {
		exe = p.Executables[0]
	}

	t.Run("profile", func(t *testing.T) {
		if p.Name == "" || p.Title == "" || p.ResumeFlag == "" {
			t.Fatalf("profile lacks name, title or resume flag: %+v", p)
		}
		if _, ok := agentprofile.CompareVersions(p.VerifiedVersion, p.VerifiedVersion); !ok {
			t.Fatalf("verified version %q is not a dotted version", p.VerifiedVersion)
		}
		if p.Connector != nil && (p.Connector.Executable == "" || p.Connector.Title == "") {
			t.Fatalf("connector lacks executable or title: %+v", p.Connector)
		}
	})

	t.Run("command", func(t *testing.T) {
		cmd, err := registry.Parse([]string{"/opt/bin/" + exe})
		if err != nil {
			t.Fatalf("bare %s: %v", exe, err)
		}
		if cmd.Agent != a || cmd.Profile != p {
			t.Fatal("parsed command belongs to another agent")
		}
		tui := cmd.TUIArgv()
		if tui[0] != "/opt/bin/"+exe || !containsAll(tui, p.TUIInject) {
			t.Fatalf("TUIArgv = %q, want the executable and %q", tui, p.TUIInject)
		}
		resume, _ := cmd.ResumeArgv("sid-1")
		if n := len(resume); n < 3 || resume[n-2] != p.ResumeFlag || resume[n-1] != "sid-1" || !containsAll(resume, p.TUIInject) {
			t.Fatalf("ResumeArgv = %q", resume)
		}
		acpArgv, _ := cmd.ACPArgv()
		want := []string{"/opt/bin/" + exe, "acp"}
		if p.Connector != nil {
			want = []string{p.Connector.Executable}
		}
		if !slices.Equal(acpArgv, want) {
			t.Fatalf("ACPArgv = %q, want %q", acpArgv, want)
		}
		if s := registry.Check([]string{"sh", "-c", exe}); s.Supported || s.Reason != agentprofile.ReasonUnsupportedForm {
			t.Fatalf("shell wrapper = %+v, want unsupported form", s)
		}
	})

	t.Run("identify", func(t *testing.T) {
		got, ok := a.Identify(f.Signals)
		if !ok || got != f.Identity || got.Agent != p.Name {
			t.Fatalf("Identify(%+v) = %+v %v, want %+v", f.Signals, got, ok, f.Identity)
		}
		for _, s := range []agentprofile.Signals{{}, {ProgramApp: "not-an-agent", RootExecutable: exe}} {
			if got, ok := a.Identify(s); ok {
				t.Fatalf("Identify(%+v) = %+v, want unrecognised", s, got)
			}
		}
	})

	t.Run("interrupt", func(t *testing.T) {
		keys := a.InterruptKeys()
		if len(keys) == 0 || slices.ContainsFunc(keys, func(k []byte) bool { return len(k) == 0 }) {
			t.Fatalf("interrupt keys %q", keys)
		}
	})

	t.Run("reapply", func(t *testing.T) {
		ids := agentprofile.ReapplyOnLoad(a)
		seen := map[string]bool{}
		for _, id := range ids {
			if id == "" || seen[id] {
				t.Fatalf("reapply options %q: empty or repeated", ids)
			}
			seen[id] = true
		}
	})

	t.Run("steering", func(t *testing.T) {
		s, ok := a.(agentprofile.Steerer)
		if !ok {
			return
		}
		if s.SteeringMethod() == "" {
			t.Fatal("steering without a method")
		}
		if s.SteeringAvailable(nil) || s.SteeringAvailable(json.RawMessage(`{"steering":{"supported":false}}`)) {
			t.Fatal("steering available without being declared")
		}
		if !s.SteeringAvailable(f.SteeringInit) {
			t.Fatalf("steering not available from %s", f.SteeringInit)
		}
		if meta := s.SteeringMeta(); meta != nil && !json.Valid(meta) {
			t.Fatalf("steering _meta %s is not JSON", meta)
		}
	})

	t.Run("force switch note", func(t *testing.T) {
		if agentprofile.ForceSwitchNote(a) == "" {
			t.Fatal("empty force switch note")
		}
	})

	t.Run("locks", func(t *testing.T) {
		l, ok := a.(agentprofile.LockTracker)
		if !ok {
			return
		}
		none := func(string) string { return "" }
		if ids := l.LockedSessions([]string{"/usr/lib/libc.so"}, none, t.TempDir()); len(ids) != 0 {
			t.Fatalf("locked sessions from unrelated files: %q", ids)
		}
	})
}

func containsAll(argv, want []string) bool {
	for _, w := range want {
		if !slices.Contains(argv, w) {
			return false
		}
	}
	return true
}
