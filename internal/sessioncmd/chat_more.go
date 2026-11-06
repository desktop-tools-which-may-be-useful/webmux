package sessioncmd

import (
	"bufio"
	"context"
	"errors"
	"flag"
	"fmt"
	"net/http"
	"net/url"
	"slices"
	"strconv"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/rainchan/webmux/internal/attach"
	"github.com/rainchan/webmux/internal/contract"
	"github.com/rainchan/webmux/internal/localclient"
)

var hybridStates = []string{"active", "closed", "interrupted"}

type stringsFlag []string

func (f *stringsFlag) String() string     { return strings.Join(*f, ",") }
func (f *stringsFlag) Set(v string) error { *f = append(*f, v); return nil }

var recordArgument = Item{Name: "RECORD", Text: "A hybrid session record ID (the session ID it had), from webmux chat ls."}

var chatListCommand = &Command{
	Name: "chat ls", Group: GroupChat, Args: "",
	Summary: "List hybrid session records: running, closed and interrupted agent sessions that can be resumed.",
	Description: "Every hybrid session keeps a record linking its WebMux session ID to the agent's own session, so it can be " +
		"resumed after it was closed or the server restarted. Records never contain messages, queued text or environment " +
		"values. --archived lists the closed and interrupted ones, the same as --state closed --state interrupted.",
	Output: "A table: ID, STATE, SIDE, AGENT, UPDATED, NAME, DIRECTORY. With --json, the HybridRecord array, most recently updated first.",
	Exit:   exitsCommon(ExitItem{Code: 1, Text: "The server refused the request or --repo names no repository."}),
	Examples: []Example{
		{Command: "webmux chat ls --archived", Text: "Find an agent session to resume."},
		{Command: "webmux chat ls --repo . --state active --json", Text: "List the running agents of the current repository."},
	},
	SeeAlso: []string{"webmux chat resume", "webmux chat forget", "webmux ls"},
	Notes:   map[string]Note{"state": {Repeatable: true}},
	Bind: func(fs *flag.FlagSet) Action {
		var states stringsFlag
		fs.Var(&states, "state", "keep records in `STATE`: active, closed or interrupted")
		archived := fs.Bool("archived", false, "keep closed and interrupted records")
		repo := fs.String("repo", "", "keep records of the registered repository `REPO` (ID, path, or . for the current one)")
		asJSON := fs.Bool("json", false, "print the HybridRecord array")
		return func(ctx context.Context, e Env, args []string) int {
			if len(args) != 0 {
				return e.Usagef("chat ls takes no arguments")
			}
			for _, s := range states {
				if !slices.Contains(hybridStates, s) {
					return e.Usagef("unknown state %q (%s)", s, strings.Join(hybridStates, ", "))
				}
			}
			if *archived {
				states = append(states, "closed", "interrupted")
			}
			client, code := e.connect()
			if client == nil {
				return code
			}
			defer client.Close()
			var records []contract.HybridRecord
			if err := client.Do(ctx, http.MethodGet, "/api/hybrid-sessions", nil, &records); err != nil {
				return e.chatFail(err)
			}
			keep := func(contract.HybridRecord) bool { return true }
			if *repo != "" {
				list, err := repositories(ctx, client)
				if err != nil {
					return e.Fail(err)
				}
				r, err := e.resolveRepository(ctx, list, *repo)
				if err != nil {
					return e.Fail(err)
				}
				keep = func(rec contract.HybridRecord) bool { return recordInRepository(rec, r) }
			}
			out := []contract.HybridRecord{}
			for _, rec := range records {
				if (len(states) == 0 || slices.Contains(states, string(rec.State))) && keep(rec) {
					out = append(out, rec)
				}
			}
			if *asJSON {
				writeJSON(e.Stdout, out)
				return contract.ExitOK
			}
			w := tabwriter.NewWriter(e.Stdout, 0, 4, 2, ' ', 0)
			_, _ = fmt.Fprintln(w, "ID\tSTATE\tSIDE\tAGENT\tUPDATED\tNAME\tDIRECTORY")
			for _, rec := range out {
				name := rec.Name
				if name == "" {
					name = rec.Title
				}
				_, _ = fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\t%s\t%s\n", rec.ID, rec.State, sideName(rec.Side), rec.Agent,
					rec.UpdatedAt.Local().Format(time.DateTime), oneLine(name, 40), rec.Cwd)
			}
			_ = w.Flush()
			return contract.ExitOK
		}
	},
}

func recordInRepository(rec contract.HybridRecord, repo contract.Repository) bool {
	if rec.Workspace != nil {
		return rec.Workspace.RepositoryID == repo.ID
	}
	return slices.ContainsFunc(repo.Worktrees, func(t contract.Worktree) bool { return within(t.Path, rec.Cwd) })
}

// sideName is the CLI's name of a side.
func sideName(side contract.HybridSide) string {
	switch side {
	case contract.HybridSideACP:
		return "chat"
	case contract.HybridSideTUI:
		return "terminal"
	}
	return string(side)
}

// oneLine shortens text to its first line and at most n runes.
func oneLine(text string, n int) string {
	text, _, _ = strings.Cut(strings.TrimSpace(text), "\n")
	if runes := []rune(text); len(runes) > n {
		return string(runes[:n-1]) + "…"
	}
	return text
}

var chatResumeCommand = &Command{
	Name: "chat resume", Group: GroupChat, Args: "RECORD",
	Summary: "Resume a closed or interrupted hybrid session under its old ID.",
	Description: "The agent's own session is loaded again, on the side the record was on unless --side says otherwise. Like " +
		"webmux run, a session resumed on its terminal side is attached when there is a terminal; otherwise, or with " +
		"--detach, its ID and link are printed. When the launch config it was started from was deleted, resuming needs " +
		"--without-env (the saved environment is gone). When the agent's session cannot be found, the candidates are listed; " +
		"repeat with --agent-session.",
	Arguments: []Item{recordArgument},
	Output:    "Attached output, or \"ID<TAB>LINK\"; with --json, {session, link}.",
	Exit: exitsCommon(
		ExitItem{Code: 1, Text: "No such record, it is active, its config was deleted without --without-env, or the agent session is ambiguous."},
	),
	Examples: []Example{
		{Command: "webmux chat resume abcd2345 --side chat --detach", Text: "Bring back yesterday's interrupted agent in chat."},
		{Command: "webmux chat resume abcd2345 --agent-session ses_42", Text: "Resume a chosen agent session after a lookup failed."},
	},
	SeeAlso: []string{"webmux chat ls", "webmux chat forget", "webmux attach"},
	Bind: func(fs *flag.FlagSet) Action {
		side := fs.String("side", "", "resume on `SIDE`: chat or terminal (default: the side it was on)")
		agentSession := fs.String("agent-session", "", "resume this agent session `ID` when the server lists candidates")
		withoutEnv := fs.Bool("without-env", false, "resume although its launch config (and environment) was deleted")
		detach := fs.Bool("detach", false, "print the session ID and link instead of attaching")
		asJSON := fs.Bool("json", false, "print {session, link} instead of attaching")
		return func(ctx context.Context, e Env, args []string) int {
			if len(args) != 1 {
				return e.Usagef("exactly one record ID is required")
			}
			client, code := e.connect()
			if client == nil {
				return code
			}
			defer client.Close()
			var record contract.HybridRecord
			if err := client.Do(ctx, http.MethodGet, "/api/hybrid-sessions/"+url.PathEscape(args[0]), nil, &record); err != nil {
				return e.chatFail(err)
			}
			request := contract.HybridResumeRequest{ClientRequestID: newRequestID(), Side: contract.HybridSideTUI,
				AgentSessionID: *agentSession, ConfirmWithoutEnv: *withoutEnv}
			if record.Side == contract.HybridSideACP {
				request.Side = contract.HybridSideACP
			}
			if *side != "" {
				s, ok := sideNamed(*side)
				if !ok {
					return e.Usagef("--side is chat or terminal")
				}
				request.Side = s
			}
			var console Console
			if request.Side == contract.HybridSideTUI && !*detach && !*asJSON && e.OpenConsole != nil {
				c, err := e.OpenConsole()
				switch {
				case errors.Is(err, attach.ErrNotTerminal):
					_, _ = fmt.Fprintln(e.Stderr, "webmux chat resume: no terminal; detaching")
				case err != nil:
					return e.Fail(err)
				default:
					console = c
				}
			}
			var info contract.SessionInfo
			if err := client.Do(ctx, http.MethodPost, "/api/sessions/"+url.PathEscape(record.ID)+"/resume", request, &info); err != nil {
				if console != nil {
					_ = console.Restore()
				}
				if conflict, ok := hybridConflict(err); ok && conflict.Reason == "config_missing" {
					_, _ = fmt.Fprintf(e.Stderr, "webmux: %s; repeat with --without-env to resume without the deleted config's environment\n", conflict.Error)
					return contract.ExitError
				}
				return e.chatFail(err)
			}
			if console != nil {
				if link, err := link(ctx, client, info.ID); err == nil {
					_, _ = fmt.Fprintf(e.Stderr, "webmux: session %s · %s · Ctrl+] to detach\r\n", info.ID, link)
				}
				return e.attach(ctx, client, info.ID, false, console)
			}
			return e.detached(ctx, client, info, runOptions{asJSON: *asJSON})
		}
	},
}

var chatForgetCommand = &Command{
	Name: "chat forget", Group: GroupChat, Args: "RECORD",
	Summary: "Delete a closed or interrupted hybrid session record; the agent keeps its own session.",
	Description: "Forgetting removes the record, its exited session and its artifacts folder, so it can no longer be resumed " +
		"from WebMux. The agent's session history stays with the agent. An active record is refused. On a terminal the " +
		"command asks first; without a terminal it needs --yes.",
	Arguments: []Item{recordArgument},
	Exit: exitsCommon(
		ExitItem{Code: 1, Text: "No such record, or it is active."},
		ExitItem{Code: 2, Text: "Also: no terminal to ask and no --yes."},
	),
	Examples: []Example{
		{Command: "webmux chat forget abcd2345", Text: "Delete a record after confirming."},
		{Command: "webmux chat ls --state closed --json | jq -r '.[].id' | xargs -n1 webmux chat forget --yes", Text: "Delete all closed records."},
	},
	SeeAlso: []string{"webmux chat ls", "webmux rm"},
	Bind: func(fs *flag.FlagSet) Action {
		yes := fs.Bool("yes", false, "do not ask for confirmation")
		return func(ctx context.Context, e Env, args []string) int {
			if len(args) != 1 {
				return e.Usagef("exactly one record ID is required")
			}
			client, code := e.connect()
			if client == nil {
				return code
			}
			defer client.Close()
			var record contract.HybridRecord
			if err := client.Do(ctx, http.MethodGet, "/api/hybrid-sessions/"+url.PathEscape(args[0]), nil, &record); err != nil {
				return e.chatFail(err)
			}
			if record.State == contract.HybridState("active") {
				_, _ = fmt.Fprintf(e.Stderr, "webmux: record %s is active; end the session first (webmux rm %s)\n", record.ID, record.ID)
				return contract.ExitError
			}
			if !*yes {
				name := record.Name
				if name == "" {
					name = record.Title
				}
				question := fmt.Sprintf("Forget %s record %s (%s, %s)?", record.State, record.ID, record.Agent, oneLine(name, 40))
				if code, ok := e.confirm(question); !ok {
					return code
				}
			}
			if err := client.Do(ctx, http.MethodDelete, "/api/hybrid-sessions/"+url.PathEscape(record.ID), nil, nil); err != nil {
				return e.chatFail(err)
			}
			return contract.ExitOK
		}
	},
}

// confirm asks question on a terminal. Without one it fails with a usage
// error naming --yes; a refusal exits 1.
func (e Env) confirm(question string) (int, bool) {
	if e.Interactive == nil || !e.Interactive() || e.Stdin == nil {
		return e.Usagef("not a terminal: repeat with --yes to confirm"), false
	}
	_, _ = fmt.Fprintf(e.Stderr, "%s [y/N] ", question)
	answer, _ := bufio.NewReader(e.Stdin).ReadString('\n')
	if a := strings.ToLower(strings.TrimSpace(answer)); a == "y" || a == "yes" {
		return contract.ExitOK, true
	}
	_, _ = fmt.Fprintln(e.Stderr, "webmux: cancelled")
	return contract.ExitError, false
}

var chatSetCommand = &Command{
	Name: "chat set", Group: GroupChat, Args: "SESSION ID=VALUE...",
	Summary: "Change the agent's settings, such as its model or mode, on the chat side.",
	Description: "Each ID is one of the agent's settings as webmux chat status lists them. A select setting takes one of its " +
		"values; a boolean setting takes true or false. Settings are applied in order; when one fails it is reported, the " +
		"command exits 1, and the ones already applied stay. Every device shows the change.",
	Arguments: []Item{chatSessionArgument, {Name: "ID=VALUE", Text: "A setting and its new value."}},
	Env:       []Item{sessionIDEnv},
	Exit:      chatExits(ExitItem{Code: 1, Text: "Also: an unknown setting or value."}),
	Examples: []Example{
		{Command: "webmux chat set abcd2345 model=sonnet", Text: "Switch the agent's model."},
		{Command: "webmux chat status abcd2345", Text: "List the settings and their values first."},
	},
	SeeAlso: []string{"webmux chat status", "webmux chat permission", "webmux launch-config options"},
	Bind: func(fs *flag.FlagSet) Action {
		return func(ctx context.Context, e Env, args []string) int {
			if len(args) < 2 {
				return e.Usagef("a session and at least one ID=VALUE are required")
			}
			type change struct{ id, value string }
			var changes []change
			for _, arg := range args[1:] {
				id, value, ok := strings.Cut(arg, "=")
				if !ok || id == "" {
					return e.Usagef("expected ID=VALUE, got %q", arg)
				}
				changes = append(changes, change{id, value})
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
			for _, c := range changes {
				if err := e.setChatConfig(ctx, client, info.ID, state.Config, c.id, c.value); err != nil {
					_, _ = fmt.Fprintf(e.Stderr, "webmux: %s=%s: ", c.id, c.value)
					return e.chatFail(err)
				}
			}
			return contract.ExitOK
		}
	},
}

func (e Env) setChatConfig(ctx context.Context, client *localclient.Client, id string, config []contract.ChatConfig, key, value string) error {
	i := slices.IndexFunc(config, func(c contract.ChatConfig) bool { return c.ID == key })
	if i < 0 {
		ids := make([]string, 0, len(config))
		for _, c := range config {
			ids = append(ids, c.ID)
		}
		return fmt.Errorf("unknown setting (the agent offers: %s)", strings.Join(ids, ", "))
	}
	request := contract.ChatConfigRequest{ClientRequestID: newRequestID()}
	if config[i].Type == "boolean" {
		enabled, err := strconv.ParseBool(value)
		if err != nil {
			return errors.New("a boolean setting takes true or false")
		}
		request.Enabled = &enabled
	} else {
		values := make([]string, 0, len(config[i].Options))
		for _, o := range config[i].Options {
			values = append(values, o.Value)
		}
		if len(values) > 0 && !slices.Contains(values, value) {
			return fmt.Errorf("unknown value (choose from: %s)", strings.Join(values, ", "))
		}
		request.Value = &value
	}
	return client.Do(ctx, http.MethodPut, "/api/sessions/"+url.PathEscape(id)+"/chat/config/"+url.PathEscape(key), request, nil)
}

var chatPermissionCommand = &Command{
	Name: "chat permission", Group: GroupChat, Args: "SESSION ask|auto",
	Summary: "Choose whether the agent's permission requests wait for an answer (ask) or are approved by the server (auto).",
	Description: "auto approves each permission request on the server with its first allow-once option (else allow-always); " +
		"questions (forms) always wait for an answer. The mode applies to the chat side and every device shows it.",
	Arguments: []Item{chatSessionArgument, {Name: "MODE", Text: "ask or auto."}},
	Env:       []Item{sessionIDEnv},
	Exit:      chatExits(),
	Examples: []Example{
		{Command: "webmux chat permission abcd2345 ask", Text: "Review every tool call before it runs."},
		{Command: "webmux chat permission self auto", Text: "Let the current agent run tools without asking."},
	},
	SeeAlso: []string{"webmux chat answer", "webmux chat status"},
	Bind: func(fs *flag.FlagSet) Action {
		return func(ctx context.Context, e Env, args []string) int {
			if len(args) != 2 || args[1] != "ask" && args[1] != "auto" {
				return e.Usagef("a session and a mode (ask or auto) are required")
			}
			return e.chatCommand(ctx, args, http.MethodPut, "permission-mode",
				contract.ChatPermissionModeRequest{ClientRequestID: newRequestID(), Mode: contract.ChatPermissionMode(args[1])})
		}
	},
}

var chatQueueGroup = &Command{
	Name: "chat queue", Group: GroupChat,
	Summary: "Inspect and edit the prompts waiting in a hybrid session's queue.",
}

var queueItemArgument = Item{Name: "ITEM", Text: "A queued prompt's ID from webmux chat queue ls."}

var chatQueueListCommand = &Command{
	Name: "chat queue ls", Group: GroupChat, Args: "SESSION",
	Summary:   "List the queued prompts in the order they will be sent.",
	Arguments: []Item{chatSessionArgument},
	Env:       []Item{sessionIDEnv},
	Output:    "Lines \"N<TAB>ITEM<TAB>TEXT\" (the text shortened to one line); with --json, the ChatQueue {paused, items}.",
	Exit:      chatExits(),
	Examples: []Example{
		{Command: "webmux chat queue ls abcd2345", Text: "See what the agent will do next."},
		{Command: "webmux chat queue ls abcd2345 --json | jq -r '.items[].id'", Text: "List queued prompt IDs."},
	},
	SeeAlso: []string{"webmux chat send", "webmux chat status"},
	Bind: func(fs *flag.FlagSet) Action {
		asJSON := fs.Bool("json", false, "print the ChatQueue object")
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
			if state.Queue.Items == nil {
				state.Queue.Items = []contract.ChatQueueItem{}
			}
			if *asJSON {
				writeJSON(e.Stdout, state.Queue)
				return contract.ExitOK
			}
			if state.Queue.Paused {
				_, _ = fmt.Fprintln(e.Stderr, "webmux: the queue is paused; webmux chat queue resume sends it again")
			}
			for i, item := range state.Queue.Items {
				_, _ = fmt.Fprintf(e.Stdout, "%d\t%s\t%s\n", i+1, item.ID, oneLine(item.Text, 80))
			}
			return contract.ExitOK
		}
	},
}

// queueCommand builds a queue write that takes SESSION and, when item is
// set, ITEM.
func queueCommand(name, summary, method, path string, item bool, examples ...Example) *Command {
	args, arguments := "SESSION", []Item{chatSessionArgument}
	if item {
		args, arguments = "SESSION ITEM", append(arguments, queueItemArgument)
	}
	return &Command{
		Name: "chat queue " + name, Group: GroupChat, Args: args,
		Summary: summary, Arguments: arguments, Env: []Item{sessionIDEnv},
		Exit:     chatExits(ExitItem{Code: 1, Text: "Also: the item was sent or removed already."}),
		Examples: examples,
		SeeAlso:  []string{"webmux chat queue ls"},
		Bind: func(fs *flag.FlagSet) Action {
			return func(ctx context.Context, e Env, a []string) int {
				want := 1
				if item {
					want = 2
				}
				if len(a) != want {
					return e.Usagef("usage: webmux chat queue %s %s", name, args)
				}
				p := path
				if item {
					p = strings.Replace(p, "{qid}", url.PathEscape(a[1]), 1)
				}
				return e.chatCommand(ctx, a, method, p, contract.ChatQueueRequest{ClientRequestID: newRequestID()})
			}
		},
	}
}

var (
	chatQueueRemoveCommand = queueCommand("rm", "Remove a queued prompt.", http.MethodDelete, "queue/{qid}", true,
		Example{Command: "webmux chat queue rm abcd2345 q3", Text: "Drop one queued prompt."},
		Example{Command: "webmux chat queue ls abcd2345 && webmux chat queue rm abcd2345 q1", Text: "Look up an item, then drop it."})
	chatQueueSendNowCommand = queueCommand("send-now", "Send a queued prompt now, cancelling the running turn.", http.MethodPost, "queue/{qid}/send-now", true,
		Example{Command: "webmux chat queue send-now abcd2345 q3", Text: "Interrupt the agent with a queued prompt."},
		Example{Command: "webmux chat queue send-now self q1", Text: "Promote the next prompt of the current session."})
	chatQueueClearCommand = queueCommand("clear", "Remove every queued prompt.", http.MethodPost, "queue/clear", false,
		Example{Command: "webmux chat queue clear abcd2345", Text: "Empty the queue on every device."},
		Example{Command: "webmux chat queue clear abcd2345 && webmux chat cancel abcd2345", Text: "Stop all pending and current work."})
	chatQueueResumeCommand = queueCommand("resume", "Resume a paused queue, sending the next prompt when the agent is idle.", http.MethodPost, "queue/resume", false,
		Example{Command: "webmux chat queue resume abcd2345", Text: "Continue after a failed send paused the queue."},
		Example{Command: "webmux chat queue ls abcd2345 && webmux chat queue resume abcd2345", Text: "Check the queue, then send it again."})
)

var chatQueueEditCommand = &Command{
	Name: "chat queue edit", Group: GroupChat, Args: "SESSION ITEM [TEXT|-]",
	Summary:   "Replace a queued prompt's text, or move it up one place with --up.",
	Arguments: []Item{chatSessionArgument, queueItemArgument, {Name: "TEXT", Text: "The new text; - reads standard input."}},
	Env:       []Item{sessionIDEnv},
	Exit:      chatExits(ExitItem{Code: 1, Text: "Also: the item was sent or removed already."}),
	Examples: []Example{
		{Command: `webmux chat queue edit abcd2345 q3 "run the tests first"`, Text: "Reword a queued prompt."},
		{Command: "webmux chat queue edit abcd2345 q3 --up", Text: "Send a prompt one place earlier."},
	},
	SeeAlso: []string{"webmux chat queue ls"},
	Bind: func(fs *flag.FlagSet) Action {
		up := fs.Bool("up", false, "move the item up one place")
		return func(ctx context.Context, e Env, args []string) int {
			if len(args) < 2 || len(args) > 3 || len(args) == 2 && !*up {
				return e.Usagef("a session, an item and new TEXT (or --up) are required")
			}
			request := contract.ChatQueueRequest{ClientRequestID: newRequestID()}
			if *up {
				request.Move = "up"
			}
			if len(args) == 3 {
				text := args[2]
				if text == "-" {
					read, err := e.readStdin()
					if err != nil {
						return e.Fail(err)
					}
					text = read
				}
				request.Text = &text
			}
			return e.chatCommand(ctx, args, http.MethodPatch, "queue/"+url.PathEscape(args[1]), request)
		}
	},
}
