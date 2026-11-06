//go:build linux || darwin

package api

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/rainchan/webmux/internal/agentguide"
	"github.com/rainchan/webmux/internal/config"
	"github.com/rainchan/webmux/internal/contract"
	"github.com/rainchan/webmux/internal/session"
)

func TestGuideRoutes(t *testing.T) {
	m := session.NewManager(session.Options{DefaultCwd: t.TempDir()})
	t.Cleanup(func() { m.Shutdown(context.Background()) })
	guide := &agentguide.Instance{Version: "7.7.7", LocalURL: "http://127.0.0.1:7681", Bin: "/opt/webmux/bin/webmux"}
	guide.SetSocket("/run/user/1000/webmux/webmux.sock")
	deps := testDeps(t, Deps{Manager: m, Guide: guide, Config: config.Config{Token: "login-secret", Listen: "127.0.0.1:7681"}})
	tcp := New(deps)
	local := NewLocal(deps, Local{Status: func() contract.StatusV1 { return contract.StatusV1{V: 1} }})
	get := func(h http.Handler, path string) *httptest.ResponseRecorder {
		req := httptest.NewRequest("GET", path, nil)
		req.Host = "127.0.0.1:7681"
		w := httptest.NewRecorder()
		h.ServeHTTP(w, req)
		return w
	}
	// No token on TCP, and nothing about the instance.
	w := get(tcp, "/llms.txt")
	if w.Code != 200 || !strings.HasPrefix(w.Body.String(), "# WebMux") || strings.Contains(w.Body.String(), "7.7.7") {
		t.Fatalf("tcp /llms.txt: %d %q", w.Code, w.Body.String())
	}
	if ct := w.Header().Get("Content-Type"); ct != "text/plain; charset=utf-8" || w.Header().Get("Cache-Control") != "no-store" {
		t.Fatalf("headers %v", w.Header())
	}
	w = get(tcp, "/llms/connect.md")
	if w.Code != 200 || w.Header().Get("Content-Type") != "text/markdown; charset=utf-8" {
		t.Fatalf("topic: %d %v", w.Code, w.Header())
	}
	for _, missing := range []string{"/llms/token.md", "/llms/connect"} {
		if w := get(tcp, missing); w.Code != http.StatusNotFound {
			t.Errorf("%s: %d", missing, w.Code)
		}
	}
	// ServeMux redirects an unclean path instead of serving it.
	if w := get(tcp, "/llms/../go.mod"); w.Code == http.StatusOK {
		t.Errorf("unclean path served: %q", w.Body.String())
	}
	w = get(tcp, "/llms-full.txt")
	if w.Code != 200 || !strings.Contains(w.Body.String(), "# 操作守则") || strings.Contains(w.Body.String(), "/opt/webmux") {
		t.Fatalf("tcp full: %d", w.Code)
	}
	// The Host check still applies.
	req := httptest.NewRequest("GET", "/llms.txt", nil)
	req.Host = "evil.example"
	w = httptest.NewRecorder()
	tcp.ServeHTTP(w, req)
	if w.Code != http.StatusForbidden {
		t.Fatalf("foreign host: %d", w.Code)
	}
	// The local socket adds the instance facts.
	w = get(local, "/llms.txt")
	for _, fact := range []string{"7.7.7", "/opt/webmux/bin/webmux", "/run/user/1000/webmux/webmux.sock"} {
		if !strings.Contains(w.Body.String(), fact) {
			t.Errorf("local index lacks %q", fact)
		}
	}
}

func TestAgentGuideBrief(t *testing.T) {
	m := session.NewManager(session.Options{DefaultCwd: t.TempDir()})
	t.Cleanup(func() { m.Shutdown(context.Background()) })
	guide := &agentguide.Instance{LocalURL: "http://127.0.0.1:7681", Bin: "/opt/webmux/bin/webmux"}
	local := NewLocal(testDeps(t, Deps{Manager: m, Guide: guide}), Local{Status: func() contract.StatusV1 { return contract.StatusV1{V: 1} }})
	s, err := m.Create(session.CreateInput{Cmd: []string{"/bin/sh", "-c", "sleep 60"}})
	if err != nil {
		t.Fatal(err)
	}
	id := s.Info().ID
	w := httptest.NewRecorder()
	local.ServeHTTP(w, httptest.NewRequest("POST", "/api/sessions/"+id+"/agent-guide", nil))
	var got contract.AgentGuidePrompt
	if err := json.Unmarshal(w.Body.Bytes(), &got); err != nil || w.Code != 200 {
		t.Fatalf("brief: %d %s", w.Code, w.Body)
	}
	if !strings.Contains(got.Prompt, id) || !strings.Contains(got.Prompt, got.Dir) || got.GuideURL != "http://127.0.0.1:7681/llms.txt" ||
		got.Bin == nil || *got.Bin != "/opt/webmux/bin/webmux" || got.Socket != nil || got.Dir == "" {
		t.Fatalf("brief %+v", got)
	}
	w = httptest.NewRecorder()
	local.ServeHTTP(w, httptest.NewRequest("POST", "/api/sessions/aaaaaaaa/agent-guide", nil))
	if w.Code != http.StatusNotFound {
		t.Fatalf("missing session: %d", w.Code)
	}
	noGuide := NewLocal(testDeps(t, Deps{Manager: m}), Local{Status: func() contract.StatusV1 { return contract.StatusV1{V: 1} }})
	w = httptest.NewRecorder()
	noGuide.ServeHTTP(w, httptest.NewRequest("POST", "/api/sessions/"+id+"/agent-guide", nil))
	if w.Code != http.StatusServiceUnavailable {
		t.Fatalf("without a guide: %d", w.Code)
	}
}
