package session

import (
	"slices"

	"github.com/rainchan/webmux/internal/procpeek"
)

// Identity is what a session knows about the program in its current run:
// bounded, printable identity fields from its terminal output and its PTY's
// root process. It is agent-neutral; internal/agentprofile interprets it.
// It lives in memory only: it is never part of Info, logged or pushed, and
// it is cleared when a run ends or a new one starts.
type Identity struct {
	// Fields of Warp cli-agent events.
	WarpAgent         string
	WarpPluginVersion string
	WarpSessionID     string
	WarpCwd           string
	// ProgramApp is the OSC 7501 root record's app.
	ProgramApp string
	// ProgramChildID is the first segment of the latest OSC 7501 child
	// record id.
	ProgramChildID string
	// PID, Executable and Cwd describe the current run's root process. They
	// are empty without a live run; Executable and Cwd are also empty when
	// the process table cannot be read.
	PID        int
	Executable string
	Cwd        string
}

// mergeSignals keeps every signal field a report carried; the rest stay.
func (i *Identity) mergeSignals(report Identity) {
	set := func(to *string, from string) {
		if from != "" {
			*to = from
		}
	}
	set(&i.WarpAgent, report.WarpAgent)
	set(&i.WarpPluginVersion, report.WarpPluginVersion)
	set(&i.WarpSessionID, report.WarpSessionID)
	set(&i.WarpCwd, report.WarpCwd)
	set(&i.ProgramApp, report.ProgramApp)
	set(&i.ProgramChildID, report.ProgramChildID)
}

// Identity returns the identity of the session's current run. The root
// process is looked up on each call, outside the session lock.
func (s *Session) Identity() Identity {
	s.mu.Lock()
	identity := s.identity
	pid := s.runPIDLocked()
	s.mu.Unlock()
	if pid > 0 {
		identity.PID = pid
		identity.Executable, identity.Cwd = procpeek.RootProcess(pid)
	}
	return identity
}

// runPIDLocked is the current run's root process ID, 0 without a live run.
func (s *Session) runPIDLocked() int {
	if r := s.run; r != nil && !r.ended() {
		return r.cmd.Process.Pid
	}
	return 0
}

func (s *Session) runPID() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.runPIDLocked()
}

// rootPIDs lists the live process roots of the session, sorted ascending:
// the current PTY run's root and any live sidecars.
func (s *Session) rootPIDs() []int {
	s.mu.Lock()
	defer s.mu.Unlock()
	var pids []int
	if pid := s.runPIDLocked(); pid > 0 {
		pids = append(pids, pid)
	}
	for sidecar := range s.sidecars {
		if sidecar.ExitCode() < 0 {
			pids = append(pids, sidecar.pid)
		}
	}
	slices.Sort(pids)
	return pids
}

// RunChildren returns the executable base names of the current run's root
// process's direct children, read on demand for profiles that allow a
// launcher as the root. Empty without a live run or a readable process
// table.
func (s *Session) RunChildren() []string {
	if pid := s.runPID(); pid > 0 {
		return procpeek.RunChildren(pid)
	}
	return nil
}

// RunOpenFiles returns the absolute paths of the regular files the current
// run's root process and its descendants hold open (at most 4096), read on
// demand for profiles that name sessions by the files their agent keeps
// open. Empty without a live run or when descriptors cannot be read. The
// paths are never logged or published.
func (s *Session) RunOpenFiles() []string {
	if pid := s.runPID(); pid > 0 {
		return procpeek.RunOpenFiles(pid)
	}
	return nil
}

// Identity returns the identity of session id's current run.
func (m *Manager) Identity(id string) (Identity, error) {
	s, err := m.Get(id)
	if err != nil {
		return Identity{}, err
	}
	return s.Identity(), nil
}
