//go:build !linux && !darwin

package main

import "fmt"

func claudeProcessIdentity(int) (string, string, error) {
	return "", "", fmt.Errorf("unsupported platform")
}
