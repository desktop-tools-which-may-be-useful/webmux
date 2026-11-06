package api

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/coder/websocket"
	"github.com/rainchan/webmux/internal/config"
	"github.com/rainchan/webmux/internal/contract"
	"github.com/rainchan/webmux/internal/session"
	"github.com/rainchan/webmux/internal/workspace"
)

func testWorkspace(t *testing.T, m *session.Manager, cwd string) *workspace.Service {
	t.Helper()
	service := workspace.New(t.TempDir(), m, nil, cwd)
	t.Cleanup(service.Close)
	return service
}

// gitRepository isolates Git from the user's configuration and returns a
// repository with one commit.
func gitRepository(t *testing.T) string {
	t.Helper()
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("requires real Git")
	}
	t.Setenv("HOME", t.TempDir())
	t.Setenv("GIT_CONFIG_NOSYSTEM", "1")
	t.Setenv("GIT_CONFIG_GLOBAL", os.DevNull)
	t.Setenv("GIT_AUTHOR_NAME", "API test")
	t.Setenv("GIT_AUTHOR_EMAIL", "api@example.invalid")
	t.Setenv("GIT_COMMITTER_NAME", "API test")
	t.Setenv("GIT_COMMITTER_EMAIL", "api@example.invalid")
	root := t.TempDir()
	for _, args := range [][]string{{"init", "-b", "main"}, {"commit", "--allow-empty", "-m", "initial"}} {
		cmd := exec.Command("git", append([]string{"-C", root}, args...)...)
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("Git fixture: %v %s", err, out)
		}
	}
	return root
}
func workspaceAPI(t *testing.T) (http.Handler, *workspace.Service, *session.Manager, string) {
	t.Helper()
	root := gitRepository(t)
	c := config.Config{DefaultCwd: root, Token: "secret"}
	m := session.NewManager(testSessionOptions(c))
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
		defer cancel()
		m.Shutdown(ctx)
	})
	service := testWorkspace(t, m, root)
	return New(testDeps(t, Deps{Config: c, Manager: m, Workspace: service})), service, m, root
}
func workspaceRequest(t *testing.T, handler http.Handler, method, path string, body any, want int) *httptest.ResponseRecorder {
	t.Helper()
	data, err := json.Marshal(body)
	if err != nil {
		t.Fatal(err)
	}
	request := httptest.NewRequest(method, path, strings.NewReader(string(data)))
	request.Header.Set("Authorization", "Bearer secret")
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, request)
	if recorder.Code != want {
		t.Fatalf("%s %s: status %d want %d: %s", method, path, recorder.Code, want, recorder.Body.String())
	}
	return recorder
}
func TestWorkspaceAPIRoutesAndErrorBoundaries(t *testing.T) {
	handler, _, m, root := workspaceAPI(t)
	for _, status := range []int{401, 403} {
		request := httptest.NewRequest("POST", "/api/repositories", strings.NewReader(fmt.Sprintf(`{"path":%q}`, root)))
		if status == 403 {
			request.Header.Set("Authorization", "Bearer secret")
			request.Header.Set("Origin", "https://evil.example")
		}
		recorder := httptest.NewRecorder()
		handler.ServeHTTP(recorder, request)
		if recorder.Code != status {
			t.Fatalf("auth boundary: %d %s", recorder.Code, recorder.Body.String())
		}
	}
	empty := workspaceRequest(t, handler, "GET", "/api/repositories", nil, 200)
	if strings.TrimSpace(empty.Body.String()) != "[]" {
		t.Fatal("unauthorized registration had side effects")
	}
	register := workspaceRequest(t, handler, "POST", "/api/repositories", map[string]string{"path": root}, 201)
	var repository contract.Repository
	if err := json.Unmarshal(register.Body.Bytes(), &repository); err != nil {
		t.Fatal(err)
	}
	duplicate := workspaceRequest(t, handler, "POST", "/api/repositories", map[string]string{"path": root}, 200)
	var again contract.Repository
	if err := json.Unmarshal(duplicate.Body.Bytes(), &again); err != nil {
		t.Fatal(err)
	}
	if again.ID != repository.ID {
		t.Fatal("duplicate registration changed identity")
	}
	base := "/api/repositories/" + repository.ID
	workspaceRequest(t, handler, "GET", base+"/worktrees", nil, 200)
	workspaceRequest(t, handler, "GET", base+"/branches", nil, 200)
	workspaceRequest(t, handler, "GET", "/api/repositories/missing/worktrees", nil, 404)
	workspaceRequest(t, handler, "POST", base+"/worktrees", contract.CreateWorktreeRequest{Path: filepath.Join(t.TempDir(), "bad"), Mode: "new", Branch: "--force", Base: "HEAD"}, 400)
	created := workspaceRequest(t, handler, "POST", base+"/worktrees", contract.CreateWorktreeRequest{Path: filepath.Join(t.TempDir(), "linked"), Mode: "new", Branch: "linked", Base: "HEAD"}, 201)
	var tree contract.Worktree
	if err := json.Unmarshal(created.Body.Bytes(), &tree); err != nil {
		t.Fatal(err)
	}
	ref := &contract.WorkspaceReference{RepositoryID: repository.ID, WorktreeID: tree.ID}
	workspaceRequest(t, handler, "POST", "/api/sessions", map[string]any{"cwd": root, "cmd": []string{"/bin/sh"}, "workspace": ref}, 400)
	started := workspaceRequest(t, handler, "POST", "/api/sessions", map[string]any{"cmd": []string{"/bin/sh", "-c", "printf READY; read value"}, "workspace": ref}, 201)
	info := createdInfo(t, started.Body.Bytes())
	if info.Cwd != tree.Path || info.Workspace == nil || info.Workspace.WorktreeID != tree.ID {
		t.Fatalf("workspace JSON: %+v", info)
	}
	treeURL := base + "/worktrees/" + tree.ID
	pre := workspaceRequest(t, handler, "POST", treeURL+"/removal-preview", struct{}{}, 200)
	var preview contract.RemovalPreview
	if err := json.Unmarshal(pre.Body.Bytes(), &preview); err != nil {
		t.Fatal(err)
	}
	if len(preview.Sessions) != 1 || !preview.ForceAllowed {
		t.Fatalf("preview JSON: %+v", preview)
	}
	workspaceRequest(t, handler, "POST", treeURL+"/remove", contract.RemoveWorktreeRequest{ConfirmationID: preview.ConfirmationID, Mode: "safe"}, 409)
	for _, status := range []int{401, 403} {
		request := httptest.NewRequest("POST", treeURL+"/remove", strings.NewReader(`{"mode":"force"}`))
		if status == 403 {
			request.Header.Set("Authorization", "Bearer secret")
			request.Header.Set("Origin", "https://evil.example")
		}
		recorder := httptest.NewRecorder()
		handler.ServeHTTP(recorder, request)
		if recorder.Code != status {
			t.Fatalf("remove auth boundary: %d", recorder.Code)
		}
	}
	item, err := m.Get(info.ID)
	if err != nil {
		t.Fatal(err)
	}
	if item.Info().Status == session.StatusExited {
		t.Fatal("rejected operations stopped PTY")
	}
	workspaceRequest(t, handler, "DELETE", base, nil, 204)
	if item.Info().Workspace != nil || item.Info().Status == session.StatusExited {
		t.Fatal("unregister changed PTY lifecycle")
	}
	if _, err := os.Stat(tree.Path); err != nil {
		t.Fatal("unregister deleted tree", err)
	}
	legacy := workspaceRequest(t, handler, "POST", "/api/sessions", map[string]any{"cwd": root, "cmd": []string{"/bin/sh", "-c", "printf LEGACY"}}, 201)
	if !strings.Contains(legacy.Body.String(), `"workspace":null`) {
		t.Fatal("legacy JSON missing explicit null workspace")
	}
}
func gitWorkspaceAPI(t *testing.T, cwd string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", append([]string{"-C", cwd}, args...)...)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("Git fixture %v: %v %s", args, err, out)
	}
}

func TestWorkspaceBranchRefreshAPI(t *testing.T) {
	handler, _, _, root := workspaceAPI(t)
	register := workspaceRequest(t, handler, "POST", "/api/repositories", map[string]string{"path": root}, 201)
	var repository contract.Repository
	if err := json.Unmarshal(register.Body.Bytes(), &repository); err != nil {
		t.Fatal(err)
	}
	base := "/api/repositories/" + repository.ID
	refreshURL := base + "/branches/refresh"
	for _, status := range []int{401, 403} {
		request := httptest.NewRequest("POST", refreshURL, strings.NewReader(`{"branch":"main"}`))
		if status == 403 {
			request.Header.Set("Authorization", "Bearer secret")
			request.Header.Set("Origin", "https://evil.example")
		}
		recorder := httptest.NewRecorder()
		handler.ServeHTTP(recorder, request)
		if recorder.Code != status {
			t.Fatalf("refresh auth boundary: got %d want %d", recorder.Code, status)
		}
	}

	listed := workspaceRequest(t, handler, "GET", base+"/branches", nil, 200)
	var branches contract.Branches
	if err := json.Unmarshal(listed.Body.Bytes(), &branches); err != nil {
		t.Fatal(err)
	}
	if branches.DefaultBranch == nil || *branches.DefaultBranch != "main" {
		t.Fatalf("default branch: %+v", branches)
	}
	gitWorkspaceAPI(t, root, "branch", "standalone")
	workspaceRequest(t, handler, "POST", refreshURL, contract.RefreshBranchBaseRequest{}, 400)
	workspaceRequest(t, handler, "POST", refreshURL, contract.RefreshBranchBaseRequest{Branch: "missing"}, 404)
	noUpstream := workspaceRequest(t, handler, "POST", refreshURL, contract.RefreshBranchBaseRequest{Branch: "standalone"}, 200)
	var fallback contract.RefreshBranchBaseResponse
	if err := json.Unmarshal(noUpstream.Body.Bytes(), &fallback); err != nil {
		t.Fatal(err)
	}
	if fallback.BaseSource != "local" || fallback.FetchWarning == "" || fallback.Upstream != "" {
		t.Fatalf("no-upstream response: %+v", fallback)
	}

	remote := filepath.Join(t.TempDir(), "remote.git")
	gitWorkspaceAPI(t, root, "clone", "--bare", root, remote)
	gitWorkspaceAPI(t, root, "remote", "add", "origin", remote)
	gitWorkspaceAPI(t, root, "push", "-u", "origin", "main")
	refreshed := workspaceRequest(t, handler, "POST", refreshURL, contract.RefreshBranchBaseRequest{Branch: "main"}, 200)
	var remoteBase contract.RefreshBranchBaseResponse
	if err := json.Unmarshal(refreshed.Body.Bytes(), &remoteBase); err != nil {
		t.Fatal(err)
	}
	if remoteBase.BaseSource != "remote" || remoteBase.Upstream != "origin/main" || remoteBase.FetchWarning != "" || remoteBase.Base == "" {
		t.Fatalf("remote response: %+v", remoteBase)
	}
	gitWorkspaceAPI(t, root, "remote", "set-url", "origin", filepath.Join(t.TempDir(), "missing.git"))
	failed := workspaceRequest(t, handler, "POST", refreshURL, contract.RefreshBranchBaseRequest{Branch: "main"}, 200)
	if err := json.Unmarshal(failed.Body.Bytes(), &fallback); err != nil {
		t.Fatal(err)
	}
	if fallback.BaseSource != "local" || fallback.Base != remoteBase.Base || fallback.FetchWarning == "" || strings.Contains(fallback.FetchWarning, "missing.git") {
		t.Fatalf("failed-fetch response: %+v", fallback)
	}
}

func TestWorkspaceAPIConfigDegradationAndTimeout(t *testing.T) {
	cwd := t.TempDir()
	t.Setenv("PATH", t.TempDir())
	m := session.NewManager(session.Options{DefaultCwd: cwd})
	defer m.Shutdown(context.Background())
	state := filepath.Join(t.TempDir(), "state-file")
	if err := os.WriteFile(state, []byte("not a directory"), 0600); err != nil {
		t.Fatal(err)
	}
	service := workspace.New(state, m, nil, cwd)
	defer service.Close()
	handler := New(testDeps(t, Deps{Config: config.Config{DefaultCwd: cwd, Token: "secret"}, Manager: m, Workspace: service}))
	response := workspaceRequest(t, handler, "GET", "/api/config", nil, 200)
	var settings struct {
		DefaultCwd string               `json:"defaultCwd"`
		Hostname   string               `json:"hostname"`
		Workspace  workspace.Capability `json:"workspace"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &settings); err != nil {
		t.Fatal(err)
	}
	hostname, err := os.Hostname()
	if err != nil {
		t.Fatal(err)
	}
	if settings.Hostname != hostname {
		t.Fatalf("hostname = %q, want %q", settings.Hostname, hostname)
	}
	if settings.Workspace.GitAvailable || settings.Workspace.RegistryAvailable || strings.Contains(response.Body.String(), "secret") || strings.Contains(response.Body.String(), "token") {
		t.Fatalf("capability/credential response: %s", response.Body.String())
	}
	workspaceRequest(t, handler, "GET", "/api/repositories", nil, 503)
	workspaceRequest(t, handler, "POST", "/api/sessions", map[string]any{"cmd": []string{"/bin/sh", "-c", "printf DIRECT"}}, 201)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	request := httptest.NewRequest("POST", "/api/sessions", strings.NewReader(`{"cmd":["/bin/sh"]}`)).WithContext(ctx)
	request.Header.Set("Authorization", "Bearer secret")
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, request)
	// A create runs to its end even when the client is gone: a retry with
	// its request ID would get the outcome.
	if recorder.Code != http.StatusCreated {
		t.Fatalf("create after the client left: %d %s", recorder.Code, recorder.Body.String())
	}
}
func TestUnavailableRepositoryIsLocalToList(t *testing.T) {
	handler, _, _, root := workspaceAPI(t)
	added := workspaceRequest(t, handler, "POST", "/api/repositories", map[string]string{"path": root}, 201)
	var repository contract.Repository
	if err := json.Unmarshal(added.Body.Bytes(), &repository); err != nil {
		t.Fatal(err)
	}
	if err := os.Rename(root, root+"-moved"); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Rename(root+"-moved", root) })
	list := workspaceRequest(t, handler, "GET", "/api/repositories", nil, 200)
	var repositories []contract.Repository
	if err := json.Unmarshal(list.Body.Bytes(), &repositories); err != nil {
		t.Fatal(err)
	}
	if len(repositories) != 1 || repositories[0].Availability != "unavailable" || repositories[0].Error == "" {
		t.Fatalf("local read failure lost registry: %s", list.Body.String())
	}
	workspaceRequest(t, handler, "GET", "/api/repositories/"+repository.ID+"/branches", nil, 503)
	workspaceRequest(t, handler, "DELETE", "/api/repositories/"+repository.ID, nil, 204)
}
func TestWorkspaceMetadataUsesExistingWebSocketProtocol(t *testing.T) {
	handler, service, m, root := workspaceAPI(t)
	repository, _, err := service.Register(context.Background(), root)
	if err != nil {
		t.Fatal(err)
	}
	created, err := service.Create(context.Background(), workspace.CreateInput{Cwd: root, Cmd: []string{"/bin/sh", "-c", "printf READY; read value"}})
	item := created.Session
	if err != nil {
		t.Fatal(err)
	}
	server := httptest.NewServer(handler)
	defer server.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	conn, _, err := websocket.Dial(ctx, "ws"+strings.TrimPrefix(server.URL, "http")+"/ws/sessions", &websocket.DialOptions{Subprotocols: tokenProtocols("secret")})
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = conn.CloseNow() }()
	_, data, err := conn.Read(ctx)
	if err != nil {
		t.Fatal(err)
	}
	var event contract.SessionEvent
	if err := json.Unmarshal(data, &event); err != nil {
		t.Fatal(err)
	}
	if event.Type != "snapshot" || len(event.Sessions) != 1 || event.Sessions[0].Workspace == nil || event.Sessions[0].Workspace.RepositoryID != repository.ID {
		t.Fatalf("list WS workspace: %s", data)
	}
	terminal, _, err := websocket.Dial(ctx, "ws"+strings.TrimPrefix(server.URL, "http")+"/ws/s/"+item.Info().ID, &websocket.DialOptions{Subprotocols: tokenProtocols("secret")})
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = terminal.CloseNow() }()
	readPosition(ctx, t, terminal, "reset")
	kind, replay, err := terminal.Read(ctx)
	if err != nil || kind != websocket.MessageBinary || len(replay) == 0 || replay[0] != 1 {
		t.Fatalf("binary replay changed: %v %q %v", kind, replay, err)
	}
	_, state, err := terminal.Read(ctx)
	if err != nil || len(state) == 0 || state[0] != 2 {
		t.Fatalf("state frame changed: %q %v", state, err)
	}
	var info contract.SessionInfo
	if err := json.Unmarshal(state[1:], &info); err != nil {
		t.Fatal(err)
	}
	if info.Workspace == nil || info.Workspace.RepositoryID != repository.ID {
		t.Fatalf("terminal WS metadata: %s", state)
	}
	if err := m.Close(item.Info().ID); err != nil {
		t.Fatal(err)
	}
}

func TestWorkspaceBatchRemovalAPIAndAuthBoundaries(t *testing.T) {
	handler, _, _, root := workspaceAPI(t)
	register := workspaceRequest(t, handler, "POST", "/api/repositories", map[string]string{"path": root}, 201)
	var repository contract.Repository
	if err := json.Unmarshal(register.Body.Bytes(), &repository); err != nil {
		t.Fatal(err)
	}
	base := "/api/repositories/" + repository.ID
	previewURL := base + "/worktrees/batch-removal-preview"
	removeURL := base + "/worktrees/batch-remove"

	for _, url := range []string{previewURL, removeURL} {
		for _, status := range []int{401, 403} {
			request := httptest.NewRequest("POST", url, strings.NewReader(`{}`))
			if status == 403 {
				request.Header.Set("Authorization", "Bearer secret")
				request.Header.Set("Origin", "https://evil.example")
			}
			recorder := httptest.NewRecorder()
			handler.ServeHTTP(recorder, request)
			if recorder.Code != status {
				t.Fatalf("batch auth boundary %s: got %d want %d", url, recorder.Code, status)
			}
		}
	}

	created := workspaceRequest(t, handler, "POST", base+"/worktrees", contract.CreateWorktreeRequest{
		Path:   filepath.Join(t.TempDir(), "batch-api-clean"),
		Mode:   "new",
		Branch: "batch-api-clean",
		Base:   "HEAD",
	}, 201)
	var clean contract.Worktree
	if err := json.Unmarshal(created.Body.Bytes(), &clean); err != nil {
		t.Fatal(err)
	}

	workspaceRequest(t, handler, "POST", previewURL, contract.BatchRemovalPreviewRequest{}, 400)
	workspaceRequest(t, handler, "POST", previewURL, contract.BatchRemovalPreviewRequest{WorktreeIDs: []string{"wt_missing"}}, 404)

	pre := workspaceRequest(t, handler, "POST", previewURL, contract.BatchRemovalPreviewRequest{
		WorktreeIDs: []string{repository.Worktrees[0].ID, clean.ID},
	}, 200)
	var preview contract.BatchRemovalPreview
	if err := json.Unmarshal(pre.Body.Bytes(), &preview); err != nil {
		t.Fatal(err)
	}
	if len(preview.Items) != 2 || preview.ConfirmationID == "" {
		t.Fatalf("batch preview JSON: %+v", preview)
	}

	workspaceRequest(t, handler, "POST", removeURL, contract.BatchRemoveWorktreesRequest{
		ConfirmationID: preview.ConfirmationID,
		IncludeForce:   true,
		CloseSessions:  false,
	}, 400)

	removed := workspaceRequest(t, handler, "POST", removeURL, contract.BatchRemoveWorktreesRequest{
		ConfirmationID: preview.ConfirmationID,
		IncludeForce:   false,
		CloseSessions:  false,
	}, 200)
	var result contract.BatchRemoveResult
	if err := json.Unmarshal(removed.Body.Bytes(), &result); err != nil {
		t.Fatal(err)
	}
	if len(result.Results) != 2 || result.Results[0].Status != "skipped" || result.Results[1].Status != "removed" {
		t.Fatalf("batch remove JSON: %+v", result)
	}
	workspaceRequest(t, handler, "POST", removeURL, contract.BatchRemoveWorktreesRequest{
		ConfirmationID: preview.ConfirmationID,
	}, 409)
}
