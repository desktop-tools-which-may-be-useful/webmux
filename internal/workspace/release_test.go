package workspace

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/rainchan/webmux/internal/hybrid"
)

// archive starts a chat session in dir and closes it, leaving an archived
// record there.
func (h *quickHarness) archive(dir string) string {
	h.t.Helper()
	saved, err := h.launches.Get(h.configID)
	if err != nil {
		h.t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), quickTimeout)
	defer cancel()
	item, err := h.hybrids.Create(ctx, hybrid.CreateInput{Cwd: dir, Name: "agent", ConfigID: saved.ID, Cmd: saved.Cmd, Env: saved.Env, StartSide: hybrid.SideACP})
	if err != nil {
		h.t.Fatal(err)
	}
	id := item.Info().ID
	if err := h.manager.Close(id); err != nil {
		h.t.Fatal(err)
	}
	waitDone(h.t, item)
	return id
}

// releasable creates a linked worktree with an ignored directory and one
// archived record.
func (h *quickHarness) releasable(branch string) (Worktree, string) {
	h.t.Helper()
	putFile(h.t, filepath.Join(h.root, ".git", "info", "exclude"), "node_modules/\n")
	tree, err := h.s.CreateWorktree(context.Background(), h.repo.ID, CreateRequest{Path: filepath.Join(h.t.TempDir(), branch), Mode: "new", Branch: branch, Base: "HEAD"})
	if err != nil {
		h.t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(tree.Path, "node_modules", "pkg"), 0o755); err != nil {
		h.t.Fatal(err)
	}
	putFile(h.t, filepath.Join(tree.Path, "node_modules", "pkg", "index.js"), "module.exports = 1;\n")
	return tree, h.archive(tree.Path)
}

func (h *quickHarness) release(tree Worktree) ReleaseResult {
	h.t.Helper()
	preview, err := h.s.PreviewRelease(context.Background(), h.repo.ID, tree.ID)
	if err != nil {
		h.t.Fatal(err)
	}
	if len(preview.Blockers) != 0 {
		h.t.Fatalf("release blockers: %v", preview.Blockers)
	}
	result, err := h.s.ReleaseWorktree(context.Background(), h.repo.ID, tree.ID, ReleaseRequest{ConfirmationID: preview.ConfirmationID})
	if err != nil {
		h.t.Fatal(err)
	}
	return result
}

func (h *quickHarness) refExists(ref string) bool {
	_, err := h.s.git(context.Background(), h.root, "rev-parse", "--verify", "--quiet", ref)
	return err == nil
}

func (h *quickHarness) released(id string) *hybrid.Release {
	h.t.Helper()
	rec, err := h.hybrids.Record(id)
	if err != nil {
		h.t.Fatal(err)
	}
	return rec.Released
}

func TestReleaseWithOnlyIgnoredFilesKeepsCommitBranchAndRecords(t *testing.T) {
	h := newQuickHarness(t, "")
	tree, id := h.releasable("ignored-only")
	preview, err := h.s.PreviewRelease(context.Background(), h.repo.ID, tree.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(preview.Blockers) != 0 || preview.Commit != *tree.Head || len(preview.Archived) != 1 || preview.Archived[0].ID != id {
		t.Fatalf("preview = %+v", preview)
	}
	if len(preview.Ignored) != 1 || preview.Ignored[0].Path != "node_modules/" || preview.Ignored[0].Bytes == nil || *preview.Ignored[0].Bytes != 20 {
		t.Fatalf("ignored = %+v", preview.Ignored)
	}
	if !preview.SizeComplete || preview.IgnoredBytes == nil || *preview.IgnoredBytes != 20 || preview.IgnoredMore != 0 {
		t.Fatalf("size = %+v", preview)
	}
	// A removal confirmation cannot execute a release, nor the reverse.
	removal := previewTest(t, h.s, h.repo, tree)
	if _, err := h.s.ReleaseWorktree(context.Background(), h.repo.ID, tree.ID, ReleaseRequest{ConfirmationID: removal.ConfirmationID}); errorKind(err) != ErrConflict {
		t.Fatalf("removal confirmation released: %v", err)
	}
	if err := h.s.RemoveWorktree(context.Background(), h.repo.ID, tree.ID, RemoveRequest{ConfirmationID: preview.ConfirmationID, Mode: "force", ConfirmPath: tree.Path, CloseSessions: true}); errorKind(err) != ErrConflict {
		t.Fatalf("release confirmation removed: %v", err)
	}
	result, err := h.s.ReleaseWorktree(context.Background(), h.repo.ID, tree.ID, ReleaseRequest{ConfirmationID: preview.ConfirmationID})
	if err != nil {
		t.Fatal(err)
	}
	if len(result.RecordIDs) != 1 || result.RecordIDs[0] != id {
		t.Fatalf("result = %+v", result)
	}
	if _, err := os.Stat(tree.Path); !os.IsNotExist(err) {
		t.Fatalf("worktree directory kept: %v", err)
	}
	if got := gitTest(t, h.root, "rev-parse", releasedRef(result.ReleaseID)); strings.TrimSpace(got) != *tree.Head {
		t.Fatalf("private ref = %q", got)
	}
	gitTest(t, h.root, "show-ref", "--verify", "refs/heads/ignored-only")
	rel := h.released(id)
	if rel == nil || rel.ID != result.ReleaseID || rel.Commit != *tree.Head || rel.Branch != "ignored-only" || rel.Path != tree.Path || rel.RepositoryID != h.repo.ID {
		t.Fatalf("release = %+v", rel)
	}
	// Resuming before a rebuild is refused.
	if _, err := h.s.ResumeHybrid(context.Background(), id, HybridResumeInput{ClientRequestID: "resume-released"}); !errors.Is(err, hybrid.ErrConflict) {
		t.Fatalf("released record resumed: %v", err)
	}
}

func TestReleaseRefusesUnsafeContents(t *testing.T) {
	h := newQuickHarness(t, "")
	for _, category := range []string{"modified", "untracked", "running"} {
		t.Run(category, func(t *testing.T) {
			tree, _ := h.releasable("unsafe-" + category)
			switch category {
			case "modified":
				putFile(t, filepath.Join(tree.Path, "tracked"), "changed\n")
			case "untracked":
				putFile(t, filepath.Join(tree.Path, "untracked"), "new\n")
			case "running":
				item, err := h.s.CreateSession(context.Background(), CreateInput{Cwd: tree.Path, Cmd: []string{"/bin/sh", "-c", "printf READY; read value"}})
				if err != nil {
					t.Fatal(err)
				}
				waitOutput(t, item, "READY")
			}
			preview, err := h.s.PreviewRelease(context.Background(), h.repo.ID, tree.ID)
			if err != nil {
				t.Fatal(err)
			}
			if len(preview.Blockers) == 0 {
				t.Fatal("unsafe release not blocked")
			}
			if _, err := h.s.ReleaseWorktree(context.Background(), h.repo.ID, tree.ID, ReleaseRequest{ConfirmationID: preview.ConfirmationID}); errorKind(err) != ErrConflict {
				t.Fatalf("unsafe release: %v", err)
			}
			assertExists(t, tree.Path)
		})
	}
}

func TestReleaseRejectsTargetChangedAfterConfirmation(t *testing.T) {
	h := newQuickHarness(t, "")
	tree, id := h.releasable("changed")
	preview, err := h.s.PreviewRelease(context.Background(), h.repo.ID, tree.ID)
	if err != nil {
		t.Fatal(err)
	}
	gitTest(t, tree.Path, "commit", "--allow-empty", "-m", "moved")
	if _, err := h.s.ReleaseWorktree(context.Background(), h.repo.ID, tree.ID, ReleaseRequest{ConfirmationID: preview.ConfirmationID}); errorKind(err) != ErrConflict {
		t.Fatalf("changed target released: %v", err)
	}
	assertExists(t, tree.Path)
	if h.released(id) != nil {
		t.Fatal("refused release marked the record")
	}
}

func TestReleaseRollsBackWhenGitRemovalFails(t *testing.T) {
	h := newQuickHarness(t, "")
	tree, id := h.releasable("rollback")
	preview, err := h.s.PreviewRelease(context.Background(), h.repo.ID, tree.ID)
	if err != nil {
		t.Fatal(err)
	}
	// A Git wrapper that refuses only the removal.
	wrapper := filepath.Join(t.TempDir(), "git")
	putFile(t, wrapper, "#!/bin/sh\nfor a in \"$@\"; do [ \"$a\" = remove ] && { echo refused >&2; exit 1; }; done\nexec "+h.s.gitPath+" \"$@\"\n")
	if err := os.Chmod(wrapper, 0o700); err != nil {
		t.Fatal(err)
	}
	h.s.gitPath = wrapper
	if _, err := h.s.ReleaseWorktree(context.Background(), h.repo.ID, tree.ID, ReleaseRequest{ConfirmationID: preview.ConfirmationID}); errorKind(err) != ErrConflict {
		t.Fatalf("failed removal: %v", err)
	}
	assertExists(t, tree.Path)
	if h.released(id) != nil {
		t.Fatal("release mark not rolled back")
	}
	if out := gitTest(t, h.root, "for-each-ref", releasedRefPrefix); strings.TrimSpace(out) != "" {
		t.Fatalf("private ref not rolled back: %q", out)
	}
}

// Git can fail after it has already dropped the worktree, e.g. when a file
// inside cannot be deleted; the commit must then stay reachable.
func TestReleaseKeepsMarkAndRefWhenGitFailsAfterRemoving(t *testing.T) {
	h := newQuickHarness(t, "")
	tree, id := h.releasable("partial")
	preview, err := h.s.PreviewRelease(context.Background(), h.repo.ID, tree.ID)
	if err != nil {
		t.Fatal(err)
	}
	wrapper := filepath.Join(t.TempDir(), "git")
	putFile(t, wrapper, "#!/bin/sh\nfor a in \"$@\"; do [ \"$a\" = remove ] && { "+h.s.gitPath+" \"$@\"; echo partial >&2; exit 1; }; done\nexec "+h.s.gitPath+" \"$@\"\n")
	if err := os.Chmod(wrapper, 0o700); err != nil {
		t.Fatal(err)
	}
	h.s.gitPath = wrapper
	if _, err := h.s.ReleaseWorktree(context.Background(), h.repo.ID, tree.ID, ReleaseRequest{ConfirmationID: preview.ConfirmationID}); errorKind(err) != ErrConflict {
		t.Fatalf("failed removal: %v", err)
	}
	rel := h.released(id)
	if rel == nil || rel.Commit != preview.Commit {
		t.Fatalf("release mark dropped: %+v", rel)
	}
	if !h.refExists(releasedRef(rel.ID)) {
		t.Fatal("private ref dropped after git removed the worktree")
	}
}

func TestRebuildModes(t *testing.T) {
	h := newQuickHarness(t, "")
	rebuild := func(id string, want, reason string) Worktree {
		t.Helper()
		preview, err := h.s.PreviewRebuild(context.Background(), id)
		if err != nil {
			t.Fatal(err)
		}
		if len(preview.Blockers) != 0 || preview.Mode != want || preview.Reason != reason {
			t.Fatalf("preview = %+v, want %s/%s", preview, want, reason)
		}
		wrong := RebuildReuse
		if want == RebuildReuse {
			wrong = RebuildDetached
		}
		if _, err := h.s.RebuildWorktree(context.Background(), id, RebuildRequest{ReleaseID: preview.ReleaseID, Mode: wrong, Commit: preview.Commit}); errorKind(err) != ErrConflict {
			t.Fatalf("mismatched mode %s accepted: %v", wrong, err)
		}
		tree, err := h.s.RebuildWorktree(context.Background(), id, RebuildRequest{ReleaseID: preview.ReleaseID, Mode: want, Commit: preview.Commit})
		if err != nil {
			t.Fatal(err)
		}
		if tree.Path != preview.Path || tree.Head == nil || *tree.Head != preview.Commit {
			t.Fatalf("rebuilt = %+v", tree)
		}
		if h.released(id) != nil || h.refExists(releasedRef(preview.ReleaseID)) {
			t.Fatal("release not cleared after rebuild")
		}
		return tree
	}

	t.Run("branch", func(t *testing.T) {
		tree, id := h.releasable("rebuild-branch")
		h.release(tree)
		got := rebuild(id, RebuildBranch, "")
		if got.Branch == nil || *got.Branch != "rebuild-branch" {
			t.Fatalf("branch = %+v", got)
		}
	})
	t.Run("detached moved", func(t *testing.T) {
		tree, id := h.releasable("rebuild-moved")
		h.release(tree)
		gitTest(t, h.root, "commit", "--allow-empty", "-m", "advance")
		gitTest(t, h.root, "branch", "-f", "rebuild-moved", "HEAD")
		if got := rebuild(id, RebuildDetached, ReasonBranchMoved); !got.Detached {
			t.Fatalf("not detached: %+v", got)
		}
	})
	t.Run("detached missing", func(t *testing.T) {
		tree, id := h.releasable("rebuild-missing")
		h.release(tree)
		gitTest(t, h.root, "branch", "-D", "rebuild-missing")
		if got := rebuild(id, RebuildDetached, ReasonBranchMissing); !got.Detached {
			t.Fatalf("not detached: %+v", got)
		}
	})
	t.Run("reuse", func(t *testing.T) {
		tree, id := h.releasable("rebuild-reuse")
		h.release(tree)
		gitTest(t, h.root, "worktree", "add", tree.Path, "rebuild-reuse")
		rebuild(id, RebuildReuse, "")
	})
	t.Run("occupied", func(t *testing.T) {
		tree, id := h.releasable("rebuild-occupied")
		result := h.release(tree)
		if err := os.MkdirAll(tree.Path, 0o755); err != nil {
			t.Fatal(err)
		}
		putFile(t, filepath.Join(tree.Path, "other"), "x")
		preview, err := h.s.PreviewRebuild(context.Background(), id)
		if err != nil {
			t.Fatal(err)
		}
		if len(preview.Blockers) == 0 {
			t.Fatalf("occupied path not blocked: %+v", preview)
		}
		if _, err := h.s.RebuildWorktree(context.Background(), id, RebuildRequest{ReleaseID: result.ReleaseID, Mode: preview.Mode, Commit: preview.Commit}); errorKind(err) != ErrConflict {
			t.Fatalf("occupied rebuild: %v", err)
		}
		if h.released(id) == nil || !h.refExists(releasedRef(result.ReleaseID)) {
			t.Fatal("blocked rebuild cleared the release")
		}
	})
}

func TestDropReleaseAfterLastRecord(t *testing.T) {
	h := newQuickHarness(t, "")
	tree, first := h.releasable("drop")
	second := h.archive(tree.Path)
	result := h.release(tree)
	if len(result.RecordIDs) != 2 {
		t.Fatalf("result = %+v", result)
	}
	rel := *h.released(first)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := h.hybrids.RemoveRecord(ctx, first); err != nil {
		t.Fatal(err)
	}
	if err := h.s.DropRelease(ctx, rel); err != nil {
		t.Fatal(err)
	}
	if !h.refExists(releasedRef(rel.ID)) {
		t.Fatal("ref dropped while a record still uses it")
	}
	if err := h.hybrids.RemoveRecord(ctx, second); err != nil {
		t.Fatal(err)
	}
	if err := h.s.DropRelease(ctx, rel); err != nil {
		t.Fatal(err)
	}
	if h.refExists(releasedRef(rel.ID)) {
		t.Fatal("ref kept after its last record")
	}
}

// A worktree later created at a released path is not the released one:
// removing it must keep the released records restorable.
func TestRemovingOccupantKeepsReleasedRecords(t *testing.T) {
	h := newQuickHarness(t, "")
	tree, id := h.releasable("released-path")
	h.release(tree)
	occupant, err := h.s.CreateWorktree(context.Background(), h.repo.ID, CreateRequest{Path: tree.Path, Mode: "new", Branch: "occupant", Base: "HEAD"})
	if err != nil {
		t.Fatal(err)
	}
	preview := previewTest(t, h.s, h.repo, occupant)
	if len(preview.Archived) != 0 {
		t.Fatalf("removal preview lists released records: %+v", preview.Archived)
	}
	if err := h.s.RemoveWorktree(context.Background(), h.repo.ID, occupant.ID, RemoveRequest{ConfirmationID: preview.ConfirmationID, Mode: "safe"}); err != nil {
		t.Fatal(err)
	}
	if h.released(id) == nil {
		t.Fatal("released record lost")
	}
}
