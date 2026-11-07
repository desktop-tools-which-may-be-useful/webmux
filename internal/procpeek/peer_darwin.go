//go:build darwin

package procpeek

// macOS does not expose other processes' environments.
func SessionMarker(int) string { return "" }
