package agentprofile

import (
	"errors"
	"fmt"
	"net/url"
	"path/filepath"
	"slices"
	"strings"
	"unicode/utf8"

	"github.com/rainchan/webmux/internal/launchicon"
)

// CommandSpec binds one complete terminal launch command to the complete
// commands used for its ACP and terminal-resume modes. Every field is argv,
// never a shell command string.
type CommandSpec struct {
	// Match is the complete inner argv stored in a launch config. The first
	// token may be a base name, in which case an actual path with that base
	// name also matches. All remaining tokens match exactly.
	Match []string `json:"match"`
	// ACP is the complete argv that starts the stdio ACP server.
	ACP []string `json:"acp"`
	// Resume is the complete argv prefix that reopens a terminal session;
	// WebMux appends the session ID as its final token.
	Resume []string `json:"resume"`
	// Connector describes a separately installed ACP program that this
	// binding needs. It does not rewrite ACP, which remains a complete argv.
	Connector *Connector `json:"connector,omitempty"`
	// Launcher is the PTY root process when the terminal command keeps the
	// agent as a direct child, for example an isolation tool.
	Launcher string `json:"launcher,omitempty"`
}

// Spec describes an ACP agent that has no built-in adapter. The stable map key
// used to construct it is independent from both its launch commands and its
// actual terminal process names.
type Spec struct {
	// Kind is the protocol family. Empty and "acp" both mean ACP.
	Kind string `json:"kind,omitempty"`
	// Title is the display name; empty falls back to the stable agent name.
	Title string `json:"title,omitempty"`
	// Icon is a launch-config icon key; empty falls back to "terminal".
	Icon string `json:"icon,omitempty"`
	// ProcessExecutables are the terminal agent's actual native process base
	// names. They are explicit and independent from the stable agent name.
	ProcessExecutables []string `json:"processExecutables,omitempty"`
	// Commands are the explicitly supported complete launch commands.
	Commands []CommandSpec `json:"commands"`
}

const (
	// KindACP is the only protocol family a Spec can describe today.
	KindACP = "acp"
	// MaxSpecTitleChars bounds the display name like launch names are bound.
	MaxSpecTitleChars = 64
)

// Validate reports whether s can build an unambiguous external agent.
func (s Spec) Validate() error {
	switch s.Kind {
	case "", KindACP:
	default:
		return fmt.Errorf("unsupported kind %q", s.Kind)
	}
	if !launchicon.Supported(s.Icon) {
		return errors.New("unsupported icon")
	}
	if !utf8.ValidString(s.Title) {
		return errors.New("title is not valid UTF-8")
	}
	if utf8.RuneCountInString(s.Title) > MaxSpecTitleChars {
		return fmt.Errorf("title exceeds %d characters", MaxSpecTitleChars)
	}
	if len(s.ProcessExecutables) == 0 {
		return errors.New("processExecutables must not be empty")
	}
	for _, executable := range s.ProcessExecutables {
		if err := validateBaseName(executable); err != nil {
			return fmt.Errorf("process executable %q: %w", executable, err)
		}
	}
	if len(s.Commands) == 0 {
		return errors.New("commands must not be empty")
	}
	for i, command := range s.Commands {
		if err := command.Validate(); err != nil {
			return fmt.Errorf("commands[%d]: %w", i, err)
		}
		for j := 0; j < i; j++ {
			if slices.Equal(s.Commands[j].Match, command.Match) {
				return fmt.Errorf("commands[%d]: duplicate match", i)
			}
		}
	}
	return nil
}

// Validate reports whether c is a complete, executable command binding.
func (c CommandSpec) Validate() error {
	for name, argv := range map[string][]string{"match": c.Match, "acp": c.ACP, "resume": c.Resume} {
		if len(argv) == 0 || argv[0] == "" {
			return fmt.Errorf("%s executable must not be empty", name)
		}
		for i, token := range argv {
			if strings.ContainsRune(token, 0) {
				return fmt.Errorf("%s[%d] contains a NUL byte", name, i)
			}
		}
	}
	if err := validateBaseName(c.Match[0]); err != nil {
		return fmt.Errorf("match[0]: %w", err)
	}
	if c.Connector != nil {
		if err := validateBaseName(c.Connector.Executable); err != nil {
			return fmt.Errorf("connector executable: %w", err)
		}
		if strings.TrimSpace(c.Connector.Title) == "" {
			return errors.New("connector title must not be empty")
		}
		if strings.TrimSpace(c.Connector.InstallCommand) == "" {
			return errors.New("connector installCommand must not be empty")
		}
		docs, err := url.Parse(c.Connector.DocsURL)
		if err != nil || (docs.Scheme != "http" && docs.Scheme != "https") || docs.Host == "" || docs.User != nil {
			return errors.New("connector docsUrl must be an HTTP(S) URL")
		}
	}
	if c.Launcher != "" {
		if err := validateBaseName(c.Launcher); err != nil {
			return fmt.Errorf("launcher: %w", err)
		}
	}
	return nil
}

func validateBaseName(name string) error {
	if name == "" || filepath.Base(name) != name || strings.ContainsAny(name, "/\\ \t\n") || strings.ContainsRune(name, 0) {
		return errors.New("must be an executable base name")
	}
	return nil
}

// Agent builds the external agent named name.
func (s Spec) Agent(name string) Agent {
	title := s.Title
	if title == "" {
		title = name
	}
	icon := s.Icon
	if icon == "" {
		icon = "terminal"
	}
	processes := slices.Clone(s.ProcessExecutables)
	profile := Profile{
		Name:        name,
		Title:       title,
		Icon:        icon,
		Executables: processes,
		Positional:  PositionalPrompt,
		Options:     map[string]Option{},
		Subcommands: SetOf(),
	}
	commands := make([]CommandSpec, len(s.Commands))
	for i, command := range s.Commands {
		commands[i] = command.clone()
	}
	return &specAgent{profile: profile, commands: commands}
}

func (c CommandSpec) clone() CommandSpec {
	c.Match = slices.Clone(c.Match)
	c.ACP = slices.Clone(c.ACP)
	c.Resume = slices.Clone(c.Resume)
	if c.Connector != nil {
		connector := *c.Connector
		c.Connector = &connector
	}
	return c
}

// commandBinder marks an agent that is recognized only by explicit complete
// command bindings rather than by Profile.Executables.
type commandBinder interface {
	commandBindings() []CommandSpec
}

// specAgent is an agent whose behaviour comes entirely from its Spec. It has
// no terminal-signal identity, so hybrid sessions match it by session/list.
type specAgent struct {
	profile  Profile
	commands []CommandSpec
}

// Profile implements Agent.
func (a *specAgent) Profile() *Profile { return &a.profile }

// Identify implements Agent. A spec agent reports no terminal identity: the
// spec has no way to describe one, and a false result sends the caller to the
// session/list fallback, which never writes to the agent's session store.
func (a *specAgent) Identify(Signals) (Identity, bool) { return Identity{}, false }

// InterruptKeys implements Agent with ctrl-c.
func (a *specAgent) InterruptKeys() [][]byte { return [][]byte{{0x03}} }

func (a *specAgent) commandBindings() []CommandSpec {
	out := make([]CommandSpec, len(a.commands))
	for i, command := range a.commands {
		out[i] = command.clone()
	}
	return out
}
