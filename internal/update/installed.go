package update

import (
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"sync"
	"time"

	"github.com/rainchan/webmux/internal/contract"
)

const (
	// installedInterval is the minimum time between two looks at the
	// executable file.
	installedInterval = 60 * time.Second
	// installedTimeout bounds one `version --json` run.
	installedTimeout = 5 * time.Second
)

// InstalledVersion reports the WebMux version installed at the running
// executable's path, which differs from the running version once a package
// upgrade or the self-updater replaced the file. It checks on demand, at
// most once per interval, and runs the file only when its identity changed
// or the last run failed, so an idle server costs nothing. It is safe for
// concurrent use.
type InstalledVersion struct {
	path    string
	running string
	now     func() time.Time
	run     func(ctx context.Context, path string) ([]byte, error)
	timeout time.Duration

	mu       sync.Mutex
	checked  time.Time
	checking bool
	// identity is the file last looked at (zero when it was missing) and
	// version its installed version ("" when unknown).
	identity fileIdentity
	version  string
}

// fileIdentity tells a replaced executable from the original. dev and ino
// are zero where the platform does not report them.
type fileIdentity struct {
	dev, ino uint64
	size     int64
	mtime    int64
}

// NewInstalledVersion records the identity of the running executable, with
// symbolic links resolved. running is this process's version. When the
// executable cannot be located it never reports a pending version.
func NewInstalledVersion(running string) *InstalledVersion {
	path, err := os.Executable()
	if err == nil {
		path, err = filepath.EvalSymlinks(path)
	}
	if err != nil {
		path = ""
	}
	return newInstalledVersion(path, running, time.Now)
}

func newInstalledVersion(path, running string, now func() time.Time) *InstalledVersion {
	v := &InstalledVersion{running: running, now: now, run: runVersion, timeout: installedTimeout, checked: now()}
	if path == "" {
		return v
	}
	id, err := statIdentity(path)
	if err != nil {
		return v
	}
	v.path, v.identity, v.version = path, id, running
	return v
}

// Pending returns the installed version when it is known and differs from
// the running version, and "" otherwise. A nil receiver returns "".
func (v *InstalledVersion) Pending() string {
	if v == nil || v.path == "" {
		return ""
	}
	v.mu.Lock()
	now := v.now()
	if v.checking || now.Sub(v.checked) < installedInterval {
		version := v.version
		v.mu.Unlock()
		return v.pending(version)
	}
	v.checking, v.checked = true, now
	known, version := v.identity, v.version
	v.mu.Unlock()

	id, err := statIdentity(v.path)
	switch {
	case err != nil:
		id, version = fileIdentity{}, ""
	case id != known:
		var ok bool
		if version, ok = v.query(); !ok {
			// Keep the old identity so the next check asks again.
			id = known
		}
	}
	v.mu.Lock()
	v.identity, v.version, v.checking = id, version, false
	v.mu.Unlock()
	return v.pending(version)
}

func (v *InstalledVersion) pending(version string) string {
	if version == v.running {
		return ""
	}
	return version
}

// query runs the executable's `version --json`. ok is false when the run
// failed or its output is not a version reply.
func (v *InstalledVersion) query() (version string, ok bool) {
	ctx, cancel := context.WithTimeout(context.Background(), v.timeout)
	defer cancel()
	out, err := v.run(ctx, v.path)
	if err != nil {
		return "", false
	}
	var reply contract.VersionV1
	if json.Unmarshal(out, &reply) != nil || reply.Version == "" {
		return "", false
	}
	return reply.Version, true
}

// runVersion executes path as argv with only PATH in its environment.
func runVersion(ctx context.Context, path string) ([]byte, error) {
	cmd := exec.CommandContext(ctx, path, "version", "--json")
	cmd.Env = []string{}
	if value, ok := os.LookupEnv("PATH"); ok {
		cmd.Env = append(cmd.Env, "PATH="+value)
	}
	// A grandchild holding stdout open must not outlive the timeout.
	cmd.WaitDelay = time.Second
	return cmd.Output()
}

func statIdentity(path string) (fileIdentity, error) {
	info, err := os.Stat(path)
	if err != nil {
		return fileIdentity{}, err
	}
	id := fileIdentity{size: info.Size(), mtime: info.ModTime().UnixNano()}
	id.dev, id.ino = fileID(info)
	return id, nil
}
