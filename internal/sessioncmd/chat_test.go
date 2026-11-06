package sessioncmd

import (
	"bufio"
	"strings"
	"testing"

	"github.com/rainchan/webmux/internal/contract"
)

func TestChatTextRendersEvents(t *testing.T) {
	var out strings.Builder
	w := bufio.NewWriter(&out)
	text := chatText{w: w}
	title, status := "Read go.mod", "completed"
	for _, ev := range []contract.ChatEvent{
		{TurnStart: &contract.ChatTurnStart{TurnID: "t1", PromptID: "p1"}},
		{Message: &contract.ChatMessageChunk{MessageID: "u1", Role: "user", Text: "hi"}},
		{Message: &contract.ChatMessageChunk{MessageID: "a1", Role: "thought", Text: "hidden"}},
		{Message: &contract.ChatMessageChunk{MessageID: "a2", Role: "agent", Text: "hel"}},
		{Message: &contract.ChatMessageChunk{MessageID: "a2", Role: "agent", Text: "lo"}},
		{Tool: &contract.ChatTool{ToolCallID: "c1", Title: &title, Status: &status}},
		{Request: &contract.ChatRequest{RequestID: "r1", Kind: "permission", Title: "Run make"}},
		{Resolved: &contract.ChatRequestOutcome{RequestID: "r1", Outcome: "selected"}},
		{TurnEnd: &contract.ChatTurnEnd{TurnID: "t1", StopReason: "error", Error: "boom"}},
	} {
		text.event(ev)
	}
	_ = w.Flush()
	want := "--- turn t1 started\nuser: hi\nagent: hello\ntool c1: Read go.mod (completed)\n" +
		"request r1 (permission): Run make\nrequest r1 closed: selected\n--- turn t1 ended: error: boom\n"
	if out.String() != want {
		t.Fatalf("got\n%s\nwant\n%s", out.String(), want)
	}
}

func TestQueuedGone(t *testing.T) {
	gone := queuedGone("q2")
	state := func(ids ...string) *contract.ChatState {
		s := &contract.ChatState{}
		for _, id := range ids {
			s.Queue.Items = append(s.Queue.Items, contract.ChatQueueItem{ID: id})
		}
		return s
	}
	// The snapshot before the prompt was queued does not count.
	if err := gone(state("q1")); err != nil {
		t.Fatal(err)
	}
	if err := gone(state("q1", "q2")); err != nil {
		t.Fatal(err)
	}
	if err := gone(state("q1")); err == nil || !strings.Contains(err.Error(), "removed") {
		t.Fatalf("removed item: %v", err)
	}
}

func TestSideNames(t *testing.T) {
	for name, side := range map[string]contract.HybridSide{"chat": contract.HybridSideACP, "terminal": contract.HybridSideTUI} {
		got, ok := sideNamed(name)
		if !ok || got != side || sideName(side) != name {
			t.Errorf("%s: %q %v %q", name, got, ok, sideName(side))
		}
	}
	if _, ok := sideNamed("tui"); ok {
		t.Error("tui is not a CLI side name")
	}
}
