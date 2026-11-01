package agents_test

import (
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"testing"

	. "github.com/rainchan/webmux/internal/agentprofile"
	"github.com/rainchan/webmux/internal/agentprofile/agents"
	"github.com/rainchan/webmux/internal/agentprofile/codex"
	"github.com/rainchan/webmux/internal/agentprofile/omp"
	"github.com/rainchan/webmux/internal/agentprofile/opencode"
)

var registry = agents.Default()

const (
	OMP      = omp.Name
	OpenCode = opencode.Name
	Codex    = codex.Name
)

var wrapped = func(args ...string) []string {
	return append([]string{"bash", "-ic", `exec -- "$@"`, "webmux"}, args...)
}

func TestCheck(t *testing.T) {
	tests := []struct {
		name   string
		argv   []string
		agent  string
		reason Reason
		arg    string
	}{
		{name: "preset bashrc omp", argv: wrapped("omp"), agent: OMP},
		{name: "preset bashrc opencode", argv: wrapped("opencode"), agent: OpenCode},
		{name: "direct path", argv: []string{"/usr/local/bin/omp", "--model", "haiku"}, agent: OMP},
		{name: "omp profile passes through", argv: []string{"omp", "--profile", "work"}, agent: OMP},
		{name: "custom shell wrapper", argv: []string{"sh", "-c", "omp"}, reason: ReasonUnsupportedForm},
		{name: "env prefix", argv: []string{"env", "A=1", "opencode"}, reason: ReasonUnsupportedForm},
		{name: "other bashrc script", argv: []string{"bash", "-ic", "exec omp", "webmux"}, reason: ReasonUnsupportedForm},
		{name: "plain shell", argv: []string{"/bin/bash", "-l"}, reason: ReasonUnknownAgent},
		{name: "other agent wrapped", argv: wrapped("claude"), reason: ReasonUnknownAgent},
		{name: "empty", argv: nil, reason: ReasonUnknownAgent},
		{name: "opencode directory", argv: []string{"opencode", "/home/u/proj"}, reason: ReasonDirectoryArg, arg: "/home/u/proj"},
		{name: "opencode server", argv: []string{"opencode", "--server", "http://x"}, reason: ReasonSessionStore, arg: "--server"},
		{name: "opencode server inline", argv: []string{"opencode", "--server=http://x"}, reason: ReasonSessionStore, arg: "--server"},
		{name: "opencode version", argv: []string{"opencode", "-v"}, reason: ReasonNonInteractive, arg: "-v"},
		{name: "omp session dir", argv: wrapped("omp", "--session-dir", "/tmp/s"), reason: ReasonSessionStore, arg: "--session-dir"},
		{name: "omp no session", argv: []string{"omp", "--no-session"}, reason: ReasonSessionStore, arg: "--no-session"},
		{name: "omp cwd", argv: []string{"omp", "--cwd=/srv"}, reason: ReasonDirectoryArg, arg: "--cwd"},
		{name: "omp print", argv: []string{"omp", "-p", "总结改动"}, reason: ReasonNonInteractive, arg: "-p"},
		{name: "omp subcommand", argv: []string{"omp", "commit"}, reason: ReasonNonInteractive, arg: "commit"},
		{name: "omp prompt equal to subcommand after prompt", argv: []string{"omp", "fix", "commit"}, agent: OMP},
		{name: "omp after double dash", argv: []string{"omp", "--", "commit"}, agent: OMP},
		{name: "codex preset", argv: wrapped("codex", "--dangerously-bypass-approvals-and-sandbox"), agent: Codex},
		{name: "codex prompt", argv: []string{"codex", "-m", "gpt-5", "fix the bug"}, agent: Codex},
		{name: "codex resume subcommand", argv: []string{"codex", "resume", "--last"}, reason: ReasonSessionArg, arg: "resume"},
		{name: "codex fork subcommand", argv: []string{"codex", "-m", "x", "fork"}, reason: ReasonSessionArg, arg: "fork"},
		{name: "codex exec", argv: []string{"codex", "exec", "修 bug"}, reason: ReasonNonInteractive, arg: "exec"},
		{name: "codex migrate prefix", argv: []string{"codex", "migrate-rollouts"}, reason: ReasonNonInteractive, arg: "migrate-rollouts"},
		{name: "codex version", argv: []string{"codex", "-V"}, reason: ReasonNonInteractive, arg: "-V"},
		{name: "codex remote", argv: []string{"codex", "--remote", "ws://h:1"}, reason: ReasonSessionStore, arg: "--remote"},
		{name: "codex cd", argv: []string{"codex", "-C", "/srv"}, reason: ReasonDirectoryArg, arg: "-C"},
		{name: "codex worktree", argv: []string{"codex", "--worktree"}, reason: ReasonDirectoryArg, arg: "--worktree"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := registry.Check(tt.argv)
			want := Support{Supported: tt.agent != "", Agent: tt.agent, Reason: tt.reason, Agents: registry.Titles()}
			if tt.agent == Codex {
				want.Connector = codex.Agent.Profile().Connector
			}
			if !reflect.DeepEqual(got, want) {
				t.Fatalf("Check(%q) = %+v, want %+v", tt.argv, got, want)
			}
			if tt.reason == "" {
				return
			}
			_, err := registry.Parse(tt.argv)
			var unsupported *UnsupportedError
			if !errors.As(err, &unsupported) || unsupported.Arg != tt.arg {
				t.Fatalf("registry.Parse(%q) error = %v, want arg %q", tt.argv, err, tt.arg)
			}
		})
	}
}

func TestResumeArgv(t *testing.T) {
	tests := []struct {
		name    string
		argv    []string
		id      string
		want    []string
		dropped []string
	}{
		{
			name:    "drop prompt and session options",
			argv:    []string{"omp", "--model", "haiku", "--continue", "修 bug"},
			id:      "01abc",
			want:    []string{"omp", "--model", "haiku", "--resume", "01abc"},
			dropped: []string{"--continue", "修 bug"},
		},
		{
			name:    "opencode standalone injected",
			argv:    []string{"opencode", "--prompt", "hi"},
			id:      "ses_1",
			want:    []string{"opencode", "--standalone", "-s", "ses_1"},
			dropped: []string{"--prompt hi"},
		},
		{
			name: "opencode standalone not duplicated",
			argv: []string{"opencode", "--auto", "--standalone", "--log-level=debug"},
			id:   "ses_2",
			want: []string{"opencode", "--auto", "--standalone", "--log-level=debug", "-s", "ses_2"},
		},
		{
			name:    "inline and separate values",
			argv:    []string{"omp", "--model=haiku", "--thinking", "low", "--resume=old", "-r", "--no-lsp"},
			id:      "01x",
			want:    []string{"omp", "--model=haiku", "--thinking", "low", "--no-lsp", "--resume", "01x"},
			dropped: []string{"--resume=old", "-r"},
		},
		{
			name:    "optional value consumed",
			argv:    []string{"omp", "-r", "01old", "--advisor"},
			id:      "01new",
			want:    []string{"omp", "--advisor", "--resume", "01new"},
			dropped: []string{"-r 01old"},
		},
		{
			name:    "unknown options dropped",
			argv:    []string{"omp", "--frobnicate", "x", "-e", "ext.ts"},
			id:      "01y",
			want:    []string{"omp", "-e", "ext.ts", "--resume", "01y"},
			dropped: []string{"--frobnicate", "x"},
		},
		{
			name: "bashrc form kept",
			argv: wrapped("omp", "--profile", "work"),
			id:   "01z",
			want: wrapped("omp", "--profile", "work", "--resume", "01z"),
		},
		{
			name:    "codex resume subcommand after global options",
			argv:    []string{"codex", "-m", "gpt-5.6-luna", "-s", "read-only", "修 bug"},
			id:      "01a18715",
			want:    []string{"codex", "--no-daemon", "-m", "gpt-5.6-luna", "-s", "read-only", "resume", "01a18715"},
			dropped: []string{"修 bug"},
		},
		{
			name:    "codex image dropped",
			argv:    wrapped("codex", "--no-daemon", "-i", "a.png", "--search"),
			id:      "01b",
			want:    wrapped("codex", "--no-daemon", "--search", "resume", "01b"),
			dropped: []string{"-i a.png"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cmd, err := registry.Parse(tt.argv)
			if err != nil {
				t.Fatal(err)
			}
			got, dropped := cmd.ResumeArgv(tt.id)
			if !slices.Equal(got, tt.want) || !slices.Equal(dropped, tt.dropped) {
				t.Fatalf("ResumeArgv = %q dropped %q, want %q dropped %q", got, dropped, tt.want, tt.dropped)
			}
		})
	}
}

func TestTUIArgv(t *testing.T) {
	tests := []struct {
		argv []string
		want []string
	}{
		{[]string{"opencode", "--prompt", "hi"}, []string{"opencode", "--standalone", "--prompt", "hi"}},
		{[]string{"opencode", "--standalone"}, []string{"opencode", "--standalone"}},
		{wrapped("opencode"), wrapped("opencode", "--standalone")},
		{[]string{"omp", "--continue", "hi"}, []string{"omp", "--continue", "hi"}},
		{wrapped("codex", "-m", "x"), wrapped("codex", "--no-daemon", "-m", "x")},
	}
	for _, tt := range tests {
		cmd, err := registry.Parse(tt.argv)
		if err != nil {
			t.Fatal(err)
		}
		if got := cmd.TUIArgv(); !slices.Equal(got, tt.want) {
			t.Errorf("TUIArgv(%q) = %q, want %q", tt.argv, got, tt.want)
		}
	}
}

func TestACPArgv(t *testing.T) {
	tests := []struct {
		name        string
		argv        []string
		want        []string
		unsupported []string
		fd3         bool
	}{
		{
			name: "omp passes kept options",
			argv: []string{"omp", "--profile", "work", "--model", "haiku", "修 bug"},
			want: []string{"omp", "acp", "--profile", "work", "--model", "haiku"},
		},
		{
			name: "opencode accepts no options",
			argv: []string{"opencode", "--auto", "--log-level", "debug"},
			want: []string{"opencode", "acp"},
		},
		{
			name: "bashrc writes protocol to fd 3",
			argv: wrapped("omp", "--auto-approve"),
			want: []string{"bash", "-ic", `exec -- "$@" 1>&3 3>&-`, "webmux", "omp", "acp", "--auto-approve"},
			fd3:  true,
		},
		{
			name:        "codex runs its connector without arguments",
			argv:        []string{"/opt/codex/bin/codex", "--search", "-m", "gpt-5", "-c", "model_reasoning_effort=high", "-c", "x=1", "-s", "danger-full-access", "-a", "never"},
			want:        []string{"codex-acp"},
			unsupported: []string{"--search", "-c x=1", "-s danger-full-access", "-a never"},
		},
		{
			name: "codex connector through bashrc",
			argv: wrapped("codex", "--dangerously-bypass-approvals-and-sandbox"),
			want: []string{"bash", "-ic", `exec -- "$@" 1>&3 3>&-`, "webmux", "codex-acp"},
			fd3:  true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cmd, err := registry.Parse(tt.argv)
			if err != nil {
				t.Fatal(err)
			}
			got, unsupported := cmd.ACPArgv()
			if !slices.Equal(got, tt.want) || !slices.Equal(unsupported, tt.unsupported) || cmd.ProtocolOnFD3() != tt.fd3 {
				t.Fatalf("ACPArgv = %q %q fd3=%v, want %q %q fd3=%v", got, unsupported, cmd.ProtocolOnFD3(), tt.want, tt.unsupported, tt.fd3)
			}
		})
	}
}

func TestIdentify(t *testing.T) {
	tests := []struct {
		name string
		in   Signals
		want Identity
		ok   bool
	}{
		{
			name: "omp from warp",
			in:   Signals{WarpAgent: "omp", WarpPluginVersion: "18.13.1", WarpSessionID: "01a", WarpCwd: "/p", ProgramApp: "omp", RootCwd: "/other"},
			want: Identity{Agent: OMP, Version: "18.13.1", AgentSessionID: "01a", Cwd: "/p"},
			ok:   true,
		},
		{
			name: "omp before first event",
			in:   Signals{ProgramApp: "omp"},
			want: Identity{Agent: OMP},
			ok:   true,
		},
		{
			name: "opencode child record and root cwd",
			in:   Signals{ProgramApp: "opencode", ProgramChildID: "ses_e7fb", RootCwd: "/p"},
			want: Identity{Agent: OpenCode, AgentSessionID: "ses_e7fb", Cwd: "/p"},
			ok:   true,
		},
		{
			name: "opencode ignores other child ids",
			in:   Signals{ProgramApp: "opencode", ProgramChildID: "task-7", RootCwd: "/p"},
			want: Identity{Agent: OpenCode, Cwd: "/p"},
			ok:   true,
		},
		{
			name: "codex names no session in its signals",
			in:   Signals{ProgramApp: "codex", RootCwd: "/p"},
			want: Identity{Agent: Codex, Cwd: "/p"},
			ok:   true,
		},
		{name: "unknown", in: Signals{ProgramApp: "claude-code", WarpSessionID: "x"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, ok := registry.Identify(tt.in)
			if got != tt.want || ok != tt.ok {
				t.Fatalf("Identify = %+v %v, want %+v %v", got, ok, tt.want, tt.ok)
			}
		})
	}
}

func TestBelowVerified(t *testing.T) {
	ompProfile := omp.Agent.Profile()
	opencodeProfile := opencode.Agent.Profile()
	tests := []struct {
		profile *Profile
		version string
		want    bool
	}{
		{ompProfile, "18.0.0", true},
		{ompProfile, "v18.13.1", false},
		{ompProfile, "18.13", true},
		{ompProfile, "18.13.1-beta.2", false},
		{ompProfile, "19", false},
		{ompProfile, "dev", false},
		{ompProfile, "", false},
		{opencodeProfile, "2.2.0", false},
		{opencodeProfile, "2.1.3", true},
	}
	for _, tt := range tests {
		if got := tt.profile.BelowVerified(tt.version); got != tt.want {
			t.Errorf("%s.BelowVerified(%q) = %v, want %v", tt.profile.Name, tt.version, got, tt.want)
		}
	}
}

func TestCodexChatConfig(t *testing.T) {
	tests := []struct {
		name        string
		argv        []string
		seeds       []Seed
		unsupported []string
	}{
		{
			name:  "preset full access",
			argv:  wrapped("codex", "--dangerously-bypass-approvals-and-sandbox"),
			seeds: []Seed{{ConfigID: "mode", Value: "agent-full-access", Arg: "--dangerously-bypass-approvals-and-sandbox"}},
		},
		{
			name: "model, quoted effort and sandbox",
			argv: []string{"codex", "--model=gpt-5.6-luna", "-c", `model_reasoning_effort="high"`, "-s", "read-only"},
			seeds: []Seed{
				{ConfigID: "model", Value: "gpt-5.6-luna", Arg: "--model=gpt-5.6-luna"},
				{ConfigID: "reasoning_effort", Value: "high", Arg: `-c model_reasoning_effort="high"`},
				{ConfigID: "mode", Value: "read-only", Arg: "-s read-only"},
			},
		},
		{
			name:  "first approval option in priority order wins",
			argv:  []string{"codex", "-s", "workspace-write", "--approve-for-me"},
			seeds: []Seed{{ConfigID: "mode", Value: "agent", Arg: "--approve-for-me"}},
		},
		{
			name:        "underivable approval and chat-only options",
			argv:        []string{"codex", "-s", "danger-full-access", "-a", "on-request", "--profile", "work", "--enable", "x"},
			unsupported: []string{"-s danger-full-access", "-a on-request", "--profile work", "--enable x"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cmd, err := registry.Parse(tt.argv)
			if err != nil {
				t.Fatal(err)
			}
			seeds, unsupported := cmd.ChatConfig()
			if !slices.Equal(seeds, tt.seeds) || !slices.Equal(unsupported, tt.unsupported) {
				t.Fatalf("ChatConfig = %+v %q, want %+v %q", seeds, unsupported, tt.seeds, tt.unsupported)
			}
		})
	}
	if seeds, unsupported := mustParse(t, "omp", "--model", "haiku").ChatConfig(); seeds != nil || unsupported != nil {
		t.Fatalf("omp derives %+v %q", seeds, unsupported)
	}
}

func mustParse(t *testing.T, argv ...string) Command {
	t.Helper()
	cmd, err := registry.Parse(argv)
	if err != nil {
		t.Fatal(err)
	}
	return cmd
}

func TestLockedSessions(t *testing.T) {
	// Open files are reported with symbolic links resolved, the
	// temporary directory's included.
	home, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	locks := filepath.Join(home, "codex", "thread-writer-locks")
	if err := os.MkdirAll(locks, 0o700); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(t.TempDir(), "codex-home")
	if err := os.Symlink(filepath.Dir(locks), link); err != nil {
		t.Fatal(err)
	}
	codexLocks := codex.Agent.(LockTracker)
	const a, b = "01a1953f-5c3a-7280-aa68-7ff5b7f7dab4", "01a1953f-8342-7e43-acfe-1b5d26be5c75"
	paths := []string{
		"/usr/lib/libc.so",
		filepath.Join(locks, ".coordination.lock"),
		filepath.Join(locks, b+".lock"),
		filepath.Join(home, "other", a+".lock"),
		filepath.Join(locks, a+".lock"),
	}
	env := func(v string) func(string) string {
		return func(name string) string {
			if name == "CODEX_HOME" {
				return v
			}
			return ""
		}
	}
	// CODEX_HOME through a symbolic link: open files are reported resolved.
	if got := codexLocks.LockedSessions(paths, env(link), "/nonexistent"); !slices.Equal(got, []string{b, a}) {
		t.Fatalf("locked sessions %q", got)
	}
	// The default home is .codex under the user's home.
	def := filepath.Join(home, ".codex", "thread-writer-locks", a+".lock")
	if got := codexLocks.LockedSessions([]string{def}, env(""), home); !slices.Equal(got, []string{a}) {
		t.Fatalf("default home %q", got)
	}
	if _, ok := omp.Agent.(LockTracker); ok {
		t.Fatal("omp tracks locks")
	}
}
