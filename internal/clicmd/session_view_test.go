package clicmd

import (
	"testing"

	"github.com/rainchan/webmux/internal/contract"
)

func TestViewChoicePending(t *testing.T) {
	id := "session1"
	for _, tt := range []struct {
		name string
		info contract.SessionInfo
		want bool
	}{
		{name: "unknown turn", info: contract.SessionInfo{ID: id, Transition: &contract.SessionTransition{Phase: contract.PhaseAwaitingTurnChoice}}, want: true},
		{name: "interrupt timeout", info: contract.SessionInfo{ID: id, Transition: &contract.SessionTransition{Phase: contract.PhaseAwaitingChoice}}, want: true},
		{name: "other phase", info: contract.SessionInfo{ID: id, Transition: &contract.SessionTransition{Phase: contract.PhaseInterrupting}}},
		{name: "other session", info: contract.SessionInfo{ID: "session2", Transition: &contract.SessionTransition{Phase: contract.PhaseAwaitingTurnChoice}}},
	} {
		t.Run(tt.name, func(t *testing.T) {
			if got := viewChoicePending(tt.info, id); got != tt.want {
				t.Fatalf("viewChoicePending() = %v, want %v", got, tt.want)
			}
		})
	}
	if got := automaticViewChoice(false); got != contract.ViewChoiceCancel {
		t.Fatalf("choice without force = %q", got)
	}
	if got := automaticViewChoice(true); got != contract.ViewChoiceForce {
		t.Fatalf("choice with force = %q", got)
	}
}
