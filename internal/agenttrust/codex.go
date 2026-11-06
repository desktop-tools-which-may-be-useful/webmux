package agenttrust

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/BurntSushi/toml"
)

// trustCodex records trust_level = "trusted" under [projects."<dir>"] in
// Codex's config.toml, ~/.codex/config.toml or $CODEX_HOME/config.toml. Codex
// works without a config file, so it is created when the Codex directory
// exists, which shows the user has run Codex.
func (t *Trust) trustCodex(dir string, env map[string]string) error {
	path, err := t.configPath(env, "CODEX_HOME", ".codex", "config.toml")
	if err != nil {
		return err
	}
	if info, statErr := os.Stat(filepath.Dir(path)); statErr != nil || !info.IsDir() {
		return nil
	}
	return editFile(path, true, func(data []byte) ([]byte, bool, error) {
		return codexTrusted(data, dir)
	})
}

// codexTrusted appends a project table instead of re-encoding the file, which
// keeps the user's comments and ordering. An existing table for dir, trusted
// or explicitly untrusted, is the user's decision and is left as it is.
func codexTrusted(data []byte, dir string) (updated []byte, changed bool, err error) {
	var doc struct {
		Projects map[string]map[string]any `toml:"projects"`
	}
	if _, err = toml.Decode(string(data), &doc); err != nil {
		return nil, false, err
	}
	if _, decided := doc.Projects[dir]; decided {
		return nil, false, nil
	}
	key, err := tomlKey(dir)
	if err != nil {
		return nil, false, err
	}
	var out bytes.Buffer
	out.Write(data)
	if len(data) > 0 && !bytes.HasSuffix(data, []byte("\n")) {
		out.WriteByte('\n')
	}
	if len(data) > 0 {
		out.WriteByte('\n')
	}
	fmt.Fprintf(&out, "[projects.%s]\ntrust_level = \"trusted\"\n", key)
	return out.Bytes(), true, nil
}

// tomlKey quotes s as a TOML basic-string key.
func tomlKey(s string) (string, error) {
	var b strings.Builder
	b.WriteByte('"')
	for _, r := range s {
		switch {
		case r == '"' || r == '\\':
			b.WriteByte('\\')
			b.WriteRune(r)
		case r < 0x20 || r == 0x7f:
			return "", fmt.Errorf("directory %q has a control character", s)
		default:
			b.WriteRune(r)
		}
	}
	b.WriteByte('"')
	return b.String(), nil
}
