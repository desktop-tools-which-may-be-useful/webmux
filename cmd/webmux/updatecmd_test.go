package main

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/rainchan/webmux/internal/fetch"
	"github.com/rainchan/webmux/internal/update"
)

// signedRelease serves a signed SHA256SUMS for version and counts requests.
func signedRelease(t *testing.T, latest string) (string, update.PublicKey, *atomic.Int32) {
	t.Helper()
	key, err := update.GenerateKey()
	if err != nil {
		t.Fatal(err)
	}
	h := sha256.Sum256([]byte("pkg"))
	sums := hex.EncodeToString(h[:]) + "  webmux_" + latest + "-1_amd64.deb\n"
	sig, err := key.Sign([]byte(sums), "webmux "+latest)
	if err != nil {
		t.Fatal(err)
	}
	var hits atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits.Add(1)
		switch filepath.Base(r.URL.Path) {
		case "SHA256SUMS":
			_, _ = w.Write([]byte(sums))
		case "SHA256SUMS.minisig":
			_, _ = w.Write(sig)
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(srv.Close)
	return srv.URL, key.Public(), &hits
}

func testUpdateEnv(base string, key update.PublicKey, owner string) updateEnv {
	return updateEnv{
		isRelease: func() bool { return true },
		exe:       func() (string, error) { return "/usr/bin/webmux", nil },
		run: func(_ context.Context, name string, args ...string) ([]byte, error) {
			if name == "dpkg-query" && owner == "deb" {
				return []byte("webmux: /usr/bin/webmux\n"), nil
			}
			return nil, errors.New("not owned")
		},
		client: update.Client{
			Downloader: fetch.Downloader{GitHubBase: base, Mirrors: []string{}},
			Keys:       []update.PublicKey{key},
		},
	}
}

func setVersion(t *testing.T, v string) {
	t.Helper()
	old := version
	version = v
	t.Cleanup(func() { version = old })
}

func TestUpdateCheckJSON(t *testing.T) {
	setVersion(t, "0.5.0")
	base, key, _ := signedRelease(t, "0.5.0")
	var out, errOut bytes.Buffer
	code := runUpdate([]string{"check", "--json"}, &out, &errOut, testUpdateEnv(base, key, "deb"))
	var compact bytes.Buffer
	if err := json.Compact(&compact, out.Bytes()); err != nil {
		t.Fatalf("stdout is not one JSON object: %q", out.String())
	}
	want := `{"v":1,"current":"0.5.0","latest":"0.5.0","available":false,"installMethod":"deb"}`
	if code != 0 || compact.String() != want {
		t.Fatalf("code %d stdout %q stderr %q", code, out.String(), errOut.String())
	}
}

func TestUpdateCheckManualInstallHint(t *testing.T) {
	setVersion(t, "0.4.0")
	base, key, _ := signedRelease(t, "0.5.0")
	var out, errOut bytes.Buffer
	code := runUpdate([]string{"check"}, &out, &errOut, testUpdateEnv(base, key, ""))
	text := out.String()
	if code != 0 || !strings.Contains(text, "0.4.0") || !strings.Contains(text, "0.5.0") ||
		!strings.Contains(text, "有可用更新") || !strings.Contains(text, "install.sh") {
		t.Fatalf("code %d stdout %q stderr %q", code, text, errOut.String())
	}
}

func TestUpdateRefusesNonReleaseBuild(t *testing.T) {
	base, key, hits := signedRelease(t, "0.5.0")
	env := testUpdateEnv(base, key, "deb")
	env.isRelease = func() bool { return false }
	for _, action := range []string{"check", "apply"} {
		var out, errOut bytes.Buffer
		if code := runUpdate([]string{action}, &out, &errOut, env); code == 0 || !strings.Contains(errOut.String(), "不是发布构建") {
			t.Fatalf("%s: code %d stderr %q", action, code, errOut.String())
		}
	}
	if hits.Load() != 0 {
		t.Fatalf("non-release build made %d requests", hits.Load())
	}
}

func TestUpdateCheckFailureExitsNonZero(t *testing.T) {
	broken := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusBadGateway)
	}))
	t.Cleanup(broken.Close)
	env := testUpdateEnv(broken.URL, update.PublicKey{}, "deb")
	var out, errOut bytes.Buffer
	if code := runUpdate([]string{"check"}, &out, &errOut, env); code == 0 || out.Len() != 0 {
		t.Fatalf("code %d stdout %q", code, out.String())
	}
}
