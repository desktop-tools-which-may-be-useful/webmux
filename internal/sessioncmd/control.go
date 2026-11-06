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
	"maps"
	"net/http"
	"net/url"
	"os"
	"slices"
	"strconv"
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
		Exit:     exitsCommon(ExitItem{Code: 1, Text: "No such session, or the server refused (for example while it changes views)."}),
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
				if err := client.Do(ctx, method, "/api/sessions/"+url.PathEscape(id)+query, nil, nil); err != nil {
					return e.sessionFail(err)
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

const (
	removeQuery = "?remove=1"
	forgetQuery = "?forget=1"
)

var rmCommand = &Command{
	Name: "rm", Group: GroupSessions, Args: "SESSION",
	Summary: "Remove a session; a running one is ended first. --forget also deletes its archive.",
	Description: "The session disappears from every client. A session with a chat view keeps its archive, so it can be " +
		"resumed (webmux resume) and is listed by webmux ls --archived; its artifacts folder stays with it. --forget deletes " +
		"the archive and the artifacts folder too; the agent keeps its own session history. An archived session can only " +
		"be forgotten. On a terminal --forget asks first; without one it needs --yes.",
	Arguments: []Item{sessionArgument},
	Env:       []Item{sessionIDEnv},
	Exit: exitsCommon(
		ExitItem{Code: 1, Text: "No such session, it is archived and --forget was not given, or the server refused (for example while it changes views)."},
		ExitItem{Code: 2, Text: "Also: --forget with no terminal to ask and no --yes."},
	),
	Examples: []Example{
		{Command: "webmux rm abcd2345", Text: "Remove a session."},
		{Command: "webmux ls --label run=42 --json | jq -r '.[].id' | xargs -n1 webmux rm", Text: "Clean up every session of one scripted run."},
		{Command: "webmux ls --archived --json | jq -r '.[].id' | xargs -n1 webmux rm --forget --yes", Text: "Delete every archived session."},
	},
	SeeAlso: []string{"webmux kill", "webmux resume", "webmux ls"},
	Notes:   map[string]Note{"yes": {Requires: []string{"forget"}}},
	Bind: func(fs *flag.FlagSet) Action {
		forget := fs.Bool("forget", false, "also delete the archive and the artifacts folder")
		yes := fs.Bool("yes", false, "with --forget, do not ask for confirmation")
		return func(ctx context.Context, e Env, args []string) int {
			if len(args) != 1 {
				return e.Usagef("exactly one session ID is required")
			}
			if *yes && !*forget {
				return e.Usagef("--yes needs --forget")
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
			var info contract.SessionInfo
			if err := client.Do(ctx, http.MethodGet, "/api/sessions/"+url.PathEscape(id), nil, &info); err != nil {
				return e.sessionFail(err)
			}
			archived := info.State != "" && info.State != contract.SessionStateActive
			if !*forget {
				if archived {
					_, _ = fmt.Fprintf(e.Stderr, "webmux: session %s is %s; delete its archive with webmux rm %s --forget\n", id, info.State, id)
					return contract.ExitError
				}
				return e.removeSession(ctx, client, id, removeQuery)
			}
			if !*yes {
				question := fmt.Sprintf("Forget %s session %s (%s)? Its archive and artifacts folder are deleted.", sessionState(info), id, oneLine(sessionName(info), 40))
				if code, ok := e.confirm(question); !ok {
					return code
				}
			}
			return e.removeSession(ctx, client, id, forgetQuery)
		}
	},
}

// sessionState is the lifecycle state of s, or its status while active.
func sessionState(s contract.SessionInfo) string {
	if s.State != "" && s.State != contract.SessionStateActive {
		return string(s.State)
	}
	return string(s.Status)
}

// sessionName is what identifies s to a person: its title, else its agent
// or command.
func sessionName(s contract.SessionInfo) string {
	switch {
	case s.Title != "":
		return s.Title
	case s.Agent != nil && s.Agent.Title != "":
		return s.Agent.Title
	case s.Agent != nil:
		return s.Agent.Name
	}
	return strings.Join(s.Cmd, " ")
}

// removeSession removes or forgets session id with query; a session whose
// process still runs is ended first.
func (e Env) removeSession(ctx context.Context, client *localclient.Client, id, query string) int {
	err := client.Do(ctx, http.MethodDelete, "/api/sessions/"+url.PathEscape(id)+query, nil, nil)
	if isStatus(err, http.StatusConflict) {
		if _, isConflict := sessionConflict(err); !isConflict {
			err = endAndRemove(ctx, client, id, query)
		}
	}
	if err != nil {
		return e.sessionFail(err)
	}
	return contract.ExitOK
}

// rmEndTimeout bounds how long rm waits for a running session to exit.
const rmEndTimeout = 15 * time.Second

func isStatus(err error, status int) bool {
	var apiErr *localclient.APIError
	return errors.As(err, &apiErr) && apiErr.Status == status
}

// endAndRemove ends a running session, waits for it to exit, and removes
// (or forgets) it with query.
func endAndRemove(ctx context.Context, client *localclient.Client, id, query string) error {
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
		if !errors.Is(err, errSessionRemoved) {
			return fmt.Errorf("the session did not exit: %w", err)
		}
		if query == removeQuery {
			return nil
		}
	}
	return client.Do(ctx, http.MethodDelete, path+query, nil, nil)
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

var resizeCommand = &Command{
	Name: "resize", Group: GroupSessions, Args: "SESSION COLSxROWS",
	Summary: "Set the size of a session's terminal, in columns and rows.",
	Description: "Resizes the PTY, so the program redraws for the new size. It works while no browser or attached terminal controls the " +
		"session, or while a script does (webmux send makes this client the controller as \"script\"). A session that a browser or " +
		"attached terminal controls refuses with exit 1, because that client sends its own window size back and the two would fight; " +
		"--take takes control for the resize, then releases it. The first browser or attached terminal to take control afterwards " +
		"resizes the session to its own window. The terminal's replay and webmux screen use the controller's size.",
	Arguments: []Item{sessionArgument, {Name: "COLSxROWS", Text: "Columns and rows, each 1-1000, such as 120x40."}},
	Env:       []Item{sessionIDEnv},
	Exit:      exitsCommon(ExitItem{Code: 1, Text: "No such session, it has exited or is on its chat view (no terminal), or a browser or attached terminal controls it and --take was not given."}),
	Examples: []Example{
		{Command: "webmux resize abcd2345 120x40", Text: "Make a script-driven session's terminal 120 columns by 40 rows."},
		{Command: "webmux resize abcd2345 200x50 --take", Text: "Resize a session even though a browser controls it."},
	},
	SeeAlso: []string{"webmux run", "webmux screen", "webmux send"},
	Bind: func(fs *flag.FlagSet) Action {
		take := fs.Bool("take", false, "take control from a browser or attached terminal for the resize, then release it")
		return func(ctx context.Context, e Env, args []string) int {
			if len(args) != 2 {
				return e.Usagef("a session ID and COLSxROWS are required")
			}
			cols, rows, ok := parseGrid(args[1])
			if !ok {
				return e.Usagef("%q is not COLSxROWS with each 1-%d, such as 120x40", args[1], contract.MaxTerminalDimension)
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
			request := contract.SessionResizeRequest{Cols: cols, Rows: rows, Take: *take}
			if err := client.Do(ctx, http.MethodPost, "/api/sessions/"+url.PathEscape(id)+"/resize", request, nil); err != nil {
				return e.sessionFail(err)
			}
			return contract.ExitOK
		}
	},
}

// parseGrid reads COLSxROWS, each 1-contract.MaxTerminalDimension.
func parseGrid(s string) (cols, rows int, ok bool) {
	c, r, found := strings.Cut(strings.ToLower(s), "x")
	if !found {
		return 0, 0, false
	}
	cols, errCols := strconv.Atoi(c)
	rows, errRows := strconv.Atoi(r)
	if errCols != nil || errRows != nil || cols < 1 || rows < 1 || cols > contract.MaxTerminalDimension || rows > contract.MaxTerminalDimension {
		return 0, 0, false
	}
	return cols, rows, true
}

func keyNames() string {
	return strings.Join(slices.Sorted(maps.Keys(contract.InputKeys)), ", ")
}

type sendOptions struct {
	enter            bool
	keys             listFlag
	attachments      listFlag
	mode             string
	wait             string
	reply            bool
	autoWaitRequests bool
	timeout          time.Duration
	asJSON           bool
}

var sendCommand = &Command{
	Name: "send", Group: GroupSessions, Args: "SESSION [TEXT|-]",
	Summary: "Send input to a session: typed into its terminal, or a prompt on its chat view.",
	Description: "The server sends the input to the session's current view. On the terminal view TEXT is one acknowledged " +
		"submission (with Enter when --enter is given) and each --key follows; the command exits 0 once the server reports " +
		"everything written. Typing takes control of the terminal; the control indicator names this client \"script\" and, " +
		"inside a session, that session. On the chat view TEXT is a prompt, sent at once or queued while a turn runs; " +
		"--attach-file uploads files into the session and sends them with it, and --mode steer injects it into the running " +
		"turn. Options of the other view are refused (wrong_view) and nothing is sent.\n\n" +
		"--wait STATE returns on the first change into STATE after the input (see webmux wait). --reply, for the chat view " +
		"only, follows the turn this input starts and prints the agent's final reply; a stop for a permission request or " +
		"question exits 1 unless --auto-wait-requests.",
	Arguments: []Item{sessionArgument, {Name: "TEXT", Text: "The text to send; - reads standard input."}},
	Env:       []Item{sessionIDEnv},
	Output: "Nothing; with --json, the SessionInputResult (view, queued, itemId, turnId, promptId). With --wait --json, the " +
		"SessionInfo that matched. With --reply, the reply text, or with --json {stopReason, error, reply, events, request}.",
	Exit: exitsCommon(
		ExitItem{Code: 1, Text: "No such session, it is not active or is changing views, an option belongs to the other view, " +
			"the input was not (fully) written, an upload failed, or with --reply the turn did not end with end_turn or waits for an answer."},
		ExitItem{Code: 124, Text: "--wait or --reply timed out; a turn goes on."},
	),
	Examples: []Example{
		{Command: "webmux send abcd2345 'make test' --enter --wait waiting,idle --timeout 10m", Text: "Run a command in a shell session and wait until its prompt is back (an idle shell prompt reads as waiting)."},
		{Command: `webmux send abcd2345 "fix the lint errors and summarize the change" --reply --timeout 20m`, Text: "Hand a task to an agent on its chat view and print its answer."},
		{Command: "git diff | webmux send abcd2345 - --enter", Text: "Paste standard input into a terminal agent and submit it."},
		{Command: "webmux send abcd2345 'what does this show?' --attach-file screenshot.png", Text: "Send a prompt with an image."},
	},
	SeeAlso: []string{"webmux interrupt", "webmux wait", "webmux screen", "webmux view", "webmux guide sessions"},
	Notes: map[string]Note{
		"key": {Repeatable: true}, "attach-file": {Repeatable: true},
		"wait":               {Conflicts: []string{"reply"}},
		"reply":              {Conflicts: []string{"wait"}},
		"auto-wait-requests": {Requires: []string{"reply"}},
	},
	Bind: func(fs *flag.FlagSet) Action {
		var o sendOptions
		fs.BoolVar(&o.enter, "enter", false, "terminal view: press Enter after the text, in the same submission")
		fs.Var(&o.keys, "key", "terminal view: send the named `KEY` after the text: "+keyNames())
		fs.Var(&o.attachments, "attach-file", "chat view: upload the file at `PATH` and send it with the prompt")
		fs.StringVar(&o.mode, "mode", "", "chat view: `MODE` auto (send now or queue, the default) or steer (inject into the running turn)")
		fs.StringVar(&o.wait, "wait", "", "wait for the first change into `STATE` after sending (see webmux wait)")
		fs.BoolVar(&o.reply, "reply", false, "chat view: follow the turn this input starts and print the agent's final reply")
		fs.BoolVar(&o.autoWaitRequests, "auto-wait-requests", false, "with --reply, keep waiting while a permission request or question is open")
		fs.DurationVar(&o.timeout, "timeout", 0, "give up --wait or --reply after this `DURATION` (exit 124); 0 waits forever")
		fs.BoolVar(&o.asJSON, "json", false, "print the result as JSON")
		return func(ctx context.Context, e Env, args []string) int { return e.send(ctx, o, args) }
	},
}

func (e Env) send(ctx context.Context, o sendOptions, args []string) int {
	if len(args) < 1 || len(args) > 2 {
		return e.Usagef("a session ID and at most one TEXT are required (quote text with spaces)")
	}
	for _, key := range o.keys {
		if _, ok := contract.InputKeys[key]; !ok {
			return e.Usagef("unknown key %q (keys: %s)", key, keyNames())
		}
	}
	var states []string
	switch {
	case o.wait != "" && o.reply:
		return e.Usagef("--wait and --reply cannot be combined")
	case o.mode != "" && o.mode != "auto" && o.mode != "steer":
		return e.Usagef("--mode is auto or steer")
	case o.autoWaitRequests && !o.reply:
		return e.Usagef("--auto-wait-requests needs --reply")
	case o.timeout != 0 && o.wait == "" && !o.reply:
		return e.Usagef("--timeout needs --wait or --reply")
	case o.wait != "":
		var err error
		if states, err = parseStates(o.wait); err != nil {
			return e.Usagef("%v", err)
		}
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
	if text == "" && !o.enter && len(o.keys) == 0 && len(o.attachments) == 0 {
		return e.Usagef("nothing to send: give TEXT, --enter, --key or --attach-file")
	}
	client, code := e.connect()
	if client == nil {
		return code
	}
	defer client.Close()
	base := "/api/sessions/" + url.PathEscape(id)
	request := contract.SessionInputRequest{ClientRequestID: newRequestID(), Text: text, Submit: o.enter, Keys: o.keys, Mode: o.mode}
	// Uploads and --reply need the chat view: check it before anything is
	// uploaded or sent.
	if o.reply || len(o.attachments) > 0 {
		var info contract.SessionInfo
		if err := client.Do(ctx, http.MethodGet, base, nil, &info); err != nil {
			return e.sessionFail(err)
		}
		if !onChat(info) {
			switch {
			case o.reply && hasChatView(info):
				_, _ = fmt.Fprintf(e.Stderr, "webmux send: session %s is on its terminal view, which has no reply to print; use --wait STATE, or webmux view %s chat first\n", id, id)
			case o.reply:
				_, _ = fmt.Fprintf(e.Stderr, "webmux send: session %s has no chat view, so there is no reply to print; use --wait STATE\n", id)
			default:
				_, _ = fmt.Fprintf(e.Stderr, "webmux send: session %s is not on its chat view; --attach-file is for the chat view (wrong_view)\n", id)
			}
			return contract.ExitError
		}
		for _, file := range o.attachments {
			path, err := e.uploadFile(ctx, client, base+"/upload", file)
			if err != nil {
				return e.Fail(fmt.Errorf("upload %s: %w; nothing was sent", file, err))
			}
			request.Attachments = append(request.Attachments, contract.ChatAttachment{Path: path})
		}
	}
	waitCtx := ctx
	if o.timeout > 0 {
		var cancel context.CancelFunc
		waitCtx, cancel = context.WithTimeout(ctx, o.timeout)
		defer cancel()
	}
	if o.reply {
		return e.sendReply(ctx, waitCtx, client, id, request, o)
	}
	var watch *sessionWatch
	if o.wait != "" {
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
	var result contract.SessionInputResult
	if err := client.Do(ctx, http.MethodPost, base+"/input", request, &result); err != nil {
		return e.sessionFail(err)
	}
	if watch == nil {
		if o.asJSON {
			writeJSON(e.Stdout, result)
		}
		return contract.ExitOK
	}
	matched, err := watch.waitFor(waitCtx, waitOptions{ids: []string{id}, states: states, changed: true, primed: true})
	if err != nil {
		return e.waitFailure(err)
	}
	if o.asJSON {
		writeJSON(e.Stdout, matched[0])
	}
	return contract.ExitOK
}

// sendReply sends a prompt and follows the turn it starts: for a queued
// prompt the turn that later sends it, with steer the running turn.
func (e Env) sendReply(ctx, waitCtx context.Context, client *localclient.Client, id string, request contract.SessionInputRequest, o sendOptions) int {
	// Subscribe first and read the snapshot, so the turn cannot start (or
	// a steered turn end) unseen.
	stream, err := openChat(waitCtx, client, id)
	if err != nil {
		return e.Fail(err)
	}
	defer stream.close()
	if _, err := stream.read(waitCtx); err != nil {
		return e.waitFailure(err)
	}
	var result contract.SessionInputResult
	if err := client.Do(ctx, http.MethodPost, "/api/sessions/"+url.PathEscape(id)+"/input", request, &result); err != nil {
		return e.sessionFail(err)
	}
	if result.View != contract.ViewChat {
		// The session changed views between the check and the input.
		_, _ = fmt.Fprintf(e.Stderr, "webmux send: the input went to the %s view, which has no reply to print\n", result.View)
		return contract.ExitError
	}
	w := turnWatch{stopAtRequest: !o.autoWaitRequests}
	switch {
	case result.Queued != nil && *result.Queued:
		_, _ = fmt.Fprintf(e.Stderr, "webmux: the prompt is queued as %s; waiting for its turn\n", result.ItemID)
		w.accept = func(t contract.ChatTurnStart) bool { return t.QueueItemID == result.ItemID }
		w.gone = queuedGone(result.ItemID)
	case result.PromptID != "":
		w.accept = func(t contract.ChatTurnStart) bool { return t.PromptID == result.PromptID }
	default:
		// Steering joins the running turn.
		if stream.state == nil || !stream.state.Turn.Running {
			return e.Fail(errors.New("no turn is running to follow"))
		}
		w.current = stream.state.Turn.TurnID
	}
	turn, err := stream.turnWait(waitCtx, w)
	if err != nil {
		return e.waitFailure(err)
	}
	return e.reportTurn(turn, o.asJSON)
}

var waitCommand = &Command{
	Name: "wait", Group: GroupSessions, Args: "SESSION...",
	Summary: "Wait until sessions reach a state: running, waiting, idle, exited or attention.",
	Description: "Subscribes to the session list (/ws/sessions) and returns when every session (or with --any, one) is in one of the " +
		"--until states: a status (running, waiting, idle, exited) or attention, a latched signal that the session needs you. " +
		"waiting means the session needs you, so a shell at its prompt is waiting; idle is long silence, not completion " +
		"(webmux guide sessions explains each). Without --changed the current state counts; with --changed only a change " +
		"after subscribing does. --stable requires the condition to hold that long, which filters short-lived transitions.",
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
		{Command: "webmux wait a1 b2 c3 --until attention --changed --any", Text: "Return when the first of three agents needs you or finishes its turn."},
	},
	SeeAlso: []string{"webmux send --wait", "webmux events", "webmux guide sessions"},
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
	Stable    bool     `json:"stable"`
	Lines     []string `json:"lines"`
}

type position struct {
	Row int `json:"row"`
	Col int `json:"col"`
}

// maxScreenScrollback bounds --scrollback: the emulator allocates its
// history up front.
const maxScreenScrollback = 100000

var screenCommand = &Command{
	Name: "screen", Group: GroupSessions, Args: "SESSION",
	Summary: "Print the text a terminal session shows right now, rendered on the client.",
	Description: "Connects as an observer, replays the session into a terminal emulator inside this command at the controller's size, " +
		"and prints the visible screen; the server does no terminal emulation. Use it to see a full-screen TUI's state, such as an " +
		"agent waiting on a permission prompt. A truncated replay may leave the screen incomplete; full-screen programs usually redraw.\n\n" +
		"The command does not wait forever for a quiet screen: it returns once the session has had no output for 150 ms, or after one second at the latest. " +
		"A program that keeps redrawing (an animated spinner) yields a snapshot flagged unstable; use logs or the chat view for those.\n\n" +
		"--scrollback N also prints up to N lines of scroll history above the screen. The history comes from the session's 2 MiB " +
		"output buffer, replayed at the controller's current width, so it holds only what that buffer still has and is cut off the same way " +
		"as a truncated replay. A program on the alternate screen (vim, less, most TUIs) has no scroll history: only the visible screen is printed.",
	Arguments: []Item{sessionArgument},
	Env:       []Item{sessionIDEnv},
	Output: "The visible lines without trailing blanks, preceded by the scroll history with --scrollback. With --json, " +
		"{cols, rows, cursor: {row, col}, altScreen, stable, lines}, where lines includes the history and stable tells whether the screen had settled. " +
		"--cursor writes the 1-based cursor position to standard error. If output is still arriving one second after the replay, the snapshot is printed anyway, " +
		"stable is false and a note goes to standard error; the exit status stays 0.",
	Exit: exitsCommon(ExitItem{Code: 1, Text: "No such session, or it is on its chat view."}),
	Examples: []Example{
		{Command: "webmux screen abcd2345", Text: "See what an agent's terminal shows."},
		{Command: "webmux screen abcd2345 --json | jq -r '.lines[-1]'", Text: "Read the last line of the screen."},
		{Command: "webmux screen abcd2345 --scrollback 200", Text: "See the last 200 lines that scrolled off the screen, then the screen."},
	},
	SeeAlso: []string{"webmux logs", "webmux send", "webmux chat status"},
	Bind: func(fs *flag.FlagSet) Action {
		asJSON := fs.Bool("json", false, "print size, cursor, alternate-screen state and lines as JSON")
		cursor := fs.Bool("cursor", false, "write the cursor position to standard error")
		scrollback := fs.Int("scrollback", 0, "also print up to `N` lines of scroll history above the screen (0-100000; none on the alternate screen)")
		return func(ctx context.Context, e Env, args []string) int {
			if len(args) != 1 {
				return e.Usagef("exactly one session ID is required")
			}
			id, code, ok := e.SessionID(args[0])
			if !ok {
				return code
			}
			if *scrollback < 0 || *scrollback > maxScreenScrollback {
				return e.Usagef("--scrollback must be 0-%d", maxScreenScrollback)
			}
			client, code := e.connect()
			if client == nil {
				return code
			}
			defer client.Close()
			view, err := e.renderScreen(ctx, client, id, *scrollback)
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

// screenQuiet is how long without output counts as a settled screen.
const screenQuiet = 150 * time.Millisecond

// screenSettleMax bounds how long screen keeps reading after the replay: a
// TUI that animates never goes quiet, so the screen is printed as received.
const screenSettleMax = time.Second

func (e Env) renderScreen(ctx context.Context, client *localclient.Client, id string, scrollback int) (screenView, error) {
	query := url.Values{"label": {e.scriptLabel()}}
	conn, err := client.DialWebSocket(ctx, "/ws/s/"+url.PathEscape(id)+"?"+query.Encode())
	if err != nil {
		return screenView{}, err
	}
	defer func() { _ = conn.CloseNow() }()
	conn.SetReadLimit(64 << 20)
	var pending [][]byte
	cols, rows, truncated := 0, 0, false
	gotSnapshot, stable := false, true
	var settleBy time.Time
	for {
		readCtx := ctx
		var cancel context.CancelFunc
		capped := false
		if gotSnapshot && cols > 0 {
			wait := screenQuiet
			if left := time.Until(settleBy); left <= 0 {
				stable = false
				break
			} else if left < wait {
				wait, capped = left, true
			}
			readCtx, cancel = context.WithTimeout(ctx, wait)
		}
		_, frame, err := conn.Read(readCtx)
		if cancel != nil {
			cancel()
		}
		if err != nil {
			if gotSnapshot && ctx.Err() == nil {
				stable = !capped || !errors.Is(err, context.DeadlineExceeded)
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
			if !gotSnapshot {
				gotSnapshot, settleBy = true, time.Now().Add(screenSettleMax)
			}
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
			if !gotSnapshot {
				gotSnapshot, settleBy = true, time.Now().Add(screenSettleMax)
			}
		}
	}
	if !stable {
		_, _ = fmt.Fprintf(e.Stderr, "webmux screen: output was still arriving after %s; this is a snapshot of a screen that kept changing, not a settled one\n", screenSettleMax)
	}
	if truncated {
		_, _ = fmt.Fprintln(e.Stderr, "webmux screen: the replay was truncated; the screen may be incomplete")
	}
	if cols == 0 {
		cols, rows = 80, 24
	}
	options := vt.NewOptions()
	options.Cols, options.Rows, options.Scrollback = cols, rows, scrollback
	term := vt.NewTerminal(options)
	for _, chunk := range pending {
		term.Write(chunk)
	}
	buffer := term.Buffer()
	view := screenView{
		Cols: cols, Rows: rows, AltScreen: buffer == term.Buffers().Alt(), Stable: stable,
		Cursor: position{Row: buffer.Y + 1, Col: buffer.X + 1},
	}
	// Without scrollback the screen is the last rows lines; with it the
	// history above them comes first.
	first := buffer.YBase
	if scrollback > 0 {
		first = 0
	}
	for y := first; y < buffer.YBase+rows; y++ {
		view.Lines = append(view.Lines, strings.TrimRight(buffer.TranslateBufferLineToString(y, true, 0, cols), " "))
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
