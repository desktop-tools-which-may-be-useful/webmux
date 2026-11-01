//go:build linux || darwin

package session

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"
)

// TestSidecarUsesLaunchPath: a launch config's PATH finds sidecar
// executables the server's PATH does not, LookPath agrees with the start,
// and defaults only fill variables the launch config leaves unset.
func TestSidecarUsesLaunchPath(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	tool := filepath.Join(dir, "webmux-test-tool")
	if err := os.WriteFile(tool, []byte("#!/bin/sh\necho \"$A-$B\"\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	m := shellManager(t)
	s, err := m.CreateShell(ShellInput{CreateInput: CreateInput{Cmd: []string{"agent"}, Env: map[string]string{"PATH": dir + ":/usr/bin:/bin", "B": "config"}}}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if got, err := s.LookPath("webmux-test-tool"); err != nil || got != tool {
		t.Fatalf("LookPath = %q, %v", got, err)
	}
	if _, err := s.LookPath("webmux-missing-tool"); !errors.Is(err, ErrNoExecutable) {
		t.Fatalf("missing tool: %v", err)
	}
	if _, err := s.StartSidecar([]string{"webmux-missing-tool"}, false, nil); !errors.Is(err, ErrNoExecutable) {
		t.Fatalf("start missing tool: %v", err)
	}
	if got := s.Getenv("B"); got != "config" {
		t.Fatalf("Getenv = %q", got)
	}
	c, err := s.StartSidecar([]string{"webmux-test-tool"}, false, map[string]string{"A": "default", "B": "default"})
	if err != nil {
		t.Fatal(err)
	}
	out := make([]byte, 64)
	n, _ := c.Stdout().Read(out)
	waitSidecarDone(t, c)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	_ = c.Stop(ctx)
	if got := strings.TrimSpace(string(out[:n])); got != "default-config" {
		t.Fatalf("sidecar environment %q", got)
	}
}

func TestOutput(t *testing.T) {
	t.Parallel()
	m := shellManager(t)
	s := createShell(t, m, nil)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	out, code, err := s.Output(ctx, []string{"/bin/sh", "-c", "echo hello; echo noise >&2; exit 3"}, 3)
	if err != nil || string(out) != "hel" || code != 3 {
		t.Fatalf("Output = %q %d %v", out, code, err)
	}
	short, cancelShort := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancelShort()
	start := time.Now()
	if _, _, err := s.Output(short, []string{"/bin/sh", "-c", "sleep 30 & wait"}, 64); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("Output past its deadline: %v", err)
	}
	if time.Since(start) > 5*time.Second {
		t.Fatal("Output did not stop at its deadline")
	}
}

// TestRunProcessFacts: the run's root process's children and the files
// its process tree holds open.
func TestRunProcessFacts(t *testing.T) {
	t.Parallel()
	file := filepath.Join(t.TempDir(), "held.lock")
	if err := os.WriteFile(file, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	want, err := filepath.EvalSymlinks(file)
	if err != nil {
		t.Fatal(err)
	}
	m := shellManager(t)
	s := createShell(t, m, nil, "/bin/sh", "-c", `exec 3<"$0"; sleep 30 & wait`, file)
	deadline := time.Now().Add(10 * time.Second)
	for !slices.Contains(s.RunChildren(), "sleep") || !slices.Contains(s.RunOpenFiles(), want) {
		if time.Now().After(deadline) {
			t.Fatalf("children %q open files %q", s.RunChildren(), s.RunOpenFiles())
		}
		time.Sleep(20 * time.Millisecond)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := s.EndRun(ctx); err != nil {
		t.Fatal(err)
	}
	if s.RunChildren() != nil || s.RunOpenFiles() != nil {
		t.Fatal("process facts without a run")
	}
}
