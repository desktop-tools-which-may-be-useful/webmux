package api

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/rainchan/webmux/internal/contract"
	"github.com/rainchan/webmux/internal/session"
)

func decodeSession(t *testing.T, body []byte) contract.SessionInfo {
	t.Helper()
	var info contract.SessionInfo
	if err := json.Unmarshal(body, &info); err != nil {
		t.Fatal(err)
	}
	return info
}

// A new session opens at the requested size; a half-given or out-of-range
// size is refused before anything starts.
func TestCreateSessionInitialSize(t *testing.T) {
	handler, _, m, _ := workspaceAPI(t)
	cmd := []string{"/bin/sleep", "30"}
	var created contract.CreateSessionResult
	body := workspaceRequest(t, handler, "POST", "/api/sessions", contract.CreateSessionRequest{Cmd: cmd, Cols: 132, Rows: 43}, 201).Body.Bytes()
	if err := json.Unmarshal(body, &created); err != nil {
		t.Fatal(err)
	}
	if created.Session.Cols != 132 || created.Session.Rows != 43 {
		t.Fatalf("created at %dx%d", created.Session.Cols, created.Session.Rows)
	}
	for _, req := range []contract.CreateSessionRequest{
		{Cmd: cmd, Cols: 132}, {Cmd: cmd, Rows: 43}, {Cmd: cmd, Cols: 1001, Rows: 43}, {Cmd: cmd, Cols: 80, Rows: -1},
	} {
		workspaceRequest(t, handler, "POST", "/api/sessions", req, 400)
	}
	if got := len(m.List()); got != 1 {
		t.Fatalf("refused creates left %d sessions", got)
	}
}

func TestSessionResizeAPI(t *testing.T) {
	handler, _, m, _ := workspaceAPI(t)
	item, err := m.Create(session.CreateInput{Cmd: []string{"/bin/sleep", "30"}})
	if err != nil {
		t.Fatal(err)
	}
	id := item.Info().ID
	path := "/api/sessions/" + id + "/resize"
	size := func() (uint16, uint16) { info := item.Info(); return info.Cols, info.Rows }

	authBoundary(t, handler, "POST", path, `{"cols":100,"rows":30}`)
	if cols, rows := size(); cols != 80 || rows != 24 {
		t.Fatalf("an unauthenticated resize changed the size to %dx%d", cols, rows)
	}

	info := decodeSession(t, workspaceRequest(t, handler, "POST", path, contract.SessionResizeRequest{Cols: 100, Rows: 30}, 200).Body.Bytes())
	if info.ID != id || info.Cols != 100 || info.Rows != 30 {
		t.Fatalf("resize answered %+v", info)
	}
	for _, req := range []any{
		contract.SessionResizeRequest{Cols: 0, Rows: 30}, contract.SessionResizeRequest{Cols: 100, Rows: 1001},
		map[string]any{"cols": 100, "rows": 30, "extra": true},
	} {
		workspaceRequest(t, handler, "POST", path, req, 400)
	}
	workspaceRequest(t, handler, "POST", "/api/sessions/aaaaaaaa/resize", contract.SessionResizeRequest{Cols: 100, Rows: 30}, 404)

	// A browser controls: the script is refused until it takes control.
	browser, err := item.AttachClient(session.AttachOptions{Client: "browser"})
	if err != nil {
		t.Fatal(err)
	}
	defer item.Detach(browser)
	workspaceRequest(t, handler, "POST", path, contract.SessionResizeRequest{Cols: 90, Rows: 25}, 409)
	if cols, rows := size(); cols != 100 || rows != 30 {
		t.Fatalf("a refused resize changed the size to %dx%d", cols, rows)
	}
	info = decodeSession(t, workspaceRequest(t, handler, "POST", path, contract.SessionResizeRequest{Cols: 90, Rows: 25, Take: true}, 200).Body.Bytes())
	if info.Cols != 90 || info.Rows != 25 {
		t.Fatalf("take answered %+v", info)
	}

	if err := m.Close(id); err != nil {
		t.Fatal(err)
	}
	deadline := time.Now().Add(10 * time.Second)
	for item.Info().ExitCode == nil {
		if time.Now().After(deadline) {
			t.Fatal("session did not exit")
		}
		time.Sleep(10 * time.Millisecond)
	}
	workspaceRequest(t, handler, "POST", path, contract.SessionResizeRequest{Cols: 100, Rows: 30}, 409)
}
