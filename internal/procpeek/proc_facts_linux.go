//go:build linux

package procpeek

import (
	"os"
	"strconv"
	"strings"
	"sync"
)

var machineID = sync.OnceValue(func() string {
	for _, path := range []string{"/etc/machine-id", "/var/lib/dbus/machine-id"} {
		if data, err := os.ReadFile(path); err == nil {
			if id := strings.TrimSpace(string(data)); id != "" {
				return id
			}
		}
	}
	return ""
})

// processFacts reads one live process without publishing any of its data.
// procStart is the exact decimal field 22 from stat, and pidDomain is
// linux:<machine-id>:<pid-namespace>, matching Claude's registry format.
func processFacts(pid int) (ProcessFacts, bool) {
	var zero ProcessFacts
	if pid <= 0 {
		return zero, false
	}
	dir := "/proc/" + strconv.Itoa(pid)
	data, err := os.ReadFile(dir + "/stat")
	if err != nil {
		return zero, false
	}
	end := strings.LastIndexByte(string(data), ')')
	if end < 0 {
		return zero, false
	}
	fields := strings.Fields(string(data[end+1:]))
	if len(fields) < 20 || len(fields[0]) != 1 || fields[0][0] == 'Z' {
		return zero, false
	}
	if _, err := strconv.ParseUint(fields[19], 10, 64); err != nil {
		return zero, false
	}
	exe, cwd := RootProcess(pid)
	namespace, err := os.Readlink(dir + "/ns/pid")
	if err != nil || machineID() == "" || exe == "" || cwd == "" {
		return zero, false
	}
	return ProcessFacts{
		PID:        pid,
		Executable: exe,
		Cwd:        cwd,
		Start:      fields[19],
		Domain:     "linux:" + machineID() + ":" + namespace,
	}, true
}
