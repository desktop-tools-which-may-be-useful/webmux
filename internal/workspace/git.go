package workspace

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/rainchan/webmux/internal/pathutil"
)

const (
	maxWorkspaceGitOutput = 32 << 20
	maxWorkspaceGitError  = 64 << 10
)

// gitEnv keeps every workspace Git call noninteractive and its output in
// the fixed English the callers parse.
var gitEnv = []string{"GIT_TERMINAL_PROMPT=0", "GCM_INTERACTIVE=never", "LC_ALL=C"}

func (s *Service) git(ctx context.Context, cwd string, args ...string) ([]byte, error) {
	out, _, err := s.gitOutput(ctx, cwd, args...)
	return out, err
}

// gitOutput is git that also returns what Git wrote to standard error, for
// callers that explain a failure themselves; the error is git's.
func (s *Service) gitOutput(ctx context.Context, cwd string, args ...string) ([]byte, string, error) {
	if s.gitPath == "" {
		return nil, "", problemf(ErrUnavailable, "%s", s.Capability().GitError)
	}
	stdout, run := s.runGit(ctx, cwd, nil, args...)
	if err := run.err; err != nil {
		if stdout.exceeded || run.stderrExceeded {
			return nil, run.stderr, problemf(ErrTooLarge, "Git 输出超过限制，实际状态可能已变化；请刷新")
		}
		if run.ctxErr != nil {
			return nil, run.stderr, problemf(ErrTimeout, "Git 操作取消或超时，实际状态可能已变化: %v", run.ctxErr)
		}
		var launch *exec.Error
		if errors.As(err, &launch) || errors.Is(err, os.ErrNotExist) || errors.Is(err, os.ErrPermission) {
			return nil, run.stderr, problemf(ErrUnavailable, "Git 无法执行: %v", err)
		}
		return nil, run.stderr, problemf(ErrInternal, "Git %s 失败: %s (%v)", args[0], strings.TrimSpace(run.stderr), err)
	}
	return stdout.Bytes(), run.stderr, nil
}

// runGit runs `git -C cwd args...` with gitEnv plus extra and bounded
// output; callers map the exit themselves. s.gitPath must be set.
func (s *Service) runGit(ctx context.Context, cwd string, extra []string, args ...string) (*boundedOutput, gitExit) {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	stdout := &boundedOutput{limit: maxWorkspaceGitOutput, cancel: cancel}
	run := runGitProcess(ctx, gitProcess{
		path:        s.gitPath,
		args:        append([]string{"-C", cwd}, args...),
		env:         append(append([]string{}, gitEnv...), extra...),
		stdout:      stdout,
		stderrLimit: maxWorkspaceGitError,
	})
	return stdout, run
}
func canonical(path string) (string, error) {
	path, err := pathutil.Expand(path)
	if err != nil {
		return "", err
	}
	return filepath.EvalSymlinks(path)
}

// canonicalTarget resolves existing ancestors too, including symlinked parents.
func canonicalTarget(path string) (string, error) {
	path, err := pathutil.Expand(path)
	if err != nil {
		return "", err
	}
	resolved, err := filepath.EvalSymlinks(path)
	if err == nil {
		return resolved, nil
	}
	if !errors.Is(err, os.ErrNotExist) {
		return "", err
	}
	parent := filepath.Dir(path)
	if parent == path {
		return "", err
	}
	resolved, err = canonicalTarget(parent)
	if err != nil {
		return "", err
	}
	return filepath.Join(resolved, filepath.Base(path)), nil
}
func within(root, path string) bool {
	rel, err := filepath.Rel(root, path)
	return err == nil && rel != ".." && !strings.HasPrefix(rel, ".."+string(os.PathSeparator)) && !filepath.IsAbs(rel)
}
func overlap(a, b string) bool      { return within(a, b) || within(b, a) }
func outputLine(data []byte) string { return strings.TrimSuffix(string(data), "\n") }
func (s *Service) common(ctx context.Context, path string) (string, error) {
	out, err := s.git(ctx, path, "rev-parse", "--path-format=absolute", "--git-common-dir")
	if err != nil {
		return "", err
	}
	return canonical(outputLine(out))
}
func (s *Service) locate(ctx context.Context, path string) (string, string, error) {
	path, err := canonical(path)
	if err != nil {
		return "", "", problemf(ErrInvalid, "仓库目录不可访问: %v", err)
	}
	common, err := s.common(ctx, path)
	if err != nil {
		if errorKind(err) == ErrInternal {
			err = problemf(ErrInvalid, "无法识别 Git 仓库（请检查目录权限和信任设置）: %v", err)
		}
		return "", "", err
	}
	out, err := s.git(ctx, path, "worktree", "list", "--porcelain", "-z")
	if err != nil {
		return "", "", err
	}
	trees, err := parseWorktrees(out, "")
	if err != nil || len(trees) == 0 {
		return "", "", problemf(ErrUnavailable, "Git 不支持所需 worktree 机器输出: %v", err)
	}
	return common, trees[0].Path, nil
}
func parseWorktrees(data []byte, repository string) ([]Worktree, error) {
	if len(data) == 0 || data[len(data)-1] != 0 {
		return nil, errors.New("worktree 输出不是 NUL 格式")
	}
	out := make([]Worktree, 0)
	var current *Worktree
	for _, raw := range bytes.Split(data, []byte{0}) {
		field := string(raw)
		if field == "" {
			current = nil
			continue
		}
		key, value, _ := strings.Cut(field, " ")
		if key == "worktree" {
			if !filepath.IsAbs(value) {
				return nil, errors.New("工作树路径不是绝对路径（来自 Git）")
			}
			path, err := canonicalTarget(value)
			if err != nil {
				path = filepath.Clean(value)
			}
			kind := "linked"
			if len(out) == 0 {
				kind = "main"
			}
			out = append(out, Worktree{ID: worktreeID(repository, path), RepositoryID: repository, Path: path, Kind: kind})
			current = &out[len(out)-1]
			continue
		}
		if current == nil {
			return nil, errors.New("工作树记录缺少路径（来自 Git）")
		}
		switch key {
		case "HEAD":
			if strings.Trim(value, "0") != "" {
				current.Head = strptr(value)
			}
		case "branch":
			current.Branch = strptr(strings.TrimPrefix(value, "refs/heads/"))
		case "detached":
			current.Detached = true
		case "bare":
			current.Kind = "bare"
		case "locked":
			current.Locked = true
			current.LockReason = value
		case "prunable":
			current.Prunable = true
		}
	}
	return out, nil
}
func (s *Service) snapshot(ctx context.Context, r *repositoryState) (Repository, error) {
	if err := ctx.Err(); err != nil {
		return Repository{}, err
	}
	r.flightMu.Lock()
	flight := r.flight
	if flight == nil {
		readCtx, cancel := context.WithTimeout(context.Background(), readTimeout)
		flight = &snapshotFlight{done: make(chan struct{}), cancel: cancel}
		r.flight = flight
		go func() {
			defer cancel()
			if err := acquire(readCtx, r); err != nil {
				flight.err = err
			} else {
				flight.repository = s.readRepository(readCtx, r)
				s.sweepOnce(readCtx, r)
				release(r)
			}
			r.flightMu.Lock()
			if r.flight == flight {
				r.flight = nil
			}
			close(flight.done)
			r.flightMu.Unlock()
		}()
	}
	flight.waiters++
	r.flightMu.Unlock()
	defer func() {
		r.flightMu.Lock()
		flight.waiters--
		if flight.waiters == 0 {
			flight.cancel()
			if r.flight == flight {
				r.flight = nil
			}
		}
		r.flightMu.Unlock()
	}()
	select {
	case <-ctx.Done():
		return Repository{}, ctx.Err()
	case <-flight.done:
		// A shared read may predate an instruction change.
		repository := flight.repository
		repository.InitInstruction = s.initInstruction(r)
		return repository, flight.err
	}
}
func repositoryResult(e registration) Repository {
	return Repository{ID: e.ID, Name: e.DisplayName, CommonDir: e.CommonDir, AnchorPath: e.AnchorPath, Availability: "unavailable", CheckedAt: time.Now().UTC(), Worktrees: []Worktree{}}
}

// unavailableRepository reports a read that failed before Git produced a
// snapshot, so one repository cannot hide the others in a list.
func (s *Service) unavailableRepository(r *repositoryState, err error) Repository {
	result := repositoryResult(r.entry)
	result.InitInstruction = s.initInstruction(r)
	result.Error = err.Error()
	return result
}

func (s *Service) readRepository(ctx context.Context, r *repositoryState) Repository {
	e := r.entry
	result := repositoryResult(e)
	result.InitInstruction = s.initInstruction(r)
	common, err := s.common(ctx, e.AnchorPath)
	if err == nil && common != e.CommonDir {
		err = errors.New("仓库 common directory 身份已改变，请移除并重新注册")
	}
	if err != nil {
		result.Error = err.Error()
		return result
	}
	data, err := s.git(ctx, e.AnchorPath, "worktree", "list", "--porcelain", "-z")
	if err != nil {
		result.Error = err.Error()
		return result
	}
	trees, err := parseWorktrees(data, e.ID)
	if err != nil {
		result.Error = err.Error()
		return result
	}
	// Worktrees have independent indexes, so their status checks run in
	// parallel; the service-wide pool avoids a process burst on large lists.
	limit := s.worktreeGate
	var wg sync.WaitGroup
	for i := range trees {
		wg.Add(1)
		go func(tree *Worktree) {
			defer wg.Done()
			select {
			case limit <- struct{}{}:
			case <-ctx.Done():
				tree.Error = ctx.Err().Error()
				return
			}
			defer func() { <-limit }()
			s.populateWorktreeStatus(ctx, e.CommonDir, tree)
		}(&trees[i])
	}
	wg.Wait()
	s.fillLastCommits(ctx, e.AnchorPath, trees)
	sort.SliceStable(trees, func(i, j int) bool {
		if trees[i].Kind != "linked" {
			return true
		}
		if trees[j].Kind != "linked" {
			return false
		}
		return trees[i].Path < trees[j].Path
	})
	result.Availability, result.Worktrees = "available", trees
	result.LastFetchedAt = lastFetched(e.CommonDir)
	return result
}

func (s *Service) populateWorktreeStatus(ctx context.Context, commonDir string, tree *Worktree) {
	stat, statErr := os.Stat(tree.Path)
	if statErr != nil || !stat.IsDir() {
		tree.Error = fmt.Sprintf("工作树目录不可访问: %v", statErr)
		return
	}
	if tree.Kind == "bare" {
		tree.Available = true
		return
	}
	actual, checkErr := s.common(ctx, tree.Path)
	if checkErr != nil || actual != commonDir {
		tree.Error = fmt.Sprintf("无法验证工作树身份: %v", checkErr)
		return
	}
	tree.Available = true
	tree.Operation = detectOperation(tree.Path)
	data, statusErr := s.git(ctx, tree.Path, "status", "--porcelain=v2", "-z", "--branch", "--untracked-files=normal", "--ignored=matching")
	if statusErr != nil {
		tree.Error = statusErr.Error()
		return
	}
	status, parseErr := parseStatus(data)
	if parseErr != nil {
		tree.Error = parseErr.Error()
		return
	}
	dirty := status.dirty
	tree.Dirty, tree.HasUntracked, tree.HasIgnored = &dirty, status.untracked, status.ignored
	tree.Upstream, tree.Ahead, tree.Behind, tree.UpstreamGone = status.upstream, status.ahead, status.behind, status.upstreamGone
	tree.Changes = &status.changes
}

// fillLastCommits describes every worktree's HEAD with one batched log;
// a failed read only leaves LastCommit empty.
func (s *Service) fillLastCommits(ctx context.Context, anchor string, trees []Worktree) {
	eligible := func(tree Worktree) bool {
		return tree.Available && tree.Kind != "bare" && tree.Head != nil && isObjectID(*tree.Head)
	}
	var heads []string
	seen := map[string]bool{}
	for _, tree := range trees {
		if eligible(tree) && !seen[*tree.Head] {
			seen[*tree.Head] = true
			heads = append(heads, *tree.Head)
		}
	}
	if len(heads) == 0 {
		return
	}
	commits, err := s.describeCommits(ctx, anchor, heads)
	if err != nil {
		return
	}
	byHash := make(map[string]Commit, len(commits))
	for _, commit := range commits {
		byHash[commit.Hash] = commit
	}
	for i := range trees {
		if !eligible(trees[i]) {
			continue
		}
		if commit, ok := byHash[*trees[i].Head]; ok {
			trees[i].LastCommit = &commit
		}
	}
}

// describeCommits reads exactly the given commits, without walking history.
func (s *Service) describeCommits(ctx context.Context, cwd string, oids []string) ([]Commit, error) {
	args := append([]string{"log", "--no-walk=unsorted", "--no-show-signature", "--format=%H%x00%s%x00%an%x00%cI%x00"}, oids...)
	data, err := s.git(ctx, cwd, append(args, "--")...)
	if err != nil {
		return nil, err
	}
	return parseCommitLog(data), nil
}

// verifyIdentity checks that the repository is still registered and its
// anchor still belongs to the registered common directory.
func (s *Service) verifyIdentity(ctx context.Context, r *repositoryState) error {
	if !s.registered(r) {
		return problemf(ErrNotFound, "仓库已移除")
	}
	common, err := s.common(ctx, r.entry.AnchorPath)
	if err == nil && common != r.entry.CommonDir {
		err = errors.New("仓库 common directory 身份已改变，请移除并重新注册")
	}
	if err != nil {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		return problemf(ErrUnavailable, "%s", err.Error())
	}
	return nil
}

func (s *Service) currentTree(ctx context.Context, r *repositoryState, id string) (Worktree, error) {
	trees, err := s.currentTrees(ctx, r, []string{id})
	if err != nil {
		return Worktree{}, err
	}
	return trees[0], nil
}

func (s *Service) currentTrees(ctx context.Context, r *repositoryState, ids []string) ([]Worktree, error) {
	selected, _, err := s.currentTreesListed(ctx, r, ids)
	return selected, err
}

// currentTreesListed is currentTrees that also returns every listed
// worktree, without its status.
func (s *Service) currentTreesListed(ctx context.Context, r *repositoryState, ids []string) ([]Worktree, []Worktree, error) {
	if err := s.verifyIdentity(ctx, r); err != nil {
		return nil, nil, err
	}
	e := r.entry
	data, err := s.git(ctx, e.AnchorPath, "worktree", "list", "--porcelain", "-z")
	if err != nil {
		if ctx.Err() != nil {
			return nil, nil, ctx.Err()
		}
		return nil, nil, problemf(ErrUnavailable, "%s", err.Error())
	}
	trees, err := parseWorktrees(data, e.ID)
	if err != nil {
		if ctx.Err() != nil {
			return nil, nil, ctx.Err()
		}
		return nil, nil, problemf(ErrUnavailable, "%s", err.Error())
	}
	byID := make(map[string]Worktree, len(trees))
	for _, tree := range trees {
		byID[tree.ID] = tree
	}
	selected := make([]Worktree, len(ids))
	for i, id := range ids {
		tree, ok := byID[id]
		if !ok {
			return nil, nil, problemf(ErrNotFound, "工作树不存在，请刷新")
		}
		selected[i] = tree
	}
	limit := s.worktreeGate
	var wg sync.WaitGroup
	for i := range selected {
		wg.Add(1)
		go func(tree *Worktree) {
			defer wg.Done()
			select {
			case limit <- struct{}{}:
			case <-ctx.Done():
				tree.Error = ctx.Err().Error()
				return
			}
			defer func() { <-limit }()
			s.populateWorktreeStatus(ctx, e.CommonDir, tree)
		}(&selected[i])
	}
	wg.Wait()
	if err := ctx.Err(); err != nil {
		return nil, nil, err
	}
	return selected, trees, nil
}
func (s *Service) current(ctx context.Context, r *repositoryState) (Repository, error) {
	if !s.registered(r) {
		return Repository{}, problemf(ErrNotFound, "仓库已移除")
	}
	repository := s.readRepository(ctx, r)
	if repository.Availability != "available" {
		if ctx.Err() != nil {
			return repository, ctx.Err()
		}
		return repository, problemf(ErrUnavailable, "%s", repository.Error)
	}
	return repository, nil
}
func findTree(repository Repository, id string) (Worktree, error) {
	for _, tree := range repository.Worktrees {
		if tree.ID == id {
			return tree, nil
		}
	}
	return Worktree{}, problemf(ErrNotFound, "工作树不存在，请刷新")
}
func (s *Service) branches(ctx context.Context, r *repositoryState, repository Repository) (Branches, error) {
	result := Branches{Branches: []Branch{}, Bases: []Base{}, DefaultParent: filepath.Join(filepath.Dir(r.entry.AnchorPath), filepath.Base(r.entry.AnchorPath)+".worktrees")}
	data, err := s.git(ctx, r.entry.AnchorPath, "for-each-ref", "--sort=-committerdate",
		"--format=%(refname)%00%(objectname)%00%(subject)%00%(upstream:short)%00%(upstream:track,nobracket)%00%(committerdate:iso-strict)%00",
		"refs/heads/", "refs/remotes/")
	if err != nil {
		return result, err
	}
	merged, err := s.mergedIntoDefault(ctx, r.entry.AnchorPath, repository)
	if err != nil {
		return result, err
	}
	const refFields = 6
	fields := bytes.Split(data, []byte{0})
	for i := 0; i+refFields-1 < len(fields); i += refFields {
		ref := strings.TrimPrefix(string(fields[i]), "\n")
		head := string(fields[i+1])
		subject := string(fields[i+2])
		committedAt := string(fields[i+5])
		if ref == "" {
			continue
		}
		kind := "remote"
		if name, ok := strings.CutPrefix(ref, "refs/heads/"); ok {
			kind = "local"
			branch := Branch{Name: name, Head: head, WorktreePaths: []string{}, Subject: subject, CommittedAt: committedAt}
			if upstream := string(fields[i+3]); upstream != "" {
				branch.Upstream = &upstream
				branch.Ahead, branch.Behind, branch.UpstreamGone = parseTrack(string(fields[i+4]))
			}
			for _, tree := range repository.Worktrees {
				if tree.Branch != nil && *tree.Branch == name {
					branch.WorktreePaths = append(branch.WorktreePaths, tree.Path)
				}
			}
			branch.MergedIntoDefault = merged[ref]
			result.Branches = append(result.Branches, branch)
		}
		result.Bases = append(result.Bases, Base{Name: ref, Head: head, Subject: subject, Kind: kind, CommittedAt: committedAt})
	}
	for _, tree := range repository.Worktrees {
		if tree.Kind == "main" && tree.Head != nil {
			result.DefaultBase = tree.Head
			result.DefaultBranch = tree.Branch
			subject, err := s.git(ctx, r.entry.AnchorPath, "log", "-1", "--no-show-signature", "--format=%s", *tree.Head, "--")
			if err != nil {
				return result, err
			}
			result.DefaultBaseSubject = strings.TrimSuffix(string(subject), "\n")
		}
	}
	return result, nil
}
func (s *Service) Branches(ctx context.Context, id string) (Branches, error) {
	ctx, cancel := context.WithTimeout(ctx, readTimeout)
	defer cancel()
	r, err := s.repository(id)
	if err != nil {
		return Branches{}, err
	}
	if err := acquire(ctx, r); err != nil {
		return Branches{}, err
	}
	defer release(r)
	repository, err := s.current(ctx, r)
	if err != nil {
		return Branches{}, err
	}
	return s.branches(ctx, r, repository)
}

// recentCommitLimit bounds the start-commit history offered to the browser.
const recentCommitLimit = 50

// maxCommitStarts bounds the tips merged into one history: the local branch
// tip and its freshly fetched upstream tip.
const maxCommitStarts = 2

// RecentCommits lists up to recentCommitLimit commits reachable from any of
// the commit object ids in `from` (a branch tip and optionally its upstream
// tip), newest first, so a fetched upstream commit appears beside local
// history. A requested tip outside the capped window is appended so it stays
// visible. Only full hexadecimal object ids are accepted so a value can never
// act as an option or ref expression.
func (s *Service) RecentCommits(ctx context.Context, id string, from []string) ([]Commit, error) {
	if len(from) == 0 || len(from) > maxCommitStarts {
		return nil, problemf(ErrInvalid, "from 需要 1 到 %d 个完整的提交 ID", maxCommitStarts)
	}
	for _, oid := range from {
		if !isObjectID(oid) {
			return nil, problemf(ErrInvalid, "from 必须是完整的提交 ID")
		}
	}
	ctx, cancel := context.WithTimeout(ctx, readTimeout)
	defer cancel()
	r, err := s.repository(id)
	if err != nil {
		return nil, err
	}
	if err := acquire(ctx, r); err != nil {
		return nil, err
	}
	defer release(r)
	if _, err := s.current(ctx, r); err != nil {
		return nil, err
	}
	for _, oid := range from {
		if _, err := s.git(ctx, r.entry.AnchorPath, "rev-parse", "--verify", "--end-of-options", oid+"^{commit}"); err != nil {
			if errorKind(err) != ErrInternal {
				return nil, err
			}
			return nil, problemf(ErrNotFound, "提交不存在")
		}
	}
	args := append([]string{"log", "-n", strconv.Itoa(recentCommitLimit), "--no-show-signature",
		"--format=%H%x00%s%x00%an%x00%cI%x00"}, from...)
	data, err := s.git(ctx, r.entry.AnchorPath, append(args, "--")...)
	if err != nil {
		return nil, err
	}
	commits := parseCommitLog(data)
	// The cap spans the merged walk, so an older requested tip can fall
	// outside it. Resolve any missing tip on its own instead of leaving the
	// browser without a row for the fetched upstream commit.
	seen := make(map[string]bool, len(commits))
	for _, commit := range commits {
		seen[commit.Hash] = true
	}
	var missing []string
	for _, oid := range from {
		if !seen[oid] {
			seen[oid] = true
			missing = append(missing, oid)
		}
	}
	if len(missing) > 0 {
		extra, err := s.describeCommits(ctx, r.entry.AnchorPath, missing)
		if err != nil {
			return nil, err
		}
		commits = append(commits, extra...)
		sort.SliceStable(commits, func(i, j int) bool {
			return commitTime(commits[i].CommittedAt).After(commitTime(commits[j].CommittedAt))
		})
	}
	return commits, nil
}

func parseCommitLog(data []byte) []Commit {
	commits := []Commit{}
	fields := bytes.Split(data, []byte{0})
	for i := 0; i+3 < len(fields); i += 4 {
		commits = append(commits, Commit{
			Hash:        strings.TrimPrefix(string(fields[i]), "\n"),
			Subject:     string(fields[i+1]),
			Author:      string(fields[i+2]),
			CommittedAt: string(fields[i+3]),
		})
	}
	return commits
}

func commitTime(value string) time.Time {
	parsed, err := time.Parse(time.RFC3339, value)
	if err != nil {
		return time.Time{}
	}
	return parsed
}

func isObjectID(value string) bool {
	if len(value) != 40 && len(value) != 64 {
		return false
	}
	for _, c := range value {
		if (c < '0' || c > '9') && (c < 'a' || c > 'f') {
			return false
		}
	}
	return true
}

// unsupportedContents is deliberately conservative: removal never traverses or
// cleans another repository, including ignored nested repositories/submodules.
func (s *Service) unsupportedContents(ctx context.Context, tree Worktree) error {
	data, err := s.git(ctx, tree.Path, "ls-files", "--stage", "-z")
	if err != nil {
		return err
	}
	for _, entry := range bytes.Split(data, []byte{0}) {
		if bytes.HasPrefix(entry, []byte("160000 ")) {
			return errors.New("工作树含子模块，请在终端处理")
		}
	}
	return filepath.WalkDir(tree.Path, func(path string, entry fs.DirEntry, walkErr error) error {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		if walkErr != nil {
			return walkErr
		}
		if path == tree.Path {
			return nil
		}
		if entry.Name() == ".git" {
			if filepath.Dir(path) != tree.Path {
				return errors.New("工作树含嵌套 Git 仓库，请在终端处理")
			}
			if entry.IsDir() {
				return filepath.SkipDir
			}
		}
		if entry.IsDir() {
			// Bare repositories have no .git entry.
			if stat, err := os.Stat(filepath.Join(path, "HEAD")); err == nil && !stat.IsDir() {
				if objects, err := os.Stat(filepath.Join(path, "objects")); err == nil && objects.IsDir() {
					return errors.New("工作树含嵌套 bare 仓库，请在终端处理")
				}
			}
		}
		return nil
	})
}
