package clicmd

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"slices"
	"strings"

	"github.com/rainchan/webmux/internal/contract"
)

// Command groups, in the order help lists them.
const (
	GroupServer    = "Server and operations"
	GroupSessions  = "Sessions"
	GroupLaunch    = "Launch"
	GroupChat      = "Chat"
	GroupWorkspace = "Workspace"
	GroupGuide     = "Guide"
	GroupHelp      = "Help and completion"
)

// Groups lists the command groups in help order.
var Groups = []string{GroupServer, GroupSessions, GroupLaunch, GroupChat, GroupWorkspace, GroupGuide, GroupHelp}

// Action runs a command with its positional arguments after its flags were
// parsed into the variables Bind declared.
type Action func(ctx context.Context, e Env, args []string) int

// Item is a named entry of a help section.
type Item struct {
	Name string `json:"name"`
	Text string `json:"text"`
}

// ExitItem documents one exit status.
type ExitItem struct {
	Code int    `json:"code"`
	Text string `json:"text"`
}

// Example is one command line with what it does.
type Example struct {
	Command string `json:"command"`
	Text    string `json:"text"`
}

// Note adds what the flag package cannot express about a flag.
type Note struct {
	// AliasOf names the long flag this one-letter flag stands for.
	AliasOf    string
	Repeatable bool
	Conflicts  []string
	Requires   []string
}

// Command declares one command or subcommand. It is the single source of
// its flag parsing, --help, help --json and completion: flags exist only
// through Bind.
type Command struct {
	// Name is the command path, such as "get" or "chat status".
	Name  string
	Group string
	// Args is the positional part of the usage line, such as "[SESSION]".
	Args        string
	Summary     string
	Description string
	Arguments   []Item
	Env         []Item
	Output      string
	Exit        []ExitItem
	Examples    []Example
	SeeAlso     []string
	Notes       map[string]Note
	// Platform is "unix" or "windows" for a command one client lacks.
	Platform string
	Hidden   bool
	// Strict stops flag parsing at the first positional argument, for a
	// command whose trailing arguments are another program's argv.
	Strict bool
	// DashArgs makes a Strict command take positional arguments only after
	// "--"; a stray word before it is a usage error, explained by StrayHint
	// when set.
	DashArgs  bool
	StrayHint func(fs *flag.FlagSet) string
	// NoGlobal leaves out the client flags (--config, --socket) of Env.
	NoGlobal bool
	// RawArgs passes every argument to the action unparsed.
	RawArgs bool
	// Bind declares the flags on fs and returns the action. A nil Bind
	// marks a group that only lists its subcommands.
	Bind func(fs *flag.FlagSet) Action
}

// Usage is the command's usage line.
func (c *Command) Usage() string {
	usage := "webmux " + c.Name
	if c.Bind == nil {
		return usage + " <command> [options]"
	}
	usage += " [options]"
	if c.Args != "" {
		usage += " " + c.Args
	}
	return usage
}

// Registry is every command a client offers.
type Registry []*Command

// Find returns the longest command path matching the start of args and the
// remaining arguments.
func (r Registry) Find(args []string) (*Command, []string) {
	var best *Command
	bestLen := 0
	for _, c := range r {
		path := strings.Fields(c.Name)
		if len(path) > len(args) || len(path) <= bestLen {
			continue
		}
		if slices.Equal(path, args[:len(path)]) {
			best, bestLen = c, len(path)
		}
	}
	if best == nil {
		return nil, args
	}
	return best, args[bestLen:]
}

// Children lists the visible direct subcommands of group name.
func (r Registry) Children(name string) []*Command {
	depth := len(strings.Fields(name)) + 1
	var out []*Command
	for _, c := range r {
		if c.Hidden || !strings.HasPrefix(c.Name, name+" ") || len(strings.Fields(c.Name)) != depth {
			continue
		}
		out = append(out, c)
	}
	return out
}

// Under lists command name and everything below it, or every command for "".
func (r Registry) Under(name string) Registry {
	var out Registry
	for _, c := range r {
		if name == "" || c.Name == name || strings.HasPrefix(c.Name, name+" ") {
			out = append(out, c)
		}
	}
	return out
}

// Visible leaves out hidden commands and commands of another platform.
func (r Registry) Visible(platform string) Registry {
	var out Registry
	for _, c := range r {
		if !c.Hidden && (c.Platform == "" || c.Platform == platform) {
			out = append(out, c)
		}
	}
	return out
}

// FlagSet declares c's flags, plus the client flags unless c opts out.
func (e Env) FlagSet(c *Command) (*flag.FlagSet, Action) {
	fs := flag.NewFlagSet("webmux "+c.Name, flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	fs.Usage = func() {}
	var action Action
	if c.Bind != nil {
		action = c.Bind(fs)
	}
	if !c.NoGlobal && e.Flags != nil {
		e.Flags(fs)
	}
	return fs, action
}

// Parse parses args for c. Unless c is Strict, flags may follow positional
// arguments; everything after "--" is positional. --help prints the help and
// returns ExitOK with ok false; a usage error is reported and returns
// ExitUsage.
func (e Env) Parse(c *Command, args []string) (Action, []string, int, bool) {
	fs, action := e.FlagSet(c)
	var positional []string
	rest := args
	for {
		if err := fs.Parse(rest); err != nil {
			if errors.Is(err, flag.ErrHelp) {
				e.PrintHelp(c)
				return nil, nil, contract.ExitOK, false
			}
			return nil, nil, e.usageError(c, err.Error()), false
		}
		remaining := fs.Args()
		if len(remaining) == 0 {
			break
		}
		// Parse consumed a "--" terminator when it stopped right after one.
		if stop := len(rest) - len(remaining); stop > 0 && rest[stop-1] == "--" {
			positional = append(positional, remaining...)
			break
		}
		if c.Strict && c.DashArgs {
			hint := "positional arguments go after --"
			if c.StrayHint != nil {
				hint = c.StrayHint(fs)
			}
			return nil, nil, e.usageError(c, fmt.Sprintf("unexpected argument %q: %s", remaining[0], hint)), false
		}
		if c.Strict {
			positional = append(positional, remaining...)
			break
		}
		positional = append(positional, remaining[0])
		rest = remaining[1:]
	}
	return action, positional, contract.ExitOK, true
}

// usageError prints a one-line reason, the usage line and a pointer to the
// help, and returns ExitUsage.
func (e Env) usageError(c *Command, reason string) int {
	_, _ = fmt.Fprintf(e.Stderr, "webmux %s: %s\nUsage: %s\nRun 'webmux %s --help' for details.\n", c.Name, reason, c.Usage(), c.Name)
	return contract.ExitUsage
}

// Usagef reports a usage error of the running command after parsing.
func (e Env) Usagef(format string, args ...any) int {
	return e.usageError(e.current, fmt.Sprintf(format, args...))
}

// Run parses args for c and runs it.
func (e Env) Run(ctx context.Context, c *Command, args []string) int {
	if c.Bind == nil {
		return e.groupHelp(c, args)
	}
	if c.RawArgs {
		e.current = c
		return c.Bind(flag.NewFlagSet(c.Name, flag.ContinueOnError))(ctx, e, args)
	}
	action, positional, code, ok := e.Parse(c, args)
	if !ok {
		return code
	}
	e.current = c
	return action(ctx, e, positional)
}

// groupHelp answers a group without a subcommand: its help goes to stderr
// and the exit status is ExitUsage; --help sends it to stdout with ExitOK.
func (e Env) groupHelp(c *Command, args []string) int {
	if len(args) > 0 && isHelpFlag(args[0]) {
		e.PrintHelp(c)
		return contract.ExitOK
	}
	if len(args) > 0 {
		_, _ = fmt.Fprintf(e.Stderr, "webmux %s: unknown subcommand %q\n", c.Name, args[0])
	}
	renderHelp(e.Stderr, e.registry(), c, e.globalFlags(c))
	return contract.ExitUsage
}

func isHelpFlag(arg string) bool {
	return arg == "-h" || arg == "--help" || arg == "-help"
}

// Dispatch runs the command args name in registry r. It returns false when
// no command matches, leaving the caller to report an unknown command.
func (e Env) Dispatch(ctx context.Context, r Registry, args []string) (int, bool) {
	if e.Commands == nil {
		e.Commands = r
	}
	c, rest := r.Find(args)
	if c == nil {
		return 0, false
	}
	if c.Platform != "" && c.Platform != e.Platform {
		return 0, false
	}
	return e.Run(ctx, c, rest), true
}

// Help entries shared by the commands that take a session.
var (
	sessionArgument = Item{Name: "SESSION", Text: `A session ID, or "self" for the session this command runs in ($WEBMUX_SESSION_ID).`}
	sessionIDEnv    = Item{Name: "WEBMUX_SESSION_ID", Text: `The current session, used for "self".`}
)

// SessionID resolves a session argument: "self" is $WEBMUX_SESSION_ID. A
// "self" outside every session is a usage error.
func (e Env) SessionID(arg string) (string, int, bool) {
	if arg != "self" {
		return arg, contract.ExitOK, true
	}
	id := ""
	if e.Getenv != nil {
		id = e.Getenv(contract.SessionIDEnv)
	}
	if id == "" {
		return "", e.Usagef(`"self" works only inside a WebMux session (WEBMUX_SESSION_ID is not set)`), false
	}
	return id, contract.ExitOK, true
}
