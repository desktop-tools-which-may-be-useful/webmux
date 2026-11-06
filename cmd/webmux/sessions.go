package main

import (
	"context"
	"flag"
	"fmt"
	"github.com/rainchan/webmux/internal/contract"
	"io"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"runtime"
	"strings"
	"syscall"

	"github.com/rainchan/webmux/internal/attach"
	"github.com/rainchan/webmux/internal/client"
	"github.com/rainchan/webmux/internal/config"
	"github.com/rainchan/webmux/internal/localclient"
	"github.com/rainchan/webmux/internal/sessioncmd"
)

// sessionEnv connects the shared session commands to this machine's
// server through its local socket.
func sessionEnv(stdout, stderr io.Writer) sessioncmd.Env {
	path := config.DefaultPath()
	explicit := false
	socket := ""
	return sessioncmd.Env{
		Flags: func(flags *flag.FlagSet) {
			flags.Func("config", "TOML configuration `PATH` (default "+path+")", func(value string) error {
				path, explicit = value, true
				return nil
			})
			flags.Func("socket", "connect to the local API socket at `PATH` instead of locating the server", func(value string) error {
				socket = value
				return nil
			})
		},
		Connect: func() (*localclient.Client, error) {
			return connectLocal(socket, path, explicit, os.Getenv)
		},
		Stdin:          os.Stdin,
		Stdout:         stdout,
		Stderr:         stderr,
		OpenConsole:    func() (sessioncmd.Console, error) { return attach.OpenConsole(func() { os.Exit(1) }) },
		OpenBrowser:    openBrowser,
		ResolveCwd:     resolveCwd,
		Environ:        os.Environ,
		Getenv:         os.Getenv,
		NotRunningHint: "start it with `webmux serve` (or your service manager)",
		Label:          "terminal",
		Platform:       "unix",
		Version:        version,
	}
}

// connectLocal picks the local API socket: --socket, then --config, then
// WEBMUX_SOCKET (set in every session), then the default configuration. A
// named socket that does not answer is reported as not running; falling
// back could reach another instance and act on the wrong sessions.
func connectLocal(socket, configPath string, configExplicit bool, getenv func(string) string) (*localclient.Client, error) {
	if socket != "" && configExplicit {
		return nil, sessioncmd.UsageError("--socket and --config cannot be combined")
	}
	if socket != "" {
		return localclient.New(localclient.UnixDialer{Path: socket}), nil
	}
	if !configExplicit {
		if fromEnv := getenv(contract.SocketEnv); fromEnv != "" {
			return localclient.New(localclient.UnixDialer{Path: fromEnv}), nil
		}
	}
	target := client.Locate(configPath, configExplicit)
	if target.ConfigErr != nil {
		return nil, fmt.Errorf("configuration: %w", target.ConfigErr)
	}
	return target.LocalClient(), nil
}

// resolveCwd keeps "~" for the server to expand and makes other paths
// absolute; the default is the current directory.
func resolveCwd(_ context.Context, cwd string) (string, error) {
	if cwd == "~" || strings.HasPrefix(cwd, "~/") {
		return cwd, nil
	}
	if cwd == "" {
		return os.Getwd()
	}
	return filepath.Abs(cwd)
}

func openBrowser(url string) error {
	opener := "xdg-open"
	if runtime.GOOS == "darwin" {
		opener = "open"
	}
	return exec.Command(opener, url).Start()
}

// sessionCommand runs a shared session command. SIGINT and SIGTERM end
// the command (attachments restore the terminal first); in raw mode Ctrl+C
// is input for the session instead. ok is false when args name no session
// command.
func sessionCommand(args []string, stdout, stderr io.Writer) (int, bool) {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM, syscall.SIGHUP)
	defer stop()
	return sessionEnv(stdout, stderr).Dispatch(ctx, sessioncmd.Commands(), args)
}
