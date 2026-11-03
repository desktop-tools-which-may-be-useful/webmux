package hybrid

import (
	"context"
	"slices"
	"strings"
	"testing"

	"github.com/rainchan/webmux/internal/agentprofile/agy"
)

func TestAgyRoundTripAndConfigReapply(t *testing.T) {
	h := newHarness(t, 0)
	h.fast()
	in := h.input(agy.Name, SideTUI)
	in.Cmd = append(in.Cmd, "--dangerously-skip-permissions", "--mode", "plan")
	s := h.create(in)
	id := s.Info().ID
	if cmd := s.Info().Cmd; !slices.Contains(cmd, "--settings") || !slices.Contains(cmd, agy.TUISettingsJSON) {
		t.Fatalf("terminal command %q lacks injected --settings", cmd)
	}
	h.waitReason(id, ReasonNoSessionID)
	tuiTurn(t, s, "one")
	h.waitEligible(id)
	sid := *s.Info().Hybrid.AgentSessionID
	if len(sid) != 36 {
		t.Fatalf("expected 36-char UUID session ID, got %q", sid)
	}

	result := h.mustSwitch(id, SideACP)
	if !slices.Equal(result.Unsupported, []string{"--mode"}) {
		t.Fatalf("unsupported %q, want [--mode]", result.Unsupported)
	}
	if got := h.replayedUsers(id); !slices.Equal(got, []string{"one"}) {
		t.Fatalf("replay after first switch %q", got)
	}

	ctx, cancel := context.WithTimeout(context.Background(), testTimeout)
	defer cancel()
	for _, kv := range []struct{ id, val string }{{"model", "gemini-3.8-flash"}, {"thought_level", "low"}} {
		val := kv.val
		if err := h.svc.SetConfig(ctx, id, ConfigInput{ClientRequestID: rid(), ConfigID: kv.id, Value: &val}); err != nil {
			t.Fatalf("set config %s=%s: %v", kv.id, kv.val, err)
		}
	}

	chatTurn(t, h, id, "two")
	// Verify echoed user_message_chunk during active turn did not duplicate the user message.
	if got := h.replayedUsers(id); !slices.Equal(got, []string{"one", "two"}) {
		t.Fatalf("user messages after chat turn %q, want [one two]", got)
	}

	h.mustSwitch(id, SideTUI)
	waitFor(t, "resumed transcript", func() bool { return strings.Contains(string(s.Buffer()), "> two") })
	tuiTurn(t, s, "three")
	h.waitEligible(id)
	if got := *s.Info().Hybrid.AgentSessionID; got != sid {
		t.Fatalf("resumed terminal session %s, want %s", got, sid)
	}

	h.mustSwitch(id, SideACP)
	if got := h.replayedUsers(id); !slices.Equal(got, []string{"one", "two", "three"}) {
		t.Fatalf("replay after round trip %q", got)
	}
	state := h.chatState(id)
	if configValue(state.Config, "model") != "gemini-3.8-flash" || configValue(state.Config, "thought_level") != "low" {
		t.Fatalf("config after cold load %+v, want gemini-3.8-flash and low", state.Config)
	}
	assertSingleWriter(t, h.store)
}
