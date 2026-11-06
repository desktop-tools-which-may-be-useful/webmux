//go:build linux || darwin

package api

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/rainchan/webmux/internal/contract"
	"github.com/rainchan/webmux/internal/session"
)

// leaderPID is the PID of a running session's command.
func leaderPID(t *testing.T, m *session.Manager) (string, int) {
	t.Helper()
	dir := t.TempDir()
	s, err := m.Create(session.CreateInput{Cwd: dir, Cmd: []string{"/bin/sh", "-c", "echo $$ > pid; exec sleep 60"}})
	if err != nil {
		t.Fatal(err)
	}
	deadline := time.Now().Add(5 * time.Second)
	for {
		data, err := os.ReadFile(dir + "/pid")
		if pid, convErr := strconv.Atoi(strings.TrimSpace(string(data))); err == nil && convErr == nil {
			return s.Info().ID, pid
		}
		if time.Now().After(deadline) {
			t.Fatal("session never wrote its PID")
		}
		time.Sleep(20 * time.Millisecond)
	}
}

func TestLocalCreateRecordsParentAndLabels(t *testing.T) {
	m := session.NewManager(session.Options{DefaultCwd: t.TempDir()})
	t.Cleanup(func() { m.Shutdown(context.Background()) })
	parent, pid := leaderPID(t, m)
	local := NewLocal(testDeps(t, Deps{Manager: m}), Local{Status: func() contract.StatusV1 { return contract.StatusV1{V: 1} }})

	body := `{"cmd":["/bin/sh","-c","sleep 60"],"labels":{"run":"7"}}`
	req := httptest.NewRequest("POST", "/api/sessions", strings.NewReader(body))
	req = req.WithContext(context.WithValue(req.Context(), peerKey{}, pid))
	w := httptest.NewRecorder()
	local.ServeHTTP(w, req)
	if w.Code != http.StatusCreated {
		t.Fatalf("create: %d %s", w.Code, w.Body)
	}
	created := createdInfo(t, w.Body.Bytes())
	if created.ParentID != parent || created.Labels["run"] != "7" {
		t.Fatalf("created %+v, want parent %s", created, parent)
	}

	// The request body cannot name a parent, and the TCP listener records none.
	tcp := New(testDeps(t, Deps{Manager: m}))
	req = httptest.NewRequest("POST", "/api/sessions", strings.NewReader(`{"cmd":["/bin/sh","-c","sleep 60"],"parentId":"`+parent+`"}`))
	req.Host = "127.0.0.1:7681"
	w = httptest.NewRecorder()
	tcp.ServeHTTP(w, req)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("parentId in the body: %d %s", w.Code, w.Body)
	}
	req = httptest.NewRequest("POST", "/api/sessions", strings.NewReader(`{"cmd":["/bin/sh","-c","sleep 60"]}`))
	req.Host = "127.0.0.1:7681"
	req = req.WithContext(context.WithValue(req.Context(), peerKey{}, pid))
	w = httptest.NewRecorder()
	tcp.ServeHTTP(w, req)
	if w.Code != http.StatusCreated || strings.Contains(w.Body.String(), "parentId") {
		t.Fatalf("tcp create: %d %s", w.Code, w.Body)
	}

	// Invalid labels start nothing.
	before := len(m.List())
	req = httptest.NewRequest("POST", "/api/sessions", strings.NewReader(`{"cmd":["/bin/sh"],"labels":{"Bad Key":"x"}}`))
	w = httptest.NewRecorder()
	local.ServeHTTP(w, req)
	if w.Code != http.StatusBadRequest || len(m.List()) != before {
		t.Fatalf("invalid labels: %d, sessions %d -> %d", w.Code, before, len(m.List()))
	}

	// Details name the child of the parent.
	w = httptest.NewRecorder()
	local.ServeHTTP(w, httptest.NewRequest("GET", "/api/sessions/"+parent+"/details", nil))
	var details contract.SessionDetails
	if err := json.Unmarshal(w.Body.Bytes(), &details); err != nil || w.Code != 200 {
		t.Fatalf("details: %d %s", w.Code, w.Body)
	}
	if len(details.Children) != 1 || details.Children[0] != created.ID || details.ArtifactsDir != nil {
		t.Fatalf("details %+v", details)
	}
}
