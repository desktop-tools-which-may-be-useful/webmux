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

// RefreshBranchBase fetches only the selected branch's configured upstream.
// Network failures degrade to the local tip so worktree creation remains usable.
func (s *Service) RefreshBranchBase(ctx context.Context, repositoryID, branch string) (BranchBaseRefresh, error) {
	if branch == "" || strings.ContainsRune(branch, '\x00') {
		return BranchBaseRefresh{}, problemf(ErrInvalid, "请选择明确的本地分支")
	}
	readCtx, cancel := context.WithTimeout(ctx, readTimeout)
	result, upstream, anchor, err := s.localBranchBase(readCtx, repositoryID, branch)
	cancel()
	if err != nil || upstream.ref == "" {
		return result, err
	}

	timeout := s.fetchTimeout
	if timeout <= 0 {
		timeout = branchFetchTimeout
	}
	fetchCtx, fetchCancel := context.WithTimeout(ctx, timeout)
	_, fetchErr := s.git(fetchCtx, anchor, "fetch", "--no-tags", upstream.remote, upstream.merge)
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

func (s *Service) localBranchBase(ctx context.Context, repositoryID, branch string) (BranchBaseRefresh, branchUpstream, string, error) {
	result := BranchBaseRefresh{Branch: branch, BaseSource: "local"}
	r, err := s.repository(repositoryID)
	if err != nil {
		return result, branchUpstream{}, "", err
	}
	if err := acquire(ctx, r); err != nil {
		return result, branchUpstream{}, "", err
	}
	defer release(r)
	repository, err := s.current(ctx, r)
	if err != nil {
		return result, branchUpstream{}, "", err
	}
	branches, err := s.branches(ctx, r, repository)
	if err != nil {
		return result, branchUpstream{}, "", err
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
		return result, branchUpstream{}, "", problemf(ErrNotFound, "本地分支不存在")
	}

	data, err := s.git(ctx, r.entry.AnchorPath, "for-each-ref", "--format=%(upstream:short)%00%(upstream)", localRef)
	if err != nil {
		return result, branchUpstream{}, "", err
	}
	short, full, ok := strings.Cut(strings.TrimSuffix(string(data), "\n"), "\x00")
	if !ok || short == "" || full == "" {
		result.FetchWarning = "该分支未配置上游，保留本地基线"
		return result, branchUpstream{}, "", nil
	}
	result.Upstream = short
	remoteData, err := s.git(ctx, r.entry.AnchorPath, "config", "--get", "branch."+branch+".remote")
	if err != nil {
		return result, branchUpstream{}, "", err
	}
	mergeData, err := s.git(ctx, r.entry.AnchorPath, "config", "--get", "branch."+branch+".merge")
	if err != nil {
		return result, branchUpstream{}, "", err
	}
	upstream := branchUpstream{remote: outputLine(remoteData), merge: outputLine(mergeData), ref: full}
	if upstream.remote == "" || upstream.merge == "" {
		result.FetchWarning = "该分支的上游配置不完整，保留本地基线"
		return result, branchUpstream{}, "", nil
	}
	if upstream.remote == "." || !strings.HasPrefix(full, "refs/remotes/") {
		result.FetchWarning = "该分支的上游不是远程仓库，保留本地基线"
		return result, branchUpstream{}, "", nil
	}
	return result, upstream, r.entry.AnchorPath, nil
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
