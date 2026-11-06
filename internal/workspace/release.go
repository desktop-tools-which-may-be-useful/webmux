package workspace

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io/fs"
	"log/slog"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"github.com/rainchan/webmux/internal/hybrid"
)

const (
	// releasedRefPrefix holds the private refs that keep released commits
	// reachable; Git neither fetches nor pushes them by default.
	releasedRefPrefix = "refs/webmux/released/"
	// maxIgnoredListed bounds the ignored entries a release preview lists.
	maxIgnoredListed = 50
	// sizeBudget and sizeEntries bound the ignored-size walk.
	sizeBudget  = 2 * time.Second
	sizeEntries = 200_000
)

// IgnoredEntry is one top-level ignored path of a worktree, relative to it;
// directories end with "/". Bytes is nil when its size was not fully
// counted.
type IgnoredEntry struct {
	Path  string
	Bytes *int64
}

// ReleasePreview describes what releasing a worktree's directory keeps and
// deletes. Non-empty Blockers refuse the release.
type ReleasePreview struct {
	ConfirmationID string
	ExpiresAt      time.Time
	Worktree       Worktree
	// Commit is the full OID the release keeps, empty when HEAD does not
	// resolve (then a blocker says so).
	Commit string
	// Archived are the records the release keeps and marks released.
	Archived []hybrid.Record
	// Push is the push check of Commit; PushUnknown when it is empty.
	Push        PushCheck
	Ignored     []IgnoredEntry
	IgnoredMore int
	// IgnoredBytes is the total counted, nil when nothing was counted.
	IgnoredBytes *int64
	// SizeComplete is false when the walk ran out of budget or failed.
	SizeComplete bool
	Blockers     []string
}

// ReleaseRequest executes a release preview.
type ReleaseRequest struct {
	ConfirmationID string
}

// ReleaseResult names the release and the records it marked.
type ReleaseResult struct {
	ReleaseID string
	RecordIDs []string
}

func releasedRef(id string) string { return releasedRefPrefix + id }

func recordIDs(list []hybrid.Record) []string {
	ids := make([]string, len(list))
	for i, rec := range list {
		ids[i] = rec.ID
	}
	slices.Sort(ids)
	return ids
}

// headCommit resolves tree's HEAD to a full commit OID, empty when it does
// not resolve.
func (s *Service) headCommit(ctx context.Context, tree Worktree) string {
	if !tree.Available {
		return ""
	}
	out, err := s.git(ctx, tree.Path, "rev-parse", "--verify", "--end-of-options", "HEAD^{commit}")
	if err != nil {
		return ""
	}
	oid := outputLine(out)
	if !isObjectID(oid) {
		return ""
	}
	return oid
}

// pinRecords keeps commit reachable under a new private ref and marks the
// archived records ids as released from tree under it, returning the
// release ID. The caller holds r's gate, which keeps the sweep from
// dropping the ref before the mark is written. A failed mark leaves the
// ref: it may have reached the disk, and the sweep drops the ref only when
// no loaded record uses it.
func (s *Service) pinRecords(ctx context.Context, r *repositoryState, repositoryID string, tree Worktree, commit string, ids []string) (string, error) {
	id, err := randomID()
	if err != nil {
		return "", err
	}
	// The empty old value makes Git refuse an existing ref.
	if _, err := s.git(ctx, r.entry.AnchorPath, "update-ref", releasedRef(id), commit, ""); err != nil {
		return "", err
	}
	rel := hybrid.Release{ID: id, RepositoryID: repositoryID, Path: tree.Path, Commit: commit, ReleasedAt: time.Now().UTC()}
	if tree.Branch != nil {
		rel.Branch = *tree.Branch
	}
	if err := s.hybrids.MarkReleased(ctx, ids, rel); err != nil {
		return "", err
	}
	return id, nil
}

// releaseBlockers are the removal blockers without ignored files, plus a
// commit that must resolve and at least one archived record.
func releaseBlockers(state removalState, commit string, records []hybrid.Record) []string {
	out := state.messages(blockIgnored)
	if commit == "" {
		out = append(out, "HEAD 无法解析为提交，不能保存释放点")
	}
	if len(records) == 0 {
		out = append(out, "工作树中没有已归档的聊天记录，无需释放")
	}
	return out
}

// PreviewRelease previews deleting a linked worktree's directory while
// keeping its archived records and commit, and stores a confirmation for
// ReleaseWorktree.
func (s *Service) PreviewRelease(ctx context.Context, repositoryID, worktreeID string) (ReleasePreview, error) {
	ctx, cancel := context.WithTimeout(ctx, readTimeout)
	defer cancel()
	r, err := s.repository(repositoryID)
	if err != nil {
		return ReleasePreview{}, err
	}
	if err := acquire(ctx, r); err != nil {
		return ReleasePreview{}, err
	}
	defer release(r)
	state, err := s.removalState(ctx, r, worktreeID)
	if err != nil {
		return ReleasePreview{}, err
	}
	commit := s.headCommit(ctx, state.tree)
	records := s.targetRecords(state.tree.Path)
	preview := ReleasePreview{
		Worktree: state.tree, Commit: commit, Archived: records, Ignored: []IgnoredEntry{},
		Blockers: releaseBlockers(state, commit, records), Push: s.checkPushed(ctx, r, commit),
	}
	if state.tree.Available {
		ignored, err := s.ignoredEntries(ctx, state.tree.Path)
		if err != nil {
			preview.Blockers = append(preview.Blockers, "无法列出忽略文件: "+err.Error())
		} else {
			measureIgnored(ctx, state.tree.Path, ignored, &preview)
		}
	}
	if err := ctx.Err(); err != nil {
		state.identity.close()
		return ReleasePreview{}, err
	}
	preview.ConfirmationID, preview.ExpiresAt, err = s.confirm(confirmation{
		purpose: purposeRelease, repositoryID: repositoryID, worktreeID: worktreeID, state: state,
		records: recordIDs(records), commit: commit,
	})
	if err != nil {
		return ReleasePreview{}, err
	}
	return preview, nil
}

// ignoredEntries lists the top-level ignored paths of the worktree at path,
// relative to it; Git collapses wholly ignored directories ("dir/").
func (s *Service) ignoredEntries(ctx context.Context, path string) ([]string, error) {
	data, err := s.git(ctx, path, "status", "--porcelain=v1", "-z", "--ignored=traditional", "--untracked-files=normal")
	if err != nil {
		return nil, err
	}
	return parseIgnored(data)
}

func parseIgnored(data []byte) ([]string, error) {
	out := []string{}
	if len(data) == 0 {
		return out, nil
	}
	if data[len(data)-1] != 0 {
		return nil, errors.New("状态输出不是 NUL 格式（来自 Git）")
	}
	records := bytes.Split(data[:len(data)-1], []byte{0})
	for i := 0; i < len(records); i++ {
		record := records[i]
		if len(record) < 4 || record[2] != ' ' {
			return nil, errors.New("状态记录无效（来自 Git）")
		}
		if string(record[:2]) == "!!" {
			out = append(out, string(record[3:]))
			continue
		}
		// A rename or copy carries its source path as the next record.
		if record[0] == 'R' || record[0] == 'C' || record[1] == 'R' || record[1] == 'C' {
			i++
		}
	}
	return out, nil
}

// measureIgnored lists at most maxIgnoredListed entries and counts their
// sizes with lstat, never following symbolic links, within sizeBudget and
// sizeEntries over all entries.
func measureIgnored(ctx context.Context, root string, ignored []string, preview *ReleasePreview) {
	deadline := time.Now().Add(sizeBudget)
	entries := 0
	complete := true
	var total int64
	counted := false
	for i, rel := range ignored {
		var size int64
		done := complete
		if done {
			err := filepath.WalkDir(filepath.Join(root, filepath.FromSlash(strings.TrimSuffix(rel, "/"))), func(_ string, entry fs.DirEntry, walkErr error) error {
				if walkErr != nil {
					return walkErr
				}
				if ctx.Err() != nil {
					return ctx.Err()
				}
				entries++
				if entries > sizeEntries || time.Now().After(deadline) {
					return errBudget
				}
				info, err := entry.Info()
				if err != nil {
					return err
				}
				if info.Mode().IsRegular() {
					size += info.Size()
				}
				counted = true
				return nil
			})
			if err != nil {
				done, complete = false, false
			}
			total += size
		}
		if i < maxIgnoredListed {
			item := IgnoredEntry{Path: rel}
			if done {
				n := size
				item.Bytes = &n
			}
			preview.Ignored = append(preview.Ignored, item)
		}
	}
	preview.IgnoredMore = max(0, len(ignored)-maxIgnoredListed)
	preview.SizeComplete = complete
	if counted {
		preview.IgnoredBytes = &total
	}
}

var errBudget = errors.New("size budget exhausted")

// ReleaseWorktree deletes a released worktree's directory after its
// confirmation: it pins the commit with a private ref, marks the archived
// records released, then lets Git remove the worktree without force, so
// uncommitted or untracked files still refuse. A failed removal clears the
// mark and the ref again. The records are kept.
func (s *Service) ReleaseWorktree(ctx context.Context, repositoryID, worktreeID string, req ReleaseRequest) (ReleaseResult, error) {
	ctx, cancel := context.WithTimeout(ctx, writeTimeout)
	defer cancel()
	c, ok := s.takeConfirmation(req.ConfirmationID, purposeRelease, repositoryID, worktreeID)
	if !ok {
		return ReleaseResult{}, problemf(ErrConflict, "释放确认已失效，请重新预览")
	}
	defer c.state.identity.close()
	if s.hybrids == nil {
		return ReleaseResult{}, problemf(ErrUnavailable, "混合会话服务不可用，不能释放")
	}
	r, err := s.repository(repositoryID)
	if err != nil {
		return ReleaseResult{}, err
	}
	if err := acquire(ctx, r); err != nil {
		return ReleaseResult{}, err
	}
	defer release(r)
	path := c.state.tree.Path
	if err := s.reserve(ctx, path); err != nil {
		return ReleaseResult{}, err
	}
	defer s.unreserve(path)
	defer func() {
		refreshCtx, refreshCancel := context.WithTimeout(context.Background(), readTimeout)
		defer refreshCancel()
		s.readRepository(refreshCtx, r)
		s.sweepReleased(refreshCtx, r)
	}()
	current, err := s.removalState(ctx, r, worktreeID)
	if err != nil {
		return ReleaseResult{}, err
	}
	defer current.identity.close()
	if !sameTarget(c.state, current) || !sameSessions(c.state.sessions, current.sessions) {
		return ReleaseResult{}, problemf(ErrConflict, "工作树身份、分支、锁或会话集合已变化，请重新预览")
	}
	commit := s.headCommit(ctx, current.tree)
	records := s.targetRecords(path)
	ids := recordIDs(records)
	if commit != c.commit || !slices.Equal(ids, c.records) {
		return ReleaseResult{}, problemf(ErrConflict, "提交或已归档记录已变化，请重新预览")
	}
	if blockers := releaseBlockers(current, commit, records); len(blockers) != 0 {
		return ReleaseResult{}, problemf(ErrConflict, "释放被阻止: %v", blockers)
	}
	if err := ctx.Err(); err != nil {
		return ReleaseResult{}, err
	}
	id, err := s.pinRecords(ctx, r, repositoryID, current.tree, commit, ids)
	if err != nil {
		return ReleaseResult{}, fmt.Errorf("无法保存释放点，工作树未释放: %w", err)
	}
	// Cleanup runs even when the request was cancelled midway.
	cleanupCtx, cleanupCancel := context.WithTimeout(context.WithoutCancel(ctx), writeTimeout)
	defer cleanupCancel()
	if _, err := s.git(ctx, r.entry.AnchorPath, "worktree", "remove", "--", path); err != nil {
		category := errorKind(err)
		if category == ErrInternal {
			category = ErrConflict
		}
		// Git deletes the directory past the point of no return even when
		// a file inside cannot go, then drops its registration. Only a
		// worktree still registered at path was left untouched; otherwise
		// the mark and the ref stay so the commit remains reachable.
		if !s.stillRegistered(cleanupCtx, r, path) {
			slog.Warn("release removal failed after git dropped the worktree", "release", id, "error", err)
			return ReleaseResult{}, problemf(category, "%v；工作树可能已部分删除，释放点已保留，请清理原路径后重建", err)
		}
		if _, clearErr := s.hybrids.ClearReleased(cleanupCtx, id); clearErr != nil {
			slog.Warn("release rollback failed to clear records", "release", id, "error", clearErr)
		} else {
			s.dropRef(cleanupCtx, r, id)
		}
		return ReleaseResult{}, problemf(category, "%v；工作树未释放，请刷新实际状态", err)
	}
	slog.Info("worktree released", "release", id, "records", len(ids))
	return ReleaseResult{ReleaseID: id, RecordIDs: ids}, nil
}

// stillRegistered reports whether Git still lists a worktree at path. A
// failed read counts as not registered, which keeps a release's mark and
// ref rather than risk losing its commit.
func (s *Service) stillRegistered(ctx context.Context, r *repositoryState, path string) bool {
	repository, err := s.current(ctx, r)
	if err != nil {
		return false
	}
	return slices.ContainsFunc(repository.Worktrees, func(tree Worktree) bool { return tree.Path == path })
}

// dropRef deletes release id's private ref; a failure is only logged and
// left to the sweep.
func (s *Service) dropRef(ctx context.Context, r *repositoryState, id string) {
	if _, err := s.git(ctx, r.entry.AnchorPath, "update-ref", "-d", releasedRef(id)); err != nil {
		slog.Warn("released ref removal failed", "release", id, "error", err)
	}
}

// DropRelease deletes rel's private ref once no record references the
// release any more. A repository that is no longer registered keeps its
// ref; the next sweep after it is registered again removes it.
func (s *Service) DropRelease(ctx context.Context, rel hybrid.Release) error {
	ctx, cancel := context.WithTimeout(ctx, writeTimeout)
	defer cancel()
	r := s.releaseRepository(ctx, rel)
	if r == nil {
		return nil
	}
	if err := acquire(ctx, r); err != nil {
		return err
	}
	defer release(r)
	if s.hybrids == nil {
		return nil
	}
	list, err := s.hybrids.Records(false)
	if err != nil {
		return err
	}
	if slices.ContainsFunc(list, func(rec hybrid.Record) bool { return rec.Released != nil && rec.Released.ID == rel.ID }) {
		return nil
	}
	if _, err := s.git(ctx, r.entry.AnchorPath, "update-ref", "-d", releasedRef(rel.ID)); err != nil {
		return err
	}
	return nil
}

// releaseRepository finds the registered repository holding rel's private
// ref: the recorded repository, else one registered again under a new ID.
// Nil when none has it.
func (s *Service) releaseRepository(ctx context.Context, rel hybrid.Release) *repositoryState {
	if r, err := s.repository(rel.RepositoryID); err == nil {
		return r
	}
	s.mu.Lock()
	entries := append([]*repositoryState(nil), s.entries...)
	s.mu.Unlock()
	for _, r := range entries {
		if _, err := s.git(ctx, r.entry.AnchorPath, "rev-parse", "--verify", "--quiet", releasedRef(rel.ID)); err == nil {
			return r
		}
	}
	return nil
}

// sweepReleased deletes r's private release refs that no record references,
// e.g. records evicted by the record limit. It does nothing unless the
// records loaded: an unreadable store cannot tell which refs are in use.
// The caller holds r's gate. It reports whether the sweep ran.
func (s *Service) sweepReleased(ctx context.Context, r *repositoryState) bool {
	if s.hybrids == nil || s.gitPath == "" {
		return false
	}
	list, err := s.hybrids.Records(false)
	if err != nil {
		return false
	}
	used := map[string]bool{}
	for _, rec := range list {
		if rec.Released != nil {
			used[rec.Released.ID] = true
		}
	}
	out, err := s.git(ctx, r.entry.AnchorPath, "for-each-ref", "--format=%(refname)", releasedRefPrefix)
	if err != nil {
		return false
	}
	for _, line := range strings.Split(strings.TrimSpace(string(out)), "\n") {
		id, ok := strings.CutPrefix(line, releasedRefPrefix)
		if !ok || id == "" || used[id] {
			continue
		}
		if _, err := s.git(ctx, r.entry.AnchorPath, "update-ref", "-d", line); err != nil {
			slog.Warn("orphan released ref removal failed", "ref", line, "error", err)
		}
	}
	return true
}

// sweepOnce runs the first successful sweep of r after the records loaded;
// the caller holds r's gate, which also guards r.swept.
func (s *Service) sweepOnce(ctx context.Context, r *repositoryState) {
	if !r.swept && s.sweepReleased(ctx, r) {
		r.swept = true
	}
}
