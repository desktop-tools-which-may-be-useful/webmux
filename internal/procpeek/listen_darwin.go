//go:build darwin

package procpeek

import (
	"path/filepath"
	"unsafe"

	"golang.org/x/sys/unix"
)

// ListeningPortsSupported reports whether Info.ListeningPorts is detected on
// this platform.
const ListeningPortsSupported = true

const (
	procPIDFDSocketInfo = 3 // PROC_PIDFDSOCKETINFO
	proxFDTypeSocket    = 2 // PROX_FDTYPE_SOCKET
)

// PortScanner reads each tree's sockets through proc_info. macOS has no
// system-wide socket table to tell when nothing changed, so it keeps no state.
type PortScanner struct{}

// Scan returns the listening sockets of each root's process tree, root
// first, keyed by root PID.
func (PortScanner) Scan(snapshot Snapshot, roots []int) map[int][]ListeningPort {
	found := make(map[int][]ListeningPort, len(roots))
	for _, root := range roots {
		var ports []ListeningPort
		for _, pid := range snapshot.Tree(root) {
			ports = append(ports, socketPorts(pid)...)
		}
		found[root] = ports
	}
	return found
}

// socketPorts lists the loopback or wildcard TCP listeners pid holds;
// nothing when its descriptors cannot be read.
func socketPorts(pid int) []ListeningPort {
	fds := make([]procFDInfo, maxListedFileDescriptor)
	size := uintptr(len(fds)) * unsafe.Sizeof(fds[0])
	n, _, errno := unix.Syscall6(unix.SYS_PROC_INFO, procInfoCallPIDInfo, uintptr(pid), procPIDListFDs, 0, uintptr(unsafe.Pointer(&fds[0])), size) //nolint:staticcheck // see Processes
	if errno != 0 {
		return nil
	}
	var ports []ListeningPort
	process, named := "", false
	for _, fd := range fds[:n/unsafe.Sizeof(fds[0])] {
		if fd.fdtype != proxFDTypeSocket {
			continue
		}
		var info [xnuSocketFDInfoSize]byte
		got, _, errno := unix.Syscall6(unix.SYS_PROC_INFO, procInfoCallPIDFDInfo, uintptr(pid), procPIDFDSocketInfo, uintptr(fd.fd), uintptr(unsafe.Pointer(&info[0])), uintptr(len(info))) //nolint:staticcheck // see Processes
		if errno != 0 || got != uintptr(len(info)) {
			continue
		}
		port, ok := parseSocketFDInfo(info[:])
		if !ok {
			continue
		}
		if !named {
			if path := procPath(pid); path != "" {
				process = filepath.Base(path)
			}
			named = true
		}
		ports = append(ports, ListeningPort{Port: port, Process: process})
	}
	return ports
}
