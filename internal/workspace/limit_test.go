package workspace

import (
	"context"
	"errors"
	"fmt"
	"testing"
)

func fakeRegistrations(n int) []registration {
	out := make([]registration, 0, n)
	for i := range n {
		out = append(out, registration{ID: fmt.Sprintf("id%04d", i), CommonDir: fmt.Sprintf("/repos/r%04d/.git", i), AnchorPath: fmt.Sprintf("/repos/r%04d", i), DisplayName: "r"})
	}
	return out
}

func TestRegistryRefusesMoreThanMaxRepositories(t *testing.T) {
	t.Parallel()
	isolateGit(t)
	root := initRepo(t)
	s, _ := testService(t, t.TempDir(), root)
	s.mu.Lock()
	for _, entry := range fakeRegistrations(MaxRepositories) {
		s.entries = append(s.entries, newRepository(entry))
	}
	s.mu.Unlock()
	if _, _, err := s.Register(context.Background(), root); !errors.Is(err, ErrInvalid) {
		t.Fatalf("register past the limit: %v, want ErrInvalid", err)
	}
	s.mu.Lock()
	n := len(s.entries)
	s.mu.Unlock()
	if n != MaxRepositories || !s.Capability().RegistryAvailable {
		t.Fatalf("registry changed by a refused registration: %d entries", n)
	}
}

func TestValidateRegistryBoundsRepositoryCount(t *testing.T) {
	t.Parallel()
	if err := validateRegistry(registry{Version: 1, Repositories: fakeRegistrations(MaxRepositories)}); err != nil {
		t.Fatalf("a full registry: %v", err)
	}
	if err := validateRegistry(registry{Version: 1, Repositories: fakeRegistrations(MaxRepositories + 1)}); err == nil {
		t.Fatal("a registry past the limit loaded")
	}
}
