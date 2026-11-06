//go:build linux

package session

import (
	"bytes"
	"os"
	"strconv"
)

// sessionMarker returns pid's WEBMUX_SESSION_ID, or "" without one.
func sessionMarker(pid int) string {
	data, err := os.ReadFile("/proc/" + strconv.Itoa(pid) + "/environ")
	if err != nil {
		return ""
	}
	for entry := range bytes.SplitSeq(data, []byte{0}) {
		if value, ok := bytes.CutPrefix(entry, []byte("WEBMUX_SESSION_ID=")); ok {
			return string(value)
		}
	}
	return ""
}
