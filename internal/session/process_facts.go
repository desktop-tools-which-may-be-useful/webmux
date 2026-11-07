package session

import "github.com/rainchan/webmux/internal/procpeek"

// ProcessFacts describes a process only for agent adapters validating their
// own session registration records. It is never part of Info, logs or pushes.
type ProcessFacts = procpeek.ProcessFacts

// LookupProcess returns verifiable facts for pid, or false when it exited, is
// a zombie, or its process data cannot be read by the service user.
func LookupProcess(pid int) (ProcessFacts, bool) { return procpeek.LookupProcess(pid) }

// ProcessTree returns pid and its descendant processes, each once. Threads
// are not processes and are not listed.
func ProcessTree(pid int) []int { return procpeek.ProcessTree(pid) }

// RunProcesses returns the current run's root and direct children. Adapters
// select their native executable when a launcher is the root. No result is
// returned without a live, fully readable root process.
func (s *Session) RunProcesses() []ProcessFacts {
	return procpeek.RunProcesses(s.runPID())
}
