package hybrid

import (
	"context"
	"fmt"
	"time"

	"github.com/rainchan/webmux/internal/attentionkind"
	"github.com/rainchan/webmux/internal/session"
)

const (
	// idlePoll is how often a switch looks at the terminal UI's status
	// while waiting for it to go idle.
	idlePoll = 50 * time.Millisecond
	// keyTimeout bounds writing the interrupt key.
	keyTimeout = 5 * time.Second
)

type terminalTurnState int

const (
	tuiTurnIdle terminalTurnState = iota
	tuiTurnBusy
	tuiTurnUnknown
)

func (t terminalTurnState) String() string {
	switch t {
	case tuiTurnBusy:
		return "busy"
	case tuiTurnUnknown:
		return "unknown"
	default:
		return "idle"
	}
}

// tuiTurnState interprets session status only for a terminal-to-chat switch.
// A heuristic wait with no attention may be either an idle base prompt or an
// in-turn terminal read, so it requires an explicit choice before interruption.
func tuiTurnState(info session.Info) terminalTurnState {
	switch info.Status {
	case session.StatusRunning:
		return tuiTurnBusy
	case session.StatusWaiting:
		if info.ProgramTurnBlocked {
			return tuiTurnBusy
		}
		if info.Attention == nil {
			return tuiTurnUnknown
		}
		if info.Attention.Kind != attentionkind.Done && info.Attention.Kind != attentionkind.Error {
			return tuiTurnBusy
		}
	}
	return tuiTurnIdle
}

// initialTUITurnState ignores an agent's running status during its startup
// quiet period, but never a wait for user intervention.
func initialTUITurnState(info session.Info, startupQuiet bool) terminalTurnState {
	turn := tuiTurnState(info)
	if turn == tuiTurnBusy && startupQuiet && info.Status == session.StatusRunning {
		return tuiTurnIdle
	}
	return turn
}

// tuiBusy reports whether the terminal UI might still be in a turn. Existing
// wait/close paths conservatively keep treating an unknown wait as busy; only
// the initial switch decision requires a user choice.
func tuiBusy(info session.Info) bool { return tuiTurnState(info) != tuiTurnIdle }

// interruptKeyGap separates key presses so the terminal UI reads each one
// separately; the M0 baseline stopped OpenCode with Esc, 150 ms, Esc.
const interruptKeyGap = 150 * time.Millisecond

// interruptKey writes the profile's interrupt key presses to the terminal UI.
func (a *actor) interruptKey() {
	for i, key := range a.cmd.Agent.InterruptKeys() {
		if i > 0 {
			time.Sleep(interruptKeyGap)
		}
		ctx, cancel := context.WithTimeout(context.Background(), keyTimeout)
		err := a.shell.WriteContext(ctx, key)
		cancel()
		if err != nil {
			a.svc.log.Info("hybrid interrupt key not written", "id", a.id, "error", err)
			return
		}
	}
}

// waitIdle waits up to d for the terminal UI to leave its turn.
func (a *actor) waitIdle(d time.Duration) bool {
	deadline := time.NewTimer(d)
	defer deadline.Stop()
	tick := time.NewTicker(idlePoll)
	defer tick.Stop()
	for {
		if !tuiBusy(a.shell.Info()) {
			return true
		}
		select {
		case <-tick.C:
		case <-deadline.C:
			return !tuiBusy(a.shell.Info())
		case <-a.quit:
			return false
		}
	}
}

// interruptOutcome is how interrupting the terminal UI ended.
type interruptOutcome int

const (
	interruptIdle interruptOutcome = iota
	interruptCancelled
	interruptGone
)

// interruptTUI sends the interrupt key and waits for the terminal UI to go
// idle. After timing.interrupt it asks the user (D21): wait another
// period, force the switch, or cancel it. The terminal going idle while
// the question is open continues the switch.
func (a *actor) interruptTUI(op *switchOp) interruptOutcome {
	a.interruptKey()
	for {
		if a.waitIdle(a.svc.timing.interrupt) {
			return interruptIdle
		}
		choice := make(chan string, 1)
		if !a.sync(func() { op.choice = choice; a.setPhase(PhaseAwaitingChoice) }) {
			return interruptGone
		}
		c := a.awaitChoice(op, choice)
		if !a.sync(func() {
			op.choice = nil
			if c == ChoiceWait {
				a.setPhase(PhaseInterrupting)
			}
		}) {
			return interruptGone
		}
		switch c {
		case ChoiceWait:
			continue
		case ChoiceCancel:
			return interruptCancelled
		case "":
			return interruptGone
		}
		return interruptIdle
	}
}

// awaitChoice waits for the user's choice; "idle" when the terminal UI
// went idle first, "" when the actor ended. The terminal going idle wins
// only while the question is still open: deciding so closes it on the
// actor, so a choice is either acted on or refused, never lost.
func (a *actor) awaitChoice(op *switchOp, choice chan string) string {
	tick := time.NewTicker(idlePoll)
	defer tick.Stop()
	for {
		select {
		case c := <-choice:
			return c
		case <-tick.C:
			if tuiBusy(a.shell.Info()) {
				continue
			}
			idle := false
			if !a.sync(func() {
				if op.choice == choice {
					op.choice, idle = nil, true
				}
			}) {
				return ""
			}
			if idle {
				return "idle"
			}
			// choose answered first and closed the question.
			return <-choice
		case <-a.quit:
			return ""
		}
	}
}

// choose answers a switch paused for a terminal turn decision or an interrupt
// timeout. Each choice is valid only in the phase that gives it meaning.
func (a *actor) choose(choice string) (SwitchResult, error) {
	op := a.switching
	if op == nil || op.choice == nil {
		return SwitchResult{}, fmt.Errorf("%w: 没有等待选择的切换", ErrConflict)
	}
	switch a.chat.Phase {
	case PhaseAwaitingTurnChoice:
		if choice == ChoiceWait {
			return SwitchResult{}, fmt.Errorf("%w: wait 只用于中断超时", ErrConflict)
		}
	case PhaseAwaitingChoice:
		if choice == ChoiceInterrupt {
			return SwitchResult{}, fmt.Errorf("%w: interrupt 只用于未知回合状态", ErrConflict)
		}
	default:
		return SwitchResult{}, fmt.Errorf("%w: 没有等待选择的切换", ErrConflict)
	}
	op.choice <- choice
	op.choice = nil
	if choice == ChoiceWait || choice == ChoiceInterrupt {
		// The question is closed now; no device may answer it again.
		a.setPhase(PhaseInterrupting)
	}
	if choice == ChoiceCancel {
		return a.resultOn(SideTUI), nil
	}
	// The switch goes on; the result names where it is heading.
	return SwitchResult{Side: op.to, Dropped: []string{}, Unsupported: []string{}}, nil
}

// runUnknownToACP waits for the explicit turn-state choice. It deliberately
// does not poll terminal idleness: only the user's answer resolves ambiguity.
func (a *actor) runUnknownToACP(op *switchOp, t target, choice <-chan string) {
	var selected string
	select {
	case selected = <-choice:
	case <-a.quit:
		return
	}
	switch selected {
	case ChoiceCancel:
		a.sync(func() { a.switchCancelled(op) })
	case ChoiceInterrupt:
		a.runToACP(op, t, true)
	case ChoiceForce:
		a.runToACP(op, t, false)
	}
}
