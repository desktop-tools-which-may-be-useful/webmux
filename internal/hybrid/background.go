package hybrid

import (
	"context"
	"time"

	"github.com/rainchan/webmux/internal/session"
)

// ACP v1 cannot say that work outlives its turn. Some agents (agy) end the
// turn with a backgrounded command still running and report its end only
// with the next prompt. The turn is then not "done" yet; the actor watches
// the agent's process tree and latches done once the processes that started
// during the turn are gone.
const (
	bgPoll     = time.Second
	bgQuiet    = 2 // consecutive polls without new processes
	bgMaxWatch = time.Hour
)

// processSet is pid and its descendants.
func processSet(pid int) map[int]bool {
	set := map[int]bool{}
	for _, p := range session.ProcessTree(pid) {
		set[p] = true
	}
	return set
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
// since the turn began is left, then latches done attention. A new turn, a
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

// backgroundDone latches done for a turn whose background work has ended,
// unless the session moved on meanwhile.
func (a *actor) backgroundDone(link *acpLink) {
	if a.link != link || a.turn != nil || a.attention != nil {
		return
	}
	a.scheduleOrLatchDone(a.chat.Turn.StartedAt, time.Now().UTC())
}
