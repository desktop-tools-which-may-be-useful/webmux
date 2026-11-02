package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

// claudeRegistration is Claude Code's per-process session registry entry.
// The fake writes the same Linux identity facts as the real TUI so hybrid
// eligibility and external-occupancy tests exercise the production parser.
type claudeRegistration struct {
	SessionID string `json:"sessionId"`
	PID       int    `json:"pid"`
	Cwd       string `json:"cwd"`
	ProcStart string `json:"procStart"`
	PIDDomain string `json:"pidDomain"`
}

func claudeConfigDir(cwd string) (string, error) {
	dir := strings.TrimSpace(os.Getenv("CLAUDE_CONFIG_DIR"))
	if dir == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", err
		}
		dir = filepath.Join(home, ".claude")
	} else if !filepath.IsAbs(dir) {
		dir = filepath.Join(cwd, dir)
	}
	return filepath.Clean(dir), nil
}

func writeClaudeRegistration(cwd, sessionID string) error {
	pid := os.Getpid()
	start, domain, err := claudeProcessIdentity(pid)
	if err != nil {
		return fmt.Errorf("Claude process identity: %w", err)
	}
	dir, err := claudeConfigDir(cwd)
	if err != nil {
		return fmt.Errorf("Claude config directory: %w", err)
	}
	dir = filepath.Join(dir, "sessions")
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return fmt.Errorf("Claude session registry: %w", err)
	}
	record := claudeRegistration{SessionID: sessionID, PID: pid, Cwd: cwd, ProcStart: start, PIDDomain: domain}
	data, err := json.Marshal(record)
	if err != nil {
		return err
	}
	path := filepath.Join(dir, strconv.Itoa(pid)+".json")
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, data, 0o600); err != nil {
		return fmt.Errorf("Claude session registry: %w", err)
	}
	if err := os.Rename(tmp, path); err != nil {
		return fmt.Errorf("Claude session registry: %w", err)
	}
	return nil
}

func removeClaudeRegistration() {
	dir, err := claudeConfigDir("")
	if err != nil {
		return
	}
	path := filepath.Join(dir, "sessions", strconv.Itoa(os.Getpid())+".json")
	if err := os.Remove(path); err != nil && !errors.Is(err, fs.ErrNotExist) {
		fmt.Fprintln(os.Stderr, "fakeagent: remove Claude session registry:", err)
	}
}
