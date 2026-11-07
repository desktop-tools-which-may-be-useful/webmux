package agenttrust

import (
	"fmt"
	"strings"
)

const (
	qoderPermissionsKey      = "permissions"
	qoderTrustDirectoriesKey = "trustDirectories"
)

// matchesQoder reports whether base is one of executables or a versioned build
// of one (for example qodercli-1.1.81 or qoderclicn-1.1.84).
func matchesQoder(executables []string, base string) bool {
	for _, exe := range executables {
		if base == exe || strings.HasPrefix(base, exe+"-") {
			return true
		}
	}
	return false
}

// trustQoder appends dir to permissions.trustDirectories in Qoder's user
// settings.json (~/.qoder/settings.json or $QODER_CONFIG_DIR/settings.json).
func (t *Trust) trustQoder(dir string, env map[string]string) error {
	return t.trustQoderVariant(dir, env, "QODER_CONFIG_DIR", ".qoder")
}

// trustQoderCN appends dir to permissions.trustDirectories in QoderCN's user
// settings.json (~/.qoder-cn/settings.json or
// $QODERCN_CONFIG_DIR/settings.json).
func (t *Trust) trustQoderCN(dir string, env map[string]string) error {
	return t.trustQoderVariant(dir, env, "QODERCN_CONFIG_DIR", ".qoder-cn")
}

// trustQoderVariant appends dir to permissions.trustDirectories in
// settings.json under variable or ~/dotDir. As with Claude Code, Copilot and
// Antigravity CLI, a missing file is left to the first-run setup, and a file
// that is not plain JSON is reported and untouched.
func (t *Trust) trustQoderVariant(dir string, env map[string]string, variable, dotDir string) error {
	path, err := t.configPath(env, variable, dotDir, "settings.json")
	if err != nil {
		return err
	}
	return editFile(path, false, func(data []byte) ([]byte, bool, error) {
		return qoderTrusted(data, dir)
	})
}

func qoderTrusted(data []byte, dir string) (updated []byte, changed bool, err error) {
	root, err := objectOf(data)
	if err != nil {
		return nil, false, err
	}
	permissions, err := objectOf(root[qoderPermissionsKey])
	if err != nil {
		return nil, false, fmt.Errorf("%s: %w", qoderPermissionsKey, err)
	}
	changed, err = appendPath(permissions, qoderTrustDirectoriesKey, dir)
	if err != nil || !changed {
		return nil, false, err
	}
	if root[qoderPermissionsKey], err = encodeJSON(permissions); err != nil {
		return nil, false, err
	}
	updated, err = indentJSON(root)
	return updated, err == nil, err
}
