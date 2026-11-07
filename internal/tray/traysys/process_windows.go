//go:build windows

package traysys

import (
	"errors"
	"fmt"
	"strconv"
	"strings"

	"golang.org/x/sys/windows"
)

const stillActive = 259

// Identify returns the creation time of a running process, which changes
// when Windows reuses its PID. It implements tray.Keepalive.Identify.
func Identify(pid int) (string, bool) {
	h, err := windows.OpenProcess(windows.PROCESS_QUERY_LIMITED_INFORMATION, false, uint32(pid))
	if err != nil {
		return "", false
	}
	defer func() { _ = windows.CloseHandle(h) }()
	var code uint32
	if windows.GetExitCodeProcess(h, &code) != nil || code != stillActive {
		return "", false
	}
	var created, exited, kernel, user windows.Filetime
	if windows.GetProcessTimes(h, &created, &exited, &kernel, &user) != nil {
		return "", false
	}
	return strconv.FormatInt(created.Nanoseconds(), 10), true
}

// Kill terminates a process. It implements tray.Keepalive.Kill.
func Kill(pid int) error {
	h, err := windows.OpenProcess(windows.PROCESS_TERMINATE, false, uint32(pid))
	if err != nil {
		return err
	}
	defer func() { _ = windows.CloseHandle(h) }()
	return windows.TerminateProcess(h, 1)
}

func shellExecute(file, args string) error {
	var params *uint16
	if args != "" {
		params = utf16Ptr(args)
	}
	return windows.ShellExecute(0, utf16Ptr("open"), utf16Ptr(file), params, nil, windows.SW_SHOWNORMAL)
}

// OpenURL opens an http(s) link in the default browser.
func OpenURL(url string) error {
	if !strings.HasPrefix(url, "http://") && !strings.HasPrefix(url, "https://") {
		return fmt.Errorf("refusing to open %q", url)
	}
	return shellExecute(url, "")
}

// OpenInNotepad opens a Windows path in Notepad. Windows paths cannot
// contain quotes, so quoting keeps the path one argument.
func OpenInNotepad(path string) error {
	if strings.ContainsRune(path, '"') {
		return fmt.Errorf("invalid path %q", path)
	}
	return shellExecute("notepad.exe", `"`+path+`"`)
}

// WSLPath maps an absolute path inside distro to its \\wsl.localhost path.
func WSLPath(distro, path string) (string, error) {
	if distro == "" {
		return "", errors.New("未知的发行版")
	}
	if !strings.HasPrefix(path, "/") {
		return "", fmt.Errorf("不是 WSL 绝对路径：%s", path)
	}
	return `\\wsl.localhost\` + distro + strings.ReplaceAll(path, "/", `\`), nil
}
