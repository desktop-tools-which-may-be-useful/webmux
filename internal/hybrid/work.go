package hybrid

import (
	"strings"
	"unicode/utf8"
)

// workMaxRunes bounds one work-detail line sent to the session card.
const workMaxRunes = 120

// maxTrackedTools bounds how many tool calls the work tracker keeps; the
// oldest finished ones are dropped first.
const maxTrackedTools = 100

// toolState is what the work detail keeps of one tool call.
type toolState struct {
	kind      string
	title     string
	status    string
	locations []string
}

// workTracker tracks tool calls and the plan to derive one readable
// "what is happening now" line for the session card. Owned by the actor.
type workTracker struct {
	tools map[string]toolState
	order []string
	plan  *Plan
}

// tool merges one tool patch into the tracker.
func (w *workTracker) tool(t Tool) {
	if t.ToolCallID == "" {
		return
	}
	if w.tools == nil {
		w.tools = map[string]toolState{}
	}
	prev, ok := w.tools[t.ToolCallID]
	if !ok {
		w.order = append(w.order, t.ToolCallID)
	}
	if t.Kind != nil {
		prev.kind = *t.Kind
	}
	if t.Title != nil {
		prev.title = *t.Title
	}
	if t.Status != nil {
		prev.status = *t.Status
	}
	if t.Locations != nil {
		prev.locations = prev.locations[:0]
		for _, l := range t.Locations {
			prev.locations = append(prev.locations, l.Path)
		}
	}
	w.tools[t.ToolCallID] = prev
	w.prune()
}

// prune drops the oldest finished tool calls once the tracker exceeds
// maxTrackedTools; if none finished, the oldest in progress goes.
func (w *workTracker) prune() {
	for len(w.order) > maxTrackedTools {
		i := 0
		for ; i < len(w.order); i++ {
			s := w.tools[w.order[i]].status
			if s == "completed" || s == "failed" {
				break
			}
		}
		if i == len(w.order) {
			i = 0
		}
		delete(w.tools, w.order[i])
		w.order = append(w.order[:i], w.order[i+1:]...)
	}
}

// endTurn marks every unfinished tool call as failed, so the next turn's
// work line does not resurrect an interrupted call.
func (w *workTracker) endTurn() {
	for i := range w.order {
		t := w.tools[w.order[i]]
		if t.status == "pending" || t.status == "in_progress" {
			t.status = "failed"
			w.tools[w.order[i]] = t
		}
	}
}

// setPlan records the latest plan.
func (w *workTracker) setPlan(p *Plan) { w.plan = p }

// line is the current work, or "" when nothing is in flight. A running or
// pending tool call takes precedence over the plan's in-progress step.
func (w *workTracker) line() string {
	for i := len(w.order) - 1; i >= 0; i-- {
		t := w.tools[w.order[i]]
		if t.status == "pending" || t.status == "in_progress" {
			return renderTool(t)
		}
	}
	if w.plan != nil {
		for _, e := range w.plan.Entries {
			if e.Status == "in_progress" {
				return truncateRunes("计划："+oneLine(e.Content), workMaxRunes)
			}
		}
	}
	return ""
}

// renderTool makes one readable line out of a tool call: the agent's own
// title when it gave one, otherwise a Chinese label for the kind and the
// first location it touches.
func renderTool(t toolState) string {
	if title := oneLine(t.title); title != "" {
		return truncateRunes(title, workMaxRunes)
	}
	label := kindWorkLabel(t.kind)
	if len(t.locations) > 0 {
		return truncateRunes(label+" "+oneLine(t.locations[0]), workMaxRunes)
	}
	return label
}

func kindWorkLabel(kind string) string {
	switch kind {
	case "read":
		return "读取"
	case "edit":
		return "编辑"
	case "delete":
		return "删除"
	case "move":
		return "移动"
	case "search":
		return "搜索"
	case "execute":
		return "执行"
	case "think":
		return "思考"
	case "fetch":
		return "获取"
	case "switch_mode":
		return "切换模式"
	default:
		return "调用工具"
	}
}

// oneLine collapses all whitespace so a title with newlines stays one line.
func oneLine(s string) string {
	return strings.Join(strings.Fields(s), " ")
}

func truncateRunes(s string, max int) string {
	if utf8.RuneCountInString(s) <= max {
		return s
	}
	r := []rune(s)
	return string(r[:max-1]) + "…"
}
