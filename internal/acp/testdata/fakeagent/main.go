// Command fakeagent is a scriptable stand-in for omp, OpenCode, Codex CLI and
// the codex-acp connector in tests and E2E. Build it with
//
//	go build -o <dir>/fakeagent ./internal/acp/testdata/fakeagent
//
// Usage:
//
//	fakeagent [flags]       TUI mode
//	fakeagent [flags] acp   ACP agent on stdio
//
// The profile comes from FAKEAGENT_PROFILE, else from the executable name:
// "opencode" is OpenCode, "codex" and "codex-acp" are Codex, anything else
// omp. Named "codex-acp" it runs in ACP mode without arguments, like the
// connector; "codex --version" (or -V) prints "codex-cli <version>" and exits.
//
// All modes share one session store (--store or FAKEAGENT_STORE), so a
// session written in one mode can be resumed or loaded in the other.
//
// TUI mode prints a transcript and reads keystrokes (raw mode when stdin is
// a terminal). For omp and opencode each session it opens is announced as a
// Warp cli-agent session_start event (agent, plugin_version, session_id,
// cwd) and OSC 7501 root (state, app) and child (id) records; turns report
// working/idle. Enter submits a line; "/new" starts a new session, "/exit"
// quits. The interrupt key stops a turn: Esc for omp and Codex, Esc Esc for
// opencode. --resume ID (alias -r) reopens an existing session; -s ID
// (alias --session) reopens or creates it. Without either, omp and Codex
// create a session at start and opencode at the first message. Like the real
// TUIs, a session started in the terminal is only saved, and so listed, once
// it has a message. SIGHUP, SIGINT and SIGTERM release the lease and exit 0.
//
// The Codex TUI reports only OSC 7501 root records with app=codex: working
// while it starts (--startup-delay), then idle, and working/idle around
// turns; no Warp events and no child id. Session IDs are UUIDv7 strings. It
// holds $CODEX_HOME/thread-writer-locks/<id>.lock (CODEX_HOME defaults to
// ~/.codex) open and flock-ed for every thread it opened, for the life of
// the process: "/new" keeps the previous thread's lock. "resume ID" (after
// any global flags) opens a fresh blank thread's lock and then ID's. A
// further positional argument is a prompt submitted at start.
//
// ACP mode implements initialize, session/new|load|resume|list|close|
// prompt|cancel|set_config_option, $/cancel_request and, for omp and Codex,
// _session/steering. A turn sends a thought, a tool call (with a
// permission request when enabled), optionally a form elicitation, a
// two-chunk agent message and a usage update. Prompt text containing
// /plan, /notice, /compact, /title, /unknown or /fs adds a plan, notice,
// compaction, session_info_update, an unknown update kind, or an
// fs/read_text_file request to the client.
//
// The Codex ACP side mimics codex-acp: agentInfo
// @agentclientprotocol/codex-acp "Codex", options mode (read-only,
// workspace-write, agent, agent-full-access), model (gpt-a, gpt-b) and
// reasoning_effort (low, medium, high), all back at their defaults after
// session/load and session/resume, and commands review and logout. Steering
// with _meta.steering.idleBehavior "promptRequired" and no running turn
// answers {outcome: promptRequired, reason: noRunningTurn}; without it an
// idle steer starts a turn ({outcome: startedNewTurn}); during a turn it
// answers {outcome: injected}. Loading or resuming a thread whose lock
// another live process holds fails with -32600 and data.reason
// "thread_active_writer". At start it logs "env CODEX_PATH=<v>
// NO_BROWSER=<v>" (each only when set, "-" when neither is) to events.log.
//
// Flags (environment variable in brackets):
//
//	--store DIR            session store directory, required [FAKEAGENT_STORE]
//	--profile omp|opencode default omp [FAKEAGENT_PROFILE, which also takes codex];
//	                       under the Codex profile --profile is Codex's own flag
//	--turn-delay DUR       pause between streamed pieces, default 20ms [FAKEAGENT_TURN_DELAY]
//	--startup-delay DUR    Codex TUI: how long it reports working at start, default 0 [FAKEAGENT_STARTUP_DELAY]
//	--delay M=DUR,...      delay ACP method M before answering [FAKEAGENT_DELAY]
//	--fail M=CODE,...      answer ACP method M with JSON-RPC error CODE [FAKEAGENT_FAIL]
//	--crash POINT          exit 3 at POINT: an ACP method name (before its
//	                       answer), "prompt-mid" (after the first chunk) or
//	                       "tui-turn" (during a TUI turn) [FAKEAGENT_CRASH]
//	--permission           ask permission for the turn's tool call [FAKEAGENT_PERMISSION=1]
//	--elicit               ask a form elicitation each turn [FAKEAGENT_ELICIT=1]
//	--ignore-interrupt     TUI mode: the interrupt key does not stop a turn [FAKEAGENT_IGNORE_INTERRUPT=1]
//	--no-resume            ACP mode: do not declare session/resume [FAKEAGENT_NO_RESUME=1]
//	--prompt-image         initialize advertises promptCapabilities.image [FAKEAGENT_PROMPT_IMAGE=1]
//	--embedded-context     initialize advertises promptCapabilities.embeddedContext [FAKEAGENT_EMBEDDED_CONTEXT=1]
//	--protocol-version N   initialize answer, default 1 [FAKEAGENT_PROTOCOL_VERSION]
//	--stdout-noise TEXT    ACP mode: print TEXT as a plain stdout line first [FAKEAGENT_STDOUT_NOISE]
//	(environment only)     Codex --version output, default 0.166.0 [FAKEAGENT_CODEX_VERSION]
//	(environment only)     codex-acp agentInfo.version, default 2.5.0 [FAKEAGENT_CONNECTOR_VERSION]
//
// For omp and opencode, agent flags such as --approval-mode, --yolo,
// --auto-approve, --auto, --config, --model, --thinking, --standalone,
// --login and -c are accepted; --yolo, --auto-approve, --auto and
// --approval-mode yolo suppress permission requests. Other positional
// arguments are ignored. Like omp, the omp profile's session/load answers
// the "mode" option at its default. The Codex profile accepts --no-daemon,
// -m/--model, -s/--sandbox, -a/--ask-for-approval, -c/--config,
// -p/--profile, --search, --dangerously-bypass-approvals-and-sandbox,
// --approve-for-me, --enable, --disable and --add-dir, without effect.
//
// The store's events.log records "<unixnano> <pid> <mode> <event> <id>" for
// open, close, fork and violation (a session opened while another live
// process held it, which also forks it like omp), thread_active_writer (a
// Codex load or resume refused) and "call <method>" for each ACP request.
package main

import (
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

type config struct {
	store            string
	profile          string
	resume           string
	session          string
	prompt           string // Codex TUI: submitted at start
	version          bool   // Codex: print the version and exit
	codexVersion     string
	connectorVersion string
	turnDelay        time.Duration
	startupDelay     time.Duration
	delays           map[string]time.Duration
	fails            map[string]int
	crash            string
	permission       bool
	elicit           bool
	ignoreInterrupt  bool
	noResume         bool
	promptImage      bool
	embeddedContext  bool
	protocolVersion  int
	noise            string
}

const connectorName = "codex-acp"

func main() {
	cfg, acpMode, err := parseArgs(os.Args[1:])
	if err != nil {
		fmt.Fprintln(os.Stderr, "fakeagent:", err)
		os.Exit(2)
	}
	if cfg.version {
		fmt.Println("codex-cli " + cfg.codexVersion)
		return
	}
	mode := "tui"
	if acpMode {
		mode = "acp"
	}
	st, err := openStore(cfg.store, mode, cfg.profile)
	if err != nil {
		fmt.Fprintln(os.Stderr, "fakeagent:", err)
		os.Exit(1)
	}
	if acpMode {
		err = runACP(cfg, st, os.Stdin, os.Stdout)
	} else {
		err = runTUI(cfg, st, os.Stdin, os.Stdout)
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, "fakeagent:", err)
		os.Exit(1)
	}
}

// profileFromName lets the binary stand in for a real agent by name: copied
// or linked as "opencode" it mimics OpenCode, as "codex" or "codex-acp"
// Codex, otherwise omp.
func profileFromName(argv0 string) string {
	switch filepath.Base(argv0) {
	case "opencode":
		return "opencode"
	case "codex", connectorName:
		return "codex"
	}
	return "omp"
}

func parseArgs(args []string) (config, bool, error) {
	cfg := config{
		store:            os.Getenv("FAKEAGENT_STORE"),
		profile:          envOr("FAKEAGENT_PROFILE", profileFromName(os.Args[0])),
		crash:            os.Getenv("FAKEAGENT_CRASH"),
		permission:       os.Getenv("FAKEAGENT_PERMISSION") == "1",
		elicit:           os.Getenv("FAKEAGENT_ELICIT") == "1",
		ignoreInterrupt:  os.Getenv("FAKEAGENT_IGNORE_INTERRUPT") == "1",
		noResume:         os.Getenv("FAKEAGENT_NO_RESUME") == "1",
		promptImage:      os.Getenv("FAKEAGENT_PROMPT_IMAGE") == "1",
		embeddedContext:  os.Getenv("FAKEAGENT_EMBEDDED_CONTEXT") == "1",
		noise:            os.Getenv("FAKEAGENT_STDOUT_NOISE"),
		codexVersion:     envOr("FAKEAGENT_CODEX_VERSION", "0.166.0"),
		connectorVersion: envOr("FAKEAGENT_CONNECTOR_VERSION", "2.5.0"),
		turnDelay:        20 * time.Millisecond,
		protocolVersion:  1,
	}
	var err error
	for _, d := range []struct {
		env string
		to  *time.Duration
	}{{"FAKEAGENT_TURN_DELAY", &cfg.turnDelay}, {"FAKEAGENT_STARTUP_DELAY", &cfg.startupDelay}} {
		if v := os.Getenv(d.env); v != "" {
			if *d.to, err = time.ParseDuration(v); err != nil {
				return cfg, false, fmt.Errorf("%s: %w", d.env, err)
			}
		}
	}
	if v := os.Getenv("FAKEAGENT_PROTOCOL_VERSION"); v != "" {
		if cfg.protocolVersion, err = strconv.Atoi(v); err != nil {
			return cfg, false, fmt.Errorf("FAKEAGENT_PROTOCOL_VERSION: %w", err)
		}
	}
	delays, fails := os.Getenv("FAKEAGENT_DELAY"), os.Getenv("FAKEAGENT_FAIL")
	var approval string
	var yolo, autoApprove, auto bool

	fs := flag.NewFlagSet("fakeagent", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	fs.StringVar(&cfg.store, "store", cfg.store, "")
	fs.DurationVar(&cfg.turnDelay, "turn-delay", cfg.turnDelay, "")
	fs.DurationVar(&cfg.startupDelay, "startup-delay", cfg.startupDelay, "")
	fs.StringVar(&delays, "delay", delays, "")
	fs.StringVar(&fails, "fail", fails, "")
	fs.StringVar(&cfg.crash, "crash", cfg.crash, "")
	fs.BoolVar(&cfg.permission, "permission", cfg.permission, "")
	fs.BoolVar(&cfg.elicit, "elicit", cfg.elicit, "")
	fs.BoolVar(&cfg.ignoreInterrupt, "ignore-interrupt", cfg.ignoreInterrupt, "")
	fs.BoolVar(&cfg.noResume, "no-resume", cfg.noResume, "")
	fs.BoolVar(&cfg.promptImage, "prompt-image", cfg.promptImage, "")
	fs.BoolVar(&cfg.embeddedContext, "embedded-context", cfg.embeddedContext, "")
	fs.IntVar(&cfg.protocolVersion, "protocol-version", cfg.protocolVersion, "")
	fs.StringVar(&cfg.noise, "stdout-noise", cfg.noise, "")
	codex := cfg.profile == "codex"
	if codex {
		// Codex CLI flags the hybrid profile keeps or passes through.
		for _, name := range []string{"no-daemon", "search", "dangerously-bypass-approvals-and-sandbox", "approve-for-me"} {
			fs.Bool(name, false, "")
		}
		for _, name := range []string{"m", "model", "s", "sandbox", "a", "ask-for-approval", "c", "config", "p", "profile", "enable", "disable", "add-dir"} {
			fs.String(name, "", "")
		}
		fs.BoolVar(&cfg.version, "version", false, "")
		fs.BoolVar(&cfg.version, "V", false, "")
	} else {
		fs.StringVar(&cfg.profile, "profile", cfg.profile, "")
		fs.StringVar(&cfg.resume, "resume", "", "")
		fs.StringVar(&cfg.resume, "r", "", "")
		fs.StringVar(&cfg.session, "s", "", "")
		fs.StringVar(&cfg.session, "session", "", "")
		// Real agent flags the hybrid profiles keep or pass through.
		fs.StringVar(&approval, "approval-mode", "", "")
		fs.BoolVar(&yolo, "yolo", false, "")
		fs.BoolVar(&autoApprove, "auto-approve", false, "")
		fs.BoolVar(&auto, "auto", false, "")
		fs.Bool("standalone", false, "")
		fs.Bool("login", false, "")
		fs.Bool("c", false, "")
		fs.Bool("continue", false, "")
		fs.String("config", "", "")
		fs.String("model", "", "")
		fs.String("thinking", "", "")
	}

	// codex-acp takes no arguments: its name alone selects ACP mode.
	acpMode := filepath.Base(os.Args[0]) == connectorName
	positional := 0
	resumeNext := false
	for {
		if err := fs.Parse(args); err != nil {
			return cfg, false, err
		}
		if fs.NArg() == 0 {
			break
		}
		arg := fs.Arg(0)
		positional++
		switch {
		case arg == "acp":
			acpMode = true
		case !codex:
		case resumeNext:
			cfg.resume, resumeNext = arg, false
		case positional == 1 && arg == "resume":
			resumeNext = true
		case cfg.prompt == "":
			cfg.prompt = arg
		}
		args = fs.Args()[1:]
	}
	if cfg.version {
		return cfg, acpMode, nil
	}
	if resumeNext {
		return cfg, false, fmt.Errorf("resume needs a session id")
	}
	if cfg.store == "" {
		return cfg, false, fmt.Errorf("--store or FAKEAGENT_STORE is required")
	}
	if cfg.profile != "omp" && cfg.profile != "opencode" && cfg.profile != "codex" {
		return cfg, false, fmt.Errorf("unknown profile %q", cfg.profile)
	}
	if yolo || autoApprove || auto || approval == "yolo" {
		cfg.permission = false
	}
	if cfg.delays, err = parseMap(delays, time.ParseDuration); err != nil {
		return cfg, false, fmt.Errorf("delay: %w", err)
	}
	if cfg.fails, err = parseMap(fails, strconv.Atoi); err != nil {
		return cfg, false, fmt.Errorf("fail: %w", err)
	}
	return cfg, acpMode, nil
}

func parseMap[T any](spec string, parse func(string) (T, error)) (map[string]T, error) {
	out := map[string]T{}
	for _, item := range strings.Split(spec, ",") {
		if strings.TrimSpace(item) == "" {
			continue
		}
		k, v, ok := strings.Cut(item, "=")
		if !ok {
			return nil, fmt.Errorf("%q is not METHOD=VALUE", item)
		}
		parsed, err := parse(strings.TrimSpace(v))
		if err != nil {
			return nil, fmt.Errorf("%s: %w", k, err)
		}
		out[strings.TrimSpace(k)] = parsed
	}
	return out, nil
}

func envOr(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

func crashAt(cfg config, point string) {
	if cfg.crash == point {
		os.Exit(3)
	}
}
