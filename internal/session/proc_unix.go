//go:build linux || darwin

package session

import (
	"os"

	"golang.org/x/sys/unix"
)

// resizePTYLocked pins the PTY descriptor against concurrent close/reuse.
// TIOCSWINSZ lets the kernel notify only this terminal's foreground job.
func (s *Session) resizePTYLocked(cols, rows uint16) error {
	if s.run == nil {
		return ErrNoRun
	}
	if s.run.terminal == nil {
		return os.ErrInvalid
	}
	raw, err := s.run.terminal.SyscallConn()
	if err != nil {
		return err
	}
	var resizeErr error
	err = raw.Control(func(fd uintptr) {
		resizeErr = unix.IoctlSetWinsize(int(fd), unix.TIOCSWINSZ, &unix.Winsize{Col: cols, Row: rows})
	})
	if err != nil {
		return err
	}
	return resizeErr
}
