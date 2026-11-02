//go:build linux

package main

import (
	"fmt"
	"os"
	"strconv"
	"strings"
)

func claudeProcessIdentity(pid int) (start, domain string, err error) {
	dir := "/proc/" + strconv.Itoa(pid)
	data, err := os.ReadFile(dir + "/stat")
	if err != nil {
		return "", "", err
	}
	end := strings.LastIndexByte(string(data), ')')
	if end < 0 {
		return "", "", fmt.Errorf("malformed proc stat")
	}
	fields := strings.Fields(string(data[end+1:]))
	if len(fields) < 20 {
		return "", "", fmt.Errorf("short proc stat")
	}
	machineID := ""
	for _, path := range []string{"/etc/machine-id", "/var/lib/dbus/machine-id"} {
		if data, err := os.ReadFile(path); err == nil {
			if id := strings.TrimSpace(string(data)); id != "" {
				machineID = id
				break
			}
		}
	}
	namespace, err := os.Readlink(dir + "/ns/pid")
	if err != nil {
		return "", "", err
	}
	if machineID == "" {
		return "", "", fmt.Errorf("machine ID unavailable")
	}
	return fields[19], "linux:" + machineID + ":" + namespace, nil
}
