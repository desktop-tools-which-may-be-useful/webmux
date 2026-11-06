package workspace

import (
	"context"
	"slices"
	"strings"
	"time"
)

// PushState says whether a commit is contained in a remote-tracking ref.
type PushState string

const (
	// PushPushed: at least one remote-tracking ref contains the commit.
	PushPushed PushState = "pushed"
	// PushUnpushed: remotes exist but no remote-tracking ref contains it.
	PushUnpushed PushState = "unpushed"
	// PushNoRemote: the repository has no remote configured.
	PushNoRemote PushState = "noRemote"
	// PushUnknown: no commit, or the check failed or ran out of time.
	PushUnknown PushState = "unknown"
)

const (
	// pushCheckTimeout bounds the two local Git reads of a push check.
	pushCheckTimeout = 5 * time.Second
	// maxPushBranches bounds the remote branches a push check names.
	maxPushBranches = 5
)

// PushCheck is the local view of whether a commit survives the repository:
// it reads remote-tracking refs only and never contacts a remote, so it can
// lag behind the remote.
type PushCheck struct {
	State PushState
	// Branches name remote-tracking branches ("origin/main") containing the
	// commit, at most maxPushBranches; More counts the rest.
	Branches []string
	More     int
}

// checkPushed classifies commit against r's remote-tracking refs.
func (s *Service) checkPushed(ctx context.Context, r *repositoryState, commit string) PushCheck {
	check := PushCheck{State: PushUnknown, Branches: []string{}}
	if commit == "" {
		return check
	}
	ctx, cancel := context.WithTimeout(ctx, pushCheckTimeout)
	defer cancel()
	remotes, err := s.git(ctx, r.entry.AnchorPath, "remote")
	if err != nil {
		return check
	}
	if strings.TrimSpace(string(remotes)) == "" {
		check.State = PushNoRemote
		return check
	}
	out, err := s.git(ctx, r.entry.AnchorPath, "for-each-ref", "--contains", commit, "--format=%(refname)", "refs/remotes")
	if err != nil {
		return check
	}
	var names []string
	for _, line := range strings.Split(string(out), "\n") {
		name := strings.TrimPrefix(strings.TrimSpace(line), "refs/remotes/")
		// A remote's HEAD only mirrors one of its branches.
		if name == "" || strings.HasSuffix(name, "/HEAD") {
			continue
		}
		names = append(names, name)
	}
	if len(names) == 0 {
		check.State = PushUnpushed
		return check
	}
	slices.Sort(names)
	check.State = PushPushed
	if len(names) > maxPushBranches {
		check.More = len(names) - maxPushBranches
		names = names[:maxPushBranches]
	}
	check.Branches = names
	return check
}
