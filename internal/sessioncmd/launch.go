package sessioncmd

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"maps"
	"net/http"
	"net/url"
	"os"
	"os/signal"
	"path/filepath"
	"slices"
	"strings"
	"syscall"
	"time"

	"github.com/rainchan/webmux/internal/attach"
	"github.com/rainchan/webmux/internal/contract"
	"github.com/rainchan/webmux/internal/localclient"
)

// labelFlag collects repeated --label KEY=VALUE.
type labelFlag map[string]string

func (f labelFlag) String() string { return "" }
func (f labelFlag) Set(value string) error {
	key, v, ok := strings.Cut(value, "=")
	if !ok || key == "" {
		return fmt.Errorf("--label expects KEY=VALUE, got %q", value)
	}
	f[key] = v
	return nil
}

// runOptions are the flags of webmux run.
type runOptions struct {
	launchConfig    string
	launchConfigSet bool
	name, cwd       string
	env             envFlag
	labels          labelFlag
	// Location.
	worktree, branch, base, worktreePath, repo      string
	newWorktree, existingBranch, noPin, refreshBase bool
	// View and first message.
	view, prompt, permission string
	init                     bool
	set                      labelFlag
	attachments              listFlag
	// How to follow the session.
	detach, attachMode, follow, strip, killOnInterrupt bool
	asJSON, dryRun, reply, autoWaitRequests            bool
	wait                                               string
	timeout                                            time.Duration
}

// runCommand creates a session from a saved launch config or an explicit
// command.
//
// A custom command inherits the caller's environment (Env.Environ) with
// --env on top, so it runs as it would in the calling shell. A launch config
// is named as in the Web UI (names are unique) and resolved to its ID through
// the summary list, which carries environment names only. The server then
// expands the ID, so the command and environment stay identical to the saved
// copy and secret values never reach the client.
var runCommand = &Command{
	Name: "run", Group: GroupLaunch, Args: "[-- COMMAND [ARG]...]", Strict: true, DashArgs: true,
	StrayHint: func(fs *flag.FlagSet) string {
		if f := fs.Lookup("new-worktree"); f != nil && f.Value.String() == "true" {
			return "--new-worktree takes no value; name the branch with --branch"
		}
		return "the command goes after --"
	},
	Summary: "Create a session in a directory or worktree, on its terminal or chat view; attach, detach, follow or wait.",
	Description: "Without --launch-config, the arguments after -- are the session's argv exactly as given; nothing is expanded by a shell. " +
		"A custom command inherits the caller's environment, minus terminal identity and WEBMUX_* variables, with --env on top. " +
		"With --launch-config, the server starts the saved Web UI launch config of that exact name, " +
		"including its environment, which never reaches the client.\n\n" +
		"Location matches the Web UI's new session dialog: the current directory (associated with the registered worktree it lies in), " +
		"an existing worktree (--worktree), or a new one (--new-worktree, on a branch named by --branch or generated), which is " +
		"pinned to the home page like the Web UI does. A launch config with the chat view can start on it (--view chat) with " +
		"agent settings, a permission mode and attachments; --prompt is the first message on either view. Everything is " +
		"created by one request; a failure names the step that failed and what was already created, and nothing is rolled back.\n\n" +
		"Without a terminal (an agent's tool call, a pipe) run detaches instead of failing; --attach insists on attaching. " +
		"--follow streams the output to standard output without a terminal and exits with the program's exit status; " +
		"Ctrl+C only stops following unless --kill-on-interrupt. --reply follows the first prompt's turn like webmux send --reply.",
	Arguments: []Item{{Name: "COMMAND [ARG]...", Text: "The program and its arguments, after --."}},
	Env: []Item{
		{Name: "WEBMUX_SOCKET", Text: "Local API socket used when neither --socket nor --config is given."},
	},
	Output: "Detached: one line with the session ID, a tab and the local link, or with --json an object {session, link, " +
		"configWarnings} plus worktree when one was created. Attached or --follow: the session's output; notices and config " +
		"warnings go to standard error. --reply: the agent's final reply. --dry-run prints the request with environment values hidden.",
	Exit: []ExitItem{
		{Code: 0, Text: "Created (detached), detached after attaching, --reply and the turn ended normally, or --follow and the program exited 0."},
		{Code: 1, Text: "A step failed (the worktree, the session, its settings or the first prompt), an upload failed, " +
			"following was lost, or with --reply the turn did not end with end_turn or waits for an answer."},
		{Code: 2, Text: "Usage error, unknown launch config or repository, a launch config without the chat view for --view chat, or --attach without a terminal."},
		{Code: 3, Text: "WebMux is not running."},
		{Code: 124, Text: "--wait or --reply timed out."},
		{Code: 130, Text: "--follow stopped by SIGINT; the session keeps running (143 for SIGTERM). Otherwise --follow exits with the program's own status, 128+N when signal N ended it."},
	},
	Examples: []Example{
		{Command: "webmux run -- claude", Text: "Start Claude Code in the current directory and attach; Ctrl+] detaches."},
		{Command: "webmux run --detach --name tests --cwd ~/src/app -- make test", Text: "Start a named session in another directory and print its ID and link."},
		{Command: "webmux run --follow --strip -- go test ./...", Text: "Run tests in a persistent session, stream plain text and exit with their status."},
		{Command: `webmux run --new-worktree --branch fix/lint --launch-config Codex --view chat --set model=gpt-5 --prompt "Fix all lint and summarize" --reply`, Text: "Start an agent on a new worktree's chat view and print its reply."},
		{Command: "webmux run --dry-run --label run=7 -- make build", Text: "Show the request run would send, without creating anything."},
	},
	SeeAlso: []string{"webmux attach", "webmux send", "webmux launch-config options", "webmux guide launch"},
	Notes: map[string]Note{
		"launch-config":      {Conflicts: []string{"env"}},
		"config-id":          {Conflicts: []string{"env"}},
		"env":                {Repeatable: true, Conflicts: []string{"launch-config"}},
		"label":              {Repeatable: true},
		"worktree":           {Conflicts: []string{"new-worktree"}},
		"new-worktree":       {Conflicts: []string{"worktree", "cwd"}},
		"branch":             {Requires: []string{"new-worktree"}},
		"base":               {Requires: []string{"new-worktree"}},
		"existing-branch":    {Requires: []string{"new-worktree", "branch"}},
		"worktree-path":      {Requires: []string{"new-worktree"}},
		"repo":               {Requires: []string{"new-worktree"}},
		"no-pin":             {Requires: []string{"new-worktree"}},
		"refresh-base":       {Requires: []string{"new-worktree"}},
		"view":               {Requires: []string{"launch-config"}},
		"init":               {Requires: []string{"launch-config"}},
		"set":                {Repeatable: true, Requires: []string{"view"}},
		"permission":         {Requires: []string{"view"}},
		"attach-file":        {Repeatable: true, Requires: []string{"view", "prompt"}},
		"reply":              {Requires: []string{"view", "prompt"}, Conflicts: []string{"wait", "follow"}},
		"auto-wait-requests": {Requires: []string{"reply"}},
		"follow":             {Conflicts: []string{"detach", "attach", "reply"}},
		"detach":             {Conflicts: []string{"attach", "follow"}},
		"attach":             {Conflicts: []string{"detach", "follow"}},
		"wait":               {Conflicts: []string{"attach", "follow", "reply"}},
	},
	Bind: func(fs *flag.FlagSet) Action {
		o := runOptions{env: envFlag{}, labels: labelFlag{}, set: labelFlag{}}
		setLaunchConfig := func(value string) error {
			if o.launchConfigSet && o.launchConfig != value {
				return errors.New("--launch-config and --config-id must name the same saved config")
			}
			o.launchConfig, o.launchConfigSet = value, true
			return nil
		}
		fs.Func("launch-config", "use a saved Web UI launch config by `NAME` (instead of -- COMMAND)", setLaunchConfig)
		fs.Func("config-id", "alias for --launch-config", setLaunchConfig)
		fs.StringVar(&o.name, "name", "", "session `NAME`")
		fs.StringVar(&o.cwd, "cwd", "", "working `DIR` on the server (default: the current directory)")
		fs.Var(o.env, "env", "set `NAME=VALUE` in the session, overriding the inherited environment")
		fs.Var(o.labels, "label", "set label `KEY=VALUE` on the session")
		fs.StringVar(&o.worktree, "worktree", "", "start in a registered worktree, as `REPO:WORKTREE` or its path")
		fs.BoolVar(&o.newWorktree, "new-worktree", false, "create a worktree and start at its root")
		fs.StringVar(&o.branch, "branch", "", "the new worktree's branch `NAME` (default: generated, wt/<date>-<time>)")
		fs.StringVar(&o.base, "base", "", "start the new branch at `REF` (default: the repository's default base)")
		fs.BoolVar(&o.existingBranch, "existing-branch", false, "check out the existing --branch instead of creating it")
		fs.StringVar(&o.worktreePath, "worktree-path", "", "put the new worktree at `DIR` (default: <repository>.worktrees/<branch leaf>)")
		fs.StringVar(&o.repo, "repo", "", "repository of the new worktree: an ID, a path or . (default: the current directory's)")
		fs.BoolVar(&o.noPin, "no-pin", false, "do not show the new worktree on the home page")
		fs.BoolVar(&o.refreshBase, "refresh-base", false, "fetch the base branch's upstream first and start from it")
		fs.StringVar(&o.view, "view", "", "start on `VIEW`: terminal (the default) or chat, which needs a launch config with the chat view and implies --detach")
		fs.StringVar(&o.prompt, "prompt", "", "first message `TEXT` (- reads standard input): the first chat prompt, or typed with Enter on the terminal view")
		fs.BoolVar(&o.init, "init", false, "put the repository's initialization instruction before the first message")
		fs.Var(o.set, "set", "chat view: set agent option `ID=VALUE` (see webmux launch-config options)")
		fs.StringVar(&o.permission, "permission", "", "chat view: permission `MODE` ask or auto")
		fs.Var(&o.attachments, "attach-file", "chat view: upload the file at `PATH` and send it with the first prompt")
		fs.BoolVar(&o.detach, "detach", false, "create the session and print its ID and link without attaching")
		fs.BoolVar(&o.attachMode, "attach", false, "attach even when that needs a terminal this process lacks (exit 2)")
		fs.BoolVar(&o.follow, "follow", false, "stream the output to standard output and exit with the program's status")
		fs.BoolVar(&o.strip, "strip", false, "with --follow, remove terminal control sequences")
		fs.BoolVar(&o.killOnInterrupt, "kill-on-interrupt", false, "with --follow, end the program on SIGINT or SIGTERM")
		fs.BoolVar(&o.asJSON, "json", false, "print {session, link, ...} as JSON when detached, or the turn with --reply")
		fs.BoolVar(&o.dryRun, "dry-run", false, "print the request without creating anything; environment values are hidden")
		fs.StringVar(&o.wait, "wait", "", "detach, then wait until the session is first in `STATE` (see webmux wait)")
		fs.BoolVar(&o.reply, "reply", false, "chat view: follow the first prompt's turn and print the agent's final reply")
		fs.BoolVar(&o.autoWaitRequests, "auto-wait-requests", false, "with --reply, keep waiting while a permission request or question is open")
		fs.DurationVar(&o.timeout, "timeout", 0, "give up --wait or --reply after this `DURATION` (exit 124); 0 waits forever")
		return func(ctx context.Context, e Env, argv []string) int { return e.run(ctx, o, argv) }
	},
}

// Run implements `webmux run`.
func Run(ctx context.Context, e Env, args []string) int { return e.Run(ctx, runCommand, args) }

// validate checks the flag combinations that need no server.
func (o *runOptions) validate(argv []string) error {
	chat := o.view == string(contract.ViewChat)
	switch {
	case o.launchConfigSet && o.launchConfig == "":
		return errors.New("--launch-config requires a non-empty saved config name")
	case o.launchConfigSet && len(argv) > 0:
		return errors.New("--launch-config cannot be combined with a command after --")
	case o.launchConfigSet && len(o.env) > 0:
		return errors.New("--launch-config cannot be combined with --env")
	case !o.launchConfigSet && (len(argv) == 0 || argv[0] == ""):
		return errors.New("a command is required after --")
	case o.worktree != "" && o.newWorktree:
		return errors.New("--worktree and --new-worktree cannot be combined")
	case !o.newWorktree && (o.branch != "" || o.base != "" || o.existingBranch || o.worktreePath != "" || o.repo != "" || o.noPin || o.refreshBase):
		return errors.New("--branch, --base, --existing-branch, --worktree-path, --repo, --no-pin and --refresh-base need --new-worktree")
	case o.existingBranch && o.branch == "":
		return errors.New("--existing-branch needs --branch")
	case o.newWorktree && o.cwd != "":
		return errors.New("--cwd cannot be combined with --new-worktree: the session starts at the new worktree's root (pick the repository with --repo)")
	case o.view != "" && o.view != string(contract.ViewTerminal) && !chat:
		return fmt.Errorf("--view is terminal or chat, not %q", o.view)
	case (chat || o.init) && !o.launchConfigSet:
		return errors.New("--view chat and --init need --launch-config")
	case !chat && (len(o.set) > 0 || o.permission != "" || len(o.attachments) > 0 || o.reply):
		return errors.New("--set, --permission, --attach-file and --reply need --view chat")
	case o.permission != "" && o.permission != string(contract.ChatPermissionAsk) && o.permission != string(contract.ChatPermissionAuto):
		return fmt.Errorf("--permission is ask or auto, not %q", o.permission)
	case len(o.attachments) > 0 && o.prompt == "":
		return errors.New("--attach-file needs --prompt: attachments go with the first prompt")
	case o.reply && o.prompt == "":
		return errors.New("--reply needs --prompt: it prints the reply to the first prompt")
	case chat && o.attachMode:
		return errors.New("--view chat cannot be attached; it starts detached")
	case countTrue(o.detach, o.attachMode, o.follow) > 1:
		return errors.New("--detach, --attach and --follow cannot be combined")
	case o.follow && (chat || o.reply):
		return errors.New("--follow cannot be combined with --view chat or --reply")
	case (o.strip || o.killOnInterrupt) && !o.follow:
		return errors.New("--strip and --kill-on-interrupt need --follow")
	case o.wait != "" && o.reply:
		return errors.New("--wait and --reply cannot be combined")
	case o.wait != "" && (o.attachMode || o.follow):
		return errors.New("--wait works only when the session is detached")
	case o.autoWaitRequests && !o.reply:
		return errors.New("--auto-wait-requests needs --reply")
	case o.timeout != 0 && o.wait == "" && !o.reply:
		return errors.New("--timeout needs --wait or --reply")
	}
	if o.wait != "" {
		if _, err := parseStates(o.wait); err != nil {
			return err
		}
	}
	return nil
}

func countTrue(values ...bool) int {
	n := 0
	for _, v := range values {
		if v {
			n++
		}
	}
	return n
}

func (e Env) run(ctx context.Context, o runOptions, argv []string) int {
	if err := o.validate(argv); err != nil {
		return e.Usagef("%v", err)
	}
	// Waiting needs the session detached, and the chat view has no
	// terminal to attach to.
	if o.wait != "" || o.reply || o.view == string(contract.ViewChat) {
		o.detach = true
	}
	if o.prompt == "-" {
		text, err := e.readStdin()
		if err != nil {
			return e.Fail(err)
		}
		o.prompt = text
	}
	client, code := e.connect()
	if client == nil {
		return code
	}
	defer client.Close()
	dir, err := e.ResolveCwd(ctx, o.cwd)
	if err != nil {
		_, _ = fmt.Fprintln(e.Stderr, "webmux run:", err)
		return contract.ExitUsage
	}
	request := contract.CreateSessionRequest{
		Cwd: dir, Name: o.name, Prompt: o.prompt, UseInitInstruction: o.init,
		View: contract.SessionView(o.view), PermissionMode: contract.ChatPermissionMode(o.permission),
	}
	if len(o.labels) > 0 {
		request.Labels = o.labels
	}
	if len(o.set) > 0 {
		request.AgentConfig = o.set
	}
	if o.launchConfigSet {
		summary, err := launchConfigByName(ctx, client, o.launchConfig)
		if err != nil {
			if errors.Is(err, errUnknownLaunchConfig) {
				_, _ = fmt.Fprintln(e.Stderr, "webmux run:", err)
				return contract.ExitUsage
			}
			return e.Fail(err)
		}
		if request.View == contract.ViewChat && !summary.Chat {
			return e.Usagef("--view chat needs a launch config with the chat view; %q has none", summary.Name)
		}
		request.ConfigID = &summary.ID
	} else {
		request.Cmd = argv
		request.Env = e.sessionEnv(o.env)
	}
	// Location: an existing worktree, the worktree the directory lies in,
	// or a new one the server creates first.
	var repos []contract.Repository
	var newRepo contract.Repository
	repos, err = repositories(ctx, client)
	if err != nil && (o.worktree != "" || o.newWorktree) {
		return e.Fail(err)
	}
	switch {
	case o.worktree != "":
		repo, tree, err := e.resolveWorktree(ctx, repos, o.worktree)
		if err != nil {
			return e.locationError(err)
		}
		if o.cwd == "" {
			request.Cwd = tree.Path
		} else if !within(tree.Path, mustLocal(ctx, e, o.cwd)) {
			return e.Usagef("--cwd %s is not inside worktree %s", o.cwd, tree.Path)
		}
		request.Workspace = &contract.WorkspaceReference{RepositoryID: repo.ID, WorktreeID: tree.ID}
	case o.newWorktree:
		newRepo, err = e.resolveRepository(ctx, repos, o.repo)
		if err != nil {
			return e.locationError(err)
		}
		request.Cwd = ""
		request.NewWorktree = &contract.NewWorktree{RepositoryID: newRepo.ID, Mode: "new", Branch: o.branch, Base: o.base}
		if o.existingBranch {
			request.NewWorktree.Mode = "existing"
		}
		if o.worktreePath != "" {
			request.NewWorktree.Path = e.worktreeDir(ctx, newRepo, o.branch, o.worktreePath)
		}
	default:
		// The server associates the session with this worktree anyway;
		// naming it lets --init find the repository's instruction.
		if local, err := e.localPath(ctx, o.cwd); err == nil {
			if repo, tree, ok := findWorktree(repos, local); ok {
				request.Workspace = &contract.WorkspaceReference{RepositoryID: repo.ID, WorktreeID: tree.ID}
			}
		}
	}
	if request.UseInitInstruction && request.Workspace == nil && request.NewWorktree == nil {
		return e.Usagef("--init needs a registered worktree: run inside one, or use --worktree or --new-worktree")
	}
	if o.dryRun {
		return e.printDryRun(request, o)
	}
	// An attached run needs a terminal: check before creating anything.
	var console Console
	if !o.detach && !o.follow {
		c, err := e.OpenConsole()
		switch {
		case errors.Is(err, attach.ErrNotTerminal) && !o.attachMode:
			_, _ = fmt.Fprintln(e.Stderr, "webmux run: no terminal; detaching (use --attach to insist)")
			o.detach = true
		case errors.Is(err, attach.ErrNotTerminal):
			_, _ = fmt.Fprintln(e.Stderr, "webmux:", err)
			return contract.ExitUsage
		case err != nil:
			return e.Fail(err)
		default:
			console = c
		}
	}
	restore := func() {
		if console != nil {
			_ = console.Restore()
		}
	}
	if o.refreshBase && !o.existingBranch {
		base, err := e.refreshBase(ctx, client, newRepo, o.base)
		if err != nil {
			restore()
			return e.Fail(err)
		}
		request.NewWorktree.Base = base
	}
	for _, file := range o.attachments {
		path, err := e.uploadFile(ctx, client, "/api/uploads", file)
		if err != nil {
			restore()
			return e.Fail(fmt.Errorf("upload %s: %w; nothing was created", file, err))
		}
		request.Attachments = append(request.Attachments, contract.ChatAttachment{Path: path})
	}
	request.ClientRequestID = newRequestID()
	var result contract.CreateSessionResult
	if err := client.Do(ctx, http.MethodPost, "/api/sessions", request, &result); err != nil {
		restore()
		return e.createFailure(err)
	}
	// An attached terminal is raw already: lines end with CR LF.
	eol := "\n"
	if console != nil {
		eol = "\r\n"
	}
	for _, warning := range result.ConfigWarnings {
		_, _ = fmt.Fprintf(e.Stderr, "webmux run: %s%s", warning, eol)
	}
	if result.Worktree != nil && !o.noPin {
		repo := newRepo
		repo.Worktrees = append(slices.Clone(newRepo.Worktrees), *result.Worktree)
		if err := pinWorktrees(ctx, client, repo, []string{result.Worktree.ID}, true); err != nil {
			_, _ = fmt.Fprintf(e.Stderr, "webmux: the worktree was created but could not be shown on the home page: %v%s", err, eol)
		}
	}
	created := result.Session
	switch {
	case o.follow:
		return e.followRun(ctx, client, created.ID, o)
	case o.reply:
		return e.replyRun(ctx, client, result, o)
	case o.detach:
		extra := map[string]any{"configWarnings": nonNil(result.ConfigWarnings)}
		if result.Worktree != nil {
			extra["worktree"] = result.Worktree
		}
		return e.detached(ctx, client, created, o, extra)
	}
	// Tell the user where to find the session (for example on a phone)
	// before the session's output starts; stdout stays the session's. The
	// terminal is already raw, so the line ends with CR LF.
	if link, err := link(ctx, client, created.ID); err == nil {
		_, _ = fmt.Fprintf(e.Stderr, "webmux: session %s · %s · Ctrl+] to detach\r\n", created.ID, link)
	} else {
		_, _ = fmt.Fprintf(e.Stderr, "webmux: session %s · Ctrl+] to detach\r\n", created.ID)
	}
	return e.attach(ctx, client, created.ID, false, console)
}

// createFailure reports a failed POST /api/sessions by its step, with what
// had been created; nothing is rolled back.
func (e Env) createFailure(err error) int {
	var apiErr *localclient.APIError
	var failure contract.CreateSessionFailure
	if !errors.As(err, &apiErr) || json.Unmarshal(apiErr.Body, &failure) != nil || failure.Step == "" {
		return e.sessionFail(err)
	}
	reason := ""
	if failure.Reason != "" {
		reason = " (" + failure.Reason + ")"
	}
	_, _ = fmt.Fprintf(e.Stderr, "webmux run: the %s step failed: %s%s\n", failure.Step, failure.Error, reason)
	if failure.Connector != nil {
		_, _ = fmt.Fprintf(e.Stderr, "webmux run: install %s: %s\n", failure.Connector.Title, failure.Connector.InstallCommand)
	}
	if failure.Worktree != nil {
		_, _ = fmt.Fprintf(e.Stderr, "webmux run: worktree %s was created and is kept\n", failure.Worktree.Path)
	}
	if failure.SessionID != "" {
		_, _ = fmt.Fprintf(e.Stderr, "webmux run: session %s was started\n", failure.SessionID)
	}
	return contract.ExitError
}

// replyRun follows the first prompt's turn of a session created on its
// chat view and prints the reply.
func (e Env) replyRun(ctx context.Context, client *localclient.Client, result contract.CreateSessionResult, o runOptions) int {
	id := result.Session.ID
	link, _ := link(ctx, client, id)
	where := ""
	if result.Worktree != nil {
		where = " · " + result.Worktree.Path
	}
	_, _ = fmt.Fprintf(e.Stderr, "webmux: session %s%s · %s\n", id, where, link)
	if o.timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, o.timeout)
		defer cancel()
	}
	stream, err := openChat(ctx, client, id)
	if err != nil {
		return e.Fail(err)
	}
	defer stream.close()
	// The first prompt's turn is the first turn of the session.
	turn, err := stream.turnWait(ctx, turnWatch{accept: func(contract.ChatTurnStart) bool { return true }, stopAtRequest: !o.autoWaitRequests})
	if err != nil {
		return e.waitFailure(err)
	}
	return e.reportTurn(turn, o.asJSON)
}

func mustLocal(ctx context.Context, e Env, dir string) string {
	local, err := e.localPath(ctx, dir)
	if err != nil {
		return dir
	}
	return local
}

// locationError reports a repository or worktree argument that names
// nothing as a usage error and anything else as a failure.
func (e Env) locationError(err error) int {
	if errors.Is(err, errRepoArgument) {
		return e.Usagef("%v", err)
	}
	return e.Fail(err)
}

func (e Env) readStdin() (string, error) {
	if e.Stdin == nil {
		return "", errors.New("standard input is not available")
	}
	data, err := io.ReadAll(e.Stdin)
	return strings.TrimRight(string(data), "\n"), err
}

// printDryRun writes the request run would send. Environment values are
// replaced: inherited ones by "<inherited>", explicit ones by "<explicit>".
// Attachments are the local files that would be uploaded first.
func (e Env) printDryRun(request contract.CreateSessionRequest, o runOptions) int {
	if request.Env != nil {
		masked := make(map[string]string, len(request.Env))
		for name := range request.Env {
			masked[name] = "<inherited>"
			if _, ok := o.env[name]; ok {
				masked[name] = "<explicit>"
			}
		}
		request.Env = masked
	}
	for _, file := range o.attachments {
		request.Attachments = append(request.Attachments, contract.ChatAttachment{Path: file})
	}
	out := map[string]any{"request": request}
	if request.NewWorktree != nil {
		out["pin"] = !o.noPin
	}
	writeJSON(e.Stdout, out)
	return contract.ExitOK
}

// worktreeDir is where a new worktree goes: dir resolved like other local
// paths (relative or ~), or the quick start default when dir is empty.
func (e Env) worktreeDir(ctx context.Context, repo contract.Repository, branch, dir string) string {
	if dir == "" {
		return defaultWorktreePath(repo, branch)
	}
	if local, err := e.localPath(ctx, dir); err == nil {
		return local
	}
	return dir
}

// refreshBase fetches the upstream of base (default: the repository's
// default branch) and returns the commit a new branch starts from; a
// failed fetch is a warning and the local branch is used.
func (e Env) refreshBase(ctx context.Context, client *localclient.Client, repo contract.Repository, base string) (string, error) {
	repoPath := "/api/repositories/" + url.PathEscape(repo.ID)
	ref := base
	if ref == "" {
		var branches contract.Branches
		if err := client.Do(ctx, http.MethodGet, repoPath+"/branches", nil, &branches); err != nil {
			return "", err
		}
		if branches.DefaultBranch != nil {
			ref = *branches.DefaultBranch
		}
	}
	var refreshed contract.RefreshBranchBaseResponse
	if err := client.Do(ctx, http.MethodPost, repoPath+"/branches/refresh", contract.RefreshBranchBaseRequest{Branch: ref}, &refreshed); err != nil {
		return "", err
	}
	if refreshed.FetchWarning != "" {
		_, _ = fmt.Fprintln(e.Stderr, "webmux: fetching the base failed; using the local branch:", refreshed.FetchWarning)
	}
	return refreshed.Base, nil
}

// createWorktree creates a worktree like the Web UI's dialog and pins it
// to the home page unless pin is false; a failed pin is only a warning.
func (e Env) createWorktree(ctx context.Context, client *localclient.Client, repo contract.Repository, branch, base, dir string, existing, refresh, pin bool) (contract.Worktree, error) {
	dir = e.worktreeDir(ctx, repo, branch, dir)
	repoPath := "/api/repositories/" + url.PathEscape(repo.ID)
	if refresh && !existing {
		refreshed, err := e.refreshBase(ctx, client, repo, base)
		if err != nil {
			return contract.Worktree{}, err
		}
		base = refreshed
	}
	mode := "new"
	if existing {
		mode = "existing"
	}
	if base == "" && !existing {
		var branches contract.Branches
		if err := client.Do(ctx, http.MethodGet, repoPath+"/branches", nil, &branches); err != nil {
			return contract.Worktree{}, err
		}
		if branches.DefaultBase != nil {
			base = *branches.DefaultBase
		}
	}
	var tree contract.Worktree
	request := contract.CreateWorktreeRequest{Path: dir, Mode: mode, Branch: branch, Base: base}
	if err := client.Do(ctx, http.MethodPost, repoPath+"/worktrees", request, &tree); err != nil {
		return contract.Worktree{}, fmt.Errorf("create worktree: %w", err)
	}
	if pin {
		refreshed := repo
		refreshed.Worktrees = append(slices.Clone(repo.Worktrees), tree)
		if err := pinWorktrees(ctx, client, refreshed, []string{tree.ID}, true); err != nil {
			_, _ = fmt.Fprintln(e.Stderr, "webmux: the worktree was created but could not be shown on the home page:", err)
		}
	}
	return tree, nil
}

// detached prints a created or resumed session and waits for --wait.
// extra adds fields to the --json object.
func (e Env) detached(ctx context.Context, client *localclient.Client, created contract.SessionInfo, o runOptions, extra map[string]any) int {
	session := created
	if o.wait != "" {
		states, _ := parseStates(o.wait)
		waitCtx := ctx
		if o.timeout > 0 {
			var cancel context.CancelFunc
			waitCtx, cancel = context.WithTimeout(ctx, o.timeout)
			defer cancel()
		}
		watch, err := watchSessions(waitCtx, client)
		if err != nil {
			return e.Fail(err)
		}
		matched, err := watch.waitFor(waitCtx, waitOptions{ids: []string{created.ID}, states: states})
		watch.close()
		if err != nil {
			return e.waitFailure(err)
		}
		session = matched[0]
	}
	link, err := link(ctx, client, created.ID)
	if err != nil {
		return e.Fail(err)
	}
	if o.asJSON {
		out := map[string]any{"session": session, "link": link}
		maps.Copy(out, extra)
		writeJSON(e.Stdout, out)
	} else {
		_, _ = fmt.Fprintf(e.Stdout, "%s\t%s\n", created.ID, link)
	}
	return contract.ExitOK
}

// waitFailure maps an ended wait: a timeout is 124.
func (e Env) waitFailure(err error) int {
	if errors.Is(err, errTimeout) {
		_, _ = fmt.Fprintln(e.Stderr, "webmux: timed out waiting")
		return contract.ExitTimeout
	}
	return e.Fail(err)
}

// followRun streams a created session until it exits. SIGINT and SIGTERM
// stop following (130, 143) and leave the program running, or end it first
// with --kill-on-interrupt.
func (e Env) followRun(ctx context.Context, client *localclient.Client, id string, o runOptions) int {
	if link, err := link(ctx, client, id); err == nil {
		_, _ = fmt.Fprintf(e.Stderr, "webmux: session %s · %s\n", id, link)
	}
	signals := make(chan os.Signal, 1)
	signal.Notify(signals, os.Interrupt, syscall.SIGTERM)
	defer signal.Stop(signals)
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	// The goroutine hands over the signal that ended following. The
	// caller's context may be cancelled by the same signal first, so after a
	// cancellation the signal is awaited briefly.
	gotSignal := make(chan os.Signal, 1)
	done := make(chan struct{})
	defer close(done)
	go func() {
		select {
		case s := <-signals:
			cancel()
			gotSignal <- s
		case <-done:
		}
	}()
	out := e.Stdout
	var strip *stripper
	if o.strip {
		strip = newStripper(e.Stdout)
		out = strip
	}
	result, err := followTerminal(ctx, client, id, out, e.scriptLabel())
	if strip != nil {
		_ = strip.Flush()
	}
	var got os.Signal
	if ctx.Err() != nil {
		select {
		case got = <-gotSignal:
		case <-time.After(200 * time.Millisecond):
		}
	}
	if err != nil && ctx.Err() != nil {
		code := 130
		if got == syscall.SIGTERM {
			code = 143
		}
		if !o.killOnInterrupt {
			_, _ = fmt.Fprintf(e.Stderr, "\nwebmux: stopped following; session %s keeps running (webmux attach %s)\n", id, id)
			return code
		}
		return e.killAndWait(client, id, code)
	}
	if err != nil {
		_, _ = fmt.Fprintf(e.Stderr, "webmux: following session %s was interrupted: %v; it may still be running (webmux attach %s)\n", id, err, id)
		return contract.ExitError
	}
	if result.Moved {
		_, _ = fmt.Fprintf(e.Stderr, "webmux: session %s moved to its chat view and keeps running; follow it with webmux chat watch %s\n", id, id)
		return contract.ExitError
	}
	if result.Exited {
		return result.ExitCode
	}
	return contract.ExitOK
}

// killAndWait ends session id and returns its exit status, or fallback when
// it cannot be read within ten seconds.
func (e Env) killAndWait(client *localclient.Client, id string, fallback int) int {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	watch, err := watchSessions(ctx, client)
	if err != nil {
		return fallback
	}
	defer watch.close()
	if err := client.Do(ctx, http.MethodDelete, "/api/sessions/"+url.PathEscape(id), nil, nil); err != nil {
		_, _ = fmt.Fprintln(e.Stderr, "webmux: could not end the session:", err)
		return fallback
	}
	matched, err := watch.waitFor(ctx, waitOptions{ids: []string{id}, states: []string{"exited"}})
	if err != nil || matched[0].ExitCode == nil {
		return fallback
	}
	return *matched[0].ExitCode
}

// scriptLabel names this client in the control indicator: "script", plus the
// session it runs in.
func (e Env) scriptLabel() string {
	if e.Getenv != nil {
		if id := e.Getenv(contract.SessionIDEnv); id != "" {
			return "script · " + id
		}
	}
	return "script"
}

var errUnknownLaunchConfig = errors.New("no saved launch config named")

// launchConfigByName finds a launch config summary by its exact name.
func launchConfigByName(ctx context.Context, client *localclient.Client, name string) (contract.LaunchConfigSummary, error) {
	var list []contract.LaunchConfigSummary
	if err := client.Do(ctx, http.MethodGet, "/api/launch-configs", nil, &list); err != nil {
		return contract.LaunchConfigSummary{}, err
	}
	names := make([]string, 0, len(list))
	for _, c := range list {
		if c.Name == name {
			return c, nil
		}
		names = append(names, fmt.Sprintf("%q", c.Name))
	}
	if len(names) == 0 {
		return contract.LaunchConfigSummary{}, fmt.Errorf("%w %q (no saved launch configs)", errUnknownLaunchConfig, name)
	}
	return contract.LaunchConfigSummary{}, fmt.Errorf("%w %q (saved: %s)", errUnknownLaunchConfig, name, strings.Join(names, ", "))
}

var chatGroup = &Command{
	Name: "chat", Group: GroupChat,
	Summary: "Act on what only the chat view has: requests, agent settings, the permission mode and the queue.",
}

var launchConfigGroup = &Command{
	Name: "launch-config", Group: GroupLaunch,
	Summary: "List, show, add, edit and remove the saved Web UI launch configs.",
}

// uploadFile uploads a local file to path and returns its server path.
func (e Env) uploadFile(ctx context.Context, client *localclient.Client, path, file string) (string, error) {
	f, err := os.Open(file)
	if err != nil {
		return "", err
	}
	defer func() { _ = f.Close() }()
	var result contract.UploadResult
	if err := client.Upload(ctx, path, filepath.Base(file), f, &result); err != nil {
		return "", err
	}
	return result.Path, nil
}

var launchConfigOptionsCommand = &Command{
	Name: "launch-config options", Group: GroupLaunch, Args: "NAME",
	Summary: "List the options (model, mode, ...) the agent of a launch config with the chat view offers.",
	Description: "The agent reports its options from a throwaway session in a repository's main worktree " +
		"(POST /api/launch-configs/{id}/options). Use the IDs and values with webmux run --view chat --set ID=VALUE.",
	Arguments: []Item{{Name: "NAME", Text: "The exact name of a launch config with the chat view."}},
	Output:    "One option per line: ID, name, type, current value and choices. With --json, the ChatConfig array.",
	Exit: []ExitItem{
		{Code: 0, Text: "Listed."},
		{Code: 1, Text: "The launch config has no chat view (no_chat_view), or the agent could not be asked."},
		{Code: 2, Text: "Usage error, unknown launch config or repository."},
		{Code: 3, Text: "WebMux is not running."},
	},
	Examples: []Example{
		{Command: `webmux launch-config options "Claude Code"`, Text: "Show the agent's options and their values."},
		{Command: `webmux launch-config options Codex --json | jq '.[] | select(.id=="model") | .options[].value'`, Text: "List the models to pass to run --set model=..."},
	},
	SeeAlso: []string{"webmux run", "webmux chat set"},
	Bind: func(fs *flag.FlagSet) Action {
		repo := fs.String("repo", "", "repository whose main worktree the agent runs in: an ID, a path or . (default: the current directory's)")
		asJSON := fs.Bool("json", false, "print the ChatConfig array as JSON")
		return func(ctx context.Context, e Env, args []string) int {
			if len(args) != 1 {
				return e.Usagef("exactly one launch config name is required")
			}
			client, code := e.connect()
			if client == nil {
				return code
			}
			defer client.Close()
			summary, err := launchConfigByName(ctx, client, args[0])
			if err != nil {
				if errors.Is(err, errUnknownLaunchConfig) {
					return e.Usagef("%v", err)
				}
				return e.Fail(err)
			}
			if !summary.Chat {
				_, _ = fmt.Fprintf(e.Stderr, "webmux: launch config %q has no chat view (no_chat_view)\n", summary.Name)
				return contract.ExitError
			}
			repos, err := repositories(ctx, client)
			if err != nil {
				return e.Fail(err)
			}
			r, err := e.resolveRepository(ctx, repos, *repo)
			if err != nil {
				return e.locationError(err)
			}
			var options []contract.ChatConfig
			req := contract.LaunchOptionsRequest{RepositoryID: r.ID}
			if err := client.Do(ctx, http.MethodPost, "/api/launch-configs/"+url.PathEscape(summary.ID)+"/options", req, &options); err != nil {
				return e.sessionFail(err)
			}
			if *asJSON {
				writeJSON(e.Stdout, options)
				return contract.ExitOK
			}
			for _, option := range options {
				value := option.Value
				if option.Type == "boolean" && option.Enabled != nil {
					value = fmt.Sprint(*option.Enabled)
				}
				choices := make([]string, 0, len(option.Options))
				for _, c := range option.Options {
					choices = append(choices, c.Value)
				}
				line := fmt.Sprintf("%s\t%s\t%s\t%s", option.ID, option.Name, option.Type, value)
				if len(choices) > 0 {
					line += "\t" + strings.Join(choices, ",")
				}
				_, _ = fmt.Fprintln(e.Stdout, line)
			}
			return contract.ExitOK
		}
	},
}

// newRequestID is a clientRequestId for one logical request; retries of
// it reuse the value.
func newRequestID() string {
	var raw [12]byte
	_, _ = rand.Read(raw[:])
	return "cli-" + hex.EncodeToString(raw[:])
}
