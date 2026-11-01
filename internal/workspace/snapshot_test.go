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

func TestUnknownStatusAndExecutablePermissionAreNotClean(t *testing.T) {
	t.Parallel()
	isolateGit(t)
	root := initRepo(t)
	s, _ := testService(t, t.TempDir(), root)
	r := registerTest(t, s, root)
	tree := linkedTest(t, s, r, "unknown")
	realGit := s.gitPath
	wrapper := filepath.Join(t.TempDir(), "git")
	putFile(t, wrapper, fmt.Sprintf("#!/bin/sh\nif [ \"$3\" = status ]; then echo 'permission denied reading index' >&2; exit 1; fi\nexec %q \"$@\"\n", realGit))
	if err := os.Chmod(wrapper, 0700); err != nil {
		t.Fatal(err)
	}
	s.gitPath = wrapper
	snapshot, err := s.Refresh(context.Background(), r.ID)
	if err != nil {
		t.Fatal(err)
	}
	unknown, err := findTree(snapshot, tree.ID)
	if err != nil {
		t.Fatal(err)
	}
	if unknown.Dirty != nil || !strings.Contains(unknown.Error, "permission denied") {
		t.Fatalf("unknown status shown clean: %+v", unknown)
	}
	preview := previewTest(t, s, r, tree)
	if preview.ForceAllowed {
		t.Fatal("unreadable status allowed force")
	}
	if err := os.Chmod(wrapper, 0600); err != nil {
		t.Fatal(err)
	}
	snapshot, err = s.Refresh(context.Background(), r.ID)
	if err != nil || snapshot.Availability != "unavailable" || snapshot.Error == "" {
		t.Fatalf("executable permission failure: %+v %v", snapshot, err)
	}
	item, err := s.CreateSession(context.Background(), SessionInput{Cwd: root, Cmd: []string{"/bin/sh", "-c", "printf PERMISSION_DIRECT"}})
	if err != nil {
		t.Fatal(err)
	}
	waitDone(t, item)
}
func TestListReadsRepositoriesConcurrently(t *testing.T) {
	t.Parallel()
	isolateGit(t)
	root := initRepo(t)
	other := initRepo(t)
	s, _ := testService(t, t.TempDir(), root)
	registerTest(t, s, root)
	registerTest(t, s, other)
	dir := t.TempDir()
	lock := filepath.Join(dir, "active")
	overlap := filepath.Join(dir, "overlap")
	wrapper := filepath.Join(dir, "git")
	realGit := s.gitPath
	// Every Git call holds a lock directory across a short sleep. A second
	// repository starting while one holds it proves List runs them in parallel.
	putFile(t, wrapper, fmt.Sprintf("#!/bin/sh\nif ! mkdir %q 2>/dev/null; then : > %q; fi\nsleep 0.5\nrmdir %q 2>/dev/null\nexec %q \"$@\"\n", lock, overlap, lock, realGit))
	if err := os.Chmod(wrapper, 0700); err != nil {
		t.Fatal(err)
	}
	s.gitPath = wrapper
	list, err := s.List(context.Background())
	if err != nil || len(list) != 2 {
		t.Fatalf("list: %+v %v", list, err)
	}
	for _, repository := range list {
		if repository.Availability != "available" {
			t.Fatalf("repository unavailable: %+v", repository)
		}
	}
	if _, err := os.Stat(overlap); err != nil {
		t.Fatal("List read repositories serially")
	}
}

func TestConcurrentSnapshotsCoalesceAndCancelOnlyLastWaiter(t *testing.T) {
	t.Parallel()
	isolateGit(t)
	root := initRepo(t)
	s, _ := testService(t, t.TempDir(), root)
	r := registerTest(t, s, root)
	repository, err := s.repository(r.ID)
	if err != nil {
		t.Fatal(err)
	}
	// Hold the repository gate before any Git command starts. Two real snapshot
	// requests must share one flight; cancelling one must not cancel the other.
	if err := acquire(context.Background(), repository); err != nil {
		t.Fatal(err)
	}
	firstCtx, cancelFirst := context.WithCancel(context.Background())
	first := make(chan error, 1)
	second := make(chan error, 1)
	go func() { _, err := s.snapshot(firstCtx, repository); first <- err }()
	deadline := time.Now().Add(3 * time.Second)
	for {
		repository.flightMu.Lock()
		flight := repository.flight
		repository.flightMu.Unlock()
		if flight != nil {
			break
		}
		if time.Now().After(deadline) {
			release(repository)
			t.Fatal("first snapshot not started")
		}
		time.Sleep(time.Millisecond)
	}
	go func() {
		value, err := s.snapshot(context.Background(), repository)
		if err == nil && value.Availability != "available" {
			err = fmt.Errorf("snapshot unavailable: %s", value.Error)
		}
		second <- err
	}()
	for {
		repository.flightMu.Lock()
		flight := repository.flight
		joined := flight != nil && flight.waiters == 2
		repository.flightMu.Unlock()
		if joined {
			break
		}
		if time.Now().After(deadline) {
			cancelFirst()
			release(repository)
			t.Fatal("concurrent snapshot was not coalesced")
		}
		time.Sleep(time.Millisecond)
	}
	cancelFirst()
	if err := <-first; errorKind(err) != ErrTimeout {
		release(repository)
		t.Fatalf("cancelled reader: %v", err)
	}
	release(repository)
	if err := <-second; err != nil {
		t.Fatal("one cancelled reader cancelled shared read", err)
	}
	// The sole waiter cancellation also releases an in-flight Git operation gate.
	if err := acquire(context.Background(), repository); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	final := make(chan error, 1)
	go func() { _, err := s.snapshot(ctx, repository); final <- err }()
	cancel()
	if err := <-final; errorKind(err) != ErrTimeout {
		release(repository)
		t.Fatalf("sole cancelled reader: %v", err)
	}
	release(repository)
	if _, err := s.Refresh(context.Background(), r.ID); err != nil {
		t.Fatal("cancelled read retained repository gate", err)
	}
}

func TestReadRepositoryLeavesFullWorktreePoolOnCancel(t *testing.T) {
	t.Parallel()
	isolateGit(t)
	root := initRepo(t)
	s, _ := testService(t, t.TempDir(), root)
	r := registerTest(t, s, root)
	tree := linkedTest(t, s, r, "pooled")
	repository, err := s.repository(r.ID)
	if err != nil {
		t.Fatal(err)
	}
	// Fill every pool slot so the worktree goroutine can only proceed when it
	// observes cancellation. A Git wrapper signals after `worktree list` runs,
	// proving the read reached the pooled phase before its context is cancelled.
	dir := t.TempDir()
	signal := filepath.Join(dir, "listed")
	realGit := s.gitPath
	wrapper := filepath.Join(dir, "git")
	putFile(t, wrapper, fmt.Sprintf("#!/bin/sh\nfor arg in \"$@\"; do\n  if [ \"$arg\" = list ]; then\n    %q \"$@\"\n    status=$?\n    : > %q\n    exit $status\n  fi\ndone\nexec %q \"$@\"\n", realGit, signal, realGit))
	if err := os.Chmod(wrapper, 0700); err != nil {
		t.Fatal(err)
	}
	s.gitPath = wrapper
	for range cap(s.worktreeGate) {
		s.worktreeGate <- struct{}{}
	}
	defer func() {
		for range cap(s.worktreeGate) {
			<-s.worktreeGate
		}
	}()
	// The signal fires before Go has collected `worktree list`, so a cancel
	// can still land before the pooled phase. Such an attempt fails the whole
	// read without worktrees; retry until cancellation hits the pooled wait.
	for attempt := 0; ; attempt++ {
		if attempt == 20 {
			t.Fatal("cancellation never reached the pooled worktree phase")
		}
		if err := os.Remove(signal); err != nil && !os.IsNotExist(err) {
			t.Fatal(err)
		}
		ctx, cancel := context.WithCancel(context.Background())
		done := make(chan Repository, 1)
		go func() { done <- s.readRepository(ctx, repository) }()
		deadline := time.Now().Add(3 * time.Second)
		for {
			if _, err := os.Stat(signal); err == nil {
				break
			}
			if time.Now().After(deadline) {
				cancel()
				t.Fatal("worktree list did not run")
			}
			time.Sleep(time.Millisecond)
		}
		time.Sleep(time.Duration(attempt) * 10 * time.Millisecond)
		cancel()
		var result Repository
		select {
		case result = <-done:
		case <-time.After(3 * time.Second):
			t.Fatal("worktree read ignored cancellation while waiting for the pool")
		}
		read, err := findTree(result, tree.ID)
		if err != nil {
			if result.Error != "" {
				continue
			}
			t.Fatalf("cancelled read: %+v %v", result, err)
		}
		if !strings.Contains(read.Error, context.Canceled.Error()) {
			t.Fatalf("cancelled worktree read: %+v", read)
		}
		return
	}
}
