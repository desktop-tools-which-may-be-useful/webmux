package hub

import (
	"testing"

	"github.com/rainchan/webmux/internal/session"
)

// The session list shows active sessions only: a hybrid shell that is
// interrupted leaves with removed, a resumed one comes back with created,
// and events about hidden shells are dropped.
func TestActiveViewHidesArchivedShells(t *testing.T) {
	hybrid := func(id, state string) session.Info {
		return session.Info{ID: id, Hybrid: &session.HybridInfo{Side: "acp", State: state}}
	}
	var v activeView
	snap := v.filter(session.Event{Type: "snapshot", Sessions: []session.Info{{ID: "plain"}, hybrid("live", "active"), hybrid("gone", "interrupted")}})
	if len(snap.Sessions) != 2 || snap.Sessions[1].ID != "live" {
		t.Fatalf("snapshot %+v", snap.Sessions)
	}
	if _, ok := v.event(session.Event{Type: "updated", Session: ptr(hybrid("gone", "interrupted"))}); ok {
		t.Fatal("update of a hidden shell sent")
	}
	if e, ok := v.event(session.Event{Type: "updated", Session: ptr(hybrid("live", "interrupted"))}); !ok || e.Type != "removed" || e.ID != "live" {
		t.Fatalf("interrupted: %+v %v", e, ok)
	}
	if e, ok := v.event(session.Event{Type: "updated", Session: ptr(hybrid("live", "active"))}); !ok || e.Type != "created" {
		t.Fatalf("active again: %+v %v", e, ok)
	}
	if _, ok := v.event(session.Event{Type: "removed", ID: "gone"}); ok {
		t.Fatal("removal of a hidden shell sent")
	}
	if e, ok := v.event(session.Event{Type: "removed", ID: "plain"}); !ok || e.ID != "plain" {
		t.Fatalf("plain removal %+v %v", e, ok)
	}
}

func ptr(i session.Info) *session.Info { return &i }
