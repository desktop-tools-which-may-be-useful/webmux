// Package agentprofile describes the CLI agents that hybrid sessions can move
// between a terminal UI and an ACP connection. Profiles are plain data plus
// pure functions over argv and terminal signals; they start no processes and
// hold no state. The parameter tables follow the measured baseline in
// openspec/changes/archive/2026-10-29-add-acp-hybrid-sessions/baseline.md
// and, for Codex, the measurements recorded in the add-codex-hybrid-profile
// change.
package agentprofile

import "time"

// Agent names identify a profile on the wire and in hybrid session records.
const (
	OpenCode = "opencode"
	OMP      = "omp"
	Codex    = "codex"
)

// Connector is a separately installed program that provides an agent's ACP
// side when the agent has no ACP mode of its own. WebMux never installs it;
// it checks that it can be found before every ACP start.
type Connector struct {
	// Executable is looked up through PATH like the agent itself.
	Executable     string
	Title          string
	InstallCommand string
	DocsURL        string
	// VerifiedVersion is the newest release measured; the ACP side reports
	// the connector's version, so an older one only produces a notice.
	VerifiedVersion string
}

// argClass says what happens to a recognised option when webmux builds a
// resume or ACP command from a launch config's argv.
type argClass int

const (
	// argKeep is written into resume commands and, for profiles that pass
	// options through, into the ACP command.
	argKeep argClass = iota
	// argDrop is omitted from resume and ACP commands and listed to the user.
	argDrop
	// argStore prevents hybrid mode: the TUI and ACP would see different
	// session stores.
	argStore
	// argDirectory prevents hybrid mode: the command names its own directory.
	argDirectory
	// argNonInteractive prevents hybrid mode: no interactive UI starts.
	argNonInteractive
)

// arity is how many values an option consumes after the option itself.
type arity int

const (
	arityNone arity = iota
	arityOne
	// arityOptional consumes the next token unless it looks like an option.
	arityOptional
)

type option struct {
	class argClass
	arity arity
}

// positionalPolicy says what a non-option argument means for the agent.
type positionalPolicy int

const (
	// positionalPrompt arguments are an initial prompt and are dropped.
	positionalPrompt positionalPolicy = iota
	// positionalDirectory arguments name a working directory.
	positionalDirectory
)

// Profile is one built-in agent. Fields are read-only after package init.
type Profile struct {
	// Name is the stable agent identifier, also the executable base name.
	Name string
	// Title is the display name.
	Title string
	// VerifiedVersion is the newest release measured in the baseline; older
	// versions only produce a notice.
	VerifiedVersion string
	// InterruptKeys are the key presses that stop a running turn, written to
	// the PTY one at a time: OpenCode needs two separate Esc presses, and a
	// single "\x1b\x1b" read reaches it as one Alt+Esc.
	InterruptKeys [][]byte
	// ReapplyOnLoad lists config option IDs that session/load does not
	// restore, so the recorded values are set again after a switch.
	ReapplyOnLoad []string
	// Steering reports the private _session/steering extension.
	Steering bool
	// Resume allows session/resume (no history replay) for waking from idle
	// sleep and reconnecting after a crash, when the agent declares it;
	// otherwise those use session/load.
	Resume bool
	// SteeringIdlePrompt asks the steering extension to hand a prompt back
	// (outcome promptRequired) instead of starting a turn by itself when no
	// turn runs.
	SteeringIdlePrompt bool
	// Connector provides the ACP side; nil when the agent's own executable
	// has an acp subcommand.
	Connector *Connector
	// Launcher is an interpreter that may run as the PTY's root process when
	// its direct child is the agent's native executable (npm installs).
	Launcher string
	// ACPEnv are variables the ACP process gets unless the launch config
	// sets them.
	ACPEnv map[string]string
	// AgentPathEnv names the ACP variable that receives the resolved path of
	// the terminal UI's executable, so both sides run the same agent; that
	// executable's --version is the agent version.
	AgentPathEnv string
	// HiddenCommands are agent slash commands left out of the chat view's
	// command menu.
	HiddenCommands []string
	// StartupQuiet ignores the terminal UI's busy signal for this long after
	// it starts: Codex reports working while it loads.
	StartupQuiet time.Duration

	resumeFlag string
	tuiInject  []string
	passToACP  bool
	// chatSide derives chat-side config values from the kept options and
	// lists the kept options the chat side cannot apply.
	chatSide    func(kept []keptOption) ([]Seed, []string)
	locks       *lockLayout
	options     map[string]option
	positional  positionalPolicy
	subcommands map[string]bool
	// subcommandPrefixes also start no interactive UI (Codex migrate-*).
	subcommandPrefixes []string
	// sessionSubcommands open an existing session (Codex resume, fork).
	sessionSubcommands map[string]bool
}

// lockLayout is where an agent holds a writer lock for each session it has
// open: <home>/<dir>/<id><suffix>, home from env variable homeEnv or else
// homeDefault under the user's home directory.
type lockLayout struct {
	homeEnv, homeDefault, dir, suffix string
}

var opencode = &Profile{
	Name:            OpenCode,
	Title:           "OpenCode",
	VerifiedVersion: "2.1.4",
	InterruptKeys:   [][]byte{[]byte("\x1b"), []byte("\x1b")},
	Resume:          true,
	resumeFlag:      "-s",
	// The default TUI is a client of the background service, where a turn
	// keeps running after the TUI exits; a private server stops with it.
	tuiInject:  []string{"--standalone"},
	positional: positionalDirectory,
	options: map[string]option{
		"--standalone":  {argKeep, arityNone},
		"--auto":        {argKeep, arityNone},
		"--log-level":   {argKeep, arityOne},
		"--print-logs":  {argKeep, arityNone},
		"-c":            {argDrop, arityNone},
		"--continue":    {argDrop, arityNone},
		"-s":            {argDrop, arityOne},
		"--session":     {argDrop, arityOne},
		"--prompt":      {argDrop, arityOne},
		"--server":      {argStore, arityOne},
		"-h":            {argNonInteractive, arityNone},
		"--help":        {argNonInteractive, arityNone},
		"-v":            {argNonInteractive, arityNone},
		"--version":     {argNonInteractive, arityNone},
		"--completions": {argNonInteractive, arityOne},
		"--wizard":      {argNonInteractive, arityNone},
	},
}

var omp = &Profile{
	Name:            OMP,
	Title:           "omp",
	VerifiedVersion: "18.13.1",
	InterruptKeys:   [][]byte{[]byte("\x1b")},
	ReapplyOnLoad:   []string{"mode"},
	Steering:        true,
	Resume:          true,
	resumeFlag:      "--resume",
	// omp acp parses the full argv and ignores unknown options, so every
	// kept option configures both sides the same way.
	passToACP:  true,
	positional: positionalPrompt,
	options:    ompOptions(),
	subcommands: setOf("acp", "agents", "anonymize", "auth", "auth-broker",
		"auth-gateway", "bench", "browser-relay", "cleanse", "clip", "collab",
		"commit", "completions", "compress", "config", "dry-balance", "find",
		"gallery", "gc", "git", "grep", "grievances", "if-bench", "images",
		"install", "join", "login", "models", "play", "plugin", "predict", "ps",
		"read", "render", "say", "search", "session", "setup", "share", "shell",
		"skill", "ssh", "stats", "stream", "tiny-models", "token", "toks",
		"ttsr", "update", "usage", "worktree"),
}

func ompOptions() map[string]option {
	options := map[string]option{
		"-c":               {argDrop, arityNone},
		"--continue":       {argDrop, arityNone},
		"-r":               {argDrop, arityOptional},
		"--resume":         {argDrop, arityOptional},
		"--restore":        {argDrop, arityOne},
		"--goal":           {argDrop, arityOne},
		"--plan-yolo":      {argDrop, arityNone},
		"--plan-yolo-into": {argDrop, arityOne},
		"--from-claude":    {argDrop, arityNone},
		"--from-codex":     {argDrop, arityNone},
		"--wait-async":     {argDrop, arityNone},
		"--print-thoughts": {argDrop, arityNone},
		"--no-ui":          {argDrop, arityNone},
		"--session-dir":    {argStore, arityOne},
		"--no-session":     {argStore, arityNone},
		"--cwd":            {argDirectory, arityOne},
		"-p":               {argNonInteractive, arityNone},
		"--print":          {argNonInteractive, arityNone},
		"--mode":           {argNonInteractive, arityOne},
		"--export":         {argNonInteractive, arityOne},
		"--alias":          {argNonInteractive, arityOne},
		"-h":               {argNonInteractive, arityNone},
		"--help":           {argNonInteractive, arityNone},
		"-v":               {argNonInteractive, arityNone},
		"--version":        {argNonInteractive, arityNone},
	}
	for _, name := range []string{"--model", "--smol", "--slow", "--plan",
		"--prewalk-into", "--provider", "--api-key", "--system-prompt",
		"--system-prompt-template", "--append-system-prompt", "--profile",
		"--config", "--add-dir", "--models", "--tools", "--thinking",
		"--service-tier", "--hook", "-e", "--extension", "--skills",
		"--max-time", "--warn-cost", "--max-cost", "--approval-mode",
		"--plugin-dir"} {
		options[name] = option{argKeep, arityOne}
	}
	for _, name := range []string{"--prewalk", "--no-prewalk", "--allow-home",
		"--no-tools", "--no-lsp", "--no-pty", "--hide-thinking", "--advisor",
		"--external-thinking", "--no-extensions", "--no-skills", "--no-rules",
		"--no-context-files", "--no-title", "--auto-approve"} {
		options[name] = option{argKeep, arityNone}
	}
	return options
}

func setOf(names ...string) map[string]bool {
	set := make(map[string]bool, len(names))
	for _, name := range names {
		set[name] = true
	}
	return set
}

var profiles = []*Profile{opencode, omp, codex}

// Lookup returns the built-in profile with the given name.
func Lookup(name string) (*Profile, bool) {
	for _, p := range profiles {
		if p.Name == name {
			return p, true
		}
	}
	return nil, false
}
