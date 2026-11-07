//go:build !linux && !darwin

package session

import "github.com/creack/pty"

func (s *Session) resizePTYLocked(cols, rows uint16) error {
	if s.run == nil {
		return ErrNoRun
	}
	return pty.Setsize(s.run.terminal, &pty.Winsize{Cols: cols, Rows: rows})
}
