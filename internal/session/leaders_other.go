//go:build !linux

package session

import (
	"os"
	"os/exec"

	"github.com/creack/pty"
	"github.com/rainchan/webmux/internal/procpeek"
)

func startPTY(cmd *exec.Cmd, size *pty.Winsize) (*os.File, error) {
	return pty.StartWithSize(cmd, size)
}
func releaseLeader(_ int)          {}
func reapAdopted(_ procpeek.Table) {}

func startChild(cmd *exec.Cmd) error { return cmd.Start() }
