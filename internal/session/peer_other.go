//go:build !linux && !darwin

package session

func sessionMarker(int) string { return "" }
func treeOwner(processTable, int, int, map[int]string) (string, bool) {
	return "", false
}
