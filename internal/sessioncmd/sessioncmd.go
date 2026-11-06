// Package sessioncmd implements the session subcommands shared by the Linux
// CLI and the Windows console client: run, attach, ls and open. Each client
// supplies its local API client, console and working-directory rules.
package sessioncmd

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"maps"
	"net/http"
	"net/url"
	"slices"
	"sort"
	"strings"
	"text/tabwriter"

	"github.com/rainchan/webmux/internal/attach"
	"github.com/rainchan/webmux/internal/contract"
	"github.com/rainchan/webmux/internal/localclient"
)

// Console is a raw terminal that can be restored.
type Console interface {
	attach.Terminal
	Restore() error
}

// Env is what a client provides to the commands.
type Env struct {
	// Flags registers client-specific flags (for example --config) on every
	// subcommand before parsing; Connect runs after parsing.
	Flags   func(*flag.FlagSet)
	Connect func() (*localclient.Client, error)
	// Stdin is read by "-" arguments; nil means none.
	Stdin  io.Reader
	Stdout io.Writer
	Stderr io.Writer
	// OpenConsole enters raw mode; it returns attach.ErrNotTerminal when
	// standard input or output is not a terminal.
	OpenConsole func() (Console, error)
	// Interactive reports whether standard input is a terminal, so a
	// destructive command may ask for confirmation; nil means it is not.
	Interactive func() bool
	// OpenBrowser opens a URL in the default browser.
	OpenBrowser func(url string) error
	// ResolveCwd maps --cwd (or "" for the current directory) to a path
	// on the server.
	ResolveCwd func(ctx context.Context, cwd string) (string, error)
	// Environ returns the caller's environment, which `run` forwards to a
	// custom-command session. Nil forwards nothing: the Windows client must
	// not copy Windows variables into WSL.
	Environ func() []string
	// NotRunningHint tells how to start the server.
	NotRunningHint string
	// Label names this client in the browser's control indicator.
	Label string
	// Getenv reads the caller's environment, for "self"; nil reads nothing.
	Getenv func(string) string
	// Platform is "unix" or "windows"; it hides the other client's commands.
	Platform string
	// Version is this client's version, shown by help --json and compared
	// with the server's when an endpoint is missing.
	Version string
	// HelpFooter follows the command overview of `help`.
	HelpFooter string
	// Commands is the registry Dispatch runs, for help to list siblings.
	Commands Registry
	// current is the command being run, for its usage errors.
	current *Command
}

// Fail prints err and returns its exit code: 3 when the server is not
// running, 1 otherwise.
func (e Env) Fail(err error) int {
	if errors.Is(err, localclient.ErrNotRunning) {
		_, _ = fmt.Fprintln(e.Stderr, "webmux: WebMux is not running;", e.NotRunningHint)
		return contract.ExitNotRunning
	}
	_, _ = fmt.Fprintln(e.Stderr, "webmux:", err)
	return contract.ExitError
}

// UsageError is a Connect failure caused by the command line, such as
// conflicting connection options; it exits 2 like other usage errors.
type UsageError string

func (u UsageError) Error() string { return string(u) }

func (e Env) connect() (*localclient.Client, int) {
	client, err := e.Connect()
	var usage UsageError
	if errors.As(err, &usage) {
		return nil, e.Usagef("%s", string(usage))
	}
	if err != nil {
		return nil, e.Fail(err)
	}
	return client, contract.ExitOK
}

// envFlag collects repeated --env NAME=VALUE.
type envFlag map[string]string

func (f envFlag) String() string { return "" }
func (f envFlag) Set(value string) error {
	name, v, ok := strings.Cut(value, "=")
	if !ok || name == "" {
		return fmt.Errorf("--env expects NAME=VALUE, got %q", value)
	}
	f[name] = v
	return nil
}

// Variables that describe the calling process rather than the session: the
// session's working directory is --cwd, not the caller's.
var callerOnlyEnv = map[string]bool{"PWD": true, "OLDPWD": true, "_": true}

// sessionEnv is the caller's environment, minus variables the server owns,
// caller-only variables and names a create request rejects, with explicit
// --env values on top. Nil means nothing to send.
func (e Env) sessionEnv(explicit envFlag) map[string]string {
	var inherited []string
	if e.Environ != nil {
		inherited = e.Environ()
	}
	env := make(map[string]string, len(inherited)+len(explicit))
	for _, entry := range inherited {
		name, value, ok := strings.Cut(entry, "=")
		if !ok || callerOnlyEnv[name] || contract.ServerOwnedEnv(name) || !contract.ValidEnvName(name) || strings.ContainsRune(value, 0) {
			continue
		}
		env[name] = value
	}
	maps.Copy(env, explicit)
	if len(env) == 0 {
		return nil
	}
	return env
}

var attachCommand = &Command{
	Name: "attach", Group: GroupSessions, Args: "SESSION",
	Summary: "Attach this terminal to a session; Ctrl+] detaches and the session keeps running.",
	Description: "The terminal takes control of the session's input and size unless --observe is given. " +
		"A hybrid session on its chat side cannot be attached.",
	Arguments: []Item{sessionArgument},
	Env:       []Item{sessionIDEnv},
	Output:    "The session's output on this terminal; notices go to standard error.",
	Exit: []ExitItem{
		{Code: 0, Text: "Detached, or the session moved to the chat view."},
		{Code: 1, Text: "The session is on its chat side or the connection failed."},
		{Code: 2, Text: "Usage error or no terminal."},
		{Code: 3, Text: "WebMux is not running."},
	},
	Examples: []Example{
		{Command: "webmux attach abcd2345", Text: "Take over session abcd2345 in this terminal."},
		{Command: "webmux attach --observe abcd2345", Text: "Watch the session without sending input or resizing it."},
	},
	SeeAlso: []string{"webmux run", "webmux ls"},
	Bind: func(flags *flag.FlagSet) Action {
		observe := flags.Bool("observe", false, "watch without taking control, sending input or resizing")
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
			console, code := e.openConsole()
			if console == nil {
				return code
			}
			return e.attach(ctx, client, id, *observe, console)
		}
	},
}

// Attach implements `webmux attach`.
func Attach(ctx context.Context, e Env, args []string) int { return e.Run(ctx, attachCommand, args) }

// openConsole enters raw mode; a missing terminal is a usage error.
func (e Env) openConsole() (Console, int) {
	console, err := e.OpenConsole()
	if errors.Is(err, attach.ErrNotTerminal) {
		_, _ = fmt.Fprintln(e.Stderr, "webmux:", err)
		return nil, contract.ExitUsage
	}
	if err != nil {
		return nil, e.Fail(err)
	}
	return console, contract.ExitOK
}

// attach runs until detach or exit and restores console before returning.
func (e Env) attach(ctx context.Context, client *localclient.Client, id string, observe bool, console Console) int {
	result, err := attach.Run(ctx, client, id, console, attach.Options{Observe: observe, Label: e.Label})
	_ = console.Restore()
	switch {
	case errors.Is(err, attach.ErrMoved):
		_, _ = fmt.Fprintln(e.Stderr, "webmux:", err)
		return contract.ExitError
	case err != nil:
		return e.Fail(err)
	case result.Detached:
		_, _ = fmt.Fprintf(e.Stderr, "\r\n[detached from %s; it keeps running]\n", id)
		return contract.ExitOK
	case result.Moved:
		// Passive detach: the session lives on in the chat view.
		_, _ = fmt.Fprintf(e.Stderr, "\r\n[已切换到聊天视图，可在 Web 端继续；切回终端后用 `webmux attach %s` 重新接入]\n", id)
		return contract.ExitOK
	}
	return result.ExitCode
}

var listCommand = &Command{
	Name: "ls", Group: GroupSessions,
	Summary: "List terminal sessions, optionally filtered by label, parent or status.",
	Description: "Filters combine: a session is listed only when it matches all of them. " +
		"--label KEY=VALUE matches a label value, --label KEY only requires the label to exist.",
	Output: "A table of ID, name, status (with exit code and hybrid side), command and launch directory. " +
		"With --json, the server's SessionInfo array, filtered when filters are given.",
	Exit: []ExitItem{
		{Code: 0, Text: "Listed."},
		{Code: 2, Text: "Usage error."},
		{Code: 3, Text: "WebMux is not running."},
	},
	Examples: []Example{
		{Command: "webmux ls", Text: "Show every session."},
		{Command: "webmux ls --parent self --json", Text: "List the sessions this session created, as JSON."},
		{Command: "webmux ls --label run=42 --status exited", Text: "Show finished sessions of one scripted run."},
	},
	SeeAlso: []string{"webmux get", "webmux label"},
	Notes: map[string]Note{
		"label":  {Repeatable: true},
		"status": {Repeatable: true},
	},
	Bind: func(flags *flag.FlagSet) Action {
		asJSON := flags.Bool("json", false, "print the server's session list as JSON")
		var labels, statuses listFlag
		flags.Var(&labels, "label", "only sessions with label `KEY=VALUE`, or with label KEY")
		parent := flags.String("parent", "", "only sessions created by `SESSION` (or self)")
		flags.Var(&statuses, "status", "only sessions in `STATE`: running, waiting, idle or exited")
		return func(ctx context.Context, e Env, args []string) int {
			if len(args) > 0 {
				return e.Usagef("unexpected argument %q", args[0])
			}
			for _, state := range statuses {
				if !slices.Contains([]string{"running", "waiting", "idle", "exited"}, state) {
					return e.Usagef("unknown status %q", state)
				}
			}
			parentID := ""
			if *parent != "" {
				id, code, ok := e.SessionID(*parent)
				if !ok {
					return code
				}
				parentID = id
			}
			return e.list(ctx, listFilter{labels: labels, parent: parentID, statuses: statuses}, *asJSON)
		}
	},
}

// listFlag collects a repeatable string flag.
type listFlag []string

func (f *listFlag) String() string     { return strings.Join(*f, ",") }
func (f *listFlag) Set(v string) error { *f = append(*f, v); return nil }

type listFilter struct {
	labels   []string
	parent   string
	statuses []string
}

func (f listFilter) empty() bool { return len(f.labels) == 0 && f.parent == "" && len(f.statuses) == 0 }

func (f listFilter) match(s contract.SessionInfo) bool {
	if f.parent != "" && s.ParentID != f.parent {
		return false
	}
	if len(f.statuses) > 0 && !slices.Contains(f.statuses, string(s.Status)) {
		return false
	}
	for _, label := range f.labels {
		key, value, hasValue := strings.Cut(label, "=")
		got, ok := s.Labels[key]
		if !ok || (hasValue && got != value) {
			return false
		}
	}
	return true
}

// List implements `webmux ls`.
func List(ctx context.Context, e Env, args []string) int { return e.Run(ctx, listCommand, args) }

func (e Env) list(ctx context.Context, filter listFilter, asJSON bool) int {
	client, code := e.connect()
	if client == nil {
		return code
	}
	defer client.Close()
	var raw json.RawMessage
	if err := client.Do(ctx, http.MethodGet, "/api/sessions", nil, &raw); err != nil {
		return e.Fail(err)
	}
	if asJSON && filter.empty() {
		_, _ = fmt.Fprintf(e.Stdout, "%s\n", raw)
		return contract.ExitOK
	}
	var list []contract.SessionInfo
	if err := json.Unmarshal(raw, &list); err != nil {
		return e.Fail(err)
	}
	if !filter.empty() {
		// Filtering re-encodes the array, so --json keeps every field the
		// server sent only through the raw objects.
		var objects []json.RawMessage
		if err := json.Unmarshal(raw, &objects); err != nil {
			return e.Fail(err)
		}
		keptRaw := make([]json.RawMessage, 0, len(list))
		kept := make([]contract.SessionInfo, 0, len(list))
		for i, s := range list {
			if filter.match(s) {
				kept = append(kept, s)
				keptRaw = append(keptRaw, objects[i])
			}
		}
		if asJSON {
			data, err := json.Marshal(keptRaw)
			if err != nil {
				return e.Fail(err)
			}
			_, _ = fmt.Fprintf(e.Stdout, "%s\n", data)
			return contract.ExitOK
		}
		list = kept
	}
	sort.SliceStable(list, func(i, j int) bool { return list[i].ID < list[j].ID })
	w := tabwriter.NewWriter(e.Stdout, 0, 4, 2, ' ', 0)
	_, _ = fmt.Fprintln(w, "ID\tNAME\tSTATUS\tCOMMAND\tCWD")
	for _, s := range list {
		status := string(s.Status)
		if s.ExitCode != nil {
			status = fmt.Sprintf("%s (%d)", s.Status, *s.ExitCode)
		}
		if s.Hybrid != nil {
			status += " · " + hybridSide(s.Hybrid)
		}
		_, _ = fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\n", s.ID, s.Title, status, strings.Join(s.Cmd, " "), s.Cwd)
	}
	_ = w.Flush()
	return contract.ExitOK
}

// hybridSide names where a hybrid session runs, for `ls`.
func hybridSide(h *contract.SessionHybrid) string {
	switch {
	case h.Side == contract.HybridSideSwitching || h.Phase != "":
		return "切换中"
	case h.Side == contract.HybridSideACP:
		return "聊天"
	default:
		return "终端"
	}
}

var openCommand = &Command{
	Name: "open", Group: GroupSessions, Args: "SESSION",
	Summary:   "Print a session's local link, and open it in the browser with --browser.",
	Arguments: []Item{sessionArgument},
	Env:       []Item{sessionIDEnv},
	Output:    "The token-free local session link.",
	Exit: []ExitItem{
		{Code: 0, Text: "Printed (and opened)."},
		{Code: 1, Text: "No such session, or the browser could not be started."},
		{Code: 2, Text: "Usage error."},
		{Code: 3, Text: "WebMux is not running."},
	},
	Examples: []Example{
		{Command: "webmux open abcd2345", Text: "Print the link of session abcd2345."},
		{Command: "webmux open --browser self", Text: "Open the current session in the default browser."},
	},
	SeeAlso: []string{"webmux get", "webmux ls"},
	Bind: func(flags *flag.FlagSet) Action {
		browser := flags.Bool("browser", false, "also open the link in the default browser")
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
			if err := client.Do(ctx, http.MethodGet, "/api/sessions/"+url.PathEscape(id), nil, nil); err != nil {
				return e.Fail(err)
			}
			link, err := link(ctx, client, id)
			if err != nil {
				return e.Fail(err)
			}
			_, _ = fmt.Fprintln(e.Stdout, link)
			if *browser {
				if err := e.OpenBrowser(link); err != nil {
					return e.Fail(fmt.Errorf("open browser: %w", err))
				}
			}
			return contract.ExitOK
		}
	},
}

// Open implements `webmux open`.
func Open(ctx context.Context, e Env, args []string) int { return e.Run(ctx, openCommand, args) }

// link is the session's token-free local link.
func link(ctx context.Context, client *localclient.Client, id string) (string, error) {
	status, err := client.Status(ctx)
	if err != nil {
		return "", err
	}
	if status.URLs.Local == "" {
		return "", errors.New("the server reported no local link")
	}
	return strings.TrimSuffix(status.URLs.Local, "/") + "/s/" + url.PathEscape(id), nil
}
