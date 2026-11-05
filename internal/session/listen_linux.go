//go:build linux

package session

import (
	"encoding/binary"
	"encoding/hex"
	"maps"
	"net/netip"
	"os"
	"slices"
	"strconv"
	"strings"
)

// ListeningPortsSupported reports whether Info.ListeningPorts is detected on
// this platform.
const ListeningPortsSupported = true

// tcpListen is the LISTEN state in /proc/net/tcp{,6}.
const tcpListen = "0A"

// portScanner maps listening sockets to session process trees. It keeps the
// previous scan: unchanged LISTEN sockets and process trees cannot hold
// different ports, so their descriptors are not read again.
type portScanner struct {
	listen map[uint64]int
	trees  map[int][]int
	found  map[int][]ListeningPort
}

// scan returns the listening sockets of each root's process tree, root
// first, keyed by root PID.
func (p *portScanner) scan(snapshot processSnapshot, roots []int) map[int][]ListeningPort {
	listen := make(map[uint64]int)
	for _, name := range []string{"/proc/net/tcp", "/proc/net/tcp6"} {
		// tcp6 is absent when IPv6 is disabled.
		if data, err := os.ReadFile(name); err == nil {
			parseProcNetTCP(data, listen)
		}
	}
	trees := make(map[int][]int, len(roots))
	for _, root := range roots {
		trees[root] = snapshot.tree(root)
	}
	if p.found != nil && maps.Equal(listen, p.listen) && maps.EqualFunc(trees, p.trees, slices.Equal[[]int]) {
		return p.found
	}
	found := make(map[int][]ListeningPort, len(trees))
	if len(listen) > 0 {
		for root, pids := range trees {
			found[root] = socketPorts(pids, listen)
		}
	}
	p.listen, p.trees, p.found = listen, trees, found
	return found
}

// socketPorts lists the sockets in listen that pids hold, in pids order.
func socketPorts(pids []int, listen map[uint64]int) []ListeningPort {
	var ports []ListeningPort
	for _, pid := range pids {
		dir := "/proc/" + strconv.Itoa(pid) + "/fd/"
		entries, err := os.ReadDir(dir)
		if err != nil {
			continue
		}
		process, named := "", false
		for _, entry := range entries {
			link, err := os.Readlink(dir + entry.Name())
			if err != nil {
				continue
			}
			inode, ok := strings.CutPrefix(link, "socket:[")
			if !ok {
				continue
			}
			number, err := strconv.ParseUint(strings.TrimSuffix(inode, "]"), 10, 64)
			if err != nil {
				continue
			}
			port, ok := listen[number]
			if !ok {
				continue
			}
			if !named {
				process, _ = rootProcess(pid)
				named = true
			}
			ports = append(ports, ListeningPort{Port: port, Process: process})
		}
	}
	return ports
}

// parseProcNetTCP adds the inode and port of every LISTEN socket on a
// loopback or wildcard address in a /proc/net/tcp or tcp6 table to listen.
func parseProcNetTCP(data []byte, listen map[uint64]int) {
	for line := range strings.Lines(string(data)) {
		// sl local_address rem_address st tx:rx tr:when retrnsmt uid timeout inode
		fields := strings.Fields(line)
		if len(fields) < 10 || fields[3] != tcpListen {
			continue
		}
		host, portHex, ok := strings.Cut(fields[1], ":")
		if !ok {
			continue
		}
		addr, ok := procNetAddr(host)
		if !ok || !localListener(addr) {
			continue
		}
		port, err := strconv.ParseUint(portHex, 16, 16)
		if err != nil || port == 0 {
			continue
		}
		inode, err := strconv.ParseUint(fields[9], 10, 64)
		if err != nil || inode == 0 {
			continue
		}
		listen[inode] = int(port)
	}
}

// procNetAddr decodes a /proc/net address: the kernel prints each 32-bit
// word of the network-order address as a host-order hex number.
func procNetAddr(s string) (netip.Addr, bool) {
	raw, err := hex.DecodeString(s)
	if err != nil || (len(raw) != 4 && len(raw) != 16) {
		return netip.Addr{}, false
	}
	for i := 0; i < len(raw); i += 4 {
		binary.NativeEndian.PutUint32(raw[i:], binary.BigEndian.Uint32(raw[i:]))
	}
	return netip.AddrFromSlice(raw)
}
