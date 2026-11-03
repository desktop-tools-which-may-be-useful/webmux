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
// keysOverride and devKeysOverride (comma-separated public key lines) and
// githubBase exist only for local smoke builds against a test release.
var (
	release         string
	keysOverride    string
	devKeysOverride string
	githubBase      string
)

// stableKeys are the trusted release public keys (minisign base64 lines);
// they sign only MAJOR.MINOR.PATCH versions. Rotate by shipping the new key
// beside the old one first; see D2 of the archived add-auto-update design.
var stableKeys = []string{
	"RWQchIwF6KH3SuZyvOfqZZAoobboXxcEAyyLB3YptnKdLs7vUvxLHm4e", // 4AF7A1E8058C841C
}

// devKeys sign only MAJOR.MINOR.PATCH-dev.N development builds and are
// trusted only on the dev channel, so their leak never reaches stable users.
var devKeys = []string{
	"RWSz8HBG69/zIIBeRUfEmjRf51vJmk7zueyTFeYAUemipmf+GU9cCiLr", // 20F3DFEB4670F0B3
}

const (
	sumsName = "SHA256SUMS"
	sigName  = "SHA256SUMS.minisig"
	// Checksum lists hold a few dozen lines.
	manifestLimit = 64 << 10
	// devTag is the single rolling pre-release holding development builds.
	devTag = "dev"
)

// ErrNotRelease means this binary was not built by the release pipeline.
var ErrNotRelease = errors.New("此构建不是发布构建，不支持更新")

// IsRelease reports whether this binary came from `make release`.
func IsRelease() bool { return release == "1" }

// StableKeys returns the trusted stable release keys.
func StableKeys() []PublicKey { return parseKeys(stableKeys, keysOverride) }

// DevKeys returns the trusted development build keys.
func DevKeys() []PublicKey { return parseKeys(devKeys, devKeysOverride) }

func parseKeys(builtIn []string, override string) []PublicKey {
	lines := builtIn
	if override != "" {
		lines = strings.Split(override, ",")
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

// AssetName is the file name of a release asset in the release holding
// version: stable releases use name unchanged, development builds insert the
// version after the "webmux" prefix, e.g. webmux-0.10.3-dev.2-linux-amd64.
// Linux packages already carry the version and come from PackageAsset.
func AssetName(version, name string) string {
	if !IsDevVersion(version) {
		return name
	}
	return "webmux-" + version + "-" + strings.TrimPrefix(name, "webmux-")
}

// ReleaseTag is the GitHub release tag holding version.
func ReleaseTag(version string) string {
	if IsDevVersion(version) {
		return devTag
	}
	return "v" + version
}

// ReleasePage is the browser URL of the release holding version.
func ReleasePage(version string) string {
	return "https://github.com/" + Repo + "/releases/tag/" + ReleaseTag(version)
}

// Client checks and downloads releases.
type Client struct {
	// Downloader supplies sources, timeouts and logging. A zero GitHubBase
	// selects the build's override, then GitHub.
	Downloader fetch.Downloader
	// Channel selects which releases count; "" means ChannelStable.
	Channel Channel
	// StableKeys nil selects StableKeys(); DevKeys nil selects DevKeys().
	// DevKeys are consulted only on the dev channel.
	StableKeys []PublicKey
	DevKeys    []PublicKey
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

// Latest reads the newest signed checksum list of the client's channel.
// The stable channel reads the latest release, trusting the stable keys
// only. The dev channel also reads the rolling dev release and returns the
// higher of the two, so a stable release supersedes the development builds
// before it; one list failing does not void the other. It returns the
// manifest and the host that served it. A context error is returned
// unwrapped.
func (c Client) Latest(ctx context.Context) (Manifest, string, error) {
	stable := c.StableKeys
	if stable == nil {
		stable = StableKeys()
	}
	if c.Channel != ChannelDev {
		return c.latestAt(ctx, "latest/download", stable, nil)
	}
	dev := c.DevKeys
	if dev == nil {
		dev = DevKeys()
	}
	dm, dhost, derr := c.latestAt(ctx, "download/"+devTag, stable, dev)
	if derr != nil && ctx.Err() != nil {
		return Manifest{}, "", ctx.Err()
	}
	sm, shost, serr := c.latestAt(ctx, "latest/download", stable, dev)
	switch {
	case derr != nil && serr != nil:
		return Manifest{}, "", errors.Join(fmt.Errorf("dev: %w", derr), fmt.Errorf("stable: %w", serr))
	case derr != nil:
		c.logger().Warn(c.downloader().Name+" dev release unavailable", "error", derr)
		return sm, shost, nil
	case serr != nil:
		if ctx.Err() != nil {
			return Manifest{}, "", ctx.Err()
		}
		c.logger().Warn(c.downloader().Name+" stable release unavailable", "error", serr)
		return dm, dhost, nil
	case Newer(sm.Version, dm.Version):
		return sm, shost, nil
	}
	return dm, dhost, nil
}

func (c Client) logger() *slog.Logger {
	if l := c.Downloader.Logger; l != nil {
		return l
	}
	return slog.Default()
}

// latestAt reads one release's checksum list under releases/<dir>. Each
// source must serve both files and a valid signature. GitHub is
// authoritative and ends the search; a mirror may serve a stale but validly
// signed list, so when GitHub fails every mirror is read and the highest
// verified version wins, the earlier mirror on a tie.
func (c Client) latestAt(ctx context.Context, dir string, stableKeys, devKeys []PublicKey) (Manifest, string, error) {
	d := c.downloader()
	logger := c.logger()
	var (
		best     Manifest
		bestHost string
		found    bool
		failures []string
	)
	for i, source := range d.Sources("/" + Repo + "/releases/" + dir + "/" + sumsName) {
		if err := ctx.Err(); err != nil {
			return Manifest{}, "", err
		}
		host := fetch.SourceHost(source)
		m, err := latestFrom(ctx, d, source, stableKeys, devKeys)
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
func latestFrom(ctx context.Context, d fetch.Downloader, source string, stableKeys, devKeys []PublicKey) (Manifest, error) {
	sums, err := d.Get(ctx, source, manifestLimit)
	if err != nil {
		return Manifest{}, fmt.Errorf("%s: %w", sumsName, err)
	}
	sig, err := d.Get(ctx, strings.TrimSuffix(source, sumsName)+sigName, manifestLimit)
	if err != nil {
		return Manifest{}, fmt.Errorf("%s: %w", sigName, err)
	}
	return VerifyManifest(sums, sig, stableKeys, devKeys)
}

// Fetch downloads asset from the release holding m (tag v<version>, or the
// dev release for a development build) into target with mode, verifying it
// against m. Development asset names carry the version; see AssetName and
// PackageAsset. target is untouched unless a source succeeds.
func (c Client) Fetch(ctx context.Context, m Manifest, asset, target string, mode os.FileMode) (string, error) {
	sum, ok := m.SHA256(asset)
	if !ok {
		return "", fmt.Errorf("%s %s 中没有 %s", sumsName, m.Version, asset)
	}
	if filepath.Base(asset) != asset {
		return "", fmt.Errorf("invalid asset name %q", asset)
	}
	return c.downloader().Download(ctx, fetch.Asset{
		Path:   "/" + Repo + "/releases/download/" + ReleaseTag(m.Version) + "/" + asset,
		SHA256: sum,
		Mode:   mode,
	}, target)
}
