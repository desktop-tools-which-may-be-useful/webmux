// Package update discovers and downloads signed WebMux releases. Every
// checksum list must carry a minisign signature from a built-in release key
// whose trusted comment names the version, so a mirror can withhold an
// update but never change its content. It depends only on the standard
// library and other shared packages, and builds on every client platform.
package update

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"

	"github.com/rainchan/webmux/internal/fetch"
)

// Repo is the GitHub owner/name serving releases.
const Repo = "mzdluo123/webmux"

// Set with -ldflags -X. `make release` sets release=1 and nothing else;
// keysOverride (comma-separated public key lines) and githubBase exist only
// for local smoke builds against a test release.
var (
	release      string
	keysOverride string
	githubBase   string
)

// releaseKeys are the trusted release public keys (minisign base64 lines).
// Rotate by shipping the new key beside the old one first; see design D2.
var releaseKeys = []string{
	"RWQchIwF6KH3SuZyvOfqZZAoobboXxcEAyyLB3YptnKdLs7vUvxLHm4e", // 4AF7A1E8058C841C
}

const (
	sumsName = "SHA256SUMS"
	sigName  = "SHA256SUMS.minisig"
	// Checksum lists hold a few dozen lines.
	manifestLimit = 64 << 10
)

// ErrNotRelease means this binary was not built by the release pipeline.
var ErrNotRelease = errors.New("此构建不是发布构建，不支持更新")

// IsRelease reports whether this binary came from `make release`.
func IsRelease() bool { return release == "1" }

// Keys returns the trusted public keys.
func Keys() []PublicKey {
	lines := releaseKeys
	if keysOverride != "" {
		lines = strings.Split(keysOverride, ",")
	}
	keys := make([]PublicKey, 0, len(lines))
	for _, line := range lines {
		k, err := ParsePublicKey(line)
		if err != nil {
			panic("update: invalid built-in release key: " + err.Error())
		}
		keys = append(keys, k)
	}
	return keys
}

// Client checks and downloads releases.
type Client struct {
	// Downloader supplies sources, timeouts and logging. A zero GitHubBase
	// selects the build's override, then GitHub.
	Downloader fetch.Downloader
	// Keys nil selects Keys().
	Keys []PublicKey
}

func (c Client) downloader() fetch.Downloader {
	d := c.Downloader
	if d.GitHubBase == "" {
		d.GitHubBase = githubBase
	}
	if d.Name == "" {
		d.Name = "webmux update"
	}
	return d
}

// Latest reads the newest release's signed checksum list. Each source must
// serve both files and a valid signature. GitHub is authoritative and ends
// the search; a mirror may serve a stale but validly signed list, so when
// GitHub fails every mirror is read and the highest verified version wins,
// the earlier mirror on a tie. It returns the manifest and the host that
// served it. A context error is returned unwrapped.
func (c Client) Latest(ctx context.Context) (Manifest, string, error) {
	keys := c.Keys
	if keys == nil {
		keys = Keys()
	}
	d := c.downloader()
	logger := d.Logger
	if logger == nil {
		logger = slog.Default()
	}
	var (
		best     Manifest
		bestHost string
		found    bool
		failures []string
	)
	for i, source := range d.Sources("/" + Repo + "/releases/latest/download/" + sumsName) {
		if err := ctx.Err(); err != nil {
			return Manifest{}, "", err
		}
		host := fetch.SourceHost(source)
		m, err := latestFrom(ctx, d, source, keys)
		if err == nil {
			if i == 0 {
				return m, host, nil
			}
			if !found || Newer(m.Version, best.Version) {
				best, bestHost, found = m, host, true
			}
			continue
		}
		if ctx.Err() != nil {
			return Manifest{}, "", ctx.Err()
		}
		logger.Warn(d.Name+" download source failed", "source", host, "error", err)
		failures = append(failures, host+": "+err.Error())
	}
	if !found {
		return Manifest{}, "", &fetch.SourcesError{Failures: failures}
	}
	return best, bestHost, nil
}

// latestFrom reads and verifies the checksum list and its signature from
// one source.
func latestFrom(ctx context.Context, d fetch.Downloader, source string, keys []PublicKey) (Manifest, error) {
	sums, err := d.Get(ctx, source, manifestLimit)
	if err != nil {
		return Manifest{}, fmt.Errorf("%s: %w", sumsName, err)
	}
	sig, err := d.Get(ctx, strings.TrimSuffix(source, sumsName)+sigName, manifestLimit)
	if err != nil {
		return Manifest{}, fmt.Errorf("%s: %w", sigName, err)
	}
	return VerifyManifest(sums, sig, keys)
}

// Fetch downloads asset from the release tag of m into target with mode,
// verifying it against m. target is untouched unless a source succeeds.
func (c Client) Fetch(ctx context.Context, m Manifest, asset, target string, mode os.FileMode) (string, error) {
	sum, ok := m.SHA256(asset)
	if !ok {
		return "", fmt.Errorf("%s %s 中没有 %s", sumsName, m.Version, asset)
	}
	if filepath.Base(asset) != asset {
		return "", fmt.Errorf("invalid asset name %q", asset)
	}
	return c.downloader().Download(ctx, fetch.Asset{
		Path:   "/" + Repo + "/releases/download/v" + m.Version + "/" + asset,
		SHA256: sum,
		Mode:   mode,
	}, target)
}
