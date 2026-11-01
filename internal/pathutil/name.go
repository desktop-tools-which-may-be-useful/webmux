package pathutil

import (
	"errors"
	"fmt"
	"path/filepath"
	"strings"
	"unicode"
)

// MaxNameBytes is the longest single path segment common filesystems accept.
const MaxNameBytes = 255

// MaxNameAttempts bounds the " (n)" suffixes tried for a taken name.
const MaxNameAttempts = 1000

// ErrName reports a name that cannot be used as one directory entry.
var ErrName = errors.New("invalid file name")

// CleanName checks that name is a single directory entry a user may create:
// not empty, ".", ".." or any case of ".git", without separators or control
// characters, and at most MaxNameBytes long.
func CleanName(name string) (string, error) {
	switch {
	case name == "" || name == "." || name == "..":
		return "", fmt.Errorf("%w: name must name a file", ErrName)
	case strings.EqualFold(name, ".git"):
		return "", fmt.Errorf("%w: .git is reserved", ErrName)
	case strings.ContainsAny(name, `/\`):
		return "", fmt.Errorf("%w: name contains a path separator", ErrName)
	case len(name) > MaxNameBytes:
		return "", fmt.Errorf("%w: name exceeds %d bytes", ErrName, MaxNameBytes)
	}
	for _, ch := range name {
		if unicode.IsControl(ch) {
			return "", fmt.Errorf("%w: name contains a control character", ErrName)
		}
	}
	return name, nil
}

// SuffixedName is name for n <= 1, else name with " (n)" placed before its
// extension. A leading dot alone (".env") is not an extension.
func SuffixedName(name string, n int) string {
	if n <= 1 {
		return name
	}
	ext := filepath.Ext(name)
	if ext == name {
		ext = ""
	}
	return fmt.Sprintf("%s (%d)%s", strings.TrimSuffix(name, ext), n, ext)
}
