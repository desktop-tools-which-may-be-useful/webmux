package procpeek

// ProcessFacts describes a process only for agent adapters validating their
// own session registration records. It is never part of Info, logs or pushes.
type ProcessFacts struct {
	PID        int
	Executable string
	Cwd        string
	Start      string
	Domain     string
}

// LookupProcess returns verifiable facts for pid, or false when it exited, is
// a zombie, or its process data cannot be read by the service user.
func LookupProcess(pid int) (ProcessFacts, bool) { return processFacts(pid) }

// RunProcesses returns pid and its direct children. Adapters select their
// native executable when a launcher is the root. No result is returned
// without a live, fully readable root process.
func RunProcesses(pid int) []ProcessFacts {
	if pid <= 0 {
		return nil
	}
	root, ok := processFacts(pid)
	if !ok {
		return nil
	}
	facts := []ProcessFacts{root}
	snapshot := NewSnapshot(Processes())
	for _, child := range snapshot.Children(pid) {
		if process, ok := processFacts(child); ok {
			facts = append(facts, process)
		}
	}
	return facts
}
