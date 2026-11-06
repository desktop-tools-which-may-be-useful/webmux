package agenttrust

import "path/filepath"

// agyTrustKey is the array of trusted directories in Antigravity CLI's
// settings.json, which the CLI itself fills when the user answers "Yes, I
// trust this folder".
const agyTrustKey = "trustedWorkspaces"

// trustAgy appends dir to trustedWorkspaces in
// ~/.gemini/antigravity-cli/settings.json. The CLI has no variable that moves
// this directory, so only the home directory is consulted. As with Claude Code,
// a missing file is left to the first-run setup, and a file that is not plain
// JSON is reported and untouched.
func (t *Trust) trustAgy(dir string, _ map[string]string) error {
	home, err := t.home()
	if err != nil {
		return err
	}
	path := filepath.Join(home, ".gemini", "antigravity-cli", "settings.json")
	return editFile(path, false, func(data []byte) ([]byte, bool, error) {
		root, err := objectOf(data)
		if err != nil {
			return nil, false, err
		}
		changed, err := appendPath(root, agyTrustKey, dir)
		if err != nil || !changed {
			return nil, false, err
		}
		updated, err := indentJSON(root)
		return updated, err == nil, err
	})
}
