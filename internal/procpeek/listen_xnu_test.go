package procpeek

import (
	"encoding/binary"
	"net/netip"
	"testing"
)

// socketFDInfo builds a socket_fdinfo with the fields parseSocketFDInfo
// reads at their XNU offsets, written independently of its constants.
func socketFDInfo(kind, state, family uint32, addr netip.Addr, port uint16) []byte {
	b := make([]byte, 792)
	binary.LittleEndian.PutUint32(b[24+160:], family)
	binary.LittleEndian.PutUint32(b[24+232:], kind)
	binary.BigEndian.PutUint16(b[264+4:], port)
	raw := addr.As16()
	if addr.Is4() {
		v4 := addr.As4()
		raw = [16]byte{}
		copy(raw[12:], v4[:])
	}
	copy(b[264+48:], raw[:])
	binary.LittleEndian.PutUint32(b[264+80:], state)
	return b
}

func TestParseSocketFDInfo(t *testing.T) {
	const tcp, listen, inet, inet6 = 2, 1, 2, 30
	for _, row := range []struct {
		name string
		info []byte
		want int
	}{
		{"IPv4 loopback", socketFDInfo(tcp, listen, inet, netip.MustParseAddr("127.0.0.1"), 5173), 5173},
		{"IPv4 loopback network", socketFDInfo(tcp, listen, inet, netip.MustParseAddr("127.1.2.3"), 5174), 5174},
		{"IPv4 wildcard", socketFDInfo(tcp, listen, inet, netip.MustParseAddr("0.0.0.0"), 3000), 3000},
		{"IPv6 loopback", socketFDInfo(tcp, listen, inet6, netip.MustParseAddr("::1"), 8000), 8000},
		{"IPv6 wildcard", socketFDInfo(tcp, listen, inet6, netip.MustParseAddr("::"), 8001), 8001},
		{"IPv4-mapped loopback", socketFDInfo(tcp, listen, inet6, netip.MustParseAddr("::ffff:127.0.0.1"), 8002), 8002},
		{"high port", socketFDInfo(tcp, listen, inet, netip.MustParseAddr("127.0.0.1"), 65535), 65535},
		{"external IPv4", socketFDInfo(tcp, listen, inet, netip.MustParseAddr("192.168.1.10"), 3000), 0},
		{"external IPv6", socketFDInfo(tcp, listen, inet6, netip.MustParseAddr("2001:db8::1"), 3000), 0},
		{"established", socketFDInfo(tcp, 4, inet, netip.MustParseAddr("127.0.0.1"), 5173), 0},
		{"not TCP", socketFDInfo(1, listen, inet, netip.MustParseAddr("127.0.0.1"), 5173), 0},
		{"unix family", socketFDInfo(tcp, listen, 1, netip.MustParseAddr("127.0.0.1"), 5173), 0},
		{"port zero", socketFDInfo(tcp, listen, inet, netip.MustParseAddr("127.0.0.1"), 0), 0},
		{"short", socketFDInfo(tcp, listen, inet, netip.MustParseAddr("127.0.0.1"), 5173)[:791], 0},
	} {
		t.Run(row.name, func(t *testing.T) {
			port, ok := parseSocketFDInfo(row.info)
			if ok != (row.want != 0) || port != row.want {
				t.Fatalf("parseSocketFDInfo() = %d, %v; want %d", port, ok, row.want)
			}
		})
	}
}
