package session

import (
	"os"

	"github.com/rainchan/webmux/internal/procpeek"
)

// ContainsProcess reports whether pid runs inside a WebMux session: on Linux
// it carries a WEBMUX_SESSION_ID marker, or one of its ancestors is a session
// leader, a member of a session's terminal session, or this server itself
// (the Linux subreaper adopts orphaned session descendants). macOS cannot
// read other processes' environments, so only the process tree counts there.
func (m *Manager) ContainsProcess(pid int) bool {
	_, inside := m.ProcessSession(pid)
	return inside
}

// ProcessSession reports which session pid runs in. id is empty when pid is
// inside some session but its session cannot be named, such as an orphan the
// server adopted, or when the marked session no longer exists.
func (m *Manager) ProcessSession(pid int) (id string, inside bool) {
	if marker := procpeek.SessionMarker(pid); marker != "" {
		if _, err := m.Get(marker); err == nil {
			return marker, true
		}
		return "", true
	}
	leaders := make(map[int]string)
	m.mu.Lock()
	sessions := make([]*Session, 0, len(m.sessions))
	for _, s := range m.sessions {
		sessions = append(sessions, s)
	}
	m.mu.Unlock()
	for _, s := range sessions {
		s.mu.Lock()
		if s.run != nil && s.run.cmd != nil && s.run.cmd.Process != nil {
			leaders[s.run.cmd.Process.Pid] = s.info.ID
		}
		for sidecar := range s.sidecars {
			leaders[sidecar.pid] = s.info.ID
		}
		s.mu.Unlock()
	}
	return procpeek.TreeOwner(procpeek.Processes(), pid, os.Getpid(), leaders)
}
