package workspace

import (
	"context"
	"strings"
	"time"
)

const branchFetchTimeout = 30 * time.Second

type branchUpstream struct {
	remote string
	merge  string
	ref    string
}

// RefreshBranchBase fetches the selected local branch's configured upstream, or
// when branch names a remote-tracking branch (or a remote exists for the ref),
// fetches that remote and resolves the selected branch. Network failures
// degrade to the local tip so worktree creation remains usable.
func (s *Service) RefreshBranchBase(ctx context.Context, repositoryID, branch string) (BranchBaseRefresh, error) {
	if branch == "" || strings.ContainsRune(branch, '\x00') {
		return BranchBaseRefresh{}, problemf(ErrInvalid, "请选择明确的分支")
	}
	readCtx, cancel := context.WithTimeout(ctx, readTimeout)
	result, upstream, anchor, remoteOnly, err := s.branchBaseTarget(readCtx, repositoryID, branch)
	cancel()
	if err != nil {
		return result, err
	}
	if remoteOnly {
		return s.refreshRemoteTrackingBranch(ctx, repositoryID, anchor, branch, result, upstream)
	}
	if upstream.ref == "" {
		return result, nil
	}

	timeout := s.fetchTimeout
	if timeout <= 0 {
		timeout = branchFetchTimeout
	}
	fetchCtx, fetchCancel := context.WithTimeout(ctx, timeout)
	_, fetchErr := s.git(fetchCtx, anchor, "fetch", "--prune", "--no-tags", upstream.remote)
	if fetchErr != nil {
		_, fetchErr = s.git(fetchCtx, anchor, "fetch", "--no-tags", upstream.remote, upstream.merge)
	}
	fetchCancel()
	if fetchErr != nil {
		if ctx.Err() != nil {
			return result, fetchErr
		}
		result.FetchWarning = "上游刷新失败或已超时，保留本地基线；请重试"
		return result, nil
	}

	readCtx, cancel = context.WithTimeout(ctx, readTimeout)
	remote, useRemote, err := s.remoteBranchBase(readCtx, repositoryID, result.Base, upstream.ref)
	cancel()
	if err != nil {
		if ctx.Err() != nil {
			return result, err
		}
		result.FetchWarning = "上游刷新后无法读取提交，保留本地基线；请重试"
		return result, nil
	}
	result.UpstreamHead = remote.Head
	if !useRemote {
		result.FetchWarning = "本地分支包含上游未有的提交，保留本地基线"
		return result, nil
	}
	result.Base = remote.Head
	result.BaseSubject = remote.Subject
	result.BaseSource = "remote"
	result.FetchWarning = ""
	return result, nil
}

func (s *Service) refreshRemoteTrackingBranch(ctx context.Context, repositoryID, anchor, branch string, result BranchBaseRefresh, upstream branchUpstream) (BranchBaseRefresh, error) {
	if upstream.remote == "" {
		if result.Base == "" {
			return result, problemf(ErrNotFound, "分支不存在")
		}
		result.FetchWarning = "该分支未配置远程仓库，保留本地基线"
		return result, nil
	}
	timeout := s.fetchTimeout
	if timeout <= 0 {
		timeout = branchFetchTimeout
	}
	fetchCtx, fetchCancel := context.WithTimeout(ctx, timeout)
	_, fetchErr := s.git(fetchCtx, anchor, "fetch", "--prune", "--no-tags", upstream.remote)
	fetchCancel()
	if fetchErr != nil {
		if ctx.Err() != nil {
			return result, fetchErr
		}
		if result.Base == "" {
			return result, problemf(ErrNotFound, "分支不存在")
		}
		result.FetchWarning = "上游刷新失败或已超时，保留本地基线；请重试"
		return result, nil
	}
	readCtx, cancel := context.WithTimeout(ctx, readTimeout)
	remote, found, err := s.readRefBase(readCtx, repositoryID, upstream.ref)
	cancel()
	if err != nil {
		if ctx.Err() != nil {
			return result, err
		}
		if result.Base == "" {
			return result, err
		}
		result.FetchWarning = "上游刷新后无法读取提交，保留本地基线；请重试"
		return result, nil
	}
	if !found {
		if result.Base == "" {
			return result, problemf(ErrNotFound, "分支不存在")
		}
		result.FetchWarning = "远程分支已不存在，保留本地基线"
		return result, nil
	}
	result.Branch = branch
	result.Base = remote.Head
	result.BaseSubject = remote.Subject
	result.BaseSource = "remote"
	result.Upstream = branch
	result.UpstreamHead = remote.Head
	result.FetchWarning = ""
	return result, nil
}

func (s *Service) branchBaseTarget(ctx context.Context, repositoryID, branch string) (BranchBaseRefresh, branchUpstream, string, bool, error) {
	result := BranchBaseRefresh{Branch: branch, BaseSource: "local"}
	r, err := s.repository(repositoryID)
	if err != nil {
		return result, branchUpstream{}, "", false, err
	}
	if err := acquire(ctx, r); err != nil {
		return result, branchUpstream{}, "", false, err
	}
	defer release(r)
	repository, err := s.current(ctx, r)
	if err != nil {
		return result, branchUpstream{}, "", false, err
	}
	branches, err := s.branches(ctx, r, repository)
	if err != nil {
		return result, branchUpstream{}, "", false, err
	}
	localRef := "refs/heads/" + branch
	for _, candidate := range branches.Bases {
		if candidate.Name == localRef {
			result.Base = candidate.Head
			result.BaseSubject = candidate.Subject
			break
		}
	}
	if result.Base == "" {
		remoteRef := "refs/remotes/" + branch
		for _, candidate := range branches.Bases {
			if candidate.Name == remoteRef && !strings.HasSuffix(candidate.Name, "/HEAD") {
				result.Base = candidate.Head
				result.BaseSubject = candidate.Subject
				result.BaseSource = "remote"
				result.Upstream = branch
				result.UpstreamHead = candidate.Head
				break
			}
		}
		remote := s.configuredRemote(ctx, r.entry.AnchorPath, branch)
		if result.Base == "" && remote == "" {
			return result, branchUpstream{}, "", false, problemf(ErrNotFound, "本地分支不存在")
		}
		return result, branchUpstream{remote: remote, ref: remoteRef}, r.entry.AnchorPath, true, nil
	}

	data, err := s.git(ctx, r.entry.AnchorPath, "for-each-ref", "--format=%(upstream:short)%00%(upstream)", localRef)
	if err != nil {
		return result, branchUpstream{}, "", false, err
	}
	short, full, ok := strings.Cut(strings.TrimSuffix(string(data), "\n"), "\x00")
	if !ok || short == "" || full == "" {
		result.FetchWarning = "该分支未配置上游，保留本地基线"
		return result, branchUpstream{}, "", false, nil
	}
	result.Upstream = short
	remoteData, err := s.git(ctx, r.entry.AnchorPath, "config", "--get", "branch."+branch+".remote")
	if err != nil {
		return result, branchUpstream{}, "", false, err
	}
	mergeData, err := s.git(ctx, r.entry.AnchorPath, "config", "--get", "branch."+branch+".merge")
	if err != nil {
		return result, branchUpstream{}, "", false, err
	}
	upstream := branchUpstream{remote: outputLine(remoteData), merge: outputLine(mergeData), ref: full}
	if upstream.remote == "" || upstream.merge == "" {
		result.FetchWarning = "该分支的上游配置不完整，保留本地基线"
		return result, branchUpstream{}, "", false, nil
	}
	if upstream.remote == "." || !strings.HasPrefix(full, "refs/remotes/") {
		result.FetchWarning = "该分支的上游不是远程仓库，保留本地基线"
		return result, branchUpstream{}, "", false, nil
	}
	return result, upstream, r.entry.AnchorPath, false, nil
}

// configuredRemote matches the longest configured git remote prefix of branch
// ("origin/feature/x" -> "origin"). It returns "" when no remote matches.
func (s *Service) configuredRemote(ctx context.Context, anchor, branch string) string {
	data, err := s.git(ctx, anchor, "remote")
	if err != nil {
		return ""
	}
	best := ""
	for _, line := range strings.Split(string(data), "\n") {
		remote := strings.TrimSpace(line)
		if remote == "" {
			continue
		}
		if strings.HasPrefix(branch, remote+"/") && len(remote) < len(branch)-1 && len(remote) > len(best) {
			best = remote
		}
	}
	return best
}

func (s *Service) readRefBase(ctx context.Context, repositoryID, ref string) (Base, bool, error) {
	r, err := s.repository(repositoryID)
	if err != nil {
		return Base{}, false, err
	}
	if err := acquire(ctx, r); err != nil {
		return Base{}, false, err
	}
	defer release(r)
	repository, err := s.current(ctx, r)
	if err != nil {
		return Base{}, false, err
	}
	branches, err := s.branches(ctx, r, repository)
	if err != nil {
		return Base{}, false, err
	}
	for _, candidate := range branches.Bases {
		if candidate.Name == ref && !strings.HasSuffix(candidate.Name, "/HEAD") {
			return candidate, true, nil
		}
	}
	return Base{}, false, nil
}

func (s *Service) remoteBranchBase(ctx context.Context, repositoryID, localOID, upstreamRef string) (Base, bool, error) {
	r, err := s.repository(repositoryID)
	if err != nil {
		return Base{}, false, err
	}
	if err := acquire(ctx, r); err != nil {
		return Base{}, false, err
	}
	defer release(r)
	repository, err := s.current(ctx, r)
	if err != nil {
		return Base{}, false, err
	}
	branches, err := s.branches(ctx, r, repository)
	if err != nil {
		return Base{}, false, err
	}
	for _, candidate := range branches.Bases {
		if candidate.Name == upstreamRef {
			_, err := s.git(ctx, r.entry.AnchorPath, "merge-base", "--is-ancestor", localOID, candidate.Head)
			if err != nil && ctx.Err() != nil {
				return Base{}, false, err
			}
			return candidate, err == nil, nil
		}
	}
	return Base{}, false, problemf(ErrInternal, "上游跟踪引用不存在")
}
