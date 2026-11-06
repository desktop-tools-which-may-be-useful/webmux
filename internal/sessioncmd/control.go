package sessioncmd

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"slices"
	"strings"
	"time"

	"github.com/0magnet/xterm-go/vt"
	"github.com/coder/websocket"
	"github.com/rainchan/webmux/internal/contract"
	"github.com/rainchan/webmux/internal/localclient"
)

// exitsCommon are the statuses every server command shares.
func exitsCommon(extra ...ExitItem) []ExitItem {
	out := []ExitItem{{Code: 0, Text: "Done."}}
	out = append(out, extra...)
	return append(out,
		ExitItem{Code: 2, Text: "Usage error."},
		ExitItem{Code: 3, Text: "WebMux is not running."},
	)
}

var apiCommand = &Command{
	Name: "api", Group: GroupSessions, Args: "PATH",
	Summary: "Send one request to the local API and print the response body unchanged.",
	Description: "The escape hatch for endpoints without a dedicated command: any /api/ or /local/v1/ path, over the local socket, " +
		"with no token. The method is GET, or POST when -d is given. The body must be JSON; multipart uploads are not supported. " +
		"Restrictions on callers inside a session (stopping the server, reading the token, tunnel and Tailscale) still apply. " +
		"WebSocket paths are refused: use webmux events, logs, screen or chat watch.",
	Arguments: []Item{{Name: "PATH", Text: "An /api/ or /local/v1/ path, with an optional query string."}},
	Output:    "The response body as the server sent it; a non-2xx error message goes to standard error.",
	Exit:      exitsCommon(ExitItem{Code: 1, Text: "The server answered with a non-2xx status."}),
	Examples: []Example{
		{Command: "webmux api /api/launch-configs", Text: "List the saved launch configs as JSON."},
		{Command: `webmux api -X PATCH /api/sessions/abcd2345 -d '{"name":"review"}'`, Text: "Rename a session through the raw API."},
		{Command: "webmux api -X DELETE /api/launch-configs/k3m9q2x7", Text: "Delete a launch config by ID."},
	},
	SeeAlso: []string{"webmux help", "docs/protocol.md in the repository"},
	Bind: func(fs *flag.FlagSet) Action {
		method := fs.String("X", "", "HTTP `METHOD` (default GET, or POST with -d)")
		data := fs.String("d", "", "JSON request body: literal `DATA`, @FILE, or - for standard input")
		return func(ctx context.Context, e Env, args []string) int {
			if len(args) != 1 {
				return e.Usagef("exactly one path is required")
			}
			path := args[0]
			switch {
			case strings.HasPrefix(path, "/ws/"):
				return e.Usagef("%s is a WebSocket; use webmux events, logs, screen or chat watch", path)
			case !strings.HasPrefix(path, "/api/") && !strings.HasPrefix(path, "/local/v1/"):
				return e.Usagef("the path must start with /api/ or /local/v1/")
			}
			var body []byte
			if *data != "" {
				var err error
				if body, err = e.readData(*data); err != nil {
					return e.Usagef("-d: %v", err)
				}
				if !json.Valid(body) {
					return e.Usagef("-d is not valid JSON")
				}
			}
			m := strings.ToUpper(*method)
			if m == "" {
				m = http.MethodGet
				if body != nil {
					m = http.MethodPost
				}
			}
			client, code := e.connect()
			if client == nil {
				return code
			}
			defer client.Close()
			status, response, err := client.Raw(ctx, m, path, body)
			if err != nil {
				return e.Fail(err)
			}
			_, _ = e.Stdout.Write(response)
			if len(response) > 0 && !bytes.HasSuffix(response, []byte("\n")) {
				_, _ = io.WriteString(e.Stdout, "\n")
			}
			if status < 200 || status > 299 {
				var failure contract.ErrorResponse
				_ = json.Unmarshal(response, &failure)
				_, _ = fmt.Fprintf(e.Stderr, "webmux api: HTTP %d %s\n", status, failure.Error)
				return contract.ExitError
			}
			return contract.ExitOK
		}
	},
}

// readData reads -d: a literal, @FILE or - for standard input.
func (e Env) readData(value string) ([]byte, error) {
	switch {
	case value == "-":
		if e.Stdin == nil {
			return nil, errors.New("standard input is not available")
		}
		return io.ReadAll(e.Stdin)
	case strings.HasPrefix(value, "@"):
		return os.ReadFile(value[1:])
	}
	return []byte(value), nil
}

// sessionAction declares a command that sends one request about a session.
func sessionAction(name, summary, description, method, query string, examples []Example) *Command {
	return &Command{
		Name: name, Group: GroupSessions, Args: "SESSION", Summary: summary, Description: description,
		Arguments: []Item{sessionArgument}, Env: []Item{sessionIDEnv},
		Exit:     exitsCommon(ExitItem{Code: 1, Text: "No such session, or the server refused (for example a hybrid session that is switching)."}),
		Examples: examples, SeeAlso: []string{"webmux ls", "webmux get"},
		Bind: func(fs *flag.FlagSet) Action {
			return func(ctx context.Context, e Env, args []string) int {
				if len(args) != 1 {
					return e.Usagef("exactly one session ID is required")
				}
				id, code, ok := e.SessionID(args[0])
				if !ok {
					return code
				}
				client, code := e.connect()
				if client == nil {
					return code
				}
				defer client.Close()
				err := client.Do(ctx, method, "/api/sessions/"+url.PathEscape(id)+query, nil, nil)
				if query == removeQuery && isStatus(err, http.StatusConflict) {
					err = endAndRemove(ctx, client, id)
				}
				if err != nil {
					return e.Fail(err)
				}
				return contract.ExitOK
			}
		},
	}
}

var killCommand = sessionAction("kill", "End a session's process; the session stays listed as exited.",
	"The process group gets SIGHUP, then SIGKILL if it does not exit. Use rm to also remove the session.",
	http.MethodDelete, "", []Example{
		{Command: "webmux kill abcd2345", Text: "Stop a runaway command."},
		{Command: "webmux ls --parent self --status running --json | jq -r '.[].id' | xargs -n1 webmux kill", Text: "End every running session this session created."},
	})

var rmCommand = sessionAction("rm", "Remove a session; a running one is ended first.",
	"The session disappears from every client. Its artifacts folder is deleted unless a hybrid record can still resume it.",
	http.MethodDelete, removeQuery, []Example{
		{Command: "webmux rm abcd2345", Text: "Remove a session."},
		{Command: "webmux ls --label run=42 --json | jq -r '.[].id' | xargs -n1 webmux rm", Text: "Clean up every session of one scripted run."},
	})

const removeQuery = "?remove=1"

// rmEndTimeout bounds how long rm waits for a running session to exit.
const rmEndTimeout = 15 * time.Second

func isStatus(err error, status int) bool {
	var apiErr *localclient.APIError
	return errors.As(err, &apiErr) && apiErr.Status == status
}

// endAndRemove ends a running session, waits for it to exit, and removes it.
func endAndRemove(ctx context.Context, client *localclient.Client, id string) error {
	ctx, cancel := context.WithTimeout(ctx, rmEndTimeout)
	defer cancel()
	watch, err := watchSessions(ctx, client)
	if err != nil {
		return err
	}
	defer watch.close()
	path := "/api/sessions/" + url.PathEscape(id)
	if err := client.Do(ctx, http.MethodDelete, path, nil, nil); err != nil {
		return err
	}
	if _, err := watch.waitFor(ctx, waitOptions{ids: []string{id}, states: []string{"exited"}}); err != nil {
		if errors.Is(err, errSessionRemoved) {
			return nil
		}
		return fmt.Errorf("the session did not exit: %w", err)
	}
	return client.Do(ctx, http.MethodDelete, path+removeQuery, nil, nil)
}

var renameCommand = &Command{
	Name: "rename", Group: GroupSessions, Args: "SESSION [NAME]",
	Summary:   "Name a session; an empty NAME hands the title back to the terminal or agent.",
	Arguments: []Item{sessionArgument, {Name: "NAME", Text: "At most 512 bytes, no control characters."}},
	Env:       []Item{sessionIDEnv},
	Exit:      exitsCommon(ExitItem{Code: 1, Text: "No such session or an invalid name."}),
	Examples: []Example{
		{Command: `webmux rename self "lint fixes"`, Text: "Name the current session."},
		{Command: "webmux rename abcd2345", Text: "Clear the name so the terminal title shows again."},
	},
	SeeAlso: []string{"webmux get", "webmux label"},
	Bind: func(fs *flag.FlagSet) Action {
		return func(ctx context.Context, e Env, args []string) int {
			if len(args) < 1 || len(args) > 2 {
				return e.Usagef("a session ID and at most one name are required (quote a name with spaces)")
			}
			id, code, ok := e.SessionID(args[0])
			if !ok {
				return code
			}
			name := ""
			if len(args) == 2 {
				name = args[1]
			}
			client, code := e.connect()
			if client == nil {
				return code
			}
			defer client.Close()
			if err := client.Do(ctx, http.MethodPatch, "/api/sessions/"+url.PathEscape(id), contract.RenameSessionRequest{Name: name}, nil); err != nil {
				return e.Fail(err)
			}
			return contract.ExitOK
		}
	},
}

// namedKeys are the keys --key sends.
var namedKeys = map[string]string{
	"enter": "\r", "esc": "\x1b", "tab": "\t", "ctrl-c": "\x03", "ctrl-d": "\x04",
	"up": "\x1b[A", "down": "\x1b[B", "right": "\x1b[C", "left": "\x1b[D", "backspace": "\x7f",
}

func keyNames() string {
	names := make([]string, 0, len(namedKeys))
	for name := range namedKeys {
		names = append(names, name)
	}
	slices.Sort(names)
	return strings.Join(names, ", ")
}

type sendOptions struct {
	enter   bool
	keys    listFlag
	wait    string
	timeout time.Duration
}

var sendCommand = &Command{
	Name: "send", Group: GroupSessions, Args: "SESSION [TEXT|-]",
	Summary: "Type text and keys into a terminal session, confirmed by the server.",
	Description: "TEXT goes in one acknowledged submission (the same path as the Web UI's draft send); with --enter, Enter is part of it. " +
		"The command exits 0 only once the server reports the input written to the PTY. --key sends named keys after the text. " +
		"Sending takes control of the terminal, as typing does in the browser; the control indicator names this client \"script\" " +
		"and, inside a session, that session.\n\n" +
		"--wait subscribes to the session list before sending and returns on the first change into STATE after the input, so a " +
		"stale idle state cannot end the wait early. A hybrid session on its chat side takes webmux chat send instead.",
	Arguments: []Item{sessionArgument, {Name: "TEXT", Text: "The text to type; - reads standard input."}},
	Env:       []Item{sessionIDEnv},
	Output:    "Nothing; with --wait --json, the SessionInfo that matched.",
	Exit: exitsCommon(
		ExitItem{Code: 1, Text: "No such session, the input was not (fully) written, or the session is on its chat side."},
		ExitItem{Code: 124, Text: "--wait timed out."},
	),
	Examples: []Example{
		{Command: "webmux send abcd2345 'make test' --enter --wait idle --timeout 10m", Text: "Run a command in a shell session and wait until it is idle again."},
		{Command: "webmux send abcd2345 --key esc", Text: "Interrupt an agent's turn."},
		{Command: "git diff | webmux send abcd2345 - --enter", Text: "Paste standard input into an agent and submit it."},
	},
	SeeAlso: []string{"webmux wait", "webmux screen", "webmux chat send"},
	Notes:   map[string]Note{"key": {Repeatable: true}, "timeout": {Requires: []string{"wait"}}},
	Bind: func(fs *flag.FlagSet) Action {
		var o sendOptions
		fs.BoolVar(&o.enter, "enter", false, "press Enter after the text, in the same submission")
		fs.Var(&o.keys, "key", "send the named `KEY` after the text: "+keyNames())
		fs.StringVar(&o.wait, "wait", "", "wait for the first change into `STATE` after sending (see webmux wait)")
		fs.DurationVar(&o.timeout, "timeout", 0, "give up --wait after this `DURATION` (exit 124); 0 waits forever")
		asJSON := fs.Bool("json", false, "with --wait, print the SessionInfo that matched")
		return func(ctx context.Context, e Env, args []string) int { return e.send(ctx, o, *asJSON, args) }
	},
}

func (e Env) send(ctx context.Context, o sendOptions, asJSON bool, args []string) int {
	if len(args) < 1 || len(args) > 2 {
		return e.Usagef("a session ID and at most one TEXT are required (quote text with spaces)")
	}
	for _, key := range o.keys {
		if _, ok := namedKeys[key]; !ok {
			return e.Usagef("unknown key %q (keys: %s)", key, keyNames())
		}
	}
	var states []string
	if o.wait != "" {
		var err error
		if states, err = parseStates(o.wait); err != nil {
			return e.Usagef("%v", err)
		}
	} else if o.timeout != 0 {
		return e.Usagef("--timeout needs --wait")
	}
	id, code, ok := e.SessionID(args[0])
	if !ok {
		return code
	}
	text := ""
	if len(args) == 2 {
		text = args[1]
		if text == "-" {
			var err error
			if text, err = e.readStdin(); err != nil {
				return e.Fail(err)
			}
		}
	}
	if text == "" && !o.enter && len(o.keys) == 0 {
		return e.Usagef("nothing to send: give TEXT, --enter or --key")
	}
	client, code := e.connect()
	if client == nil {
		return code
	}
	defer client.Close()
	var info contract.SessionInfo
	if err := client.Do(ctx, http.MethodGet, "/api/sessions/"+url.PathEscape(id), nil, &info); err != nil {
		return e.Fail(err)
	}
	if info.Hybrid != nil && info.Hybrid.Side != contract.HybridSideTUI {
		_, _ = fmt.Fprintf(e.Stderr, "webmux send: session %s is on its chat side; use webmux chat send\n", id)
		return contract.ExitError
	}
	waitCtx := ctx
	var watch *sessionWatch
	if o.wait != "" {
		if o.timeout > 0 {
			var cancel context.CancelFunc
			waitCtx, cancel = context.WithTimeout(ctx, o.timeout)
			defer cancel()
		}
		var err error
		// Subscribe before the input: the change it causes must be seen.
		if watch, err = watchSessions(waitCtx, client); err != nil {
			return e.Fail(err)
		}
		defer watch.close()
		if _, err := watch.next(waitCtx); err != nil {
			return e.waitFailure(err)
		}
	}
	if err := e.writeInput(ctx, client, info, text, o); err != nil {
		return e.Fail(err)
	}
	if watch == nil {
		return contract.ExitOK
	}
	matched, err := watch.waitFor(waitCtx, waitOptions{ids: []string{id}, states: states, changed: true, primed: true})
	if err != nil {
		return e.waitFailure(err)
	}
	if asJSON {
		writeJSON(e.Stdout, matched[0])
	}
	return contract.ExitOK
}

// writeInput submits text (with Enter) and waits for its acknowledgement,
// then sends the named keys.
func (e Env) writeInput(ctx context.Context, client *localclient.Client, info contract.SessionInfo, text string, o sendOptions) error {
	clientID := newRequestID()
	query := url.Values{"client": {clientID}, "label": {e.scriptLabel()}}
	conn, err := client.DialWebSocket(ctx, "/ws/s/"+url.PathEscape(info.ID)+"?"+query.Encode())
	if err != nil {
		return err
	}
	defer func() { _ = conn.CloseNow() }()
	conn.SetReadLimit(64 << 20)
	// Text, Enter and every key are acknowledged submissions, so send
	// returns only once each reached the PTY.
	submit := func(body, key string) error {
		id := newRequestID()
		header, _ := json.Marshal(contract.SubmissionHeader{ID: id, Generation: info.Generation, BodyBytes: len(body)})
		frame := contract.Frame(contract.FrameSubmit, append(append(header, '\n'), body+key...))
		if err := conn.Write(ctx, websocket.MessageBinary, frame); err != nil {
			return err
		}
		return awaitSubmission(ctx, conn, id)
	}
	if text != "" || o.enter {
		key := ""
		if o.enter {
			key = "\r"
		}
		if err := submit(text, key); err != nil {
			return err
		}
	}
	for _, key := range o.keys {
		if err := submit(namedKeys[key], ""); err != nil {
			return err
		}
	}
	// Close cleanly so queued input reaches the PTY before the server
	// tears the connection down.
	_ = conn.Close(websocket.StatusNormalClosure, "sent")
	return nil
}

// awaitSubmission reads terminal frames until submission id is written or
// fails.
func awaitSubmission(ctx context.Context, conn *websocket.Conn, id string) error {
	for {
		_, frame, err := conn.Read(ctx)
		if err != nil {
			if websocket.CloseStatus(err) == contract.CloseMoved {
				return errors.New("the session moved to its chat side; use webmux chat send")
			}
			return fmt.Errorf("connection lost before the input was confirmed: %w", err)
		}
		if len(frame) == 0 || frame[0] != contract.FrameSubmission {
			continue
		}
		var status contract.SubmissionStatus
		if json.Unmarshal(frame[1:], &status) != nil || status.ID != id {
			continue
		}
		switch status.State {
		case "written":
			return nil
		case "received":
			continue
		}
		if status.Error != "" {
			return fmt.Errorf("the input was not written: %s (%s)", status.State, status.Error)
		}
		return fmt.Errorf("the input was not written: %s", status.State)
	}
}

var waitCommand = &Command{
	Name: "wait", Group: GroupSessions, Args: "SESSION...",
	Summary: "Wait until sessions reach a state: running, waiting, idle, exited or attention.",
	Description: "Subscribes to the session list (/ws/sessions) and returns when every session (or with --any, one) is in one of the " +
		"--until states; attention means the session has a latched attention signal. Without --changed the current state counts; " +
		"with --changed only a change after subscribing does. --stable requires the condition to hold that long, which filters an " +
		"agent's brief idle moments between tool calls.",
	Arguments: []Item{sessionArgument},
	Env:       []Item{sessionIDEnv},
	Output:    "Nothing; with --json the matching SessionInfo, or an array for several sessions.",
	Exit: exitsCommon(
		ExitItem{Code: 1, Text: "A session was removed, or exited when exited was not awaited."},
		ExitItem{Code: 124, Text: "Timed out."},
	),
	Examples: []Example{
		{Command: "webmux wait abcd2345 --until attention,idle --changed --timeout 30m", Text: "Wait until an agent needs you or finishes its turn."},
		{Command: "webmux wait abcd2345 --until exited --json | jq .exitCode", Text: "Wait for a command and read its exit code."},
		{Command: "webmux wait a1 b2 c3 --until idle --stable 5s --any", Text: "Return when the first of three agents has been idle for five seconds."},
	},
	SeeAlso: []string{"webmux send --wait", "webmux events"},
	Bind: func(fs *flag.FlagSet) Action {
		until := fs.String("until", "", "comma-separated `STATES` to wait for (required)")
		changed := fs.Bool("changed", false, "ignore the state at subscription; wait for a change into the states")
		stable := fs.Duration("stable", 0, "require the condition to hold for this `DURATION`")
		anyOne := fs.Bool("any", false, "return when one session matches instead of all")
		timeout := fs.Duration("timeout", 0, "give up after this `DURATION` (exit 124); 0 waits forever")
		asJSON := fs.Bool("json", false, "print the matching SessionInfo")
		return func(ctx context.Context, e Env, args []string) int {
			if len(args) == 0 {
				return e.Usagef("at least one session ID is required")
			}
			if *until == "" {
				return e.Usagef("--until is required")
			}
			states, err := parseStates(*until)
			if err != nil {
				return e.Usagef("%v", err)
			}
			ids := make([]string, 0, len(args))
			for _, arg := range args {
				id, code, ok := e.SessionID(arg)
				if !ok {
					return code
				}
				ids = append(ids, id)
			}
			client, code := e.connect()
			if client == nil {
				return code
			}
			defer client.Close()
			if *timeout > 0 {
				var cancel context.CancelFunc
				ctx, cancel = context.WithTimeout(ctx, *timeout)
				defer cancel()
			}
			watch, err := watchSessions(ctx, client)
			if err != nil {
				return e.Fail(err)
			}
			defer watch.close()
			matched, err := watch.waitFor(ctx, waitOptions{ids: ids, states: states, changed: *changed, stable: *stable, any: *anyOne})
			if err != nil {
				return e.waitFailure(err)
			}
			if !slices.Contains(states, "exited") {
				for _, s := range matched {
					if s.Status == contract.SessionStatusExited {
						_, _ = fmt.Fprintf(e.Stderr, "webmux wait: session %s exited\n", s.ID)
						return contract.ExitError
					}
				}
			}
			if *asJSON {
				if len(matched) == 1 && len(ids) == 1 {
					writeJSON(e.Stdout, matched[0])
				} else {
					writeJSON(e.Stdout, matched)
				}
			}
			return contract.ExitOK
		}
	},
}

var logsCommand = &Command{
	Name: "logs", Group: GroupSessions, Args: "SESSION",
	Summary: "Print a session's terminal output; --follow keeps streaming as an observer.",
	Description: "Prints the session's retained output (GET /api/sessions/{id}/buffer) as raw bytes. --strip removes terminal control " +
		"sequences and keeps line text, which suits line-oriented programs but not full-screen TUIs; use webmux screen for those. " +
		"--follow connects as an observer, prints the replay and then new output, never taking control or resizing, until the session exits.",
	Arguments: []Item{sessionArgument},
	Env:       []Item{sessionIDEnv},
	Output:    "The output bytes, or plain text with --strip. With --follow, the exit code of the session goes to standard error.",
	Exit:      exitsCommon(ExitItem{Code: 1, Text: "No such session, or following was interrupted."}),
	Examples: []Example{
		{Command: "webmux logs abcd2345 --strip --tail 50", Text: "Show the last 50 lines as plain text."},
		{Command: "webmux logs self --follow --strip | grep --line-buffered ERROR", Text: "Watch a session's output for errors."},
	},
	SeeAlso: []string{"webmux screen", "webmux attach --observe"},
	Notes:   map[string]Note{"tail": {Conflicts: []string{"follow"}}, "follow": {Conflicts: []string{"tail"}}},
	Bind: func(fs *flag.FlagSet) Action {
		follow := fs.Bool("follow", false, "keep printing new output until the session exits")
		strip := fs.Bool("strip", false, "remove terminal control sequences")
		tail := fs.Int("tail", 0, "print only the last `N` lines")
		return func(ctx context.Context, e Env, args []string) int {
			if len(args) != 1 {
				return e.Usagef("exactly one session ID is required")
			}
			if *follow && *tail > 0 {
				return e.Usagef("--tail cannot be combined with --follow")
			}
			if *tail < 0 {
				return e.Usagef("--tail must not be negative")
			}
			id, code, ok := e.SessionID(args[0])
			if !ok {
				return code
			}
			client, code := e.connect()
			if client == nil {
				return code
			}
			defer client.Close()
			out := e.Stdout
			var s *stripper
			if *strip {
				s = newStripper(out)
				out = s
			}
			if *follow {
				result, err := followTerminal(ctx, client, id, out, e.scriptLabel())
				if s != nil {
					_ = s.Flush()
				}
				if err != nil {
					return e.Fail(err)
				}
				if result.Exited {
					_, _ = fmt.Fprintf(e.Stderr, "webmux logs: session %s exited with %d\n", id, result.ExitCode)
				}
				return contract.ExitOK
			}
			data, err := client.Get(ctx, "/api/sessions/"+url.PathEscape(id)+"/buffer")
			if err != nil {
				return e.Fail(err)
			}
			if *tail == 0 {
				_, _ = out.Write(data)
				if s != nil {
					_ = s.Flush()
				}
				return contract.ExitOK
			}
			var text bytes.Buffer
			if s != nil {
				t := newStripper(&text)
				_, _ = t.Write(data)
				_ = t.Flush()
			} else {
				text.Write(data)
			}
			_, _ = e.Stdout.Write(lastLines(text.Bytes(), *tail))
			return contract.ExitOK
		}
	},
}

// lastLines returns the last n lines of data.
func lastLines(data []byte, n int) []byte {
	trimmed := bytes.TrimRight(data, "\n")
	cut := len(trimmed)
	for i := 0; i < n; i++ {
		at := bytes.LastIndexByte(trimmed[:cut], '\n')
		if at < 0 {
			return data
		}
		cut = at
	}
	return data[cut+1:]
}

// screenView is the --json form of webmux screen.
type screenView struct {
	Cols      int      `json:"cols"`
	Rows      int      `json:"rows"`
	Cursor    position `json:"cursor"`
	AltScreen bool     `json:"altScreen"`
	Lines     []string `json:"lines"`
}

type position struct {
	Row int `json:"row"`
	Col int `json:"col"`
}

var screenCommand = &Command{
	Name: "screen", Group: GroupSessions, Args: "SESSION",
	Summary: "Print the text a terminal session shows right now, rendered on the client.",
	Description: "Connects as an observer, replays the session into a terminal emulator inside this command at the controller's size, " +
		"and prints the visible screen; the server does no terminal emulation. Use it to see a full-screen TUI's state, such as an " +
		"agent waiting on a permission prompt. A truncated replay may leave the screen incomplete; full-screen programs usually redraw.",
	Arguments: []Item{sessionArgument},
	Env:       []Item{sessionIDEnv},
	Output:    "The visible lines without trailing blanks. With --json, {cols, rows, cursor: {row, col}, altScreen, lines}. --cursor writes the 1-based cursor position to standard error.",
	Exit:      exitsCommon(ExitItem{Code: 1, Text: "No such session, or it is on its chat side."}),
	Examples: []Example{
		{Command: "webmux screen abcd2345", Text: "See what an agent's terminal shows."},
		{Command: "webmux screen abcd2345 --json | jq -r '.lines[-1]'", Text: "Read the last line of the screen."},
	},
	SeeAlso: []string{"webmux logs", "webmux send", "webmux chat status"},
	Bind: func(fs *flag.FlagSet) Action {
		asJSON := fs.Bool("json", false, "print size, cursor, alternate-screen state and lines as JSON")
		cursor := fs.Bool("cursor", false, "write the cursor position to standard error")
		return func(ctx context.Context, e Env, args []string) int {
			if len(args) != 1 {
				return e.Usagef("exactly one session ID is required")
			}
			id, code, ok := e.SessionID(args[0])
			if !ok {
				return code
			}
			client, code := e.connect()
			if client == nil {
				return code
			}
			defer client.Close()
			view, err := e.renderScreen(ctx, client, id)
			if err != nil {
				return e.Fail(err)
			}
			if *cursor {
				_, _ = fmt.Fprintf(e.Stderr, "cursor %d,%d\n", view.Cursor.Row, view.Cursor.Col)
			}
			if *asJSON {
				writeJSON(e.Stdout, view)
				return contract.ExitOK
			}
			w := bufio.NewWriter(e.Stdout)
			for _, line := range view.Lines {
				_, _ = w.WriteString(line + "\n")
			}
			_ = w.Flush()
			return contract.ExitOK
		}
	},
}

// screenQuiet is how long screen waits for output after the replay.
const screenQuiet = 150 * time.Millisecond

func (e Env) renderScreen(ctx context.Context, client *localclient.Client, id string) (screenView, error) {
	query := url.Values{"label": {e.scriptLabel()}}
	conn, err := client.DialWebSocket(ctx, "/ws/s/"+url.PathEscape(id)+"?"+query.Encode())
	if err != nil {
		return screenView{}, err
	}
	defer func() { _ = conn.CloseNow() }()
	conn.SetReadLimit(64 << 20)
	var pending [][]byte
	cols, rows, truncated := 0, 0, false
	gotSnapshot := false
	for {
		readCtx := ctx
		var cancel context.CancelFunc
		if gotSnapshot && cols > 0 {
			readCtx, cancel = context.WithTimeout(ctx, screenQuiet)
		}
		_, frame, err := conn.Read(readCtx)
		if cancel != nil {
			cancel()
		}
		if err != nil {
			if gotSnapshot && ctx.Err() == nil {
				break
			}
			switch websocket.CloseStatus(err) {
			case contract.CloseMoved:
				return screenView{}, errors.New("the session is on its chat side; use webmux chat status")
			case 4404:
				return screenView{}, errors.New("session not found")
			}
			return screenView{}, err
		}
		if len(frame) == 0 {
			continue
		}
		switch frame[0] {
		case contract.FrameSnapshot:
			pending = [][]byte{frame[1:]}
			gotSnapshot = true
		case contract.FrameOutput:
			pending = append(pending, frame[1:])
		case contract.FramePosition:
			var p contract.StreamPosition
			if json.Unmarshal(frame[1:], &p) == nil && p.Truncated {
				truncated = true
			}
		case contract.FrameState:
			var info contract.SessionInfo
			if json.Unmarshal(frame[1:], &info) == nil && info.Cols > 0 {
				cols, rows = int(info.Cols), int(info.Rows)
			}
		case contract.FrameExit:
			gotSnapshot = true
		}
	}
	if truncated {
		_, _ = fmt.Fprintln(e.Stderr, "webmux screen: the replay was truncated; the screen may be incomplete")
	}
	if cols == 0 {
		cols, rows = 80, 24
	}
	options := vt.NewOptions()
	options.Cols, options.Rows, options.Scrollback = cols, rows, 0
	term := vt.NewTerminal(options)
	for _, chunk := range pending {
		term.Write(chunk)
	}
	buffer := term.Buffer()
	view := screenView{
		Cols: cols, Rows: rows, AltScreen: buffer == term.Buffers().Alt(),
		Cursor: position{Row: buffer.Y + 1, Col: buffer.X + 1},
	}
	for y := 0; y < rows; y++ {
		view.Lines = append(view.Lines, strings.TrimRight(buffer.TranslateBufferLineToString(buffer.YBase+y, true, 0, cols), " "))
	}
	for len(view.Lines) > 0 && view.Lines[len(view.Lines)-1] == "" {
		view.Lines = view.Lines[:len(view.Lines)-1]
	}
	if view.Lines == nil {
		view.Lines = []string{}
	}
	return view, nil
}

var eventsCommand = &Command{
	Name: "events", Group: GroupSessions,
	Summary: "Print session list events as NDJSON until interrupted.",
	Description: "Each /ws/sessions event (snapshot, created, updated, removed, forwards) is written as one JSON line, unchanged. " +
		"With --session, only events about those sessions are written and the snapshot keeps only them. After a lost connection " +
		"the command reconnects and writes a new snapshot.",
	Output: "One JSON object per line.",
	Exit:   exitsCommon(ExitItem{Code: 1, Text: "The connection failed."}),
	Examples: []Example{
		{Command: `webmux events | jq -c 'select(.type=="updated") | {id: .session.id, status: .session.status}'`, Text: "Print every status change."},
		{Command: "webmux events --session abcd2345", Text: "Follow one session's events."},
	},
	SeeAlso: []string{"webmux wait"},
	Notes:   map[string]Note{"session": {Repeatable: true}},
	Bind: func(fs *flag.FlagSet) Action {
		var only listFlag
		fs.Var(&only, "session", "only events about `SESSION` (or self)")
		return func(ctx context.Context, e Env, args []string) int {
			if len(args) > 0 {
				return e.Usagef("unexpected argument %q", args[0])
			}
			ids := make([]string, 0, len(only))
			for _, arg := range only {
				id, code, ok := e.SessionID(arg)
				if !ok {
					return code
				}
				ids = append(ids, id)
			}
			client, code := e.connect()
			if client == nil {
				return code
			}
			defer client.Close()
			for attempt := 0; ; attempt++ {
				err := e.streamEvents(ctx, client, ids)
				if ctx.Err() != nil {
					return contract.ExitOK
				}
				if attempt >= 5 {
					return e.Fail(err)
				}
				select {
				case <-time.After(time.Second):
				case <-ctx.Done():
					return contract.ExitOK
				}
			}
		}
	},
}

func (e Env) streamEvents(ctx context.Context, client *localclient.Client, ids []string) error {
	conn, err := client.DialWebSocket(ctx, "/ws/sessions")
	if err != nil {
		return err
	}
	defer func() { _ = conn.CloseNow() }()
	conn.SetReadLimit(64 << 20)
	for {
		_, data, err := conn.Read(ctx)
		if err != nil {
			return err
		}
		if len(ids) > 0 {
			var event contract.SessionEvent
			if json.Unmarshal(data, &event) != nil {
				continue
			}
			switch event.Type {
			case "snapshot":
				event.Sessions = slices.DeleteFunc(event.Sessions, func(s contract.SessionInfo) bool { return !slices.Contains(ids, s.ID) })
				data, _ = json.Marshal(event)
			case "created", "updated":
				if event.Session == nil || !slices.Contains(ids, event.Session.ID) {
					continue
				}
			case "removed":
				if !slices.Contains(ids, event.ID) {
					continue
				}
			default:
				continue
			}
		}
		_, _ = e.Stdout.Write(append(bytes.TrimSpace(data), '\n'))
	}
}
