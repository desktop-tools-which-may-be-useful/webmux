package session

import "github.com/rainchan/webmux/internal/procpeek"

// ListeningPortsSupported reports whether Info.ListeningPorts is detected on
// this platform.
const ListeningPortsSupported = procpeek.ListeningPortsSupported

// ListeningPort is a TCP port a session's process tree listens on (loopback
// or wildcard).
type ListeningPort = procpeek.ListeningPort

// scanPorts stores each session's listening ports. Sessions without live
// processes have none. It reads the process table and descriptors, so callers
// must not hold the manager lock.
func (m *Manager) scanPorts(scanner *procpeek.PortScanner, snapshot procpeek.Snapshot, list []*Session) {
	roots := make(map[*Session][]int, len(list))
	var pids []int
	for _, s := range list {
		if list := s.rootPIDs(); len(list) > 0 {
			roots[s] = list
			pids = append(pids, list...)
		}
	}
	found := scanner.Scan(snapshot, pids)
	for _, s := range list {
		var raw []ListeningPort
		for _, pid := range roots[s] {
			raw = append(raw, found[pid]...)
		}
		ports := procpeek.FilterListeningPorts(raw, m.options.ExcludePorts)
		s.mu.Lock()
		s.info.ListeningPorts = ports
		s.mu.Unlock()
	}
}
