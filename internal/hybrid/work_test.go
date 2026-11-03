package hybrid

import (
	"strings"
	"testing"

	"github.com/rainchan/webmux/internal/agentprofile/omp"
)

func strPtr(s string) *string { return &s }

func TestWorkLinePrefersLatestActiveTool(t *testing.T) {
	var w workTracker
	pending := "pending"
	inProgress := "in_progress"
	done := "completed"
	read := "read"
	execute := "execute"
	w.tool(Tool{ToolCallID: "a", Kind: &read, Status: &pending})
	w.tool(Tool{ToolCallID: "b", Kind: &execute, Status: &inProgress})
	if got := w.line(); got != "执行" {
		t.Fatalf("kind fallback without title/locations: %q", got)
	}
	// A title arrives for the earlier call; the newest active call wins.
	w.tool(Tool{ToolCallID: "a", Title: strPtr("Read README.md")})
	if got := w.line(); got != "执行" {
		t.Fatalf("latest active tool should win: %q", got)
	}
	// The execute call finishes; the read tool is still pending.
	w.tool(Tool{ToolCallID: "b", Status: &done})
	if got := w.line(); got != "Read README.md" {
		t.Fatalf("falling back to the remaining active call: %q", got)
	}
}

func TestWorkLineToolTitleWins(t *testing.T) {
	var w workTracker
	inProgress := "in_progress"
	read := "read"
	title := "read the config\ncarefully"
	loc := []Location{{Path: "/repo/config.toml"}}
	w.tool(Tool{ToolCallID: "a", Kind: &read, Title: &title, Status: &inProgress, Locations: loc})
	if got := w.line(); got != "read the config carefully" {
		t.Fatalf("title kept on one line: %q", got)
	}
}

func TestWorkLineKindAndLocationFallback(t *testing.T) {
	var w workTracker
	pending := "pending"
	edit := "edit"
	w.tool(Tool{ToolCallID: "a", Kind: &edit, Status: &pending, Locations: []Location{{Path: "/repo/main.go"}}})
	if got := w.line(); got != "编辑 /repo/main.go" {
		t.Fatalf("kind label and location: %q", got)
	}
}

func TestWorkLinePlanFallback(t *testing.T) {
	var w workTracker
	w.setPlan(&Plan{Entries: []PlanEntry{
		{Content: "先写测试", Status: "completed"},
		{Content: "实现登录\n流程", Status: "in_progress"},
	}})
	if got := w.line(); got != "计划：实现登录 流程" {
		t.Fatalf("plan fallback: %q", got)
	}
	// A pending tool call takes precedence over the plan.
	pending := "pending"
	w.tool(Tool{ToolCallID: "a", Status: &pending, Title: strPtr("Run tests")})
	if got := w.line(); got != "Run tests" {
		t.Fatalf("tool wins over plan: %q", got)
	}
}

func TestWorkLineTruncation(t *testing.T) {
	var w workTracker
	inProgress := "in_progress"
	long := strings.Repeat("很长的标题", 40)
	w.tool(Tool{ToolCallID: "a", Status: &inProgress, Title: &long})
	got := w.line()
	if n := len([]rune(got)); n != workMaxRunes {
		t.Fatalf("truncated to %d runes, got %d: %q", workMaxRunes, n, got)
	}
	if !strings.HasSuffix(got, "…") {
		t.Fatalf("truncated line ends with ellipsis: %q", got)
	}
}

func TestWorkLineClearedWhenNothingActive(t *testing.T) {
	var w workTracker
	failed := "failed"
	w.tool(Tool{ToolCallID: "a", Status: &failed, Title: strPtr("Run tests")})
	if got := w.line(); got != "" {
		t.Fatalf("finished tool is not work: %q", got)
	}
}

func TestWorkEndTurnDropsInterruptedCalls(t *testing.T) {
	var w workTracker
	inProgress := "in_progress"
	pending := "pending"
	w.tool(Tool{ToolCallID: "a", Status: &inProgress, Title: strPtr("Run tests")})
	w.tool(Tool{ToolCallID: "b", Status: &pending, Title: strPtr("Read README.md")})
	w.endTurn()
	if got := w.line(); got != "" {
		t.Fatalf("interrupted calls are not work: %q", got)
	}
}

func TestHybridWorkDetailDuringTurn(t *testing.T) {
	h := newHarness(t, 0)
	s := h.create(h.input(omp.Name, SideACP, "FAKEAGENT_TURN_DELAY=300ms"))
	id := s.Info().ID
	st := h.follow(id)
	prompt(t, h, id, "hello")
	waitFor(t, "work detail", func() bool {
		i := s.Info()
		return i.Hybrid != nil && i.Hybrid.Work == "Read file"
	})
	st.until("turn end", hasKind(EventTurnEnd))
	waitFor(t, "work cleared", func() bool {
		i := s.Info()
		return i.Hybrid != nil && i.Hybrid.Work == ""
	})
}
