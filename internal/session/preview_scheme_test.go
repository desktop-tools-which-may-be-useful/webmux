package session

import (
	"bytes"
	"context"
	"testing"
	"time"
)

// A program enables the mode once at startup; a browser attaching after the
// replay buffer dropped those bytes must still be told, and a later change
// must reach attached browsers after the output that made it.
func TestColorSchemeReportsOutliveReplayAndFollowOutput(t *testing.T) {
	t.Parallel()
	script := `printf '\033[?2031h'; head -c 4096 /dev/zero | tr '\0' x; IFS= read -r _; printf '\033[?2031l'; exec sleep 30`
	m := NewManager(Options{ScrollbackBytes: 256, DefaultCwd: t.TempDir()})
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 7*time.Second)
		defer cancel()
		m.Shutdown(ctx)
	})
	s, err := m.Create(CreateInput{Cwd: "", Cmd: []string{"/bin/bash", "-c", script}, Name: "", Env: nil, ConfigID: ""})
	if err != nil {
		t.Fatal(err)
	}
	until := time.Now().Add(5 * time.Second)
	for !bytes.HasSuffix(s.Buffer(), bytes.Repeat([]byte("x"), 256)) {
		if time.Now().After(until) {
			t.Fatal("output did not fill the replay buffer")
		}
		time.Sleep(10 * time.Millisecond)
	}
	if bytes.Contains(s.Buffer(), []byte("2031")) {
		t.Fatal("enabling sequence still replayable")
	}
	sub := attached(s.AttachClient(AttachOptions{Client: "late-browser"}))
	defer s.Detach(sub)
	if m := <-sub.Out; m.Control == nil || !m.Control.ColorSchemeReports {
		t.Fatalf("attach control message: %+v", m)
	}
	queuedKinds(sub)
	if err = s.Write([]byte("\n")); err != nil {
		t.Fatal(err)
	}
	disabled := false
	timeout := time.After(5 * time.Second)
	for {
		select {
		case m := <-sub.Out:
			if len(m.Frame) > 0 && m.Frame[0] == 0 && bytes.Contains(m.Frame, []byte("\x1b[?2031l")) {
				disabled = true
			}
			if m.Control == nil {
				continue
			}
			if !disabled || m.Control.ColorSchemeReports {
				t.Fatalf("control message %+v before the disabling output", m.Control)
			}
			return
		case <-timeout:
			t.Fatal("no control frame after the program disabled reports")
		}
	}
}
