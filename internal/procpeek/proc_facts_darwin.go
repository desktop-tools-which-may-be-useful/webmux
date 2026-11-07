//go:build darwin

package procpeek

import (
	"strconv"

	"golang.org/x/sys/unix"
)

// processFacts uses the kernel's microsecond start timestamp. macOS has no
// PID namespace identifier; its stable domain is simply darwin.
func processFacts(pid int) (ProcessFacts, bool) {
	var zero ProcessFacts
	if pid <= 0 {
		return zero, false
	}
	process, err := unix.SysctlKinfoProc("kern.proc.pid", pid)
	if err != nil || process.Proc.P_stat == darwinZombie {
		return zero, false
	}
	exe, cwd := RootProcess(pid)
	if exe == "" || cwd == "" {
		return zero, false
	}
	start := uint64(process.Proc.P_starttime.Sec)*1_000_000 + uint64(process.Proc.P_starttime.Usec)
	return ProcessFacts{PID: pid, Executable: exe, Cwd: cwd, Start: strconv.FormatUint(start, 10), Domain: "darwin"}, true
}
