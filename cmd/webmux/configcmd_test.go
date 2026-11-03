package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/rainchan/webmux/deploy"
	"github.com/rainchan/webmux/internal/config"
	"github.com/rainchan/webmux/internal/contract"
)

func runConfig(t *testing.T, args ...string) (string, string, int) {
	t.Helper()
	var stdout, stderr bytes.Buffer
	code := dispatch(append([]string{"config"}, args...), &stdout, &stderr)
	return stdout.String(), stderr.String(), code
}

func TestConfigShowAndPath(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("XDG_STATE_HOME", "")
	defaultPath := filepath.Join(home, ".config", "webmux", "config.toml")

	stdout, _, code := runConfig(t, "path", "--json")
	var where contract.ConfigPathV1
	if err := json.Unmarshal([]byte(stdout), &where); err != nil || code != 0 || where.Path != defaultPath || where.Exists {
		t.Fatalf("path %d %s", code, stdout)
	}
	stdout, _, code = runConfig(t, "show", "--json")
	var shown contract.ConfigV1
	if err := json.Unmarshal([]byte(stdout), &shown); err != nil || code != 0 || shown.Exists || shown.Listen != "127.0.0.1:7681" || shown.TokenConfigured || shown.StateDir != filepath.Join(home, ".local", "state", "webmux") {
		t.Fatalf("defaults %d %s", code, stdout)
	}
	if _, _, code := runConfig(t, "show", "--config", filepath.Join(home, "missing.toml")); code != 1 {
		t.Fatalf("explicit missing file exit %d", code)
	}

	path := filepath.Join(home, "custom.toml")
	content := "token = 'very-secret-token'\nlisten = '0.0.0.0:9000'\n"
	if err := os.WriteFile(path, []byte(content), 0600); err != nil {
		t.Fatal(err)
	}
	for _, args := range [][]string{{"show", "--config", "~/custom.toml", "--json"}, {"show", "--config", path}} {
		stdout, stderr, code := runConfig(t, args...)
		if code != 0 || strings.Contains(stdout+stderr, "very-secret-token") {
			t.Fatalf("%v: %d %s %s", args, code, stdout, stderr)
		}
		if args[len(args)-1] == "--json" {
			var custom contract.ConfigV1
			if err := json.Unmarshal([]byte(stdout), &custom); err != nil || !custom.TokenConfigured || custom.Path != path {
				t.Fatalf("custom %s", stdout)
			}
		}
	}
}

func TestConfigValidate(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("HOME", dir)
	for name, row := range map[string]struct {
		content, want string
	}{
		"syntax":       {"listen = '127.0.0.1:7681'\nscrollback_bytes = 10\ntoken = \"unterminated\n", ":3:"},
		"short tunnel": {"token = 'short'\n[tunnel]\nenabled = true\n", "at least 43"},
	} {
		path := filepath.Join(dir, name+".toml")
		if err := os.WriteFile(path, []byte(row.content), 0600); err != nil {
			t.Fatal(err)
		}
		before, _ := os.ReadFile(path)
		_, stderr, code := runConfig(t, "validate", "--config", path)
		if code != 1 || !strings.Contains(stderr, row.want) {
			t.Errorf("%s: %d %q", name, code, stderr)
		}
		stdout, _, _ := runConfig(t, "validate", "--config", path, "--json")
		var result contract.ValidateV1
		if err := json.Unmarshal([]byte(stdout), &result); err != nil || result.OK || len(result.Errors) != 1 {
			t.Errorf("%s JSON: %s", name, stdout)
		}
		if after, _ := os.ReadFile(path); !bytes.Equal(before, after) {
			t.Errorf("%s: validate modified the file", name)
		}
	}
}

var tokenValue = regexp.MustCompile(`(?m)^token = "([A-Za-z0-9_-]+)"$`)

func TestConfigInit(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("HOME", dir)
	path := filepath.Join(dir, "nested", "webmux", "config.toml")

	stdout, stderr, code := runConfig(t, "init", "--config", path, "--listen", "100.64.0.5:7681")
	if code != 0 || stdout != path+"\n" || stderr != "" {
		t.Fatalf("init %d %q %q", code, stdout, stderr)
	}
	for file, mode := range map[string]os.FileMode{path: 0600, filepath.Dir(path): 0700, filepath.Dir(filepath.Dir(path)): 0700} {
		if info, err := os.Stat(file); err != nil || info.Mode().Perm() != mode {
			t.Fatalf("%s: %v %v", file, info, err)
		}
	}
	first, _ := os.ReadFile(path)
	match := tokenValue.FindSubmatch(first)
	if match == nil || len(match[1]) != 43 || strings.Contains(stdout, string(match[1])) {
		t.Fatalf("generated token in %s", first)
	}
	expected := tokenLine.ReplaceAllLiteralString(deploy.ConfigExample, `token = "`+string(match[1])+`"`)
	expected = listenLine.ReplaceAllLiteralString(expected, `listen = "100.64.0.5:7681"`)
	if string(first) != expected {
		t.Fatalf("template changed beyond token and listen:\n%s", first)
	}
	if _, _, code := runConfig(t, "validate", "--config", path); code != 0 {
		t.Fatal("generated configuration does not validate")
	}

	_, stderr, code = runConfig(t, "init", "--config", path)
	if after, _ := os.ReadFile(path); code != 1 || !strings.Contains(stderr, "--force") || !bytes.Equal(after, first) {
		t.Fatalf("existing file: %d %q", code, stderr)
	}
	stdout, _, code = runConfig(t, "init", "--config", path, "--force")
	second, _ := os.ReadFile(path)
	backup, _ := os.ReadFile(path + ".bak")
	next := tokenValue.FindSubmatch(second)
	if code != 0 || stdout != path+"\n" || !bytes.Equal(backup, first) || next == nil || bytes.Equal(next[1], match[1]) {
		t.Fatalf("forced init %d backup=%v", code, bytes.Equal(backup, first))
	}
	if strings.Contains(string(second), "100.64.0.5") {
		t.Fatal("listen written without --listen")
	}
}

func TestConfigListen(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("HOME", dir)
	path := filepath.Join(dir, "webmux", "config.toml")

	// Creating from scratch when missing populates a token and the requested listen.
	stdout, stderr, code := runConfig(t, "listen", "--listen", "0.0.0.0:7681", "--config", path)
	if code != 0 || stdout != path+"\n" || stderr != "" {
		t.Fatalf("listen create %d %q %q", code, stdout, stderr)
	}
	first, _ := os.ReadFile(path)
	match := tokenValue.FindSubmatch(first)
	if match == nil || len(match[1]) != 43 || !bytes.Contains(first, []byte(`listen = "0.0.0.0:7681"`)) {
		t.Fatalf("created config:\n%s", first)
	}

	// Updating an existing file preserves the token and comments.
	stdout, stderr, code = runConfig(t, "listen", "127.0.0.1:7681", "--config", path)
	if code != 0 || stdout != path+"\n" || stderr != "" {
		t.Fatalf("listen update %d %q %q", code, stdout, stderr)
	}
	second, _ := os.ReadFile(path)
	next := tokenValue.FindSubmatch(second)
	if next == nil || !bytes.Equal(next[1], match[1]) || !bytes.Contains(second, []byte(`listen = "127.0.0.1:7681"`)) {
		t.Fatalf("updated config:\n%s", second)
	}
	if _, err := os.Stat(path + ".bak"); !os.IsNotExist(err) {
		t.Fatal("config listen created an unexpected .bak file")
	}

	// Switching an empty-token config to non-loopback populates a persistent token.
	emptyTok := filepath.Join(dir, "empty-token.toml")
	if err := os.WriteFile(emptyTok, []byte("listen = \"127.0.0.1:7681\"\ntoken = \"\"\n"), 0600); err != nil {
		t.Fatal(err)
	}
	if _, stderr, code := runConfig(t, "listen", "0.0.0.0:7681", "--config", emptyTok); code != 0 {
		t.Fatalf("empty-token listen update %d %q", code, stderr)
	}
	updatedEmpty, _ := os.ReadFile(emptyTok)
	if m := tokenValue.FindSubmatch(updatedEmpty); m == nil || len(m[1]) != 43 {
		t.Fatalf("expected token to be populated:\n%s", updatedEmpty)
	}

	// Missing root token with `[tunnel] token = ""` must not overwrite tunnel.token.
	tunnelCfg := filepath.Join(dir, "tunnel-empty.toml")
	if err := os.WriteFile(tunnelCfg, []byte("listen = \"127.0.0.1:7681\"\n\n[tunnel]\nenabled = true\ntoken = \"\"\n"), 0600); err != nil {
		t.Fatal(err)
	}
	if _, stderr, code := runConfig(t, "listen", "0.0.0.0:7681", "--config", tunnelCfg); code != 0 {
		t.Fatalf("tunnel-empty listen update %d %q", code, stderr)
	}
	loaded, err := config.Load(tunnelCfg, true)
	if err != nil {
		t.Fatalf("tunnel-empty load: %v", err)
	}
	if err := loaded.Validate(); err != nil {
		t.Fatalf("tunnel-empty config invalid after listen update: %v", err)
	}
	if len(loaded.Token) != 43 || loaded.Tunnel.Token != "" {
		t.Fatalf("tunnel-empty tokens: root=%q tunnel=%q", loaded.Token, loaded.Tunnel.Token)
	}

	// Invalid listen address is rejected with ExitUsage without touching the file.
	if _, _, code := runConfig(t, "listen", "not-a-hostport", "--config", path); code != contract.ExitUsage {
		t.Fatalf("invalid listen exit %d", code)
	}
}
