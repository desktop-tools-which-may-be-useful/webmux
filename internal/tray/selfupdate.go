package tray

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/rainchan/webmux/internal/update"
)

// Timing of the tray's own update checks; see windows-tray-self-update.
const (
	UpdateFirstCheck = 2 * time.Minute
	UpdateInterval   = 24 * time.Hour
	// UpdateIdlePoll is how often a downloaded installer waits for the
	// tray to become idle.
	UpdateIdlePoll = 5 * time.Second
	// UpdateInstallWait bounds how long a running installer keeps the
	// updater busy when it has not closed this tray.
	UpdateInstallWait = 15 * time.Minute

	updateCleanRetry   = 10 * time.Second
	updateCleanRetries = 30
)

// ReleaseClient reads signed releases; update.Client implements it.
type ReleaseClient interface {
	Latest(ctx context.Context) (update.Manifest, string, error)
	Fetch(ctx context.Context, m update.Manifest, asset, target string, mode os.FileMode) (string, error)
}

// SelfUpdateOptions configure a SelfUpdater.
type SelfUpdateOptions struct {
	// Version is the tray version; Arch the Windows architecture of the
	// installer ("amd64", "arm64").
	Version, Arch string
	// Release is update.IsRelease(): only release builds check.
	Release bool
	// Installed is true for a tray running from the directory the
	// installer registered; any other tray is portable and never
	// downloads or runs an installer.
	Installed bool
	// Dir holds the downloaded installer (%LOCALAPPDATA%\WebMux\updates).
	Dir    string
	Client ReleaseClient
	Clock  Clock
	// AutoUpdate reads Settings.AutoUpdate at the time of each decision.
	AutoUpdate func() bool
	// Idle reports that no server install, start, stop or restart runs and
	// no dialog is open, so the installer may close the tray.
	Idle func() bool
	// RecordAttempt saves Settings.UpdateAttempt before an installer
	// starts; the installer does not run when it fails.
	RecordAttempt func(UpdateAttempt) error
	// Launch starts the installer silently and detached; wait reports its
	// exit code once it exits.
	Launch func(path string) (wait func() (int, error), err error)
	// Publish is called after every change of View.
	Publish func()
	Logf    func(format string, args ...any)
	// Updated is the version announced once as "已更新到", from
	// RecordVersion; "" announces nothing.
	Updated string
	// FailedAttempt is the failed install RecordVersion found. It is
	// announced once, and checks the user did not ask to install skip its
	// release until UpdateInterval after the attempt.
	FailedAttempt UpdateAttempt
}

// SelfUpdateView is what the menu shows about the tray's own update.
type SelfUpdateView struct {
	Release, Installed bool
	// Busy is true while a check, download or install runs.
	Busy bool
	// Latest is a newer release found by the latest check, or "".
	Latest string
	// Status is the status line, or "".
	Status string
}

// SelfUpdater checks for newer tray releases and, for an installed tray,
// downloads the signed installer and runs it silently once the tray is
// idle. The installer then closes this tray as with "仅退出托盘".
type SelfUpdater struct {
	o      SelfUpdateOptions
	ctx    context.Context
	cancel context.CancelFunc
	wg     sync.WaitGroup

	mu     sync.Mutex
	busy   bool
	latest string
	status string
	// notice is shown until the menu opened twice: once to show it.
	notice     string
	noticeSeen bool
	// servers are newer WSL versions that already triggered a check.
	servers map[string]bool
}

// NewSelfUpdater returns an updater; Start runs its schedule.
func NewSelfUpdater(o SelfUpdateOptions) *SelfUpdater {
	if o.Clock == nil {
		o.Clock = RealClock
	}
	if o.Logf == nil {
		o.Logf = func(string, ...any) {}
	}
	if o.Publish == nil {
		o.Publish = func() {}
	}
	if o.AutoUpdate == nil {
		o.AutoUpdate = func() bool { return false }
	}
	ctx, cancel := context.WithCancel(context.Background())
	u := &SelfUpdater{o: o, ctx: ctx, cancel: cancel, servers: map[string]bool{}}
	if o.Updated != "" {
		u.notice = "已更新到 " + o.Updated
		o.Logf("tray updated to %s", o.Updated)
	}
	if f := o.FailedAttempt; !f.IsZero() {
		u.notice = "更新到 " + f.Version + " 失败，已恢复原托盘"
		o.Logf("tray update to %s started at %s did not complete; not installing it automatically before %s",
			f.Version, f.At.Format(time.RFC3339), f.At.Add(UpdateInterval).Format(time.RFC3339))
	}
	return u
}

// Start removes earlier downloads, then checks UpdateFirstCheck after the
// start and every UpdateInterval while automatic updates are on.
func (u *SelfUpdater) Start() {
	u.wg.Go(u.schedule)
}

// Close stops the schedule and any check or download, and waits for them.
func (u *SelfUpdater) Close() {
	u.cancel()
	u.wg.Wait()
}

// Check checks now, from the menu; it does not depend on AutoUpdate.
func (u *SelfUpdater) Check() { u.spawn(true, false) }

// Install checks now and installs a newer release on an installed tray,
// whether or not automatic updates are on.
func (u *SelfUpdater) Install() { u.spawn(true, true) }

// Observe checks once per newer WSL webmux version when automatic updates
// are on, so an installed tray catches up with its server. A version counts
// only once its check started, so one that arrives while another operation
// runs is checked on a later snapshot.
func (u *SelfUpdater) Observe(snap Snapshot) {
	version := snap.Installed.Version
	if !snap.Compat.Newer() || !releaseVersion(version) || !u.o.Release || !u.o.Installed || !u.o.AutoUpdate() {
		return
	}
	u.start(false, false, func() bool {
		if u.servers[version] {
			return false
		}
		u.servers[version] = true
		u.o.Logf("update: WSL webmux %s is newer than tray %s; checking", version, u.o.Version)
		return true
	})
}

// MenuOpened dismisses a notice the user has already seen in an earlier
// opening of the menu.
func (u *SelfUpdater) MenuOpened() {
	u.mu.Lock()
	changed := false
	if u.notice != "" {
		if u.noticeSeen {
			u.notice, u.noticeSeen, changed = "", false, true
		} else {
			u.noticeSeen = true
		}
	}
	u.mu.Unlock()
	if changed {
		u.o.Publish()
	}
}

// View returns the current state for the menu.
func (u *SelfUpdater) View() SelfUpdateView {
	u.mu.Lock()
	defer u.mu.Unlock()
	var lines []string
	for _, line := range []string{u.notice, u.status} {
		if line != "" {
			lines = append(lines, line)
		}
	}
	return SelfUpdateView{
		Release:   u.o.Release,
		Installed: u.o.Installed,
		Busy:      u.busy,
		Latest:    u.latest,
		Status:    strings.Join(lines, "；"),
	}
}

func (u *SelfUpdater) schedule() {
	next := u.o.Clock.After(UpdateFirstCheck)
	// The installer that started this tray is still finishing, so its
	// file may resist the first removals.
	var retry <-chan time.Time
	retries := 0
	if !u.tidy() {
		retry = u.o.Clock.After(updateCleanRetry)
	}
	for {
		select {
		case <-u.ctx.Done():
			return
		case <-retry:
			retry = nil
			if retries++; !u.tidy() && retries < updateCleanRetries {
				retry = u.o.Clock.After(updateCleanRetry)
			}
			continue
		case <-next:
		}
		next = u.o.Clock.After(UpdateInterval)
		if u.o.Release && u.o.AutoUpdate() {
			u.run(false, false)
		} else {
			u.tidy()
		}
	}
}

// tidy removes earlier downloads unless an update runs, and reports
// whether none remain.
func (u *SelfUpdater) tidy() bool {
	if !u.begin(nil) {
		return true
	}
	defer u.end()
	return u.clean()
}

func (u *SelfUpdater) spawn(manual, install bool) { u.start(manual, install, nil) }

// start claims the updater and runs one check in the background; claim,
// when set, runs under the lock once nothing else holds the updater and
// may refuse the check.
func (u *SelfUpdater) start(manual, install bool, claim func() bool) {
	if !u.o.Release || u.ctx.Err() != nil || !u.begin(claim) {
		return
	}
	u.wg.Go(func() { u.work(manual, install) })
}

// begin claims the updater; it fails while another operation runs, which
// includes an installer that has not exited, or when claim refuses.
func (u *SelfUpdater) begin(claim func() bool) bool {
	u.mu.Lock()
	defer u.mu.Unlock()
	if u.busy || (claim != nil && !claim()) {
		return false
	}
	u.busy = true
	return true
}

func (u *SelfUpdater) end() {
	u.mu.Lock()
	u.busy = false
	u.mu.Unlock()
	u.o.Publish()
}

func (u *SelfUpdater) set(edit func()) {
	u.mu.Lock()
	edit()
	u.mu.Unlock()
	u.o.Publish()
}

func (u *SelfUpdater) setStatus(status string) {
	u.set(func() { u.status = status })
}

// run is one check unless another operation runs.
func (u *SelfUpdater) run(manual, install bool) {
	if u.begin(nil) {
		u.work(manual, install)
	}
}

// work is one claimed check, and for an installed tray the download and
// install; the updater stays busy until the installer exits.
func (u *SelfUpdater) work(manual, install bool) {
	u.o.Publish()
	u.update(manual, install)
	u.end()
}

func (u *SelfUpdater) update(manual, install bool) {
	ctx := u.ctx
	u.clean()
	m, host, err := u.o.Client.Latest(ctx)
	if err != nil {
		u.fail("check", err)
		return
	}
	if !update.Newer(m.Version, u.o.Version) {
		u.o.Logf("update check: tray %s is current; latest %s from %s", u.o.Version, m.Version, host)
		u.set(func() {
			u.latest, u.status = "", ""
			if manual {
				u.notice, u.noticeSeen = "托盘已是最新版本 "+u.o.Version, false
			}
		})
		return
	}
	u.o.Logf("update check: tray %s, latest %s from %s", u.o.Version, m.Version, host)
	u.set(func() { u.latest, u.status = m.Version, "有新版本 "+m.Version })
	if !u.o.Installed || (!install && !u.o.AutoUpdate()) {
		return
	}
	if f := u.o.FailedAttempt; !install && f.Version == m.Version && u.o.Clock.Now().Before(f.At.Add(UpdateInterval)) {
		u.o.Logf("update install: %s failed to install at %s; not retrying it automatically yet", m.Version, f.At.Format(time.RFC3339))
		return
	}
	asset := "webmux-setup-windows-" + u.o.Arch + ".exe"
	target := filepath.Join(u.o.Dir, asset)
	u.setStatus("正在下载托盘 " + m.Version + "…")
	if err := os.MkdirAll(u.o.Dir, 0o700); err != nil {
		u.fail("download", fmt.Errorf("下载目录：%w", err))
		return
	}
	host, err = u.o.Client.Fetch(ctx, m, asset, target, 0o700)
	if err != nil {
		u.fail("download", fmt.Errorf("下载 %s %s：%w", asset, m.Version, err))
		return
	}
	u.o.Logf("update download: %s %s from %s", asset, m.Version, host)
	for {
		// The user may turn automatic updates off while this waits.
		if !install && !u.o.AutoUpdate() {
			u.o.Logf("update install: automatic updates turned off; not installing %s", m.Version)
			u.setStatus("有新版本 " + m.Version)
			return
		}
		if u.o.Idle() {
			break
		}
		u.setStatus("托盘 " + m.Version + " 已下载，当前操作完成后安装")
		select {
		case <-ctx.Done():
			return
		case <-u.o.Clock.After(UpdateIdlePoll):
		}
	}
	if err := u.o.RecordAttempt(UpdateAttempt{Version: m.Version, At: u.o.Clock.Now()}); err != nil {
		u.fail("install", fmt.Errorf("无法记录更新尝试：%w", err))
		return
	}
	wait, err := u.o.Launch(target)
	if err != nil {
		u.clearAttempt()
		u.fail("install", fmt.Errorf("无法运行安装程序：%w", err))
		return
	}
	u.o.Logf("update install: started %s /S for %s", target, m.Version)
	u.setStatus("正在安装托盘 " + m.Version + "…")
	u.awaitInstaller(wait)
}

// awaitInstaller returns when the tray closes, when the installer exits
// or after UpdateInstallWait. A successful installer closes this tray
// before it exits, so an exit seen here means the update failed without
// closing the tray: exit code 2 or 3 (deploy/windows/webmux.nsi), and the
// attempt is cleared, as no tray was replaced. A failure after closing it
// (code 4) is never seen here; the installer restarts the old tray, which
// finds Settings.UpdateAttempt still recorded. A timeout or an unobservable
// exit keeps the attempt: the installer may still close the tray.
func (u *SelfUpdater) awaitInstaller(wait func() (int, error)) {
	type exit struct {
		code int
		err  error
	}
	// The wait cannot be cancelled; a tray that closes leaves it behind.
	exited := make(chan exit, 1)
	go func() {
		code, err := wait()
		exited <- exit{code, err}
	}()
	select {
	case <-u.ctx.Done():
	case e := <-exited:
		if e.err != nil {
			u.fail("install", fmt.Errorf("等待安装程序：%w", e.err))
			return
		}
		u.clearAttempt()
		u.fail("install", fmt.Errorf("安装程序退出（代码 %d）", e.code))
	case <-u.o.Clock.After(UpdateInstallWait):
		u.fail("install", fmt.Errorf("安装程序 %d 分钟内未完成", int(UpdateInstallWait/time.Minute)))
	}
}

// clearAttempt removes the recorded attempt of an installer that ended
// while this tray still runs, so the next start reports no failure.
func (u *SelfUpdater) clearAttempt() {
	if u.ctx.Err() != nil {
		return
	}
	if err := u.o.RecordAttempt(UpdateAttempt{}); err != nil {
		u.o.Logf("update install: clear recorded attempt: %v", err)
	}
}

func (u *SelfUpdater) fail(stage string, err error) {
	if u.ctx.Err() != nil {
		return
	}
	u.o.Logf("update %s failed: %v", stage, err)
	u.setStatus("更新失败：" + err.Error())
}

// clean removes earlier downloads and reports whether none remain; an
// installer that still runs keeps its file until a later attempt.
func (u *SelfUpdater) clean() bool {
	if u.o.Dir == "" {
		return true
	}
	entries, err := os.ReadDir(u.o.Dir)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return true
		}
		u.o.Logf("update cleanup: %v", err)
		return false
	}
	clean := true
	for _, e := range entries {
		if err := os.RemoveAll(filepath.Join(u.o.Dir, e.Name())); err != nil {
			u.o.Logf("update cleanup: %v", err)
			clean = false
		}
	}
	return clean
}
