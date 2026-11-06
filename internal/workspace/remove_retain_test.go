package workspace

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/rainchan/webmux/internal/hybrid"
)

// cleanArchived creates a linked worktree with nothing but one archived
// record, so a safe removal is allowed.
func (h *quickHarness) cleanArchived(branch string) (Worktree, string) {
	h.t.Helper()
	tree, err := h.s.CreateWorktree(context.Background(), h.repo.ID, CreateRequest{Path: filepath.Join(h.t.TempDir(), branch), Mode: "new", Branch: branch, Base: "HEAD"})
	if err != nil {
		h.t.Fatal(err)
	}
	return tree, h.archive(tree.Path)
}

func (h *quickHarness) remove(tree Worktree, mode string) error {
	h.t.Helper()
	preview := previewTest(h.t, h.s, h.repo, tree)
	req := RemoveRequest{ConfirmationID: preview.ConfirmationID, Mode: mode}
	if mode == "force" {
		req = forceRequest(preview)
	}
	return h.s.RemoveWorktree(context.Background(), h.repo.ID, tree.ID, req)
}

func TestRemovalKeepsArchivedRecordsAsReleased(t *testing.T) {
	for _, mode := range []string{"safe", "force"} {
		t.Run(mode, func(t *testing.T) {
			h := newQuickHarness(t, "")
			tree, id := h.cleanArchived("keep-" + mode)
			preview := previewTest(t, h.s, h.repo, tree)
			if preview.Commit != *tree.Head || len(preview.Archived) != 1 || preview.Archived[0].ID != id {
				t.Fatalf("preview = %+v", preview)
			}
			if err := h.remove(tree, mode); err != nil {
				t.Fatal(err)
			}
			if _, err := os.Stat(tree.Path); !os.IsNotExist(err) {
				t.Fatalf("worktree directory kept: %v", err)
			}
			rel := h.released(id)
			if rel == nil || rel.Commit != *tree.Head || rel.Path != tree.Path || rel.Branch != "keep-"+mode || rel.RepositoryID != h.repo.ID {
				t.Fatalf("release = %+v", rel)
			}
			if got := gitTest(t, h.root, "rev-parse", releasedRef(rel.ID)); strings.TrimSpace(got) != *tree.Head {
				t.Fatalf("private ref = %q", got)
			}
			// The kept record rebuilds at its path and resumes.
			plan, err := h.s.PreviewRebuild(context.Background(), id)
			if err != nil || len(plan.Blockers) != 0 {
				t.Fatalf("rebuild preview = %+v, %v", plan, err)
			}
			rebuilt, err := h.s.RebuildWorktree(context.Background(), id, RebuildRequest{ReleaseID: plan.ReleaseID, Mode: plan.Mode, Commit: plan.Commit})
			if err != nil {
				t.Fatal(err)
			}
			if rebuilt.Path != tree.Path || rebuilt.Head == nil || *rebuilt.Head != *tree.Head || h.released(id) != nil {
				t.Fatalf("rebuilt = %+v", rebuilt)
			}
			if _, err := h.s.ResumeHybrid(context.Background(), id, HybridResumeInput{ClientRequestID: "resume-" + mode}); err != nil {
				t.Fatalf("resume after rebuild: %v", err)
			}
		})
	}
}

// A force removal closes running sessions after the preview: the records
// they leave are kept too.
func TestForceRemovalKeepsRecordsOfClosedSessions(t *testing.T) {
	h := newQuickHarness(t, "")
	tree, archived := h.cleanArchived("force-keep")
	saved, err := h.launches.Get(h.configID)
	if err != nil {
		t.Fatal(err)
	}
	item, err := h.hybrids.Create(context.Background(), hybrid.CreateInput{Cwd: tree.Path, Name: "live", ConfigID: saved.ID, Cmd: saved.Cmd, Env: saved.Env, StartSide: hybrid.SideACP})
	if err != nil {
		t.Fatal(err)
	}
	live := item.Info().ID
	if err := h.remove(tree, "force"); err != nil {
		t.Fatal(err)
	}
	first, second := h.released(archived), h.released(live)
	if first == nil || second == nil || first.ID != second.ID || !h.refExists(releasedRef(first.ID)) {
		t.Fatalf("releases = %+v, %+v", first, second)
	}
}

// Without a resolvable HEAD no commit can be pinned: the removal still
// succeeds and the record stays as it was.
func TestRemovalWithoutCommitKeepsRecordsUnreleased(t *testing.T) {
	h := newQuickHarness(t, "")
	path := filepath.Join(t.TempDir(), "orphan")
	gitTest(t, h.root, "worktree", "add", "--orphan", "-b", "orphan", path)
	repo, err := h.s.Refresh(context.Background(), h.repo.ID)
	if err != nil {
		t.Fatal(err)
	}
	var tree Worktree
	for _, candidate := range repo.Worktrees {
		if candidate.Path == path {
			tree = candidate
		}
	}
	if tree.ID == "" {
		t.Fatalf("orphan worktree missing from %+v", repo.Worktrees)
	}
	id := h.archive(path)
	preview := previewTest(t, h.s, h.repo, tree)
	if preview.Commit != "" || len(preview.Archived) != 1 || preview.Push.State != PushUnknown {
		t.Fatalf("preview = %+v", preview)
	}
	if err := h.remove(tree, "safe"); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatalf("worktree directory kept: %v", err)
	}
	if h.released(id) != nil {
		t.Fatal("record marked released without a commit")
	}
	if out := gitTest(t, h.root, "for-each-ref", releasedRefPrefix); strings.TrimSpace(out) != "" {
		t.Fatalf("private refs: %q", out)
	}
}

// A release that already holds a record is not redone by removing whatever
// occupies its path.
func TestRemovalDoesNotReprocessReleasedRecords(t *testing.T) {
	h := newQuickHarness(t, "")
	tree, id := h.releasable("reprocess")
	h.release(tree)
	before := *h.released(id)
	occupant, err := h.s.CreateWorktree(context.Background(), h.repo.ID, CreateRequest{Path: tree.Path, Mode: "new", Branch: "reprocess-occupant", Base: "HEAD"})
	if err != nil {
		t.Fatal(err)
	}
	if err := h.remove(occupant, "safe"); err != nil {
		t.Fatal(err)
	}
	if after := h.released(id); after == nil || after.ID != before.ID {
		t.Fatalf("release = %+v, want %s", after, before.ID)
	}
	if refs := strings.Fields(gitTest(t, h.root, "for-each-ref", "--format=%(refname)", releasedRefPrefix)); len(refs) != 1 {
		t.Fatalf("private refs = %v", refs)
	}
}

func TestCheckPushedClassifiesCommit(t *testing.T) {
	h := newQuickHarness(t, "")
	r, err := h.s.repository(h.repo.ID)
	if err != nil {
		t.Fatal(err)
	}
	head := strings.TrimSpace(gitTest(t, h.root, "rev-parse", "HEAD"))
	check := func(ctx context.Context, commit string) PushCheck {
		t.Helper()
		return h.s.checkPushed(ctx, r, commit)
	}
	if got := check(context.Background(), head); got.State != PushNoRemote {
		t.Fatalf("no remote = %+v", got)
	}
	gitTest(t, h.root, "remote", "add", "origin", filepath.Join(t.TempDir(), "remote.git"))
	if got := check(context.Background(), head); got.State != PushUnpushed || len(got.Branches) != 0 {
		t.Fatalf("unpushed = %+v", got)
	}
	// A remote's HEAD mirrors a branch and does not count on its own.
	gitTest(t, h.root, "update-ref", "refs/remotes/origin/main", head)
	gitTest(t, h.root, "symbolic-ref", "refs/remotes/origin/HEAD", "refs/remotes/origin/main")
	got := check(context.Background(), head)
	if got.State != PushPushed || len(got.Branches) != 1 || got.Branches[0] != "origin/main" || got.More != 0 {
		t.Fatalf("pushed = %+v", got)
	}
	// A commit past the remote branch is not contained in it.
	gitTest(t, h.root, "commit", "--allow-empty", "-m", "local only")
	local := strings.TrimSpace(gitTest(t, h.root, "rev-parse", "HEAD"))
	if got := check(context.Background(), local); got.State != PushUnpushed {
		t.Fatalf("local commit = %+v", got)
	}
	for i := range maxPushBranches + 2 {
		gitTest(t, h.root, "update-ref", "refs/remotes/origin/b"+string(rune('a'+i)), head)
	}
	if got := check(context.Background(), head); got.State != PushPushed || len(got.Branches) != maxPushBranches || got.More != 3 {
		t.Fatalf("many branches = %+v", got)
	}
	if got := check(context.Background(), ""); got.State != PushUnknown {
		t.Fatalf("no commit = %+v", got)
	}
	if got := check(context.Background(), strings.Repeat("0", 40)); got.State != PushUnknown {
		t.Fatalf("missing object = %+v", got)
	}
	cancelled, cancel := context.WithCancel(context.Background())
	cancel()
	if got := check(cancelled, head); got.State != PushUnknown || got.Branches == nil {
		t.Fatalf("cancelled = %+v", got)
	}
}
