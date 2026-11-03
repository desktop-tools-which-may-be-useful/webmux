package update

import (
	"fmt"
	"strconv"
	"strings"
)

// Manifest is a release's checksum list whose signature has been verified.
// Version comes from the signed trusted comment, never from a URL.
type Manifest struct {
	Version string
	sums    map[string]string
}

// SHA256 returns the hex digest listed for an asset.
func (m Manifest) SHA256(name string) (string, bool) {
	sum, ok := m.sums[name]
	return sum, ok
}

// VerifyManifest authenticates SHA256SUMS with its minisign signature. The
// trusted comment must be "webmux <MAJOR.MINOR.PATCH>".
func VerifyManifest(sums, signature []byte, keys []PublicKey) (Manifest, error) {
	trusted, err := Verify(sums, signature, keys)
	if err != nil {
		return Manifest{}, err
	}
	version, ok := strings.CutPrefix(trusted, "webmux ")
	if _, valid := ParseVersion(version); !ok || !valid {
		return Manifest{}, fmt.Errorf("%w: trusted comment %q", ErrBadSignature, trusted)
	}
	parsed, err := ParseSums(sums)
	if err != nil {
		return Manifest{}, err
	}
	return Manifest{Version: version, sums: parsed}, nil
}

// ParseSums reads sha256sum output: "<hex>  <name>" or "<hex> *<name>".
func ParseSums(b []byte) (map[string]string, error) {
	sums := map[string]string{}
	for n, line := range strings.Split(string(b), "\n") {
		line = strings.TrimSuffix(line, "\r")
		if line == "" {
			continue
		}
		sum, name, ok := strings.Cut(line, " ")
		name = strings.TrimPrefix(strings.TrimPrefix(name, " "), "*")
		if !ok || len(sum) != 64 || strings.Trim(sum, "0123456789abcdef") != "" || name == "" {
			return nil, fmt.Errorf("SHA256SUMS line %d is malformed", n+1)
		}
		sums[name] = sum
	}
	return sums, nil
}

// Version is a release version MAJOR.MINOR.PATCH.
type Version [3]int

// ParseVersion accepts exactly three non-negative decimal components.
func ParseVersion(s string) (Version, bool) {
	var v Version
	parts := strings.Split(s, ".")
	if len(parts) != 3 {
		return v, false
	}
	for i, p := range parts {
		if p == "" || strings.Trim(p, "0123456789") != "" || (len(p) > 1 && p[0] == '0') {
			return v, false
		}
		n, err := strconv.Atoi(p)
		if err != nil {
			return v, false
		}
		v[i] = n
	}
	return v, true
}

// Compare returns -1, 0 or 1.
func (v Version) Compare(o Version) int {
	for i := range v {
		switch {
		case v[i] < o[i]:
			return -1
		case v[i] > o[i]:
			return 1
		}
	}
	return 0
}

// Newer reports whether latest is strictly above current; both must be
// release versions.
func Newer(latest, current string) bool {
	l, ok1 := ParseVersion(latest)
	c, ok2 := ParseVersion(current)
	return ok1 && ok2 && l.Compare(c) > 0
}
