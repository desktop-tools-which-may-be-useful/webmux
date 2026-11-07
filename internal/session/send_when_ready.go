package session

import (
	"context"
	"strings"
	"time"
)

const (
	// initialQuiet is how long the program's output must stay silent before
	// typed-in text is sent: a TUI that has drawn its prompt is waiting.
	initialQuiet = 700 * time.Millisecond
	// initialWait caps the wait for that silence; a program that never goes
	// quiet (a spinner) or never prints gets the text anyway.
	initialWait = 15 * time.Second
	// submitDelay separates the text from Enter, so a TUI that takes input
	// in bursts does not read the Enter as part of a paste.
	submitDelay = 200 * time.Millisecond
	pollEvery   = 50 * time.Millisecond
)

// SendWhenReady types text into the running program as a message and presses
// Enter, once the program has printed something and gone quiet. Multi-line
// text is pasted (bracketed when the program asked for that) so its line
// breaks are not read as Enter. It stops with ctx's error, or ErrNoRun when
// the session ends first.
func (s *Session) SendWhenReady(ctx context.Context, text string) error {
	if err := s.waitQuiet(ctx); err != nil {
		return err
	}
	s.mu.Lock()
	bracketed := s.preview.BracketedPaste()
	s.mu.Unlock()
	body := strings.ReplaceAll(strings.ReplaceAll(text, "\r\n", "\r"), "\n", "\r")
	if bracketed {
		body = "\x1b[200~" + body + "\x1b[201~"
	}
	if err := s.WriteContext(ctx, []byte(body)); err != nil {
		return err
	}
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-s.done:
		return ErrNoRun
	case <-time.After(submitDelay):
	}
	return s.WriteContext(ctx, []byte("\r"))
}

// waitQuiet returns when the current run has produced output and been silent
// for initialQuiet, or after initialWait.
func (s *Session) waitQuiet(ctx context.Context) error {
	deadline := time.NewTimer(initialWait)
	defer deadline.Stop()
	tick := time.NewTicker(pollEvery)
	defer tick.Stop()
	for {
		s.mu.Lock()
		r := s.run
		var written uint64
		if r != nil {
			_, written = r.ring.Range()
		}
		last := s.info.LastActiveAt
		s.mu.Unlock()
		if r == nil {
			return ErrNoRun
		}
		if written > 0 && time.Since(last) >= initialQuiet {
			return nil
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-s.done:
			return ErrNoRun
		case <-deadline.C:
			return nil
		case <-tick.C:
		}
	}
}
