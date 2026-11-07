package procpeek

import (
	"encoding/binary"
	"net/netip"
)

// struct socket_fdinfo from XNU <sys/proc_info.h>, as PROC_PIDFDSOCKETINFO
// returns it on 64-bit macOS. The layout is parsed by offset so it builds
// and is tested on every platform:
//
//	proc_fileinfo pfi                     0 (24 bytes)
//	socket_info psi                       24
//	  vinfo_stat soi_stat                 +0 (136 bytes)
//	  int soi_family                      +160
//	  int soi_kind                        +232
//	  union soi_proto                     +240 (528 bytes)
//	    tcp_sockinfo pri_tcp
//	      in_sockinfo tcpsi_ini           +0
//	        int insi_lport                +4, network byte order
//	        in4in6_addr|in6_addr laddr    +48 (IPv4 in the last 4 bytes)
//	      int tcpsi_state                 +80
const (
	xnuSocketFDInfoSize = 792
	xnuSocketInfo       = 24
	xnuFamily           = xnuSocketInfo + 160
	xnuKind             = xnuSocketInfo + 232
	xnuProto            = xnuSocketInfo + 240
	xnuLocalPort        = xnuProto + 4
	xnuLocalAddr        = xnuProto + 48
	xnuTCPState         = xnuProto + 80

	xnuAFInet      = 2  // AF_INET
	xnuAFInet6     = 30 // AF_INET6 on Darwin
	xnuSockInfoTCP = 2  // SOCKINFO_TCP
	xnuTCPListen   = 1  // TSI_S_LISTEN
)

// parseSocketFDInfo returns the port of a TCP socket listening on a loopback
// or wildcard address, read from a socket_fdinfo; false for anything else.
// macOS runs only on little-endian CPUs.
func parseSocketFDInfo(b []byte) (int, bool) {
	if len(b) != xnuSocketFDInfoSize {
		return 0, false
	}
	if binary.LittleEndian.Uint32(b[xnuKind:]) != xnuSockInfoTCP || binary.LittleEndian.Uint32(b[xnuTCPState:]) != xnuTCPListen {
		return 0, false
	}
	var addr netip.Addr
	switch binary.LittleEndian.Uint32(b[xnuFamily:]) {
	case xnuAFInet:
		addr = netip.AddrFrom4([4]byte(b[xnuLocalAddr+12 : xnuLocalAddr+16]))
	case xnuAFInet6:
		addr = netip.AddrFrom16([16]byte(b[xnuLocalAddr : xnuLocalAddr+16]))
	default:
		return 0, false
	}
	// The int holds the network-order port in its first two bytes.
	port := int(binary.BigEndian.Uint16(b[xnuLocalPort:]))
	if port == 0 || !localListener(addr) {
		return 0, false
	}
	return port, true
}
