package api

import (
	"context"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/coder/websocket"
	"github.com/rainchan/webmux/internal/config"
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
