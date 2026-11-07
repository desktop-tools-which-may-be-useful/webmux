//go:build darwin

package procpeek

import (
	"unsafe"

	"golang.org/x/sys/unix"
)

const (
	procPIDListFDs          = 1 // PROC_PIDLISTFDS
	procInfoCallPIDFDInfo   = 3 // PROC_INFO_CALL_PIDFDINFO
	procPIDFDVnodePathInfo  = 2 // PROC_PIDFDVNODEPATHINFO
	proxFDTypeVnode         = 1 // PROX_FDTYPE_VNODE
	vnodeRegular            = 1 // VREG
	procFileInfoSize        = 24
	vnodeInfoTypeOffset     = 136 // vi_type follows the 136-byte vinfo_stat
	maxListedFileDescriptor = 4096
)

// procFDInfo mirrors struct proc_fdinfo from <sys/proc_info.h>.
type procFDInfo struct {
	fd     int32
	fdtype uint32
}

// vnodeFDInfoWithPath mirrors struct vnode_fdinfowithpath: a proc_fileinfo,
// then a vnode_info and the vnode's path.
type vnodeFDInfoWithPath struct {
	fileInfo [procFileInfoSize]byte
	info     [vnodeInfoSize]byte
	path     [maxPathLen]byte
}

// processOpenFiles lists the regular files pid holds open, by absolute
// path; nothing when its descriptors cannot be read.
func processOpenFiles(pid int) []string {
	fds := make([]procFDInfo, maxListedFileDescriptor)
	size := uintptr(len(fds)) * unsafe.Sizeof(fds[0])
	n, _, errno := unix.Syscall6(unix.SYS_PROC_INFO, procInfoCallPIDInfo, uintptr(pid), procPIDListFDs, 0, uintptr(unsafe.Pointer(&fds[0])), size) //nolint:staticcheck // see Processes
	if errno != 0 {
		return nil
	}
	var paths []string
	for _, fd := range fds[:n/unsafe.Sizeof(fds[0])] {
		if fd.fdtype != proxFDTypeVnode {
			continue
		}
		var info vnodeFDInfoWithPath
		got, _, errno := unix.Syscall6(unix.SYS_PROC_INFO, procInfoCallPIDFDInfo, uintptr(pid), procPIDFDVnodePathInfo, uintptr(fd.fd), uintptr(unsafe.Pointer(&info)), unsafe.Sizeof(info)) //nolint:staticcheck // see Processes
		if errno != 0 || got != unsafe.Sizeof(info) {
			continue
		}
		if *(*int32)(unsafe.Pointer(&info.info[vnodeInfoTypeOffset])) != vnodeRegular {
			continue
		}
		if path := cString(info.path[:]); len(path) > 0 && path[0] == '/' {
			paths = append(paths, path)
		}
	}
	return paths
}
