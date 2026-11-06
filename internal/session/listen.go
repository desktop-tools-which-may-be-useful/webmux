package session

import (
	"cmp"
	"net/netip"
	"slices"
)

// ListeningPort is a TCP port a session's process tree listens on (loopback
// or wildcard).
type ListeningPort struct {
	Port int
	// Process is the executable base name of the process holding the
	// socket, for example "node"; empty when it cannot be read.
	Process string
}

// localListener reports whether a socket bound to addr accepts connections
// from this machine: a loopback or wildcard address, IPv4-mapped included.
func localListener(addr netip.Addr) bool {
	addr = addr.Unmap()
	return addr.IsLoopback() || addr.IsUnspecified()
}

// listeningPorts turns the sockets found in one process tree, root first,
// into the published list: ascending by port, each port once (IPv4 and IPv6
// sockets share it) under the first process found, without exclude.
func listeningPorts(found []ListeningPort, exclude []int) []ListeningPort {
	var ports []ListeningPort
	for _, p := range found {
		if slices.Contains(exclude, p.Port) || slices.ContainsFunc(ports, func(q ListeningPort) bool { return q.Port == p.Port }) {
			continue
		}
		ports = append(ports, p)
	}
	slices.SortFunc(ports, func(a, b ListeningPort) int { return cmp.Compare(a.Port, b.Port) })
	return ports
}

// scanPorts stores each session's listening ports. Sessions without live
// processes have none. It reads the process table and descriptors, so callers
// must not hold the manager lock.
func (m *Manager) scanPorts(scanner *portScanner, snapshot processSnapshot, list []*Session) {
	roots := make(map[*Session][]int, len(list))
	var pids []int
	for _, s := range list {
		if list := s.rootPIDs(); len(list) > 0 {
			roots[s] = list
			pids = append(pids, list...)
		}
	}
	found := scanner.scan(snapshot, pids)
	for _, s := range list {
		var raw []ListeningPort
		for _, pid := range roots[s] {
			raw = append(raw, found[pid]...)
		}
		ports := listeningPorts(raw, m.options.ExcludePorts)
		s.mu.Lock()
		s.info.ListeningPorts = ports
		s.mu.Unlock()
	}
}
