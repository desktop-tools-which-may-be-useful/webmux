//go:build unix

package update

import (
	"context"
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"
)

// installedFixture runs real executable scripts at path under a fake clock.
type installedFixture struct {
	t     *testing.T
	path  string
	clock time.Time
	runs  atomic.Int32
	v     *InstalledVersion
}

func newInstalledFixture(t *testing.T, script string) *installedFixture {
	t.Helper()
	f := &installedFixture{t: t, path: filepath.Join(t.TempDir(), "webmux"), clock: time.Unix(1_700_000_000, 0)}
	f.install(script)
	f.v = newInstalledVersion(f.path, "0.4.0", func() time.Time { return f.clock })
	f.v.run = func(ctx context.Context, path string) ([]byte, error) {
		f.runs.Add(1)
		return runVersion(ctx, path)
	}
	return f
}

// install replaces the file the way package managers do: a new inode
// renamed over the old path.
func (f *installedFixture) install(script string) {
	f.t.Helper()
	next := f.path + ".new"
	if err := os.WriteFile(next, []byte("#!/bin/sh\n"+script+"\n"), 0o700); err != nil {
		f.t.Fatal(err)
	}
	if err := os.Rename(next, f.path); err != nil {
		f.t.Fatal(err)
	}
}

func versionScript(version string) string {
	return `echo '{"v":1,"version":"` + version + `","contract":1}'`
}

func (f *installedFixture) after(d time.Duration, want string, runs int32) {
	f.t.Helper()
	f.clock = f.clock.Add(d)
	if got := f.v.Pending(); got != want {
		f.t.Fatalf("Pending() = %q, want %q", got, want)
	}
	if got := f.runs.Load(); got != runs {
		f.t.Fatalf("runs = %d, want %d", got, runs)
	}
}

func TestInstalledVersionUnchangedDoesNotRun(t *testing.T) {
	f := newInstalledFixture(t, versionScript("0.4.0"))
	f.after(0, "", 0)
	f.after(installedInterval, "", 0)
	f.after(installedInterval, "", 0)
}

func TestInstalledVersionReplaced(t *testing.T) {
	f := newInstalledFixture(t, versionScript("0.4.0"))
	f.install(versionScript("0.5.0"))
	// Not before the interval has passed.
	f.after(installedInterval-time.Second, "", 0)
	f.after(time.Second, "0.5.0", 1)
	// Cached by identity.
	f.after(installedInterval, "0.5.0", 1)
	f.install(versionScript("0.6.0"))
	f.after(installedInterval, "0.6.0", 2)
	// Reinstalling the running version clears the notice.
	f.install(versionScript("0.4.0"))
	f.after(installedInterval, "", 3)
}

func TestInstalledVersionEnvironmentHasOnlyPath(t *testing.T) {
	t.Setenv("WEBMUX_INSTALLED_TEST", "9.9.9")
	f := newInstalledFixture(t, versionScript("0.4.0"))
	f.install(`echo "{\"v\":1,\"version\":\"${WEBMUX_INSTALLED_TEST:-0.5.0}\",\"contract\":1}"`)
	f.after(installedInterval, "0.5.0", 1)
}

func TestInstalledVersionDeleted(t *testing.T) {
	f := newInstalledFixture(t, versionScript("0.4.0"))
	f.install(versionScript("0.5.0"))
	f.after(installedInterval, "0.5.0", 1)
	if err := os.Remove(f.path); err != nil {
		t.Fatal(err)
	}
	f.after(installedInterval, "", 1)
	f.after(installedInterval, "", 1)
	f.install(versionScript("0.6.0"))
	f.after(installedInterval, "0.6.0", 2)
}

func TestInstalledVersionRetriesAfterFailure(t *testing.T) {
	for _, tc := range []struct{ name, script string }{
		{"timeout", "exec sleep 10"},
		{"failure", versionScript("0.5.0") + "\nexit 1"},
		{"garbage", "echo webmux 0.5.0"},
		{"empty", `echo '{"v":1}'`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f := newInstalledFixture(t, versionScript("0.4.0"))
			f.v.timeout = 100 * time.Millisecond
			// The same file fails until the marker exists.
			f.install(`if [ -e "$0.ok" ]; then ` + versionScript("0.5.0") + "\nelse\n" + tc.script + "\nfi")
			start := time.Now()
			f.after(installedInterval, "", 1)
			if elapsed := time.Since(start); elapsed > 5*time.Second {
				t.Fatalf("check took %v", elapsed)
			}
			// At most one run per interval.
			f.after(installedInterval-time.Second, "", 1)
			// A failure is not remembered for the same file.
			f.after(time.Second, "", 2)
			if err := os.WriteFile(f.path+".ok", nil, 0o600); err != nil {
				t.Fatal(err)
			}
			f.after(installedInterval, "0.5.0", 3)
			// A success is cached by identity.
			f.after(installedInterval, "0.5.0", 3)
		})
	}
}

func TestInstalledVersionNil(t *testing.T) {
	var v *InstalledVersion
	if got := v.Pending(); got != "" {
		t.Fatalf("nil Pending() = %q", got)
	}
	if got := newInstalledVersion(filepath.Join(t.TempDir(), "missing"), "0.4.0", time.Now).Pending(); got != "" {
		t.Fatalf("missing executable Pending() = %q", got)
	}
}
