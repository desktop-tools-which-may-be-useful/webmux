// Package launchicon defines the browser launch-config icon catalog shared by
// launch configs and agent profiles.
package launchicon

// Supported reports whether icon is a browser launch-config icon key. Empty
// keeps legacy launch configs valid; callers choose their own fallback.
func Supported(icon string) bool {
	switch icon {
	case "", "terminal", "claudecode", "codex", "antigravity", "geminicli", "cursor", "githubcopilot", "opencode", "qwen", "kimi", "codebuddy", "pi", "qoder":
		return true
	default:
		return false
	}
}
