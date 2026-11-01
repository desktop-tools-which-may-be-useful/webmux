//go:build linux

package session

import (
	"os"
	"strconv"
	"strings"
)

// processOpenFiles lists the regular files pid holds open, by absolute
// path; nothing when its descriptors cannot be read.
func processOpenFiles(pid int) []string {
	dir := "/proc/" + strconv.Itoa(pid) + "/fd/"
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil
	}
	var paths []string
	for _, entry := range entries {
		link, err := os.Readlink(dir + entry.Name())
		if err != nil || !strings.HasPrefix(link, "/") || strings.HasSuffix(link, " (deleted)") {
			continue
		}
		if info, err := os.Stat(link); err == nil && info.Mode().IsRegular() {
			paths = append(paths, link)
		}
	}
	return paths
}
