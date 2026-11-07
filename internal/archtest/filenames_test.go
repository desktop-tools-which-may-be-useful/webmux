package archtest

import (
	"io/fs"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// goFileName is the shape every Go source file name takes: lowercase words
// joined by underscores (docs/agent-rules/naming.md).
var goFileName = regexp.MustCompile(`^[a-z0-9]+(_[a-z0-9]+)*\.go$`)

// vagueFileWords name a file after what it is not about. A file is named for
// the noun or responsibility it owns, so a reader can predict its location.
var vagueFileWords = map[string]bool{
	"util": true, "utils": true, "helper": true, "helpers": true,
	"misc": true, "more": true, "common": true, "stuff": true,
	"extra": true, "other2": true, "new": true, "old": true,
}

// TestGoFileNames keeps Go file names predictable: snake_case words and no
// catch-all words. Platform suffixes (_linux, _unix, _other) are ordinary words.
func TestGoFileNames(t *testing.T) {
	root := filepath.Join("..", "..")
	for _, dir := range []string{"internal", "cmd"} {
		err := filepath.WalkDir(filepath.Join(root, dir), func(path string, d fs.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if d.IsDir() {
				if d.Name() == "testdata" || d.Name() == "node_modules" {
					return fs.SkipDir
				}
				return nil
			}
			if !strings.HasSuffix(path, ".go") {
				return nil
			}
			name := d.Name()
			rel, _ := filepath.Rel(root, path)
			if !goFileName.MatchString(name) {
				t.Errorf("%s: file names are lowercase words joined by underscores (docs/agent-rules/naming.md)", rel)
				return nil
			}
			stem := strings.TrimSuffix(strings.TrimSuffix(name, ".go"), "_test")
			for _, word := range strings.Split(stem, "_") {
				if vagueFileWords[word] {
					t.Errorf("%s: %q says nothing about the contents; name the file after the noun or responsibility it owns (docs/agent-rules/naming.md)", rel, word)
					break
				}
			}
			return nil
		})
		if err != nil {
			t.Fatal(err)
		}
	}
}
