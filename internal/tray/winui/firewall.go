//go:build windows

package winui

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"unsafe"

	"golang.org/x/sys/windows"

	"github.com/rainchan/webmux/internal/tray"
)

var procShellExecuteExW = shell32.NewProc("ShellExecuteExW")

const (
	seeMaskNoCloseProcess = 0x00000040
	seeMaskFlagNoUI       = 0x00000400
	seeMaskNoConsole      = 0x00008000
)

// shellExecuteInfo is SHELLEXECUTEINFOW; Go's natural alignment matches the
// Windows layout on every supported architecture.
type shellExecuteInfo struct {
	size       uint32
	mask       uint32
	hwnd       windows.HWND
	verb       *uint16
	file       *uint16
	parameters *uint16
	directory  *uint16
	show       int32
	instApp    windows.Handle
	idList     uintptr
	class      *uint16
	keyClass   windows.Handle
	hotKey     uint32
	iconOrMon  windows.Handle
	process    windows.Handle
}

// FirewallCommand runs `webmux-tray firewall allow|remove [PATH]` and
// returns its exit code: the rule belongs to this executable, or for
// remove to PATH. The installer runs it; the tray itself never changes the
// firewall.
func FirewallCommand(args []string) int {
	allow, exe, err := tray.ParseFirewall(args)
	if err != nil {
		return tray.FirewallUsage
	}
	if exe == "" {
		if exe, err = os.Executable(); err != nil {
			return tray.FirewallFailed
		}
		if exe, err = filepath.Abs(exe); err != nil {
			return tray.FirewallFailed
		}
	}
	system, err := windows.GetSystemDirectory()
	if err != nil {
		return tray.FirewallFailed
	}
	netsh := filepath.Join(system, "netsh.exe")
	return tray.Firewall{
		Rule:    tray.FirewallRule{Exe: exe},
		Exists:  func(args []string) (bool, error) { return netshSucceeds(netsh, args) },
		Elevate: func(args []string) (int, error) { return runElevated(netsh, args) },
	}.Apply(allow)
}

// netshSucceeds runs netsh without a console and reports whether it exited
// 0; any other exit code is false, a failure to run it an error.
func netshSucceeds(netsh string, args []string) (bool, error) {
	cmd := exec.Command(netsh, args...)
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true, CreationFlags: windows.CREATE_NO_WINDOW}
	err := cmd.Run()
	var exit *exec.ExitError
	if errors.As(err, &exit) {
		return false, nil
	}
	return err == nil, err
}

// runElevated starts file with args through the "runas" verb, which shows
// the administrator prompt, waits for it and returns its exit code.
func runElevated(file string, args []string) (int, error) {
	quoted := make([]string, len(args))
	for i, a := range args {
		quoted[i] = syscall.EscapeArg(a)
	}
	info := shellExecuteInfo{
		mask:       seeMaskNoCloseProcess | seeMaskFlagNoUI | seeMaskNoConsole,
		verb:       utf16Ptr("runas"),
		file:       utf16Ptr(file),
		parameters: utf16Ptr(strings.Join(quoted, " ")),
		show:       windows.SW_HIDE,
	}
	info.size = uint32(unsafe.Sizeof(info))
	r, _, callErr := procShellExecuteExW.Call(uintptr(unsafe.Pointer(&info)))
	if err := lastError(r, callErr); err != nil {
		if errors.Is(err, windows.ERROR_CANCELLED) {
			return 0, tray.ErrElevationDeclined
		}
		return 0, err
	}
	if info.process == 0 {
		return 0, errors.New("runas started no process")
	}
	defer func() { _ = windows.CloseHandle(info.process) }()
	if _, err := windows.WaitForSingleObject(info.process, windows.INFINITE); err != nil {
		return 0, err
	}
	var code uint32
	if err := windows.GetExitCodeProcess(info.process, &code); err != nil {
		return 0, err
	}
	return int(code), nil
}
