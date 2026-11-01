package agentprofile

import (
	"fmt"
	"path/filepath"
	"slices"
	"strings"
)

// Form is how a launch config's argv runs the agent.
type Form int

const (
	// FormDirect executes the agent: [executable, args...].
	FormDirect Form = iota
	// FormBashrc runs the agent through WebMux's interactive bash wrapper so
	// ~/.bashrc applies: bash -ic 'exec -- "$@"' webmux executable args...
	FormBashrc
)

const (
	bashrcScript = `exec -- "$@"`
	// bashrcACPScript moves the agent's stdout to fd 3, the protocol pipe,
	// so anything ~/.bashrc prints stays on stdout/stderr and out of ACP.
	bashrcACPScript = `exec -- "$@" 1>&3 3>&-`
	bashrcName      = "webmux"
)

// Reason codes explain why a command cannot use hybrid mode. Clients map
// them to text; they are stable wire values.
type Reason string

const (
	// ReasonUnknownAgent: no built-in profile matches the command.
	ReasonUnknownAgent Reason = "unknown_agent"
	// ReasonUnsupportedForm: a profile's executable appears, but not in a
	// direct or bashrc-wrapped form.
	ReasonUnsupportedForm Reason = "unsupported_form"
	// ReasonSessionStore: an option makes the TUI and ACP use different
	// session stores.
	ReasonSessionStore Reason = "session_store"
	// ReasonDirectoryArg: the command names its own working directory.
	ReasonDirectoryArg Reason = "directory_arg"
	// ReasonNonInteractive: an option or subcommand starts no interactive UI.
	ReasonNonInteractive Reason = "non_interactive"
	// ReasonSessionArg: a subcommand opens an existing session.
	ReasonSessionArg Reason = "session_arg"
)

// Message is the user-facing explanation of r, used in API errors.
func (r Reason) Message() string {
	switch r {
	case ReasonUnsupportedForm:
		return "命令写法不受支持，请直接执行 agent 或使用“加载 .bashrc”"
	case ReasonSessionStore:
		return "该参数使聊天视图无法找到会话"
	case ReasonDirectoryArg:
		return "请在新建会话时选择目录，不要在命令中写目录"
	case ReasonNonInteractive:
		return "该参数不启动交互界面"
	case ReasonSessionArg:
		return "该参数打开已有会话，混合配置请不要指定会话"
	default:
		return "未识别到支持的 Agent，当前版本只支持 OpenCode、omp 与 Codex CLI"
	}
}

// UnsupportedError reports why argv cannot run as a hybrid session. Arg is
// the offending argument, empty when the whole command is unrecognised.
type UnsupportedError struct {
	Reason Reason
	Arg    string
}

func (e *UnsupportedError) Error() string {
	if e.Arg == "" {
		return fmt.Sprintf("hybrid mode unavailable: %s", e.Reason)
	}
	return fmt.Sprintf("hybrid mode unavailable: %s (%s)", e.Reason, e.Arg)
}

// Support is the hybrid capability of a command, as reported to clients.
// Connector is the profile's ACP connector, whether or not it is installed.
type Support struct {
	Supported bool
	Agent     string
	Reason    Reason
	Connector *Connector
}

// Check reports whether argv can run as a hybrid session.
func Check(argv []string) Support {
	cmd, err := Parse(argv)
	if err != nil {
		return Support{Reason: err.(*UnsupportedError).Reason}
	}
	return Support{Supported: true, Agent: cmd.Profile.Name, Connector: cmd.Profile.Connector}
}

// Seed is a chat-side config value derived from the launch arguments: the
// option ConfigID set to Value, because of argument Arg as written.
type Seed struct {
	ConfigID string
	Value    string
	Arg      string
}

// keptOption is one kept option: its name, value (empty without one) and
// the argument as written, an option and its value joined by a space.
type keptOption struct {
	name, value, token string
}

// canonical maps long option names to the short ones in aliases.
func (o keptOption) canonical(aliases map[string]string) string {
	if short, ok := aliases[o.name]; ok {
		return short
	}
	return o.name
}

// Command is a parsed launch command for a built-in agent.
type Command struct {
	Profile *Profile
	Form    Form
	// Executable is argv's agent executable as written (possibly a path).
	Executable string
	args       []string
	kept       []string
	keptOpts   []keptOption
	dropped    []string
}

// Parse recognises argv as a profile's command in a supported form and
// classifies its arguments. The returned error is always *UnsupportedError.
func Parse(argv []string) (Command, error) {
	form, exe, args, ok := splitForm(argv)
	profile, known := Lookup(filepath.Base(exe))
	if !ok || !known {
		return Command{}, &UnsupportedError{Reason: unrecognised(argv)}
	}
	cmd := Command{Profile: profile, Form: form, Executable: exe, args: slices.Clone(args)}
	if err := cmd.classify(); err != nil {
		return Command{}, err
	}
	return cmd, nil
}

func splitForm(argv []string) (Form, string, []string, bool) {
	if len(argv) >= 5 && argv[0] == "bash" && argv[1] == "-ic" && argv[2] == bashrcScript && argv[3] == bashrcName {
		return FormBashrc, argv[4], argv[5:], true
	}
	if len(argv) >= 1 && argv[0] != "" && argv[0] != "bash" {
		return FormDirect, argv[0], argv[1:], true
	}
	return FormDirect, "", nil, false
}

// unrecognised distinguishes a wrapper around a known agent, which the user
// can rewrite, from a command that runs no supported agent at all.
func unrecognised(argv []string) Reason {
	for _, token := range argv {
		for _, word := range strings.Fields(token) {
			if _, ok := Lookup(filepath.Base(word)); ok {
				return ReasonUnsupportedForm
			}
		}
	}
	return ReasonUnknownAgent
}

func (c *Command) classify() error {
	p := c.Profile
	positionalSeen := false
	optionsDone := false
	for i := 0; i < len(c.args); i++ {
		token := c.args[i]
		if !optionsDone && token == "--" {
			optionsDone = true
			continue
		}
		if optionsDone || !strings.HasPrefix(token, "-") || token == "-" {
			if p.positional == positionalDirectory {
				return &UnsupportedError{Reason: ReasonDirectoryArg, Arg: token}
			}
			if !positionalSeen && !optionsDone {
				if p.sessionSubcommands[token] {
					return &UnsupportedError{Reason: ReasonSessionArg, Arg: token}
				}
				if p.subcommands[token] || hasAnyPrefix(token, p.subcommandPrefixes) {
					return &UnsupportedError{Reason: ReasonNonInteractive, Arg: token}
				}
			}
			positionalSeen = true
			c.dropped = append(c.dropped, token)
			continue
		}
		name, inlineValue, inline := strings.Cut(token, "=")
		opt, known := p.options[name]
		taken := []string{token}
		if known && !inline {
			next := i + 1
			switch {
			case opt.arity == arityOne && next < len(c.args):
				taken = append(taken, c.args[next])
				i = next
			case opt.arity == arityOptional && next < len(c.args) && !strings.HasPrefix(c.args[next], "-"):
				taken = append(taken, c.args[next])
				i = next
			}
		}
		if !known {
			c.dropped = append(c.dropped, token)
			continue
		}
		switch opt.class {
		case argKeep:
			c.kept = append(c.kept, taken...)
			kept := keptOption{name: name, value: inlineValue, token: strings.Join(taken, " ")}
			if len(taken) > 1 {
				kept.value = taken[1]
			}
			c.keptOpts = append(c.keptOpts, kept)
		case argDrop:
			c.dropped = append(c.dropped, strings.Join(taken, " "))
		case argStore:
			return &UnsupportedError{Reason: ReasonSessionStore, Arg: name}
		case argDirectory:
			return &UnsupportedError{Reason: ReasonDirectoryArg, Arg: name}
		case argNonInteractive:
			return &UnsupportedError{Reason: ReasonNonInteractive, Arg: name}
		}
	}
	return nil
}

func hasAnyPrefix(s string, prefixes []string) bool {
	for _, prefix := range prefixes {
		if strings.HasPrefix(s, prefix) {
			return true
		}
	}
	return false
}

func (c Command) wrap(script string, args ...string) []string {
	return c.wrapExec(script, c.Executable, args...)
}

// wrapExec runs executable in the command's form.
func (c Command) wrapExec(script, executable string, args ...string) []string {
	if c.Form == FormBashrc {
		return append([]string{"bash", "-ic", script, bashrcName, executable}, args...)
	}
	return append([]string{executable}, args...)
}

// Lookup runs `command -v` for each name in the command's form, so a
// bashrc form sees the PATH ~/.bashrc sets up: its output ends with a line
// of the marker followed by the first name's path, or nothing when that is
// not found, then one such line per further name. Startup-file output
// comes before the marker lines. FormDirect returns nil: the caller looks
// names up in the session's PATH itself.
func (c Command) Lookup(names ...string) []string {
	if c.Form != FormBashrc {
		return nil
	}
	script := `for n in "$@"; do printf '\n` + LookupMarker + `%s' "$(command -v -- "$n" 2>/dev/null)"; done; printf '\n'`
	return append([]string{"bash", "-ic", script, bashrcName}, names...)
}

// LookupMarker starts each answer line of a Lookup command.
const LookupMarker = "\x01webmux-path:"

// injected returns the profile's TUI options that args does not already hold.
func (c Command) injected(args []string) []string {
	var missing []string
	for _, option := range c.Profile.tuiInject {
		if !slices.Contains(args, option) {
			missing = append(missing, option)
		}
	}
	return missing
}

// TUIArgv is the first terminal launch: the original command plus the
// profile's required TUI options, in the original form.
func (c Command) TUIArgv() []string {
	return c.wrap(bashrcScript, append(c.injected(c.args), c.args...)...)
}

// ResumeArgv opens agentSessionID in the terminal UI, keeping only the
// options the profile preserves. Dropped lists the omitted arguments as
// written, an option and its value joined by a space.
func (c Command) ResumeArgv(agentSessionID string) (argv, dropped []string) {
	args := append(c.injected(c.kept), c.kept...)
	args = append(args, c.Profile.resumeFlag, agentSessionID)
	return c.wrap(bashrcScript, args...), slices.Clone(c.dropped)
}

// ACPArgv starts the agent's ACP server in the original form: the agent's
// acp subcommand, or the profile's connector without arguments. A bashrc
// form writes ACP to fd 3, which the caller must connect to the protocol
// pipe (ProtocolOnFD3). Unsupported lists kept options that change
// behaviour but do not reach the chat side, for the user to be told.
func (c Command) ACPArgv() (argv, unsupported []string) {
	_, unsupported = c.ChatConfig()
	if connector := c.Profile.Connector; connector != nil {
		return c.wrapExec(bashrcACPScript, connector.Executable), unsupported
	}
	args := []string{"acp"}
	if c.Profile.passToACP {
		args = append(args, c.kept...)
	}
	return c.wrap(bashrcACPScript, args...), unsupported
}

// ChatConfig derives chat-side config values from the kept options, and
// lists the kept options the chat side cannot apply.
func (c Command) ChatConfig() (seeds []Seed, unsupported []string) {
	if c.Profile.chatSide == nil {
		return nil, nil
	}
	return c.Profile.chatSide(c.keptOpts)
}

// ProtocolOnFD3 reports that ACPArgv's process writes the protocol to fd 3.
func (c Command) ProtocolOnFD3() bool { return c.Form == FormBashrc }

// Dropped lists the arguments that resume and ACP commands omit.
func (c Command) Dropped() []string { return slices.Clone(c.dropped) }
