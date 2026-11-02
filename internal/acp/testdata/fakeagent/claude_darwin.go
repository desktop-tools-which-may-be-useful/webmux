//go:build darwin

package main

import (
	"strconv"

	"golang.org/x/sys/unix"
)

func claudeProcessIdentity(pid int) (start, domain string, err error) {
	process, err := unix.SysctlKinfoProc("kern.proc.pid", pid)
	if err != nil {
		return "", "", err
	}
	microseconds := uint64(process.Proc.P_starttime.Sec)*1_000_000 + uint64(process.Proc.P_starttime.Usec)
	return strconv.FormatUint(microseconds, 10), "darwin", nil
}
