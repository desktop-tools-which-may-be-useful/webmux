package api

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/rainchan/webmux/internal/artifacts"
	"github.com/rainchan/webmux/internal/config"
	"github.com/rainchan/webmux/internal/contract"
	"github.com/rainchan/webmux/internal/hybrid"
	"github.com/rainchan/webmux/internal/launch"
	"github.com/rainchan/webmux/internal/session"
	"github.com/rainchan/webmux/internal/workspace"
)

type hybridAPI struct {
	t       *testing.T
	handler http.Handler
	manager *session.Manager
	store   *launch.Service
	hybrids *hybrid.Service
	folders *artifacts.Service
	state   string
	cwd     string
}

// newHybridAPI serves sessions with a hybrid service whose state directory
// prepare may fill first. Artifacts folders are wired as cmd/webmux does:
// a recorded hybrid session keeps its folder.
func newHybridAPI(t *testing.T, prepare func(state string)) *hybridAPI {
	return newHybridAPIWithSessionOptions(t, prepare, nil)
}

func newHybridAPIWithSessionOptions(t *testing.T, prepare func(state string), configure func(*session.Options)) *hybridAPI {
	t.Helper()
	h := &hybridAPI{t: t, state: t.TempDir(), cwd: t.TempDir()}
	if prepare != nil {
		prepare(h.state)
	}
	options := session.Options{DefaultCwd: h.cwd}
	if configure != nil {
		configure(&options)
	}
	h.manager = session.NewManager(options)
	h.store = openLaunchStore(t)
	h.folders = artifacts.New(artifacts.Options{TempDir: t.TempDir(), StateDir: h.state, Manager: h.manager})
	hybrids := hybrid.New(hybrid.Options{StateDir: h.state, Manager: h.manager, Uploads: h.folders.UploadPath})
	h.hybrids = hybrids
	h.folders.Start(func(id string) bool {
		_, err := hybrids.Record(id)
		return !errors.Is(err, hybrid.ErrNotFound)
	})
	ws := workspace.New(t.TempDir(), h.manager, h.store, h.cwd)
	ws.UseHybrid(hybrids)
	ws.UseArtifacts(h.folders)
	t.Cleanup(func() {
		hybrids.Close()
		ws.Close()
		h.folders.Close()
		ctx, cancel := context.WithTimeout(context.Background(), 7*time.Second)
		defer cancel()
		h.manager.Shutdown(ctx)
	})
	h.handler = New(testDeps(t, Deps{Config: config.Config{Token: "secret", DefaultCwd: h.cwd}, Manager: h.manager, Launch: h.store, Workspace: ws, Hybrid: hybrids, Artifacts: h.folders}))
	return h
}

// config saves a launch config; agent names an executable "omp" script
// that stands in for the agent's terminal UI.
func (h *hybridAPI) config(name string, hybridMode bool) string {
	h.t.Helper()
	dir := h.t.TempDir()
	agent := filepath.Join(dir, "omp")
	if err := os.WriteFile(agent, []byte("#!/bin/sh\nexec sleep 60\n"), 0o755); err != nil {
		h.t.Fatal(err)
	}
	c, err := h.store.Create(launch.Input{Name: name, Cmd: []string{agent}, Env: map[string]string{"API_KEY": "secret-value"}, Hybrid: hybridMode})
	if err != nil {
		h.t.Fatal(err)
	}
	return c.ID
}

func (h *hybridAPI) post(body string) *httptest.ResponseRecorder {
	h.t.Helper()
	r := httptest.NewRequest("POST", "http://webmux.example/api/sessions", strings.NewReader(body))
	r.Header.Set("Authorization", "Bearer secret")
	w := httptest.NewRecorder()
	h.handler.ServeHTTP(w, r)
	return w
}

func (h *hybridAPI) expect(body string, status int) []byte {
	h.t.Helper()
	w := h.post(body)
	if w.Code != status {
		h.t.Fatalf("POST /api/sessions %s: status %d want %d: %s", body, w.Code, status, w.Body.String())
	}
	return w.Body.Bytes()
}

func TestCreateHybridSession(t *testing.T) {
	h := newHybridAPI(t, nil)
	id := h.config("omp", true)
	var info contract.SessionInfo
	if err := json.Unmarshal(sessionOf(h.t, h.expect(fmt.Sprintf(`{"configId":%q,"view":"terminal"}`, id), 201)), &info); err != nil {
		t.Fatal(err)
	}
	if info.Agent == nil || info.View != contract.ViewTerminal || info.State != contract.SessionStateActive || info.Agent.Name != "omp" || info.ConfigID != id ||
		len(info.Views) != 2 || info.Transition != nil {
		t.Fatalf("created %+v agent %+v", info, info.Agent)
	}
	file, err := os.ReadFile(filepath.Join(h.state, "hybrid-sessions.json"))
	if err != nil || !strings.Contains(string(file), info.ID) || strings.Contains(string(file), "secret-value") {
		t.Fatalf("record file %s: %v", file, err)
	}
	// Ordinary configs have the terminal view only.
	plain := h.config("plain", false)
	var ordinary map[string]any
	if err := json.Unmarshal(sessionOf(h.t, h.expect(fmt.Sprintf(`{"configId":%q}`, plain), 201)), &ordinary); err != nil {
		t.Fatal(err)
	}
	if _, ok := ordinary["hybrid"]; ok || ordinary["agent"] != nil || fmt.Sprint(ordinary["views"]) != "[terminal]" {
		t.Fatalf("ordinary session %v", ordinary)
	}
}

func TestCreateSessionStartSideNeedsHybridConfig(t *testing.T) {
	h := newHybridAPI(t, nil)
	plain := h.config("plain", false)
	for _, body := range []string{
		fmt.Sprintf(`{"configId":%q,"view":"chat"}`, plain),
		`{"cmd":["sh"],"view":"chat"}`,
		fmt.Sprintf(`{"configId":%q,"startSide":"acp"}`, h.config("omp-old", true)),
		fmt.Sprintf(`{"configId":%q,"view":"other"}`, h.config("omp", true)),
	} {
		h.expect(body, 400)
	}
	if len(h.manager.List()) != 0 {
		t.Fatal("a rejected request created a session")
	}
}

func TestCreateHybridSessionStoreUnavailable(t *testing.T) {
	h := newHybridAPI(t, func(state string) {
		if err := os.WriteFile(filepath.Join(state, "hybrid-sessions.json"), []byte("{broken"), 0o600); err != nil {
			t.Fatal(err)
		}
	})
	body := h.expect(fmt.Sprintf(`{"configId":%q}`, h.config("omp", true)), 503)
	if !strings.Contains(string(body), "hybrid-sessions.json") {
		t.Fatalf("503 body lacks the store path: %s", body)
	}
	h.expect(fmt.Sprintf(`{"configId":%q}`, h.config("plain", false)), 201)
	// Recovery is disabled the same way; the file stays as it was.
	for _, req := range []struct {
		method, path string
		body         any
	}{
		{"GET", "/api/sessions?state=archived", nil},
		{"GET", "/api/sessions/abcdefgh", nil},
		{"DELETE", "/api/sessions/abcdefgh?forget=1", nil},
		{"POST", "/api/sessions/abcdefgh/resume", contract.ResumeRequest{ClientRequestID: chatRID(), View: contract.ViewTerminal}},
	} {
		if body := h.expectCommand(req.method, req.path, req.body, 503); !strings.Contains(string(body), "hybrid-sessions.json") {
			t.Fatalf("%s %s: 503 body lacks the store path: %s", req.method, req.path, body)
		}
	}
	if b, _ := os.ReadFile(filepath.Join(h.state, "hybrid-sessions.json")); string(b) != "{broken" {
		t.Fatalf("corrupt store rewritten: %q", b)
	}
}

func TestCreateHybridSessionRecordLimitEvictsOldest(t *testing.T) {
	h := newHybridAPI(t, func(state string) {
		const alphabet = "abcdefghijklmnopqrstuvwxyz234567"
		records := make([]map[string]any, hybrid.MaxRecords)
		for i := range records {
			records[i] = map[string]any{
				"id": fmt.Sprintf("lim%c%caaa", alphabet[i/32], alphabet[i%32]), "name": "old", "configId": "c",
				"agent": "omp", "cwd": "/p", "cmd": []string{"omp"}, "side": "tui", "previousIds": []string{},
				"config": map[string]string{}, "state": "closed", "queuedCount": 0,
				"createdAt": "2026-01-01T00:00:00Z", "updatedAt": "2026-01-01T00:00:00Z",
			}
		}
		b, err := json.Marshal(map[string]any{"v": 1, "records": records})
		if err == nil {
			err = os.WriteFile(filepath.Join(state, "hybrid-sessions.json"), b, 0o600)
		}
		if err != nil {
			t.Fatal(err)
		}
	})
	// A full store makes room by evicting the least recently updated
	// archived record instead of refusing the session.
	h.expect(fmt.Sprintf(`{"configId":%q}`, h.config("omp", true)), 201)
	// Ties in update time fall to the smallest ID, where digits sort first.
	h.expectCommand("GET", "/api/sessions/lima2aaa", nil, 404)
	// Removing an old record still makes room for another session.
	h.expectCommand("DELETE", "/api/sessions/limaaaaa?forget=1", nil, 204)
	h.expect(fmt.Sprintf(`{"configId":%q}`, h.config("omp-again", true)), 201)
}

// A force removal closes the worktree's hybrid session, whose shell leaves
// the list once its close is recorded, then removes the worktree and keeps
// the session's record and folder, marked released like the archived
// record's: both can be rebuilt and resumed. A closed session elsewhere is
// left as it is.
func TestForceRemoveWorktreeKeepsHybridRecords(t *testing.T) {
	root := gitRepository(t)
	h := newHybridAPI(t, nil)
	var repository contract.Repository
	if err := json.Unmarshal(workspaceRequest(t, h.handler, "POST", "/api/repositories", map[string]string{"path": root}, 201).Body.Bytes(), &repository); err != nil {
		t.Fatal(err)
	}
	base := "/api/repositories/" + repository.ID
	var tree contract.Worktree
	created := workspaceRequest(t, h.handler, "POST", base+"/worktrees", contract.CreateWorktreeRequest{Path: filepath.Join(t.TempDir(), "agent"), Mode: "new", Branch: "agent", Base: "HEAD"}, 201)
	if err := json.Unmarshal(created.Body.Bytes(), &tree); err != nil {
		t.Fatal(err)
	}
	configID := h.config("omp", true)
	outside := h.createSide(configID, contract.ViewTerminal)
	h.closeSession(outside)
	start := func() string {
		var info contract.SessionInfo
		ref := fmt.Sprintf(`{"configId":%q,"view":"terminal","workspace":{"repositoryId":%q,"worktreeId":%q}}`, configID, repository.ID, tree.ID)
		if err := json.Unmarshal(sessionOf(h.t, h.expect(ref, 201)), &info); err != nil {
			t.Fatal(err)
		}
		return info.ID
	}
	archived := start()
	h.closeSession(archived)
	inside := start()
	var folder contract.ArtifactsFolder
	if err := json.Unmarshal(h.expectCommand("POST", "/api/sessions/"+inside+"/artifacts", nil, 200), &folder); err != nil {
		t.Fatal(err)
	}
	treeURL := base + "/worktrees/" + tree.ID
	var preview contract.RemovalPreview
	if err := json.Unmarshal(workspaceRequest(t, h.handler, "POST", treeURL+"/removal-preview", struct{}{}, 200).Body.Bytes(), &preview); err != nil {
		t.Fatal(err)
	}
	if len(preview.Sessions) != 1 || preview.Sessions[0].ID != inside {
		t.Fatalf("preview sessions %+v", preview.Sessions)
	}
	if len(preview.Archived) != 1 || preview.Archived[0].ID != archived {
		t.Fatalf("preview archived %+v", preview.Archived)
	}
	workspaceRequest(t, h.handler, "POST", treeURL+"/remove", contract.RemoveWorktreeRequest{
		ConfirmationID: preview.ConfirmationID, Mode: "force", ConfirmPath: preview.Worktree.Path, CloseSessions: true,
	}, 204)
	if _, err := os.Stat(tree.Path); !os.IsNotExist(err) {
		t.Fatalf("worktree still present: %v", err)
	}
	// Both records stay, released together at the removed worktree's commit,
	// with their artifact folders: they can be rebuilt and resumed.
	var releaseID string
	for _, id := range []string{inside, archived} {
		rec, err := h.hybrids.Record(id)
		if err != nil || rec.Released == nil || rec.Released.Commit != preview.Commit || rec.Released.Path != tree.Path {
			t.Fatalf("removed worktree's record %s: %+v %v", id, rec, err)
		}
		if releaseID != "" && rec.Released.ID != releaseID {
			t.Fatalf("records released apart: %s and %s", releaseID, rec.Released.ID)
		}
		releaseID = rec.Released.ID
	}
	if _, err := os.Stat(folder.Path); err != nil {
		t.Fatalf("kept record's folder: %v", err)
	}
	if rec, err := h.hybrids.Record(outside); err != nil || rec.State != hybrid.StateClosed || rec.Released != nil {
		t.Fatalf("record outside the worktree: %+v %v", rec, err)
	}
}
