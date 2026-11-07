//go:build windows

package traysys

import (
	"errors"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"

	"golang.org/x/sys/windows"
	"golang.org/x/sys/windows/registry"
)

// uninstallKey is UNINST_KEY in deploy/windows/webmux.nsi.
const uninstallKey = `Software\Microsoft\Windows\CurrentVersion\Uninstall\WebMux`

// InstalledCopy reports whether exe runs from the directory the installer
// registered as InstallLocation; any other tray is portable.
func InstalledCopy(exe string) bool {
	k, err := registry.OpenKey(registry.CURRENT_USER, uninstallKey, registry.QUERY_VALUE)
	if err != nil {
		return false
	}
	defer func() { _ = k.Close() }()
	location, _, err := k.GetStringValue("InstallLocation")
	if err != nil {
		return false
	}
	location = strings.TrimRight(strings.Trim(strings.TrimSpace(location), `"`), `\`)
	return SamePath(location, filepath.Dir(exe))
}

// LaunchInstaller starts the downloaded installer silently, detached from
// the tray and without a console; wait returns its exit code. The installer
// normally closes this tray before it exits.
func LaunchInstaller(path string) (wait func() (int, error), err error) {
	cmd := exec.Command(path, "/S")
	cmd.Dir = filepath.Dir(path)
	cmd.SysProcAttr = &syscall.SysProcAttr{CreationFlags: windows.CREATE_NEW_PROCESS_GROUP | windows.DETACHED_PROCESS}
	if err := cmd.Start(); err != nil {
		return nil, err
	}
	return func() (int, error) {
		err := cmd.Wait()
		var exit *exec.ExitError
		if errors.As(err, &exit) {
			return exit.ExitCode(), nil
		}
		if err != nil {
			return 0, err
		}
		return cmd.ProcessState.ExitCode(), nil
	}, nil
}
