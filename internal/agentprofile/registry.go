package agentprofile

import (
	"fmt"
	"path/filepath"
	"slices"
	"strings"
	"time"
)

// Registry is an immutable, ordered set of agents. Order matters only for
// Identify: the first agent that recognises the signals wins.
type Registry struct {
	agents []Agent
}

// NewRegistry lists agents in Identify order. It panics on an invalid list;
// source-defined registries are programming constants. Config loaders use
// NewRegistryChecked so one invalid declaration can be skipped.
func NewRegistry(agents ...Agent) *Registry {
	registry, err := NewRegistryChecked(agents...)
	if err != nil {
		panic("agentprofile: " + err.Error())
	}
	return registry
}

// NewRegistryChecked builds a registry or reports a name or command-binding
// conflict without panicking.
func NewRegistryChecked(agents ...Agent) (*Registry, error) {
	seenNames := map[string]bool{}
	seenCommands := map[string]bool{}
	for _, agent := range agents {
		name := agent.Profile().Name
		if name == "" || seenNames[name] {
			return nil, fmt.Errorf("invalid or duplicate agent name %q", name)
		}
		seenNames[name] = true
		bound, ok := agent.(commandBinder)
		if !ok {
			continue
		}
		for _, command := range bound.commandBindings() {
			key := strings.Join(command.Match, "\x00")
			if seenCommands[key] {
				return nil, fmt.Errorf("duplicate command binding %q", command.Match)
			}
			seenCommands[key] = true
			for _, other := range agents {
				if _, declared := other.(commandBinder); !declared && other.Profile().RunsAs(command.Match[0]) {
					return nil, fmt.Errorf("command binding %q shadows a built-in executable", command.Match)
				}
			}
		}
	}
	return &Registry{agents: slices.Clone(agents)}, nil
}

// Agents returns the registered agents in order.
func (r *Registry) Agents() []Agent { return append([]Agent(nil), r.agents...) }

// Lookup returns the agent named name.
func (r *Registry) Lookup(name string) (Agent, bool) {
	for _, a := range r.agents {
		if a.Profile().Name == name {
			return a, true
		}
	}
	return nil, false
}

// ForExecutable returns the built-in agent that runs as executable base name
// base. Declared agents are recognised only by complete command bindings, so a
// process executable cannot accidentally become another launch command.
func (r *Registry) ForExecutable(base string) (Agent, bool) {
	for _, a := range r.agents {
		if _, declared := a.(commandBinder); declared {
			continue
		}
		if a.Profile().RunsAs(base) {
			return a, true
		}
	}
	return nil, false
}

// SubmitDelay returns the terminal submission separation for argv's agent.
// It depends only on built-in executable profiles; declared agents use the
// default zero delay.
func (r *Registry) SubmitDelay(argv []string) time.Duration {
	_, _, exe, _, ok := splitForm(argv)
	if !ok {
		return 0
	}
	a, ok := r.ForExecutable(filepath.Base(exe))
	if !ok {
		return 0
	}
	return SubmitDelay(a)
}

// Identify maps signals to the first agent that recognises them.
func (r *Registry) Identify(s Signals) (Identity, bool) {
	for _, a := range r.agents {
		if id, ok := a.Identify(s); ok {
			return id, true
		}
	}
	return Identity{}, false
}

// Explain is the user-facing explanation of reason, used in API errors;
// an unknown agent is told which agents are supported.
func (r *Registry) Explain(reason Reason) string {
	if reason != ReasonUnknownAgent {
		return reason.Message()
	}
	return reason.Message() + "，当前版本支持：" + strings.Join(r.Titles(), "、")
}

// Titles lists the agents' display names in order.
func (r *Registry) Titles() []string {
	titles := make([]string, len(r.agents))
	for i, a := range r.agents {
		titles[i] = a.Profile().Title
	}
	return titles
}

// Check reports whether argv can run as a hybrid session.
func (r *Registry) Check(argv []string) Support {
	cmd, err := r.Parse(argv)
	if err != nil {
		return Support{Reason: err.(*UnsupportedError).Reason, Agents: r.Titles()}
	}
	return Support{Supported: true, Agent: cmd.Profile.Name, Connector: cmd.Connector(), Agents: r.Titles()}
}

// Parse recognises argv as an agent's command in a supported form and
// classifies its arguments. Declared agents match an explicit complete argv;
// built-in agents keep their executable-and-options parser. The returned error
// is always *UnsupportedError.
func (r *Registry) Parse(argv []string) (Command, error) {
	form, shell, exe, args, ok := splitForm(argv)
	if !ok {
		return Command{}, &UnsupportedError{Reason: r.unrecognised(argv)}
	}
	inner := append([]string{exe}, args...)
	for _, agent := range r.agents {
		bound, declared := agent.(commandBinder)
		if !declared {
			continue
		}
		for _, command := range bound.commandBindings() {
			if commandMatches(command.Match, inner) {
				return newBoundCommand(agent, form, shell, argv, inner, command), nil
			}
		}
	}
	agent, known := r.ForExecutable(filepath.Base(exe))
	if !known {
		return Command{}, &UnsupportedError{Reason: r.unrecognised(argv)}
	}
	return newCommand(agent, form, shell, exe, args)
}

func commandMatches(pattern, actual []string) bool {
	return len(pattern) == len(actual) && filepath.Base(actual[0]) == pattern[0] && slices.Equal(pattern[1:], actual[1:])
}

// unrecognised distinguishes explicit env or shell wrappers around a known
// built-in agent from commands that run no supported built-in. It never scans
// arbitrary arguments, and declared commands are recognised only by exact
// bindings.
func (r *Registry) unrecognised(argv []string) Reason {
	if executable, ok := wrappedExecutable(argv); ok {
		if _, known := r.ForExecutable(filepath.Base(executable)); known {
			return ReasonUnsupportedForm
		}
	}
	return ReasonUnknownAgent
}

func wrappedExecutable(argv []string) (string, bool) {
	if len(argv) == 0 {
		return "", false
	}
	switch filepath.Base(argv[0]) {
	case "env":
		i := 1
		for i < len(argv) && strings.Contains(argv[i], "=") && !strings.HasPrefix(argv[i], "=") {
			i++
		}
		if i == len(argv) {
			return "", false
		}
		if executable, ok := wrappedExecutable(argv[i:]); ok {
			return executable, true
		}
		return argv[i], true
	case "sh", ShellBash, ShellZsh:
		if len(argv) < 3 || (argv[1] != "-c" && argv[1] != "-ic") {
			return "", false
		}
		words := strings.Fields(argv[2])
		if len(words) > 0 && words[0] == "exec" {
			words = words[1:]
		}
		if len(words) > 0 && words[0] == "--" {
			words = words[1:]
		}
		if len(words) == 0 {
			return "", false
		}
		return words[0], true
	default:
		return "", false
	}
}
