//go:build linux || darwin

package procpeek

import (
	"fmt"
	"testing"
)

func TestProcessTreeWaiting(t *testing.T) {
	table := Table{1: {PPID: 0, State: 'S'}, 2: {PPID: 1, State: 'S'}, 3: {PPID: 2, State: 'R'}}
	if NewSnapshot(table).ForegroundBlocked(1) {
		t.Fatal("running descendant must prevent waiting")
	}
	table[3] = State{PPID: 2, State: 'S'}
	if !NewSnapshot(table).ForegroundBlocked(1) {
		t.Fatal("all sleeping leaves should wait")
	}
	delete(table, 1)
	if NewSnapshot(table).ForegroundBlocked(1) {
		t.Fatal("missing root must not wait")
	}
}

func TestForegroundBlockedStates(t *testing.T) {
	tests := []struct {
		name  string
		table Table
		want  bool
	}{
		{name: "missing root", table: Table{2: {PPID: 1, State: 'S'}}},
		{name: "sleeping root", table: Table{1: {State: 'S'}}, want: true},
		{name: "running root", table: Table{1: {State: 'R'}, 2: {PPID: 1, State: 'S'}}},
		{name: "unknown root leaf", table: Table{1: {State: '?'}}},
		{name: "unknown internal root", table: Table{1: {State: '?'}, 2: {PPID: 1, State: 'S'}}, want: true},
		{name: "unknown internal descendant", table: Table{1: {State: 'S'}, 2: {PPID: 1, State: '?'}, 3: {PPID: 2, State: 'S'}}, want: true},
		{name: "running internal descendant", table: Table{1: {State: 'S'}, 2: {PPID: 1, State: 'R'}, 3: {PPID: 2, State: 'S'}}},
		{name: "all sleeping leaves", table: Table{1: {State: 'S'}, 2: {PPID: 1, State: 'S'}, 3: {PPID: 1, State: 'S'}}, want: true},
		{name: "unknown leaf", table: Table{1: {State: 'S'}, 2: {PPID: 1, State: 'S'}, 3: {PPID: 1, State: '?'}}},
		{name: "zombie leaf", table: Table{1: {State: 'S'}, 2: {PPID: 1, State: 'Z'}}},
		{name: "unrelated running process", table: Table{1: {State: 'S'}, 2: {PPID: 99, State: 'R'}}, want: true},
		{name: "self cycle", table: Table{1: {PPID: 1, State: 'S'}}},
		{name: "root cycle without leaves", table: Table{1: {PPID: 2, State: 'S'}, 2: {PPID: 1, State: 'S'}}},
		{name: "root cycle with sleeping leaf", table: Table{1: {PPID: 2, State: 'S'}, 2: {PPID: 1, State: 'S'}, 3: {PPID: 2, State: 'S'}}, want: true},
		{name: "unrelated cycle", table: Table{1: {State: 'S'}, 2: {PPID: 3, State: 'S'}, 3: {PPID: 2, State: 'S'}}, want: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := NewSnapshot(tt.table).ForegroundBlocked(1); got != tt.want {
				t.Fatalf("ForegroundBlocked() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestAwaitingInputIgnoresTimerLeaves(t *testing.T) {
	// Timer enrichment may read the OS process table; never collide with PID 1.
	const root = 1 << 30
	parent := Table{root: {State: 'S'}, root + 1: {PPID: root, State: 'S', Timer: true}}
	if NewSnapshot(parent).AwaitingInput(root) {
		t.Fatal("parent waiting on sleep was treated as terminal input")
	}
	if !NewSnapshot(Table{root: {State: 'S'}}).AwaitingInput(root) {
		t.Fatal("unclassified sleep should still count as input wait")
	}
	if NewSnapshot(Table{root: {State: 'S', Timer: true}}).AwaitingInput(root) {
		t.Fatal("sleeping root timer was treated as terminal input")
	}
	mixed := Table{root: {State: 'S'}, root + 1: {PPID: root, State: 'S', Timer: true}, root + 2: {PPID: root, State: 'S'}}
	if !NewSnapshot(mixed).AwaitingInput(root) {
		t.Fatal("non-timer leaf was hidden by a timer sibling")
	}
	if NewSnapshot(Table{root: {State: 'S', Timer: true}, root + 1: {PPID: root, State: 'R'}}).AwaitingInput(root) {
		t.Fatal("running descendant was treated as terminal input")
	}
}

func TestTimerWaitNames(t *testing.T) {
	for _, name := range []string{"hrtimer_nanosleep", "nanosleep", "nanoslee", "nanoslee\x00", "nanosle", "nanosle\x00"} {
		if !timerWait(name) {
			t.Fatalf("%q should be a timer wait", name)
		}
	}
	for _, name := range []string{"", "0", "wait_woken", "poll_schedule_timeout.constprop.0", "ttyin"} {
		if timerWait(name) {
			t.Fatalf("%q should not be a timer wait", name)
		}
	}
}

func TestSleepCommand(t *testing.T) {
	for _, name := range []string{"sleep", "/bin/sleep", "/usr/bin/sleep", "sleep\x00junk", "/bin/sleep\x00"} {
		if !sleepCommand(name) {
			t.Fatalf("%q should be sleep", name)
		}
	}
	for _, name := range []string{"", "bash", "/bin/bash", "sleeping", "/bin/sleeping", "nanosleep"} {
		if sleepCommand(name) {
			t.Fatalf("%q should not be sleep", name)
		}
	}
}

func BenchmarkForegroundBlocked(b *testing.B) {
	for _, shape := range []string{"chain", "wide", "unrelated"} {
		for _, size := range []int{128, 1024, 8192} {
			b.Run(fmt.Sprintf("%s/%d", shape, size), func(b *testing.B) {
				table := make(Table, size)
				table[1] = State{State: 'S'}
				for pid := 2; pid <= size; pid++ {
					parent := 1
					switch shape {
					case "chain":
						parent = pid - 1
					case "unrelated":
						parent = size + 1
					}
					table[pid] = State{PPID: parent, State: 'S'}
				}
				snapshot := NewSnapshot(table)
				b.ReportAllocs()
				b.ResetTimer()
				for i := 0; i < b.N; i++ {
					if !snapshot.ForegroundBlocked(1) {
						b.Fatal("sleeping process tree must be blocked")
					}
				}
			})
		}
	}
}

func BenchmarkProcessSnapshotManySessions(b *testing.B) {
	table := make(Table, 5000)
	for pid := 1; pid <= 5000; pid++ {
		table[pid] = State{PPID: 5000 + pid/3, State: 'S'}
	}
	b.ReportAllocs()
	for b.Loop() {
		snapshot := NewSnapshot(table)
		for pid := 1; pid <= 100; pid++ {
			if !snapshot.ForegroundBlocked(pid) {
				b.Fatal("sleeping leaf must be blocked")
			}
		}
	}
}
