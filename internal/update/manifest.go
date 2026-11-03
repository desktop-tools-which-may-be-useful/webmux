package update

import (
	"cmp"
	"errors"
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
// trusted comment must be "webmux <version>". A stable key may only sign a
// release version and a development key only a "-dev.N" version, so a leaked
// development key can never produce a list a stable client accepts; pass nil
// devKeys to trust the stable keys alone.
func VerifyManifest(sums, signature []byte, stableKeys, devKeys []PublicKey) (Manifest, error) {
	trusted, err := Verify(sums, signature, stableKeys)
	devSigned := false
	if errors.Is(err, ErrUnknownKey) && len(devKeys) > 0 {
		trusted, err = Verify(sums, signature, devKeys)
		devSigned = true
	}
	if err != nil {
		return Manifest{}, err
	}
	version, ok := strings.CutPrefix(trusted, "webmux ")
	v, valid := ParseVersion(version)
	if !ok || !valid {
		return Manifest{}, fmt.Errorf("%w: trusted comment %q", ErrBadSignature, trusted)
	}
	if v.IsDev() != devSigned {
		return Manifest{}, fmt.Errorf("%w: version %s signed by the wrong key group", ErrBadSignature, version)
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

// Version is a release version MAJOR.MINOR.PATCH, or the development build
// MAJOR.MINOR.PATCH-dev.N built from main before that release. Dev is 0 for
// a release.
type Version struct {
	Core [3]int
	Dev  int
}

// devSuffix is the only pre-release label release builds carry.
const devSuffix = "-dev."

// ParseVersion accepts three non-negative decimal components, optionally
// followed by "-dev.N" with N a positive integer. Leading zeros and any other
// suffix are rejected.
func ParseVersion(s string) (Version, bool) {
	var v Version
	core, dev, isDev := strings.Cut(s, devSuffix)
	if isDev {
		n, ok := decimal(dev)
		if !ok || n == 0 {
			return Version{}, false
		}
		v.Dev = n
	}
	parts := strings.Split(core, ".")
	if len(parts) != 3 {
		return Version{}, false
	}
	for i, p := range parts {
		n, ok := decimal(p)
		if !ok {
			return Version{}, false
		}
		v.Core[i] = n
	}
	return v, true
}

// decimal parses a non-negative integer without sign or leading zeros.
func decimal(p string) (int, bool) {
	if p == "" || strings.Trim(p, "0123456789") != "" || (len(p) > 1 && p[0] == '0') {
		return 0, false
	}
	n, err := strconv.Atoi(p)
	return n, err == nil
}

// IsDev reports whether v is a development build.
func (v Version) IsDev() bool { return v.Dev > 0 }

// Compare returns -1, 0 or 1 by semver precedence: a development build sorts
// below the release with the same MAJOR.MINOR.PATCH.
func (v Version) Compare(o Version) int {
	for i := range v.Core {
		if c := cmp.Compare(v.Core[i], o.Core[i]); c != 0 {
			return c
		}
	}
	switch {
	case v.Dev == o.Dev:
		return 0
	case v.Dev == 0:
		return 1
	case o.Dev == 0:
		return -1
	}
	return cmp.Compare(v.Dev, o.Dev)
}

// IsDevVersion reports whether s is a valid development build version.
func IsDevVersion(s string) bool {
	v, ok := ParseVersion(s)
	return ok && v.IsDev()
}

// Newer reports whether latest is strictly above current; both must be
// release or development versions.
func Newer(latest, current string) bool {
	l, ok1 := ParseVersion(latest)
	c, ok2 := ParseVersion(current)
	return ok1 && ok2 && l.Compare(c) > 0
}
