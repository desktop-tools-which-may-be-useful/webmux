package claudecode

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	ap "github.com/rainchan/webmux/internal/agentprofile"
)

const testSessionID = "01a18715-8504-7020-9f3d-257900000000"

func TestRegisteredSession(t *testing.T) {
	home := canonicalDir(t)
	cwd := filepath.Join(home, "work")
	if err := os.Mkdir(cwd, 0o700); err != nil {
		t.Fatal(err)
	}
	process := ap.ProcessFacts{PID: 42, Executable: "2.1.316", Cwd: cwd, Start: "12345", Domain: "linux:machine:namespace"}
	writeRecord(t, filepath.Join(home, ".claude"), process.PID, registration{SessionID: testSessionID, PID: process.PID, Cwd: cwd, ProcStart: process.Start, PIDDomain: process.Domain})
	registry := Agent.(ap.ProcessRegistry)
	id, ok := registry.RegisteredSession(process, env(nil), home)
	if !ok || id != testSessionID {
		t.Fatalf("RegisteredSession = %q %v", id, ok)
	}

	tests := []struct {
		name    string
		mutate  func(*registration)
		process ap.ProcessFacts
	}{
		{name: "PID reuse", mutate: func(r *registration) { r.ProcStart = "999" }},
		{name: "PID domain", mutate: func(r *registration) { r.PIDDomain = "other" }},
		{name: "record PID", mutate: func(r *registration) { r.PID++ }},
		{name: "cwd", mutate: func(r *registration) { r.Cwd = "/other" }},
		{name: "UUID", mutate: func(r *registration) { r.SessionID = "not-a-uuid" }},
		{name: "wrong executable", process: ap.ProcessFacts{PID: 42, Executable: "node", Cwd: cwd, Start: "12345", Domain: "linux:machine:namespace"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			record := registration{SessionID: testSessionID, PID: process.PID, Cwd: cwd, ProcStart: process.Start, PIDDomain: process.Domain}
			if tt.mutate != nil {
				tt.mutate(&record)
			}
			writeRecord(t, filepath.Join(home, ".claude"), process.PID, record)
			candidate := process
			if tt.process.PID != 0 {
				candidate = tt.process
			}
			if id, ok := registry.RegisteredSession(candidate, env(nil), home); ok || id != "" {
				t.Fatalf("accepted invalid record %q %v", id, ok)
			}
		})
	}
}

func TestRegisteredSessionRelativeConfigAndBounds(t *testing.T) {
	home := canonicalDir(t)
	cwd := filepath.Join(home, "work")
	config := filepath.Join(cwd, "config")
	if err := os.MkdirAll(config, 0o700); err != nil {
		t.Fatal(err)
	}
	process := ap.ProcessFacts{PID: 7, Executable: "claude", Cwd: cwd, Start: "1", Domain: "d"}
	writeRecord(t, config, process.PID, registration{SessionID: testSessionID, PID: 7, Cwd: cwd, ProcStart: "1", PIDDomain: "d"})
	registry := Agent.(ap.ProcessRegistry)
	if id, ok := registry.RegisteredSession(process, env(map[string]string{configDirEnv: "config"}), home); !ok || id != testSessionID {
		t.Fatalf("relative config = %q %v", id, ok)
	}
	if err := os.WriteFile(recordPath(config, 7), []byte(strings.Repeat(" ", maxRecordBytes+1)), 0o600); err != nil {
		t.Fatal(err)
	}
	if id, ok := registry.RegisteredSession(process, env(map[string]string{configDirEnv: "config"}), home); ok || id != "" {
		t.Fatal("accepted oversized record")
	}
}

func TestExternallyHeld(t *testing.T) {
	home := canonicalDir(t)
	cwd := filepath.Join(home, "work")
	if err := os.Mkdir(cwd, 0o700); err != nil {
		t.Fatal(err)
	}
	current := ap.ProcessFacts{PID: 10, Executable: "claude", Cwd: cwd, Start: "10", Domain: "d"}
	external := ap.ProcessFacts{PID: 20, Executable: "2.1.316", Cwd: cwd, Start: "20", Domain: "d"}
	writeRecord(t, filepath.Join(home, ".claude"), current.PID, registration{SessionID: testSessionID, PID: 10, Cwd: cwd, ProcStart: "10", PIDDomain: "d"})
	writeRecord(t, filepath.Join(home, ".claude"), external.PID, registration{SessionID: testSessionID, PID: 20, Cwd: cwd, ProcStart: "20", PIDDomain: "d"})
	registry := Agent.(ap.ProcessRegistry)
	lookup := func(pid int) (ap.ProcessFacts, bool) {
		if pid == external.PID {
			return external, true
		}
		return ap.ProcessFacts{}, false
	}
	if !registry.ExternallyHeld(testSessionID, current, lookup, env(nil), home) {
		t.Fatal("external valid writer not reported")
	}
	if registry.ExternallyHeld(testSessionID, external, lookup, env(nil), home) {
		t.Fatal("current process counted as external")
	}
	stale := external
	stale.Start = "changed"
	lookup = func(pid int) (ap.ProcessFacts, bool) { return stale, true }
	if registry.ExternallyHeld(testSessionID, current, lookup, env(nil), home) {
		t.Fatal("stale record counted as external")
	}
	lookup = func(int) (ap.ProcessFacts, bool) { return ap.ProcessFacts{}, false }
	if registry.ExternallyHeld(testSessionID, current, lookup, env(nil), home) {
		t.Fatal("exited process counted as external")
	}
}

func TestExternalScanEntryBound(t *testing.T) {
	home := canonicalDir(t)
	cwd := filepath.Join(home, "work")
	if err := os.Mkdir(cwd, 0o700); err != nil {
		t.Fatal(err)
	}
	current := ap.ProcessFacts{PID: 1, Executable: "claude", Cwd: cwd}
	external := ap.ProcessFacts{PID: 999, Executable: "claude", Cwd: cwd, Start: "1", Domain: "d"}
	writeRecord(t, filepath.Join(home, ".claude"), external.PID, registration{SessionID: testSessionID, PID: external.PID, Cwd: cwd, ProcStart: "1", PIDDomain: "d"})
	for i := 2; i <= maxRegistryEntries+2; i++ {
		writeRecord(t, filepath.Join(home, ".claude"), i, registration{SessionID: testSessionID, PID: i, Cwd: cwd, ProcStart: "1", PIDDomain: "d"})
	}
	registry := Agent.(ap.ProcessRegistry)
	lookups := 0
	lookup := func(pid int) (ap.ProcessFacts, bool) {
		lookups++
		if pid == external.PID {
			return external, true
		}
		return ap.ProcessFacts{}, false
	}
	registry.ExternallyHeld(testSessionID, current, lookup, env(nil), home)
	if lookups > maxRegistryEntries {
		t.Fatalf("scan performed %d process lookups, limit %d", lookups, maxRegistryEntries)
	}
}

func writeRecord(t *testing.T, config string, pid int, record registration) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(recordPath(config, pid)), 0o700); err != nil {
		t.Fatal(err)
	}
	data := fmt.Sprintf(`{"sessionId":%q,"pid":%d,"cwd":%q,"procStart":%q,"pidDomain":%q}`,
		record.SessionID, record.PID, record.Cwd, record.ProcStart, record.PIDDomain)
	if err := os.WriteFile(recordPath(config, pid), []byte(data), 0o600); err != nil {
		t.Fatal(err)
	}
}

func recordPath(config string, pid int) string {
	return filepath.Join(config, "sessions", fmt.Sprintf("%d.json", pid))
}

func canonicalDir(t *testing.T) string {
	t.Helper()
	dir, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	return dir
}

func env(values map[string]string) func(string) string {
	return func(name string) string { return values[name] }
}
