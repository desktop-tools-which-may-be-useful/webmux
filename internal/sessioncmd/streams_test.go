package sessioncmd

import (
	"bytes"
	"github.com/rainchan/webmux/internal/contract"
	"strings"
	"testing"
	"time"
)

func TestStripper(t *testing.T) {
	for _, tc := range []struct{ in, want string }{
		{"plain\n", "plain\n"},
		{"crlf\r\nnext\n", "crlf\nnext\n"},
		{"pty\r\r\nline\n", "pty\nline\n"},
		{"progress 10%\rprogress 99%\rdone\n", "done\n"},
		{"\x1b[31mred\x1b[0m \x1b[1;2Hmoved\n", "red moved\n"},
		{"title\x1b]0;window\x07 osc\x1b]8;;http://x\x1b\\link\x1b]8;;\x1b\\\n", "title osclink\n"},
		{"dcs\x1bPq#0;2\x1b\\ done\n", "dcs done\n"},
		{"abc\bd\n", "abd\n"},
		{"\x1b(Bcharset\x1bcreset\n", "charsetreset\n"},
		{"tab\there\x07bell\n", "tab\therebell\n"},
		{"中文\b字\n", "中字\n"},
		{"unfinished", "unfinished"},
	} {
		var out bytes.Buffer
		s := newStripper(&out)
		// Byte by byte: state carries across writes.
		for i := range len(tc.in) {
			_, _ = s.Write([]byte{tc.in[i]})
		}
		_ = s.Flush()
		if out.String() != tc.want {
			t.Errorf("%q: got %q, want %q", tc.in, out.String(), tc.want)
		}
	}
}

func TestParseStates(t *testing.T) {
	states, err := parseStates("idle, attention")
	if err != nil || strings.Join(states, ",") != "idle,attention" {
		t.Fatalf("%v %v", states, err)
	}
	if _, err := parseStates("idle,done"); err == nil {
		t.Fatal("unknown state accepted")
	}
}

// A --changed wait must see a session return to its subscription state after
// leaving it: a shell at its prompt is waiting, runs a command, and is
// waiting again.
func TestMatchingChangedAcceptsRoundTrip(t *testing.T) {
	w := &sessionWatch{sessions: map[string]contract.SessionInfo{}}
	opts := waitOptions{ids: []string{"a"}, states: []string{"waiting"}, changed: true}
	baseline := map[string]string{"a": string(contract.SessionStatusWaiting)}
	set := func(status contract.SessionStatus) {
		w.sessions["a"] = contract.SessionInfo{ID: "a", Status: status}
	}
	set(contract.SessionStatusWaiting)
	if len(w.matching(opts, baseline)) != 0 {
		t.Fatal("the subscription state matched without a change")
	}
	set(contract.SessionStatusRunning)
	if len(w.matching(opts, baseline)) != 0 {
		t.Fatal("a state outside --until matched")
	}
	set(contract.SessionStatusWaiting)
	if len(w.matching(opts, baseline)) != 1 {
		t.Fatal("returning to the subscription state after a change did not match")
	}
}

func TestSuggestBranchAvoidsTakenNames(t *testing.T) {
	now := time.Date(2026, 11, 5, 18, 4, 5, 0, time.UTC)
	taken := map[string]bool{"wt/20261105-180405": true, "wt/20261105-180405-2": true}
	if got := suggestBranch(taken, now); got != "wt/20261105-180405-3" {
		t.Fatalf("got %s", got)
	}
}

// Server paths stay POSIX on every client OS.
func TestServerPathHelpers(t *testing.T) {
	for _, tc := range []struct {
		root, dir string
		in        bool
	}{
		{"/w/app", "/w/app", true},
		{"/w/app", "/w/app/src", true},
		{"/w/app", "/w/app2", false},
		{"/w/app", "/w", false},
		{"/w/app/", "/w/app/x/../y", true},
		{"/", "/anything", true},
		{"", "/w", false},
	} {
		if got := within(tc.root, tc.dir); got != tc.in {
			t.Errorf("within(%q, %q) = %v", tc.root, tc.dir, got)
		}
	}
	repo := contract.Repository{AnchorPath: "/home/u/src/app"}
	if got := defaultWorktreePath(repo, "feat/x"); got != "/home/u/src/app.worktrees/x" {
		t.Errorf("default worktree path %q", got)
	}
	if !isDriveLetter("C") || isDriveLetter("repo") || isDriveLetter("1") {
		t.Error("drive letters")
	}
}
