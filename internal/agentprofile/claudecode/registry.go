package claudecode

import (
	"bytes"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	ap "github.com/rainchan/webmux/internal/agentprofile"
)

const (
	configDirEnv       = "CLAUDE_CONFIG_DIR"
	maxRecordBytes     = 64 << 10
	maxRegistryEntries = 256
)

type registration struct {
	SessionID string `json:"sessionId"`
	PID       int    `json:"pid"`
	Cwd       string `json:"cwd"`
	ProcStart string `json:"procStart"`
	PIDDomain string `json:"pidDomain"`
}

// RegisteredSession returns the current native process's validated session
// ID. All failures deliberately collapse to false: hybrid then uses its
// generic session/list candidate fallback.
func (adapter) RegisteredSession(process ap.ProcessFacts, getenv func(string) string, userHome string) (string, bool) {
	if !profile.RunsProcess(process.Executable) || process.PID <= 0 {
		return "", false
	}
	record, ok := readRegistration(configDir(process.Cwd, getenv, userHome), process.PID)
	if !ok || !validRegistration(record, process) {
		return "", false
	}
	return record.SessionID, true
}

// ExternallyHeld reports a best-effort match in the same configuration
// directory. Invalid, stale, exited and current-process records are ignored.
func (adapter) ExternallyHeld(sessionID string, current ap.ProcessFacts, lookup func(int) (ap.ProcessFacts, bool), getenv func(string) string, userHome string) bool {
	if !validUUID(sessionID) || lookup == nil {
		return false
	}
	dir := filepath.Join(configDir(current.Cwd, getenv, userHome), "sessions")
	directory, err := os.Open(dir)
	if err != nil {
		return false
	}
	defer func() { _ = directory.Close() }()
	entries, err := directory.ReadDir(maxRegistryEntries)
	if err != nil && len(entries) == 0 {
		return false
	}
	for _, entry := range entries {
		pid, ok := recordFilenamePID(entry.Name())
		if !ok || entry.IsDir() || pid == current.PID {
			continue
		}
		record, ok := readRegistration(configDir(current.Cwd, getenv, userHome), pid)
		if !ok || record.SessionID != sessionID {
			continue
		}
		process, ok := lookup(pid)
		if ok && profile.RunsProcess(process.Executable) && validRegistration(record, process) {
			return true
		}
	}
	return false
}

func configDir(cwd string, getenv func(string) string, userHome string) string {
	configured := ""
	if getenv != nil {
		configured = strings.TrimSpace(getenv(configDirEnv))
	}
	if configured == "" {
		if userHome == "" {
			return ""
		}
		return filepath.Join(userHome, ".claude")
	}
	if !filepath.IsAbs(configured) && cwd != "" {
		configured = filepath.Join(cwd, configured)
	}
	return filepath.Clean(configured)
}

func readRegistration(configDir string, pid int) (registration, bool) {
	var zero registration
	if configDir == "" || pid <= 0 {
		return zero, false
	}
	path := filepath.Join(configDir, "sessions", strconv.Itoa(pid)+".json")
	file, err := os.Open(path)
	if err != nil {
		return zero, false
	}
	defer func() { _ = file.Close() }()
	data, err := io.ReadAll(io.LimitReader(file, maxRecordBytes+1))
	if err != nil || len(data) > maxRecordBytes {
		return zero, false
	}
	var record registration
	decoder := json.NewDecoder(bytes.NewReader(data))
	if err := decoder.Decode(&record); err != nil {
		return zero, false
	}
	var extra any
	if err := decoder.Decode(&extra); err != io.EOF {
		return zero, false
	}
	return record, true
}

func validRegistration(record registration, process ap.ProcessFacts) bool {
	return record.PID == process.PID && validUUID(record.SessionID) &&
		record.ProcStart != "" && record.ProcStart == process.Start &&
		record.PIDDomain != "" && record.PIDDomain == process.Domain &&
		samePath(record.Cwd, process.Cwd)
}

func samePath(a, b string) bool {
	if a == "" || b == "" {
		return false
	}
	a, b = filepath.Clean(a), filepath.Clean(b)
	if resolved, err := filepath.EvalSymlinks(a); err == nil {
		a = resolved
	}
	if resolved, err := filepath.EvalSymlinks(b); err == nil {
		b = resolved
	}
	return a == b
}

func recordFilenamePID(name string) (int, bool) {
	base, ok := strings.CutSuffix(name, ".json")
	if !ok {
		return 0, false
	}
	pid, err := strconv.Atoi(base)
	return pid, err == nil && pid > 0
}

func validUUID(id string) bool {
	if len(id) != 36 {
		return false
	}
	for i := range len(id) {
		if i == 8 || i == 13 || i == 18 || i == 23 {
			if id[i] != '-' {
				return false
			}
			continue
		}
		if !strings.ContainsRune("0123456789abcdefABCDEF", rune(id[i])) {
			return false
		}
	}
	return true
}
