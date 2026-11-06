package workspace

import (
	"context"
	"errors"
	"log/slog"
	"os"

	"github.com/rainchan/webmux/internal/hybrid"
)

// Rebuild modes and detached reasons.
const (
	RebuildReuse    = "reuse"
	RebuildBranch   = "branch"
	RebuildDetached = "detached"

	ReasonBranchMissing    = "branchMissing"
	ReasonBranchMoved      = "branchMoved"
	ReasonBranchCheckedOut = "branchCheckedOut"
)

// RebuildPreview says how a released record's worktree comes back: the
// existing worktree is reused, its branch is checked out again, or the
// commit is checked out detached for Reason. Non-empty Blockers refuse.
type RebuildPreview struct {
	ReleaseID string
	Path      string
	Commit    string
	Branch    string
	Mode      string
	Reason    string
	Blockers  []string
}

// RebuildRequest executes a rebuild preview; Mode and Commit must still be
// what the service computes.
type RebuildRequest struct {
	ReleaseID string
	Mode      string
	Commit    string
}

// releasedRecord returns record id's release, ErrConflict when it has none.
func (s *Service) releasedRecord(id string) (hybrid.Release, error) {
	if s.hybrids == nil {
		return hybrid.Release{}, hybrid.ErrNotFound
	}
	rec, err := s.hybrids.Record(id)
	if err != nil {
		return hybrid.Release{}, err
	}
	if rec.Released == nil {
		return hybrid.Release{}, problemf(ErrConflict, "该记录的工作树没有释放空间，无需重建")
	}
	return *rec.Released, nil
}

// rebuildPlan computes the rebuild of rel under r's gate; r is nil when no
// registered repository holds the release, which a blocker reports.
func (s *Service) rebuildPlan(ctx context.Context, r *repositoryState, rel hybrid.Release) (RebuildPreview, Worktree, error) {
	plan := RebuildPreview{ReleaseID: rel.ID, Path: rel.Path, Commit: rel.Commit, Branch: rel.Branch, Mode: RebuildDetached, Blockers: []string{}}
	if r == nil {
		plan.Blockers = append(plan.Blockers, "仓库未登记，请重新登记仓库后再重建")
		return plan, Worktree{}, nil
	}
	repository, err := s.current(ctx, r)
	if err != nil {
		if ctx.Err() != nil {
			return plan, Worktree{}, ctx.Err()
		}
		plan.Blockers = append(plan.Blockers, "仓库不可用: "+err.Error())
		return plan, Worktree{}, nil
	}
	var registered *Worktree
	for i, tree := range repository.Worktrees {
		if tree.Path == rel.Path {
			registered = &repository.Worktrees[i]
		}
	}
	if _, err := os.Lstat(rel.Path); !errors.Is(err, os.ErrNotExist) {
		if err == nil && registered != nil && registered.Kind == "linked" && registered.Available &&
			registered.Head != nil && *registered.Head == rel.Commit {
			plan.Mode = RebuildReuse
			return plan, *registered, nil
		}
		plan.Blockers = append(plan.Blockers, "原路径已被占用，请先清理后再重建: "+rel.Path)
		return plan, Worktree{}, nil
	}
	if registered != nil {
		plan.Blockers = append(plan.Blockers, "Git 仍登记原路径的工作树，请先在终端执行 git worktree prune")
		return plan, Worktree{}, nil
	}
	if _, err := s.git(ctx, r.entry.AnchorPath, "rev-parse", "--verify", "--end-of-options", rel.Commit+"^{commit}"); err != nil {
		if errorKind(err) != ErrInternal {
			return plan, Worktree{}, err
		}
		plan.Blockers = append(plan.Blockers, "保存的提交已不存在，无法重建")
		return plan, Worktree{}, nil
	}
	if rel.Branch == "" {
		return plan, Worktree{}, nil
	}
	tip, err := s.git(ctx, r.entry.AnchorPath, "rev-parse", "--verify", "--quiet", "--end-of-options", "refs/heads/"+rel.Branch+"^{commit}")
	switch {
	case err != nil && errorKind(err) != ErrInternal:
		return plan, Worktree{}, err
	case err != nil:
		plan.Reason = ReasonBranchMissing
	case outputLine(tip) != rel.Commit:
		plan.Reason = ReasonBranchMoved
	default:
		for _, tree := range repository.Worktrees {
			if tree.Branch != nil && *tree.Branch == rel.Branch {
				plan.Reason = ReasonBranchCheckedOut
			}
		}
	}
	if plan.Reason == "" {
		plan.Mode = RebuildBranch
	}
	return plan, Worktree{}, nil
}

// PreviewRebuild previews rebuilding released record id's worktree.
func (s *Service) PreviewRebuild(ctx context.Context, id string) (RebuildPreview, error) {
	ctx, cancel := context.WithTimeout(ctx, readTimeout)
	defer cancel()
	rel, err := s.releasedRecord(id)
	if err != nil {
		return RebuildPreview{}, err
	}
	r := s.releaseRepository(ctx, rel)
	if r != nil {
		if err := acquire(ctx, r); err != nil {
			return RebuildPreview{}, err
		}
		defer release(r)
	}
	plan, _, err := s.rebuildPlan(ctx, r, rel)
	return plan, err
}

// RebuildWorktree rebuilds released record id's worktree as previewed and
// clears the release from every record sharing it. A changed mode or
// commit, or a blocker, refuses with ErrConflict.
func (s *Service) RebuildWorktree(ctx context.Context, id string, req RebuildRequest) (Worktree, error) {
	ctx, cancel := context.WithTimeout(ctx, writeTimeout)
	defer cancel()
	if req.Mode != RebuildReuse && req.Mode != RebuildBranch && req.Mode != RebuildDetached {
		return Worktree{}, problemf(ErrInvalid, "重建方式必须为 reuse、branch 或 detached")
	}
	rel, err := s.releasedRecord(id)
	if err != nil {
		return Worktree{}, err
	}
	if rel.ID != req.ReleaseID || rel.Commit != req.Commit {
		return Worktree{}, problemf(ErrConflict, "释放信息已变化，请重新确认")
	}
	r := s.releaseRepository(ctx, rel)
	if r == nil {
		return Worktree{}, problemf(ErrConflict, "仓库未登记，请重新登记仓库后再重建")
	}
	if err := acquire(ctx, r); err != nil {
		return Worktree{}, err
	}
	defer release(r)
	plan, reused, err := s.rebuildPlan(ctx, r, rel)
	if err != nil {
		return Worktree{}, err
	}
	if len(plan.Blockers) != 0 {
		return Worktree{}, problemf(ErrConflict, "重建被阻止: %v", plan.Blockers)
	}
	if plan.Mode != req.Mode {
		return Worktree{}, problemf(ErrConflict, "重建方式已变化，请重新确认")
	}
	tree := reused
	if plan.Mode != RebuildReuse {
		tree, err = s.addReleased(ctx, r, rel, plan.Mode)
		if err != nil {
			return Worktree{}, err
		}
	}
	// The worktree exists again whatever the client does now.
	cleanupCtx, cleanupCancel := context.WithTimeout(context.WithoutCancel(ctx), writeTimeout)
	defer cleanupCancel()
	if _, err := s.hybrids.ClearReleased(cleanupCtx, rel.ID); err != nil {
		return Worktree{}, err
	}
	s.dropRef(cleanupCtx, r, rel.ID)
	s.sweepReleased(cleanupCtx, r)
	slog.Info("worktree rebuilt", "release", rel.ID, "mode", plan.Mode)
	return tree, nil
}

// addReleased checks rel's worktree out again at its path: its branch, or
// its commit detached. The caller holds r's gate.
func (s *Service) addReleased(ctx context.Context, r *repositoryState, rel hybrid.Release, mode string) (Worktree, error) {
	if mode == RebuildDetached {
		return s.addDetached(ctx, r, rel.Path, rel.Commit)
	}
	return s.addWorktree(ctx, r, rel.Path, []string{"worktree", "add", "--no-guess-remote", "--", rel.Path, rel.Branch})
}

// addDetached checks commit out detached at path. It is internal to
// rebuilds; CreateRequest keeps its new/existing modes.
func (s *Service) addDetached(ctx context.Context, r *repositoryState, path, commit string) (Worktree, error) {
	return s.addWorktree(ctx, r, path, []string{"worktree", "add", "--detach", "--", path, commit})
}

// addWorktree runs a worktree add for path and returns the refreshed
// worktree; like CreateWorktree it never rolls back.
func (s *Service) addWorktree(ctx context.Context, r *repositoryState, path string, argv []string) (Worktree, error) {
	if err := s.beginStart(path); err != nil {
		return Worktree{}, err
	}
	defer s.endStart(path)
	_, operationErr := s.git(ctx, r.entry.AnchorPath, argv...)
	refreshCtx, refreshCancel := context.WithTimeout(context.Background(), readTimeout)
	defer refreshCancel()
	actual := s.readRepository(refreshCtx, r)
	if operationErr != nil {
		category := errorKind(operationErr)
		if category == ErrInternal {
			category = ErrConflict
		}
		return Worktree{}, problemf(category, "%v；请刷新实际工作树，可能存在残留", operationErr)
	}
	if actual.Availability != "available" {
		return Worktree{}, problemf(ErrConflict, "Git 已重建工作树但刷新失败: %s", actual.Error)
	}
	for _, tree := range actual.Worktrees {
		if tree.Path == path {
			return tree, nil
		}
	}
	return Worktree{}, problemf(ErrConflict, "Git 重建后未发现目标，请刷新实际状态")
}
