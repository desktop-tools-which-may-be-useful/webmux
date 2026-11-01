package agentprofile

import (
	"strings"
	"time"
)

// Codex config option IDs and mode values of the codex-acp connector.
const (
	codexModel     = "model"
	codexEffort    = "reasoning_effort"
	codexMode      = "mode"
	codexEffortKey = "model_reasoning_effort"
)

// codex has no ACP mode of its own: the codex-acp connector translates ACP
// to the Codex app-server protocol and starts `codex app-server` itself
// (from CODEX_PATH, so both sides run the same codex).
var codex = &Profile{
	Name:            Codex,
	Title:           "Codex CLI",
	VerifiedVersion: "0.165.0",
	InterruptKeys:   [][]byte{[]byte("\x1b")},
	// session/load returns the connector's defaults, whatever the session
	// last used.
	ReapplyOnLoad:      []string{codexModel, codexEffort, codexMode},
	Steering:           true,
	SteeringIdlePrompt: true,
	Resume:             true,
	Connector: &Connector{
		Executable:      "codex-acp",
		Title:           "Codex ACP 适配器",
		InstallCommand:  "npm install -g @agentclientprotocol/codex-acp",
		DocsURL:         "https://github.com/agentclientprotocol/codex-acp",
		VerifiedVersion: "2.3.0",
	},
	// npm installs run codex through a node script that starts the native
	// executable.
	Launcher: "node",
	// The chat side never runs a browser login.
	ACPEnv:         map[string]string{"NO_BROWSER": "1"},
	AgentPathEnv:   "CODEX_PATH",
	HiddenCommands: []string{"logout"},
	StartupQuiet:   3 * time.Second,
	// resume is a subcommand: global options come before it.
	resumeFlag: "resume",
	// The default TUI hands turns to a shared app-server daemon outside the
	// session's process group, where a turn outlives the TUI.
	tuiInject:  []string{"--no-daemon"},
	positional: positionalPrompt,
	options:    codexOptions(),
	chatSide:   codexChatSide,
	locks:      &lockLayout{homeEnv: "CODEX_HOME", homeDefault: ".codex", dir: "thread-writer-locks", suffix: ".lock"},
	subcommands: setOf("exec", "e", "review", "login", "logout", "mcp", "plugin",
		"app-server", "remote-control", "app", "completion", "update", "doctor",
		"sandbox", "debug", "apply", "a", "cloud", "exec-server", "features",
		"agents", "queue", "archive", "delete", "unarchive", "help"),
	subcommandPrefixes: []string{"migrate-"},
	sessionSubcommands: setOf("resume", "fork"),
}

func codexOptions() map[string]option {
	options := map[string]option{
		"-i":                      {argDrop, arityOne},
		"--image":                 {argDrop, arityOne},
		"--remote":                {argStore, arityOne},
		"--remote-auth-token-env": {argStore, arityOne},
		"-C":                      {argDirectory, arityOne},
		"--cd":                    {argDirectory, arityOne},
		"--worktree":              {argDirectory, arityNone},
		"-h":                      {argNonInteractive, arityNone},
		"--help":                  {argNonInteractive, arityNone},
		"-V":                      {argNonInteractive, arityNone},
		"--version":               {argNonInteractive, arityNone},
	}
	for _, name := range []string{"-m", "--model", "-p", "--profile", "-s",
		"--sandbox", "-a", "--ask-for-approval", "-c", "--config", "--enable",
		"--disable", "--add-dir", "--local-provider"} {
		options[name] = option{argKeep, arityOne}
	}
	for _, name := range []string{"--approve-for-me",
		"--dangerously-bypass-approvals-and-sandbox", "--dangerously-bypass-hook-trust",
		"--search", "--oss", "--no-alt-screen", "--strict-config", "--no-daemon"} {
		options[name] = option{argKeep, arityNone}
	}
	return options
}

// codexModes maps approval and sandbox options to the connector's mode, in
// priority order: the first present one wins.
var codexModes = []struct {
	name, value, mode string
}{
	{"--dangerously-bypass-approvals-and-sandbox", "", "agent-full-access"},
	{"--approve-for-me", "", "agent"},
	{"-s", "read-only", "read-only"},
	{"-s", "workspace-write", "workspace-write"},
}

// codexChatOnly are kept options the connector has no equivalent for.
var codexChatOnly = setOf("-p", "--profile", "--enable", "--disable", "--search",
	"--add-dir", "--oss", "--local-provider")

// codexChatSide derives the model, reasoning effort and mode from argv,
// and lists the options that do not reach the chat side.
func codexChatSide(kept []keptOption) ([]Seed, []string) {
	var seeds []Seed
	var unsupported []string
	mode := -1
	for _, o := range kept {
		name := o.canonical(map[string]string{"--model": "-m", "--sandbox": "-s", "--ask-for-approval": "-a", "--config": "-c"})
		switch {
		case name == "-m":
			seeds = append(seeds, Seed{ConfigID: codexModel, Value: o.value, Arg: o.token})
		case name == "-c":
			key, value, ok := strings.Cut(o.value, "=")
			if ok && strings.TrimSpace(key) == codexEffortKey {
				seeds = append(seeds, Seed{ConfigID: codexEffort, Value: unquote(strings.TrimSpace(value)), Arg: o.token})
				continue
			}
			unsupported = append(unsupported, o.token)
		case name == "-a":
			unsupported = append(unsupported, o.token)
		case name == "-s" || name == "--approve-for-me" || name == "--dangerously-bypass-approvals-and-sandbox":
			i := modeIndex(name, o.value)
			if i < 0 {
				unsupported = append(unsupported, o.token)
				continue
			}
			if mode < 0 || i < mode {
				mode = i
			}
		case codexChatOnly[name]:
			unsupported = append(unsupported, o.token)
		}
	}
	if mode >= 0 {
		m := codexModes[mode]
		arg := m.name
		if m.value != "" {
			arg += " " + m.value
		}
		seeds = append(seeds, Seed{ConfigID: codexMode, Value: m.mode, Arg: arg})
	}
	return seeds, unsupported
}

func modeIndex(name, value string) int {
	for i, m := range codexModes {
		if m.name == name && m.value == value {
			return i
		}
	}
	return -1
}

// unquote strips one pair of matching TOML string quotes.
func unquote(s string) string {
	if len(s) >= 2 && (s[0] == '"' || s[0] == '\'') && s[len(s)-1] == s[0] {
		return s[1 : len(s)-1]
	}
	return s
}
