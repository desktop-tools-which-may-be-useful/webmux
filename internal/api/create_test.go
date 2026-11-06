package api

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os/exec"
	"strings"
	"testing"

	"github.com/rainchan/webmux/internal/agentprofile/codex"
	"github.com/rainchan/webmux/internal/agentprofile/omp"
	"github.com/rainchan/webmux/internal/contract"
	"github.com/rainchan/webmux/internal/hybrid"
	"github.com/rainchan/webmux/internal/launch"
)

// authBoundary sends body without credentials (401) and from a foreign
// Origin (403).
func authBoundary(t *testing.T, handler http.Handler, method, path, body string) {
	t.Helper()
	for _, status := range []int{http.StatusUnauthorized, http.StatusForbidden} {
		r := httptest.NewRequest(method, "http://webmux.example"+path, strings.NewReader(body))
		if status == http.StatusForbidden {
			r.Header.Set("Authorization", "Bearer secret")
			r.Header.Set("Origin", "https://evil.example")
		}
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, r)
		if w.Code != status {
			t.Fatalf("%s %s: status %d want %d: %s", method, path, w.Code, status, w.Body.String())
		}
	}
}

func TestRepositoryInitInstructionAPI(t *testing.T) {
	handler, _, _, root := workspaceAPI(t)
	var repository contract.Repository
	register := workspaceRequest(t, handler, "POST", "/api/repositories", contract.RegisterRepositoryRequest{Path: root}, 201)
	if err := json.Unmarshal(register.Body.Bytes(), &repository); err != nil || repository.InitInstruction != "" {
		t.Fatalf("register: %+v %v", repository, err)
	}
	base := "/api/repositories/" + repository.ID
	authBoundary(t, handler, "PATCH", base, `{"initInstruction":"forged"}`)
	workspaceRequest(t, handler, "POST", "/api/repositories", map[string]any{"path": root, "initInstruction": "x"}, 400)
	workspaceRequest(t, handler, "PATCH", base, contract.UpdateRepositoryRequest{InitInstruction: "先读 AGENTS.md"}, 200)
	var again contract.Repository
	duplicate := workspaceRequest(t, handler, "POST", "/api/repositories", contract.RegisterRepositoryRequest{Path: root}, 200)
	if err := json.Unmarshal(duplicate.Body.Bytes(), &again); err != nil || again.InitInstruction != "先读 AGENTS.md" {
		t.Fatalf("duplicate registration: %+v %v", again, err)
	}
	var updated contract.Repository
	patch := workspaceRequest(t, handler, "PATCH", base, contract.UpdateRepositoryRequest{InitInstruction: "改完运行 make check"}, 200)
	if err := json.Unmarshal(patch.Body.Bytes(), &updated); err != nil || updated.InitInstruction != "改完运行 make check" || updated.ID != repository.ID {
		t.Fatalf("patch: %+v %v", updated, err)
	}
	workspaceRequest(t, handler, "PATCH", base, contract.UpdateRepositoryRequest{InitInstruction: strings.Repeat("x", 4001)}, 400)
	workspaceRequest(t, handler, "PATCH", "/api/repositories/missing", contract.UpdateRepositoryRequest{}, 404)
	workspaceRequest(t, handler, "PATCH", base, map[string]any{"initInstruction": "x", "name": "y"}, 400)
	var listed []contract.Repository
	if err := json.Unmarshal(workspaceRequest(t, handler, "GET", "/api/repositories", nil, 200).Body.Bytes(), &listed); err != nil || len(listed) != 1 || listed[0].InitInstruction != "改完运行 make check" {
		t.Fatalf("list after rejected updates: %+v %v", listed, err)
	}
	cleared := workspaceRequest(t, handler, "PATCH", base, contract.UpdateRepositoryRequest{}, 200)
	if !strings.Contains(cleared.Body.String(), `"initInstruction":""`) {
		t.Fatalf("cleared: %s", cleared.Body.String())
	}
}

type quickAPI struct {
	*hybridAPI
	root       string
	repository contract.Repository
	agent      string
}

// newQuickAPI serves quick starts into a registered repository, with a
// hybrid launch config of the fake agent; launch config deletion forgets
// its options as cmd/webmux wires it.
func newQuickAPI(t *testing.T) *quickAPI {
	t.Helper()
	root := gitRepository(t)
	q := &quickAPI{hybridAPI: newHybridAPI(t, nil), root: root}
	q.store.OnDelete(func(id string) { _ = q.hybrids.ForgetAgentOptions(id) })
	if err := json.Unmarshal(workspaceRequest(t, q.handler, "POST", "/api/repositories", map[string]string{"path": root}, 201).Body.Bytes(), &q.repository); err != nil {
		t.Fatal(err)
	}
	c, err := q.store.Create(launch.Input{Name: "agent", Cmd: []string{fakeAgentPath(t, omp.Name)}, Env: map[string]string{"FAKEAGENT_STORE": t.TempDir(), "FAKEAGENT_TURN_DELAY": "5ms"}, Hybrid: true})
	if err != nil {
		t.Fatal(err)
	}
	q.agent = c.ID
	return q
}

// request is the quick start page's create: a chat in a new worktree.
func (q *quickAPI) request(requestID, configID, branch string) contract.CreateSessionRequest {
	return contract.CreateSessionRequest{
		ClientRequestID: requestID, ConfigID: &configID, View: contract.ViewChat,
		NewWorktree: &contract.NewWorktree{RepositoryID: q.repository.ID, Mode: "new", Base: "HEAD", Branch: branch},
		Prompt:      "修复首页排序",
	}
}

func (q *quickAPI) failure(req contract.CreateSessionRequest, status int) contract.CreateSessionFailure {
	q.t.Helper()
	var body contract.CreateSessionFailure
	if err := json.Unmarshal(q.expectCommand("POST", "/api/sessions", req, status), &body); err != nil {
		q.t.Fatal(err)
	}
	return body
}

func (q *quickAPI) worktrees() int {
	q.t.Helper()
	out, err := exec.Command("git", "-C", q.root, "worktree", "list", "--porcelain").Output()
	if err != nil {
		q.t.Fatal(err)
	}
	return strings.Count(string(out), "worktree ")
}

func TestCreateAPIRefusesBeforeCreating(t *testing.T) {
	q := newQuickAPI(t)
	body, err := json.Marshal(q.request("api-quick-001", q.agent, "wt/unauthorized"))
	if err != nil {
		t.Fatal(err)
	}
	authBoundary(t, q.handler, "POST", "/api/sessions", string(body))

	plain := q.config("plain", false)
	failed := q.failure(q.request("api-quick-002", plain, "wt/plain"), http.StatusBadRequest)
	if failed.Step != "precheck" || failed.Error == "" || failed.Worktree != nil || failed.SessionID != "" {
		t.Fatalf("not hybrid: %+v", failed)
	}
	codexConfig, err := q.store.Create(launch.Input{Name: "codex", Cmd: []string{fakeAgentPath(t, codex.Name)}, Env: map[string]string{"PATH": "/nonexistent"}, Hybrid: true})
	if err != nil {
		t.Fatal(err)
	}
	failed = q.failure(q.request("api-quick-003", codexConfig.ID, "wt/codex"), http.StatusConflict)
	if failed.Step != "precheck" || failed.Reason != "connector_missing" || failed.Connector == nil || failed.Connector.Executable != "codex-acp" {
		t.Fatalf("connector missing: %+v", failed)
	}
	failed = q.failure(q.request("api-quick-004", "zzzzzzzz", "wt/unknown"), http.StatusNotFound)
	if failed.Step != "precheck" {
		t.Fatalf("unknown config: %+v", failed)
	}
	r := httptest.NewRequest("POST", "http://webmux.example/api/sessions", strings.NewReader(`{"clientRequestId":"api-quick-005","extra":1}`))
	r.Header.Set("Authorization", "Bearer secret")
	w := httptest.NewRecorder()
	q.handler.ServeHTTP(w, r)
	if w.Code != http.StatusBadRequest || !strings.Contains(w.Body.String(), `"step":"precheck"`) {
		t.Fatalf("malformed body: %d %s", w.Code, w.Body.String())
	}
	if got := q.worktrees(); got != 1 || len(q.manager.List()) != 0 {
		t.Fatalf("refused quick starts created %d worktrees, sessions %+v", got, q.manager.List())
	}
}

// The quick start goes on when the client is gone before it ends; the
// launch config list then offers the agent's options until the config is
// deleted.
func TestCreateAPICompletesAfterDisconnect(t *testing.T) {
	q := newQuickAPI(t)
	if strings.Contains(string(q.expectCommand("GET", "/api/launch-configs", nil, 200)), `"chatOptions":[{`) {
		t.Fatal("options before any session")
	}
	data, err := json.Marshal(q.request("api-quick-101", q.agent, "wt/gone"))
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	r := httptest.NewRequestWithContext(ctx, "POST", "http://webmux.example/api/sessions", strings.NewReader(string(data)))
	r.Header.Set("Authorization", "Bearer secret")
	w := httptest.NewRecorder()
	q.handler.ServeHTTP(w, r)
	if w.Code != http.StatusCreated {
		t.Fatalf("disconnected quick start: %d %s", w.Code, w.Body.String())
	}
	var result contract.CreateSessionResult
	if err := json.Unmarshal(w.Body.Bytes(), &result); err != nil {
		t.Fatal(err)
	}
	if result.Session.ID == "" || result.Session.View != contract.ViewChat || result.Worktree == nil || result.Worktree.Branch == nil || *result.Worktree.Branch != "wt/gone" || result.ConfigWarnings == nil {
		t.Fatalf("result %+v", result)
	}
	watch := observe(t, q.hybrids, result.Session.ID)
	watch.until("first prompt", func(o *observer) bool {
		for _, e := range o.events {
			if e.Kind == hybrid.EventMessage && e.Message != nil && e.Message.Role == hybrid.RoleUser && e.Message.Text == "修复首页排序" {
				return true
			}
		}
		return false
	})
	// A retry of the same request gets the same session.
	var retried contract.CreateSessionResult
	if err := json.Unmarshal(q.expectCommand("POST", "/api/sessions", q.request("api-quick-101", q.agent, "wt/gone"), 201), &retried); err != nil || retried.Session.ID != result.Session.ID {
		t.Fatalf("retry: %+v %v", retried, err)
	}
	var list []contract.LaunchConfigSummary
	if err := json.Unmarshal(q.expectCommand("GET", "/api/launch-configs", nil, 200), &list); err != nil {
		t.Fatal(err)
	}
	found := false
	for _, c := range list {
		if c.ID == q.agent {
			found = len(c.ChatOptions) == 2
		} else if c.ChatOptions == nil || len(c.ChatOptions) != 0 {
			t.Fatalf("config %s options %+v", c.ID, c.ChatOptions)
		}
	}
	if !found {
		t.Fatalf("agent options not listed: %+v", list)
	}
	q.expectCommand("DELETE", "/api/launch-configs/"+q.agent, nil, http.StatusNoContent)
	if options, err := q.hybrids.AgentOptions(q.agent); err != nil || len(options) != 0 {
		t.Fatalf("options of a deleted config: %+v %v", options, err)
	}
	// The worktree exists: a new request for its branch conflicts.
	failed := q.failure(q.request("api-quick-102", q.config("again", true), "wt/gone"), http.StatusConflict)
	if failed.Step != "worktree" || failed.Reason != "conflict" {
		t.Fatalf("existing worktree: %+v", failed)
	}
}

// A file uploaded before the session exists goes with the first prompt
// from the new session's uploads; a path from anywhere else is refused
// before anything is created.
func TestCreateAPISendsStagedAttachments(t *testing.T) {
	q := newQuickAPI(t)
	body, contentType := uploadBody(t, "spec.md", []byte("# 规格"), "")
	upload := func(auth bool) *httptest.ResponseRecorder {
		r := httptest.NewRequest(http.MethodPost, "http://webmux.example/api/uploads", strings.NewReader(string(body)))
		if auth {
			r.Header.Set("Authorization", "Bearer secret")
		}
		r.Header.Set("Content-Type", contentType)
		w := httptest.NewRecorder()
		q.handler.ServeHTTP(w, r)
		return w
	}
	if w := upload(false); w.Code != http.StatusUnauthorized {
		t.Fatalf("unauthenticated upload: %d", w.Code)
	}
	w := upload(true)
	var stored contract.UploadResult
	if err := json.Unmarshal(w.Body.Bytes(), &stored); err != nil || w.Code != http.StatusCreated {
		t.Fatalf("upload: %d %s", w.Code, w.Body.String())
	}

	forged := q.request("api-quick-201", q.agent, "wt/forged")
	forged.Attachments = []contract.ChatAttachment{{Path: q.root + "/README.md"}}
	if failed := q.failure(forged, http.StatusBadRequest); failed.Step != "precheck" || q.worktrees() != 1 {
		t.Fatalf("forged attachment: %+v", failed)
	}

	req := q.request("api-quick-202", q.agent, "wt/attached")
	req.Attachments = []contract.ChatAttachment{{Path: stored.Path}}
	var result contract.CreateSessionResult
	if err := json.Unmarshal(q.expectCommand("POST", "/api/sessions", req, 201), &result); err != nil {
		t.Fatal(err)
	}
	watch := observe(t, q.hybrids, result.Session.ID)
	watch.until("first prompt with its attachment", func(o *observer) bool {
		for _, e := range o.events {
			if e.Kind != hybrid.EventMessage || e.Message == nil || e.Message.Role != hybrid.RoleUser || len(e.Message.Attachments) != 1 {
				continue
			}
			got := e.Message.Attachments[0]
			path, err := q.folders.UploadPath(result.Session.ID, got.Path)
			return err == nil && path == got.Path && got.Name == "spec.md"
		}
		return false
	})
}
