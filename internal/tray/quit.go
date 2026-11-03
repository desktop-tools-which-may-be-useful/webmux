package tray

import (
	"errors"
	"flag"
	"io"
	"time"
)

// Exit codes of `webmux-tray quit`.
const (
	QuitDone    = 0 // the running tray has exited
	QuitRunning = 1 // a tray still runs
	QuitUsage   = 2 // invalid arguments
	QuitNone    = 3 // no tray was running
)

// ParseQuit parses the arguments after `quit`: an optional
// `--kill-path PATH`, nothing else.
func ParseQuit(args []string) (killPath string, err error) {
	fs := flag.NewFlagSet("quit", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	fs.StringVar(&killPath, "kill-path", "", "")
	if err := fs.Parse(args); err != nil {
		return "", err
	}
	if fs.NArg() > 0 {
		return "", errors.New("quit takes no positional arguments")
	}
	return killPath, nil
}

// Quitter asks the running tray of this Windows user to exit as with
// "仅退出托盘", then waits for it. Running, Signal and Kill are the Windows
// primitives; the durations default to the documented 10 s and 2 s.
type Quitter struct {
	// Running reports whether a tray of this user still runs.
	Running func() bool
	// Signal sets the tray's quit event; false means the running tray is
	// an older one without the event.
	Signal func() bool
	// Kill terminates this user's trays started from path.
	Kill func(path string) error
	Logf func(format string, args ...any)

	Wait, Grace, Poll time.Duration
}

// Quit returns QuitNone, QuitDone or QuitRunning. With killPath set, trays
// started from it are terminated once Wait passes; trays elsewhere are left
// alone.
func (q Quitter) Quit(killPath string) int {
	if !q.Running() {
		return QuitNone
	}
	if !q.Signal() {
		q.logf("the running tray does not support quit requests")
	}
	if q.until(q.durationOr(q.Wait, 10*time.Second)) {
		return QuitDone
	}
	if killPath == "" {
		return QuitRunning
	}
	if err := q.Kill(killPath); err != nil {
		q.logf("terminate %s: %v", killPath, err)
	}
	if q.until(q.durationOr(q.Grace, 2*time.Second)) {
		return QuitDone
	}
	return QuitRunning
}

// until polls Running for at most d and reports whether the tray is gone.
func (q Quitter) until(d time.Duration) bool {
	poll := q.durationOr(q.Poll, 100*time.Millisecond)
	deadline := time.Now().Add(d)
	for {
		if !q.Running() {
			return true
		}
		if !time.Now().Before(deadline) {
			return false
		}
		time.Sleep(poll)
	}
}

func (q Quitter) durationOr(d, def time.Duration) time.Duration {
	if d > 0 {
		return d
	}
	return def
}

func (q Quitter) logf(format string, args ...any) {
	if q.Logf != nil {
		q.Logf(format, args...)
	}
}
