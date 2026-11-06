//go:build linux || darwin

package session

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/creack/pty"
)

// ptySize is the grid the kernel holds for s's terminal and the grid its
// info and preview report; they must agree.
func ptySize(t *testing.T, s *Session) (cols, rows int) {
	t.Helper()
	s.mu.Lock()
	defer s.mu.Unlock()
	size, err := pty.GetsizeFull(s.run.terminal)
	if err != nil {
		t.Fatal(err)
	}
	if int(size.Cols) != int(s.info.Cols) || int(size.Rows) != int(s.info.Rows) {
		t.Fatalf("pty %dx%d, info %dx%d", size.Cols, size.Rows, s.info.Cols, s.info.Rows)
	}
	// The preview screen exists at once for a custom size, and lazily at
	// the default one.
	if screen := s.preview.screen; screen != nil && (screen.Cols() != int(size.Cols) || screen.Rows() != int(size.Rows)) {
		t.Fatalf("preview does not follow the pty size %dx%d", size.Cols, size.Rows)
	}
	return int(size.Cols), int(size.Rows)
}

func sleepSession(t *testing.T, in CreateInput) *Session {
	t.Helper()
	m := shellManager(t)
	in.Cmd = []string{"/bin/sleep", "30"}
	s, err := m.Create(in)
	if err != nil {
		t.Fatal(err)
	}
	return s
}

func TestCreateWithInitialSize(t *testing.T) {
	t.Parallel()
	s := sleepSession(t, CreateInput{Cols: 132, Rows: 43})
	if cols, rows := ptySize(t, s); cols != 132 || rows != 43 {
		t.Fatalf("initial size %dx%d", cols, rows)
	}
	if cols, rows := ptySize(t, sleepSession(t, CreateInput{})); cols != DefaultCols || rows != DefaultRows {
		t.Fatalf("default size %dx%d", cols, rows)
	}
}

func TestCreateRejectsPartialOrOutOfRangeSize(t *testing.T) {
	t.Parallel()
	m := shellManager(t)
	for _, size := range [][2]int{{100, 0}, {0, 30}, {1001, 24}, {80, 1001}, {-1, 24}} {
		_, err := m.Create(CreateInput{Cmd: []string{"/bin/sleep", "30"}, Cols: size[0], Rows: size[1]})
		if !errors.Is(err, ErrInvalid) {
			t.Errorf("size %v: %v", size, err)
		}
	}
	if got := m.List(); len(got) != 0 {
		t.Fatalf("rejected creates left sessions: %d", len(got))
	}
}

// A hybrid session's terminal side starts later than the shell; it must
// still open at the size the session was created with.
func TestShellRunStartsAtInitialSize(t *testing.T) {
	t.Parallel()
	m := shellManager(t)
	s, err := m.CreateShell(ShellInput{CreateInput: CreateInput{Cmd: []string{"agent"}, Cols: 150, Rows: 45}}, newFakeDriver())
	if err != nil {
		t.Fatal(err)
	}
	if err = s.StartRun([]string{"/bin/sleep", "30"}); err != nil {
		t.Fatal(err)
	}
	if cols, rows := ptySize(t, s); cols != 150 || rows != 45 {
		t.Fatalf("terminal run size %dx%d", cols, rows)
	}
}

func TestResizeScriptControlRules(t *testing.T) {
	t.Parallel()
	script := func(s *Session) {
		s.mu.Lock()
		s.control, s.controlSub = &Controller{ClientID: serverClientPrefix + "held", Label: "script"}, nil
		s.mu.Unlock()
	}
	t.Run("nobody controls", func(t *testing.T) {
		t.Parallel()
		s := sleepSession(t, CreateInput{})
		if err := s.ResizeScript(ScriptResize{Cols: 100, Rows: 40}); err != nil {
			t.Fatal(err)
		}
		if cols, rows := ptySize(t, s); cols != 100 || rows != 40 {
			t.Fatalf("size %dx%d", cols, rows)
		}
	})
	t.Run("a script controls", func(t *testing.T) {
		t.Parallel()
		s := sleepSession(t, CreateInput{})
		script(s)
		if err := s.ResizeScript(ScriptResize{Cols: 120, Rows: 41}); err != nil {
			t.Fatal(err)
		}
		if cols, rows := ptySize(t, s); cols != 120 || rows != 41 {
			t.Fatalf("size %dx%d", cols, rows)
		}
		s.mu.Lock()
		held := s.control != nil && s.control.Label == "script"
		s.mu.Unlock()
		if !held {
			t.Fatal("a resize by the controlling script released its control")
		}
	})
	t.Run("a browser controls", func(t *testing.T) {
		t.Parallel()
		s := sleepSession(t, CreateInput{})
		browser := attached(s.AttachClient(AttachOptions{Client: "browser"}))
		defer s.Detach(browser)
		if err := s.ResizeFrom(browser, 90, 30); err != nil {
			t.Fatal(err)
		}
		if err := s.ResizeScript(ScriptResize{Cols: 100, Rows: 40}); !errors.Is(err, ErrNotController) {
			t.Fatalf("resize against a browser controller: %v", err)
		}
		if cols, rows := ptySize(t, s); cols != 90 || rows != 30 {
			t.Fatalf("a refused resize changed the size to %dx%d", cols, rows)
		}
	})
	t.Run("take", func(t *testing.T) {
		t.Parallel()
		s := sleepSession(t, CreateInput{})
		browser := attached(s.AttachClient(AttachOptions{Client: "browser"}))
		defer s.Detach(browser)
		if err := s.ResizeScript(ScriptResize{Cols: 100, Rows: 40, Take: true, Label: "script · abcd2345"}); err != nil {
			t.Fatal(err)
		}
		if cols, rows := ptySize(t, s); cols != 100 || rows != 40 {
			t.Fatalf("size %dx%d", cols, rows)
		}
		// The browser saw the script hold control at the new grid, then
		// control released; it no longer controls, so it cannot resize.
		heldNewGrid, released := false, false
		for !released {
			m := nextMessage(t, browser)
			switch {
			case m.Control == nil:
			case m.Control.Controller == nil:
				released = heldNewGrid
			case m.Control.Controller.Label == "script · abcd2345":
				heldNewGrid = heldNewGrid || (m.Control.Cols == 100 && m.Control.Rows == 40)
			}
		}
		if err := s.ResizeFrom(browser, 80, 24); !errors.Is(err, ErrNotController) {
			t.Fatalf("browser resize after the take: %v", err)
		}
	})
}

func TestResizeScriptRejectsInvalidSize(t *testing.T) {
	t.Parallel()
	s := sleepSession(t, CreateInput{})
	for _, size := range [][2]int{{0, 24}, {80, 0}, {1001, 24}, {80, 1001}, {-5, 24}} {
		if err := s.ResizeScript(ScriptResize{Cols: size[0], Rows: size[1]}); !errors.Is(err, ErrInvalid) {
			t.Errorf("size %v: %v", size, err)
		}
	}
	if cols, rows := ptySize(t, s); cols != DefaultCols || rows != DefaultRows {
		t.Fatalf("a refused resize changed the size to %dx%d", cols, rows)
	}
}

func TestResizeScriptNeedsALiveTerminalRun(t *testing.T) {
	t.Parallel()
	m := shellManager(t)
	chat, err := m.CreateShell(ShellInput{CreateInput: CreateInput{Cmd: []string{"agent"}}}, newFakeDriver())
	if err != nil {
		t.Fatal(err)
	}
	if err = chat.ResizeScript(ScriptResize{Cols: 100, Rows: 40}); !errors.Is(err, ErrNoRun) {
		t.Fatalf("shell without a terminal run: %v", err)
	}
	s := sleepSession(t, CreateInput{})
	s.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	s.awaitProcesses(ctx)
	for s.Info().ExitCode == nil {
		if ctx.Err() != nil {
			t.Fatal("session did not exit")
		}
		time.Sleep(10 * time.Millisecond)
	}
	if err = s.ResizeScript(ScriptResize{Cols: 100, Rows: 40}); !errors.Is(err, ErrExited) {
		t.Fatalf("exited session: %v", err)
	}
}
