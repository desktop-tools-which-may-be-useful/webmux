package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"runtime"
	"syscall"

	"github.com/rainchan/webmux/internal/contract"
	"github.com/rainchan/webmux/internal/update"
)

const installScriptURL = "https://github.com/" + update.Repo + "/releases/latest/download/install.sh"

// updateEnv holds what the update commands read from the system, so tests
// can replace it.
type updateEnv struct {
	isRelease func() bool
	exe       func() (string, error)
	run       update.Runner
	client    update.Client
	euid      int
	lockPath  string
	cacheDir  string
}

func systemUpdateEnv() updateEnv {
	cache := os.Getenv("CACHE_DIRECTORY") // set by systemd CacheDirectory=
	if cache == "" {
		cache = "/var/cache/webmux-update"
	}
	return updateEnv{
		isRelease: update.IsRelease,
		exe:       installedExecutable,
		run:       runCombined,
		euid:      os.Geteuid(),
		lockPath:  "/run/webmux-update.lock",
		cacheDir:  cache,
	}
}

func installedExecutable() (string, error) {
	exe, err := os.Executable()
	if err != nil {
		return "", err
	}
	return filepath.EvalSymlinks(exe)
}

// runCombined starts each program in its own process group, so a Ctrl-C at
// the terminal reaches only webmux, which never interrupts dpkg or rpm.
func runCombined(ctx context.Context, name string, args ...string) ([]byte, error) {
	cmd := exec.CommandContext(ctx, name, args...)
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	return cmd.CombinedOutput()
}

func updateCommand(args []string, stdout, stderr io.Writer) int {
	return runUpdate(args, stdout, stderr, systemUpdateEnv())
}

func runUpdate(args []string, stdout, stderr io.Writer, env updateEnv) int {
	if len(args) == 0 || args[0] == "--help" || args[0] == "-h" {
		_, _ = fmt.Fprintln(stdout, "Usage: webmux update <check [--json]|apply>")
		return contract.ExitOK
	}
	action := args[0]
	if action != "check" && action != "apply" {
		_, _ = fmt.Fprintf(stderr, "webmux update: unknown command %q\n", action)
		return contract.ExitUsage
	}
	flags := flag.NewFlagSet("webmux update "+action, flag.ContinueOnError)
	flags.SetOutput(stderr)
	asJSON := false
	if action == "check" {
		flags.BoolVar(&asJSON, "json", false, "print the result as JSON")
	}
	if err := flags.Parse(args[1:]); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return contract.ExitOK
		}
		return contract.ExitUsage
	}
	if flags.NArg() != 0 {
		_, _ = fmt.Fprintf(stderr, "webmux update: unexpected argument %q\n", flags.Arg(0))
		return contract.ExitUsage
	}
	if !env.isRelease() {
		_, _ = fmt.Fprintln(stderr, "webmux update:", update.ErrNotRelease)
		return contract.ExitError
	}
	// Apply bounds its own network phase and lets a started install finish.
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	exe, err := env.exe()
	if err != nil {
		_, _ = fmt.Fprintln(stderr, "webmux update:", err)
		return contract.ExitError
	}
	if action == "apply" {
		err := update.Apply(ctx, update.ApplyOptions{
			Current: version, Exe: exe, GOARCH: runtime.GOARCH, Euid: env.euid,
			LockPath: env.lockPath, CacheDir: env.cacheDir,
			Client: env.client, Run: env.run, Out: stdout,
		})
		if err != nil {
			_, _ = fmt.Fprintln(stderr, "webmux update:", err)
			return contract.ExitError
		}
		return contract.ExitOK
	}
	method := update.DetectMethod(ctx, env.run, exe)
	checkCtx, cancel := context.WithTimeout(ctx, update.DownloadTimeout)
	defer cancel()
	m, host, err := env.client.Latest(checkCtx)
	if err != nil {
		_, _ = fmt.Fprintln(stderr, "webmux update: 检查更新失败：", err)
		return contract.ExitError
	}
	available := update.Newer(m.Version, version)
	if asJSON {
		writeJSON(stdout, contract.UpdateCheckV1{V: contract.V, Current: version, Latest: m.Version, Available: available, InstallMethod: string(method)})
		return contract.ExitOK
	}
	_, _ = fmt.Fprintf(stdout, "当前版本：%s\n最新版本：%s（来源 %s）\n", version, m.Version, host)
	if !available {
		_, _ = fmt.Fprintln(stdout, "已是最新版本。")
		return contract.ExitOK
	}
	_, _ = fmt.Fprintln(stdout, "有可用更新。")
	if method == update.MethodOther {
		_, _ = fmt.Fprintf(stdout, "当前安装方式不支持自动更新；重新运行 install.sh 升级：\n  curl -fsSL %s | sh\n", installScriptURL)
	} else {
		_, _ = fmt.Fprintln(stdout, "系统定时任务 webmux-update.timer 会自动安装；立即安装：sudo webmux update apply")
	}
	return contract.ExitOK
}
