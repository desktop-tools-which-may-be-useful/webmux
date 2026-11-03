package update

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/rainchan/webmux/internal/fetch"
)

func testKey(t *testing.T) SecretKey {
	t.Helper()
	k, err := GenerateKey()
	if err != nil {
		t.Fatal(err)
	}
	return k
}

func signed(t *testing.T, k SecretKey, sums, trusted string) []byte {
	t.Helper()
	sig, err := k.Sign([]byte(sums), trusted)
	if err != nil {
		t.Fatal(err)
	}
	return sig
}

func sumLine(content, name string) string {
	h := sha256.Sum256([]byte(content))
	return hex.EncodeToString(h[:]) + "  " + name + "\n"
}

func TestBuiltInKeysParse(t *testing.T) {
	if len(Keys()) == 0 {
		t.Fatal("no release keys")
	}
}

func TestVerifyManifest(t *testing.T) {
	k := testKey(t)
	other := testKey(t)
	keys := []PublicKey{k.Public()}
	sums := sumLine("payload", "webmux-linux-amd64")

	m, err := VerifyManifest([]byte(sums), signed(t, k, sums, "webmux 0.5.0"), keys)
	if err != nil || m.Version != "0.5.0" {
		t.Fatalf("valid manifest: %v %+v", err, m)
	}
	if sum, ok := m.SHA256("webmux-linux-amd64"); !ok || len(sum) != 64 {
		t.Fatalf("sum %q %v", sum, ok)
	}

	sig := signed(t, k, sums, "webmux 0.5.0")
	cases := map[string]struct {
		sums, sig []byte
		want      error
	}{
		"tampered content": {[]byte(sumLine("evil", "webmux-linux-amd64")), sig, ErrBadSignature},
		"tampered comment": {[]byte(sums), bytes.Replace(sig, []byte("webmux 0.5.0"), []byte("webmux 0.6.0"), 1), ErrBadSignature},
		"unknown key":      {[]byte(sums), signed(t, other, sums, "webmux 0.5.0"), ErrUnknownKey},
		"comment format":   {[]byte(sums), signed(t, k, sums, "timestamp:1 file:SHA256SUMS"), ErrBadSignature},
		"comment version":  {[]byte(sums), signed(t, k, sums, "webmux dev"), ErrBadSignature},
		"truncated sig":    {[]byte(sums), sig[:len(sig)/2], ErrBadSignature},
	}
	for name, c := range cases {
		if _, err := VerifyManifest(c.sums, c.sig, keys); !errors.Is(err, c.want) {
			t.Errorf("%s: got %v, want %v", name, err, c.want)
		}
	}
}

func TestSecretKeyRoundTrip(t *testing.T) {
	k := testKey(t)
	back, err := ParseSecretKey(k.String())
	if err != nil || back.Public().String() != k.Public().String() {
		t.Fatalf("round trip: %v", err)
	}
	pub, err := ParsePublicKey(k.Public().File())
	if err != nil || pub.String() != k.Public().String() {
		t.Fatalf("public file: %v", err)
	}
}

func TestVersions(t *testing.T) {
	for _, bad := range []string{"", "dev", "1.2", "1.2.3.4", "v1.2.3", "01.2.3", "1.2.-3", "1.2.3-rc1"} {
		if _, ok := ParseVersion(bad); ok {
			t.Errorf("%q parsed", bad)
		}
	}
	cases := []struct {
		latest, current string
		newer           bool
	}{
		{"0.5.0", "0.4.9", true},
		{"0.10.0", "0.9.0", true},
		{"1.0.0", "0.99.99", true},
		{"0.5.0", "0.5.0", false},
		{"0.4.0", "0.5.0", false},
		{"0.5.0", "dev", false},
	}
	for _, c := range cases {
		if got := Newer(c.latest, c.current); got != c.newer {
			t.Errorf("Newer(%s, %s) = %v", c.latest, c.current, got)
		}
	}
}

func TestParseSumsRejectsMalformed(t *testing.T) {
	if _, err := ParseSums([]byte("xyz  name\n")); err == nil {
		t.Fatal("malformed accepted")
	}
	sums, err := ParseSums([]byte(strings.Repeat("a", 64) + " *bin\n\n"))
	if err != nil || sums["bin"] != strings.Repeat("a", 64) {
		t.Fatalf("binary marker: %v %v", sums, err)
	}
}

// release serves files by name under any path, recording requested paths.
type fakeRelease struct {
	files map[string][]byte
	paths []string
}

func (r *fakeRelease) handler(w http.ResponseWriter, req *http.Request) {
	r.paths = append(r.paths, req.URL.Path)
	b, ok := r.files[filepath.Base(req.URL.Path)]
	if !ok {
		http.NotFound(w, req)
		return
	}
	_, _ = w.Write(b)
}

func serve(t *testing.T, r *fakeRelease) string {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(r.handler))
	t.Cleanup(srv.Close)
	return srv.URL
}

func TestLatestFallsBackAcrossSources(t *testing.T) {
	k := testKey(t)
	payload := "new binary"
	sums := sumLine(payload, "webmux-linux-amd64")
	// GitHub serves a forged list under a valid-looking signature.
	forged := sumLine("evil", "webmux-linux-amd64")
	gh := &fakeRelease{files: map[string][]byte{
		"SHA256SUMS":         []byte(forged),
		"SHA256SUMS.minisig": signed(t, k, sums, "webmux 0.6.0"),
	}}
	mirror := &fakeRelease{files: map[string][]byte{
		"SHA256SUMS":         []byte(sums),
		"SHA256SUMS.minisig": signed(t, k, sums, "webmux 0.6.0"),
		"webmux-linux-amd64": []byte(payload),
	}}
	mirrorURL := serve(t, mirror)
	c := Client{
		Downloader: fetch.Downloader{GitHubBase: serve(t, gh), Mirrors: []string{mirrorURL}},
		Keys:       []PublicKey{k.Public()},
	}
	m, host, err := c.Latest(context.Background())
	if err != nil || m.Version != "0.6.0" || !strings.Contains(mirrorURL, host) {
		t.Fatalf("latest: %v %q %q", err, m.Version, host)
	}
	if gh.paths[0] != "/"+Repo+"/releases/latest/download/SHA256SUMS" {
		t.Fatalf("GitHub path %q", gh.paths[0])
	}

	// Assets come from the signed version's tag, not from latest.
	target := filepath.Join(t.TempDir(), "webmux.deb")
	gh.files = map[string][]byte{}
	if _, err := c.Fetch(context.Background(), m, "webmux-linux-amd64", target, 0o644); err != nil {
		t.Fatal(err)
	}
	if got := mirror.paths[len(mirror.paths)-1]; got != "/https://github.com/"+Repo+"/releases/download/v0.6.0/webmux-linux-amd64" {
		t.Fatalf("asset path %q", got)
	}
	info, err := os.Stat(target)
	if err != nil || info.Mode().Perm() != 0o644 {
		t.Fatalf("installed mode: %v %v", err, info)
	}
}

// A mirror can lag behind with a stale but validly signed list, so when
// GitHub fails the newest verified mirror wins; GitHub itself is final.
func TestLatestPrefersNewestMirror(t *testing.T) {
	k := testKey(t)
	sums := sumLine("x", "webmux-linux-amd64")
	release := func(version string) *fakeRelease {
		return &fakeRelease{files: map[string][]byte{
			"SHA256SUMS":         []byte(sums),
			"SHA256SUMS.minisig": signed(t, k, sums, "webmux "+version),
		}}
	}
	down := &fakeRelease{}
	other := testKey(t)
	forged := release("0.7.0")
	forged.files["SHA256SUMS.minisig"] = signed(t, other, sums, "webmux 0.7.0")
	for _, tc := range []struct {
		name    string
		github  *fakeRelease
		mirrors []*fakeRelease
		version string
		host    int // index into mirrors, -1 for GitHub
	}{
		{"newer later mirror", down, []*fakeRelease{release("0.5.0"), release("0.6.0")}, "0.6.0", 1},
		{"stale later mirror", down, []*fakeRelease{release("0.6.0"), release("0.5.0")}, "0.6.0", 0},
		{"bad later signature", down, []*fakeRelease{release("0.5.0"), forged}, "0.5.0", 0},
		{"failing later mirror", down, []*fakeRelease{release("0.5.0"), down}, "0.5.0", 0},
		{"GitHub is final", release("0.5.0"), []*fakeRelease{release("0.6.0")}, "0.5.0", -1},
	} {
		t.Run(tc.name, func(t *testing.T) {
			gh := serve(t, tc.github)
			var mirrors []string
			for _, m := range tc.mirrors {
				m.paths = nil
				mirrors = append(mirrors, serve(t, m))
			}
			c := Client{Downloader: fetch.Downloader{GitHubBase: gh, Mirrors: mirrors}, Keys: []PublicKey{k.Public()}}
			m, host, err := c.Latest(context.Background())
			want := gh
			if tc.host >= 0 {
				want = mirrors[tc.host]
			}
			if err != nil || m.Version != tc.version || !strings.Contains(want, host) {
				t.Fatalf("latest: %v %q %q, want %q from %q", err, m.Version, host, tc.version, want)
			}
			if tc.host < 0 {
				for _, mirror := range tc.mirrors {
					if len(mirror.paths) != 0 {
						t.Fatalf("mirror contacted after GitHub answered: %v", mirror.paths)
					}
				}
			}
		})
	}
}

func TestOldManifestIsNotNew(t *testing.T) {
	k := testKey(t)
	sums := sumLine("x", "webmux-linux-amd64")
	// A source answering the latest path with the signed 0.4.0 list.
	src := &fakeRelease{files: map[string][]byte{
		"SHA256SUMS":         []byte(sums),
		"SHA256SUMS.minisig": signed(t, k, sums, "webmux 0.4.0"),
	}}
	c := Client{Downloader: fetch.Downloader{GitHubBase: serve(t, src), Mirrors: []string{}}, Keys: []PublicKey{k.Public()}}
	m, _, err := c.Latest(context.Background())
	if err != nil || m.Version != "0.4.0" {
		t.Fatalf("latest %v %q", err, m.Version)
	}
	if Newer(m.Version, "0.5.0") {
		t.Fatal("older signed list treated as an update")
	}
}

func TestFetchDiscardsMismatchedAsset(t *testing.T) {
	k := testKey(t)
	sums := sumLine("good", "pkg.deb")
	m, err := VerifyManifest([]byte(sums), signed(t, k, sums, "webmux 0.6.0"), []PublicKey{k.Public()})
	if err != nil {
		t.Fatal(err)
	}
	truncated := &fakeRelease{files: map[string][]byte{"pkg.deb": []byte("goo")}}
	c := Client{Downloader: fetch.Downloader{GitHubBase: serve(t, truncated), Mirrors: []string{}}}
	dir := t.TempDir()
	_, err = c.Fetch(context.Background(), m, "pkg.deb", filepath.Join(dir, "pkg.deb"), 0o644)
	var sources *fetch.SourcesError
	if !errors.As(err, &sources) {
		t.Fatalf("mismatch: %v", err)
	}
	if entries, _ := os.ReadDir(dir); len(entries) != 0 {
		t.Fatalf("left files: %v", entries)
	}
	if _, err := c.Fetch(context.Background(), m, "missing.deb", filepath.Join(dir, "x"), 0o644); err == nil {
		t.Fatal("unlisted asset fetched")
	}
}
