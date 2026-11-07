package voice

import (
	"bytes"
	"compress/gzip"
	"crypto/sha256"
	_ "embed"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
)

// bundledScript is the single-file Node.js program (all cloud services, its
// dependencies and the Opus codec) that speaks the STDIN/STDOUT protocol.
// Usage: node voice.js <provider>.
//
//go:embed asr/voice.js.gz
var bundledScript []byte

// ExtractScript writes the bundled recognition program under dir and returns
// its path. The file name carries a content hash, so a newer binary never runs
// an older copy and an existing copy is reused.
func ExtractScript(dir string) (string, error) {
	sum := sha256.Sum256(bundledScript)
	path := filepath.Join(dir, "voice", "voice-"+hex.EncodeToString(sum[:4])+".js")
	if info, err := os.Stat(path); err == nil && info.Mode().IsRegular() {
		return path, nil
	}
	reader, err := gzip.NewReader(bytes.NewReader(bundledScript))
	if err != nil {
		return "", fmt.Errorf("open bundled voice program: %w", err)
	}
	defer func() { _ = reader.Close() }()
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return "", fmt.Errorf("create voice directory: %w", err)
	}
	tmp, err := os.CreateTemp(filepath.Dir(path), "voice-*.tmp")
	if err != nil {
		return "", fmt.Errorf("write voice program: %w", err)
	}
	defer func() { _ = os.Remove(tmp.Name()) }()
	if _, err := io.Copy(tmp, reader); err != nil {
		_ = tmp.Close()
		return "", fmt.Errorf("write voice program: %w", err)
	}
	if err := tmp.Close(); err != nil {
		return "", fmt.Errorf("write voice program: %w", err)
	}
	if err := os.Chmod(tmp.Name(), 0o600); err != nil {
		return "", fmt.Errorf("write voice program: %w", err)
	}
	if err := os.Rename(tmp.Name(), path); err != nil {
		if errors.Is(err, os.ErrExist) {
			return path, nil
		}
		return "", fmt.Errorf("install voice program: %w", err)
	}
	return path, nil
}
