package sessioncmd

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/coder/websocket"

	"github.com/rainchan/webmux/internal/contract"
)

// replayStream serves frames over a real WebSocket and returns a chatStream
// reading them.
func replayStream(t *testing.T, frames ...contract.ChatFrame) *chatStream {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		c, err := websocket.Accept(w, r, nil)
		if err != nil {
			return
		}
		for _, f := range frames {
			data, _ := json.Marshal(f)
			if c.Write(r.Context(), websocket.MessageText, data) != nil {
				return
			}
		}
		<-r.Context().Done()
	}))
	t.Cleanup(srv.Close)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	t.Cleanup(cancel)
	conn, _, err := websocket.Dial(ctx, "ws"+strings.TrimPrefix(srv.URL, "http"), nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = conn.CloseNow() })
	return &chatStream{conn: conn}
}

func events(items ...contract.ChatEvent) contract.ChatFrame {
	return contract.ChatFrame{Type: contract.ChatFrameEvents, Items: items}
}

func turnStart(id string) contract.ChatEvent {
	return contract.ChatEvent{Kind: contract.ChatEventTurnStart, TurnStart: &contract.ChatTurnStart{TurnID: id, PromptID: "p" + id}}
}

func turnEnd(id, reason string) contract.ChatEvent {
	return contract.ChatEvent{Kind: contract.ChatEventTurnEnd, TurnEnd: &contract.ChatTurnEnd{TurnID: id, StopReason: reason}}
}

func agent(message, text string) contract.ChatEvent {
	return contract.ChatEvent{Kind: contract.ChatEventMessage, Message: &contract.ChatMessageChunk{MessageID: message, Role: "agent", Text: text}}
}

func request(id string) contract.ChatEvent {
	return contract.ChatEvent{Kind: contract.ChatEventRequest, Request: &contract.ChatRequest{RequestID: id, Kind: "permission"}}
}

func resolved(id string) contract.ChatEvent {
	return contract.ChatEvent{Kind: contract.ChatEventResolved, Resolved: &contract.ChatRequestOutcome{RequestID: id, Outcome: "selected"}}
}

// A steered turn skips earlier turns in the replay, and a request answered
// within the history does not stop the wait.
func TestTurnWaitFollowsTheRunningTurnPastHistory(t *testing.T) {
	stream := replayStream(t,
		events(turnStart("t1"), request("r1"), resolved("r1"), agent("m1", "old reply"), turnEnd("t1", "end_turn"),
			turnStart("t2"), request("r2"), resolved("r2"), agent("m2", "work")),
		events(agent("m3", "steered reply"), turnEnd("t2", "end_turn")),
	)
	result, err := stream.turnWait(context.Background(), turnWatch{current: "t2", stopAtRequest: true})
	if err != nil {
		t.Fatal(err)
	}
	if result.StopReason != "end_turn" || result.Reply != "steered reply" || result.Request != nil {
		t.Fatalf("result %+v", result)
	}
	for _, ev := range result.Events {
		if ev.Message != nil && ev.Message.Text == "old reply" {
			t.Fatal("an earlier turn's events were collected")
		}
	}
}

// When the replay no longer holds the running turn's start, the turn is
// adopted after the replay.
func TestTurnWaitAdoptsATruncatedRunningTurn(t *testing.T) {
	stream := replayStream(t,
		events(agent("m0", "older turn")),
		events(agent("m1", "live"), turnEnd("t9", "cancelled")),
	)
	result, err := stream.turnWait(context.Background(), turnWatch{current: "t9", stopAtRequest: true})
	if err != nil || result.StopReason != "cancelled" || result.Reply != "live" {
		t.Fatalf("result %+v, %v", result, err)
	}
}

func TestTurnWaitStopsAtAnOpenRequest(t *testing.T) {
	stream := replayStream(t,
		events(turnStart("t1"), agent("m1", "checking")),
		events(request("r1")),
	)
	result, err := stream.turnWait(context.Background(), turnWatch{accept: func(contract.ChatTurnStart) bool { return true }, stopAtRequest: true})
	if err != nil || result.Request == nil || result.Request.RequestID != "r1" || result.Reply != "checking" {
		t.Fatalf("result %+v, %v", result, err)
	}
}
