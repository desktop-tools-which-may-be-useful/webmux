//go:build darwin

package session

// macOS does not expose other processes' environments.
func sessionMarker(int) string { return "" }
