package workspace

import (
	"context"
	"time"
)

// PreviewBatchRemoval inspects the requested worktrees of repositoryID under a
// single repository gate and stores one batch-removal confirmation covering
// every target's identity, sessions, and resolved HEAD commit.
func (s *Service) PreviewBatchRemoval(ctx context.Context, repositoryID string, worktreeIDs []string) (BatchPreview, error) {
	if len(worktreeIDs) == 0 {
		return BatchPreview{}, problemf(ErrInvalid, "至少需要选择一个工作树")
	}
	seen := make(map[string]bool, len(worktreeIDs))
	ids := make([]string, 0, len(worktreeIDs))
	for _, raw := range worktreeIDs {
		if raw == "" {
			return BatchPreview{}, problemf(ErrInvalid, "工作树 ID 不能为空")
		}
		if !seen[raw] {
			seen[raw] = true
			ids = append(ids, raw)
		}
	}
	if len(ids) > maxBatchWorktrees {
		return BatchPreview{}, problemf(ErrInvalid, "单次批量管理最多支持 %d 个工作树", maxBatchWorktrees)
	}
	timeout := min(maxBatchWriteTimeout, max(readTimeout, time.Duration(len(ids))*2*time.Second))
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	r, err := s.repository(repositoryID)
	if err != nil {
		return BatchPreview{}, err
	}
	if err := acquire(ctx, r); err != nil {
		return BatchPreview{}, err
	}
	defer release(r)

	trees, err := s.currentTrees(ctx, r, ids)
	if err != nil {
		return BatchPreview{}, err
	}

	batch := make([]batchTarget, 0, len(trees))
	items := make([]BatchPreviewItem, 0, len(trees))
	closeAll := func() {
		for i := range batch {
			batch[i].state.identity.close()
		}
	}
	for _, tree := range trees {
		state, err := s.removalStateFromTree(ctx, r, tree)
		if err != nil {
			closeAll()
			return BatchPreview{}, err
		}
		commit := s.headCommit(ctx, r, state.tree)
		records := s.targetRecords(state.tree.Path)
		check := s.checkPushed(ctx, r, commit)
		batch = append(batch, batchTarget{state: state, commit: commit})
		items = append(items, BatchPreviewItem{
			Worktree:     state.tree,
			Sessions:     state.sessions,
			Archived:     records,
			Commit:       commit,
			Push:         check,
			Blockers:     state.messages(),
			Warnings:     append([]string{}, state.warnings...),
			ForceAllowed: state.supported,
		})
	}
	if err := ctx.Err(); err != nil {
		closeAll()
		return BatchPreview{}, err
	}
	id, expires, err := s.confirm(confirmation{
		purpose:      purposeBatchRemove,
		repositoryID: repositoryID,
		batch:        batch,
	})
	if err != nil {
		return BatchPreview{}, err
	}
	return BatchPreview{
		ConfirmationID: id,
		ExpiresAt:      expires,
		Items:          items,
	}, nil
}

// BatchRemoveWorktrees executes a confirmed batch removal. Unsupported items
// (and blocked items when IncludeForce is false) are skipped; actionable paths
// are reserved up front and removed one by one without letting one target's
// failure abort independent targets.
func (s *Service) BatchRemoveWorktrees(ctx context.Context, repositoryID string, req BatchRemoveRequest) (BatchRemoveResult, error) {
	if req.IncludeForce && !req.CloseSessions {
		return BatchRemoveResult{}, problemf(ErrInvalid, "批量强制删除必须明确授权关闭运行中的会话")
	}
	c, ok := s.takeConfirmation(req.ConfirmationID, purposeBatchRemove, repositoryID, "")
	if !ok {
		return BatchRemoveResult{}, problemf(ErrConflict, "批量删除确认已失效，请重新预览")
	}
	defer c.closeHandles()

	timeout := min(maxBatchWriteTimeout, max(writeTimeout, time.Duration(len(c.batch))*15*time.Second))
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	r, err := s.repository(repositoryID)
	if err != nil {
		return BatchRemoveResult{}, err
	}
	if err := acquire(ctx, r); err != nil {
		return BatchRemoveResult{}, err
	}
	defer release(r)
	defer func() {
		refreshCtx, refreshCancel := context.WithTimeout(context.Background(), readTimeout)
		defer refreshCancel()
		s.readRepository(refreshCtx, r)
	}()

	results := make([]BatchRemoveItemResult, len(c.batch))
	modes := make([]string, len(c.batch))
	for i, target := range c.batch {
		branch := ""
		if target.state.tree.Branch != nil {
			branch = *target.state.tree.Branch
		}
		results[i] = BatchRemoveItemResult{
			WorktreeID: target.state.tree.ID,
			Path:       target.state.tree.Path,
			Branch:     branch,
		}
		if !target.state.supported || (len(target.state.blockers) > 0 && !req.IncludeForce) {
			results[i].Status = "skipped"
			continue
		}
		if len(target.state.blockers) > 0 {
			modes[i] = "force"
		} else {
			modes[i] = "safe"
		}
	}

	reserved := make([]string, 0, len(c.batch))
	defer func() {
		for _, path := range reserved {
			s.unreserve(path)
		}
	}()
	for i, target := range c.batch {
		if modes[i] == "" {
			continue
		}
		path := target.state.tree.Path
		if err := s.reserve(ctx, path); err != nil {
			results[i].Status = "failed"
			results[i].Error = err.Error()
			modes[i] = ""
			continue
		}
		reserved = append(reserved, path)
	}

	for i, target := range c.batch {
		if modes[i] == "" {
			continue
		}
		if err := s.executeLockedRemoval(ctx, r, repositoryID, target.state, target.commit, modes[i]); err != nil {
			results[i].Status = "failed"
			results[i].Error = err.Error()
			continue
		}
		results[i].Status = "removed"
	}
	return BatchRemoveResult{Results: results}, nil
}
