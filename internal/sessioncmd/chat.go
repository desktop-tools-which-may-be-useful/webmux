package sessioncmd

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"net/http"
	"net/url"
	"slices"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/coder/websocket"

	"github.com/rainchan/webmux/internal/contract"
	"github.com/rainchan/webmux/internal/localclient"
)

// The chat commands drive a hybrid session's chat side through the same
// endpoints as the Web UI's chat view: every write carries a fresh
// clientRequestId, so the server applies it once.

var chatSessionArgument = Item{Name: "SESSION", Text: `A hybrid session ID, or "self" for the current session.`}

func chatExits(extra ...ExitItem) []ExitItem {
	return exitsCommon(append([]ExitItem{{Code: 1, Text: "Not a hybrid session, on the wrong side, or the server refused the request."}}, extra...)...)
}

// hybridSession reads session arg and checks that it is a hybrid session,
// on its chat side when chat is set.
func (e Env) hybridSession(ctx context.Context, client *localclient.Client, arg string, chat bool) (contract.SessionInfo, int, bool) {
	id, code, ok := e.SessionID(arg)
	if !ok {
		return contract.SessionInfo{}, code, false
	}
	var info contract.SessionInfo
	if err := client.Do(ctx, http.MethodGet, "/api/sessions/"+url.PathEscape(id), nil, &info); err != nil {
		return info, e.Fail(err), false
	}
	if info.Hybrid == nil {
		_, _ = fmt.Fprintf(e.Stderr, "webmux: session %s is not a hybrid session; start one with webmux chat new or run --launch-config\n", id)
		return info, contract.ExitError, false
	}
	if chat && info.Hybrid.Side != contract.HybridSideACP {
		_, _ = fmt.Fprintf(e.Stderr, "webmux: session %s is on its terminal side; run webmux chat switch %s --to chat first, or use webmux send\n", id, id)
		return info, contract.ExitError, false
	}
	return info, contract.ExitOK, true
}

// chatFail prints a chat or hybrid command failure with its conflict
// reason and candidates.
func (e Env) chatFail(err error) int {
	conflict, ok := hybridConflict(err)
	if !ok {
		return e.Fail(err)
	}
	_, _ = fmt.Fprintf(e.Stderr, "webmux: %s (%s)\n", conflict.Error, conflict.Reason)
	if len(conflict.Candidates) > 0 {
		_, _ = fmt.Fprintln(e.Stderr, "webmux: pick an agent session with --agent-session:")
		for _, c := range conflict.Candidates {
			_, _ = fmt.Fprintf(e.Stderr, "webmux:   %s\t%s\t%s\n", c.AgentSessionID, c.UpdatedAt.Local().Format(time.DateTime), c.Title)
		}
	}
	return contract.ExitError
}

func hybridConflict(err error) (contract.HybridConflict, bool) {
	var apiErr *localclient.APIError
	var conflict contract.HybridConflict
	if !errors.As(err, &apiErr) || apiErr.Status != http.StatusConflict || json.Unmarshal(apiErr.Body, &conflict) != nil || conflict.Reason == "" {
		return conflict, false
	}
	return conflict, true
}

// chatCommand runs one chat side write for the session in args[0].
func (e Env) chatCommand(ctx context.Context, args []string, method, path string, body any) int {
	client, code := e.connect()
	if client == nil {
		return code
	}
	defer client.Close()
	info, code, ok := e.hybridSession(ctx, client, args[0], true)
	if !ok {
		return code
	}
	if err := client.Do(ctx, method, "/api/sessions/"+url.PathEscape(info.ID)+"/chat/"+path, body, nil); err != nil {
		return e.chatFail(err)
	}
	return contract.ExitOK
}

type attachFlag []string

func (f *attachFlag) String() string     { return strings.Join(*f, ",") }
func (f *attachFlag) Set(v string) error { *f = append(*f, v); return nil }

type chatSendOptions struct {
	attachments      attachFlag
	mode             string
	wait, asJSON     bool
	autoWaitRequests bool
	timeout          time.Duration
}

var chatSendCommand = &Command{
	Name: "chat send", Group: GroupChat, Args: "SESSION [TEXT|-]",
	Summary: "Send a prompt to a hybrid session's agent; with --wait, print its reply.",
	Description: "The prompt is sent at once when the agent is idle and otherwise joins the server-side queue every device shows " +
		"(queued is true in the result). --attach uploads files into the session first and sends them with the prompt; when " +
		"an upload fails nothing is sent. --mode steer injects the text into the running turn instead of queueing it.\n\n" +
		"--wait connects to the chat stream before sending and follows the turn this prompt starts (for a queued prompt, the " +
		"turn that later sends it; with steer, the running turn). It prints the agent's final message, or with --json the " +
		"turn's events. A turn that ends with end_turn exits 0, any other stop reason 1. When the agent asks for a permission " +
		"or an answer during the turn, the request is printed to standard error and the command exits 1, leaving the turn " +
		"waiting; --auto-wait-requests keeps waiting until someone answers it. A timeout exits 124 and does not cancel the turn.",
	Arguments: []Item{chatSessionArgument, {Name: "TEXT", Text: "The prompt; - reads standard input."}},
	Env:       []Item{sessionIDEnv},
	Output: "Without --wait, the ChatPromptResult as JSON (queued, itemId, turnId, promptId). With --wait, the reply text, " +
		"or with --json {stopReason, error, reply, events, request}.",
	Exit: chatExits(
		ExitItem{Code: 1, Text: "Also: an upload failed, the turn did not end with end_turn, or a request is waiting."},
		ExitItem{Code: 124, Text: "--wait timed out; the turn goes on."},
	),
	Examples: []Example{
		{Command: `webmux chat send abcd2345 "fix the lint errors and summarize the change" --wait --timeout 20m`, Text: "Hand a task to another agent and capture its answer."},
		{Command: "webmux chat send abcd2345 'what does this show?' --attach screenshot.png", Text: "Send a prompt with an image."},
		{Command: "git diff | webmux chat send abcd2345 - --wait --json", Text: "Review standard input and keep the turn's events."},
	},
	SeeAlso: []string{"webmux chat status", "webmux chat answer", "webmux chat queue"},
	Notes: map[string]Note{
		"attach": {Repeatable: true}, "timeout": {Requires: []string{"wait"}},
		"auto-wait-requests": {Requires: []string{"wait"}},
	},
	Bind: func(fs *flag.FlagSet) Action {
		var o chatSendOptions
		fs.Var(&o.attachments, "attach", "upload the file at `PATH` and send it with the prompt")
		fs.StringVar(&o.mode, "mode", "auto", "`MODE`: auto (send or queue) or steer (inject into the running turn)")
		fs.BoolVar(&o.wait, "wait", false, "follow the prompt's turn and print the reply")
		fs.DurationVar(&o.timeout, "timeout", 0, "give up --wait after this `DURATION` (exit 124); 0 waits forever")
		fs.BoolVar(&o.autoWaitRequests, "auto-wait-requests", false, "with --wait, keep waiting while a request is open")
		fs.BoolVar(&o.asJSON, "json", false, "with --wait, print the turn's events as JSON")
		return func(ctx context.Context, e Env, args []string) int { return e.chatSend(ctx, o, args) }
	},
}

func (e Env) chatSend(ctx context.Context, o chatSendOptions, args []string) int {
	if len(args) < 1 || len(args) > 2 {
		return e.Usagef("a session and at most one TEXT are required (quote text with spaces)")
	}
	if o.mode != "auto" && o.mode != "steer" {
		return e.Usagef("--mode is auto or steer")
	}
	text := ""
	if len(args) == 2 {
		text = args[1]
	}
	if text == "-" {
		read, err := e.readStdin()
		if err != nil {
			return e.Fail(err)
		}
		text = read
	}
	if strings.TrimSpace(text) == "" && len(o.attachments) == 0 {
		return e.Usagef("the prompt is empty")
	}
	client, code := e.connect()
	if client == nil {
		return code
	}
	defer client.Close()
	info, code, ok := e.hybridSession(ctx, client, args[0], true)
	if !ok {
		return code
	}
	base := "/api/sessions/" + url.PathEscape(info.ID)
	request := contract.ChatPromptRequest{ClientRequestID: newRequestID(), Text: text, Mode: o.mode}
	for _, file := range o.attachments {
		path, err := e.uploadFile(ctx, client, base+"/upload", file)
		if err != nil {
			return e.Fail(fmt.Errorf("upload %s: %w; nothing was sent", file, err))
		}
		request.Attachments = append(request.Attachments, contract.ChatAttachment{Path: path})
	}
	if !o.wait {
		var result contract.ChatPromptResult
		if err := client.Do(ctx, http.MethodPost, base+"/chat/prompts", request, &result); err != nil {
			return e.chatFail(err)
		}
		writeJSON(e.Stdout, result)
		return contract.ExitOK
	}
	waitCtx := ctx
	if o.timeout > 0 {
		var cancel context.CancelFunc
		waitCtx, cancel = context.WithTimeout(ctx, o.timeout)
		defer cancel()
	}
	// Subscribe first and read the snapshot, so the turn cannot start (or
	// a steered turn end) unseen.
	stream, err := openChat(waitCtx, client, info.ID)
	if err != nil {
		return e.Fail(err)
	}
	defer stream.close()
	if _, err := stream.read(waitCtx); err != nil {
		return e.waitFailure(err)
	}
	var result contract.ChatPromptResult
	if err := client.Do(ctx, http.MethodPost, base+"/chat/prompts", request, &result); err != nil {
		return e.chatFail(err)
	}
	w := turnWatch{stopAtRequest: !o.autoWaitRequests}
	switch {
	case result.Queued:
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

// queuedGone ends a wait for a queued prompt once it left the queue
// without a turn sending it. The turn.start that sends it is published
// before the state that drops it, so a missing item means it was removed.
func queuedGone(item string) func(*contract.ChatState) error {
	seen := false
	return func(state *contract.ChatState) error {
		present := slices.ContainsFunc(state.Queue.Items, func(it contract.ChatQueueItem) bool { return it.ID == item })
		if present {
			seen = true
			return nil
		}
		if seen {
			return fmt.Errorf("the queued prompt %s was removed before it was sent", item)
		}
		return nil
	}
}

var chatWatchCommand = &Command{
	Name: "chat watch", Group: GroupChat, Args: "SESSION",
	Summary: "Stream a hybrid session's chat: messages, tool calls, requests and turns.",
	Description: "Watching follows the chat stream until interrupted or the session ends. Text output shows each user and agent " +
		"message, tool call titles with their status, requests and turn boundaries; --json prints every server frame as one " +
		"line of JSON (the ChatFrame of /ws/s/{id}/chat). Earlier history is skipped unless --history is given. Watching " +
		"works on either side and changes nothing.",
	Arguments: []Item{chatSessionArgument},
	Env:       []Item{sessionIDEnv},
	Output:    "Text lines, or NDJSON frames with --json.",
	Exit:      chatExits(ExitItem{Code: 130, Text: "Interrupted."}),
	Examples: []Example{
		{Command: "webmux chat watch abcd2345", Text: "Follow what an agent is doing."},
		{Command: "webmux chat watch abcd2345 --json | jq -c 'select(.type==\"events\") | .items[]'", Text: "Process chat events in a script."},
	},
	SeeAlso: []string{"webmux chat status", "webmux events"},
	Bind: func(fs *flag.FlagSet) Action {
		asJSON := fs.Bool("json", false, "print each frame as one line of JSON")
		history := fs.Bool("history", false, "also print the history the server still holds")
		return func(ctx context.Context, e Env, args []string) int {
			if len(args) != 1 {
				return e.Usagef("exactly one session is required")
			}
			client, code := e.connect()
			if client == nil {
				return code
			}
			defer client.Close()
			info, code, ok := e.hybridSession(ctx, client, args[0], false)
			if !ok {
				return code
			}
			stream, err := openChat(ctx, client, info.ID)
			if err != nil {
				return e.Fail(err)
			}
			defer stream.close()
			err = e.watchChat(ctx, stream, *asJSON, *history)
			if ctx.Err() != nil {
				return 130
			}
			if err != nil {
				return e.Fail(err)
			}
			return contract.ExitOK
		}
	},
}

func (e Env) watchChat(ctx context.Context, stream *chatStream, asJSON, history bool) error {
	out := bufio.NewWriter(e.Stdout)
	defer func() { _ = out.Flush() }()
	text := chatText{w: out}
	replay := false
	for {
		frame, err := stream.read(ctx)
		if err != nil {
			if status := websocket.CloseStatus(err); status == websocket.StatusNormalClosure || status == contract.CloseMoved {
				return nil
			}
			return err
		}
		// The events frame right after a snapshot replays the history.
		skip := replay && !history && frame.Type == contract.ChatFrameEvents
		replay = frame.Type == contract.ChatFrameSnapshot
		if skip {
			continue
		}
		if asJSON {
			line, _ := json.Marshal(frame)
			_, _ = out.Write(append(line, '\n'))
		} else {
			for _, ev := range frame.Items {
				text.event(ev)
			}
		}
		if err := out.Flush(); err != nil {
			return err
		}
	}
}

// chatText renders chat events as lines; message chunks continue the
// line of the message they belong to.
type chatText struct {
	w       *bufio.Writer
	message string
}

func (t *chatText) line(format string, args ...any) {
	if t.message != "" {
		_, _ = t.w.WriteString("\n")
		t.message = ""
	}
	_, _ = fmt.Fprintf(t.w, format+"\n", args...)
}

func (t *chatText) event(ev contract.ChatEvent) {
	switch {
	case ev.Message != nil:
		if ev.Message.Role == "thought" {
			return
		}
		if t.message != ev.Message.MessageID {
			if t.message != "" {
				_, _ = t.w.WriteString("\n")
			}
			t.message = ev.Message.MessageID
			_, _ = fmt.Fprintf(t.w, "%s: ", ev.Message.Role)
		}
		_, _ = t.w.WriteString(ev.Message.Text)
	case ev.Tool != nil && (ev.Tool.Title != nil || ev.Tool.Status != nil):
		title, status := "", ""
		if ev.Tool.Title != nil {
			title = *ev.Tool.Title
		}
		if ev.Tool.Status != nil {
			status = " (" + *ev.Tool.Status + ")"
		}
		t.line("tool %s: %s%s", ev.Tool.ToolCallID, title, status)
	case ev.TurnStart != nil:
		t.line("--- turn %s started", ev.TurnStart.TurnID)
	case ev.TurnEnd != nil:
		reason := ev.TurnEnd.StopReason
		if ev.TurnEnd.Error != "" {
			reason += ": " + ev.TurnEnd.Error
		}
		t.line("--- turn %s ended: %s", ev.TurnEnd.TurnID, reason)
	case ev.Request != nil:
		t.line("request %s (%s): %s", ev.Request.RequestID, ev.Request.Kind, requestSummary(*ev.Request))
	case ev.Resolved != nil:
		t.line("request %s closed: %s", ev.Resolved.RequestID, ev.Resolved.Outcome)
	case ev.Notice != nil:
		t.line("%s: %s", ev.Notice.Level, ev.Notice.Text)
	}
}

func requestSummary(r contract.ChatRequest) string {
	if r.Title != "" {
		return r.Title
	}
	if r.Form != nil {
		return r.Form.Message
	}
	return ""
}

// chatSnapshot reads the chat state once.
func chatSnapshot(ctx context.Context, client *localclient.Client, id string) (contract.ChatState, error) {
	stream, err := openChat(ctx, client, id)
	if err != nil {
		return contract.ChatState{}, err
	}
	defer stream.close()
	for stream.state == nil {
		if _, err := stream.read(ctx); err != nil {
			return contract.ChatState{}, err
		}
	}
	return *stream.state, nil
}

var chatStatusCommand = &Command{
	Name: "chat status", Group: GroupChat, Args: "SESSION",
	Summary: "Show a hybrid session's side, turn, queue, permission mode, pending requests and agent settings.",
	Description: "Status reads the chat state once. Pending requests are listed with their IDs and options, which webmux chat " +
		"answer takes; the agent settings list the IDs and values webmux chat set takes.",
	Arguments: []Item{chatSessionArgument},
	Env:       []Item{sessionIDEnv},
	Output:    "Aligned lines; with --json, the ChatState object.",
	Exit:      chatExits(),
	Examples: []Example{
		{Command: "webmux chat status abcd2345", Text: "See whether the agent is busy or waiting for an answer."},
		{Command: "webmux chat status self --json | jq '.requests[].requestId'", Text: "List pending request IDs."},
	},
	SeeAlso: []string{"webmux chat answer", "webmux chat set", "webmux get"},
	Bind: func(fs *flag.FlagSet) Action {
		asJSON := fs.Bool("json", false, "print the ChatState object")
		return func(ctx context.Context, e Env, args []string) int {
			if len(args) != 1 {
				return e.Usagef("exactly one session is required")
			}
			client, code := e.connect()
			if client == nil {
				return code
			}
			defer client.Close()
			info, code, ok := e.hybridSession(ctx, client, args[0], false)
			if !ok {
				return code
			}
			state, err := chatSnapshot(ctx, client, info.ID)
			if err != nil {
				return e.Fail(err)
			}
			if *asJSON {
				writeJSON(e.Stdout, state)
				return contract.ExitOK
			}
			e.printChatState(info, state)
			return contract.ExitOK
		}
	},
}

func (e Env) printChatState(info contract.SessionInfo, state contract.ChatState) {
	w := tabwriter.NewWriter(e.Stdout, 0, 4, 2, ' ', 0)
	side := string(state.Side)
	if state.Phase != "" {
		side += " (" + string(state.Phase) + ")"
	}
	turn := "idle"
	switch {
	case state.Turn.Running:
		turn = "running " + state.Turn.TurnID
	case state.Turn.TurnID != "":
		turn = "idle; " + state.Turn.TurnID + " ended with " + state.Turn.StopReason
	}
	queue := fmt.Sprintf("%d", len(state.Queue.Items))
	if state.Queue.Paused {
		queue += " (paused)"
	}
	agentSession := "-"
	if state.AgentSessionID != nil {
		agentSession = *state.AgentSessionID
	}
	for _, row := range [][2]string{
		{"session", info.ID}, {"agent", state.Agent}, {"agent session", agentSession}, {"side", side},
		{"turn", turn}, {"queue", queue}, {"permission", string(state.PermissionMode)},
	} {
		_, _ = fmt.Fprintf(w, "%s\t%s\n", row[0], row[1])
	}
	_ = w.Flush()
	if len(state.Requests) > 0 {
		_, _ = fmt.Fprintln(e.Stdout, "\nPending requests:")
		for _, r := range state.Requests {
			_, _ = fmt.Fprintf(e.Stdout, "  %s (%s) %s\n", r.RequestID, r.Kind, requestSummary(r))
			for _, o := range r.Options {
				_, _ = fmt.Fprintf(e.Stdout, "      --option %s  %s\n", o.OptionID, o.Name)
			}
			if r.Form != nil {
				for _, f := range r.Form.Fields {
					_, _ = fmt.Fprintf(e.Stdout, "      field %s (%s) %s\n", f.Name, f.Type, f.Title)
				}
			}
		}
	}
	if len(state.Config) > 0 {
		_, _ = fmt.Fprintln(e.Stdout, "\nAgent settings (webmux chat set ID=VALUE):")
		for _, c := range state.Config {
			value := c.Value
			if c.Type == "boolean" && c.Enabled != nil {
				value = fmt.Sprint(*c.Enabled)
			}
			choices := make([]string, 0, len(c.Options))
			for _, o := range c.Options {
				choices = append(choices, o.Value)
			}
			extra := ""
			if len(choices) > 0 {
				extra = "  [" + strings.Join(choices, ", ") + "]"
			}
			_, _ = fmt.Fprintf(e.Stdout, "  %s=%s  %s%s\n", c.ID, value, c.Name, extra)
		}
	}
}

var chatCancelCommand = &Command{
	Name: "chat cancel", Group: GroupChat, Args: "SESSION",
	Summary:   "Cancel the running turn of a hybrid session; nothing happens when the agent is idle.",
	Arguments: []Item{chatSessionArgument},
	Env:       []Item{sessionIDEnv},
	Exit:      chatExits(),
	Examples: []Example{
		{Command: "webmux chat cancel abcd2345", Text: "Stop the agent's current turn."},
		{Command: "webmux chat cancel abcd2345 && webmux chat send abcd2345 'start over with the smaller fix'", Text: "Redirect the agent."},
	},
	SeeAlso: []string{"webmux chat queue", "webmux send"},
	Bind: func(fs *flag.FlagSet) Action {
		return func(ctx context.Context, e Env, args []string) int {
			if len(args) != 1 {
				return e.Usagef("exactly one session is required")
			}
			return e.chatCommand(ctx, args, http.MethodPost, "cancel", contract.ChatCommandRequest{ClientRequestID: newRequestID()})
		}
	},
}

var chatAnswerCommand = &Command{
	Name: "chat answer", Group: GroupChat, Args: "SESSION REQUEST",
	Summary: "Answer an agent's pending permission request or question.",
	Description: "A permission request takes --option with one of its option IDs, or --cancel. A form (a question) takes " +
		"--content with a JSON object of field values, --decline or --cancel. Every device closes the request card at once. " +
		"webmux chat status lists pending requests with their options and fields.",
	Arguments: []Item{chatSessionArgument, {Name: "REQUEST", Text: "The request ID from chat status, chat watch or a stopped --wait."}},
	Env:       []Item{sessionIDEnv},
	Exit:      chatExits(ExitItem{Code: 1, Text: "Also: no such pending request, or it was answered already."}),
	Examples: []Example{
		{Command: "webmux chat answer abcd2345 r3 --option allow_once", Text: "Approve one tool call."},
		{Command: `webmux chat answer abcd2345 r4 --content '{"plan":"B"}'`, Text: "Answer a question form."},
		{Command: "webmux chat answer abcd2345 r3 --cancel", Text: "Dismiss a request."},
	},
	SeeAlso: []string{"webmux chat status", "webmux chat permission"},
	Notes: map[string]Note{
		"option":  {Conflicts: []string{"cancel", "content", "decline"}},
		"cancel":  {Conflicts: []string{"option", "content", "decline"}},
		"content": {Conflicts: []string{"option", "cancel", "decline"}},
		"decline": {Conflicts: []string{"option", "cancel", "content"}},
	},
	Bind: func(fs *flag.FlagSet) Action {
		option := fs.String("option", "", "select the permission option `ID`")
		cancelIt := fs.Bool("cancel", false, "cancel the request")
		content := fs.String("content", "", "submit a form with this `JSON` object of field values")
		decline := fs.Bool("decline", false, "decline a form")
		return func(ctx context.Context, e Env, args []string) int {
			if len(args) != 2 {
				return e.Usagef("a session and a request ID are required")
			}
			if countTrue(*option != "", *cancelIt, *content != "", *decline) != 1 {
				return e.Usagef("give exactly one of --option, --cancel, --content or --decline")
			}
			var fields map[string]json.RawMessage
			if *content != "" {
				if err := json.Unmarshal([]byte(*content), &fields); err != nil {
					return e.Usagef("--content must be a JSON object: %v", err)
				}
			}
			client, code := e.connect()
			if client == nil {
				return code
			}
			defer client.Close()
			info, code, ok := e.hybridSession(ctx, client, args[0], true)
			if !ok {
				return code
			}
			state, err := chatSnapshot(ctx, client, info.ID)
			if err != nil {
				return e.Fail(err)
			}
			i := slices.IndexFunc(state.Requests, func(r contract.ChatRequest) bool { return r.RequestID == args[1] })
			if i < 0 {
				_, _ = fmt.Fprintf(e.Stderr, "webmux: no pending request %s (answered already, or see webmux chat status)\n", args[1])
				return contract.ExitError
			}
			request := contract.ChatAnswerRequest{ClientRequestID: newRequestID()}
			form := state.Requests[i].Kind == "form"
			switch {
			case *option != "" && form, (*content != "" || *decline) && !form:
				return e.Usagef("request %s is a %s; see webmux chat answer --help", args[1], state.Requests[i].Kind)
			case *option != "":
				request.Outcome, request.OptionID = "selected", *option
			case *cancelIt && form:
				request.Outcome = "cancel"
			case *cancelIt:
				request.Outcome = "cancelled"
			case *decline:
				request.Outcome = "decline"
			default:
				request.Outcome, request.Content = "accept", fields
			}
			if err := client.Do(ctx, http.MethodPost, "/api/sessions/"+url.PathEscape(info.ID)+"/chat/requests/"+url.PathEscape(args[1]), request, nil); err != nil {
				return e.chatFail(err)
			}
			return contract.ExitOK
		}
	},
}

var chatSwitchCommand = &Command{
	Name: "chat switch", Group: GroupChat, Args: "SESSION",
	Summary: "Move a hybrid session to its chat or terminal side and wait until the switch is over.",
	Description: "Switching resumes the same agent session on the other side. Terminal clients (webmux attach, browsers) are " +
		"detached when the session moves to chat, and chat views close when it moves to the terminal. When the terminal UI " +
		"does not become idle within its interrupt timeout, the switch needs a choice: --force interrupts it, and without " +
		"--force the switch is cancelled and the command exits 1. When the agent's session cannot be identified, the " +
		"candidates are listed; repeat with --agent-session.",
	Arguments: []Item{chatSessionArgument},
	Env:       []Item{sessionIDEnv},
	Output:    "Nothing; with --json, the HybridSwitchResult (side, draft of queued prompts returned to the terminal, dropped and unsupported arguments).",
	Exit:      chatExits(ExitItem{Code: 1, Text: "Also: the switch needed a choice without --force, or the agent session is ambiguous."}),
	Examples: []Example{
		{Command: "webmux chat switch abcd2345 --to chat && webmux chat send abcd2345 continue", Text: "Move to chat, then prompt."},
		{Command: "webmux chat switch abcd2345 --to terminal --force", Text: "Return to the terminal UI even if it is busy."},
	},
	SeeAlso: []string{"webmux chat status", "webmux attach"},
	Bind: func(fs *flag.FlagSet) Action {
		to := fs.String("to", "", "the target `SIDE`: chat or terminal (required)")
		force := fs.Bool("force", false, "interrupt a busy terminal UI instead of cancelling the switch")
		agentSession := fs.String("agent-session", "", "resume this agent session `ID` when the server lists candidates")
		asJSON := fs.Bool("json", false, "print the HybridSwitchResult")
		return func(ctx context.Context, e Env, args []string) int {
			if len(args) != 1 {
				return e.Usagef("exactly one session is required")
			}
			side, ok := sideNamed(*to)
			if !ok {
				return e.Usagef("--to is chat or terminal")
			}
			client, code := e.connect()
			if client == nil {
				return code
			}
			defer client.Close()
			info, code, ok := e.hybridSession(ctx, client, args[0], false)
			if !ok {
				return code
			}
			if info.Hybrid.Side == side {
				if *asJSON {
					writeJSON(e.Stdout, contract.HybridSwitchResult{Side: side, Dropped: []string{}, Unsupported: []string{}})
				}
				return contract.ExitOK
			}
			result, chose, err := e.switchSide(ctx, client, info.ID, contract.HybridSwitchRequest{
				ClientRequestID: newRequestID(), To: side, AgentSessionID: *agentSession,
			}, *force)
			if err != nil {
				return e.chatFail(err)
			}
			if chose == contract.HybridChoiceCancel {
				_, _ = fmt.Fprintln(e.Stderr, "webmux: the terminal UI stayed busy, so the switch was cancelled; repeat with --force to interrupt it")
				return contract.ExitError
			}
			if result.Draft != "" {
				_, _ = fmt.Fprintln(e.Stderr, "webmux: queued prompts were returned as a draft (see --json)")
			}
			if *asJSON {
				writeJSON(e.Stdout, result)
			}
			return contract.ExitOK
		}
	},
}

func sideNamed(name string) (contract.HybridSide, bool) {
	switch name {
	case "chat":
		return contract.HybridSideACP, true
	case "terminal":
		return contract.HybridSideTUI, true
	}
	return "", false
}

// switchSide runs a switch and answers its choice, when it pauses for one,
// with force or cancel. It reports the choice it made.
func (e Env) switchSide(ctx context.Context, client *localclient.Client, id string, request contract.HybridSwitchRequest, force bool) (contract.HybridSwitchResult, string, error) {
	watch, err := watchSessions(ctx, client)
	if err != nil {
		return contract.HybridSwitchResult{}, "", err
	}
	defer watch.close()
	type outcome struct {
		result contract.HybridSwitchResult
		err    error
	}
	done := make(chan outcome, 1)
	go func() {
		var result contract.HybridSwitchResult
		err := client.Do(ctx, http.MethodPost, "/api/sessions/"+url.PathEscape(id)+"/switch", request, &result)
		done <- outcome{result, err}
	}()
	choiceCtx, stop := context.WithCancel(ctx)
	defer stop()
	choices := make(chan string, 1)
	go func() {
		awaiting := func(s contract.SessionInfo) bool {
			return s.ID == id && s.Hybrid != nil && s.Hybrid.Phase == contract.HybridPhaseAwaitingChoice
		}
		for {
			event, err := watch.next(choiceCtx)
			if err != nil {
				return
			}
			if (event.Session != nil && awaiting(*event.Session)) || slices.ContainsFunc(event.Sessions, awaiting) {
				choice := contract.HybridChoiceCancel
				if force {
					choice = contract.HybridChoiceForce
				}
				choices <- choice
				return
			}
		}
	}()
	chose := ""
	for {
		select {
		case o := <-done:
			return o.result, chose, o.err
		case choice := <-choices:
			chose = choice
			answer := contract.HybridSwitchRequest{ClientRequestID: newRequestID(), Choice: choice}
			if err := client.Do(ctx, http.MethodPost, "/api/sessions/"+url.PathEscape(id)+"/switch", answer, nil); err != nil {
				return contract.HybridSwitchResult{}, chose, err
			}
		}
	}
}
