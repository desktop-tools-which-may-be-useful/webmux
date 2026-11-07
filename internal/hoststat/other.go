//go:build !linux && !darwin

package hoststat

func readLoad() *Load     { return nil }
func readMemory() *Memory { return nil }
