package agentprofile

import (
	"path/filepath"
	"strconv"
	"strings"
)

// Signals are the bounded identity fields a session collected from its
// terminal output and process table. They are agent-neutral; an adapter
// decides which of them identify its sessions.
type Signals struct {
	// Warp cli-agent event fields.
	WarpAgent         string
	WarpPluginVersion string
	WarpSessionID     string
	WarpCwd           string
	// ProgramApp is the OSC 7501 root record's app.
	ProgramApp string
	// ProgramChildID is the first segment of the latest OSC 7501 child
	// record id.
	ProgramChildID string
	// RootExecutable and RootCwd describe the PTY's root process.
	RootExecutable string
	RootCwd        string
}

// Identity is an adapter's interpretation of Signals.
type Identity struct {
	Agent          string
	Version        string
	AgentSessionID string
	Cwd            string
}

// BelowVerified reports whether version is older than the newest release
// measured for the profile. Unparseable versions report false.
func (p *Profile) BelowVerified(version string) bool {
	order, ok := CompareVersions(version, p.VerifiedVersion)
	return ok && order < 0
}

// CompareVersions orders dotted numeric versions, ignoring a leading "v"
// and any pre-release or build suffix. ok is false if either is unparseable.
func CompareVersions(a, b string) (order int, ok bool) {
	pa, okA := parseVersion(a)
	pb, okB := parseVersion(b)
	if !okA || !okB {
		return 0, false
	}
	for i := range max(len(pa), len(pb)) {
		var x, y int
		if i < len(pa) {
			x = pa[i]
		}
		if i < len(pb) {
			y = pb[i]
		}
		if x != y {
			if x < y {
				return -1, true
			}
			return 1, true
		}
	}
	return 0, true
}

func parseVersion(v string) ([]int, bool) {
	v = strings.TrimPrefix(strings.TrimSpace(v), "v")
	if cut := strings.IndexAny(v, "-+ "); cut >= 0 {
		v = v[:cut]
	}
	if v == "" {
		return nil, false
	}
	fields := strings.Split(v, ".")
	parts := make([]int, len(fields))
	for i, field := range fields {
		n, err := strconv.Atoi(field)
		if err != nil || n < 0 {
			return nil, false
		}
		parts[i] = n
	}
	return parts, true
}

// LockLayout is where an agent holds a writer lock for each session it has
// open: <home>/<Dir>/<id><Suffix>, home from environment variable HomeEnv
// or else HomeDefault under the user's home directory.
type LockLayout struct {
	HomeEnv, HomeDefault, Dir, Suffix string
}

// Sessions returns the session IDs whose writer locks are among paths, in
// paths order (see LockTracker). The lock directory is compared with
// symbolic links resolved, as the process table reports open files.
func (l LockLayout) Sessions(paths []string, getenv func(string) string, userHome string) []string {
	home := getenv(l.HomeEnv)
	if home == "" {
		if userHome == "" {
			return nil
		}
		home = filepath.Join(userHome, l.HomeDefault)
	}
	dir := filepath.Join(home, l.Dir)
	if resolved, err := filepath.EvalSymlinks(dir); err == nil {
		dir = resolved
	}
	var ids []string
	for _, path := range paths {
		if filepath.Dir(path) != dir {
			continue
		}
		id, ok := strings.CutSuffix(filepath.Base(path), l.Suffix)
		if ok && sessionLockID(id) {
			ids = append(ids, id)
		}
	}
	return ids
}

// sessionLockID accepts UUID-shaped names: hex digits and dashes, 36 bytes.
// The lock directory also holds other locks (.coordination.lock).
func sessionLockID(id string) bool {
	if len(id) != 36 {
		return false
	}
	for i := 0; i < len(id); i++ {
		c := id[i]
		switch {
		case c == '-', c >= '0' && c <= '9', c >= 'a' && c <= 'f', c >= 'A' && c <= 'F':
		default:
			return false
		}
	}
	return true
}
