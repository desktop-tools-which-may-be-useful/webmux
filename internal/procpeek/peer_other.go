//go:build !linux && !darwin

package procpeek

func SessionMarker(int) string { return "" }
func TreeOwner(Table, int, int, map[int]string) (string, bool) {
	return "", false
}
