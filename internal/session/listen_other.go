//go:build !linux && !darwin

package session

// ListeningPortsSupported reports whether Info.ListeningPorts is detected on
// this platform.
const ListeningPortsSupported = false

// portScanner finds no sockets: this platform has no supported socket table.
type portScanner struct{}

func (portScanner) scan(processSnapshot, []int) map[int][]ListeningPort { return nil }
