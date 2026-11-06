//go:build linux || darwin

package session

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/rainchan/webmux/internal/attentionkind"
)

func TestRaiseAttentionOverridesAndClears(t *testing.T) {
	m := relationsManager(t, Options{})
	s, err := m.Create(CreateInput{Cmd: []string{"/bin/sh", "-c", "sleep 30"}})
	if err != nil {
		t.Fatal(err)
	}
	id := s.Info().ID
	sub := m.Subscribe()
	defer m.Unsubscribe(sub)

	for _, tc := range []struct {
		name    string
		kind    attentionkind.Kind
		message string
	}{
		{"permission is the agent's", attentionkind.Permission, ""},
		{"bell is the terminal's", attentionkind.Bell, ""},
		{"unknown", "nope", ""},
		{"long", attentionkind.Notify, strings.Repeat("长", MaxAttentionMessageRunes+1)},
		{"control", attentionkind.Notify, "a\nb"},
	} {
		if _, err := m.RaiseAttention(id, tc.kind, tc.message); !errors.Is(err, ErrInvalid) {
			t.Errorf("%s: %v, want ErrInvalid", tc.name, err)
		}
	}
	if _, err := m.RaiseAttention("missing1", attentionkind.Notify, ""); !errors.Is(err, ErrNotFound) {
		t.Fatalf("missing session: %v", err)
	}

	// A stronger layer's attention is replaced, whatever its rank.
	s.mu.Lock()
	s.agentEventLocked("permission_request", time.Now())
	s.mu.Unlock()
	info, err := m.RaiseAttention(id, attentionkind.Done, "  "+strings.Repeat("完", MaxAttentionMessageRunes)+" ")
	if err != nil {
		t.Fatal(err)
	}
	a := info.Attention
	if a == nil || a.Kind != attentionkind.Done || a.Source != SourceCLI || len([]rune(a.Message)) != MaxAttentionMessageRunes || info.Status != StatusWaiting {
		t.Fatalf("raised: %+v %s", a, info.Status)
	}
	waitEvent(t, sub, func(e Event) bool {
		return e.Session != nil && e.Session.Attention != nil && e.Session.Attention.Source == SourceCLI
	})
	// Bells and OSC 9 cannot replace it; the program's own events can.
	s.mu.Lock()
	s.raiseAttentionLocked(attentionkind.Bell, SourceTerminal, layerBell, time.Now())
	s.raiseAttentionLocked(attentionkind.Notify, SourceTerminal, layerNotify, time.Now())
	if s.info.Attention.Source != SourceCLI {
		t.Fatalf("a weaker layer replaced cli attention: %+v", s.info.Attention)
	}
	s.agentEventLocked("question_asked", time.Now())
	if s.info.Attention.Source != SourceAgent || s.info.Attention.Message != "" {
		t.Fatalf("the agent's question did not replace cli attention: %+v", s.info.Attention)
	}
	s.mu.Unlock()

	// Clearing removes attention from any source and is idempotent.
	if info, err := m.ClearAttention(id); err != nil || info.Attention != nil {
		t.Fatalf("clear: %+v %v", info.Attention, err)
	}
	if _, err := m.ClearAttention(id); err != nil {
		t.Fatalf("clear without attention: %v", err)
	}

	// Input clears it like user input.
	if _, err := m.RaiseAttention(id, attentionkind.Notify, "x"); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := s.WriteContext(ctx, []byte("y")); err != nil {
		t.Fatal(err)
	}
	if s.Info().Attention != nil {
		t.Fatal("input left cli attention latched")
	}
}

func TestRaiseAttentionRefusesExitedSessions(t *testing.T) {
	m := relationsManager(t, Options{})
	s, err := m.Create(CreateInput{Cmd: []string{"/bin/sh", "-c", "exit 0"}})
	if err != nil {
		t.Fatal(err)
	}
	<-s.Done()
	deadline := time.Now().Add(5 * time.Second)
	for s.Info().Status != StatusExited {
		if time.Now().After(deadline) {
			t.Fatal("session did not exit")
		}
		time.Sleep(10 * time.Millisecond)
	}
	if _, err := m.RaiseAttention(s.Info().ID, attentionkind.Notify, ""); !errors.Is(err, ErrExited) {
		t.Fatalf("exited session: %v", err)
	}
}

func TestRaiseAttentionOnTheChatSide(t *testing.T) {
	t.Parallel()
	m := shellManager(t)
	s := createShell(t, m, newFakeDriver())
	id := s.Info().ID
	s.SetOverlay(&Overlay{Hybrid: &HybridInfo{Side: "acp", State: "active"}, Status: StatusIdle})
	info, err := m.RaiseAttention(id, attentionkind.Question, "need a decision")
	if err != nil {
		t.Fatal(err)
	}
	if info.Attention == nil || info.Attention.Source != SourceCLI || info.Status != StatusWaiting {
		t.Fatalf("chat side: %+v %s", info.Attention, info.Status)
	}
	// The same overlay reported again keeps it; a new turn drops it.
	s.SetOverlay(&Overlay{Hybrid: &HybridInfo{Side: "acp", State: "active"}, Status: StatusIdle})
	if s.Info().Attention == nil {
		t.Fatal("an unchanged overlay dropped cli attention")
	}
	s.SetOverlay(&Overlay{Hybrid: &HybridInfo{Side: "acp", State: "active"}, Status: StatusRunning})
	if got := s.Info(); got.Attention != nil || got.Status != StatusRunning {
		t.Fatalf("a running turn kept cli attention: %+v %s", got.Attention, got.Status)
	}
	// A move to the other side drops it, and it does not come back.
	if _, err := m.RaiseAttention(id, attentionkind.Done, "old"); err != nil {
		t.Fatal(err)
	}
	s.SetOverlay(&Overlay{Hybrid: &HybridInfo{Side: "tui", State: "active"}})
	s.SetOverlay(&Overlay{Hybrid: &HybridInfo{Side: "acp", State: "active"}, Status: StatusIdle})
	if got := s.Info().Attention; got != nil {
		t.Fatalf("cli attention survived a side change: %+v", got)
	}
	if _, err := m.RaiseAttention(id, attentionkind.Done, ""); err != nil {
		t.Fatal(err)
	}
	if info, _ := m.ClearAttention(id); info.Attention != nil {
		t.Fatalf("clear on the chat side: %+v", info.Attention)
	}
}

func waitEvent(t *testing.T, sub *ListSubscriber, match func(Event) bool) {
	t.Helper()
	deadline := time.After(5 * time.Second)
	for {
		select {
		case e := <-sub.Out:
			if match(e) {
				return
			}
		case <-deadline:
			t.Fatal("no matching event")
		}
	}
}
