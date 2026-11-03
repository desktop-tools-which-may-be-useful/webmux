package main

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/rainchan/webmux/internal/update"
)

// The signer holds the release key, and the release workflow's sign job
// runs it on a fresh runner with GOPROXY off: it must link no third-party
// module, only the standard library and this module's own packages.
func TestSignerLinksOnlyStandardLibrary(t *testing.T) {
	out, err := exec.Command("go", "list", "-deps", "-f", "{{if not .Standard}}{{.ImportPath}}{{end}}", ".").Output()
	if err != nil {
		t.Fatalf("go list: %v", err)
	}
	for _, path := range strings.Fields(string(out)) {
		if path != "github.com/rainchan/webmux" && !strings.HasPrefix(path, "github.com/rainchan/webmux/") {
			t.Errorf("release signer depends on third-party package %s", path)
		}
	}
}

func TestSignVerifyRoundTrip(t *testing.T) {
	dir := t.TempDir()
	secret := filepath.Join(dir, "key")
	var pub bytes.Buffer
	if err := run([]string{"keygen", "-secret", secret}, &pub); err != nil {
		t.Fatal(err)
	}
	if info, err := os.Stat(secret); err != nil || info.Mode().Perm() != 0o600 {
		t.Fatalf("secret file mode: %v %v", err, info)
	}
	key, err := os.ReadFile(secret)
	if err != nil {
		t.Fatal(err)
	}
	pubLine := strings.TrimSpace(pub.String()[strings.LastIndexByte(strings.TrimSpace(pub.String()), '\n')+1:])
	sums := filepath.Join(dir, "SHA256SUMS")
	if err := os.WriteFile(sums, []byte(strings.Repeat("a", 64)+"  webmux-linux-amd64\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	t.Setenv(keyEnv, "")
	if err := run([]string{"sign", "-version", "0.5.0", sums}, nil); err == nil {
		t.Fatal("signed without a key")
	}
	t.Setenv(keyEnv, string(key))
	if err := run([]string{"sign", "-version", "0.5.0", sums}, nil); err != nil {
		t.Fatal(err)
	}
	var out bytes.Buffer
	if err := run([]string{"verify", "-pubkey", pubLine, "-version", "0.5.0", sums}, &out); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), "webmux 0.5.0") {
		t.Fatalf("verify output %q", out.String())
	}
	if err := run([]string{"verify", "-pubkey", pubLine, "-version", "0.6.0", sums}, &out); err == nil {
		t.Fatal("wrong version accepted")
	}
	// The built-in keys never trust a freshly generated key.
	if err := run([]string{"verify", sums}, &out); err == nil {
		t.Fatal("built-in keys accepted a test key")
	}
	if _, err := update.ParsePublicKey(pubLine); err != nil {
		t.Fatal(err)
	}
}
