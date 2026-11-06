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
	changed, err = appendPath(root, key, dir)
	if err != nil || !changed {
		return nil, false, err
	}
	updated, err = indentJSON(root)
	return updated, err == nil, err
}

// appendPath adds dir to the JSON string array root[key], creating the array
// when it is missing. changed is false when dir is already listed.
func appendPath(root map[string]json.RawMessage, key, dir string) (changed bool, err error) {
	var paths []json.RawMessage
	if raw := root[key]; len(raw) > 0 && string(raw) != "null" {
		if err = json.Unmarshal(raw, &paths); err != nil {
			return false, fmt.Errorf("%s: %w", key, err)
		}
	}
	for _, path := range paths {
		var existing string
		if json.Unmarshal(path, &existing) == nil && existing == dir {
			return false, nil
		}
	}
	entry, err := encodeJSON(dir)
	if err != nil {
		return false, err
	}
	if root[key], err = encodeJSON(append(paths, entry)); err != nil {
		return false, err
	}
	return true, nil
}
