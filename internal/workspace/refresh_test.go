package workspace

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func upstreamFixture(t *testing.T) (*Service, Repository, string, string) {
	t.Helper()
	root := initRepo(t)
	remote := filepath.Join(t.TempDir(), "remote.git")
	gitTest(t, root, "clone", "--bare", root, remote)
	gitTest(t, root, "remote", "add", "origin", remote)
	gitTest(t, root, "push", "-u", "origin", "main")
	s, _ := testService(t, t.TempDir(), root)
	r := registerTest(t, s, root)
	return s, r, root, remote
}

func advanceUpstream(t *testing.T, root, remote, subject string) string {
	t.Helper()
	other := filepath.Join(t.TempDir(), "other")
	gitTest(t, root, "clone", remote, other)
	putFile(t, filepath.Join(other, "remote"), subject+"\n")
	gitTest(t, other, "add", "remote")
	gitTest(t, other, "commit", "-m", subject)
	gitTest(t, other, "push", "origin", "main")
	return gitTest(t, other, "rev-parse", "HEAD")
}

func TestRefreshBranchBaseUsesConfiguredUpstream(t *testing.T) {
	t.Parallel()
	isolateGit(t)
	s, r, root, remote := upstreamFixture(t)
	localHead := gitTest(t, root, "rev-parse", "main")
	remoteHead := advanceUpstream(t, root, remote, "remote latest")
	result, err := s.RefreshBranchBase(context.Background(), r.ID, "main")
	if err != nil {
		t.Fatal(err)
	}
	if result.Branch != "main" || result.Base != remoteHead || result.UpstreamHead != remoteHead || result.BaseSubject != "remote latest" || result.BaseSource != "remote" || result.Upstream != "origin/main" || result.FetchWarning != "" {
		t.Fatalf("remote refresh: %+v", result)
	}
	if head := gitTest(t, root, "rev-parse", "main"); head != localHead {
		t.Fatalf("fetch reset local branch: %s -> %s", localHead, head)
	}
	if branch := gitTest(t, root, "branch", "--show-current"); branch != "main" {
		t.Fatalf("fetch changed worktree branch: %q", branch)
	}
	if tracking := gitTest(t, root, "rev-parse", "refs/remotes/origin/main"); tracking != remoteHead {
		t.Fatalf("tracking ref not refreshed: %s", tracking)
	}
}

func TestRefreshBranchBaseRequiresConfiguredRemoteUpstream(t *testing.T) {
	t.Parallel()
	isolateGit(t)
	root := initRepo(t)
	gitTest(t, root, "branch", "isolated")
	gitTest(t, root, "branch", "local-upstream")
	gitTest(t, root, "update-ref", "refs/remotes/origin/isolated", "HEAD")
	gitTest(t, root, "config", "branch.local-upstream.remote", ".")
	gitTest(t, root, "config", "branch.local-upstream.merge", "refs/heads/main")
	s, _ := testService(t, t.TempDir(), root)
	r := registerTest(t, s, root)
	localHead := gitTest(t, root, "rev-parse", "HEAD")

	result, err := s.RefreshBranchBase(context.Background(), r.ID, "isolated")
	if err != nil {
		t.Fatal(err)
	}
	if result.Base != localHead || result.BaseSource != "local" || result.Upstream != "" || !strings.Contains(result.FetchWarning, "未配置上游") {
		t.Fatalf("guessed same-name remote: %+v", result)
	}
	result, err = s.RefreshBranchBase(context.Background(), r.ID, "local-upstream")
	if err != nil {
		t.Fatal(err)
	}
	if result.Base != localHead || result.BaseSource != "local" || result.Upstream != "main" || !strings.Contains(result.FetchWarning, "不是远程仓库") {
		t.Fatalf("local upstream used as remote: %+v", result)
	}
	if _, err := s.RefreshBranchBase(context.Background(), r.ID, "missing"); errorKind(err) != ErrNotFound {
		t.Fatalf("unknown branch: %v", err)
	}
}

func TestRefreshBranchBaseKeepsDivergedLocalAndReportsUpstreamTip(t *testing.T) {
	t.Parallel()
	isolateGit(t)
	s, r, root, remote := upstreamFixture(t)
	putFile(t, filepath.Join(root, "local"), "local only\n")
	gitTest(t, root, "add", "local")
	gitTest(t, root, "commit", "-m", "local only")
	localHead := gitTest(t, root, "rev-parse", "main")
	remoteHead := advanceUpstream(t, root, remote, "remote latest")
	result, err := s.RefreshBranchBase(context.Background(), r.ID, "main")
	if err != nil {
		t.Fatal(err)
	}
	if result.Base != localHead || result.BaseSubject != "local only" || result.BaseSource != "local" || !strings.Contains(result.FetchWarning, "上游未有") {
		t.Fatalf("diverged branch replaced by upstream: %+v", result)
	}
	// The fetched tip is still reported so the browser can offer it.
	if result.UpstreamHead != remoteHead {
		t.Fatalf("upstream tip not reported: %+v", result)
	}
}

func TestRefreshBranchBaseFailureAndTimeoutKeepLocalBase(t *testing.T) {
	t.Parallel()
	isolateGit(t)
	t.Run("failure", func(t *testing.T) {
		s, r, root, _ := upstreamFixture(t)
		localHead := gitTest(t, root, "rev-parse", "main")
		gitTest(t, root, "remote", "set-url", "origin", filepath.Join(t.TempDir(), "missing.git"))
		result, err := s.RefreshBranchBase(context.Background(), r.ID, "main")
		if err != nil {
			t.Fatal(err)
		}
		if result.Base != localHead || result.BaseSource != "local" || !strings.Contains(result.FetchWarning, "失败") {
			t.Fatalf("failed fetch did not degrade: %+v", result)
		}
		if strings.Contains(result.FetchWarning, "missing.git") {
			t.Fatalf("warning exposes remote detail: %q", result.FetchWarning)
		}
	})
	t.Run("timeout", func(t *testing.T) {
		s, r, root, _ := upstreamFixture(t)
		localHead := gitTest(t, root, "rev-parse", "main")
		realGit := s.gitPath
		wrapper := filepath.Join(t.TempDir(), "git")
		putFile(t, wrapper, fmt.Sprintf("#!/bin/sh\nif [ \"$3\" = fetch ]; then sleep 5; fi\nexec %q \"$@\"\n", realGit))
		if err := os.Chmod(wrapper, 0700); err != nil {
			t.Fatal(err)
		}
		s.gitPath = wrapper
		s.fetchTimeout = 30 * time.Millisecond
		started := time.Now()
		result, err := s.RefreshBranchBase(context.Background(), r.ID, "main")
		if err != nil {
			t.Fatal(err)
		}
		if time.Since(started) > 2*time.Second || result.Base != localHead || result.BaseSource != "local" || !strings.Contains(result.FetchWarning, "超时") {
			t.Fatalf("timed out fetch did not degrade promptly: %+v", result)
		}
	})
}

func TestRefreshBranchBaseDoesNotBlockWorktreeCreation(t *testing.T) {
	t.Parallel()
	isolateGit(t)
	s, r, _, _ := upstreamFixture(t)
	realGit := s.gitPath
	dir := t.TempDir()
	wrapper := filepath.Join(dir, "git")
	started := filepath.Join(dir, "fetch-started")
	releaseFetch := filepath.Join(dir, "release-fetch")
	putFile(t, wrapper, fmt.Sprintf("#!/bin/sh\nif [ \"$3\" = fetch ]; then\n  : > %q\n  while [ ! -e %q ]; do sleep 0.02; done\nfi\nexec %q \"$@\"\n", started, releaseFetch, realGit))
	if err := os.Chmod(wrapper, 0700); err != nil {
		t.Fatal(err)
	}
	s.gitPath = wrapper
	defer func() { _ = os.WriteFile(releaseFetch, nil, 0600) }()

	type refreshResult struct {
		result BranchBaseRefresh
		err    error
	}
	refreshed := make(chan refreshResult, 1)
	go func() {
		result, err := s.RefreshBranchBase(context.Background(), r.ID, "main")
		refreshed <- refreshResult{result: result, err: err}
	}()
	deadline := time.Now().Add(5 * time.Second)
	for {
		if _, err := os.Stat(started); err == nil {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("fetch did not start")
		}
		time.Sleep(10 * time.Millisecond)
	}

	type createResult struct {
		tree Worktree
		err  error
	}
	created := make(chan createResult, 1)
	createPath := filepath.Join(t.TempDir(), "during-fetch")
	go func() {
		tree, err := s.CreateWorktree(context.Background(), r.ID, CreateRequest{Path: createPath, Mode: "new", Branch: "during-fetch", Base: "HEAD"})
		created <- createResult{tree: tree, err: err}
	}()
	select {
	case result := <-created:
		if result.err != nil || result.tree.Branch == nil || *result.tree.Branch != "during-fetch" {
			t.Fatalf("concurrent create: %+v %v", result.tree, result.err)
		}
	case <-time.After(3 * time.Second):
		_ = os.WriteFile(releaseFetch, nil, 0600)
		t.Fatal("worktree creation waited for network fetch")
	}
	if err := os.WriteFile(releaseFetch, nil, 0600); err != nil {
		t.Fatal(err)
	}
	select {
	case result := <-refreshed:
		if result.err != nil || result.result.BaseSource != "remote" {
			t.Fatalf("released refresh: %+v %v", result.result, result.err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("fetch did not finish after release")
	}
}
