package artifacts

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
)

// A staged upload is accepted only from the staging folder, moves to the
// session's uploads where UploadPath accepts it, survives the sweep and is
// dropped when the server starts again.
func TestStagedUploadsMoveToTheirSession(t *testing.T) {
	m := newManager(t)
	s := New(Options{TempDir: t.TempDir(), StateDir: t.TempDir(), Manager: m})
	dir, err := s.EnsureStaging()
	if err != nil {
		t.Fatal(err)
	}
	staged := filepath.Join(dir, "spec.md")
	if err := os.WriteFile(staged, []byte("# spec"), 0o600); err != nil {
		t.Fatal(err)
	}
	if got, err := s.StagedPath(staged); err != nil || got != staged {
		t.Fatalf("staged path = %q, %v", got, err)
	}
	outside := filepath.Join(t.TempDir(), "spec.md")
	if err := os.WriteFile(outside, []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	for _, path := range []string{outside, "spec.md", filepath.Join(dir, "missing")} {
		if _, err := s.StagedPath(path); !errors.Is(err, ErrInvalid) {
			t.Fatalf("staged path %q = %v", path, err)
		}
	}
	link := filepath.Join(dir, "link")
	if err := os.Symlink(outside, link); err != nil {
		t.Fatal(err)
	}
	if _, err := s.StagedPath(link); !errors.Is(err, ErrInvalid) {
		t.Fatalf("symlink accepted: %v", err)
	}

	id := create(t, m)
	s.Sweep(func(string) bool { return false })
	adopted, err := s.Adopt(id, []string{staged})
	if err != nil || len(adopted) != 1 {
		t.Fatalf("adopt = %v, %v", adopted, err)
	}
	if got, err := s.UploadPath(id, adopted[0]); err != nil || filepath.Base(got) != "spec.md" {
		t.Fatalf("adopted path %q not an upload: %v", got, err)
	}
	if _, err := os.Lstat(staged); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("staged file still there: %v", err)
	}
	if _, err := s.Adopt(id, []string{staged}); !errors.Is(err, ErrInvalid) {
		t.Fatalf("adopting twice = %v", err)
	}

	left := filepath.Join(dir, "left.txt")
	if err := os.WriteFile(left, []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	restarted := New(Options{TempDir: filepath.Dir(s.root), StateDir: t.TempDir(), Manager: m})
	restarted.root = s.root
	restarted.Start(func(string) bool { return false })
	defer restarted.Close()
	if _, err := os.Lstat(left); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("staged file kept after start: %v", err)
	}
	if _, err := os.Lstat(adopted[0]); err != nil {
		t.Fatalf("session upload removed on start: %v", err)
	}
}
