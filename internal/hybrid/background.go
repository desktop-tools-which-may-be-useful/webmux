package hybrid

import (
	"context"
	"encoding/json"
	"time"

	"github.com/rainchan/webmux/internal/acp"
	"github.com/rainchan/webmux/internal/attentionkind"
	"github.com/rainchan/webmux/internal/session"
)

// ACP v1 cannot say that work outlives its turn. Agents may leave the tool
// call running and report its end only with the next prompt (agy), or complete
// the call that only started the work without a later notification (Kimi).
// The turn is then not "done" yet; the actor watches the agent's process tree
// and asks the agent to refresh its status once those processes are gone.
const (
	bgPoll                 = time.Second
	bgQuiet                = 2 // consecutive polls without new processes
	bgMaxWatch             = time.Hour
	backgroundStatusPrompt = "The background processes from the previous turn have exited. Please check the latest background task status and report the outcome to the user. Do not start any new tasks."
)

// processSet is pid and its descendants.
func processSet(pid int) map[int]bool {
	set := map[int]bool{}
	for _, p := range session.ProcessTree(pid) {
		set[p] = true
	}
	return set
}

func (w *workTracker) noteBackground(u acp.Update) {
	var raw json.RawMessage
	switch u := u.(type) {
	case *acp.ToolCall:
		raw = u.RawInput
	case *acp.ToolCallUpdate:
		raw = u.RawInput
	default:
		return
	}
	var input struct {
		RunInBackground bool `json:"run_in_background"`
		IsDaemon        bool `json:"IsDaemon"`
	}
	if json.Unmarshal(raw, &input) == nil && (input.RunInBackground || input.IsDaemon) {
		w.background = true
	}
}

// snapshotProcesses records the agent's process tree at the start of a turn,
// so servers and helpers it keeps are not taken for the turn's work.
func (a *actor) snapshotProcesses() {
	a.stopBackgroundWatch()
	a.bgBase = nil
	if a.link != nil {
		a.bgBase = processSet(a.link.sidecar.PID())
	}
}

func (a *actor) stopBackgroundWatch() {
	if a.bgCancel != nil {
		a.bgCancel()
		a.bgCancel = nil
	}
}

// watchBackground polls the agent's process tree until no process started
// since the turn began is left, then starts a status refresh. A new turn, a
// reconnect or closing the session ends the watch.
func (a *actor) watchBackground() {
	link, base := a.link, a.bgBase
	if link == nil || base == nil {
		return
	}
	ctx, cancel := context.WithTimeout(a.probeCtx, bgMaxWatch)
	a.bgCancel = cancel
	pid := link.sidecar.PID()
	poll := a.svc.timing.bgPoll
	if poll <= 0 {
		poll = bgPoll
	}
	a.probes.Add(1)
	go func() {
		defer a.probes.Done()
		defer cancel()
		tick := time.NewTicker(poll)
		defer tick.Stop()
		quiet := 0
		for {
			select {
			case <-ctx.Done():
				return
			case <-tick.C:
			}
			if hasNewProcess(pid, base) {
				quiet = 0
				continue
			}
			if quiet++; quiet < bgQuiet {
				continue
			}
			a.post(func() { a.backgroundDone(link) })
			return
		}
	}()
}

func hasNewProcess(pid int, base map[int]bool) bool {
	for _, p := range session.ProcessTree(pid) {
		if !base[p] {
			return true
		}
	}
	return false
}

// backgroundDone starts one agent turn to flush and report the status that
// agents such as agy only release with a prompt. Its end latches done attention.
func (a *actor) backgroundDone(link *acpLink) {
	if a.link != link || a.turn != nil || a.attention != nil {
		return
	}
	if _, err := a.sendTurn(backgroundStatusPrompt, nil, "", "", true); err != nil {
		a.svc.log.Warn("background status refresh failed to start", "id", a.id, "error", err)
		a.emit(Event{Kind: EventNotice, Notice: &Notice{Level: acp.NoticeError, Text: "后台任务状态刷新失败，请发送消息重试"}})
		if a.attentionPolicy.Mode != session.AttentionModeOff {
			a.attention = &session.Attention{Kind: attentionkind.Error, Source: session.SourceAgent, Since: time.Now().UTC()}
			a.dirty = true
		}
	}
}
