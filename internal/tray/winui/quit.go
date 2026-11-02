//go:build windows

package winui

import (
	"errors"
	"unsafe"

	"golang.org/x/sys/windows"

	"github.com/rainchan/webmux/internal/tray"
)

// QuitCommand runs `webmux-tray quit [--kill-path PATH]` and returns its
// exit code. It never shows an icon or takes the instance mutex.
func QuitCommand(args []string) int {
	killPath, err := tray.ParseQuit(args)
	if err != nil {
		return tray.QuitUsage
	}
	sid, err := userSID()
	if err != nil {
		return tray.QuitRunning
	}
	return tray.Quitter{
		Running: func() bool { return mutexExists(instanceName(sid)) },
		Signal:  func() bool { return signal(quitName(sid)) },
		Kill:    killTrays,
	}.Quit(killPath)
}

// mutexExists reports whether any process still holds a handle to the
// named mutex. The tray creates it unowned, so waiting on it cannot tell
// when the tray exits; its disappearance can.
func mutexExists(name string) bool {
	h, err := windows.OpenMutex(windows.SYNCHRONIZE, false, utf16Ptr(name))
	if err != nil {
		return !errors.Is(err, windows.ERROR_FILE_NOT_FOUND)
	}
	_ = windows.CloseHandle(h)
	return true
}

// killTrays terminates every process of this user and logon session whose
// image is path, except this process.
func killTrays(path string) error {
	self := windows.GetCurrentProcessId()
	var session uint32
	if err := windows.ProcessIdToSessionId(self, &session); err != nil {
		return err
	}
	me, err := windows.GetCurrentProcessToken().GetTokenUser()
	if err != nil {
		return err
	}
	snap, err := windows.CreateToolhelp32Snapshot(windows.TH32CS_SNAPPROCESS, 0)
	if err != nil {
		return err
	}
	defer func() { _ = windows.CloseHandle(snap) }()
	entry := windows.ProcessEntry32{Size: uint32(unsafe.Sizeof(windows.ProcessEntry32{}))}
	var errs []error
	for err = windows.Process32First(snap, &entry); err == nil; err = windows.Process32Next(snap, &entry) {
		if entry.ProcessID == self {
			continue
		}
		var s uint32
		if windows.ProcessIdToSessionId(entry.ProcessID, &s) != nil || s != session {
			continue
		}
		if err := killIfMatches(entry.ProcessID, path, me.User.Sid); err != nil {
			errs = append(errs, err)
		}
	}
	return errors.Join(errs...)
}

func killIfMatches(pid uint32, path string, owner *windows.SID) error {
	h, err := windows.OpenProcess(windows.PROCESS_QUERY_LIMITED_INFORMATION|windows.PROCESS_TERMINATE, false, pid)
	if err != nil {
		return nil // other users' and protected processes
	}
	defer func() { _ = windows.CloseHandle(h) }()
	buf := make([]uint16, windows.MAX_LONG_PATH)
	size := uint32(len(buf))
	if windows.QueryFullProcessImageName(h, 0, &buf[0], &size) != nil || !samePath(windows.UTF16ToString(buf[:size]), path) {
		return nil
	}
	var token windows.Token
	if windows.OpenProcessToken(h, windows.TOKEN_QUERY, &token) != nil {
		return nil
	}
	defer func() { _ = token.Close() }()
	user, err := token.GetTokenUser()
	if err != nil || !user.User.Sid.Equals(owner) {
		return nil
	}
	return windows.TerminateProcess(h, 1)
}
