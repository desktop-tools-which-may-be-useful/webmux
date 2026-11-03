package tray

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/rainchan/webmux/internal/fetch"
	"github.com/rainchan/webmux/internal/update"
)

// jump moves time at once, firing every timer that falls due.
func (c *fakeClock) jump(d time.Duration) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.now = c.now.Add(d)
	kept := c.waiters[:0]
	for _, w := range c.waiters {
		if !w.at.After(c.now) {
			w.ch <- c.now
		} else {
			kept = append(kept, w)
		}
	}
	c.waiters = kept
}

func (c *fakeClock) pending() int {
	c.mu.Lock()
	defer c.mu.Unlock()
	return len(c.waiters)
}

func eventually(t *testing.T, what string, ok func() bool) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for !ok() {
		if time.Now().After(deadline) {
			t.Fatalf("timed out waiting for %s", what)
		}
		time.Sleep(time.Millisecond)
	}
}

// fakeReleases serves one release, and through channel also a dev build;
// Fetch writes the installer.
type fakeReleases struct {
	mu        sync.Mutex
	latest    string
	dev       string
	latestErr error
	checks    int
	fetches   int
	// channels lists the channel of each check.
	channels []update.Channel
}

func (f *fakeReleases) Latest(ctx context.Context) (update.Manifest, string, error) {
	return f.channel(update.ChannelStable).Latest(ctx)
}

func (f *fakeReleases) Fetch(_ context.Context, m update.Manifest, _, target string, _ os.FileMode) (string, error) {
	f.mu.Lock()
	f.fetches++
	f.mu.Unlock()
	return "github.com", os.WriteFile(target, []byte("installer "+m.Version), 0o600)
}

// channel returns the client of ch: the dev build counts only on the dev
// channel, and only when it is higher than the release.
func (f *fakeReleases) channel(ch update.Channel) ReleaseClient { return channelReleases{f, ch} }

type channelReleases struct {
	*fakeReleases
	ch update.Channel
}

func (c channelReleases) Latest(context.Context) (update.Manifest, string, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.checks++
	c.channels = append(c.channels, c.ch)
	version := c.latest
	if c.ch == update.ChannelDev && update.Newer(c.dev, version) {
		version = c.dev
	}
	return update.Manifest{Version: version}, "github.com", c.latestErr
}

func (f *fakeReleases) counts() (checks, fetches int) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.checks, f.fetches
}

type updateHarness struct {
	t       *testing.T
	clock   *fakeClock
	dir     string
	auto    atomic.Bool
	idle    atomic.Bool
	mu      sync.Mutex
	channel update.Channel
	// channelSaves counts SetChannel calls.
	channelSaves int
	launched     []string
	attempts     []UpdateAttempt
	// recordErr fails RecordAttempt; launchErr fails Launch.
	recordErr, launchErr error
	// exits ends the running installer with a code.
	exits chan int
	u     *SelfUpdater
}

// newUpdateHarness builds an updater on the stable channel whose checks
// all use client; edit, when set, adjusts its options.
func newUpdateHarness(t *testing.T, installed bool, client ReleaseClient, edit func(*SelfUpdateOptions)) *updateHarness {
	h := &updateHarness{t: t, clock: newFakeClock(), dir: filepath.Join(t.TempDir(), "updates"), exits: make(chan int, 1), channel: update.ChannelStable}
	h.auto.Store(true)
	h.idle.Store(true)
	o := SelfUpdateOptions{
		Version:    "0.4.0",
		Arch:       "amd64",
		Release:    true,
		Installed:  installed,
		Dir:        h.dir,
		Client:     func(update.Channel) ReleaseClient { return client },
		Channel:    h.currentChannel,
		SetChannel: h.setChannel,
		Clock:      h.clock,
		AutoUpdate: h.auto.Load,
		Idle:       h.idle.Load,
		RecordAttempt: func(a UpdateAttempt) error {
			h.mu.Lock()
			defer h.mu.Unlock()
			if h.recordErr != nil {
				return h.recordErr
			}
			h.attempts = append(h.attempts, a)
			return nil
		},
		Launch: func(path string) (func() (int, error), error) {
			h.mu.Lock()
			defer h.mu.Unlock()
			h.launched = append(h.launched, path)
			if h.launchErr != nil {
				return nil, h.launchErr
			}
			return func() (int, error) { return <-h.exits, nil }, nil
		},
	}
	if edit != nil {
		edit(&o)
	}
	h.u = NewSelfUpdater(o)
	t.Cleanup(h.u.Close)
	return h
}

func (h *updateHarness) launches() []string {
	h.mu.Lock()
	defer h.mu.Unlock()
	return append([]string(nil), h.launched...)
}

func (h *updateHarness) recorded() []UpdateAttempt {
	h.mu.Lock()
	defer h.mu.Unlock()
	return append([]UpdateAttempt(nil), h.attempts...)
}

// currentChannel reads the stored channel as Settings.Channel does.
func (h *updateHarness) currentChannel() (update.Channel, error) {
	return Settings{UpdateChannel: h.storedChannel()}.Channel()
}

func (h *updateHarness) storedChannel() update.Channel {
	h.mu.Lock()
	defer h.mu.Unlock()
	return h.channel
}

// setChannel stores ch as the menu or a settings file edit would.
func (h *updateHarness) setChannel(ch update.Channel) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.channel = ch
	h.channelSaves++
}

// settled waits until no check runs and the schedule waits for its timer.
func (h *updateHarness) settled(timers int) {
	h.t.Helper()
	eventually(h.t, "idle updater", func() bool { return !h.u.View().Busy && h.clock.pending() == timers })
}

// The first check comes two minutes after the start, then one a day;
// with automatic updates off nothing is checked unless the user asks.
func TestSelfUpdateSchedule(t *testing.T) {
	releases := &fakeReleases{latest: "0.5.0"}
	h := newUpdateHarness(t, false, releases, nil)
	h.u.Start()
	h.settled(1)
	h.clock.jump(UpdateFirstCheck - time.Second)
	if checks, _ := releases.counts(); checks != 0 {
		t.Fatalf("checked before two minutes: %d", checks)
	}
	h.clock.jump(time.Second)
	eventually(t, "first check", func() bool { c, _ := releases.counts(); return c == 1 })
	h.settled(1)
	if v := h.u.View(); v.Latest != "0.5.0" || v.Status != "有新版本 0.5.0" {
		t.Fatalf("after check %+v", v)
	}
	h.clock.jump(UpdateInterval - time.Second)
	if checks, _ := releases.counts(); checks != 1 {
		t.Fatalf("checked before a day: %d", checks)
	}
	h.clock.jump(time.Second)
	eventually(t, "daily check", func() bool { c, _ := releases.counts(); return c == 2 })
	h.settled(1)

	h.auto.Store(false)
	for range 3 {
		h.clock.jump(UpdateInterval)
		h.settled(1)
	}
	if checks, _ := releases.counts(); checks != 2 {
		t.Fatalf("checked with automatic updates off: %d", checks)
	}
}

// With automatic updates off a manual check only reports the release; the
// installed tray installs it when the user picks "更新到".
func TestSelfUpdateManual(t *testing.T) {
	releases := &fakeReleases{latest: "0.5.0"}
	h := newUpdateHarness(t, true, releases, nil)
	h.auto.Store(false)
	h.u.Check()
	eventually(t, "check", func() bool { c, _ := releases.counts(); return c == 1 && !h.u.View().Busy })
	v := h.u.View()
	if _, fetches := releases.counts(); fetches != 0 || len(h.launches()) != 0 || v.Status != "有新版本 0.5.0" {
		t.Fatalf("manual check %+v", v)
	}
	if entry := UpdateEntry(Snapshot{}, v); entry.Title != "更新到 0.5.0" || entry.Action != UpdateInstall || !entry.Enabled {
		t.Fatalf("entry %+v", entry)
	}
	h.u.Install()
	eventually(t, "installer", func() bool { return len(h.launches()) == 1 })
	if got := h.launches()[0]; got != filepath.Join(h.dir, "webmux-setup-windows-amd64.exe") {
		t.Fatalf("launched %q", got)
	}
	if v := h.u.View(); !v.Busy || v.Status != "正在安装托盘 0.5.0…" {
		t.Fatalf("after launch %+v", v)
	}
	// The installer closes this tray; nothing starts a second one.
	h.u.Check()
	time.Sleep(10 * time.Millisecond)
	if checks, _ := releases.counts(); checks != 2 || len(h.launches()) != 1 {
		t.Fatalf("checked after launch: %d %v", checks, h.launches())
	}
}

// A downloaded installer waits while the tray restarts WebMux, installs a
// server or shows a dialog.
func TestSelfUpdateWaitsForIdle(t *testing.T) {
	releases := &fakeReleases{latest: "0.5.0"}
	h := newUpdateHarness(t, true, releases, nil)
	h.idle.Store(false)
	h.u.Check()
	eventually(t, "download", func() bool { return h.clock.pending() == 1 })
	if v := h.u.View(); v.Status != "托盘 0.5.0 已下载，当前操作完成后安装" || !v.Busy {
		t.Fatalf("waiting %+v", v)
	}
	for range 3 {
		h.clock.jump(UpdateIdlePoll)
		eventually(t, "next poll", func() bool { return h.clock.pending() == 1 })
	}
	if len(h.launches()) != 0 {
		t.Fatalf("installer ran while busy: %v", h.launches())
	}
	h.idle.Store(true)
	h.clock.jump(UpdateIdlePoll)
	eventually(t, "installer", func() bool { return len(h.launches()) == 1 })
}

// Turning automatic updates off while a downloaded installer waits for the
// tray to become idle keeps it from running; the release stays offered.
func TestSelfUpdateStopsWhenAutoUpdateTurnedOff(t *testing.T) {
	releases := &fakeReleases{latest: "0.5.0"}
	h := newUpdateHarness(t, true, releases, nil)
	h.idle.Store(false)
	h.u.Check()
	eventually(t, "download", func() bool { return h.clock.pending() == 1 })
	h.auto.Store(false)
	h.idle.Store(true)
	h.clock.jump(UpdateIdlePoll)
	eventually(t, "stopped", func() bool { return !h.u.View().Busy })
	if v := h.u.View(); v.Status != "有新版本 0.5.0" || v.Latest != "0.5.0" || len(h.launches()) != 0 || len(h.recorded()) != 0 {
		t.Fatalf("after turning off %+v, launched %v", v, h.launches())
	}
	// "更新到" still installs it.
	h.u.Install()
	eventually(t, "installer", func() bool { return len(h.launches()) == 1 })
}

// A portable tray reports the release and links its page, but never
// downloads or runs an installer, even when asked.
func TestSelfUpdatePortableNeverDownloads(t *testing.T) {
	releases := &fakeReleases{latest: "0.5.0"}
	h := newUpdateHarness(t, false, releases, nil)
	h.u.Check()
	eventually(t, "check", func() bool { c, _ := releases.counts(); return c == 1 && !h.u.View().Busy })
	h.u.Install()
	eventually(t, "second check", func() bool { c, _ := releases.counts(); return c == 2 && !h.u.View().Busy })
	if _, fetches := releases.counts(); fetches != 0 || len(h.launches()) != 0 {
		t.Fatalf("portable tray downloaded %d, launched %v", fetches, h.launches())
	}
	v := h.u.View()
	entry := UpdateEntry(Snapshot{}, v)
	if v.Status != "有新版本 0.5.0" || entry.Title != "下载托盘 0.5.0" || entry.URL != "https://github.com/mzdluo123/webmux/releases/tag/v0.5.0" || entry.Action != UpdateOpenRelease {
		t.Fatalf("portable %+v %+v", v, entry)
	}
	if _, err := os.Stat(h.dir); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("updates directory created: %v", err)
	}
}

// signedRelease serves a release whose checksum list is signed by key.
type signedRelease struct {
	key       update.SecretKey
	sums      string
	installer []byte
	checks    atomic.Int32
}

func (r *signedRelease) serve(t *testing.T) string {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		base := "/" + update.Repo + "/releases/"
		switch req.URL.Path {
		case base + "latest/download/SHA256SUMS":
			r.checks.Add(1)
			_, _ = w.Write([]byte(r.sums))
		case base + "latest/download/SHA256SUMS.minisig":
			sig, err := r.key.Sign([]byte(r.sums), "webmux 0.5.0")
			if err != nil {
				t.Error(err)
			}
			_, _ = w.Write(sig)
		case base + "download/v0.5.0/webmux-setup-windows-amd64.exe":
			_, _ = w.Write(r.installer)
		default:
			http.NotFound(w, req)
		}
	}))
	t.Cleanup(srv.Close)
	return srv.URL
}

func testReleaseKey(t *testing.T) update.SecretKey {
	t.Helper()
	k, err := update.GenerateKey()
	if err != nil {
		t.Fatal(err)
	}
	return k
}

// Nothing unverified is ever run: a checksum list signed by an unknown key
// and an installer that does not match the signed list both fail, the
// status line shows why, and the next scheduled check retries.
func TestSelfUpdateRunsOnlyVerifiedInstallers(t *testing.T) {
	trusted := testReleaseKey(t)
	installer := []byte("MZ installer")
	sum := sha256.Sum256(installer)
	sums := hex.EncodeToString(sum[:]) + "  webmux-setup-windows-amd64.exe\n"
	for _, tc := range []struct {
		name   string
		signer update.SecretKey
		served []byte
		reason string
	}{
		{"unknown key", testReleaseKey(t), installer, "更新失败："},
		{"tampered installer", trusted, []byte("MZ evil"), "更新失败：下载 webmux-setup-windows-amd64.exe 0.5.0："},
	} {
		t.Run(tc.name, func(t *testing.T) {
			release := &signedRelease{key: tc.signer, sums: sums, installer: tc.served}
			client := update.Client{
				Downloader: fetch.Downloader{GitHubBase: release.serve(t), Mirrors: []string{}},
				StableKeys: []update.PublicKey{trusted.Public()},
			}
			h := newUpdateHarness(t, true, client, nil)
			h.u.Start()
			h.settled(1)
			h.clock.jump(UpdateFirstCheck)
			eventually(t, "failed check", func() bool {
				return release.checks.Load() == 1 && strings.HasPrefix(h.u.View().Status, "更新失败")
			})
			h.settled(1)
			if v := h.u.View(); !strings.HasPrefix(v.Status, tc.reason) || len(h.launches()) != 0 {
				t.Fatalf("status %q, launched %v", v.Status, h.launches())
			}
			if entries, _ := os.ReadDir(h.dir); len(entries) != 0 {
				t.Fatalf("unverified download kept: %v", entries)
			}
			h.clock.jump(UpdateInterval)
			eventually(t, "retry", func() bool { return release.checks.Load() == 2 })
			h.settled(1)
			if len(h.launches()) != 0 {
				t.Fatalf("launched %v", h.launches())
			}
		})
	}
	// The same release, correctly signed, installs.
	release := &signedRelease{key: trusted, sums: sums, installer: installer}
	client := update.Client{
		Downloader: fetch.Downloader{GitHubBase: release.serve(t), Mirrors: []string{}},
		StableKeys: []update.PublicKey{trusted.Public()},
	}
	h := newUpdateHarness(t, true, client, nil)
	h.u.Check()
	eventually(t, "installer", func() bool { return len(h.launches()) == 1 })
	if data, err := os.ReadFile(h.launches()[0]); err != nil || string(data) != string(installer) {
		t.Fatalf("installer %q %v", data, err)
	}
}

// The new tray announces its version until the menu opened twice, reports
// a manual check that found nothing, and empties the updates directory.
func TestSelfUpdateNotices(t *testing.T) {
	releases := &fakeReleases{latest: "0.4.0"}
	h := newUpdateHarness(t, true, releases, func(o *SelfUpdateOptions) { o.Updated = "0.4.0" })
	if err := os.MkdirAll(h.dir, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(h.dir, "webmux-setup-windows-amd64.exe"), []byte("old"), 0o600); err != nil {
		t.Fatal(err)
	}
	h.u.Start()
	eventually(t, "cleanup", func() bool { entries, _ := os.ReadDir(h.dir); return len(entries) == 0 })
	if v := h.u.View(); v.Status != "已更新到 0.4.0" {
		t.Fatalf("notice %+v", v)
	}
	h.u.MenuOpened()
	if v := h.u.View(); v.Status != "已更新到 0.4.0" {
		t.Fatalf("first menu %+v", v)
	}
	h.u.MenuOpened()
	if v := h.u.View(); v.Status != "" {
		t.Fatalf("second menu %+v", v)
	}
	h.u.Check()
	eventually(t, "check", func() bool { c, _ := releases.counts(); return c == 1 && !h.u.View().Busy })
	if v := h.u.View(); v.Status != "托盘已是最新版本 0.4.0" || v.Latest != "" {
		t.Fatalf("current %+v", v)
	}
	if _, fetches := releases.counts(); fetches != 0 {
		t.Fatal("downloaded the current version")
	}
}

// A newer WSL webmux makes an installed tray with automatic updates check
// once per version; portable trays and disabled updates never check. A
// version seen while the updater is busy is checked once it is free.
func TestSelfUpdateFollowsNewerServer(t *testing.T) {
	newer := Snapshot{Tray: BuildInfo{Version: "0.4.0", Contract: 1}, Installed: BuildInfo{Version: "0.5.0", Contract: 1}, Compat: CompatServerNewer}
	newest := newer
	newest.Installed.Version = "0.6.0"
	releases := &fakeReleases{latest: "0.5.0"}
	h := newUpdateHarness(t, true, releases, nil)
	h.u.Observe(newer)
	eventually(t, "installer", func() bool { return len(h.launches()) == 1 })
	h.u.Observe(newer)
	h.u.Observe(newest)
	time.Sleep(10 * time.Millisecond)
	if checks, _ := releases.counts(); checks != 1 {
		t.Fatalf("checks %d", checks)
	}
	h.exits <- 2
	eventually(t, "failed install", func() bool { return !h.u.View().Busy })
	h.u.Observe(newer)
	time.Sleep(10 * time.Millisecond)
	if checks, _ := releases.counts(); checks != 1 {
		t.Fatalf("checked %s again: %d", newer.Installed.Version, checks)
	}
	h.u.Observe(newest)
	eventually(t, "check for "+newest.Installed.Version, func() bool { c, _ := releases.counts(); return c == 2 })

	for name, installed := range map[string]bool{"portable": false, "disabled": true} {
		releases := &fakeReleases{latest: "0.5.0"}
		h := newUpdateHarness(t, installed, releases, nil)
		h.auto.Store(name != "disabled")
		h.u.Observe(newer)
		time.Sleep(10 * time.Millisecond)
		if checks, _ := releases.counts(); checks != 0 {
			t.Errorf("%s: checked %d", name, checks)
		}
	}
}

// An installer that exits while this tray still runs failed (it closes the
// tray first when it succeeds): the status shows its exit code and the next
// check may start. One still running keeps the updater busy until
// UpdateInstallWait has passed.
func TestSelfUpdateInstallerExit(t *testing.T) {
	releases := &fakeReleases{latest: "0.5.0"}
	h := newUpdateHarness(t, true, releases, nil)
	h.auto.Store(false)
	h.u.Install()
	eventually(t, "installer", func() bool { return len(h.launches()) == 1 && h.clock.pending() == 1 })
	h.clock.jump(UpdateInstallWait - time.Second)
	h.u.Check()
	time.Sleep(10 * time.Millisecond)
	if v := h.u.View(); !v.Busy || v.Status != "正在安装托盘 0.5.0…" {
		t.Fatalf("running installer %+v", v)
	}
	if checks, _ := releases.counts(); checks != 1 {
		t.Fatalf("checked while the installer runs: %d", checks)
	}

	h.exits <- 2
	eventually(t, "failed install", func() bool { return !h.u.View().Busy })
	if v := h.u.View(); v.Status != "更新失败：安装程序退出（代码 2）" || v.Latest != "0.5.0" {
		t.Fatalf("after exit %+v", v)
	}
	// No tray was replaced, so the next start must not report a failure.
	if got := h.recorded(); len(got) != 2 || got[0].Version != "0.5.0" || !got[1].IsZero() {
		t.Fatalf("recorded after exit %+v", got)
	}
	h.u.Check()
	eventually(t, "manual check", func() bool { c, _ := releases.counts(); return c == 2 && !h.u.View().Busy })
	if v := h.u.View(); v.Status != "有新版本 0.5.0" {
		t.Fatalf("after manual check %+v", v)
	}

	h.u.Install()
	// The first installer's timer is still pending; it fires unread.
	eventually(t, "second installer", func() bool { return len(h.launches()) == 2 && h.clock.pending() == 2 })
	h.clock.jump(UpdateInstallWait)
	eventually(t, "installer timeout", func() bool { return !h.u.View().Busy })
	if v := h.u.View(); v.Status != "更新失败：安装程序 15 分钟内未完成" {
		t.Fatalf("after timeout %+v", v)
	}
	// The late installer may still close this tray: the attempt stays.
	if got := h.recorded(); len(got) != 3 || got[2].Version != "0.5.0" {
		t.Fatalf("recorded after timeout %+v", got)
	}
}

// A download the starting tray cannot remove yet (its installer is still
// finishing) is removed by a later attempt.
func TestSelfUpdateRetriesCleanup(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("root ignores directory permissions")
	}
	h := newUpdateHarness(t, true, &fakeReleases{latest: "0.4.0"}, nil)
	if err := os.MkdirAll(h.dir, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(h.dir, "webmux-setup-windows-amd64.exe"), []byte("running"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(h.dir, 0o500); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(h.dir, 0o700) })
	h.u.Start()
	h.settled(2) // first check and cleanup retry
	if err := os.Chmod(h.dir, 0o700); err != nil {
		t.Fatal(err)
	}
	h.clock.jump(updateCleanRetry)
	eventually(t, "cleanup", func() bool { entries, _ := os.ReadDir(h.dir); return len(entries) == 0 })
	h.settled(1)
}

// A tray the installer started again after a failed install (exit code 4:
// it closed this tray first) finds the attempt in its settings. It reports
// the failure, and neither the schedule nor a newer WSL webmux runs that
// release's installer again until a day after the attempt.
func TestSelfUpdateAfterFailedInstall(t *testing.T) {
	newer := Snapshot{Tray: BuildInfo{Version: "0.4.0", Contract: 1}, Installed: BuildInfo{Version: "0.5.0", Contract: 1}, Compat: CompatServerNewer}
	releases := &fakeReleases{latest: "0.5.0"}
	h := newUpdateHarness(t, true, releases, func(o *SelfUpdateOptions) {
		o.FailedAttempt = UpdateAttempt{Version: "0.5.0", At: o.Clock.Now().Add(-time.Hour)}
	})
	const failed = "更新到 0.5.0 失败，已恢复原托盘"
	if v := h.u.View(); v.Status != failed {
		t.Fatalf("at start %+v", v)
	}
	h.u.Start()
	h.settled(1)
	h.clock.jump(UpdateFirstCheck)
	eventually(t, "first check", func() bool { c, _ := releases.counts(); return c == 1 })
	h.settled(1)
	h.u.Observe(newer)
	eventually(t, "check for the server", func() bool { c, _ := releases.counts(); return c == 2 })
	h.settled(1)
	if _, fetches := releases.counts(); fetches != 0 || len(h.launches()) != 0 {
		t.Fatalf("retried within a day: fetched %d, launched %v", fetches, h.launches())
	}
	if v := h.u.View(); v.Status != failed+"；有新版本 0.5.0" || v.Latest != "0.5.0" {
		t.Fatalf("after checks %+v", v)
	}
	// The next daily check comes after the attempt's day has passed.
	h.clock.jump(UpdateInterval)
	eventually(t, "retry", func() bool { return len(h.launches()) == 1 })
	if got := h.recorded(); len(got) != 1 || got[0].Version != "0.5.0" || !got[0].At.Equal(h.clock.Now()) {
		t.Fatalf("recorded %+v", got)
	}
}

// A failed attempt never stops "更新到" from installing that release, nor
// automatic updates from installing a later one.
func TestSelfUpdateFailedInstallBlocksOnlyAutomaticRetry(t *testing.T) {
	releases := &fakeReleases{latest: "0.5.0"}
	h := newUpdateHarness(t, true, releases, func(o *SelfUpdateOptions) {
		o.FailedAttempt = UpdateAttempt{Version: "0.5.0", At: o.Clock.Now()}
	})
	h.u.Install()
	eventually(t, "manual install", func() bool { return len(h.launches()) == 1 })
	h.exits <- 2
	eventually(t, "failed install", func() bool { return !h.u.View().Busy })
	releases.mu.Lock()
	releases.latest = "0.6.0"
	releases.mu.Unlock()
	h.u.Check()
	eventually(t, "automatic install of a later release", func() bool { return len(h.launches()) == 2 })
	if got := h.recorded(); len(got) != 3 || got[0].Version != "0.5.0" || !got[1].IsZero() || got[2].Version != "0.6.0" {
		t.Fatalf("recorded %+v", got)
	}
}

// An attempt that cannot be recorded would let a failed install repeat
// unnoticed, so the installer does not run.
func TestSelfUpdateNeedsRecordedAttempt(t *testing.T) {
	releases := &fakeReleases{latest: "0.5.0"}
	h := newUpdateHarness(t, true, releases, nil)
	h.recordErr = errors.New("拒绝访问")
	h.u.Install()
	eventually(t, "failed install", func() bool { return !h.u.View().Busy })
	if v := h.u.View(); v.Status != "更新失败：无法记录更新尝试：拒绝访问" || len(h.launches()) != 0 {
		t.Fatalf("after failed record %+v, launched %v", v, h.launches())
	}
}

// An installer that never started replaced no tray: its attempt is cleared.
func TestSelfUpdateLaunchFailureClearsAttempt(t *testing.T) {
	releases := &fakeReleases{latest: "0.5.0"}
	h := newUpdateHarness(t, true, releases, nil)
	h.launchErr = errors.New("拒绝访问")
	h.u.Install()
	eventually(t, "failed install", func() bool { return !h.u.View().Busy })
	if v := h.u.View(); v.Status != "更新失败：无法运行安装程序：拒绝访问" {
		t.Fatalf("after failed launch %+v", v)
	}
	if got := h.recorded(); len(got) != 2 || got[0].Version != "0.5.0" || !got[1].IsZero() {
		t.Fatalf("recorded %+v", got)
	}
}

// Picking "开发版" saves the channel and checks at once: an installed tray
// with automatic updates on downloads the dev build's installer from
// Release dev, verified against the dev keys, and runs it. Picking the
// current channel does nothing.
func TestSelfUpdateSelectDevChannel(t *testing.T) {
	stableKey, devKey := testReleaseKey(t), testReleaseKey(t)
	installer := []byte("MZ dev installer")
	const devAsset = "webmux-0.10.3-dev.4-setup-windows-amd64.exe"
	stableSums := sum([]byte("MZ installer")) + "  webmux-setup-windows-amd64.exe\n"
	devSums := sum(installer) + "  " + devAsset + "\n"
	var requests atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		requests.Add(1)
		signed := func(k update.SecretKey, sums, version string) []byte {
			sig, err := k.Sign([]byte(sums), "webmux "+version)
			if err != nil {
				t.Error(err)
			}
			return sig
		}
		base := "/" + update.Repo + "/releases/"
		var body []byte
		switch req.URL.Path {
		case base + "latest/download/SHA256SUMS":
			body = []byte(stableSums)
		case base + "latest/download/SHA256SUMS.minisig":
			body = signed(stableKey, stableSums, "0.10.2")
		case base + "download/dev/SHA256SUMS":
			body = []byte(devSums)
		case base + "download/dev/SHA256SUMS.minisig":
			body = signed(devKey, devSums, "0.10.3-dev.4")
		case base + "download/dev/" + devAsset:
			body = installer
		default:
			http.NotFound(w, req)
			return
		}
		_, _ = w.Write(body)
	}))
	t.Cleanup(srv.Close)
	h := newUpdateHarness(t, true, nil, func(o *SelfUpdateOptions) {
		o.Version = "0.10.2"
		o.Client = func(ch update.Channel) ReleaseClient {
			return update.Client{
				Downloader: fetch.Downloader{GitHubBase: srv.URL, Mirrors: []string{}},
				Channel:    ch,
				StableKeys: []update.PublicKey{stableKey.Public()},
				DevKeys:    []update.PublicKey{devKey.Public()},
			}
		}
	})
	h.u.SelectChannel(update.ChannelStable)
	time.Sleep(10 * time.Millisecond)
	if n := requests.Load(); n != 0 {
		t.Fatalf("picking the current channel checked: %d requests", n)
	}
	h.u.SelectChannel(update.ChannelDev)
	eventually(t, "installer", func() bool { return len(h.launches()) == 1 })
	if ch := h.storedChannel(); ch != update.ChannelDev {
		t.Fatalf("channel %s", ch)
	}
	if got := h.launches()[0]; got != filepath.Join(h.dir, devAsset) {
		t.Fatalf("launched %q", got)
	}
	if data, err := os.ReadFile(h.launches()[0]); err != nil || string(data) != string(installer) {
		t.Fatalf("installer %q %v", data, err)
	}
	if v := h.u.View(); v.Channel != update.ChannelDev || v.Status != "正在安装托盘 0.10.3-dev.4…" {
		t.Fatalf("view %+v", v)
	}
}

// Leaving the dev channel never downgrades: the dev build stays, the
// status line says why, and the release that passes it installs.
func TestSelfUpdateBackToStableKeepsDevBuild(t *testing.T) {
	releases := &fakeReleases{latest: "0.10.2", dev: "0.10.3-dev.4"}
	h := newUpdateHarness(t, true, nil, func(o *SelfUpdateOptions) {
		o.Version = "0.10.3-dev.4"
		o.Client = releases.channel
	})
	h.setChannel(update.ChannelDev)
	h.u.SelectChannel(update.ChannelStable)
	eventually(t, "check", func() bool { c, _ := releases.counts(); return c == 1 && !h.u.View().Busy })
	v := h.u.View()
	if _, fetches := releases.counts(); fetches != 0 || len(h.launches()) != 0 || v.Latest != "" ||
		v.Status != "当前开发版 0.10.3-dev.4 高于最新正式版 0.10.2，正式版超过它后自动更新" {
		t.Fatalf("after switching back %+v, launched %v", v, h.launches())
	}
	releases.mu.Lock()
	releases.latest = "0.10.3"
	releases.mu.Unlock()
	h.u.Check()
	eventually(t, "installer", func() bool { return len(h.launches()) == 1 })
	if got := h.launches()[0]; got != filepath.Join(h.dir, "webmux-setup-windows-amd64.exe") {
		t.Fatalf("launched %q", got)
	}
}

// A newer dev build in WSL is never offered on the stable channel, so it
// starts no automatic check there. "切换到开发版通道并更新托盘" saves the
// dev channel and installs at once, like "更新到": with automatic updates
// off and right after a failed attempt at that build.
func TestSelfUpdateSwitchToDevForDevServer(t *testing.T) {
	releases := &fakeReleases{latest: "0.10.2", dev: "0.10.3-dev.7"}
	h := newUpdateHarness(t, true, nil, func(o *SelfUpdateOptions) {
		o.Version = "0.10.2"
		o.Client = releases.channel
		o.FailedAttempt = UpdateAttempt{Version: "0.10.3-dev.7", At: o.Clock.Now()}
	})
	tray := BuildInfo{Version: "0.10.2", Contract: 1}
	devWSL := Snapshot{Tray: tray, Installed: BuildInfo{Version: "0.10.3-dev.7", Contract: 1}}
	devWSL.Compat = Classify(devWSL.Tray, devWSL.Installed)
	h.u.Observe(devWSL)
	time.Sleep(10 * time.Millisecond)
	if checks, _ := releases.counts(); checks != 0 {
		t.Fatalf("stable channel checked for a dev server: %d", checks)
	}
	h.auto.Store(false)
	if entry := UpdateEntry(devWSL, h.u.View()); entry.Action != UpdateSwitchToDev || !entry.Enabled {
		t.Fatalf("entry %+v", entry)
	}
	h.u.SwitchToDev()
	eventually(t, "installer", func() bool { return len(h.launches()) == 1 })
	if ch := h.storedChannel(); ch != update.ChannelDev {
		t.Fatalf("channel %s", ch)
	}
	if got := h.launches()[0]; got != filepath.Join(h.dir, "webmux-0.10.3-dev.7-setup-windows-amd64.exe") {
		t.Fatalf("launched %q", got)
	}
	if got := h.recorded(); len(got) != 1 || got[0].Version != "0.10.3-dev.7" {
		t.Fatalf("recorded %+v", got)
	}
}

func (f *fakeReleases) checked() []update.Channel {
	f.mu.Lock()
	defer f.mu.Unlock()
	return slices.Clone(f.channels)
}

// Switching channels while a downloaded dev build waits to install forgets
// it at once, never launches or records it, and checks the new channel as
// soon as the running operation ends.
func TestSelfUpdateSwitchDuringInstallDropsOldChannelBuild(t *testing.T) {
	releases := &fakeReleases{latest: "0.10.2", dev: "0.10.3-dev.4"}
	h := newUpdateHarness(t, true, nil, func(o *SelfUpdateOptions) {
		o.Version = "0.10.2"
		o.Client = releases.channel
	})
	h.setChannel(update.ChannelDev)
	h.idle.Store(false)
	h.u.Check()
	eventually(t, "download", func() bool { return h.clock.pending() == 1 })
	if v := h.u.View(); v.Latest != "0.10.3-dev.4" {
		t.Fatalf("before the switch %+v", v)
	}
	h.u.SelectChannel(update.ChannelStable)
	if v := h.u.View(); v.Latest != "" || v.Status != "" || !v.Busy {
		t.Fatalf("after the switch %+v", v)
	}
	h.idle.Store(true)
	h.clock.jump(UpdateIdlePoll)
	eventually(t, "stable check", func() bool { c, _ := releases.counts(); return c == 2 && !h.u.View().Busy })
	if got := releases.checked(); !slices.Equal(got, []update.Channel{update.ChannelDev, update.ChannelStable}) {
		t.Fatalf("checked %v", got)
	}
	if len(h.launches()) != 0 || len(h.recorded()) != 0 {
		t.Fatalf("launched %v, recorded %+v", h.launches(), h.recorded())
	}
	if v := h.u.View(); v.Latest != "" || v.Status != "托盘已是最新版本 0.10.2" {
		t.Fatalf("after the stable check %+v", v)
	}
}

// "切换到开发版通道并更新托盘" while a stable build waits to install drops
// that build and installs the dev build once the operation ends.
func TestSelfUpdateSwitchToDevQueuesInstall(t *testing.T) {
	releases := &fakeReleases{latest: "0.10.2", dev: "0.10.3-dev.4"}
	h := newUpdateHarness(t, true, nil, func(o *SelfUpdateOptions) {
		o.Version = "0.10.1"
		o.Client = releases.channel
	})
	h.auto.Store(false)
	h.idle.Store(false)
	h.u.Install()
	eventually(t, "download", func() bool { return h.clock.pending() == 1 })
	h.u.SwitchToDev()
	h.idle.Store(true)
	h.clock.jump(UpdateIdlePoll)
	eventually(t, "installer", func() bool { return len(h.launches()) == 1 })
	if got := h.launches()[0]; got != filepath.Join(h.dir, "webmux-0.10.3-dev.4-setup-windows-amd64.exe") {
		t.Fatalf("launched %q", got)
	}
	if got := h.recorded(); len(got) != 1 || got[0].Version != "0.10.3-dev.4" {
		t.Fatalf("recorded %+v", got)
	}
}

// Editing updateChannel in tray.json while a dev build waits to install
// acts like a switch from the menu: the build is forgotten at once, never
// launched, and the new channel is checked once the operation ends.
func TestSelfUpdateChannelChangedInSettingsFile(t *testing.T) {
	releases := &fakeReleases{latest: "0.10.2", dev: "0.10.3-dev.4"}
	h := newUpdateHarness(t, true, nil, func(o *SelfUpdateOptions) {
		o.Version = "0.10.2"
		o.Client = releases.channel
	})
	h.setChannel(update.ChannelDev)
	h.idle.Store(false)
	h.u.Check()
	eventually(t, "download", func() bool { return h.clock.pending() == 1 })
	if v := h.u.View(); v.Status != "托盘 0.10.3-dev.4 已下载，当前操作完成后安装" {
		t.Fatalf("waiting %+v", v)
	}
	h.setChannel(update.ChannelStable) // the reloaded file
	h.u.ChannelChanged()
	if v := h.u.View(); v.Latest != "" || v.Status != "" || !v.Busy {
		t.Fatalf("after the reload %+v", v)
	}
	h.idle.Store(true)
	h.clock.jump(UpdateIdlePoll)
	eventually(t, "stable check", func() bool { c, _ := releases.counts(); return c == 2 && !h.u.View().Busy })
	if got := releases.checked(); !slices.Equal(got, []update.Channel{update.ChannelDev, update.ChannelStable}) {
		t.Fatalf("checked %v", got)
	}
	if len(h.launches()) != 0 || len(h.recorded()) != 0 {
		t.Fatalf("launched %v, recorded %+v", h.launches(), h.recorded())
	}
}

// The last of several switches made while busy decides the queued check:
// "正式版" picked after "切换到开发版通道并更新托盘" does not inherit its
// install, so with automatic updates off it only reports the release.
func TestSelfUpdateLastQueuedSwitchDecides(t *testing.T) {
	releases := &fakeReleases{latest: "0.10.2", dev: "0.10.3-dev.4"}
	h := newUpdateHarness(t, true, nil, func(o *SelfUpdateOptions) {
		o.Version = "0.10.1"
		o.Client = releases.channel
	})
	h.setChannel(update.ChannelDev)
	h.auto.Store(false)
	h.idle.Store(false)
	h.u.Install()
	eventually(t, "download", func() bool { return h.clock.pending() == 1 })
	h.u.SwitchToDev()
	h.u.SelectChannel(update.ChannelStable)
	h.idle.Store(true)
	h.clock.jump(UpdateIdlePoll)
	eventually(t, "stable check", func() bool { c, _ := releases.counts(); return c == 2 && !h.u.View().Busy })
	if len(h.launches()) != 0 || len(h.recorded()) != 0 {
		t.Fatalf("launched %v, recorded %+v", h.launches(), h.recorded())
	}
	if v := h.u.View(); v.Latest != "0.10.2" || v.Status != "有新版本 0.10.2" {
		t.Fatalf("after the stable check %+v", v)
	}
}

// An unrecognised stored channel counts as stable; picking "正式版" saves
// it without a check, and picking it again does nothing.
func TestSelfUpdateSelectChannelRewritesUnrecognisedValue(t *testing.T) {
	releases := &fakeReleases{latest: "0.4.0"}
	h := newUpdateHarness(t, true, releases, nil)
	h.setChannel("beta")
	h.u.SelectChannel(update.ChannelStable)
	h.u.SelectChannel(update.ChannelStable)
	time.Sleep(10 * time.Millisecond)
	h.mu.Lock()
	stored, saves := h.channel, h.channelSaves
	h.mu.Unlock()
	if checks, _ := releases.counts(); stored != update.ChannelStable || saves != 2 || checks != 0 {
		t.Fatalf("stored %q after %d saves, %d checks", stored, saves, checks)
	}
}
