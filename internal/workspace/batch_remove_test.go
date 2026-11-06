package workspace

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/rainchan/webmux/internal/hybrid"
	"github.com/rainchan/webmux/internal/session"
)

func TestPrunableMissingWorktreeCleanupRetainsBranchesUnselectedEntriesAndArchivedRecords(t *testing.T) {
	h := newQuickHarness(t, "")
	target, recID := h.cleanArchived("prunable-target")
	gitTest(t, target.Path, "commit", "--allow-empty", "-m", "prunable-commit")
	refreshed, err := h.s.Refresh(context.Background(), h.repo.ID)
	if err != nil {
		t.Fatal(err)
	}
	for _, wt := range refreshed.Worktrees {
		if wt.ID == target.ID {
			target = wt
		}
	}
	keep := linkedTest(t, h.s, h.repo, "prunable-keep")
	toctou := linkedTest(t, h.s, h.repo, "prunable-toctou")

	for _, path := range []string{target.Path, keep.Path, toctou.Path} {
		if err := os.RemoveAll(path); err != nil {
			t.Fatal(err)
		}
	}

	// Release remains strictly for available directories.
	relPreview, err := h.s.PreviewRelease(context.Background(), h.repo.ID, target.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(relPreview.Blockers) == 0 {
		t.Fatalf("expected missing directory to block release, got %+v", relPreview)
	}

	// Recreating a path after previewing a prunable worktree is rejected before git remove.
	toctouPreview := previewTest(t, h.s, h.repo, toctou)
	if len(toctouPreview.Blockers) != 0 || !toctouPreview.ForceAllowed {
		t.Fatalf("toctou preview = %+v", toctouPreview)
	}
	if err := os.MkdirAll(toctou.Path, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := h.s.RemoveWorktree(context.Background(), h.repo.ID, toctou.ID, RemoveRequest{
		ConfirmationID: toctouPreview.ConfirmationID,
		Mode:           "safe",
	}); errorKind(err) != ErrConflict {
		t.Fatalf("recreated prunable path accepted: %v", err)
	}

	// Safe cleanup removes only target's stale registration, keeps local branch,
	// leaves unselected prunable entries untouched, and pins archived chat commit.
	preview := previewTest(t, h.s, h.repo, target)
	if len(preview.Blockers) != 0 || !preview.ForceAllowed || preview.Commit != *target.Head || len(preview.Archived) != 1 || preview.Archived[0].ID != recID {
		t.Fatalf("prunable preview = %+v", preview)
	}
	if err := h.s.RemoveWorktree(context.Background(), h.repo.ID, target.ID, RemoveRequest{
		ConfirmationID: preview.ConfirmationID,
		Mode:           "safe",
	}); err != nil {
		t.Fatal(err)
	}

	after, err := h.s.Refresh(context.Background(), h.repo.ID)
	if err != nil {
		t.Fatal(err)
	}
	foundTarget := false
	foundKeep := false
	for _, wt := range after.Worktrees {
		if wt.ID == target.ID {
			foundTarget = true
		}
		if wt.ID == keep.ID {
			foundKeep = wt.Prunable && !wt.Available
		}
	}
	if foundTarget {
		t.Fatal("cleaned prunable worktree remained in snapshot")
	}
	if !foundKeep {
		t.Fatal("unselected prunable worktree was pruned unexpectedly")
	}
	gitTest(t, h.root, "show-ref", "--verify", "refs/heads/prunable-target")
	rel := h.released(recID)
	if rel == nil || rel.Commit != *target.Head || rel.Branch != "prunable-target" {
		t.Fatalf("archived record not retained as released: %+v", rel)
	}
	if got := gitTest(t, h.root, "rev-parse", releasedRef(rel.ID)); strings.TrimSpace(got) != *target.Head {
		t.Fatalf("released ref = %q, want %s", got, *target.Head)
	}
}

func TestBatchRemovalSafeAndForceModesAndPurposeIsolation(t *testing.T) {
	h := newQuickHarness(t, "")
	putFile(t, filepath.Join(h.root, ".git", "info", "exclude"), ".env\n")

	mainTree := h.repo.Worktrees[0]
	lockedTree := linkedTest(t, h.s, h.repo, "batch-locked")
	gitTest(t, h.root, "worktree", "lock", "--reason", "pinned-by-user", lockedTree.Path)

	cleanTree := linkedTest(t, h.s, h.repo, "batch-clean")
	putFile(t, filepath.Join(cleanTree.Path, ".env"), "LOCAL=1\n")

	prunableTree, prunableRec := h.cleanArchived("batch-prunable")
	if err := os.RemoveAll(prunableTree.Path); err != nil {
		t.Fatal(err)
	}

	dirtyTree := linkedTest(t, h.s, h.repo, "batch-dirty")
	putFile(t, filepath.Join(dirtyTree.Path, "untracked.txt"), "draft\n")

	activeTree := linkedTest(t, h.s, h.repo, "batch-active")
	ptyItem, err := h.s.CreateSession(context.Background(), CreateInput{
		Cwd: activeTree.Path,
		Cmd: []string{"/bin/sh", "-c", "printf READY; read value"},
	})
	if err != nil {
		t.Fatal(err)
	}
	waitOutput(t, ptyItem, "READY")

	saved, err := h.launches.Get(h.configID)
	if err != nil {
		t.Fatal(err)
	}
	liveHybrid, err := h.hybrids.Create(context.Background(), hybrid.CreateInput{
		Cwd:       activeTree.Path,
		Name:      "live-agent",
		ConfigID:  saved.ID,
		Cmd:       saved.Cmd,
		Env:       saved.Env,
		StartSide: hybrid.SideACP,
	})
	if err != nil {
		t.Fatal(err)
	}
	liveHybridID := liveHybrid.Info().ID

	// Request validation boundaries.
	if _, err := h.s.PreviewBatchRemoval(context.Background(), h.repo.ID, nil); errorKind(err) != ErrInvalid {
		t.Fatalf("empty ids: %v", err)
	}
	tooMany := make([]string, maxBatchWorktrees+1)
	for i := range tooMany {
		tooMany[i] = string(rune('a'+(i%26))) + "-" + strings.Repeat("x", i/26+1)
	}
	if _, err := h.s.PreviewBatchRemoval(context.Background(), h.repo.ID, tooMany); errorKind(err) != ErrInvalid {
		t.Fatalf("exceeding maxBatchWorktrees: %v", err)
	}
	if _, err := h.s.PreviewBatchRemoval(context.Background(), h.repo.ID, []string{"wt_missing"}); errorKind(err) != ErrNotFound {
		t.Fatalf("missing worktree id: %v", err)
	}

	// Purpose isolation: batch confirmation rejected by single remove/release, and vice versa.
	isoPreview, err := h.s.PreviewBatchRemoval(context.Background(), h.repo.ID, []string{cleanTree.ID})
	if err != nil {
		t.Fatal(err)
	}
	if err := h.s.RemoveWorktree(context.Background(), h.repo.ID, cleanTree.ID, RemoveRequest{
		ConfirmationID: isoPreview.ConfirmationID,
		Mode:           "safe",
	}); errorKind(err) != ErrConflict {
		t.Fatalf("batch confirmation accepted by RemoveWorktree: %v", err)
	}
	singlePreview := previewTest(t, h.s, h.repo, cleanTree)
	if _, err := h.s.BatchRemoveWorktrees(context.Background(), h.repo.ID, BatchRemoveRequest{
		ConfirmationID: singlePreview.ConfirmationID,
	}); errorKind(err) != ErrConflict {
		t.Fatalf("single confirmation accepted by BatchRemoveWorktrees: %v", err)
	}

	// Deduplicated preview across all 6 worktrees.
	batchPreview, err := h.s.PreviewBatchRemoval(context.Background(), h.repo.ID, []string{
		mainTree.ID,
		lockedTree.ID,
		cleanTree.ID,
		prunableTree.ID,
		dirtyTree.ID,
		activeTree.ID,
		cleanTree.ID,
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(batchPreview.Items) != 6 {
		t.Fatalf("expected 6 deduplicated items, got %d", len(batchPreview.Items))
	}

	// IncludeForce without CloseSessions is rejected before consuming confirmation.
	if _, err := h.s.BatchRemoveWorktrees(context.Background(), h.repo.ID, BatchRemoveRequest{
		ConfirmationID: batchPreview.ConfirmationID,
		IncludeForce:   true,
		CloseSessions:  false,
	}); errorKind(err) != ErrInvalid {
		t.Fatalf("unconfirmed closeSessions accepted: %v", err)
	}

	// Safe batch execution deletes only safe items and skips unsupported + blocked items.
	safeRes, err := h.s.BatchRemoveWorktrees(context.Background(), h.repo.ID, BatchRemoveRequest{
		ConfirmationID: batchPreview.ConfirmationID,
		IncludeForce:   false,
		CloseSessions:  false,
	})
	if err != nil {
		t.Fatal(err)
	}
	wantSafe := map[string]string{
		mainTree.ID:     "skipped",
		lockedTree.ID:   "skipped",
		cleanTree.ID:    "removed",
		prunableTree.ID: "removed",
		dirtyTree.ID:    "skipped",
		activeTree.ID:   "skipped",
	}
	for _, item := range safeRes.Results {
		if item.Status != wantSafe[item.WorktreeID] {
			t.Fatalf("worktree %s status = %q, want %q (err=%q)", item.WorktreeID, item.Status, wantSafe[item.WorktreeID], item.Error)
		}
	}
	if _, err := os.Stat(cleanTree.Path); !os.IsNotExist(err) {
		t.Fatalf("cleanTree kept: %v", err)
	}
	assertExists(t, dirtyTree.Path)
	assertExists(t, activeTree.Path)
	if ptyItem.Info().Status == session.StatusExited {
		t.Fatal("safe batch closed active PTY")
	}
	if h.released(prunableRec) == nil {
		t.Fatal("prunable archived record not marked released")
	}
	gitTest(t, h.root, "show-ref", "--verify", "refs/heads/batch-clean")
	gitTest(t, h.root, "show-ref", "--verify", "refs/heads/batch-prunable")

	// Force batch execution closes active sessions, retains closed hybrid records,
	// deletes blocked items, and still skips locked/unsupported items.
	forcePreview, err := h.s.PreviewBatchRemoval(context.Background(), h.repo.ID, []string{
		lockedTree.ID,
		dirtyTree.ID,
		activeTree.ID,
	})
	if err != nil {
		t.Fatal(err)
	}
	forceRes, err := h.s.BatchRemoveWorktrees(context.Background(), h.repo.ID, BatchRemoveRequest{
		ConfirmationID: forcePreview.ConfirmationID,
		IncludeForce:   true,
		CloseSessions:  true,
	})
	if err != nil {
		t.Fatal(err)
	}
	wantForce := map[string]string{
		lockedTree.ID: "skipped",
		dirtyTree.ID:  "removed",
		activeTree.ID: "removed",
	}
	for _, item := range forceRes.Results {
		if item.Status != wantForce[item.WorktreeID] {
			t.Fatalf("force worktree %s status = %q, want %q (err=%q)", item.WorktreeID, item.Status, wantForce[item.WorktreeID], item.Error)
		}
	}
	select {
	case <-ptyItem.Done():
	default:
		t.Fatal("force batch returned before PTY exited")
	}
	if h.released(liveHybridID) == nil {
		t.Fatal("closed hybrid session was not retained as released")
	}
	if _, err := os.Stat(dirtyTree.Path); !os.IsNotExist(err) {
		t.Fatalf("dirtyTree kept: %v", err)
	}
	if _, err := os.Stat(activeTree.Path); !os.IsNotExist(err) {
		t.Fatalf("activeTree kept: %v", err)
	}
	assertExists(t, lockedTree.Path)
	gitTest(t, h.root, "show-ref", "--verify", "refs/heads/batch-dirty")
	gitTest(t, h.root, "show-ref", "--verify", "refs/heads/batch-active")
}

func TestBatchRemovalPartialFailureIsolationAndPathReservation(t *testing.T) {
	t.Parallel()
	isolateGit(t)
	root := initRepo(t)
	s, _ := testService(t, t.TempDir(), root)
	r := registerTest(t, s, root)

	cleanA := linkedTest(t, s, r, "iso-a")
	cleanB := linkedTest(t, s, r, "iso-b")
	cleanC := linkedTest(t, s, r, "iso-c")

	preview, err := s.PreviewBatchRemoval(context.Background(), r.ID, []string{cleanA.ID, cleanB.ID, cleanC.ID})
	if err != nil {
		t.Fatal(err)
	}
	// Mutate cleanB after preview; even if IncludeForce is true, an item that was
	// clean in preview runs in safe mode and fails without aborting cleanA/cleanC.
	putFile(t, filepath.Join(cleanB.Path, "after-preview.txt"), "keep-me\n")

	res, err := s.BatchRemoveWorktrees(context.Background(), r.ID, BatchRemoveRequest{
		ConfirmationID: preview.ConfirmationID,
		IncludeForce:   true,
		CloseSessions:  true,
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Results) != 3 ||
		res.Results[0].Status != "removed" ||
		res.Results[1].Status != "failed" || res.Results[1].Error == "" ||
		res.Results[2].Status != "removed" {
		t.Fatalf("unexpected partial failure results: %+v", res.Results)
	}
	assertExists(t, filepath.Join(cleanB.Path, "after-preview.txt"))
	if _, err := os.Stat(cleanA.Path); !os.IsNotExist(err) {
		t.Fatalf("cleanA not removed: %v", err)
	}
	if _, err := os.Stat(cleanC.Path); !os.IsNotExist(err) {
		t.Fatalf("cleanC not removed: %v", err)
	}

	// Verify all actionable target paths are reserved up front while batch remove runs.
	resTarget1 := linkedTest(t, s, r, "reserve-1")
	resTarget2 := linkedTest(t, s, r, "reserve-2")
	resPreview, err := s.PreviewBatchRemoval(context.Background(), r.ID, []string{resTarget1.ID, resTarget2.ID})
	if err != nil {
		t.Fatal(err)
	}

	entered := make(chan struct{})
	releaseGit := make(chan struct{})
	var once sync.Once
	realGit := s.gitPath
	wrapper := filepath.Join(t.TempDir(), "git-pause")
	gateFile := filepath.Join(t.TempDir(), "gate")
	enteredFile := filepath.Join(t.TempDir(), "entered")
	script := "#!/bin/sh\n" +
		"for a in \"$@\"; do\n" +
		"  if [ \"$a\" = \"remove\" ] && [ ! -f \"" + enteredFile + "\" ]; then\n" +
		"    : > \"" + enteredFile + "\"\n" +
		"    while [ ! -f \"" + gateFile + "\" ]; do sleep 0.02; done\n" +
		"  fi\n" +
		"done\n" +
		"exec \"" + realGit + "\" \"$@\"\n"
	putFile(t, wrapper, script)
	if err := os.Chmod(wrapper, 0o700); err != nil {
		t.Fatal(err)
	}
	s.gitPath = wrapper

	go func() {
		for {
			if _, statErr := os.Stat(enteredFile); statErr == nil {
				once.Do(func() { close(entered) })
				break
			}
			time.Sleep(10 * time.Millisecond)
		}
		<-releaseGit
		_ = os.WriteFile(gateFile, []byte("ok"), 0o600)
	}()

	done := make(chan BatchRemoveResult, 1)
	errCh := make(chan error, 1)
	go func() {
		out, batchErr := s.BatchRemoveWorktrees(context.Background(), r.ID, BatchRemoveRequest{
			ConfirmationID: resPreview.ConfirmationID,
		})
		if batchErr != nil {
			errCh <- batchErr
			return
		}
		done <- out
	}()

	select {
	case <-entered:
	case <-time.After(5 * time.Second):
		t.Fatal("timed out waiting for batch git remove")
	}

	// Even while removing resTarget1, resTarget2's path is already reserved!
	if _, err := s.CreateSession(context.Background(), CreateInput{
		Cwd: resTarget2.Path,
		Cmd: []string{"/bin/sh", "-c", "exit 0"},
	}); errorKind(err) != ErrConflict {
		t.Fatalf("expected concurrent CreateSession on reserved target2 to conflict, got: %v", err)
	}
	close(releaseGit)

	select {
	case batchErr := <-errCh:
		t.Fatal(batchErr)
	case out := <-done:
		if len(out.Results) != 2 || out.Results[0].Status != "removed" || out.Results[1].Status != "removed" {
			t.Fatalf("batch remove after unblock: %+v", out.Results)
		}
	case <-time.After(10 * time.Second):
		t.Fatal("timed out waiting for batch remove completion")
	}

	// Reservations are released once BatchRemoveWorktrees finishes.
	s.coord.Lock()
	leftover := len(s.reservations)
	s.coord.Unlock()
	if leftover != 0 {
		t.Fatalf("path reservations leaked: %d", leftover)
	}
}
