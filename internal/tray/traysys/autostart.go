// Package traysys holds the Windows system administration operations used by
// the WebMux tray and its installer: Windows Defender Firewall rules, login
// item registration, single-instance and quit synchronization, self-update
// installer launch, and host process/file helpers. Platform-independent rule
// and command logic lives in unguarded files so it is tested on every OS.
package traysys

import (
	"path/filepath"
	"strings"
)

// CommandPath extracts the executable path from a Windows Run registry value.
func CommandPath(command string) string {
	command = strings.TrimSpace(command)
	if rest, ok := strings.CutPrefix(command, `"`); ok {
		if end := strings.IndexByte(rest, '"'); end >= 0 {
			return rest[:end]
		}
		return rest
	}
	if end := strings.IndexByte(command, ' '); end >= 0 && strings.HasSuffix(strings.ToLower(command[:end]), ".exe") {
		return command[:end]
	}
	return command
}

// SamePath reports whether two non-empty paths name the same cleaned path,
// ignoring ASCII case.
func SamePath(a, b string) bool {
	if a == "" || b == "" {
		return false
	}
	return strings.EqualFold(filepath.Clean(a), filepath.Clean(b))
}
