package clicmd

import (
	"context"
	"encoding/json"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"slices"
	"sync"
	"testing"

	"github.com/rainchan/webmux/internal/contract"
	"github.com/rainchan/webmux/internal/localclient"
)

func homeRepo() contract.Repository {
	return contract.Repository{ID: "r1", Availability: "available", Worktrees: []contract.Worktree{
		{ID: "main", Kind: "main"}, {ID: "a", Kind: "linked"}, {ID: "b", Kind: "linked"}, {ID: "c", Kind: "linked"},
	}}
}

// TestHomeStatusFollowsTheWebRules mirrors homeLayout in
// web/src/homeWorktrees.ts: an uncurated repository shows its main
// worktree, curated lists are taken as they are, and sessions or archived
// records always show a worktree.
func TestHomeStatusFollowsTheWebRules(t *testing.T) {
	repo := homeRepo()
	for _, tc := range []struct {
		name string
		h    home
		want map[string]string
	}{
		{"uncurated", home{}, map[string]string{"main": homePinned, "a": homeHidden, "b": homeHidden, "c": homeHidden}},
		{"curated", home{pins: map[string][]string{"r1": {"a"}}}, map[string]string{"main": homeHidden, "a": homePinned, "b": homeHidden}},
		{"curated empty", home{pins: map[string][]string{"r1": {}}}, map[string]string{"main": homeHidden, "a": homeHidden}},
		{"sessions and archived", home{sessions: map[string][]contract.SessionInfo{"b": {{ID: "s1"}}}, archived: map[string][]contract.SessionInfo{"c": {{ID: "h1"}, {ID: "h2"}}}},
			map[string]string{"main": homePinned, "a": homeHidden, "b": homeSessions, "c": homeSessions}},
		{"pinned beats sessions", home{pins: map[string][]string{"r1": {"b"}}, sessions: map[string][]contract.SessionInfo{"b": {{ID: "s1"}}}}, map[string]string{"b": homePinned}},
	} {
		for _, tree := range repo.Worktrees {
			want, ok := tc.want[tree.ID]
			if !ok {
				continue
			}
			if got := tc.h.status(repo, tree); got != want {
				t.Errorf("%s: %s is %s, want %s", tc.name, tree.ID, got, want)
			}
		}
	}
}

// TestArchivedWorktreeFollowsTheWeb mirrors the archived session placement
// of web/src/archived.ts: a recorded association wins, and a session without
// one joins the deepest registered worktree containing its cwd.
func TestArchivedWorktreeFollowsTheWeb(t *testing.T) {
	repos := []contract.Repository{{ID: "r1", Worktrees: []contract.Worktree{
		{ID: "main", Path: "/w/repo"}, {ID: "nested", Path: "/w/repo/sub"},
	}}}
	for _, tc := range []struct {
		record contract.SessionInfo
		want   string
		ok     bool
	}{
		{contract.SessionInfo{Cwd: "/w/repo/x", Workspace: &contract.SessionWorkspace{RepositoryID: "r1", WorktreeID: "nested"}}, "nested", true},
		{contract.SessionInfo{Cwd: "/w/repo/sub/deep"}, "nested", true},
		{contract.SessionInfo{Cwd: "/w/repo"}, "main", true},
		{contract.SessionInfo{Cwd: "/w/repository"}, "", false},
	} {
		got, ok := archivedWorktree(tc.record, repos)
		if got != tc.want || ok != tc.ok {
			t.Errorf("%+v: %q %v, want %q %v", tc.record, got, ok, tc.want, tc.ok)
		}
	}
}

// settingsServer is a /api/settings fake that rejects the first n PATCHes
// with 409 after another client changed another repository.
type settingsServer struct {
	mu        sync.Mutex
	revision  int64
	home      map[string][]string
	conflicts int
	patches   int
}

func (s *settingsServer) serve(t *testing.T) Env {
	t.Helper()
	dir, err := os.MkdirTemp("/tmp", "scw")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(dir) })
	socket := filepath.Join(dir, "s.sock")
	l, err := net.Listen("unix", socket)
	if err != nil {
		t.Fatal(err)
	}
	mux := http.NewServeMux()
	// Pins are pruned against the repository list read on each attempt.
	mux.HandleFunc("GET /api/repositories", func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode([]contract.Repository{homeRepo()})
	})
	mux.HandleFunc("GET /api/settings", func(w http.ResponseWriter, r *http.Request) {
		s.mu.Lock()
		defer s.mu.Unlock()
		_ = json.NewEncoder(w).Encode(contract.SettingsSnapshot{Revision: s.revision, Values: contract.SettingsValues{HomeWorktrees: s.home}})
	})
	mux.HandleFunc("PATCH /api/settings", func(w http.ResponseWriter, r *http.Request) {
		s.mu.Lock()
		defer s.mu.Unlock()
		s.patches++
		var patch contract.SettingsPatch
		_ = json.NewDecoder(r.Body).Decode(&patch)
		if s.conflicts > 0 {
			// Another browser changed another repository meanwhile.
			s.conflicts--
			s.revision++
			s.home["other"] = append(s.home["other"], "x")
			w.WriteHeader(http.StatusConflict)
			_, _ = w.Write([]byte(`{"error":"conflict"}`))
			return
		}
		if patch.BaseRevision == nil || *patch.BaseRevision != s.revision {
			w.WriteHeader(http.StatusConflict)
			return
		}
		_ = json.Unmarshal(patch.Changes["homeWorktrees"], &s.home)
		s.revision++
		_ = json.NewEncoder(w).Encode(contract.SettingsSnapshot{Revision: s.revision})
	})
	srv := &http.Server{Handler: mux}
	go func() { _ = srv.Serve(l) }()
	t.Cleanup(func() { _ = srv.Close() })
	return Env{Connect: func() (*localclient.Client, error) { return localclient.New(localclient.UnixDialer{Path: socket}), nil }}
}

func TestPinReplaysAfterAConflict(t *testing.T) {
	s := &settingsServer{revision: 3, home: map[string][]string{"other": {"o1"}}, conflicts: 2}
	e := s.serve(t)
	client, _ := e.Connect()
	defer client.Close()
	// The caller's snapshot is stale: the server no longer lists "gone".
	repo := homeRepo()
	repo.Worktrees = append(repo.Worktrees, contract.Worktree{ID: "gone"})
	s.mu.Lock()
	s.home["r1"] = []string{"main", "gone"}
	s.mu.Unlock()
	if err := pinWorktrees(context.Background(), client, repo, []string{"a"}, true); err != nil {
		t.Fatal(err)
	}
	if s.patches != 3 || !slices.Equal(s.home["r1"], []string{"main", "a"}) || !slices.Equal(s.home["other"], []string{"o1", "x", "x"}) {
		t.Fatalf("after replay: %d patches, %v", s.patches, s.home)
	}
	// Pins of worktrees a healthy repository no longer lists are dropped.
	s.home["r1"] = []string{"main", "a", "stale"}
	if err := pinWorktrees(context.Background(), client, homeRepo(), []string{"a"}, false); err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(s.home["r1"], []string{"main"}) {
		t.Fatalf("after unpin: %v", s.home["r1"])
	}
	// Three conflicts in a row give up.
	s.conflicts = 3
	if err := pinWorktrees(context.Background(), client, homeRepo(), []string{"b"}, true); err == nil {
		t.Fatal("kept conflicting but succeeded")
	}
}
