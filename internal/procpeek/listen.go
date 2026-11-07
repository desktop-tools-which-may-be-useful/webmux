package procpeek

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

// FilterListeningPorts turns the sockets found in one process tree, root first,
// into the published list: ascending by port, each port once (IPv4 and IPv6
// sockets share it) under the first process found, without exclude.
func FilterListeningPorts(found []ListeningPort, exclude []int) []ListeningPort {
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
