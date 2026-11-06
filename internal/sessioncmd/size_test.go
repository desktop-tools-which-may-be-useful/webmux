//go:build linux || darwin

package sessioncmd

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/coder/websocket"
	"github.com/rainchan/webmux/internal/contract"
	"github.com/rainchan/webmux/internal/localclient"
)

// sizeServer serves a terminal of 40x5 whose replay is output, and records
// resize requests.
type sizeServer struct {
	output string
	mu     sync.Mutex
	resize []contract.SessionResizeRequest
	status int
}

func (s *sizeServer) handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /ws/s/{id}", func(w http.ResponseWriter, r *http.Request) {
		c, err := websocket.Accept(w, r, &websocket.AcceptOptions{Subprotocols: []string{"webmux"}})
		if err != nil {
			return
		}
		defer func() { _ = c.CloseNow() }()
		_ = c.Write(r.Context(), websocket.MessageBinary, append([]byte{contract.FrameState}, `{"cols":40,"rows":5}`...))
		_ = c.Write(r.Context(), websocket.MessageBinary, append([]byte{contract.FrameSnapshot}, s.output...))
		_, _, _ = c.Read(r.Context())
	})
	mux.HandleFunc("POST /api/sessions/{id}/resize", func(w http.ResponseWriter, r *http.Request) {
		var req contract.SessionResizeRequest
		_ = json.NewDecoder(r.Body).Decode(&req)
		s.mu.Lock()
		s.resize = append(s.resize, req)
		status := s.status
		s.mu.Unlock()
		if status != 0 {
			w.WriteHeader(status)
			_, _ = io.WriteString(w, `{"error":"connection does not control the terminal (a browser or attached client controls it)"}`)
			return
		}
		_, _ = io.WriteString(w, `{"id":"abcdefgh","cols":120,"rows":40}`)
	})
	return mux
}

func (s *sizeServer) env(t *testing.T) (Env, *bytes.Buffer, *bytes.Buffer) {
	t.Helper()
	dir, err := os.MkdirTemp("/tmp", "scm")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(dir) })
	socket := filepath.Join(dir, "s.sock")
	l, err := net.Listen("unix", socket)
	if err != nil {
		t.Fatal(err)
	}
	srv := &http.Server{Handler: s.handler()}
	go func() { _ = srv.Serve(l) }()
	t.Cleanup(func() { _ = srv.Close() })
	var stdout, stderr bytes.Buffer
	return Env{
		Connect: func() (*localclient.Client, error) { return localclient.New(localclient.UnixDialer{Path: socket}), nil },
		Stdout:  &stdout, Stderr: &stderr, NotRunningHint: "start it",
	}, &stdout, &stderr
}

// thirty is thirty numbered lines: six times the screen of the test terminal.
func thirty() string {
	var b strings.Builder
	for i := 1; i <= 30; i++ {
		fmt.Fprintf(&b, "line %d\r\n", i)
	}
	return b.String()
}

func screenLines(t *testing.T, e Env, stdout *bytes.Buffer, args ...string) []string {
	t.Helper()
	stdout.Reset()
	if code := e.Run(context.Background(), screenCommand, append([]string{"abcdefgh"}, args...)); code != contract.ExitOK {
		t.Fatalf("screen %v: exit %d", args, code)
	}
	return strings.Split(strings.TrimRight(stdout.String(), "\n"), "\n")
}

func TestScreenScrollbackPrintsHistoryAboveTheScreen(t *testing.T) {
	e, stdout, stderr := (&sizeServer{output: thirty()}).env(t)

	// Without the flag only the 5-row screen is printed.
	if got := screenLines(t, e, stdout); len(got) != 4 || got[0] != "line 27" || got[3] != "line 30" {
		t.Fatalf("default screen %q", got)
	}
	got := screenLines(t, e, stdout, "--scrollback", "200")
	if len(got) != 30 || got[0] != "line 1" || got[29] != "line 30" {
		t.Fatalf("full history %q", got)
	}
	// The history is bounded by N lines above the screen.
	got = screenLines(t, e, stdout, "--scrollback", "10")
	if len(got) != 14 || got[0] != "line 17" || got[13] != "line 30" {
		t.Fatalf("10 lines of history %q", got)
	}

	stdout.Reset()
	if code := e.Run(context.Background(), screenCommand, []string{"abcdefgh", "--json", "--scrollback", "200"}); code != contract.ExitOK {
		t.Fatalf("screen --json: exit %d %q", code, stderr)
	}
	var view screenView
	if err := json.Unmarshal(stdout.Bytes(), &view); err != nil {
		t.Fatal(err)
	}
	if view.Cols != 40 || view.Rows != 5 || len(view.Lines) != 30 || view.Lines[0] != "line 1" || view.AltScreen {
		t.Fatalf("json %+v", view)
	}
}

// A program on the alternate screen has no scroll history, whatever the
// history of the shell it was started from.
func TestScreenScrollbackHasNoHistoryOnTheAlternateScreen(t *testing.T) {
	e, stdout, _ := (&sizeServer{output: thirty() + "\x1b[?1049h\x1b[Halt text"}).env(t)
	got := screenLines(t, e, stdout, "--scrollback", "200")
	if len(got) != 1 || got[0] != "alt text" {
		t.Fatalf("alternate screen with scrollback %q", got)
	}
}

func TestScreenRejectsScrollbackOutOfRange(t *testing.T) {
	e, _, _ := (&sizeServer{output: "x"}).env(t)
	for _, n := range []string{"-1", "100001"} {
		if code := e.Run(context.Background(), screenCommand, []string{"abcdefgh", "--scrollback", n}); code != contract.ExitUsage {
			t.Errorf("--scrollback %s: exit %d", n, code)
		}
	}
}

func TestResizeSendsTheGridAndTake(t *testing.T) {
	server := &sizeServer{}
	e, _, stderr := server.env(t)
	for _, args := range [][]string{{"abcdefgh", "120x40"}, {"abcdefgh", "200X50", "--take"}} {
		if code := e.Run(context.Background(), resizeCommand, args); code != contract.ExitOK {
			t.Fatalf("resize %v: exit %d %q", args, code, stderr)
		}
	}
	want := []contract.SessionResizeRequest{{Cols: 120, Rows: 40}, {Cols: 200, Rows: 50, Take: true}}
	if len(server.resize) != 2 || server.resize[0] != want[0] || server.resize[1] != want[1] {
		t.Fatalf("requests %+v", server.resize)
	}
}

func TestResizeUsageErrorsSendNothing(t *testing.T) {
	server := &sizeServer{}
	e, _, _ := server.env(t)
	for _, args := range [][]string{
		{"abcdefgh"}, {"abcdefgh", "120"}, {"abcdefgh", "0x40"}, {"abcdefgh", "120x0"}, {"abcdefgh", "1001x40"},
		{"abcdefgh", "axb"}, {"abcdefgh", "-5x40"}, {"abcdefgh", "120x40", "extra"},
	} {
		if code := e.Run(context.Background(), resizeCommand, args); code != contract.ExitUsage {
			t.Errorf("resize %v: exit %d", args, code)
		}
	}
	if len(server.resize) != 0 {
		t.Fatalf("usage errors sent %d requests", len(server.resize))
	}
}

// A browser that controls the terminal makes the server answer 409; the
// command fails (exit 1) and says what the server said.
func TestResizeReportsAControllerConflict(t *testing.T) {
	server := &sizeServer{status: http.StatusConflict}
	e, _, stderr := server.env(t)
	if code := e.Run(context.Background(), resizeCommand, []string{"abcdefgh", "120x40"}); code != contract.ExitError || !strings.Contains(stderr.String(), "attached client controls it") {
		t.Fatalf("exit %d, stderr %q", code, stderr)
	}
}

func TestRunSendsInitialSizeAndRequiresBothDimensions(t *testing.T) {
	f := &fakeServer{}
	e, _, stderr, _ := env(t, f)
	if code := Run(context.Background(), e, []string{"--detach", "--cols", "200", "--rows", "50", "--", "true"}); code != contract.ExitOK {
		t.Fatalf("exit %d %q", code, stderr)
	}
	if f.bodies[0]["cols"] != float64(200) || f.bodies[0]["rows"] != float64(50) {
		t.Fatalf("request %#v", f.bodies[0])
	}
	if code := Run(context.Background(), e, []string{"--detach", "--", "true"}); code != contract.ExitOK {
		t.Fatal("plain run failed")
	}
	if _, ok := f.bodies[1]["cols"]; ok {
		t.Fatalf("a run without a size sent one: %#v", f.bodies[1])
	}
	for _, args := range [][]string{
		{"--cols", "100"}, {"--rows", "30"}, {"--cols", "0", "--rows", "30"}, {"--cols", "100", "--rows", "1001"}, {"--cols", "-1", "--rows", "30"},
	} {
		if code := Run(context.Background(), e, append(append([]string{"--detach"}, args...), "--", "true")); code != contract.ExitUsage {
			t.Errorf("%v: exit %d", args, code)
		}
	}
	if len(f.bodies) != 2 {
		t.Fatalf("usage errors sent %d requests", len(f.bodies)-2)
	}
}
