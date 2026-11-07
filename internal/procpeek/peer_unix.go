//go:build linux || darwin

package procpeek

// TreeOwner walks pid's ancestors to the session leader they descend from;
// leaders maps leader PIDs to session IDs. A process whose terminal session
// is a leader's session also counts, which covers job-control groups that
// re-parented away from the leader. Reaching the server itself means an
// adopted orphan: inside a session, but which one is unknown.
func TreeOwner(table Table, pid, server int, leaders map[int]string) (string, bool) {
	seen := make(map[int]bool)
	for current := pid; current > 1 && !seen[current]; {
		seen[current] = true
		state, ok := table[current]
		if !ok {
			return "", false
		}
		if id, ok := leaders[current]; ok {
			return id, true
		}
		if id, ok := leaders[state.SID]; ok {
			return id, true
		}
		if current != pid && current == server {
			return "", true
		}
		current = state.PPID
	}
	return "", false
}
