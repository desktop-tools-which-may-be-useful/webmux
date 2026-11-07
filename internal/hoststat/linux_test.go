//go:build linux

package hoststat

import "testing"

func TestParseMemoryUsesAvailableNotFree(t *testing.T) {
	m := parseMemory([]byte("MemTotal:       16000000 kB\nMemFree:          100000 kB\nMemAvailable:    8000000 kB\n"))
	if m == nil || m.Total != 16000000<<10 || m.Available != 8000000<<10 {
		t.Fatalf("got %+v", m)
	}
}

func TestParseMemoryRejectsMissingAvailable(t *testing.T) {
	if m := parseMemory([]byte("MemTotal: 1 kB\n")); m != nil {
		t.Fatalf("got %+v", m)
	}
}

func TestParseLoad(t *testing.T) {
	l := parseLoad("0.52 1.25 2.00 1/300 12345\n")
	if l == nil || l.One != 0.52 || l.Five != 1.25 || l.Fifteen != 2 {
		t.Fatalf("got %+v", l)
	}
	if parseLoad("garbage") != nil {
		t.Fatal("garbage must be unavailable")
	}
}
