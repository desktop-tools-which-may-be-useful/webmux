//go:build darwin

package hoststat

import (
	"encoding/binary"

	"golang.org/x/sys/unix"
)

// loadavg is struct loadavg from <sys/sysctl.h>: three fixed-point values
// scaled by Fscale (the int64 sits after 4 bytes of padding).
const loadavgSize = 24

func readLoad() *Load {
	raw, err := unix.SysctlRaw("vm.loadavg")
	if err != nil || len(raw) < loadavgSize {
		return nil
	}
	scale := float64(int64(binary.NativeEndian.Uint64(raw[16:24])))
	if scale <= 0 {
		return nil
	}
	at := func(i int) float64 {
		return float64(binary.NativeEndian.Uint32(raw[i*4:])) / scale
	}
	return &Load{One: at(0), Five: at(1), Fifteen: at(2)}
}

// readMemory counts free, speculative, purgeable and file-backed pages as
// available, like Activity Monitor's "cached files" plus free memory.
func readMemory() *Memory {
	total, err := unix.SysctlUint64("hw.memsize")
	if err != nil || total == 0 {
		return nil
	}
	pageSize, err := unix.SysctlUint32("vm.pagesize")
	if err != nil {
		return nil
	}
	var pages uint64
	for _, name := range []string{
		"vm.page_free_count",
		"vm.page_speculative_count",
		"vm.page_purgeable_count",
		"vm.page_pageable_external_count",
	} {
		n, err := unix.SysctlUint32(name)
		if err != nil {
			return nil
		}
		pages += uint64(n)
	}
	return &Memory{Total: total, Available: min(pages*uint64(pageSize), total)}
}
