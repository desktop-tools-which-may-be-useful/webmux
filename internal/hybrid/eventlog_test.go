package hybrid

import (
	"errors"
	"slices"
	"strings"
	"testing"
	"time"
	"unicode/utf8"

	"github.com/rainchan/webmux/internal/acp"
)

func textEvent(text string) Event {
	return Event{Kind: EventMessage, Message: &MessageChunk{MessageID: "m", Role: RoleAgent, Text: text}}
}

func TestChatLogResume(t *testing.T) {
	l := newChatLog(1<<20, ChatState{Agent: "omp"})
	now := time.Now()
	for range 5 {
		l.append(now, textEvent("x"))
	}
	fresh, err := l.subscribe(nil)
	if err != nil {
		t.Fatal(err)
	}
	if s := fresh.Snapshot; s.From != 1 || s.Next != 6 || s.Reset || s.Truncated || len(fresh.Events) != 5 || s.State.Agent != "omp" {
		t.Fatalf("fresh %+v, %d events", s, len(fresh.Events))
	}
	tail, _ := l.subscribe(&ResumePoint{Generation: fresh.Snapshot.Generation, Seq: 4})
	if tail.Snapshot.Reset || len(tail.Events) != 2 || tail.Events[0].Seq != 4 {
		t.Fatalf("resume within the log: %+v %d", tail.Snapshot, len(tail.Events))
	}
	current, _ := l.subscribe(&ResumePoint{Generation: fresh.Snapshot.Generation, Seq: 6})
	if current.Snapshot.Reset || len(current.Events) != 0 {
		t.Fatalf("resume at the end: %+v", current.Snapshot)
	}
	other, _ := l.subscribe(&ResumePoint{Generation: "other", Seq: 3})
	if !other.Snapshot.Reset || len(other.Events) != 5 {
		t.Fatalf("resume of another generation: %+v", other.Snapshot)
	}
	// Live frames follow the snapshot without a gap.
	l.append(now, textEvent("y"))
	l.setState(ChatState{Agent: "opencode"})
	f := <-tail.Out
	if len(f.Events) != 1 || f.Events[0].Seq != 6 || f.Generation != fresh.Snapshot.Generation {
		t.Fatalf("live events %+v", f)
	}
	if f = <-tail.Out; f.State == nil || f.State.Agent != "opencode" {
		t.Fatalf("live state %+v", f)
	}
	l.restart()
	if <-tail.Done; !errors.Is(tail.Err(), ErrReset) {
		t.Fatalf("restart ended subscriber with %v", tail.Err())
	}
	after, _ := l.subscribe(&ResumePoint{Generation: fresh.Snapshot.Generation, Seq: 6})
	if !after.Snapshot.Reset || after.Snapshot.Generation == fresh.Snapshot.Generation || len(after.Events) != 0 || after.Snapshot.From != 7 {
		t.Fatalf("after restart %+v", after.Snapshot)
	}
	l.end()
	if <-after.Done; !errors.Is(after.Err(), ErrEnded) {
		t.Fatalf("end: %v", after.Err())
	}
	if _, err := l.subscribe(nil); !errors.Is(err, ErrNotFound) {
		t.Fatalf("subscribe to an ended log: %v", err)
	}
}

func TestChatLogDropsOldestByBytes(t *testing.T) {
	one := eventSize(textEvent(strings.Repeat("a", 100)))
	l := newChatLog(3*one, ChatState{})
	for range 5 {
		l.append(time.Now(), textEvent(strings.Repeat("a", 100)))
	}
	sub, _ := l.subscribe(nil)
	if s := sub.Snapshot; !s.Truncated || s.From != 3 || s.Next != 6 || len(sub.Events) != 3 {
		t.Fatalf("snapshot %+v, %d events", s, len(sub.Events))
	}
	// The newest event stays even when it alone exceeds the budget.
	l.append(time.Now(), textEvent(strings.Repeat("b", 10*one)))
	sub, _ = l.subscribe(nil)
	if len(sub.Events) != 1 || sub.Snapshot.From != 6 {
		t.Fatalf("oversized newest event: %+v", sub.Snapshot)
	}
}

func chunk(id, role, text string) Event {
	return Event{Kind: EventMessage, Message: &MessageChunk{MessageID: id, Role: role, Text: text}}
}

func marker() Event { return Event{Kind: EventMarker, Marker: &Marker{Kind: MarkerLoaded}} }

// replayText applies events the way the web client does: a message's text
// is the concatenation of its chunks, and an event at or below the applied
// sequence is ignored. It returns the texts by message ID and the next
// expected sequence.
func replayText(events []Event, next uint64) (map[string]string, uint64) {
	out := map[string]string{}
	for _, e := range events {
		if e.Seq < next {
			continue
		}
		next = e.Seq + 1
		if e.Message != nil {
			out[e.Message.MessageID] += e.Message.Text
		}
	}
	return out, next
}

func TestChatLogMergesFinishedChunks(t *testing.T) {
	l := newChatLog(1<<20, ChatState{})
	now := time.Now()
	live, _ := l.subscribe(nil)
	l.append(now, chunk("a", RoleAgent, "he"), chunk("a", RoleAgent, "llo "), chunk("a", RoleAgent, "wörld"))
	// A run still streaming is stored chunk by chunk.
	if s, _ := l.subscribe(nil); len(s.Events) != 3 {
		t.Fatalf("open run merged: %d events", len(s.Events))
	}
	l.append(now, marker())
	s, _ := l.subscribe(nil)
	if len(s.Events) != 2 || s.Snapshot.From != 1 || s.Snapshot.Next != 5 {
		t.Fatalf("merged log: %d events, %+v", len(s.Events), s.Snapshot)
	}
	m := s.Events[0]
	if m.Seq != 3 || m.Message.Text != "hello wörld" || m.Message.MessageID != "a" || m.Message.Role != RoleAgent || !m.At.Equal(now) {
		t.Fatalf("merged event %+v %+v", m, m.Message)
	}
	// The subscriber that was connected all along got every chunk live.
	if len(live.Out) != 2 {
		t.Fatalf("live frames: %d", len(live.Out))
	}
	if got, _ := replayText(s.Events, 1); got["a"] != "hello wörld" {
		t.Fatalf("replayed text %q", got["a"])
	}
}

func TestChatLogKeepsUnmergeableChunks(t *testing.T) {
	l := newChatLog(1<<20, ChatState{})
	withFile := chunk("c", RoleAgent, "x")
	withFile.Message.Attachments = []Attachment{{Path: "/a"}}
	l.append(time.Now(),
		chunk("u", RoleUser, "one"), chunk("u", RoleUser, "two"),
		chunk("a", RoleAgent, "x"), chunk("b", RoleAgent, "y"),
		chunk("t", RoleThought, "p"), chunk("t", RoleAgent, "q"),
		withFile, withFile, marker())
	s, _ := l.subscribe(nil)
	if len(s.Events) != 9 {
		t.Fatalf("expected no merge, got %d events", len(s.Events))
	}
}

func TestChatLogResumeInsideMergedRun(t *testing.T) {
	l := newChatLog(1<<20, ChatState{})
	l.append(time.Now(), chunk("a", RoleAgent, "ab"), chunk("a", RoleAgent, "é"), chunk("a", RoleAgent, "cd"), marker())
	gen := l.generation
	for _, row := range []struct {
		seq  uint64
		text string
		n    int
	}{{1, "abécd", 2}, {2, "écd", 2}, {3, "cd", 2}, {4, "", 1}, {5, "", 0}} {
		s, err := l.subscribe(&ResumePoint{Generation: gen, Seq: row.seq})
		if err != nil {
			t.Fatal(err)
		}
		if s.Snapshot.Reset || len(s.Events) != row.n {
			t.Fatalf("resume %d: reset %v, %d events", row.seq, s.Snapshot.Reset, len(s.Events))
		}
		if row.text != "" && (s.Events[0].Message.Text != row.text || s.Events[0].Seq != 3) {
			t.Fatalf("resume %d: %q at %d", row.seq, s.Events[0].Message.Text, s.Events[0].Seq)
		}
	}
	// A client that applied the first chunk ends up with the whole text.
	prefix, next := replayText([]Event{{Seq: 1, Message: &MessageChunk{MessageID: "a", Text: "ab"}}}, 1)
	s, _ := l.subscribe(&ResumePoint{Generation: gen, Seq: next})
	rest, _ := replayText(s.Events, next)
	if prefix["a"]+rest["a"] != "abécd" {
		t.Fatalf("client text %q", prefix["a"]+rest["a"])
	}
	// The log still starts at the first chunk: an older resume point resets.
	if s, _ := l.subscribe(&ResumePoint{Generation: gen, Seq: 0}); !s.Snapshot.Reset {
		t.Fatal("resume before the log did not reset")
	}
}

func TestChatLogDropsMergedEventAsAWhole(t *testing.T) {
	l := newChatLog(1<<20, ChatState{})
	l.append(time.Now(), chunk("a", RoleAgent, strings.Repeat("a", 100)), chunk("a", RoleAgent, strings.Repeat("a", 100)), marker())
	merged := eventSize(chunk("a", RoleAgent, strings.Repeat("a", 200)))
	l.capacity = merged + eventSize(marker()) - 1
	l.append(time.Now(), marker())
	s, _ := l.subscribe(nil)
	if !s.Snapshot.Truncated || s.Snapshot.From != 3 || len(s.Events) != 2 {
		t.Fatalf("after drop: %+v, %d events", s.Snapshot, len(s.Events))
	}
}

func toolEvent(id, output string) Event {
	title, status := "Read file", "completed"
	return Event{Kind: EventTool, Tool: &Tool{ToolCallID: id, Title: &title, Status: &status,
		Locations: []Location{{Path: "/a.go"}}, Content: []Content{{Type: "text", Text: output}}}}
}

func toolText(e Event) string { return e.Tool.Content[0].Text }

func TestChatLogDegradesOldToolOutput(t *testing.T) {
	big := strings.Repeat("x", 10<<10)
	full := eventSize(toolEvent("t", big))
	l := newChatLog(3*full, ChatState{})
	old := toolEvent("old", big)
	// Under budget nothing is shortened.
	l.append(time.Now(), old, toolEvent("mid", big))
	held, _ := l.subscribe(nil)
	if s := held.Events; len(s) != 2 || len(toolText(s[0])) != len(big) || s[0].Tool.Content[0].Truncated {
		t.Fatal("output shortened without pressure")
	}
	// Over budget the oldest outputs are shortened until the log fits.
	l.append(time.Now(), toolEvent("new", big), toolEvent("last", big))
	s, _ := l.subscribe(nil)
	if s.Snapshot.Truncated || len(s.Events) != 4 {
		t.Fatalf("events dropped before output was shortened: %+v, %d", s.Snapshot, len(s.Events))
	}
	first := s.Events[0]
	if got := toolText(first); len(got) != degradedToolBytes || !first.Tool.Content[0].Truncated {
		t.Fatalf("old output: %d bytes, truncated %v", len(got), first.Tool.Content[0].Truncated)
	}
	if *first.Tool.Title != "Read file" || *first.Tool.Status != "completed" || first.Tool.Locations[0].Path != "/a.go" || first.Seq != 1 || first.Tool.ToolCallID != "old" {
		t.Fatalf("degraded event lost its identity: %+v", first.Tool)
	}
	if len(toolText(s.Events[3])) != len(big) {
		t.Fatal("newest output was shortened")
	}
	// A subscriber that already holds the event keeps what it received.
	if len(toolText(held.Events[0])) != len(big) || held.Events[0].Tool.Content[0].Truncated {
		t.Fatal("stored event changed under a subscriber")
	}
}

func TestChatLogDegradeKeepsUTF8AndDiffs(t *testing.T) {
	old := strings.Repeat("é", degradedToolBytes) // two bytes each: the limit falls inside no rune
	odd := strings.Repeat("a", degradedToolBytes-1) + "é"
	diff := Event{Kind: EventTool, Tool: &Tool{ToolCallID: "d", Content: []Content{
		{Type: "diff", Path: "a.go", OldText: &old, NewText: odd},
		{Type: "text", Text: "short"},
	}}}
	short, ok := degradeTool(diff)
	if !ok {
		t.Fatal("long diff not shortened")
	}
	c := short.Tool.Content
	if !utf8.ValidString(c[0].NewText) || len(c[0].NewText) != degradedToolBytes-1 || len(*c[0].OldText) != degradedToolBytes || !c[0].Truncated || c[0].Path != "a.go" {
		t.Fatalf("diff %d %d %v", len(c[0].NewText), len(*c[0].OldText), c[0].Truncated)
	}
	if c[1].Truncated || c[1].Text != "short" {
		t.Fatal("short block changed")
	}
	if len(*diff.Tool.Content[0].OldText) != 2*degradedToolBytes {
		t.Fatal("input event modified")
	}
	if _, ok := degradeTool(short); ok {
		t.Fatal("degrading twice changed the event again")
	}
}

func TestChatLogOutputDoesNotCrowdOutConversation(t *testing.T) {
	// A synthetic session: every turn is a prompt, a read with a large
	// output, streamed reply chunks and a turn end. A log that only drops
	// the oldest events keeps fewer turns than one that shortens old output
	// and merges finished chunks.
	const turns = 400
	reply := strings.Repeat("The reply explains the change in a few short sentences. ", 4)
	var raw []Event
	for i := range turns {
		raw = append(raw, Event{Kind: EventTurnStart, TurnStart: &TurnStart{TurnID: "t" + strings.Repeat("0", 8) + string(rune('a'+i%26))}})
		raw = append(raw, toolEvent("call", strings.Repeat("line of file output\n", 400)))
		for off := 0; off < len(reply); off += 12 {
			raw = append(raw, chunk("m"+string(rune('a'+i%26)), RoleAgent, reply[off:min(off+12, len(reply))]))
		}
		raw = append(raw, Event{Kind: EventTurnEnd, TurnEnd: &TurnEnd{StopReason: "end_turn"}})
	}
	const capacity = 512 << 10
	naive, total := 0, 0
	for i := len(raw) - 1; i >= 0; i-- {
		if total += eventSize(raw[i]); total > capacity {
			break
		}
		if raw[i].Kind == EventTurnStart {
			naive++
		}
	}
	l := newChatLog(capacity, ChatState{})
	for _, e := range raw {
		l.append(time.Now(), e)
	}
	s, _ := l.subscribe(nil)
	kept := 0
	for _, e := range s.Events {
		if e.Kind == EventTurnStart {
			kept++
		}
	}
	if kept < naive*3/2 {
		t.Fatalf("log keeps %d turns, a drop-only log keeps %d", kept, naive)
	}
	if !s.Snapshot.Truncated || l.bytes > capacity {
		t.Fatalf("log is not bounded: %d bytes, truncated %v", l.bytes, s.Snapshot.Truncated)
	}
	t.Logf("turns kept: %d, drop-only: %d", kept, naive)
}

func TestChatLogDegradeCursorSurvivesMerge(t *testing.T) {
	// A long streamed run that crosses the protected part of the log
	// collapses into one event. The scan position for shortening tool
	// output must follow, or later tool output is evicted instead of shortened.
	l := newChatLog(8000, ChatState{})
	now := time.Now()
	for range 50 {
		l.append(now, chunk("a", RoleAgent, ""))
	}
	l.append(now, marker()) // seals the run
	l.append(now, marker())
	if l.degraded > len(l.events) {
		t.Fatalf("scan position %d beyond %d events", l.degraded, len(l.events))
	}
	// Shortening the oldest output (about 1 KiB each) is enough to fit.
	l.capacity = 9600
	for _, id := range []string{"t1", "t2", "t3"} {
		l.append(now, toolEvent(id, strings.Repeat("x", 3000)))
	}
	s, _ := l.subscribe(nil)
	got := map[string]int{}
	for _, e := range s.Events {
		if e.Tool != nil {
			got[e.Tool.ToolCallID] = len(toolText(e))
		}
	}
	if got["t1"] != degradedToolBytes || got["t3"] != 3000 {
		t.Fatalf("tool output lengths %v: the oldest should be shortened and the newest kept", got)
	}
}

func TestChatLogShortensOversizedNewestOutputBeforeDroppingConversation(t *testing.T) {
	// The protected window is by size: a newest event that alone exceeds it
	// is outside it. Shortening that output keeps the older conversation.
	l := newChatLog(5200, ChatState{})
	now := time.Now()
	for range 5 {
		l.append(now, marker())
	}
	l.append(now, toolEvent("big", strings.Repeat("x", 6000)))
	s, _ := l.subscribe(nil)
	if s.Snapshot.Truncated || len(s.Events) != 6 {
		t.Fatalf("conversation dropped: %+v, %d events", s.Snapshot, len(s.Events))
	}
	if got := toolText(s.Events[5]); len(got) != degradedToolBytes {
		t.Fatalf("newest output %d bytes", len(got))
	}
}

func TestChatLogConcurrentAppendAndSubscribe(t *testing.T) {
	// Merging, shortening and dropping rewrite the log while subscribers
	// read it; run with -race. Every subscriber must see a consistent text.
	big := strings.Repeat("o", 6<<10)
	l := newChatLog(64<<10, ChatState{})
	done := make(chan struct{})
	go func() {
		defer close(done)
		for i := range 300 {
			id := string(rune('a' + i%26))
			l.append(time.Now(), chunk(id, RoleAgent, "ab"), chunk(id, RoleAgent, "cd"), toolEvent("c", big), chunk(id, RoleThought, "x"), marker())
		}
	}()
	for {
		select {
		case <-done:
			return
		default:
		}
		s, err := l.subscribe(nil)
		if err != nil {
			t.Fatal(err)
		}
		next := uint64(0)
		for _, e := range s.Events {
			if e.Seq < next {
				t.Fatalf("sequences go backwards: %d after %d", e.Seq, next)
			}
			next = e.Seq + 1
			if e.Message != nil && e.Message.Role == RoleAgent && len(e.Message.Text) != 2 && e.Message.Text != "abcd" {
				t.Fatalf("message text %q", e.Message.Text)
			}
		}
		s.Close()
	}
}

func TestChatLogDropsSlowSubscriber(t *testing.T) {
	l := newChatLog(1<<20, ChatState{})
	sub, _ := l.subscribe(nil)
	for range subscriberQueue + 1 {
		l.append(time.Now(), textEvent("x"))
	}
	select {
	case <-sub.Done:
	default:
		t.Fatal("slow subscriber kept")
	}
	if !errors.Is(sub.Err(), ErrSlow) {
		t.Fatalf("slow subscriber ended with %v", sub.Err())
	}
	sub.Close() // after the drop: no effect
}

func TestCapTextKeepsUTF8(t *testing.T) {
	s := strings.Repeat("a", maxFieldBytes-1) + "é" + "tail"
	got, cut := capText(s)
	if !cut || len(got) != maxFieldBytes-1 || !utf8.ValidString(got) {
		t.Fatalf("cut %v at %d bytes", cut, len(got))
	}
	if got, cut := capText("short"); cut || got != "short" {
		t.Fatal("short text changed")
	}
}

func TestSessionTitleTrimsAndBounds(t *testing.T) {
	if got := sessionTitle("  chat title \n"); got != "chat title" {
		t.Fatalf("trimmed title %q", got)
	}
	if got := sessionTitle("   "); got != "" {
		t.Fatalf("blank title %q", got)
	}
	if got := sessionTitle(strings.Repeat("a", maxTitleBytes+10)); len(got) != maxTitleBytes {
		t.Fatalf("title kept %d bytes", len(got))
	}
	// A cut inside a multi-byte rune backs off to the previous boundary.
	edge := strings.Repeat("a", maxTitleBytes-1) + "é"
	if got := sessionTitle(edge); len(got) != maxTitleBytes-1 || !utf8.ValidString(got) {
		t.Fatalf("title cut at %d bytes, valid %v", len(got), utf8.ValidString(got))
	}
}

func TestTranslateContentAndMessages(t *testing.T) {
	var tr translator
	big := strings.Repeat("z", maxFieldBytes+1)
	old := "before"
	status := acp.ToolCompleted
	events := tr.update(&acp.ToolCallUpdate{ToolCallID: "c", Status: &status, Content: []acp.ToolCallContent{
		{Type: acp.ToolContentDiff, Path: "a.go", OldText: &old, NewText: big},
		{Type: acp.ToolContentTerminal, TerminalID: "t"},
		{Type: acp.ToolContentContent, Content: &acp.ContentBlock{Type: acp.ContentImage}},
		{Type: acp.ToolContentContent, Content: &acp.ContentBlock{Type: acp.ContentResourceLink, URI: "file:///a", Name: "a"}},
	}})
	c := events[0].Tool.Content
	if !c[0].Truncated || len(c[0].NewText) != maxFieldBytes || *c[0].OldText != old || c[0].Type != "diff" {
		t.Fatalf("diff %+v", c[0].Type)
	}
	if c[1].Type != "unsupported" || c[1].Unsupported != "terminal" || c[2].Unsupported != "image" || c[3].Type != "resource_link" || c[3].URI != "file:///a" {
		t.Fatalf("content %+v", c[1:])
	}
	if events[0].Tool.Title != nil || events[0].Tool.Locations != nil {
		t.Fatal("patch carries unchanged fields")
	}
	// Chunks without a messageId share an ID until another event or role.
	chunk := func(u acp.Update) string { return tr.update(u)[0].Message.MessageID }
	a := chunk(&acp.AgentMessageChunk{Content: acp.TextBlock("a")})
	b := chunk(&acp.AgentMessageChunk{Content: acp.TextBlock("b")})
	th := chunk(&acp.AgentThoughtChunk{Content: acp.TextBlock("t")})
	c2 := chunk(&acp.AgentMessageChunk{Content: acp.TextBlock("c")})
	tr.update(&acp.UsageUpdate{Used: 1, Size: 2})
	d := chunk(&acp.AgentMessageChunk{Content: acp.TextBlock("d")})
	given := chunk(&acp.AgentMessageChunk{Content: acp.TextBlock("e"), MessageID: "msg_9"})
	// A thought sharing the reply's ACP messageId is its own entry.
	thought := chunk(&acp.AgentThoughtChunk{Content: acp.TextBlock("f"), MessageID: "msg_9"})
	again := chunk(&acp.AgentMessageChunk{Content: acp.TextBlock("g"), MessageID: "msg_9"})
	if a != b || a == th || th == c2 || c2 == a || d == c2 || given != "msg_9" || thought == given || again != given {
		t.Fatalf("message IDs %q %q %q %q %q %q %q %q", a, b, th, c2, d, given, thought, again)
	}
	if text := tr.update(&acp.UserMessageChunk{Content: acp.ContentBlock{Type: acp.ContentAudio}})[0].Message.Text; !strings.Contains(text, "audio") {
		t.Fatalf("unsupported message content %q", text)
	}
	if tr.update(&acp.UnknownUpdate{Kind: "future"}) != nil {
		t.Fatal("unknown update produced events")
	}
	comp := tr.update(&acp.CompactionUpdate{CompactionID: "k", Status: acp.CompactionInProgress})[0].Compaction
	sum := tr.update(&acp.CompactionSummaryChunk{CompactionID: "k", Content: acp.TextBlock("sum")})[0].Compaction
	doneC := tr.update(&acp.CompactionUpdate{CompactionID: "k", Status: acp.CompactionCancelled})[0].Compaction
	if comp.Status != "running" || sum.Summary != "sum" || doneC.Status != "failed" || doneC.Summary != "sum" {
		t.Fatalf("compaction %+v %+v %+v", comp, sum, doneC)
	}
}

func TestFormRequest(t *testing.T) {
	r := formRequest("r1", acp.CreateElicitationRequest{Mode: acp.ElicitationForm, Message: "m", RequestedSchema: &acp.ElicitationSchema{
		Properties: map[string]acp.ElicitationProperty{
			"name":  {Type: "string", Default: []byte(`"x"`)},
			"color": {Type: "string", OneOf: []acp.EnumOption{{Const: "r", Title: "Red"}}},
			"ok":    {Type: "boolean"},
			"tags":  {Type: "array", Items: &acp.MultiSelectItems{AnyOf: []acp.EnumOption{{Const: "a", Title: "Alpha"}}}},
			"plain": {Type: "array", Items: &acp.MultiSelectItems{Type: "string", Enum: []string{"x", "y"}}},
		},
		Required: []string{"name"},
	}})
	f := r.Form
	if r.Kind != RequestForm || !f.Supported || len(f.Fields) != 5 || f.Fields[0].Name != "color" || f.Fields[0].Type != "enum" || f.Fields[0].EnumNames[0] != "Red" {
		t.Fatalf("form %+v", f)
	}
	if f.Fields[1].Name != "name" || !f.Fields[1].Required || string(f.Fields[1].Default) != `"x"` || f.Fields[2].Type != "boolean" {
		t.Fatalf("fields %+v", f.Fields)
	}
	if f.Fields[3].Name != "plain" || f.Fields[3].Type != "array" || !slices.Equal(f.Fields[3].Enum, []string{"x", "y"}) {
		t.Fatalf("plain array %+v", f.Fields[3])
	}
	if f.Fields[4].Name != "tags" || f.Fields[4].Type != "array" || !slices.Equal(f.Fields[4].Enum, []string{"a"}) || !slices.Equal(f.Fields[4].EnumNames, []string{"Alpha"}) {
		t.Fatalf("titled array %+v", f.Fields[4])
	}
	r = formRequest("r2", acp.CreateElicitationRequest{Mode: acp.ElicitationForm, RequestedSchema: &acp.ElicitationSchema{
		Properties: map[string]acp.ElicitationProperty{"n": {Type: "number"}},
	}})
	if r.Form.Supported || r.Form.Fields[0].Type != "unsupported" {
		t.Fatalf("number field %+v", r.Form)
	}
	r = formRequest("r3", acp.CreateElicitationRequest{Mode: acp.ElicitationForm, RequestedSchema: &acp.ElicitationSchema{
		Properties: map[string]acp.ElicitationProperty{"empty": {Type: "array"}},
	}})
	if r.Form.Supported || r.Form.Fields[0].Type != "unsupported" {
		t.Fatalf("empty array field %+v", r.Form)
	}
}

func TestStopReason(t *testing.T) {
	for _, row := range []struct {
		resp acp.PromptResponse
		err  error
		stop string
	}{
		{acp.PromptResponse{StopReason: acp.StopRefusal}, nil, acp.StopRefusal},
		{acp.PromptResponse{StopReason: "weird"}, nil, StopError},
		{acp.PromptResponse{}, &acp.RPCError{Code: acp.CodeAuthRequired, Message: "x"}, StopAuthRequired},
		{acp.PromptResponse{}, &acp.RPCError{Code: -32603, Message: "Authentication required: provider"}, StopAuthRequired},
		{acp.PromptResponse{}, &acp.RPCError{Code: -32603, Message: "Upstream request failed"}, StopError},
		{acp.PromptResponse{}, acp.ErrDisconnected, acp.StopCancelled},
	} {
		if stop, _ := stopReason(row.resp, row.err); stop != row.stop {
			t.Fatalf("%+v %v: %s", row.resp, row.err, stop)
		}
	}
}
