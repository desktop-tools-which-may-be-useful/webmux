//go:build linux

package hoststat

import (
	"bufio"
	"bytes"
	"os"
	"strconv"
	"strings"
)

func readLoad() *Load {
	data, err := os.ReadFile("/proc/loadavg")
	if err != nil {
		return nil
	}
	return parseLoad(string(data))
}

func readMemory() *Memory {
	data, err := os.ReadFile("/proc/meminfo")
	if err != nil {
		return nil
	}
	return parseMemory(data)
}

func parseLoad(text string) *Load {
	fields := strings.Fields(text)
	if len(fields) < 3 {
		return nil
	}
	var v [3]float64
	for i := range v {
		f, err := strconv.ParseFloat(fields[i], 64)
		if err != nil {
			return nil
		}
		v[i] = f
	}
	return &Load{One: v[0], Five: v[1], Fifteen: v[2]}
}

// parseMemory reads MemTotal and MemAvailable (kB) from /proc/meminfo.
func parseMemory(data []byte) *Memory {
	var total, avail uint64
	var haveTotal, haveAvail bool
	sc := bufio.NewScanner(bytes.NewReader(data))
	for sc.Scan() {
		name, rest, ok := strings.Cut(sc.Text(), ":")
		if !ok || (name != "MemTotal" && name != "MemAvailable") {
			continue
		}
		fields := strings.Fields(rest)
		if len(fields) == 0 {
			return nil
		}
		kb, err := strconv.ParseUint(fields[0], 10, 64)
		if err != nil {
			return nil
		}
		if name == "MemTotal" {
			total, haveTotal = kb<<10, true
		} else {
			avail, haveAvail = kb<<10, true
		}
	}
	if !haveTotal || !haveAvail || total == 0 {
		return nil
	}
	return &Memory{Total: total, Available: min(avail, total)}
}
