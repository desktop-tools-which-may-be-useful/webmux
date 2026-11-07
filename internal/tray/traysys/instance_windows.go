//go:build windows

package traysys

import (
	"errors"
	"time"
	"unsafe"

	"golang.org/x/sys/windows"
)

// Instance is the single tray instance of this Windows user session.
//
// The design asks the second instance to PostMessage the hidden window of
// the first. fyne.io/systray's window has a generic class and no title, so
// the first instance instead waits on a named auto-reset event that the
// second instance sets; the effect is the same. `webmux-tray quit` sets a
// second event the same way.
type Instance struct {
	mutex windows.Handle
	wake  windows.Handle
	quit  windows.Handle
}

func instanceName(sid string) string { return `Local\WebMuxTray-` + sid }
func wakeName(sid string) string     { return `Local\WebMuxTrayWake-` + sid }
func quitName(sid string) string     { return `Local\WebMuxTrayQuit-` + sid }

func userSID() (string, error) {
	user, err := windows.GetCurrentProcessToken().GetTokenUser()
	if err != nil {
		return "", err
	}
	return user.User.Sid.String(), nil
}

// AcquireInstance takes `Local\WebMuxTray-<SID>`. When another instance
// holds it, AcquireInstance wakes that instance and returns nil, nil.
func AcquireInstance() (*Instance, error) {
	sid, err := userSID()
	if err != nil {
		return nil, err
	}
	mutex, err := windows.CreateMutex(nil, false, utf16Ptr(instanceName(sid)))
	if errors.Is(err, windows.ERROR_ALREADY_EXISTS) {
		_ = windows.CloseHandle(mutex)
		signal(wakeName(sid))
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	i := &Instance{mutex: mutex}
	if i.wake, err = createEvent(wakeName(sid)); err == nil {
		i.quit, err = createEvent(quitName(sid))
	}
	if err != nil {
		for _, h := range []windows.Handle{i.wake, i.mutex} {
			if h != 0 {
				_ = windows.CloseHandle(h)
			}
		}
		return nil, err
	}
	return i, nil
}

func createEvent(name string) (windows.Handle, error) {
	h, err := windows.CreateEvent(nil, 0, 0, utf16Ptr(name))
	if err != nil && !errors.Is(err, windows.ERROR_ALREADY_EXISTS) {
		return 0, err
	}
	return h, nil
}

// signal sets a named event of the first instance, which may still be
// creating it. It reports false when the event never appears.
func signal(name string) bool {
	for range 20 {
		event, err := windows.OpenEvent(windows.EVENT_MODIFY_STATE, false, utf16Ptr(name))
		if err == nil {
			_ = windows.SetEvent(event)
			_ = windows.CloseHandle(event)
			return true
		}
		time.Sleep(100 * time.Millisecond)
	}
	return false
}

// Wakes calls f each time another instance starts.
func (i *Instance) Wakes(f func()) { watch(i.wake, f) }

// Quits calls f when `webmux-tray quit` asks this instance to exit.
func (i *Instance) Quits(f func()) { watch(i.quit, f) }

func watch(event windows.Handle, f func()) {
	go func() {
		for {
			r, err := windows.WaitForSingleObject(event, windows.INFINITE)
			if err != nil || r != windows.WAIT_OBJECT_0 {
				return
			}
			f()
		}
	}()
}

// QuitCommand runs `webmux-tray quit [--kill-path PATH]` and returns its
// exit code. It never shows an icon or takes the instance mutex.
func QuitCommand(args []string) int {
	killPath, err := ParseQuit(args)
	if err != nil {
		return QuitUsage
	}
	sid, err := userSID()
	if err != nil {
		return QuitRunning
	}
	return Quitter{
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
	if windows.QueryFullProcessImageName(h, 0, &buf[0], &size) != nil || !SamePath(windows.UTF16ToString(buf[:size]), path) {
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
