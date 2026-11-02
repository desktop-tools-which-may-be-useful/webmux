//go:build windows

package winui

import (
	"errors"
	"time"

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
