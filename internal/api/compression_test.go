package api

import (
	"bytes"
	"compress/flate"
	"context"
	"io"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/coder/websocket"
	"github.com/rainchan/webmux/internal/config"
	"github.com/rainchan/webmux/internal/contract"
	"github.com/rainchan/webmux/internal/session"
)

// Browsers always offer permessage-deflate; only compress=1 may accept it.
func TestWebSocketCompressionOptIn(t *testing.T) {
	c, err := config.Load(t.TempDir()+"/absent", false)
	if err != nil {
		t.Fatal(err)
	}
	c.Token = "secret"
	c.DefaultCwd = t.TempDir()
	if err = c.Validate(); err != nil {
		t.Fatal(err)
	}
	m := session.NewManager(testSessionOptions(c))
	server := httptest.NewServer(New(testDeps(t, Deps{Config: c, Manager: m, Workspace: testWorkspace(t, m, c.DefaultCwd)})))
	t.Cleanup(server.Close)
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 7*time.Second)
		defer cancel()
		m.Shutdown(ctx)
	})
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	for _, row := range []struct {
		query string
		want  bool
	}{{"", false}, {"?compress=0", false}, {"?compress=1", true}} {
		conn, response, err := websocket.Dial(ctx, "ws"+strings.TrimPrefix(server.URL, "http")+"/ws/sessions"+row.query, &websocket.DialOptions{
			Subprotocols:    tokenProtocols("secret"),
			CompressionMode: websocket.CompressionContextTakeover,
		})
		if err != nil {
			t.Fatal(err)
		}
		got := strings.Contains(response.Header.Get("Sec-WebSocket-Extensions"), "permessage-deflate")
		if got != row.want {
			t.Fatalf("%q negotiated deflate=%v want %v", row.query, got, row.want)
		}
		if _, raw, err := conn.Read(ctx); err != nil || !strings.Contains(string(raw), `"snapshot"`) {
			t.Fatalf("%q snapshot %s %v", row.query, raw, err)
		}
		_ = conn.CloseNow()
	}
}

// zreplay=1 asks for a DEFLATEd replay, but never on top of permessage-deflate.
func TestTerminalReplayDeflateOptIn(t *testing.T) {
	f := newContinuityFixture(t, []string{"/bin/bash", "-c", `stty raw -echo; for i in $(seq 400); do echo replayed line; done; printf READY; exec cat >/dev/null`})
	for _, row := range []struct {
		query string
		want  byte
	}{{"zreplay=1", contract.FrameDeflate}, {"zreplay=1&compress=1", contract.FrameSnapshot}, {"", contract.FrameSnapshot}} {
		conn := f.dial(row.query)
		var kind byte
		var replay []byte
		for kind == 0 || kind == contract.FrameDeflate && replay == nil {
			_, p, err := conn.Read(f.ctx)
			if err != nil {
				t.Fatalf("%q: %v", row.query, err)
			}
			switch p[0] {
			case contract.FrameSnapshot:
				kind, replay = p[0], p[1:]
			case contract.FrameDeflate:
				inflated, err := io.ReadAll(flate.NewReader(bytes.NewReader(p[1:])))
				if err != nil || len(inflated) == 0 || inflated[0] != contract.FrameSnapshot {
					t.Fatalf("%q: bad deflate frame: %v", row.query, err)
				}
				if len(p) >= len(inflated) {
					t.Fatalf("%q: deflate frame did not shrink the snapshot", row.query)
				}
				kind, replay = p[0], inflated[1:]
			}
		}
		if kind != row.want {
			t.Fatalf("%q: snapshot arrived as frame %d, want %d", row.query, kind, row.want)
		}
		if !bytes.Contains(replay, []byte("replayed line")) || !bytes.HasSuffix(replay, []byte("READY")) {
			t.Fatalf("%q: replay content %q", row.query, replay)
		}
	}
}
