//go:build linux || darwin

package session

import (
	"syscall"
	"time"

	"github.com/rainchan/webmux/internal/procpeek"
)

// Find every session process by terminal session, process tree, and the
// inherited environment marker, so separate job-control groups are covered too.
// An empty id skips the marker: a shell's sidecars share the marker with its
// PTY run, so stopping a sidecar must follow only its own process tree. A
// run's cleanup does match the marker and so also ends the shell's sidecars.
func cleanupProcesses(pid int, id string) {
	table := procpeek.Processes()
	reapAdopted(table)
	targets := make(map[int]uint64)
	var marker []byte
	if id != "" {
		marker = []byte("WEBMUX_SESSION_ID=" + id)
	}
	for child, state := range table {
		owned := state.SID == pid
		if !owned && marker != nil && state.State != 'Z' {
			owned = procpeek.SessionMarked(child, marker)
		}
		if owned {
			targets[child] = state.Start
		}
	}
	signalTargets(targets, table, syscall.SIGHUP)
	start := time.Now()
	ticker := time.NewTicker(50 * time.Millisecond)
	defer ticker.Stop()
	for {
		table = procpeek.Processes()
		reapAdopted(table)
		// Shell job-control groups share a session ID; include new descendants too.
		for child, state := range table {
			if state.SID == pid {
				targets[child] = state.Start
			}
		}
		for changed := true; changed; {
			changed = false
			for child, state := range table {
				if _, known := targets[child]; known {
					continue
				}
				if parentStart, owned := targets[state.PPID]; owned && table[state.PPID].Start == parentStart {
					targets[child] = state.Start
					changed = true
				}
			}
		}
		for child, born := range targets {
			state, exists := table[child]
			if !exists || state.Start != born {
				delete(targets, child)
				continue
			}
		}
		if len(targets) == 0 {
			return
		}
		if time.Since(start) >= 5*time.Second {
			signalTargets(targets, table, syscall.SIGKILL)
		}
		if time.Since(start) >= 7*time.Second {
			return
		}
		<-ticker.C
	}
}
func signalTargets(targets map[int]uint64, table procpeek.Table, signal syscall.Signal) {
	for pid, born := range targets {
		if state, exists := table[pid]; exists && state.Start == born && state.State != 'Z' {
			_ = syscall.Kill(pid, signal)
		}
	}
}
func forceSession(pid int, _ string) {
	table := procpeek.Processes()
	targets := make(map[int]uint64)
	for child, state := range table {
		if state.SID == pid {
			targets[child] = state.Start
		}
	}
	signalTargets(targets, table, syscall.SIGKILL)
	_ = syscall.Kill(-pid, syscall.SIGKILL)
}
