package workspace

import (
	"context"
	"errors"
	"io/fs"
	"path/filepath"
	"time"
)

const (
	// usageBudget and usageEntries bound a worktree size measurement, which
	// the user asks for explicitly; the release preview's ignored-size walk
	// keeps its smaller bounds.
	usageBudget  = 8 * time.Second
	usageEntries = 1_000_000
)

// Usage is the disk space of a worktree's directory: bytes of regular files
// (apparent size, symbolic links neither followed nor counted) and the number
// of entries, directories included. Complete is false when the walk ran out
// of budget or met an unreadable directory, so both are lower bounds.
type Usage struct {
	Bytes    int64
	Entries  int
	Complete bool
}

// MeasureWorktree walks a worktree's directory and counts its size. The
// repository gate is held only to resolve the worktree, so a long walk never
// blocks other worktree operations.
func (s *Service) MeasureWorktree(ctx context.Context, repositoryID, worktreeID string) (Usage, error) {
	tree, err := s.measuredTree(ctx, repositoryID, worktreeID)
	if err != nil {
		return Usage{}, err
	}
	if !tree.Available {
		return Usage{}, problemf(ErrConflict, "工作树目录缺失或不可验证，无法计算容量")
	}
	return measureDir(ctx, tree.Path)
}

func (s *Service) measuredTree(ctx context.Context, repositoryID, worktreeID string) (Worktree, error) {
	ctx, cancel := context.WithTimeout(ctx, readTimeout)
	defer cancel()
	r, err := s.repository(repositoryID)
	if err != nil {
		return Worktree{}, err
	}
	if err := acquire(ctx, r); err != nil {
		return Worktree{}, err
	}
	defer release(r)
	return s.currentTree(ctx, r, worktreeID)
}

func measureDir(ctx context.Context, root string) (Usage, error) {
	deadline := time.Now().Add(usageBudget)
	usage := Usage{Complete: true}
	err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, walkErr error) error {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		if walkErr != nil {
			if path == root {
				return walkErr
			}
			// An unreadable entry is skipped; the total is then a lower bound.
			usage.Complete = false
			if entry != nil && entry.IsDir() {
				return fs.SkipDir
			}
			return nil
		}
		usage.Entries++
		if usage.Entries > usageEntries || time.Now().After(deadline) {
			usage.Complete = false
			usage.Entries--
			return fs.SkipAll
		}
		info, err := entry.Info()
		if err != nil {
			usage.Complete = false
			return nil
		}
		if info.Mode().IsRegular() {
			usage.Bytes += info.Size()
		}
		return nil
	})
	if err != nil {
		if ctx.Err() != nil {
			return Usage{}, ctx.Err()
		}
		if errors.Is(err, fs.ErrNotExist) {
			return Usage{}, problemf(ErrConflict, "工作树目录缺失，无法计算容量")
		}
		return Usage{}, problemf(ErrUnavailable, "无法读取工作树目录: %v", err)
	}
	return usage, nil
}
