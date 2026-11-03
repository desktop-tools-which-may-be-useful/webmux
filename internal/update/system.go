package update

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"strings"
)

// Errors returned by package updates.
var (
	ErrUnsupportedInstall = errors.New("当前安装方式不支持自动更新")
	ErrNotRoot            = errors.New("需要 root 权限")
)

// Method is how the running binary was installed.
type Method string

// Install methods reported by `webmux update check`.
const (
	MethodDeb   Method = "deb"
	MethodRPM   Method = "rpm"
	MethodOther Method = "other"
)

// Runner runs a program with argv (never a shell) and returns its combined
// output.
type Runner func(ctx context.Context, name string, args ...string) ([]byte, error)

// DetectMethod reports which package manager owns path: dpkg, then rpm.
// Missing tools or an unowned path yield MethodOther.
func DetectMethod(ctx context.Context, run Runner, path string) Method {
	if out, err := run(ctx, "dpkg-query", "-S", "--", path); err == nil {
		// "webmux: /usr/bin/webmux"; diversions print other lines first.
		for _, line := range strings.Split(string(out), "\n") {
			if pkg, file, ok := strings.Cut(line, ": "); ok && strings.TrimSpace(file) == path && packageName(pkg) == "webmux" {
				return MethodDeb
			}
		}
	}
	if out, err := run(ctx, "rpm", "-qf", "--qf", "%{NAME}\n", "--", path); err == nil && strings.TrimSpace(string(out)) == "webmux" {
		return MethodRPM
	}
	return MethodOther
}

// packageName drops a dpkg architecture qualifier ("webmux:amd64").
func packageName(s string) string {
	name, _, _ := strings.Cut(strings.TrimSpace(s), ":")
	return name
}

// PackageAsset names the release package for method, version and GOARCH.
func PackageAsset(m Method, version, goarch string) (string, error) {
	debArch, rpmArch := "", ""
	switch goarch {
	case "amd64":
		debArch, rpmArch = "amd64", "x86_64"
	case "arm64":
		debArch, rpmArch = "arm64", "aarch64"
	default:
		return "", fmt.Errorf("没有 %s 架构的安装包", goarch)
	}
	switch m {
	case MethodDeb:
		return "webmux_" + version + "-1_" + debArch + ".deb", nil
	case MethodRPM:
		return "webmux-" + version + "-1." + rpmArch + ".rpm", nil
	}
	return "", ErrUnsupportedInstall
}

// installCommand is the package manager argv for a downloaded package. The
// release rpm carries no rpm signature: its integrity comes from the signed
// SHA256SUMS checked before install, so rpm skips signature checks (Fedora
// 45's rpm 6.1 rejects unsigned packages by default) but still verifies its
// digests.
func installCommand(m Method, file string) (string, []string) {
	if m == MethodRPM {
		return "rpm", []string{"-U", "--nosignature", "--", file}
	}
	return "dpkg", []string{"-i", "--", file}
}

// LockFiles locates the dnf versionlock files Held reads. Empty fields select
// the system paths.
type LockFiles struct {
	DNF4Conf string // dnf4 plugin config; `locklist` in [main] names the list
	DNF4List string // dnf4 list used when the config names none
	DNF5     string // dnf5 versionlock.toml
}

func (f LockFiles) withDefaults() LockFiles {
	if f.DNF4Conf == "" {
		f.DNF4Conf = "/etc/dnf/plugins/versionlock.conf"
	}
	if f.DNF4List == "" {
		f.DNF4List = "/etc/dnf/plugins/versionlock.list"
	}
	if f.DNF5 == "" {
		f.DNF5 = "/etc/dnf/versionlock.toml"
	}
	return f
}

// Held reports whether the administrator pinned the webmux package: an
// `apt-mark hold` for dpkg, or a dnf4 or dnf5 versionlock entry for rpm.
// dpkg -i and rpm -U ignore both, so the updater must check them itself. The
// versionlock files are read directly, because dnf loads repository metadata
// and fails when a repository is unreachable; a missing file holds nothing.
// An error means the state is unknown, and callers must treat it as held.
func Held(ctx context.Context, run Runner, m Method, files LockFiles) (bool, error) {
	switch m {
	case MethodDeb:
		out, err := run(ctx, "dpkg-query", "-W", "-f", "${Status}", "webmux")
		if err != nil {
			if msg := strings.TrimSpace(string(out)); msg != "" {
				err = fmt.Errorf("%w：%s", err, msg)
			}
			return false, fmt.Errorf("dpkg-query：%w", err)
		}
		want, _, _ := strings.Cut(strings.TrimSpace(string(out)), " ")
		return want == "hold", nil
	case MethodRPM:
		files = files.withDefaults()
		list, err := dnf4LockList(files.DNF4Conf, files.DNF4List)
		if err != nil {
			return false, err
		}
		if held, err := dnf4Locked(list); held || err != nil {
			return held, err
		}
		return dnf5Locked(files.DNF5)
	}
	return false, nil
}

// readOptional reads path; a missing file is empty.
func readOptional(path string) ([]byte, error) {
	b, err := os.ReadFile(path)
	if errors.Is(err, fs.ErrNotExist) {
		return nil, nil
	}
	return b, err
}

// dnf4LockList returns the list named by `locklist` in the [main] section of
// the versionlock plugin config, or def when the config does not name one.
func dnf4LockList(conf, def string) (string, error) {
	b, err := readOptional(conf)
	if err != nil {
		return "", err
	}
	section := ""
	for _, line := range strings.Split(string(b), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || line[0] == '#' || line[0] == ';' {
			continue
		}
		if strings.HasPrefix(line, "[") {
			section = strings.TrimSpace(strings.Trim(line, "[]"))
			continue
		}
		if key, value, ok := strings.Cut(line, "="); ok && section == "main" && strings.TrimSpace(key) == "locklist" {
			return strings.TrimSpace(value), nil
		}
	}
	return def, nil
}

// dnf4Locked reports whether the dnf4 versionlock list pins or excludes
// webmux.
func dnf4Locked(path string) (bool, error) {
	b, err := readOptional(path)
	if err != nil {
		return false, err
	}
	for _, line := range strings.Split(string(b), "\n") {
		if versionlocked(strings.TrimSpace(line)) {
			return true, nil
		}
	}
	return false, nil
}

// versionlocked matches a dnf4 versionlock spec for webmux, locked or
// excluded with "!": the bare name, the name with an architecture, or the
// name, a dash, then the epoch or version, which starts with a digit.
func versionlocked(spec string) bool {
	spec = strings.TrimSpace(strings.TrimPrefix(spec, "!"))
	rest, ok := strings.CutPrefix(spec, "webmux")
	switch {
	case !ok:
		return false
	case rest == "" || rest[0] == '.':
		return true
	}
	return len(rest) > 1 && rest[0] == '-' && rest[1] >= '0' && rest[1] <= '9'
}

// dnf5Locked reports whether dnf5's versionlock.toml names webmux. Only
// [[packages]] entries carry a `name` key, so a line scan suffices; it keeps
// a TOML module out of this package, which the release signer links. A name
// whose value is not a plain quoted string cannot be judged and is an error.
func dnf5Locked(path string) (bool, error) {
	b, err := readOptional(path)
	if err != nil {
		return false, err
	}
	for i, line := range strings.Split(string(b), "\n") {
		key, value, ok := strings.Cut(line, "=")
		if !ok || strings.TrimSpace(key) != "name" {
			continue
		}
		value, _, _ = strings.Cut(value, "#")
		value = strings.TrimSpace(value)
		if len(value) < 2 || value[0] != value[len(value)-1] || (value[0] != '"' && value[0] != '\'') {
			return false, fmt.Errorf("%s 第 %d 行：无法解析 name", path, i+1)
		}
		if value[1:len(value)-1] == "webmux" {
			return true, nil
		}
	}
	return false, nil
}
