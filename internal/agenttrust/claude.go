package agenttrust

import (
	"bytes"
	"encoding/json"
	"fmt"
)

// trustClaude sets projects[dir].hasTrustDialogAccepted in Claude Code's
// global config, ~/.claude.json or $CLAUDE_CONFIG_DIR/.claude.json. A missing
// file is left alone: Claude Code then runs its first-time setup, which
// includes the dialog, and creating the file would skip that setup.
func (t *Trust) trustClaude(dir string, env map[string]string) error {
	path, err := t.configPath(env, "CLAUDE_CONFIG_DIR", "", ".claude.json")
	if err != nil {
		return err
	}
	return editFile(path, false, func(data []byte) ([]byte, bool, error) {
		return claudeTrusted(data, dir)
	})
}

// claudeTrusted returns data with the trust record added. changed is false
// when the record is already there.
func claudeTrusted(data []byte, dir string) (updated []byte, changed bool, err error) {
	root, err := objectOf(data)
	if err != nil {
		return nil, false, err
	}
	projects, err := objectOf(root["projects"])
	if err != nil {
		return nil, false, fmt.Errorf("projects: %w", err)
	}
	entry, err := objectOf(projects[dir])
	if err != nil {
		return nil, false, fmt.Errorf("projects[%q]: %w", dir, err)
	}
	if bytes.Equal(entry["hasTrustDialogAccepted"], []byte("true")) {
		return nil, false, nil
	}
	entry["hasTrustDialogAccepted"] = json.RawMessage("true")
	if projects[dir], err = encodeJSON(entry); err != nil {
		return nil, false, err
	}
	if root["projects"], err = encodeJSON(projects); err != nil {
		return nil, false, err
	}
	updated, err = indentJSON(root)
	return updated, err == nil, err
}
