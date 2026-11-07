//go:build windows

package traysys

import (
	"errors"

	"golang.org/x/sys/windows/registry"
)

const (
	runKey       = `Software\Microsoft\Windows\CurrentVersion\Run`
	runValueName = "WebMuxTray"
)

// AutostartEnabled reports whether the login item exists and points to exe.
func AutostartEnabled(exe string) bool {
	k, err := registry.OpenKey(registry.CURRENT_USER, runKey, registry.QUERY_VALUE)
	if err != nil {
		return false
	}
	defer func() { _ = k.Close() }()
	value, _, err := k.GetStringValue(runValueName)
	if err != nil {
		return false
	}
	return SamePath(CommandPath(value), exe)
}

// SetAutostart registers exe as a login item, or removes the item.
func SetAutostart(on bool, exe string) error {
	k, _, err := registry.CreateKey(registry.CURRENT_USER, runKey, registry.SET_VALUE)
	if err != nil {
		return err
	}
	defer func() { _ = k.Close() }()
	if on {
		return k.SetStringValue(runValueName, `"`+exe+`"`)
	}
	if err := k.DeleteValue(runValueName); err != nil && !errors.Is(err, registry.ErrNotExist) {
		return err
	}
	return nil
}
