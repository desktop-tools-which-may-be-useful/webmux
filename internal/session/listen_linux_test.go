//go:build linux

package session

import (
	"encoding/binary"
	"maps"
	"testing"
)

// Addresses as a little-endian kernel prints them.
const procNetTCP = `  sl  local_address rem_address   st tx_queue rx_queue tr tm->when retrnsmt   uid  timeout inode
   0: 0100007F:1F90 00000000:0000 0A 00000000:00000000 00:00000000 00000000  1000        0 101 1 0000000000000000 100 0 0 10 0
   1: 00000000:0BB8 00000000:0000 0A 00000000:00000000 00:00000000 00000000  1000        0 102 1 0000000000000000 100 0 0 10 0
   2: 0A01A8C0:0BB9 00000000:0000 0A 00000000:00000000 00:00000000 00000000  1000        0 103 1 0000000000000000 100 0 0 10 0
   3: 0100007F:1F90 0100007F:D431 01 00000000:00000000 00:00000000 00000000  1000        0 104 1 0000000000000000 20 4 30 10 -1
   4: 0100017F:1F91 00000000:0000 0A 00000000:00000000 00:00000000 00000000  1000        0 105 1 0000000000000000 100 0 0 10 0
   5: 0100007F:1F92 00000000:0000 0A 00000000:00000000 00:00000000 00000000  1000        0 0 1 0000000000000000 100 0 0 10 0
`

const procNetTCP6 = `  sl  local_address                         remote_address                        st tx_queue rx_queue tr tm->when retrnsmt   uid  timeout inode
   0: 00000000000000000000000001000000:1F90 00000000000000000000000000000000:0000 0A 00000000:00000000 00:00000000 00000000  1000        0 201 1 0000000000000000 100 0 0 10 0
   1: 00000000000000000000000000000000:0BB8 00000000000000000000000000000000:0000 0A 00000000:00000000 00:00000000 00000000  1000        0 202 1 0000000000000000 100 0 0 10 0
   2: 0000000000000000FFFF00000100007F:1389 00000000000000000000000000000000:0000 0A 00000000:00000000 00:00000000 00000000  1000        0 203 1 0000000000000000 100 0 0 10 0
   3: B80D0120000000000000000001000000:1F93 00000000000000000000000000000000:0000 0A 00000000:00000000 00:00000000 00000000  1000        0 204 1 0000000000000000 100 0 0 10 0
`

func TestParseProcNetTCP(t *testing.T) {
	if binary.NativeEndian.Uint16([]byte{1, 0}) != 1 {
		t.Skip("fixtures hold little-endian kernel output")
	}
	listen := make(map[uint64]int)
	parseProcNetTCP([]byte(procNetTCP), listen)
	parseProcNetTCP([]byte(procNetTCP6), listen)
	want := map[uint64]int{
		101: 8080, // 127.0.0.1
		102: 3000, // 0.0.0.0
		105: 8081, // 127.1.0.1
		201: 8080, // ::1
		202: 3000, // ::
		203: 5001, // ::ffff:127.0.0.1
	}
	// 192.168.1.10:3001 and 2001:db8::1:8083 are external, 104 is
	// established and 8082 has no inode.
	if !maps.Equal(listen, want) {
		t.Fatalf("listen = %v, want %v", listen, want)
	}
}

func TestParseProcNetTCPRejectsMalformedLines(t *testing.T) {
	listen := make(map[uint64]int)
	parseProcNetTCP([]byte("   0: 0100007F 00000000:0000 0A 0 0 0 0 0 1\n"+
		"   1: 0100007G:1F90 00000000:0000 0A 0 0 0 0 0 2\n"+
		"   2: 0100007F:XYZ 00000000:0000 0A 0 0 0 0 0 3\n"+
		"   3: 01007F:1F90 00000000:0000 0A 0 0 0 0 0 4\n"+
		"   4: 0100007F:1F90 00000000:0000 0A 0 0 0 0 0 inode\n"+
		"   5: 0100007F:1F90 00000000:0000 0A\n"), listen)
	if len(listen) != 0 {
		t.Fatalf("malformed lines parsed: %v", listen)
	}
}
