package workspace

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func storedInstructions(t *testing.T, state string) map[string]string {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(state, "repositories.json"))
	if err != nil {
		t.Fatal(err)
	}
	var stored registry
	if err := json.Unmarshal(data, &stored); err != nil {
		t.Fatal(err)
	}
	out := map[string]string{}
	for _, entry := range stored.Repositories {
		out[entry.ID] = entry.InitInstruction
	}
	return out
}

func listedInstruction(t *testing.T, s *Service, id string) string {
	t.Helper()
	list, err := s.List(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	for _, repository := range list {
		if repository.ID == id {
			return repository.InitInstruction
		}
	}
	t.Fatalf("repository %s not listed", id)
	return ""
}

// The instruction is saved with the registration, survives a restart,
// can be replaced and cleared, is limited to MaxInitInstruction characters,
// and a duplicate registration keeps it.
func TestInitInstructionLifecycle(t *testing.T) {
	isolateGit(t)
	root := initRepo(t)
	state := t.TempDir()
	s, _ := testService(t, state, root)
	ctx := context.Background()
	if _, _, err := s.Register(ctx, root, strings.Repeat("长", MaxInitInstruction+1)); errorKind(err) != ErrInvalid {
		t.Fatalf("over-long registration: %v", err)
	}
	if _, err := os.Stat(filepath.Join(state, "repositories.json")); !os.IsNotExist(err) {
		t.Fatalf("rejected registration wrote the registry: %v", err)
	}
	repository, created, err := s.Register(ctx, root, "先读 AGENTS.md")
	if err != nil || !created || repository.InitInstruction != "先读 AGENTS.md" {
		t.Fatalf("register: %+v %v %v", repository, created, err)
	}
	if got := storedInstructions(t, state)[repository.ID]; got != "先读 AGENTS.md" {
		t.Fatalf("stored instruction %q", got)
	}
	again, created, err := s.Register(ctx, root, "另一条指令")
	if err != nil || created || again.ID != repository.ID || again.InitInstruction != "先读 AGENTS.md" {
		t.Fatalf("duplicate registration: %+v %v %v", again, created, err)
	}

	exact := strings.Repeat("长", MaxInitInstruction)
	updated, err := s.SetInitInstruction(ctx, repository.ID, exact)
	if err != nil || updated.InitInstruction != exact || updated.ID != repository.ID {
		t.Fatalf("set at the limit: %v", err)
	}
	if _, err := s.SetInitInstruction(ctx, repository.ID, exact+"x"); errorKind(err) != ErrInvalid {
		t.Fatalf("over the limit: %v", err)
	}
	if got := listedInstruction(t, s, repository.ID); got != exact {
		t.Fatal("a rejected instruction replaced the saved one")
	}
	if _, err := s.SetInitInstruction(ctx, "missing", "x"); errorKind(err) != ErrNotFound {
		t.Fatalf("unknown repository: %v", err)
	}
	if _, err := s.SetInitInstruction(ctx, repository.ID, "改完运行 make check"); err != nil {
		t.Fatal(err)
	}
	if got := listedInstruction(t, s, repository.ID); got != "改完运行 make check" {
		t.Fatalf("listed instruction %q", got)
	}
	s.Close()
	reopened, _ := testService(t, state, root)
	if got := listedInstruction(t, reopened, repository.ID); got != "改完运行 make check" {
		t.Fatalf("instruction after restart %q", got)
	}
	cleared, err := reopened.SetInitInstruction(ctx, repository.ID, "  \n")
	if err != nil || cleared.InitInstruction != "" {
		t.Fatalf("clear: %+v %v", cleared, err)
	}
	data, err := os.ReadFile(filepath.Join(state, "repositories.json"))
	if err != nil || strings.Contains(string(data), "initInstruction") {
		t.Fatalf("cleared instruction still stored: %s %v", data, err)
	}
}

// A registry written before instructions existed loads as repositories
// without one.
func TestInitInstructionLegacyRegistry(t *testing.T) {
	isolateGit(t)
	root := initRepo(t)
	state := t.TempDir()
	first, _ := testService(t, state, root)
	repository := registerTest(t, first, root)
	first.Close()
	data, err := os.ReadFile(filepath.Join(state, "repositories.json"))
	if err != nil || strings.Contains(string(data), "initInstruction") {
		t.Fatalf("registry without an instruction: %s %v", data, err)
	}
	s, _ := testService(t, state, root)
	if !s.Capability().RegistryAvailable {
		t.Fatalf("legacy registry rejected: %+v", s.Capability())
	}
	if got := listedInstruction(t, s, repository.ID); got != "" {
		t.Fatalf("legacy instruction %q", got)
	}
}

// A save that fails before publication leaves the previous instruction
// in memory and on disk.
func TestInitInstructionSaveFailureKeepsPrevious(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("root bypasses directory write permissions")
	}
	isolateGit(t)
	root := initRepo(t)
	state := t.TempDir()
	s, _ := testService(t, state, root)
	repository, _, err := s.Register(context.Background(), root, "before")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(state, 0o500); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(state, 0o700) })
	if _, err := s.SetInitInstruction(context.Background(), repository.ID, "after"); err == nil {
		t.Fatal("unwritable registry reported success")
	}
	if err := os.Chmod(state, 0o700); err != nil {
		t.Fatal(err)
	}
	if got := listedInstruction(t, s, repository.ID); got != "before" {
		t.Fatalf("instruction after a failed save %q", got)
	}
	if got := storedInstructions(t, state)[repository.ID]; got != "before" {
		t.Fatalf("stored instruction after a failed save %q", got)
	}
}
