package workspace

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"github.com/rainchan/webmux/internal/hybrid"
	"github.com/rainchan/webmux/internal/session"
)

type identity struct {
	root            os.FileInfo
	admin           os.FileInfo
	common          os.FileInfo
	gitFile         os.FileInfo
	adminPath       string
	gitContents     string
	prunableMissing bool
	// Holding the objects prevents inode reuse from making a removed/recreated
	// checkout compare equal to an earlier confirmation.
	handles []*os.File
}

func (id *identity) hold(path string) (os.FileInfo, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	id.handles = append(id.handles, file)
	return file.Stat()
}
func (id *identity) close() {
	for _, file := range id.handles {
		_ = file.Close()
	}
	id.handles = nil
}

// blockerKind tells removal and release which blockers they share.
type blockerKind int

const (
	blockUnsupported blockerKind = iota
	blockDirty
	blockUntracked
	blockSessions
)

type blocker struct {
	kind    blockerKind
	message string
}

type removalState struct {
	tree     Worktree
	identity identity
	sessions []session.Info
	blockers []blocker
	// warnings do not block removal or release; the preview shows them.
	warnings  []string
	supported bool
}

// messages lists the blocker messages except those of kinds skip; never nil.
func (st removalState) messages(skip ...blockerKind) []string {
	out := []string{}
	for _, b := range st.blockers {
		if !slices.Contains(skip, b.kind) {
			out = append(out, b.message)
		}
	}
	return out
}

// purpose keeps the confirmations of different operations apart: one
// cannot execute another.
type purpose string

const (
	purposeRemove      purpose = "remove"
	purposeRelease     purpose = "release"
	purposeBatchRemove purpose = "batch-remove"
	purposeSwitch      purpose = "switch"
)

type batchTarget struct {
	state  removalState
	commit string
}

type confirmation struct {
	purpose      purpose
	repositoryID string
	worktreeID   string
	expires      time.Time
	state        removalState
	batch        []batchTarget
	// records are the archived record IDs a release preview showed.
	records []string
	// commit is the commit the preview resolved, empty when HEAD does not
	// resolve.
	commit string
	// switching is what a switch preview bound.
	switching *switchState
	timer     *time.Timer
}

func (c *confirmation) closeHandles() {
	c.state.identity.close()
	for i := range c.batch {
		c.batch[i].state.identity.close()
	}
}

func (s *Service) identify(ctx context.Context, r *repositoryState, tree Worktree) (id identity, err error) {
	defer func() {
		if err != nil {
			id.close()
		}
	}()
	id.root, err = id.hold(tree.Path)
	if err != nil {
		return id, err
	}
	path, err := canonical(tree.Path)
	if err != nil || path != tree.Path {
		return id, fmt.Errorf("工作树规范路径身份已改变: %v", err)
	}
	data, err := s.git(ctx, tree.Path, "rev-parse", "--absolute-git-dir")
	if err != nil {
		return id, err
	}
	id.adminPath, err = canonical(outputLine(data))
	if err != nil {
		return id, err
	}
	if !within(r.entry.CommonDir, id.adminPath) || id.adminPath == r.entry.CommonDir {
		return id, errors.New("linked worktree 管理目录身份无效")
	}
	id.admin, err = id.hold(id.adminPath)
	if err != nil {
		return id, err
	}
	id.common, err = id.hold(r.entry.CommonDir)
	if err != nil {
		return id, err
	}
	file := filepath.Join(tree.Path, ".git")
	id.gitFile, err = os.Lstat(file)
	if err != nil {
		return id, err
	}
	if !id.gitFile.Mode().IsRegular() {
		return id, errors.New("linked worktree 的 .git 不是普通文件")
	}
	held, err := id.hold(file)
	if err != nil {
		return id, err
	}
	if !os.SameFile(id.gitFile, held) {
		return id, errors.New(".git 文件身份在读取期间改变")
	}
	contents, err := os.ReadFile(file)
	if err != nil {
		return id, err
	}
	id.gitContents = string(contents)
	return id, nil
}
func findWorktreeAdmin(commonDir, targetPath string) (string, string, error) {
	worktreesDir := filepath.Join(commonDir, "worktrees")
	entries, err := os.ReadDir(worktreesDir)
	if err != nil {
		return "", "", err
	}
	var matchedPath, matchedContents string
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		candidateDir := filepath.Join(worktreesDir, entry.Name())
		data, readErr := os.ReadFile(filepath.Join(candidateDir, "gitdir"))
		if readErr != nil {
			continue
		}
		raw := strings.TrimRight(string(data), "\r\n")
		if raw == "" {
			continue
		}
		if !filepath.IsAbs(raw) {
			raw = filepath.Join(candidateDir, raw)
		}
		wtPath := filepath.Dir(filepath.Clean(raw))
		if resolved, resolveErr := canonicalTarget(wtPath); resolveErr == nil {
			wtPath = resolved
		}
		if wtPath != targetPath {
			continue
		}
		adminPath, canonErr := canonical(candidateDir)
		if canonErr != nil {
			return "", "", canonErr
		}
		if !within(commonDir, adminPath) || adminPath == commonDir {
			return "", "", errors.New("linked worktree 管理目录身份无效")
		}
		if matchedPath != "" {
			return "", "", errors.New("存在多个匹配的 worktree 管理目录")
		}
		matchedPath = adminPath
		matchedContents = string(data)
	}
	if matchedPath == "" {
		return "", "", errors.New("未找到对应的 worktree 管理目录")
	}
	return matchedPath, matchedContents, nil
}

func (s *Service) identifyPrunable(r *repositoryState, tree Worktree) (id identity, err error) {
	defer func() {
		if err != nil {
			id.close()
		}
	}()
	if _, statErr := os.Lstat(tree.Path); !errors.Is(statErr, os.ErrNotExist) {
		return id, fmt.Errorf("待清理工作树路径仍存在或不可验证: %v", statErr)
	}
	resolved, err := canonicalTarget(tree.Path)
	if err != nil || resolved != tree.Path {
		return id, fmt.Errorf("工作树规范路径身份已改变: %v", err)
	}
	id.common, err = id.hold(r.entry.CommonDir)
	if err != nil {
		return id, err
	}
	adminPath, gitdirContents, err := findWorktreeAdmin(r.entry.CommonDir, tree.Path)
	if err != nil {
		return id, err
	}
	id.adminPath = adminPath
	id.admin, err = id.hold(adminPath)
	if err != nil {
		return id, err
	}
	id.gitContents = gitdirContents
	id.prunableMissing = true
	return id, nil
}

func sameIdentity(a, b identity) bool {
	if a.prunableMissing || b.prunableMissing {
		return a.prunableMissing && b.prunableMissing &&
			a.admin != nil && b.admin != nil && a.common != nil && b.common != nil &&
			os.SameFile(a.admin, b.admin) && os.SameFile(a.common, b.common) &&
			a.adminPath == b.adminPath && a.gitContents == b.gitContents
	}
	return a.root != nil && b.root != nil && a.admin != nil && b.admin != nil && a.common != nil && b.common != nil && a.gitFile != nil && b.gitFile != nil &&
		os.SameFile(a.root, b.root) && os.SameFile(a.admin, b.admin) && os.SameFile(a.common, b.common) && os.SameFile(a.gitFile, b.gitFile) && a.adminPath == b.adminPath && a.gitContents == b.gitContents
}
func sameText(a, b *string) bool {
	if a == nil || b == nil {
		return a == b
	}
	return *a == *b
}
func sameTarget(a, b removalState) bool {
	return sameIdentity(a.identity, b.identity) && a.supported == b.supported &&
		a.tree.ID == b.tree.ID && a.tree.Path == b.tree.Path && a.tree.Kind == b.tree.Kind &&
		sameText(a.tree.Branch, b.tree.Branch) && sameText(a.tree.Head, b.tree.Head) && a.tree.Detached == b.tree.Detached &&
		a.tree.Locked == b.tree.Locked && a.tree.LockReason == b.tree.LockReason && a.tree.Prunable == b.tree.Prunable && a.tree.Available == b.tree.Available
}
func (s *Service) removalState(ctx context.Context, r *repositoryState, worktreeID string) (removalState, error) {
	tree, err := s.currentTree(ctx, r, worktreeID)
	if err != nil {
		return removalState{}, err
	}
	return s.removalStateFromTree(ctx, r, tree)
}

func (s *Service) removalStateFromTree(ctx context.Context, r *repositoryState, tree Worktree) (removalState, error) {
	var err error
	state := removalState{tree: tree, sessions: s.targetSessions(tree.Path), blockers: []blocker{}, supported: true}
	block := func(message string) {
		state.supported = false
		state.blockers = append(state.blockers, blocker{kind: blockUnsupported, message: message})
	}
	add := func(kind blockerKind, message string) {
		state.blockers = append(state.blockers, blocker{kind: kind, message: message})
	}
	isMissingPrunable := false
	if tree.Kind == "linked" && !tree.Locked && tree.Prunable && !tree.Available {
		if _, statErr := os.Lstat(tree.Path); errors.Is(statErr, os.ErrNotExist) {
			isMissingPrunable = true
		}
	}
	if tree.Kind != "linked" {
		block("主工作区和 bare 仓库不能在页面删除")
	}
	if tree.Locked {
		block("工作树已锁定，请先在终端解锁: " + tree.LockReason)
	}
	if isMissingPrunable {
		if state.supported {
			state.identity, err = s.identifyPrunable(r, tree)
			if err != nil {
				block("无法验证待清理工作树身份: " + err.Error())
			}
		}
	} else {
		if !tree.Available || tree.Prunable {
			block("工作树目录缺失或不可验证，请在终端处理")
		}
		if tree.Dirty == nil || tree.Error != "" {
			block("工作树状态未知，不能确认安全删除: " + tree.Error)
		}
		if state.supported {
			state.identity, err = s.identify(ctx, r, tree)
			if err != nil {
				block("无法验证工作树身份: " + err.Error())
			}
			if err := s.unsupportedContents(ctx, tree); err != nil {
				block(err.Error())
			}
		}
	}
	if !state.supported {
		state.identity.close()
	}
	if tree.Dirty != nil && *tree.Dirty {
		add(blockDirty, "存在未提交修改")
	}
	if tree.HasUntracked {
		add(blockUntracked, "存在未跟踪文件")
	}
	if tree.HasIgnored {
		state.warnings = append(state.warnings, "存在忽略文件（可能包含 .env 或构建产物），删除后将一并丢失")
	}
	for _, info := range state.sessions {
		if info.Status != session.StatusExited {
			add(blockSessions, "存在运行中的 WebMux 会话")
			break
		}
	}
	if ctx.Err() != nil {
		state.identity.close()
		return state, ctx.Err()
	}
	return state, nil
}
func (s *Service) PreviewRemoval(ctx context.Context, repositoryID, worktreeID string) (Preview, error) {
	ctx, cancel := context.WithTimeout(ctx, readTimeout)
	defer cancel()
	r, err := s.repository(repositoryID)
	if err != nil {
		return Preview{}, err
	}
	if err := acquire(ctx, r); err != nil {
		return Preview{}, err
	}
	defer release(r)
	state, err := s.removalState(ctx, r, worktreeID)
	if err != nil {
		return Preview{}, err
	}
	commit := s.headCommit(ctx, r, state.tree)
	records := s.targetRecords(state.tree.Path)
	check := s.checkPushed(ctx, r, commit)
	c := confirmation{purpose: purposeRemove, repositoryID: repositoryID, worktreeID: worktreeID, state: state, commit: commit}
	id, expires, err := s.confirm(c)
	if err != nil {
		return Preview{}, err
	}
	return Preview{
		ConfirmationID: id, ExpiresAt: expires, Worktree: state.tree, Sessions: state.sessions, Archived: records, Commit: commit, Push: check,
		Blockers: state.messages(), Warnings: append([]string{}, state.warnings...), ForceAllowed: state.supported,
	}, nil
}

// confirm stores c under a new confirmation ID until it expires; it owns
// c's identity handles from now on, also on failure.
func (s *Service) confirm(c confirmation) (string, time.Time, error) {
	id, err := randomID()
	if err != nil {
		c.closeHandles()
		return "", time.Time{}, err
	}
	now := time.Now()
	c.expires = now.Add(confirmationLifetime)
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := s.requireRegistryLocked(); err != nil {
		c.closeHandles()
		return "", time.Time{}, err
	}
	s.coord.Lock()
	defer s.coord.Unlock()
	for key, old := range s.confirmations {
		if !now.Before(old.expires) {
			old.timer.Stop()
			old.closeHandles()
			delete(s.confirmations, key)
		}
	}
	c.timer = time.AfterFunc(confirmationLifetime, func() {
		s.coord.Lock()
		if expired, exists := s.confirmations[id]; exists {
			delete(s.confirmations, id)
			expired.closeHandles()
		}
		s.coord.Unlock()
	})
	s.confirmations[id] = c
	return id, c.expires, nil
}

// takeConfirmation removes confirmation id and returns it when it is
// still valid for purpose p on the worktree. The caller closes the
// returned confirmation's identity handles.
func (s *Service) takeConfirmation(id string, p purpose, repositoryID, worktreeID string) (confirmation, bool) {
	s.coord.Lock()
	c, ok := s.confirmations[id]
	if ok && c.purpose == p {
		delete(s.confirmations, id)
	}
	s.coord.Unlock()
	if !ok || c.purpose != p {
		return confirmation{}, false
	}
	c.timer.Stop()
	if !time.Now().Before(c.expires) || c.repositoryID != repositoryID || c.worktreeID != worktreeID {
		c.closeHandles()
		return confirmation{}, false
	}
	return c, true
}

// RemoveWorktree removes a linked worktree after its confirmation. The
// closed and interrupted hybrid records under the removed directory stay,
// marked released at the commit the confirmation resolved, so they can be
// rebuilt and resumed; without such a commit they stay as they were.
func (s *Service) RemoveWorktree(ctx context.Context, repositoryID, worktreeID string, req RemoveRequest) error {
	return s.removeWorktree(ctx, repositoryID, worktreeID, req)
}

// targetRecords lists the closed and interrupted hybrid records whose
// directory lies in path, most recently updated first. Released records
// are skipped: their worktree is gone, so whatever now occupies path is not
// theirs, and removing or releasing it must leave them restorable.
func (s *Service) targetRecords(path string) []hybrid.Record {
	result := make([]hybrid.Record, 0)
	if s.hybrids == nil {
		return result
	}
	list, err := s.hybrids.Records(true)
	if err != nil {
		return result
	}
	for _, rec := range list {
		if rec.Released != nil {
			continue
		}
		cwd, err := canonicalTarget(rec.Cwd)
		if err != nil {
			cwd = filepath.Clean(rec.Cwd)
		}
		if within(path, cwd) || within(path, rec.Cwd) {
			result = append(result, rec)
		}
	}
	return result
}

// retainRecords marks the closed and interrupted hybrid records whose
// directory lies in removed worktree tree as released at commit, including
// those of the sessions the removal closed (closedHybrids), whose records
// are awaited first. Without a commit nothing can be pinned, so the
// records stay unchanged and can only be viewed or deleted. The caller
// holds r's gate. The worktree is gone whatever the client does now, so a
// cancelled request still retains. A failure is logged: the records then
// stay as they were.
func (s *Service) retainRecords(ctx context.Context, r *repositoryState, repositoryID string, tree Worktree, commit string, closedHybrids []string) {
	ctx, cancel := context.WithTimeout(context.WithoutCancel(ctx), writeTimeout)
	defer cancel()
	s.awaitClosed(ctx, closedHybrids)
	records := s.targetRecords(tree.Path)
	if len(records) == 0 {
		return
	}
	if commit == "" {
		slog.Info("archived records kept unreleased: HEAD did not resolve", "records", len(records))
		return
	}
	id, err := s.pinRecords(ctx, r, repositoryID, tree, commit, recordIDs(records))
	if err != nil {
		slog.Warn("archived records could not be marked released after removal", "records", len(records), "error", err)
		return
	}
	slog.Info("worktree removed, archived records released", "release", id, "records", len(records))
}

// closeRecordWait bounds the wait for a closed session's record.
const closeRecordWait = 5 * time.Second

// awaitClosed waits, within closeRecordWait, until each hybrid session in
// ids has recorded its close; a record that never shows is left out.
func (s *Service) awaitClosed(ctx context.Context, ids []string) {
	deadline := time.Now().Add(closeRecordWait)
	for _, id := range ids {
		for time.Now().Before(deadline) && ctx.Err() == nil {
			if rec, err := s.hybrids.Record(id); err == nil && rec.State != hybrid.StateActive {
				break
			}
			time.Sleep(20 * time.Millisecond)
		}
	}
}

// removeWorktree performs the removal and retains the archived records.
func (s *Service) removeWorktree(ctx context.Context, repositoryID, worktreeID string, req RemoveRequest) error {
	ctx, cancel := context.WithTimeout(ctx, writeTimeout)
	defer cancel()
	if req.Mode != "safe" && req.Mode != "force" {
		return problemf(ErrInvalid, "删除模式必须为 safe 或 force")
	}
	confirmation, ok := s.takeConfirmation(req.ConfirmationID, purposeRemove, repositoryID, worktreeID)
	if !ok {
		return problemf(ErrConflict, "删除确认已失效，请重新预览")
	}
	defer confirmation.closeHandles()
	if req.Mode == "force" && (req.ConfirmPath != confirmation.state.tree.Path || !req.CloseSessions) {
		return problemf(ErrInvalid, "强制删除必须输入完整目标路径并明确授权关闭会话")
	}
	r, err := s.repository(repositoryID)
	if err != nil {
		return err
	}
	if err := acquire(ctx, r); err != nil {
		return err
	}
	defer release(r)
	path := confirmation.state.tree.Path
	if err := s.reserve(ctx, path); err != nil {
		return err
	}
	defer s.unreserve(path)
	// Every execution path releases the reservation and re-reads real Git state,
	// including cancellation after Git or session cleanup partially took effect.
	defer func() {
		refreshCtx, refreshCancel := context.WithTimeout(context.Background(), readTimeout)
		defer refreshCancel()
		s.readRepository(refreshCtx, r)
	}()
	return s.executeLockedRemoval(ctx, r, repositoryID, confirmation.state, confirmation.commit, req.Mode)
}

func (s *Service) executeLockedRemoval(ctx context.Context, r *repositoryState, repositoryID string, confirmed removalState, confirmedCommit, mode string) error {
	worktreeID := confirmed.tree.ID
	path := confirmed.tree.Path
	current, err := s.removalState(ctx, r, worktreeID)
	if err != nil {
		return err
	}
	defer current.identity.close()
	if !current.supported {
		return problemf(ErrConflict, "不支持删除此工作树: %v", current.messages())
	}
	if !sameTarget(confirmed, current) || !sameSessions(confirmed.sessions, current.sessions) {
		return problemf(ErrConflict, "工作树身份、分支、锁或会话集合已变化，请重新预览；尚未关闭任何会话")
	}
	if mode == "safe" && len(current.blockers) != 0 {
		return problemf(ErrConflict, "安全删除被阻止: %v", current.messages())
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	closed := make(map[string]bool)
	if mode == "force" {
		active := make([]*session.Session, 0, len(current.sessions))
		for _, info := range current.sessions {
			if info.Status == session.StatusExited {
				continue
			}
			item, err := s.manager.Get(info.ID)
			if err != nil {
				return problemf(ErrConflict, "会话集合已变化，请重新预览")
			}
			active = append(active, item)
		}
		// Manager.Close hands a hybrid shell to its driver, which records the
		// close and removes the shell; Done closes either way.
		for _, item := range active {
			_ = s.manager.Close(item.Info().ID)
		}
		for _, item := range active {
			select {
			case <-item.Done():
				closed[item.Info().ID] = true
			case <-ctx.Done():
				return problemf(ErrTimeout, "关闭会话未在期限内完成，工作树未删除；部分会话可能已关闭")
			}
		}
	}
	final, err := s.removalState(ctx, r, worktreeID)
	if err != nil {
		return err
	}
	defer final.identity.close()
	if !final.supported || !sameTarget(current, final) || !remainingSessions(current.sessions, final.sessions) {
		return problemf(ErrConflict, "关闭期间目标身份或会话集合变化，工作树未删除；部分会话可能已关闭")
	}
	for _, info := range final.sessions {
		if !closed[info.ID] && info.Status != session.StatusExited {
			return problemf(ErrConflict, "仍有运行会话，工作树未删除")
		}
	}
	if mode == "safe" && len(final.blockers) != 0 {
		return problemf(ErrConflict, "安全删除被最新状态阻止: %v", final.messages())
	}
	// The commit the records will be pinned to is the one the user saw.
	if s.headCommit(ctx, r, final.tree) != confirmedCommit {
		return problemf(ErrConflict, "提交已变化，工作树未删除，请重新预览；部分会话可能已关闭")
	}
	if final.identity.prunableMissing {
		if _, statErr := os.Lstat(path); !errors.Is(statErr, os.ErrNotExist) {
			return problemf(ErrConflict, "待清理工作树原路径已重新出现文件或目录，请重新预览")
		}
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	argv := []string{"worktree", "remove"}
	if mode == "force" {
		argv = append(argv, "--force")
	}
	argv = append(argv, "--", path)
	if _, err := s.git(ctx, r.entry.AnchorPath, argv...); err != nil {
		return problemf(errorKind(err), "%v；删除未完整成功，部分会话可能已关闭，请刷新实际状态", err)
	}
	// The records of the removed directory, including those of sessions the
	// removal closed, stay restorable.
	var closedHybrids []string
	for _, info := range current.sessions {
		if closed[info.ID] && info.Hybrid != nil {
			closedHybrids = append(closedHybrids, info.ID)
		}
	}
	s.retainRecords(ctx, r, repositoryID, confirmed.tree, confirmedCommit, closedHybrids)
	return nil
}

func (s *Service) clearConfirmations() {
	s.coord.Lock()
	defer s.coord.Unlock()
	for id, confirmation := range s.confirmations {
		confirmation.timer.Stop()
		confirmation.closeHandles()
		delete(s.confirmations, id)
	}
}
