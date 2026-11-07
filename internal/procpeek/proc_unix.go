//go:build linux || darwin

package procpeek

import (
	"path"
	"slices"
	"strings"
)

// State uses ps letters: R running, S sleeping, Z zombie. Other values, including
// '?' for a process whose run state is unreadable, never count as waiting.
// Timer is a positive nanosleep or sleep(1) wait, not an unread terminal.
type State struct {
	PPID  int
	State byte
	SID   int
	Start uint64
	Timer bool
}

type Table map[int]State

// A probe's immutable index is shared by every session until the next probe.
type Snapshot struct {
	table    Table
	children map[int][]int
}

func NewSnapshot(table Table) Snapshot {
	children := make(map[int][]int, len(table))
	for child, state := range table {
		children[state.PPID] = append(children[state.PPID], child)
	}
	// Ascending children keep tree walks independent of map order.
	for _, list := range children {
		slices.Sort(list)
	}
	return Snapshot{table: table, children: children}
}

// Children returns pid's direct child PIDs in ascending order.
func (p Snapshot) Children(pid int) []int {
	return p.children[pid]
}

// Tree returns pid and its descendants breadth first, each once. pid is
// included even when it is newer than the snapshot.
func (p Snapshot) Tree(pid int) []int {
	pids := []int{pid}
	seen := map[int]bool{pid: true}
	for i := 0; i < len(pids); i++ {
		for _, child := range p.children[pids[i]] {
			// Process snapshots can contain cycles; visit each PID only once.
			if !seen[child] {
				seen[child] = true
				pids = append(pids, child)
			}
		}
	}
	return pids
}

func (p Snapshot) ForegroundBlocked(pid int) bool {
	root, ok := p.table[pid]
	if !ok {
		return false
	}
	if root.State == 'R' {
		return false
	}
	seen := map[int]bool{pid: true}
	pending := []int{pid}
	leaves := 0
	for len(pending) > 0 {
		member := pending[len(pending)-1]
		pending = pending[:len(pending)-1]
		state := p.table[member]
		if state.State == 'R' {
			return false
		}
		if len(p.children[member]) == 0 {
			leaves++
			if state.State != 'S' {
				return false
			}
		}
		for _, child := range p.children[member] {
			// Process snapshots can contain cycles; visit each PID only once.
			if !seen[child] {
				seen[child] = true
				pending = append(pending, child)
			}
		}
	}
	return leaves > 0
}

// AwaitingInput is the user-attention half of a quiescent tree. Confirmed
// timer leaves (sleep, nanosleep) are waiting on the clock, not the keyboard.
func (p Snapshot) AwaitingInput(pid int) bool {
	if !p.ForegroundBlocked(pid) {
		return false
	}
	return !p.timerLeaves(pid)
}

func (p Snapshot) timerLeaves(pid int) bool {
	seen := map[int]bool{pid: true}
	pending := []int{pid}
	leaves := 0
	for len(pending) > 0 {
		member := pending[len(pending)-1]
		pending = pending[:len(pending)-1]
		if len(p.children[member]) == 0 {
			leaves++
			if !enrichTimer(member, p.table[member]).Timer {
				return false
			}
		}
		for _, child := range p.children[member] {
			if !seen[child] {
				seen[child] = true
				pending = append(pending, child)
			}
		}
	}
	return leaves > 0
}

func cutCString(s string) string {
	if i := strings.IndexByte(s, 0); i >= 0 {
		s = s[:i]
	}
	return strings.TrimRight(s, " ")
}

func sleepCommand(name string) bool {
	return path.Base(cutCString(name)) == "sleep"
}

func timerWait(name string) bool {
	name = cutCString(name)
	if name == "" || name == "0" {
		return false
	}
	// Darwin e_wmesg is 8 bytes with WMESGLEN 7, so nanosleep is stored as "nanosle".
	return strings.Contains(name, "nanosleep") || strings.HasPrefix(name, "nanoslee") || name == "nanosle"
}
