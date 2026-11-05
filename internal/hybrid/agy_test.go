package hybrid

import (
	"context"
	"slices"
	"strings"
	"testing"

	"github.com/rainchan/webmux/internal/acp"
	"github.com/rainchan/webmux/internal/agentprofile/agy"
)

func TestAgyFormElicitationMultiSelect(t *testing.T) {
	h := newHarness(t, 0)
	h.fast()
	s := h.create(h.input(agy.Name, SideACP, "FAKEAGENT_ELICIT=1"))
	id := s.Info().ID
	st := h.follow(id)
	prompt(t, h, id, "ask me")
	events := st.until("form request", hasKind(EventRequest))
	req := events[len(events)-1].Request
	if req.Kind != RequestForm || req.Form == nil || !req.Form.Supported || req.Form.Message != "The agent has 3 questions." {
		t.Fatalf("request %+v, form %+v", req, req.Form)
	}
	var q2 *FormField
	for i := range req.Form.Fields {
		if req.Form.Fields[i].Name == "q2" {
			q2 = &req.Form.Fields[i]
		}
	}
	if q2 == nil || q2.Type != "array" || !slices.Equal(q2.Enum, []string{"lint", "unit", "e2e"}) {
		t.Fatalf("q2 field %+v", q2)
	}
	err := h.svc.Answer(ctxT(t), id, rid(), req.RequestID, Answer{
		Outcome: acp.ElicitAccept,
		Content: map[string]any{
			"q1": "staging",
			"q2": []string{"lint", "e2e"},
			"q3": "ship it",
		},
	})
	if err != nil {
		t.Fatalf("answer elicitation: %v", err)
	}
	events = st.until("turn end", hasKind(EventTurnEnd))
	var reply strings.Builder
	for _, e := range events {
		if e.Kind == EventMessage && e.Message.Role == RoleAgent {
			reply.WriteString(e.Message.Text)
		}
	}
	if got := reply.String(); !strings.Contains(got, "(q1=staging q2=[lint e2e] q3=ship it)") {
		t.Fatalf("agent reply %q", got)
	}
}

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
