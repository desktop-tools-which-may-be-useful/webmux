//go:build windows

// Command webmux-windows (installed as webmux.exe) runs agents in WebMux
// inside WSL from PowerShell or Windows Terminal. Every connection is a
// `wsl.exe --exec webmux bridge` process speaking the local API, so no token
// or WSL networking setting is involved.
package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"os/signal"
	"strings"
	"time"

	"golang.org/x/sys/windows"

	"github.com/rainchan/webmux/internal/attach"
	"github.com/rainchan/webmux/internal/contract"
	"github.com/rainchan/webmux/internal/localclient"
	"github.com/rainchan/webmux/internal/sessioncmd"
	"github.com/rainchan/webmux/internal/wslclient"
)

var version = "dev"

const usage = "usage: webmux [--distro NAME] [--webmux PATH] COMMAND [ARGS]\nRun 'webmux help' for the list of commands.\n"

// windowsCommands are the commands only this client has; the session
// commands come from sessioncmd.
func windowsCommands() sessioncmd.Registry {
	exits := []sessioncmd.ExitItem{{Code: 0, Text: "Done."}, {Code: 1, Text: "Failed."}, {Code: 3, Text: "WebMux in WSL is not running."}}
	return sessioncmd.Registry{
		{
			Name: "status", Group: sessioncmd.GroupServer, Platform: "windows", NoGlobal: true,
			Summary: "Show the status of WebMux inside WSL.",
			Bind: func(fs *flag.FlagSet) sessioncmd.Action {
				fs.Bool("json", false, "print the status contract v1 as JSON")
				return nil
			},
			Output: "A human-readable summary, or the StatusV1 object with --json.", Exit: exits,
			Examples: []sessioncmd.Example{
				{Command: "webmux status", Text: "Show whether WebMux runs in WSL."},
				{Command: "webmux --distro Ubuntu status --json", Text: "Check another distribution."},
			},
		},
		{
			Name: "version", Group: sessioncmd.GroupServer, Platform: "windows", NoGlobal: true,
			Bind:    func(*flag.FlagSet) sessioncmd.Action { return nil },
			Summary: "Print the version of this Windows client.", Output: "webmux-windows VERSION.", Exit: exits[:1],
			Examples: []sessioncmd.Example{
				{Command: "webmux version", Text: "Print the client version."},
				{Command: "webmux status", Text: "Show the server version instead."},
			},
		},
		{
			Name: "help", Group: sessioncmd.GroupHelp, Platform: "windows", NoGlobal: true, Args: "[COMMAND...]",
			Bind: func(fs *flag.FlagSet) sessioncmd.Action {
				fs.Bool("json", false, "print the commands, options and examples as JSON")
				return nil
			},
			Summary: "Show help for all commands or one command; --json prints the command tree.",
			Output:  "Help text, or the HelpTree object with --json.",
			Exit:    []sessioncmd.ExitItem{{Code: 0, Text: "Printed."}, {Code: 2, Text: "Unknown command."}},
			Examples: []sessioncmd.Example{
				{Command: "webmux help run", Text: "Show the help of webmux run."},
				{Command: "webmux help --json", Text: "Print the command tree."},
			},
		},
	}
}

// helpRequested reports a help flag before any "--".
func helpRequested(args []string) bool {
	for _, arg := range args {
		if arg == "--" {
			return false
		}
		if arg == "-h" || arg == "--help" || arg == "-help" {
			return true
		}
	}
	return false
}

func main() {
	os.Exit(run(os.Args[1:], os.Stdout, os.Stderr))
}

func run(args []string, stdout, stderr io.Writer) int {
	global := flag.NewFlagSet("webmux", flag.ContinueOnError)
	// The flag package calls Usage on -h and on every parse error; usage is
	// printed explicitly below instead, so help leaves stderr empty and an
	// error prints it once.
	global.Usage = func() {}
	distro := global.String("distro", "", "WSL distribution (default: WEBMUX_DISTRO, the tray setting, then WSL's default)")
	webmux := global.String("webmux", "", "webmux inside WSL (default: WEBMUX_WSL_PATH, the tray setting, then ~/.local/bin/webmux)")
	registry := append(windowsCommands(), sessioncmd.Commands()...)
	help := sessioncmd.Env{Stdout: stdout, Stderr: stderr, Platform: "windows", Version: version, Commands: registry,
		HelpFooter: "Global options before COMMAND: --distro NAME selects the WSL distribution, --webmux PATH the webmux inside it."}
	// Help before any command (webmux --help, -h, -help) is the overview on
	// standard output, as on Linux and macOS.
	global.SetOutput(io.Discard)
	if err := global.Parse(args); errors.Is(err, flag.ErrHelp) {
		return help.HelpCommand(registry, version, nil)
	} else if err != nil {
		_, _ = fmt.Fprintln(stderr, "webmux:", err)
		_, _ = fmt.Fprint(stderr, usage)
		return contract.ExitUsage
	}
	if global.NArg() == 0 {
		_, _ = fmt.Fprint(stderr, usage)
		return contract.ExitUsage
	}
	command, rest := global.Arg(0), global.Args()[1:]
	if command == "help" {
		return help.HelpCommand(registry, version, rest)
	}
	// Shared commands handle help in their own parser; the Windows-only ones
	// take it anywhere before "--".
	if c, more := registry.Find(global.Args()); c != nil && c.Platform == "windows" && helpRequested(more) {
		help.PrintHelp(c)
		return contract.ExitOK
	}
	if command == "version" {
		_, _ = fmt.Fprintln(stdout, "webmux-windows", version)
		return contract.ExitOK
	}
	target := wslclient.ResolveTarget(*distro, *webmux, os.Getenv, wslclient.TraySettingsPath(os.Getenv("APPDATA")))
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()
	c := &client{target: target, stdout: stdout, stderr: stderr}
	e := sessioncmd.Env{
		Connect:        c.connect,
		Stdin:          os.Stdin,
		Stdout:         stdout,
		Stderr:         stderr,
		OpenConsole:    func() (sessioncmd.Console, error) { return attach.OpenConsole(func() { os.Exit(1) }) },
		OpenBrowser:    openBrowser,
		ResolveCwd:     c.resolveCwd,
		NotRunningHint: "start it from the WebMux tray or run `webmux serve` inside WSL",
		Label:          "Windows console",
		Platform:       "windows",
		Version:        version,
	}
	if command == "status" {
		return c.status(ctx, rest)
	}
	if code, ok := e.Dispatch(ctx, sessioncmd.Commands(), global.Args()); ok {
		return code
	}
	_, _ = fmt.Fprintf(stderr, "webmux: unknown command %q\n", command)
	_, _ = fmt.Fprint(stderr, usage)
	return contract.ExitUsage
}

type client struct {
	target         wslclient.Target
	stdout, stderr io.Writer
}

// connect verifies webmux once, then dials through bridges. An older
// webmux treats every unknown first argument as `serve`, so `bridge` must
// never reach one: its flag list is checked first.
func (c *client) connect() (*localclient.Client, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	cmd := wslclient.NewCommand(c.target.Command("-h"))
	var output bytes.Buffer
	cmd.Stdout, cmd.Stderr = &output, &output
	if err := runContext(ctx, cmd); err != nil && output.Len() == 0 {
		return nil, fmt.Errorf("cannot run wsl.exe: %w", err)
	}
	text := wslclient.DecodeOutput(output.Bytes())
	switch {
	case strings.Contains(text, "-log-file"):
	case strings.Contains(strings.ToLower(text), "no such file") || strings.Contains(text, "execvpe"):
		return nil, fmt.Errorf("webmux not found in WSL at %s (set --webmux or WEBMUX_WSL_PATH)", c.target.Webmux)
	default:
		return nil, errors.New("the webmux inside WSL is too old for the local API; upgrade it")
	}
	return localclient.New(&wslclient.BridgeDialer{Command: func() *exec.Cmd {
		return wslclient.NewCommand(c.target.Bridge())
	}}), nil
}

func runContext(ctx context.Context, cmd *exec.Cmd) error {
	if err := cmd.Start(); err != nil {
		return err
	}
	done := make(chan error, 1)
	go func() { done <- cmd.Wait() }()
	select {
	case err := <-done:
		return err
	case <-ctx.Done():
		_ = cmd.Process.Kill()
		<-done
		return ctx.Err()
	}
}

// resolveCwd maps the Windows working directory (or --cwd) into the
// target distribution; unmappable directories are a usage error.
func (c *client) resolveCwd(ctx context.Context, cwd string) (string, error) {
	base, err := os.Getwd()
	if err != nil {
		return "", err
	}
	if cwd == "" {
		cwd = base
	}
	distro := c.target.Distro
	if distro == "" && strings.HasPrefix(cwd, `\\`) {
		if distro, err = defaultDistro(ctx); err != nil {
			return "", err
		}
	}
	linux, drive, err := wslclient.MapPath(cwd, base, distro)
	if err != nil || linux != "" {
		return linux, err
	}
	cmd := wslclient.NewCommand(c.target.Wslpath(drive))
	var out, errOut bytes.Buffer
	cmd.Stdout, cmd.Stderr = &out, &errOut
	if err := runContext(ctx, cmd); err != nil {
		return "", fmt.Errorf("wslpath %s: %v %s", drive, err, strings.TrimSpace(wslclient.DecodeOutput(errOut.Bytes())))
	}
	return strings.TrimSpace(wslclient.DecodeOutput(out.Bytes())), nil
}

func defaultDistro(ctx context.Context) (string, error) {
	cmd := wslclient.NewCommand(wslclient.ListVerbose())
	var out bytes.Buffer
	cmd.Stdout = &out
	if err := runContext(ctx, cmd); err != nil {
		return "", fmt.Errorf("wsl.exe --list --verbose: %w", err)
	}
	for _, d := range wslclient.ParseListVerbose(out.Bytes()) {
		if d.Default {
			return d.Name, nil
		}
	}
	return "", errors.New("no default WSL distribution")
}

// status uses the local API, or `webmux status --json` inside WSL when the
// server is not running.
func (c *client) status(ctx context.Context, args []string) int {
	flags := flag.NewFlagSet("status", flag.ContinueOnError)
	flags.SetOutput(c.stderr)
	asJSON := flags.Bool("json", false, "print the status contract v1 as JSON")
	if err := flags.Parse(args); err != nil || flags.NArg() > 0 {
		return contract.ExitUsage
	}
	local, err := c.connect()
	if err != nil {
		_, _ = fmt.Fprintln(c.stderr, "webmux:", err)
		return contract.ExitError
	}
	defer local.Close()
	requestCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	status, err := local.Status(requestCtx)
	cancel()
	code := contract.ExitOK
	if errors.Is(err, localclient.ErrNotRunning) {
		cmd := wslclient.NewCommand(c.target.Command("status", "--json"))
		var out bytes.Buffer
		cmd.Stdout, cmd.Stderr = &out, c.stderr
		runErr := runContext(ctx, cmd)
		var exit *exec.ExitError
		if runErr != nil && !errors.As(runErr, &exit) {
			_, _ = fmt.Fprintln(c.stderr, "webmux:", runErr)
			return contract.ExitError
		}
		code = cmd.ProcessState.ExitCode()
		if *asJSON {
			_, _ = c.stdout.Write(out.Bytes())
			return code
		}
		if err := json.Unmarshal(out.Bytes(), &status); err != nil {
			_, _ = fmt.Fprintln(c.stderr, "webmux: unreadable status:", err)
			return contract.ExitError
		}
	} else if err != nil {
		_, _ = fmt.Fprintln(c.stderr, "webmux:", err)
		return contract.ExitError
	} else if status.State != contract.StateRunning || status.Health.Status != contract.HealthOK {
		code = contract.ExitError
	}
	if *asJSON {
		encoder := json.NewEncoder(c.stdout)
		encoder.SetIndent("", "  ")
		_ = encoder.Encode(status)
		return code
	}
	sessioncmd.PrintStatus(c.stdout, status)
	return code
}

func openBrowser(url string) error {
	verb, _ := windows.UTF16PtrFromString("open")
	target, err := windows.UTF16PtrFromString(url)
	if err != nil {
		return err
	}
	return windows.ShellExecute(0, verb, target, nil, nil, windows.SW_SHOWNORMAL)
}
