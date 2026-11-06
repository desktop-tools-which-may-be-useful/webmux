package agenttrust

import (
	"encoding/json"
	"fmt"
)

// trustCopilot appends dir to trustedFolders in GitHub Copilot CLI's
// config.json, ~/.copilot/config.json or $COPILOT_HOME/config.json. As with
// Claude Code, a missing file is left to the first-run setup. A file that is
// not plain JSON (for example one with comments) is reported and untouched.
func (t *Trust) trustCopilot(dir string, env map[string]string) error {
	path, err := t.configPath(env, "COPILOT_HOME", ".copilot", "config.json")
	if err != nil {
		return err
	}
	return editFile(path, false, func(data []byte) ([]byte, bool, error) {
		return copilotTrusted(data, dir)
	})
}

// copilotFoldersKey is the array Copilot CLI reads, per GitHub's "Configuring
// GitHub Copilot CLI". Older snake_case spelling is only kept when a file
// already uses it and has no current key.
const (
	copilotFoldersKey       = "trustedFolders"
	copilotLegacyFoldersKey = "trusted_folders"
)

func copilotTrusted(data []byte, dir string) (updated []byte, changed bool, err error) {
	root, err := objectOf(data)
	if err != nil {
		return nil, false, err
	}
	key := copilotFoldersKey
	if _, current := root[key]; !current {
		if _, legacy := root[copilotLegacyFoldersKey]; legacy {
			key = copilotLegacyFoldersKey
		}
	}
	var folders []json.RawMessage
	if raw := root[key]; len(raw) > 0 && string(raw) != "null" {
		if err = json.Unmarshal(raw, &folders); err != nil {
			return nil, false, fmt.Errorf("%s: %w", key, err)
		}
	}
	for _, folder := range folders {
		var existing string
		if json.Unmarshal(folder, &existing) == nil && existing == dir {
			return nil, false, nil
		}
	}
	entry, err := encodeJSON(dir)
	if err != nil {
		return nil, false, err
	}
	if root[key], err = encodeJSON(append(folders, entry)); err != nil {
		return nil, false, err
	}
	updated, err = indentJSON(root)
	return updated, err == nil, err
}
