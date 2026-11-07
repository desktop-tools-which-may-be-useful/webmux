//go:build !linux && !darwin

package procpeek

// ListeningPortsSupported reports whether Info.ListeningPorts is detected on
// this platform.
const ListeningPortsSupported = false

// PortScanner finds no sockets: this platform has no supported socket table.
type PortScanner struct{}

func (PortScanner) Scan(Snapshot, []int) map[int][]ListeningPort { return nil }
