package api

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/rainchan/webmux/internal/agentprofile/omp"
	"github.com/rainchan/webmux/internal/contract"
	"github.com/rainchan/webmux/internal/launch"
)

// request sends a request with optional token and Origin.
func (h *hybridAPI) request(method, path, body, token, origin string) *httptest.ResponseRecorder {
	h.t.Helper()
	var reader io.Reader
	if body != "" {
		reader = strings.NewReader(body)
	}
	r := httptest.NewRequest(method, "http://webmux.example"+path, reader)
	if token != "" {
		r.Header.Set("Authorization", "Bearer "+token)
	}
	if origin != "" {
		r.Header.Set("Origin", origin)
	}
	w := httptest.NewRecorder()
	h.handler.ServeHTTP(w, r)
	return w
}

func (h *hybridAPI) records(query string) []contract.SessionInfo {
	h.t.Helper()
	w := h.request("GET", "/api/sessions"+query, "", "secret", "")
	if w.Code != http.StatusOK {
		h.t.Fatalf("GET /api/sessions%s: %d %s", query, w.Code, w.Body.String())
	}
	if strings.Contains(w.Body.String(), "secret-value") || strings.Contains(w.Body.String(), "API_KEY") {
		h.t.Fatalf("record list exposes the environment: %s", w.Body.String())
	}
	var list []contract.SessionInfo
	if err := json.Unmarshal(w.Body.Bytes(), &list); err != nil {
		h.t.Fatal(err)
	}
	return list
}

func (h *hybridAPI) createSide(configID string, side contract.SessionView) string {
	h.t.Helper()
	var info contract.SessionInfo
	if err := json.Unmarshal(sessionOf(h.t, h.expect(fmt.Sprintf(`{"configId":%q,"view":%q}`, configID, side), 201)), &info); err != nil {
		h.t.Fatal(err)
	}
	return info.ID
}

// closeSession closes id through the API and waits for its shell to go.
func (h *hybridAPI) closeSession(id string) {
	h.t.Helper()
	h.expectCommand("DELETE", "/api/sessions/"+id, nil, http.StatusNoContent)
	waitAPI(h.t, "closed "+id, func() bool { _, err := h.manager.Get(id); return err != nil })
}

func (h *hybridAPI) resume(id string, req contract.ResumeRequest) contract.SessionInfo {
	h.t.Helper()
	req.ClientRequestID = chatRID()
	var info contract.SessionInfo
	if err := json.Unmarshal(h.expectCommand("POST", "/api/sessions/"+id+"/resume", req, http.StatusOK), &info); err != nil {
		h.t.Fatal(err)
	}
	if info.ID != id || info.Agent == nil || (req.View != "" && info.View != req.View) || info.State != contract.SessionStateActive {
		h.t.Fatalf("resumed %+v agent %+v", info, info.Agent)
	}
	return info
}

func waitAPI(t *testing.T, what string, ok func() bool) {
	t.Helper()
	deadline := time.Now().Add(chatTimeout)
	for !ok() {
		if time.Now().After(deadline) {
			t.Fatalf("timed out waiting for %s", what)
		}
		time.Sleep(10 * time.Millisecond)
	}
}

// processEnv reads the environment of session id's recorded root process
// (Linux only; nil elsewhere).
func (h *hybridAPI) processEnv(id string) []string {
	h.t.Helper()
	if runtime.GOOS != "linux" {
		return nil
	}
	rec, err := h.hybrids.Record(id)
	if err != nil || rec.PID <= 0 {
		h.t.Fatalf("record %+v: %v", rec, err)
	}
	b, err := os.ReadFile(fmt.Sprintf("/proc/%d/environ", rec.PID))
	if err != nil {
		h.t.Fatal(err)
	}
	return strings.Split(string(b), "\x00")
}

func hasEnv(env []string, kv string) bool {
	for _, e := range env {
		if e == kv {
			return true
		}
	}
	return false
}

func TestHybridRecordEndpoints(t *testing.T) {
	h := newHybridAPI(t, nil)
	configID := h.config("omp", true)
	closed := h.createSide(configID, contract.ViewTerminal)
	active := h.createSide(configID, contract.ViewTerminal)
	var folder contract.ArtifactsFolder
	if err := json.Unmarshal(h.expectCommand("POST", "/api/sessions/"+closed+"/artifacts", nil, 200), &folder); err != nil {
		t.Fatal(err)
	}
	h.closeSession(closed)
	// Closing keeps the folder for a resume.
	time.Sleep(100 * time.Millisecond)
	if _, err := os.Stat(folder.Path); err != nil {
		t.Fatalf("closing removed the folder: %v", err)
	}

	list := h.records("?state=archived")
	if len(list) != 1 || list[0].ID != closed || list[0].State != contract.SessionStateClosed || list[0].Archive == nil || list[0].Archive.ConfigMissing ||
		list[0].ConfigID != configID || list[0].Agent == nil || list[0].Agent.Name != omp.Name || list[0].View != contract.ViewTerminal ||
		list[0].Archive.PreviousAgentSessionIDs == nil || list[0].Archive.InterruptedAt != nil || list[0].Status != contract.SessionStatusExited || list[0].Generation != "" {
		t.Fatalf("archived %+v", list)
	}
	if running := h.records(""); len(running) != 1 || running[0].ID != active || running[0].State != contract.SessionStateActive {
		t.Fatalf("active sessions %+v", running)
	}
	if all := h.records("?state=all"); len(all) != 2 || all[0].State != contract.SessionStateActive || all[1].ID != closed {
		t.Fatalf("all sessions %+v", all)
	}
	if both := h.records("?state=active&state=closed"); len(both) != 2 {
		t.Fatalf("repeated state %+v", both)
	}
	var one contract.SessionInfo
	if err := json.Unmarshal(h.expectCommand("GET", "/api/sessions/"+closed, nil, 200), &one); err != nil || one.ID != closed || one.Archive == nil {
		t.Fatalf("archived session %+v: %v", one, err)
	}
	h.expectCommand("GET", "/api/sessions/zzzzzzzz", nil, 404)
	h.expectCommand("GET", "/api/sessions?state=recoverable", nil, 400)
	h.expectCommand("GET", "/api/hybrid-sessions", nil, 404)
	if w := h.request("GET", "/api/sessions?state=archived", "", "", ""); w.Code != http.StatusUnauthorized {
		t.Fatalf("without token: %d", w.Code)
	}
	if w := h.request("DELETE", "/api/sessions/"+closed+"?forget=1", "", "secret", "https://evil.example"); w.Code != http.StatusForbidden {
		t.Fatalf("foreign origin: %d", w.Code)
	}
	if w := h.request("POST", "/api/sessions/"+closed+"/resume", `{"clientRequestId":"api-req-cross","view":"terminal"}`, "secret", "https://evil.example"); w.Code != http.StatusForbidden {
		t.Fatalf("foreign origin resume: %d", w.Code)
	}

	if err := h.store.Delete(configID); err != nil {
		t.Fatal(err)
	}
	if list := h.records("?state=archived"); len(list) != 1 || !list[0].Archive.ConfigMissing {
		t.Fatalf("after deleting the config %+v", list)
	}
	body := h.expectCommand("POST", "/api/sessions/"+active+"/resume", contract.ResumeRequest{ClientRequestID: chatRID(), View: contract.ViewTerminal}, 409)
	if !strings.Contains(string(body), `"reason":"not_archived"`) {
		t.Fatalf("resume of an active session: %s", body)
	}
	h.expectCommand("DELETE", "/api/sessions/"+closed+"?forget=1", nil, 204)
	h.expectCommand("GET", "/api/sessions/"+closed, nil, 404)
	if _, err := os.Stat(folder.Path); !os.IsNotExist(err) {
		t.Fatalf("forgetting the session kept the folder: %v", err)
	}
	h.expectCommand("DELETE", "/api/sessions/"+closed+"?forget=1", nil, 404)
	if list := h.records("?state=archived"); len(list) != 0 {
		t.Fatalf("forgotten session listed %+v", list)
	}
	// Forgetting an active session closes it first.
	h.expectCommand("DELETE", "/api/sessions/"+active+"?forget=1", nil, 204)
	if list := h.records("?state=all"); len(list) != 0 {
		t.Fatalf("after forgetting the active session %+v", list)
	}
}

// Resuming reads the config's environment again, asks before resuming
// without a deleted config's environment, and offers candidates when the
// agent session is gone.
func TestResumeHybridSession(t *testing.T) {
	h := newHybridAPI(t, nil)
	agent := fakeAgentPath(t, omp.Name)
	store := t.TempDir()
	// The store reaches the agent even without the config's environment.
	t.Setenv("FAKEAGENT_STORE", store)
	saved, err := h.store.Create(launch.Input{
		Name: "omp-chat", Cmd: []string{agent}, Hybrid: true,
		Env: map[string]string{"API_KEY": "secret-value", "FAKEAGENT_TURN_DELAY": "5ms"},
	})
	if err != nil {
		t.Fatal(err)
	}
	id := h.createSide(saved.ID, contract.ViewChat)
	h.closeSession(id)

	h.resume(id, contract.ResumeRequest{View: contract.ViewTerminal})
	if env := h.processEnv(id); env != nil && !hasEnv(env, "API_KEY=secret-value") {
		t.Fatal("resumed terminal lacks the config's environment")
	}
	h.expectCommand("POST", "/api/sessions/"+id+"/resume", contract.ResumeRequest{ClientRequestID: chatRID(), View: contract.ViewChat}, 409)
	h.closeSession(id)

	if err := h.store.Delete(saved.ID); err != nil {
		t.Fatal(err)
	}
	h.expectConflict("POST", "/api/sessions/"+id+"/resume", contract.ResumeRequest{ClientRequestID: chatRID(), View: contract.ViewChat}, "config_missing")
	if _, err := h.manager.Get(id); err == nil {
		t.Fatal("config_missing started a shell")
	}
	h.resume(id, contract.ResumeRequest{View: contract.ViewChat, ConfirmWithoutEnv: true})
	if env := h.processEnv(id); env != nil && hasEnv(env, "API_KEY=secret-value") {
		t.Fatal("resumed chat kept the deleted config's environment")
	}
	h.closeSession(id)

	rec, err := h.hybrids.Record(id)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(filepath.Join(store, "sessions", rec.AgentSessionID+".json")); err != nil {
		t.Fatal(err)
	}
	candidate := fmt.Sprintf(`{"id":"candidate1","cwd":%q,"title":"earlier","updatedAt":"2026-01-02T03:04:05Z","messages":[],"config":{"mode":"code","model":"fast"}}`, rec.Cwd)
	if err := os.WriteFile(filepath.Join(store, "sessions", "candidate1.json"), []byte(candidate), 0o600); err != nil {
		t.Fatal(err)
	}
	body := h.expectCommand("POST", "/api/sessions/"+id+"/resume", contract.ResumeRequest{ClientRequestID: chatRID(), View: contract.ViewTerminal, ConfirmWithoutEnv: true}, 409)
	var conflict contract.SessionConflict
	if err := json.Unmarshal(body, &conflict); err != nil {
		t.Fatal(err)
	}
	if conflict.Reason != "session_missing" || len(conflict.Candidates) != 1 || conflict.Candidates[0].AgentSessionID != "candidate1" ||
		conflict.Candidates[0].Title != "earlier" || !conflict.Candidates[0].UpdatedAt.Equal(time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC)) {
		t.Fatalf("conflict %s", body)
	}
	info := h.resume(id, contract.ResumeRequest{View: contract.ViewTerminal, AgentSessionID: "candidate1", ConfirmWithoutEnv: true})
	if info.Agent.SessionID == nil || *info.Agent.SessionID != "candidate1" {
		t.Fatalf("resumed agent session %v", info.Agent.SessionID)
	}
	file, err := os.ReadFile(filepath.Join(h.state, "hybrid-sessions.json"))
	if err != nil || strings.Contains(string(file), "secret-value") {
		t.Fatalf("record file %s: %v", file, err)
	}
}
