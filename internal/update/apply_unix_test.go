//go:build unix

package update

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync/atomic"
	"syscall"
	"testing"
	"time"

	"github.com/rainchan/webmux/internal/fetch"
)

func asRelease(t *testing.T) {
	t.Helper()
	old := release
	release = "1"
	t.Cleanup(func() { release = old })
}

type call struct {
	name string
	args []string
}

// fakeSystem answers ownership queries for one package manager and records
// installs; installErr fails the install; status is what `dpkg-query -W`
// prints and statusErr fails it; onInstall runs inside the install with its
// context.
type fakeSystem struct {
	owner      Method
	installErr error
	status     string
	statusErr  error
	onInstall  func(ctx context.Context)
	calls      []call
	installed  []byte // package bytes seen at install time
}

func (f *fakeSystem) run(ctx context.Context, name string, args ...string) ([]byte, error) {
	f.calls = append(f.calls, call{name, args})
	switch {
	case name == "dpkg-query" && args[0] == "-W":
		if f.statusErr != nil {
			return []byte("dpkg-query: error: parsing file '/var/lib/dpkg/status'"), f.statusErr
		}
		if f.status != "" {
			return []byte(f.status), nil
		}
		return []byte("install ok installed"), nil
	case name == "dpkg-query" && args[0] == "-S" && f.owner == MethodDeb:
		return []byte("webmux:amd64: /usr/bin/webmux\n"), nil
	case name == "rpm" && args[0] == "-qf" && f.owner == MethodRPM:
		return []byte("webmux\n"), nil
	case name == "dpkg" || (name == "rpm" && args[0] == "-U"):
		f.installed, _ = os.ReadFile(args[len(args)-1])
		if f.onInstall != nil {
			f.onInstall(ctx)
		}
		if f.installErr != nil {
			return []byte("dpkg: error: dpkg frontend lock is locked by another process"), f.installErr
		}
		return nil, nil
	}
	return nil, errors.New("not owned")
}

// applyRelease serves a signed release with deb and rpm packages.
func applyRelease(t *testing.T, latest string) (Client, *atomic.Int32) {
	t.Helper()
	k := testKey(t)
	sums := sumLine("deb-bytes", "webmux_"+latest+"-1_amd64.deb") + sumLine("rpm-bytes", "webmux-"+latest+"-1.aarch64.rpm")
	files := map[string][]byte{
		"SHA256SUMS":                          []byte(sums),
		"SHA256SUMS.minisig":                  signed(t, k, sums, "webmux "+latest),
		"webmux_" + latest + "-1_amd64.deb":   []byte("deb-bytes"),
		"webmux-" + latest + "-1.aarch64.rpm": []byte("rpm-bytes"),
	}
	var hits atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits.Add(1)
		b, ok := files[filepath.Base(r.URL.Path)]
		if !ok {
			http.NotFound(w, r)
			return
		}
		_, _ = w.Write(b)
	}))
	t.Cleanup(srv.Close)
	return Client{Downloader: fetch.Downloader{GitHubBase: srv.URL, Mirrors: []string{}}, StableKeys: []PublicKey{k.Public()}}, &hits
}

// applyOptions runs Apply in a temporary directory, which also holds the
// dnf versionlock files (none exist until a test writes them).
func applyOptions(t *testing.T, c Client, sys *fakeSystem, current, arch string) (ApplyOptions, *bytes.Buffer) {
	dir := t.TempDir()
	var out bytes.Buffer
	return ApplyOptions{
		Current: current, Exe: "/usr/bin/webmux", GOARCH: arch,
		LockPath: filepath.Join(dir, "lock"), CacheDir: filepath.Join(dir, "cache"),
		Client: c, Run: sys.run, Out: &out,
		Locks: LockFiles{
			DNF4Conf: filepath.Join(dir, "versionlock.conf"),
			DNF4List: filepath.Join(dir, "versionlock.list"),
			DNF5:     filepath.Join(dir, "versionlock.toml"),
		},
	}, &out
}

func TestApplyInstallsNewerPackage(t *testing.T) {
	asRelease(t)
	for _, tc := range []struct {
		owner      Method
		arch, want string
		cmd        string
		// argv before the package path; the rpm has no rpm signature,
		// SHA256SUMS authenticates it.
		flags []string
	}{
		{MethodDeb, "amd64", "deb-bytes", "dpkg", []string{"-i", "--"}},
		{MethodRPM, "arm64", "rpm-bytes", "rpm", []string{"-U", "--nosignature", "--"}},
	} {
		c, _ := applyRelease(t, "0.5.0")
		sys := &fakeSystem{owner: tc.owner}
		o, out := applyOptions(t, c, sys, "0.4.0", tc.arch)
		if err := Apply(context.Background(), o); err != nil {
			t.Fatalf("%s: %v", tc.owner, err)
		}
		last := sys.calls[len(sys.calls)-1]
		n := len(last.args) - 1
		if last.name != tc.cmd || n < 0 || !slices.Equal(last.args[:n], tc.flags) || filepath.Dir(last.args[n]) != o.CacheDir {
			t.Fatalf("%s install argv %+v", tc.owner, last)
		}
		if string(sys.installed) != tc.want {
			t.Fatalf("%s installed %q", tc.owner, sys.installed)
		}
		if entries, _ := os.ReadDir(o.CacheDir); len(entries) != 0 {
			t.Fatalf("%s left %v in cache", tc.owner, entries)
		}
		if !strings.Contains(out.String(), "已安装 0.5.0") {
			t.Fatalf("%s log %q", tc.owner, out.String())
		}
	}
}

func TestApplyUpToDateDownloadsNothing(t *testing.T) {
	asRelease(t)
	c, hits := applyRelease(t, "0.5.0")
	sys := &fakeSystem{owner: MethodDeb}
	o, out := applyOptions(t, c, sys, "0.5.0", "amd64")
	if err := Apply(context.Background(), o); err != nil {
		t.Fatal(err)
	}
	if hits.Load() != 2 || !strings.Contains(out.String(), "已是最新版本 0.5.0") {
		t.Fatalf("hits %d log %q", hits.Load(), out.String())
	}
	for _, c := range sys.calls {
		if c.name == "dpkg" {
			t.Fatal("installed while up to date")
		}
	}
}

// Back on the stable channel, a development build above the newest stable
// release is kept and explained; there is no downgrade.
func TestApplyKeepsDevBuildAboveStable(t *testing.T) {
	asRelease(t)
	c, _ := applyRelease(t, "0.10.2")
	sys := &fakeSystem{owner: MethodDeb}
	o, out := applyOptions(t, c, sys, "0.10.3-dev.4", "amd64")
	if err := Apply(context.Background(), o); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), "当前开发版 0.10.3-dev.4 高于最新正式版 0.10.2") {
		t.Fatalf("log %q", out.String())
	}
	for _, c := range sys.calls {
		if c.name == "dpkg" {
			t.Fatal("downgraded to the stable release")
		}
	}
}

// On the dev channel a development build installs from its versioned
// package name (-dev.N in the file name, ~dev.N only inside the package).
func TestApplyInstallsDevPackage(t *testing.T) {
	asRelease(t)
	for _, tc := range []struct {
		owner      Method
		arch, want string
	}{
		{MethodDeb, "amd64", "deb-bytes"},
		{MethodRPM, "arm64", "rpm-bytes"},
	} {
		c, _ := applyRelease(t, "0.10.3-dev.4")
		c.Channel, c.DevKeys, c.StableKeys = ChannelDev, c.StableKeys, []PublicKey{testKey(t).Public()}
		sys := &fakeSystem{owner: tc.owner}
		o, out := applyOptions(t, c, sys, "0.10.2", tc.arch)
		if err := Apply(context.Background(), o); err != nil {
			t.Fatalf("%s: %v", tc.owner, err)
		}
		if string(sys.installed) != tc.want || !strings.Contains(out.String(), "已安装 0.10.3-dev.4") {
			t.Fatalf("%s installed %q, log %q", tc.owner, sys.installed, out.String())
		}
	}
}

func TestApplyRefusesBeforeNetwork(t *testing.T) {
	asRelease(t)
	c, hits := applyRelease(t, "0.5.0")

	o, _ := applyOptions(t, c, &fakeSystem{owner: MethodDeb}, "0.4.0", "amd64")
	o.Euid = 1000
	if err := Apply(context.Background(), o); !errors.Is(err, ErrNotRoot) {
		t.Fatalf("non-root: %v", err)
	}

	o, _ = applyOptions(t, c, &fakeSystem{owner: MethodOther}, "0.4.0", "amd64")
	if err := Apply(context.Background(), o); !errors.Is(err, ErrUnsupportedInstall) {
		t.Fatalf("unowned binary: %v", err)
	}
	if hits.Load() != 0 {
		t.Fatalf("refusals made %d requests", hits.Load())
	}
}

func TestApplyExitsWhileLocked(t *testing.T) {
	asRelease(t)
	c, hits := applyRelease(t, "0.5.0")
	o, _ := applyOptions(t, c, &fakeSystem{owner: MethodDeb}, "0.4.0", "amd64")
	held, err := os.OpenFile(o.LockPath, os.O_RDWR|os.O_CREATE, 0o600)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = held.Close() }()
	if err := syscall.Flock(int(held.Fd()), syscall.LOCK_EX); err != nil {
		t.Fatal(err)
	}
	if err := Apply(context.Background(), o); !errors.Is(err, ErrBusy) {
		t.Fatalf("locked: %v", err)
	}
	if hits.Load() != 0 {
		t.Fatal("locked apply used the network")
	}
}

func TestApplyPackageManagerFailure(t *testing.T) {
	asRelease(t)
	c, _ := applyRelease(t, "0.5.0")
	sys := &fakeSystem{owner: MethodDeb, installErr: errors.New("exit status 2")}
	o, _ := applyOptions(t, c, sys, "0.4.0", "amd64")
	err := Apply(context.Background(), o)
	if err == nil || !strings.Contains(err.Error(), "lock is locked") {
		t.Fatalf("failure: %v", err)
	}
	if entries, _ := os.ReadDir(o.CacheDir); len(entries) != 0 {
		t.Fatalf("left %v in cache", entries)
	}
}

func TestApplyRequiresReleaseBuild(t *testing.T) {
	c, hits := applyRelease(t, "0.5.0")
	o, _ := applyOptions(t, c, &fakeSystem{owner: MethodDeb}, "0.4.0", "amd64")
	if err := Apply(context.Background(), o); !errors.Is(err, ErrNotRelease) || hits.Load() != 0 {
		t.Fatalf("dev build: %v, %d requests", err, hits.Load())
	}
}

// writeLocks creates files in the versionlock directory of o: "/" makes a
// directory, which cannot be read; "{dir}" in a content names that directory.
func writeLocks(t *testing.T, o ApplyOptions, files map[string]string) {
	t.Helper()
	dir := filepath.Dir(o.Locks.DNF4Conf)
	for name, content := range files {
		path := filepath.Join(dir, name)
		var err error
		if content == "/" {
			err = os.Mkdir(path, 0o700)
		} else {
			err = os.WriteFile(path, []byte(strings.ReplaceAll(content, "{dir}", dir)), 0o600)
		}
		if err != nil {
			t.Fatal(err)
		}
	}
}

func TestApplySkipsHeldPackage(t *testing.T) {
	asRelease(t)
	const dnf5 = "version = \"1.0\"\n\n[[packages]]\nname = \"%s\"\ncomment = \"dnf5 versionlock add\"\n[[packages.conditions]]\nkey = \"evr\"\ncomparator = \"=\"\nvalue = \"0.4.0-1\"\n"
	for _, tc := range []struct {
		name      string
		owner     Method
		status    string
		statusErr error
		files     map[string]string
		want      string // log line; "" means the update installs
	}{
		{name: "apt-mark hold", owner: MethodDeb, status: "hold ok installed", want: "已被锁定版本"},
		{name: "dpkg-query fails", owner: MethodDeb, statusErr: errors.New("exit status 2"), want: "无法确认"},
		{name: "dpkg installed", owner: MethodDeb},
		{name: "dnf4 lock", owner: MethodRPM, files: map[string]string{"versionlock.list": "# Added lock on Mon\nwebmux-0:0.4.0-1.*\n"}, want: "已被锁定版本"},
		{name: "dnf4 bare name", owner: MethodRPM, files: map[string]string{"versionlock.list": "webmux\n"}, want: "已被锁定版本"},
		{name: "dnf4 arch", owner: MethodRPM, files: map[string]string{"versionlock.list": "webmux.aarch64\n"}, want: "已被锁定版本"},
		{name: "dnf4 exclude", owner: MethodRPM, files: map[string]string{"versionlock.list": "!webmux-0:0.5.0-1.*\n"}, want: "已被锁定版本"},
		{name: "dnf4 conf locklist", owner: MethodRPM, files: map[string]string{
			"versionlock.conf": "[main]\nenabled = 1\nlocklist = {dir}/custom.list\n",
			"custom.list":      "webmux-0.4.0-1.aarch64\n",
		}, want: "已被锁定版本"},
		{name: "dnf4 conf names another list", owner: MethodRPM, files: map[string]string{
			"versionlock.conf": "[main]\nlocklist = {dir}/custom.list\n",
			"versionlock.list": "webmux\n",
		}},
		{name: "dnf4 other packages", owner: MethodRPM, files: map[string]string{"versionlock.list": "webmux-tray-0:1.0-1.*\nwebmuxd\n#webmux\n"}},
		{name: "dnf5 lock", owner: MethodRPM, files: map[string]string{"versionlock.toml": fmt.Sprintf(dnf5, "webmux")}, want: "已被锁定版本"},
		{name: "dnf5 other package", owner: MethodRPM, files: map[string]string{"versionlock.toml": fmt.Sprintf(dnf5, "webmux-tray")}},
		{name: "dnf5 unparsable", owner: MethodRPM, files: map[string]string{"versionlock.toml": "[[packages]]\nname = webmux\n"}, want: "无法确认"},
		{name: "dnf4 list unreadable", owner: MethodRPM, files: map[string]string{"versionlock.list": "/"}, want: "无法确认"},
		{name: "dnf4 conf unreadable", owner: MethodRPM, files: map[string]string{"versionlock.conf": "/"}, want: "无法确认"},
		{name: "no lock files", owner: MethodRPM},
	} {
		c, hits := applyRelease(t, "0.5.0")
		sys := &fakeSystem{owner: tc.owner, status: tc.status, statusErr: tc.statusErr}
		o, out := applyOptions(t, c, sys, "0.4.0", map[Method]string{MethodDeb: "amd64", MethodRPM: "arm64"}[tc.owner])
		writeLocks(t, o, tc.files)
		if err := Apply(context.Background(), o); err != nil {
			t.Fatalf("%s: %v", tc.name, err)
		}
		if tc.want == "" {
			if !strings.Contains(out.String(), "已安装 0.5.0") {
				t.Fatalf("%s: not installed, log %q", tc.name, out.String())
			}
			continue
		}
		if hits.Load() != 0 || sys.installed != nil || !strings.Contains(out.String(), tc.want) {
			t.Fatalf("%s: %d requests, log %q", tc.name, hits.Load(), out.String())
		}
	}
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

// closeHook runs fn when the response body is closed.
type closeHook struct {
	io.ReadCloser
	fn func()
}

func (c closeHook) Close() error {
	err := c.ReadCloser.Close()
	c.fn()
	return err
}

func TestApplyInstallOutlivesCancel(t *testing.T) {
	asRelease(t)
	c, _ := applyRelease(t, "0.5.0")
	var requests, unbounded atomic.Int32
	c.Downloader.Client = &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		requests.Add(1)
		if d, ok := r.Context().Deadline(); !ok || time.Until(d) > DownloadTimeout {
			unbounded.Add(1)
		}
		return http.DefaultTransport.RoundTrip(r)
	})}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	var installErr error
	installDeadline := false
	sys := &fakeSystem{owner: MethodDeb, onInstall: func(ictx context.Context) {
		cancel() // systemd stops the unit while dpkg runs
		installErr = ictx.Err()
		_, installDeadline = ictx.Deadline()
	}}
	o, out := applyOptions(t, c, sys, "0.4.0", "amd64")
	if err := Apply(ctx, o); err != nil {
		t.Fatal(err)
	}
	if requests.Load() == 0 || unbounded.Load() != 0 {
		t.Fatalf("%d of %d requests lacked the download deadline", unbounded.Load(), requests.Load())
	}
	if installErr != nil || installDeadline {
		t.Fatalf("install context: err %v, deadline %v", installErr, installDeadline)
	}
	if !strings.Contains(out.String(), "已安装 0.5.0") {
		t.Fatalf("log %q", out.String())
	}
}

func TestApplyCancelledAfterDownloadDoesNotInstall(t *testing.T) {
	asRelease(t)
	c, _ := applyRelease(t, "0.5.0")
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	c.Downloader.Client = &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		resp, err := http.DefaultTransport.RoundTrip(r)
		if err == nil && strings.HasSuffix(r.URL.Path, ".deb") {
			resp.Body = closeHook{resp.Body, cancel}
		}
		return resp, err
	})}
	sys := &fakeSystem{owner: MethodDeb}
	o, _ := applyOptions(t, c, sys, "0.4.0", "amd64")
	err := Apply(ctx, o)
	if !errors.Is(err, context.Canceled) || !strings.Contains(err.Error(), "未安装") {
		t.Fatalf("cancelled apply: %v", err)
	}
	if sys.installed != nil {
		t.Fatal("installed after cancel")
	}
	if entries, _ := os.ReadDir(o.CacheDir); len(entries) != 0 {
		t.Fatalf("left %v in cache", entries)
	}
}
