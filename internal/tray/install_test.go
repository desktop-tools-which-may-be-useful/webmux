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

	"github.com/rainchan/webmux/internal/contract"
	"github.com/rainchan/webmux/internal/fetch"
	"github.com/rainchan/webmux/internal/update"
)

// installRunner plays a distribution: uname, wslpath, install, version.
type installRunner struct {
	mu        sync.Mutex
	machine   string
	noMount   bool
	installed string // version reported after install
	calls     [][]string
}

func (r *installRunner) Run(_ context.Context, argv, _ []string) (Result, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.calls = append(r.calls, argv)
	exec := slices.Index(argv, "--exec")
	switch argv[exec+1] {
	case "uname":
		return Result{Stdout: []byte(r.machine + "\n")}, nil
	case "wslpath":
		if r.noMount {
			return Result{Stderr: []byte("wslpath: not mounted"), Code: 1}, nil
		}
		return Result{Stdout: []byte("/mnt/c/dl/" + filepath.Base(argv[len(argv)-1]) + "\n")}, nil
	case "install":
		return Result{}, nil
	}
	if slices.Equal(argv[len(argv)-2:], []string{"version", "--json"}) {
		return Result{Stdout: []byte(`{"v":1,"version":"` + r.installed + `","contract":1}` + "\n")}, nil
	}
	return Result{Code: 1}, nil
}

func (r *installRunner) Start([]string, []string) (Process, error) { return nil, errors.New("unused") }

func (r *installRunner) program(name string) [][]string {
	r.mu.Lock()
	defer r.mu.Unlock()
	var out [][]string
	for _, argv := range r.calls {
		if exec := slices.Index(argv, "--exec"); exec >= 0 && argv[exec+1] == name {
			out = append(out, argv)
		}
	}
	return out
}

type releaseServer struct {
	*httptest.Server
	hits  atomic.Int32
	paths chan string
}

func newReleaseServer(t *testing.T, body []byte) *releaseServer {
	s := &releaseServer{paths: make(chan string, 8)}
	s.Server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		s.hits.Add(1)
		s.paths <- r.URL.Path
		_, _ = w.Write(body)
	}))
	t.Cleanup(s.Close)
	return s
}

func sum(b []byte) string {
	h := sha256.Sum256(b)
	return hex.EncodeToString(h[:])
}

var serverBinary = []byte("#!/bin/sh\necho webmux 0.2.0\n")

func testInstaller(t *testing.T, runner *installRunner, base string, digests map[string]string) Installer {
	return Installer{
		Runner:     runner,
		Target:     Target{Distro: "Ubuntu", Webmux: "~/my bin/web;mux"},
		Version:    "0.2.0",
		Dir:        filepath.Join(t.TempDir(), "downloads"),
		Digests:    digests,
		Downloader: fetch.Downloader{Name: "webmux", GitHubBase: base, Mirrors: []string{}},
	}
}

func TestInstallerInstallsVerifiedRelease(t *testing.T) {
	srv := newReleaseServer(t, serverBinary)
	runner := &installRunner{machine: "aarch64", installed: "0.2.0"}
	in := testInstaller(t, runner, srv.URL, map[string]string{"amd64": "unused", "arm64": sum(serverBinary)})
	if err := in.Install(context.Background()); err != nil {
		t.Fatal(err)
	}
	if got := <-srv.paths; got != "/"+update.Repo+"/releases/download/v0.2.0/webmux-linux-arm64" {
		t.Fatalf("release path %q", got)
	}
	installs := runner.program("install")
	want := []string{"wsl.exe", "-d", "Ubuntu", "--cd", "~", "--exec", "install", "-D", "-m", "0755", "/mnt/c/dl/webmux-0.2.0-linux-arm64", "./my bin/web;mux"}
	if len(installs) != 1 || !slices.Equal(installs[0], want) {
		t.Fatalf("install argv %q", installs)
	}
	// A verified download is reused for the next install.
	if err := in.Install(context.Background()); err != nil || srv.hits.Load() != 1 {
		t.Fatalf("reinstall: %v, %d downloads", err, srv.hits.Load())
	}
}

// A dev tray installs the server of its own build from Release dev under
// the versioned name; the cache keeps the usual name.
func TestInstallerDevBuild(t *testing.T) {
	srv := newReleaseServer(t, serverBinary)
	runner := &installRunner{machine: "x86_64", installed: "0.10.3-dev.4"}
	in := testInstaller(t, runner, srv.URL, map[string]string{"amd64": sum(serverBinary), "arm64": "unused"})
	in.Version = "0.10.3-dev.4"
	if err := in.Install(context.Background()); err != nil {
		t.Fatal(err)
	}
	if got := <-srv.paths; got != "/mzdluo123/webmux/releases/download/dev/webmux-0.10.3-dev.4-linux-amd64" {
		t.Fatalf("release path %q", got)
	}
	if installs := runner.program("install"); len(installs) != 1 || !slices.Contains(installs[0], "/mnt/c/dl/webmux-0.10.3-dev.4-linux-amd64") {
		t.Fatalf("install argv %q", installs)
	}
}

// Release dev keeps two builds. When GitHub no longer has an older dev
// tray's server, the reason says to update the tray first and still lists
// every source; a release tray's missing file is reported as before.
func TestInstallerDevBuildGone(t *testing.T) {
	gone := httptest.NewServer(http.NotFoundHandler())
	t.Cleanup(gone.Close)
	mirror := newReleaseServer(t, []byte("not found page"))
	for _, tc := range []struct {
		version string
		gone    bool
	}{
		{"0.10.3-dev.4", true},
		{"0.10.2", false},
	} {
		runner := &installRunner{machine: "x86_64", installed: tc.version}
		in := testInstaller(t, runner, gone.URL, map[string]string{"amd64": sum(serverBinary), "arm64": "unused"})
		in.Version = tc.version
		in.Downloader.Mirrors = []string{mirror.URL}
		err := in.Install(context.Background())
		msg := "开发版 0.10.3-dev.4 的服务端已不在 dev Release，请先更新托盘"
		if err == nil || strings.Contains(err.Error(), msg) != tc.gone || !strings.Contains(err.Error(), "HTTP 404") || !strings.Contains(err.Error(), "sha256 mismatch") {
			t.Errorf("%s: %v", tc.version, err)
		}
		if len(runner.program("install")) != 0 {
			t.Errorf("%s: installed without a download", tc.version)
		}
	}
}

// The Windows installer pre-seeds the server where the tray caches
// downloads: a matching file is installed offline, a damaged one is fetched
// again.
func TestInstallerUsesPreseededServer(t *testing.T) {
	for name, tc := range map[string]struct {
		seeded    []byte
		downloads int32
	}{
		"verified file, no network": {seeded: serverBinary, downloads: 0},
		"truncated file refetched":  {seeded: serverBinary[:5], downloads: 1},
	} {
		t.Run(name, func(t *testing.T) {
			srv := newReleaseServer(t, serverBinary)
			runner := &installRunner{machine: "x86_64", installed: "0.2.0"}
			in := testInstaller(t, runner, srv.URL, map[string]string{"amd64": sum(serverBinary), "arm64": "unused"})
			if err := os.MkdirAll(in.Dir, 0o700); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(filepath.Join(in.Dir, "webmux-0.2.0-linux-amd64"), tc.seeded, 0o600); err != nil {
				t.Fatal(err)
			}
			if err := in.Install(context.Background()); err != nil {
				t.Fatal(err)
			}
			if got := srv.hits.Load(); got != tc.downloads {
				t.Fatalf("%d downloads, want %d", got, tc.downloads)
			}
			if len(runner.program("install")) != 1 {
				t.Fatalf("install calls %q", runner.program("install"))
			}
		})
	}
}

func TestInstallerRefusesUnverifiedFiles(t *testing.T) {
	srv := newReleaseServer(t, []byte("tampered"))
	runner := &installRunner{machine: "x86_64", installed: "0.2.0"}
	in := testInstaller(t, runner, srv.URL, map[string]string{"amd64": sum(serverBinary), "arm64": sum(serverBinary)})
	err := in.Install(context.Background())
	var sources *fetch.SourcesError
	if !errors.As(err, &sources) || !strings.Contains(err.Error(), "sha256 mismatch") {
		t.Fatalf("tampered download: %v", err)
	}
	if len(runner.program("install")) != 0 {
		t.Fatal("installed an unverified file")
	}
	entries, _ := os.ReadDir(in.Dir)
	if len(entries) != 0 {
		t.Fatalf("download left behind: %v", entries)
	}
}

func TestInstallerPreconditions(t *testing.T) {
	digests := map[string]string{"amd64": sum(serverBinary), "arm64": sum(serverBinary)}
	for name, tc := range map[string]struct {
		runner  *installRunner
		digests map[string]string
		version string
		want    string
	}{
		"development tray": {&installRunner{machine: "x86_64"}, nil, "0.2.0", ErrInstallUnsupported.Error()},
		"dev version":      {&installRunner{machine: "x86_64"}, digests, "dev", ErrInstallUnsupported.Error()},
		"architecture":     {&installRunner{machine: "riscv64"}, digests, "0.2.0", "不支持的 WSL 架构 riscv64"},
		"no automount":     {&installRunner{machine: "x86_64", noMount: true}, digests, "0.2.0", ErrNoWindowsFiles.Error()},
		"wrong version":    {&installRunner{machine: "x86_64", installed: "0.1.0"}, digests, "0.2.0", `报告版本 "0.1.0"`},
	} {
		t.Run(name, func(t *testing.T) {
			srv := newReleaseServer(t, serverBinary)
			in := testInstaller(t, tc.runner, srv.URL, tc.digests)
			in.Version = tc.version
			err := in.Install(context.Background())
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("error %v, want %q", err, tc.want)
			}
			if tc.want == ErrInstallUnsupported.Error() && (len(tc.runner.calls) != 0 || srv.hits.Load() != 0) {
				t.Fatal("unsupported tray touched WSL or the network")
			}
			if tc.runner.noMount || tc.runner.machine == "riscv64" {
				if len(tc.runner.program("install")) != 0 {
					t.Fatal("install ran after a failed precondition")
				}
			}
		})
	}
}

func TestParseVersion(t *testing.T) {
	for in, want := range map[string]string{"webmux 0.2.0\n": "0.2.0", "webmux dev": "dev", "usage: webmux": "", "": ""} {
		if got := ParseVersion(in); got != want {
			t.Errorf("ParseVersion(%q) = %q, want %q", in, got, want)
		}
	}
}

// A release tray with automatic updates upgrades an older WSL webmux once
// the check finishes; the running instance and its sessions stay, and the
// status line says a restart applies the new version.
func TestAutoUpgradeOlderServer(t *testing.T) {
	service := &fakeService{running: true, launcher: contract.LauncherWindowsTray, record: true, runningAs: "0.2.0", version: "0.2.0", installs: "0.3.0", sessions: 2}
	h := newUpgradingHarness(t, service, DefaultSettings(), "0.3.0", true)
	snap := h.wait("upgraded", func(s Snapshot) bool {
		return s.Compat == CompatMatched && !s.Installing && s.State == StateRunning && s.Sessions == 2
	})
	if service.count("install") != 1 || service.count("startTray") != 0 || service.count("shutdown") != 0 {
		t.Fatalf("calls %v", service.calls)
	}
	if snap.Running == nil || snap.Running.Version != "0.2.0" || VersionHint(snap) != "WSL 中已安装 0.3.0，重启 WebMux 后生效，重启会结束所有会话" {
		t.Fatalf("after upgrade %q %+v", VersionHint(snap), snap.Running)
	}
	if notes := h.notifications(); len(notes) != 0 {
		t.Fatalf("automatic upgrade notified: %v", notes)
	}
}

// A webmux too old for the tray's contract or local API is upgraded too.
func TestAutoUpgradeTooOldServer(t *testing.T) {
	service := &fakeService{checkErr: ErrWebmuxTooOld, installs: "0.3.0"}
	h := newUpgradingHarness(t, service, manualSettings(), "0.3.0", true)
	snap := h.wait("upgraded", func(s Snapshot) bool { return s.Compat == CompatMatched && !s.Installing })
	if service.count("install") != 1 || snap.Blocker != "" || snap.State != StateStopped || service.count("startTray") != 0 {
		t.Fatalf("after upgrade %+v %v", snap, service.calls)
	}
}

// Nothing is written unasked when automatic updates are off, the tray is
// not a release build, or the server is missing, newer or not comparable.
func TestAutoUpgradeOnlyOlderServers(t *testing.T) {
	off := manualSettings()
	off.AutoUpdate = false
	for _, tc := range []struct {
		name     string
		service  *fakeService
		settings Settings
		release  bool
		checked  func(Snapshot) bool
	}{
		{"automatic updates off", &fakeService{version: "0.2.0"}, off, true, func(s Snapshot) bool { return s.Compat == CompatServerOlder }},
		{"development tray", &fakeService{version: "0.2.0"}, manualSettings(), false, func(s Snapshot) bool { return s.Compat == CompatServerOlder }},
		{"missing", &fakeService{checkErr: ErrWebmuxMissing}, manualSettings(), true, func(s Snapshot) bool { return s.Blocker == BlockerMissing }},
		{"newer", &fakeService{version: "0.4.0"}, manualSettings(), true, func(s Snapshot) bool { return s.Compat == CompatServerNewer }},
		{"contract too new", &fakeService{version: "0.2.0", contract: 2}, manualSettings(), true, func(s Snapshot) bool { return s.Compat == CompatIncompatibleNewer }},
		{"not comparable", &fakeService{version: "dev"}, manualSettings(), true, func(s Snapshot) bool { return s.Compat == CompatUnknown }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			h := newUpgradingHarness(t, tc.service, tc.settings, "0.3.0", tc.release)
			snap := h.wait("checked", tc.checked)
			if snap.Installing || tc.service.count("install") != 0 {
				t.Fatalf("installed unasked: %v", tc.service.calls)
			}
		})
	}
}

// Turning automatic updates on upgrades an older server right away.
func TestAutoUpgradeWhenEnabled(t *testing.T) {
	settings := manualSettings()
	settings.AutoUpdate = false
	service := &fakeService{version: "0.2.0", installs: "0.3.0"}
	h := newUpgradingHarness(t, service, settings, "0.3.0", true)
	h.wait("checked", func(s Snapshot) bool { return s.Compat == CompatServerOlder })
	if InstallEntry(h.sup.Snapshot(), true).Title != "升级 WSL 中的 webmux（0.2.0 → 0.3.0）" {
		t.Fatalf("entry %+v", InstallEntry(h.sup.Snapshot(), true))
	}
	settings.AutoUpdate = true
	h.sup.SetSettings(settings)
	h.wait("upgraded", func(s Snapshot) bool { return s.Compat == CompatMatched && !s.Installing })
	if service.count("install") != 1 {
		t.Fatalf("calls %v", service.calls)
	}
}

// A failed automatic upgrade shows why in the status line, leaves the file
// as it was, and waits for the next check instead of retrying.
func TestAutoUpgradeFailure(t *testing.T) {
	service := &fakeService{version: "0.2.0", installErr: errors.New("安装到 /usr/bin/webmux 失败：install: cannot create regular file '/usr/bin/webmux': Permission denied")}
	h := newUpgradingHarness(t, service, manualSettings(), "0.3.0", true)
	snap := h.wait("failed", func(s Snapshot) bool { return s.UpgradeError != "" })
	if !strings.Contains(VersionHint(snap), "自动升级 WSL 中的 webmux 失败：安装到 /usr/bin/webmux 失败") || snap.Compat != CompatServerOlder || snap.Installing {
		t.Fatalf("failure %q %+v", VersionHint(snap), snap)
	}
	h.clock.Advance(3 * StatusInterval)
	if service.count("install") != 1 || len(h.notifications()) != 0 {
		t.Fatalf("retried before the next check: %v %v", service.calls, h.notifications())
	}
	h.sup.Recheck()
	h.wait("second attempt", func(Snapshot) bool { return service.count("install") == 2 })
	snap = h.wait("failed again", func(s Snapshot) bool { return s.UpgradeError != "" && !s.Installing })
	if snap.Installed.Version != "0.2.0" {
		t.Fatalf("file changed %+v", snap.Installed)
	}
}
