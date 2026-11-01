package agentprofile

import (
	"path/filepath"
	"strconv"
	"strings"
)

// Signals are the bounded identity fields a session collected from its
// terminal output and process table. They are agent-neutral; a profile
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

// Identity is a profile's interpretation of Signals.
type Identity struct {
	Agent          string
	Version        string
	AgentSessionID string
	Cwd            string
}

// opencodeSessionPrefix marks OpenCode session IDs. OpenCode borrows the
// 7501 child record id for its session; only this profile reads it.
const opencodeSessionPrefix = "ses_"

// Identify maps signals to the agent that produced them. ok is false when
// no profile recognises the signals.
func Identify(s Signals) (Identity, bool) {
	switch {
	case s.WarpAgent == OMP || s.ProgramApp == OMP:
		return Identity{Agent: OMP, Version: s.WarpPluginVersion, AgentSessionID: s.WarpSessionID, Cwd: s.WarpCwd}, true
	case s.ProgramApp == OpenCode:
		id := ""
		if strings.HasPrefix(s.ProgramChildID, opencodeSessionPrefix) {
			id = s.ProgramChildID
		}
		return Identity{Agent: OpenCode, AgentSessionID: id, Cwd: s.RootCwd}, true
	case s.ProgramApp == Codex:
		// Codex names its session in no signal: see LockedSessions.
		return Identity{Agent: Codex, Cwd: s.RootCwd}, true
	}
	return Identity{}, false
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

// TracksLocks reports that the agent's session IDs come from the writer
// locks its processes hold open (LockedSessions) rather than its signals.
func (p *Profile) TracksLocks() bool { return p.locks != nil }

// LockedSessions returns the agent session IDs whose writer locks are among
// paths, the open files of the terminal UI's processes, in paths order.
// getenv reads the session's environment; userHome is the service user's
// home directory. The lock directory is compared with symbolic links
// resolved, as the process table reports open files.
func (p *Profile) LockedSessions(paths []string, getenv func(string) string, userHome string) []string {
	l := p.locks
	if l == nil {
		return nil
	}
	home := getenv(l.homeEnv)
	if home == "" {
		if userHome == "" {
			return nil
		}
		home = filepath.Join(userHome, l.homeDefault)
	}
	dir := filepath.Join(home, l.dir)
	if resolved, err := filepath.EvalSymlinks(dir); err == nil {
		dir = resolved
	}
	var ids []string
	for _, path := range paths {
		if filepath.Dir(path) != dir {
			continue
		}
		id, ok := strings.CutSuffix(filepath.Base(path), l.suffix)
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
