package hybrid

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/rainchan/webmux/internal/session"
)

// InputRequest is input for a session, dispatched by its current view. On
// the terminal Text (with Enter when Submit) is one acknowledged submission
// and Keys follow; Attachments and Mode belong to the chat and fail there
// with ErrWrongSide. On the chat Text and Attachments are a prompt sent
// with Mode; Submit is ignored and Keys fail with ErrWrongSide. Label names
// the terminal controller while the input is written.
type InputRequest struct {
	ClientRequestID string
	Label           string
	Text            string
	Submit          bool
	Keys            [][]byte
	Attachments     []Attachment
	Mode            string
}

// InputResult is where input went: the side, and on the chat the prompt's
// outcome.
type InputResult struct {
	Side   Side
	Prompt PromptResult
}

// InputFailedError is terminal input that was not written whole; Status is
// its receipt.
type InputFailedError struct {
	Status session.SubmissionStatus
}

func (e *InputFailedError) Error() string {
	return fmt.Sprintf("%v: %s", ErrInputFailed, e.Status.State)
}
func (e *InputFailedError) Unwrap() error { return ErrInputFailed }

// terminalInterruptKey is what interrupts a terminal without an agent
// profile: Ctrl-C.
var terminalInterruptKey = []byte("\x03")

// TerminalInput writes in to an ordinary session's terminal, as Input does
// for a hybrid session on its terminal side. It needs no hybrid state, so s
// may be nil: ordinary sessions take input without hybrid sessions.
func (s *Service) TerminalInput(ctx context.Context, shell *session.Session, in InputRequest) (InputResult, error) {
	return terminalInput(ctx, shell, in)
}

func terminalInput(ctx context.Context, shell *session.Session, in InputRequest) (InputResult, error) {
	if len(in.Attachments) > 0 || in.Mode != "" {
		return InputResult{}, fmt.Errorf("%w: 附件与 mode 只适用于聊天视图", ErrWrongSide)
	}
	body := []byte(in.Text)
	var submit []byte
	if in.Submit {
		submit = []byte("\r")
	}
	if err := writeTerminal(ctx, shell, session.ServerInput{
		RequestID: in.ClientRequestID, Label: in.Label, Body: body, Submit: submit, Keys: in.Keys,
	}); err != nil {
		return InputResult{}, err
	}
	return InputResult{Side: SideTUI}, nil
}

// TerminalInterrupt writes Ctrl-C to an ordinary session's terminal; like
// TerminalInput, s may be nil.
func (s *Service) TerminalInterrupt(ctx context.Context, shell *session.Session, requestID, label string) error {
	return writeTerminal(ctx, shell, session.ServerInput{RequestID: requestID, Label: label, Keys: [][]byte{terminalInterruptKey}})
}

// writeTerminal writes server input and turns a receipt short of written
// into an *InputFailedError.
func writeTerminal(ctx context.Context, shell *session.Session, in session.ServerInput) error {
	status, err := shell.SubmitServer(ctx, in)
	switch {
	case errors.Is(err, session.ErrNoRun):
		return &InputFailedError{Status: session.SubmissionStatus{ID: in.RequestID, State: session.SubmissionMissing}}
	case err != nil:
		return err
	case status.State != session.SubmissionWritten:
		return &InputFailedError{Status: status}
	}
	return nil
}

// view is the side hybrid session id settled on, for a command that acts
// on it: ErrNotActive once it is closed or interrupted, ErrSwitching while
// it moves.
func (s *Service) view(ctx context.Context, id string) (*actor, Side, error) {
	a, err := s.actor(id)
	if err != nil {
		return nil, "", err
	}
	side, err := call(ctx, a, func() (Side, error) {
		switch {
		case a.state != StateActive || a.closing:
			return "", ErrNotActive
		case a.side == SideSwitching:
			return "", ErrSwitching
		}
		return a.side, nil
	})
	return a, side, err
}

// Input sends in to hybrid session id on its current side.
func (s *Service) Input(ctx context.Context, id string, in InputRequest) (InputResult, error) {
	a, side, err := s.view(ctx, id)
	if err != nil {
		return InputResult{}, err
	}
	if side == SideTUI {
		return terminalInput(ctx, a.shell, in)
	}
	if len(in.Keys) > 0 {
		return InputResult{}, fmt.Errorf("%w: 按键只适用于终端视图", ErrWrongSide)
	}
	result, err := s.Prompt(ctx, id, PromptInput{ClientRequestID: in.ClientRequestID, Text: in.Text, Mode: in.Mode, Attachments: in.Attachments})
	return InputResult{Side: SideACP, Prompt: result}, err
}

// Interrupt stops what hybrid session id is doing on its current side: on
// the chat it cancels the running turn (nothing without one), on the
// terminal it writes the profile's interrupt key presses.
func (s *Service) Interrupt(ctx context.Context, id, requestID, label string) error {
	a, side, err := s.view(ctx, id)
	if err != nil {
		return err
	}
	if side == SideACP {
		return s.Cancel(ctx, id, requestID)
	}
	keys := a.cmd.Agent.InterruptKeys()
	for i, key := range keys {
		if i > 0 {
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(interruptKeyGap):
			}
		}
		// Each press is its own receipt; a retry of the request finds every
		// press recorded and writes none of them again.
		in := session.ServerInput{RequestID: fmt.Sprintf("%s-%d", requestID, i), Label: label, Keys: [][]byte{key}}
		if err := writeTerminal(ctx, a.shell, in); err != nil {
			return err
		}
	}
	return nil
}

// HasChatView reports whether session id has a chat view: a running hybrid
// session or an archive record.
func (s *Service) HasChatView(id string) bool {
	if _, err := s.actor(id); err == nil {
		return true
	}
	_, err := s.Record(id)
	return err == nil
}
