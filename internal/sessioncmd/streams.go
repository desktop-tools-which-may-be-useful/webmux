package sessioncmd

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/url"
	"slices"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/coder/websocket"
	"github.com/rainchan/webmux/internal/attach"
	"github.com/rainchan/webmux/internal/contract"
	"github.com/rainchan/webmux/internal/localclient"
)

// sessionStates are the values of --until and --wait.
var sessionStates = []string{"running", "waiting", "idle", "exited", "attention"}

// parseStates splits a comma-separated list of states.
func parseStates(value string) ([]string, error) {
	var out []string
	for part := range strings.SplitSeq(value, ",") {
		part = strings.TrimSpace(part)
		if !slices.Contains(sessionStates, part) {
			return nil, fmt.Errorf("unknown state %q (states: %s)", part, strings.Join(sessionStates, ", "))
		}
		out = append(out, part)
	}
	return out, nil
}

// inState reports whether s is in any of states.
func inState(s contract.SessionInfo, states []string) bool {
	for _, state := range states {
		if state == "attention" && s.Attention != nil {
			return true
		}
		if string(s.Status) == state {
			return true
		}
	}
	return false
}

// errSessionRemoved ends a wait whose session left the server.
var errSessionRemoved = errors.New("the session was removed")

// errTimeout ends a wait that ran out of time.
var errTimeout = errors.New("timed out")

// sessionWatch is one /ws/sessions subscription that tracks the latest
// SessionInfo of every session.
type sessionWatch struct {
	conn     *websocket.Conn
	sessions map[string]contract.SessionInfo
}

func watchSessions(ctx context.Context, client *localclient.Client) (*sessionWatch, error) {
	conn, err := client.DialWebSocket(ctx, "/ws/sessions")
	if err != nil {
		return nil, err
	}
	conn.SetReadLimit(64 << 20)
	return &sessionWatch{conn: conn, sessions: map[string]contract.SessionInfo{}}, nil
}

func (w *sessionWatch) close() { _ = w.conn.CloseNow() }

// next reads one event and applies it; it returns the event.
func (w *sessionWatch) next(ctx context.Context) (contract.SessionEvent, error) {
	_, data, err := w.conn.Read(ctx)
	if err != nil {
		return contract.SessionEvent{}, err
	}
	var event contract.SessionEvent
	if err := json.Unmarshal(data, &event); err != nil {
		return contract.SessionEvent{}, fmt.Errorf("session event: %w", err)
	}
	switch event.Type {
	case "snapshot":
		w.sessions = map[string]contract.SessionInfo{}
		for _, s := range event.Sessions {
			w.sessions[s.ID] = s
		}
	case "created", "updated":
		if event.Session != nil {
			w.sessions[event.Session.ID] = *event.Session
		}
	case "removed":
		delete(w.sessions, event.ID)
	}
	return event, nil
}

// waitOptions say when waitFor returns.
type waitOptions struct {
	ids    []string
	states []string
	// changed ignores the state at subscription: only later events count.
	changed bool
	// stable requires the condition to hold this long.
	stable time.Duration
	// any returns when one session matches instead of all.
	any bool
}

// waitFor waits on an open subscription until opts hold, ctx ends (with
// errTimeout when its deadline passed) or a session is removed. It returns
// the matching sessions' latest info.
func (w *sessionWatch) waitFor(ctx context.Context, opts waitOptions) ([]contract.SessionInfo, error) {
	seenSnapshot := false
	var since time.Time
	baseline := map[string]string{}
	for {
		readCtx := ctx
		var cancel context.CancelFunc
		if !since.IsZero() {
			readCtx, cancel = context.WithDeadline(ctx, since.Add(opts.stable))
		}
		event, err := w.next(readCtx)
		if cancel != nil {
			cancel()
		}
		stableElapsed := err != nil && !since.IsZero() && ctx.Err() == nil && time.Since(since) >= opts.stable
		if err != nil && !stableElapsed {
			if ctx.Err() != nil {
				if errors.Is(ctx.Err(), context.DeadlineExceeded) {
					return nil, errTimeout
				}
				return nil, ctx.Err()
			}
			return nil, err
		}
		if err == nil {
			if event.Type == "removed" && slices.Contains(opts.ids, event.ID) {
				return nil, fmt.Errorf("%w: %s", errSessionRemoved, event.ID)
			}
			if event.Type == "snapshot" {
				for _, id := range opts.ids {
					s, ok := w.sessions[id]
					if !ok {
						return nil, fmt.Errorf("%w: %s", errSessionRemoved, id)
					}
					baseline[id] = stateKey(s)
				}
				seenSnapshot = true
			}
		}
		if !seenSnapshot {
			continue
		}
		matched := w.matching(opts, baseline)
		done := len(matched) == len(opts.ids) || (opts.any && len(matched) > 0)
		switch {
		case !done:
			since = time.Time{}
		case opts.stable <= 0 || stableElapsed:
			return matched, nil
		case since.IsZero():
			since = time.Now()
		}
	}
}

// stateKey identifies the state a --changed wait must see change.
func stateKey(s contract.SessionInfo) string {
	key := string(s.Status)
	if s.Attention != nil {
		key += "+" + string(s.Attention.Kind)
	}
	return key
}

func (w *sessionWatch) matching(opts waitOptions, baseline map[string]string) []contract.SessionInfo {
	var out []contract.SessionInfo
	for _, id := range opts.ids {
		s, ok := w.sessions[id]
		if !ok || !inState(s, opts.states) {
			continue
		}
		// --changed: the state must differ from the one at subscription;
		// once it has, the baseline no longer applies.
		if opts.changed && baseline[id] == stateKey(s) {
			continue
		}
		if opts.changed {
			baseline[id] = ""
		}
		out = append(out, s)
	}
	return out
}

// observer is a terminal for attach.Run that never reads input and writes
// the session's output to w.
type observer struct {
	w io.Writer
}

func (o observer) ReadContext(ctx context.Context, _ []byte) (int, error) {
	<-ctx.Done()
	return 0, ctx.Err()
}
func (o observer) Write(p []byte) (int, error) { return o.w.Write(p) }
func (o observer) Size() (int, int, error)     { return 80, 24, nil }

// followTerminal streams session id's replay and output to w as an
// observer, without control or resizing, until the session exits or ctx
// ends.
func followTerminal(ctx context.Context, client *localclient.Client, id string, w io.Writer, label string) (attach.Result, error) {
	return attach.Run(ctx, client, id, observer{w: w}, attach.Options{Observe: true, Label: label})
}

// stripper removes terminal control sequences from a stream and keeps
// line text: CRLF becomes LF, a lone CR starts the line over, backspace
// deletes a rune. It is best effort and unsuitable for full-screen
// programs.
type stripper struct {
	w     io.Writer
	state int
	line  []byte
	cr    bool
	// skip counts the bytes of a charset designation still to drop.
	skip int
}

const (
	stripText = iota
	stripEscape
	stripCSI
	stripString // OSC, DCS, SOS, PM, APC: up to BEL or ESC \
	stripStringEscape
)

func newStripper(w io.Writer) *stripper { return &stripper{w: w} }

func (s *stripper) Write(p []byte) (int, error) {
	var out bytes.Buffer
	for _, b := range p {
		if s.skip > 0 {
			s.skip--
			continue
		}
		switch s.state {
		case stripEscape:
			switch b {
			case '[':
				s.state = stripCSI
			case ']', 'P', 'X', '^', '_':
				s.state = stripString
			case '(', ')', '*', '+', '-', '.', '/', '#', '%':
				s.state, s.skip = stripText, 1
			default:
				s.state = stripText
			}
			continue
		case stripCSI:
			if b >= 0x40 && b <= 0x7e {
				s.state = stripText
			}
			continue
		case stripString:
			switch b {
			case 0x07:
				s.state = stripText
			case 0x1b:
				s.state = stripStringEscape
			}
			continue
		case stripStringEscape:
			if b == '\\' {
				s.state = stripText
			} else {
				s.state = stripString
			}
			continue
		}
		if s.cr && b != '\n' && b != '\r' {
			// A lone CR: the line is written over from its start. The PTY
			// turns "\r\n" into "\r\r\n", so repeated CRs wait for what
			// follows them.
			s.line = s.line[:0]
		}
		s.cr = false
		switch {
		case b == 0x1b:
			s.state = stripEscape
		case b == '\r':
			s.cr = true
		case b == '\n':
			out.Write(s.line)
			out.WriteByte('\n')
			s.line = s.line[:0]
		case b == '\b':
			if len(s.line) > 0 {
				_, size := utf8.DecodeLastRune(s.line)
				s.line = s.line[:len(s.line)-size]
			}
		case b == '\t' || b >= 0x20 && b != 0x7f:
			s.line = append(s.line, b)
		}
	}
	if out.Len() > 0 {
		if _, err := s.w.Write(out.Bytes()); err != nil {
			return 0, err
		}
	}
	return len(p), nil
}

// Flush writes the unfinished last line.
func (s *stripper) Flush() error {
	if len(s.line) == 0 {
		return nil
	}
	_, err := s.w.Write(s.line)
	s.line = s.line[:0]
	return err
}

// chatStream is one /ws/s/{id}/chat subscription.
type chatStream struct {
	conn  *websocket.Conn
	state *contract.ChatState
	next  uint64
}

func openChat(ctx context.Context, client *localclient.Client, id string) (*chatStream, error) {
	conn, err := client.DialWebSocket(ctx, "/ws/s/"+url.PathEscape(id)+"/chat")
	if err != nil {
		return nil, err
	}
	conn.SetReadLimit(64 << 20)
	return &chatStream{conn: conn}, nil
}

func (c *chatStream) close() { _ = c.conn.CloseNow() }

func (c *chatStream) read(ctx context.Context) (contract.ChatFrame, error) {
	_, data, err := c.conn.Read(ctx)
	if err != nil {
		if websocket.CloseStatus(err) == 4404 {
			return contract.ChatFrame{}, errors.New("the session is not a hybrid session or no longer exists")
		}
		return contract.ChatFrame{}, err
	}
	var frame contract.ChatFrame
	if err := json.Unmarshal(data, &frame); err != nil {
		return contract.ChatFrame{}, fmt.Errorf("chat frame: %w", err)
	}
	if frame.State != nil {
		c.state = frame.State
	}
	if frame.Type == contract.ChatFrameSnapshot {
		c.next = frame.Next
	}
	return frame, nil
}

// turnResult is the outcome of one tracked turn.
type turnResult struct {
	StopReason string               `json:"stopReason"`
	Error      string               `json:"error,omitempty"`
	Reply      string               `json:"reply"`
	Events     []contract.ChatEvent `json:"events"`
	// Request is an open permission request or form when the wait stopped
	// for it.
	Request *contract.ChatRequest `json:"request,omitempty"`
}

// turnWait tracks the first turn whose turn.start passes accept and
// collects its events until turn.end, or until a request opens when
// stopAtRequest.
func (c *chatStream) turnWait(ctx context.Context, accept func(seq uint64) bool, stopAtRequest bool) (turnResult, error) {
	var result turnResult
	turn := ""
	messages := map[string]*strings.Builder{}
	var order []string
	for {
		frame, err := c.read(ctx)
		if err != nil {
			if errors.Is(ctx.Err(), context.DeadlineExceeded) {
				return result, errTimeout
			}
			return result, err
		}
		for _, ev := range frame.Items {
			if turn == "" {
				if ev.Kind == contract.ChatEventTurnStart && ev.TurnStart != nil && accept(ev.Seq) {
					turn = ev.TurnStart.TurnID
					result.Events = append(result.Events, ev)
				}
				continue
			}
			result.Events = append(result.Events, ev)
			switch ev.Kind {
			case contract.ChatEventMessage:
				if ev.Message != nil && ev.Message.Role == "agent" {
					b, ok := messages[ev.Message.MessageID]
					if !ok {
						b = &strings.Builder{}
						messages[ev.Message.MessageID] = b
						order = append(order, ev.Message.MessageID)
					}
					b.WriteString(ev.Message.Text)
				}
			case contract.ChatEventRequest:
				if stopAtRequest && ev.Request != nil {
					result.Request = ev.Request
					result.Reply = lastReply(messages, order)
					return result, nil
				}
			case contract.ChatEventTurnEnd:
				if ev.TurnEnd != nil && ev.TurnEnd.TurnID == turn {
					result.StopReason = ev.TurnEnd.StopReason
					result.Error = ev.TurnEnd.Error
					result.Reply = lastReply(messages, order)
					return result, nil
				}
			}
		}
	}
}

func lastReply(messages map[string]*strings.Builder, order []string) string {
	if len(order) == 0 {
		return ""
	}
	return strings.TrimSpace(messages[order[len(order)-1]].String())
}

// reportTurn prints a tracked turn and returns the exit status:
// end_turn is 0, a stop for a request or any other reason is 1.
func (e Env) reportTurn(result turnResult, asJSON bool) int {
	if asJSON {
		writeJSON(e.Stdout, result)
	} else if result.Reply != "" {
		_, _ = fmt.Fprintln(e.Stdout, result.Reply)
	}
	if result.Request != nil {
		_, _ = fmt.Fprintf(e.Stderr, "webmux: the agent is waiting for an answer: request %s (%s) %s; answer it in the Web UI\n",
			result.Request.RequestID, result.Request.Kind, result.Request.Title)
		return contract.ExitError
	}
	if result.StopReason != "end_turn" {
		reason := result.StopReason
		if result.Error != "" {
			reason += ": " + result.Error
		}
		_, _ = fmt.Fprintln(e.Stderr, "webmux: the turn ended with", reason)
		return contract.ExitError
	}
	return contract.ExitOK
}

func writeJSON(w io.Writer, v any) {
	encoder := json.NewEncoder(w)
	encoder.SetEscapeHTML(false)
	encoder.SetIndent("", "  ")
	_ = encoder.Encode(v)
}
