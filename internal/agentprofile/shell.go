package agentprofile

import "path/filepath"

// Startup-file shells WebMux can wrap. Other shells are not wrapped.
const (
	ShellBash = "bash"
	ShellZsh  = "zsh"
)

// SupportedStartupShell reports whether shell is a startup-file shell WebMux
// wraps.
func SupportedStartupShell(shell string) bool {
	return shell == ShellBash || shell == ShellZsh
}

// DefaultShell maps the service's $SHELL path to the startup-file shell used
// for new presets and the editor: "bash", "zsh", or "" for a shell WebMux does
// not wrap. An unset path keeps the historical Bash default.
func DefaultShell(path string) string {
	if path == "" {
		return ShellBash
	}
	switch filepath.Base(path) {
	case ShellBash:
		return ShellBash
	case ShellZsh:
		return ShellZsh
	default:
		return ""
	}
}

// StartupArgv wraps program in shell's startup-file form:
// [shell, "-ic", `exec -- "$@"`, "webmux", program, args...]. An unsupported
// shell returns the direct argv.
func StartupArgv(shell, program string, args ...string) []string {
	if !SupportedStartupShell(shell) {
		return append([]string{program}, args...)
	}
	return append([]string{shell, "-ic", startupScript, startupName, program}, args...)
}
