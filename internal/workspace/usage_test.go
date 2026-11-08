package workspace

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestMeasureWorktreeCountsRegularFilesWithoutFollowingLinks(t *testing.T) {
	h := newQuickHarness(t, "")
	tree, err := h.s.CreateWorktree(context.Background(), h.repo.ID, CreateRequest{Path: filepath.Join(t.TempDir(), "sized"), Mode: "new", Branch: "sized", Base: "HEAD"})
	if err != nil {
		t.Fatal(err)
	}
	measure := func() Usage {
		t.Helper()
		usage, err := h.s.MeasureWorktree(context.Background(), h.repo.ID, tree.ID)
		if err != nil {
			t.Fatal(err)
		}
		if !usage.Complete {
			t.Fatalf("incomplete: %+v", usage)
		}
		return usage
	}
	before := measure()

	if err := os.MkdirAll(filepath.Join(tree.Path, "cache", "deep"), 0o755); err != nil {
		t.Fatal(err)
	}
	putFile(t, filepath.Join(tree.Path, "cache", "deep", "a.bin"), "12345")
	putFile(t, filepath.Join(tree.Path, "cache", "b.bin"), "123")
	// A link to a large file outside the worktree adds an entry, not bytes.
	outside := filepath.Join(t.TempDir(), "outside.bin")
	putFile(t, outside, string(make([]byte, 4096)))
	if err := os.Symlink(outside, filepath.Join(tree.Path, "link")); err != nil {
		t.Fatal(err)
	}

	after := measure()
	if got := after.Bytes - before.Bytes; got != 8 {
		t.Fatalf("bytes grew by %d, want 8", got)
	}
	// cache/, cache/deep/, two files and the link.
	if got := after.Entries - before.Entries; got != 5 {
		t.Fatalf("entries grew by %d, want 5", got)
	}
}

func TestMeasureWorktreeRefusesMissingDirectoryAndUnknownIDs(t *testing.T) {
	h := newQuickHarness(t, "")
	tree, err := h.s.CreateWorktree(context.Background(), h.repo.ID, CreateRequest{Path: filepath.Join(t.TempDir(), "gone"), Mode: "new", Branch: "gone", Base: "HEAD"})
	if err != nil {
		t.Fatal(err)
	}
	if err := os.RemoveAll(tree.Path); err != nil {
		t.Fatal(err)
	}
	if _, err := h.s.MeasureWorktree(context.Background(), h.repo.ID, tree.ID); errorKind(err) != ErrConflict {
		t.Fatalf("missing directory: %v", err)
	}
	if _, err := h.s.MeasureWorktree(context.Background(), h.repo.ID, "missing"); errorKind(err) != ErrNotFound {
		t.Fatalf("unknown worktree: %v", err)
	}
	if _, err := h.s.MeasureWorktree(context.Background(), "missing", tree.ID); errorKind(err) != ErrNotFound {
		t.Fatalf("unknown repository: %v", err)
	}
}
