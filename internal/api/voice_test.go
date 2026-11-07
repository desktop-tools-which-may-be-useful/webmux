package api

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/coder/websocket"
	"github.com/rainchan/webmux/internal/config"
	"github.com/rainchan/webmux/internal/contract"
	"github.com/rainchan/webmux/internal/session"
	"github.com/rainchan/webmux/internal/voice"
)

func TestConfigVoiceFlagAndNoLeak(t *testing.T) {
	const loginToken = "webmux-login-token"
	m := session.NewManager(session.Options{DefaultCwd: t.TempDir()})
	defer m.Shutdown(context.Background())

	cfg := config.Config{
		Token: loginToken,
		Voice: config.VoiceConfig{
			Cmd: []string{"/secret/path/asr.js"},
			Env: []string{"SECRET_API_KEY=super-secret-value"},
		},
	}
	voiceSvc := voice.New(voice.Options{Cmd: cfg.Voice.Cmd, Env: cfg.Voice.Env})
	handler := New(testDeps(t, Deps{
		Config:    cfg,
		Manager:   m,
		Workspace: testWorkspace(t, m, t.TempDir()),
		Voice:     voiceSvc,
	}))

	req := httptest.NewRequest(http.MethodGet, "/api/config", nil)
	req.Header.Set("Authorization", "Bearer "+loginToken)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status %d: %s", rec.Code, rec.Body.String())
	}
	body := rec.Body.String()
	for _, secret := range []string{"/secret/path/asr.js", "SECRET_API_KEY", "super-secret-value"} {
		if strings.Contains(body, secret) {
			t.Fatalf("GET /api/config leaked %q: %s", secret, body)
		}
	}
	var parsed contract.ServerConfig
	if err := json.Unmarshal(rec.Body.Bytes(), &parsed); err != nil {
		t.Fatal(err)
	}
	if !parsed.Voice {
		t.Fatalf("expected voice=true, got %+v", parsed)
	}
}

func TestVoiceWebSocketAuthDisabledAndStreaming(t *testing.T) {
	m := session.NewManager(session.Options{DefaultCwd: t.TempDir()})
	defer m.Shutdown(context.Background())

	t.Run("disabled returns 404", func(t *testing.T) {
		server := httptest.NewServer(New(testDeps(t, Deps{
			Config:    config.Config{Token: "secret"},
			Manager:   m,
			Workspace: testWorkspace(t, m, t.TempDir()),
		})))
		defer server.Close()

		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		header := http.Header{"Origin": []string{server.URL}}
		_, resp, err := websocket.Dial(ctx, "ws"+strings.TrimPrefix(server.URL, "http")+"/ws/voice", &websocket.DialOptions{
			HTTPHeader:   header,
			Subprotocols: tokenProtocols("secret"),
		})
		if err == nil {
			t.Fatal("expected dial to fail when voice is disabled")
		}
		if resp == nil || resp.StatusCode != http.StatusNotFound {
			t.Fatalf("status = %v, want 404", resp)
		}
	})

	t.Run("streaming audio and stop returns final transcription", func(t *testing.T) {
		script := `printf '{"type":"ready"}\n'; bytes=$(wc -c | tr -d ' '); printf '{"type":"partial","text":"recv:%s"}\n' "$bytes"; printf '{"type":"final","text":"语音识别结果:%s"}\n' "$bytes"`
		voiceSvc := voice.New(voice.Options{
			Cmd: []string{"/bin/sh", "-c", script},
		})
		server := httptest.NewServer(New(testDeps(t, Deps{
			Config:    config.Config{Token: "secret"},
			Manager:   m,
			Workspace: testWorkspace(t, m, t.TempDir()),
			Voice:     voiceSvc,
		})))
		defer server.Close()

		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		wsURL := "ws" + strings.TrimPrefix(server.URL, "http") + "/ws/voice"

		// Unauthenticated rejected with 401
		_, resp, _ := websocket.Dial(ctx, wsURL, &websocket.DialOptions{
			HTTPHeader: http.Header{"Origin": []string{server.URL}},
		})
		if resp == nil || resp.StatusCode != http.StatusUnauthorized {
			t.Fatalf("unauthenticated status = %v, want 401", resp)
		}

		// Cross-origin rejected with 403
		_, resp, _ = websocket.Dial(ctx, wsURL, &websocket.DialOptions{
			HTTPHeader:   http.Header{"Origin": []string{"https://evil.example"}},
			Subprotocols: tokenProtocols("secret"),
		})
		if resp == nil || resp.StatusCode != http.StatusForbidden {
			t.Fatalf("cross-origin status = %v, want 403", resp)
		}

		// Authorized streaming
		conn, _, err := websocket.Dial(ctx, wsURL, &websocket.DialOptions{
			HTTPHeader:   http.Header{"Origin": []string{server.URL}},
			Subprotocols: tokenProtocols("secret"),
		})
		if err != nil {
			t.Fatal(err)
		}
		defer func() { _ = conn.CloseNow() }()

		readEvent := func() contract.VoiceServerEvent {
			typ, data, rerr := conn.Read(ctx)
			if rerr != nil {
				t.Fatalf("read event: %v", rerr)
			}
			if typ != websocket.MessageText {
				t.Fatalf("message type = %v, want text", typ)
			}
			var ev contract.VoiceServerEvent
			if uerr := json.Unmarshal(data, &ev); uerr != nil {
				t.Fatalf("unmarshal event: %v", uerr)
			}
			return ev
		}

		if ev := readEvent(); ev.Type != "ready" {
			t.Fatalf("first event = %+v, want ready", ev)
		}
		if err := conn.Write(ctx, websocket.MessageBinary, make([]byte, 640)); err != nil {
			t.Fatal(err)
		}
		if err := conn.Write(ctx, websocket.MessageBinary, make([]byte, 640)); err != nil {
			t.Fatal(err)
		}
		if err := conn.Write(ctx, websocket.MessageText, []byte(`{"type":"stop"}`)); err != nil {
			t.Fatal(err)
		}

		if ev := readEvent(); ev.Type != "partial" || ev.Text != "recv:1280" {
			t.Fatalf("second event = %+v, want partial recv:1280", ev)
		}
		if ev := readEvent(); ev.Type != "final" || ev.Text != "语音识别结果:1280" {
			t.Fatalf("third event = %+v, want final 语音识别结果:1280", ev)
		}
	})
}
