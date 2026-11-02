package web

import (
	"bytes"
	"compress/gzip"
	"io"
	"io/fs"
	"os"
	"path"
	"testing"
)

// Compare the Go ZIP reader against Vite's ordinary output. Compressible files
// of at least 1 KiB are embedded only as stored .br and .gz siblings (see
// web/scripts/embed-assets.ts); everything else, including fonts, images and
// small files, is embedded under its own name. This covers both forms,
// including lazy language chunks.
//
// The .gz sibling is inflated and compared byte for byte. Go's standard library
// cannot decode Brotli and this module takes no Brotli dependency, so the .br
// sibling is only required to exist and be non-empty.
func TestEmbeddedAssetsMatchBuild(t *testing.T) {
	count := 0
	err := fs.WalkDir(os.DirFS("dist"), ".", func(name string, entry fs.DirEntry, err error) error {
		if err != nil || entry.IsDir() || name == "embed.zip" || name == ".gitkeep" {
			return err
		}
		want, err := os.ReadFile(path.Join("dist", name))
		if err != nil {
			return err
		}
		count++

		raw, rawErr := fs.ReadFile(Assets, "dist/"+name)
		gz, gzErr := fs.ReadFile(Assets, "dist/"+name+".gz")
		br, brErr := fs.ReadFile(Assets, "dist/"+name+".br")
		switch {
		case gzErr == nil || brErr == nil:
			if rawErr == nil {
				t.Errorf("asset %s is embedded both as itself and as precompressed siblings", name)
			}
			if gzErr != nil || brErr != nil {
				t.Errorf("asset %s needs both .br and .gz siblings: br: %v, gz: %v", name, brErr, gzErr)
				return nil
			}
			if len(br) == 0 {
				t.Errorf("embedded asset %s.br is empty", name)
			}
			zr, err := gzip.NewReader(bytes.NewReader(gz))
			if err != nil {
				t.Errorf("embedded asset %s.gz is not valid gzip: %v", name, err)
				return nil
			}
			got, err := io.ReadAll(zr)
			if err != nil {
				t.Errorf("inflate embedded asset %s.gz: %v", name, err)
				return nil
			}
			if !bytes.Equal(got, want) {
				t.Errorf("embedded asset %s.gz differs from build", name)
			}
		case rawErr == nil:
			if !bytes.Equal(raw, want) {
				t.Errorf("embedded asset %s differs from build", name)
			}
		default:
			t.Errorf("asset %s is missing from the embedded archive", name)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if count == 0 {
		t.Fatal("no built assets checked")
	}
	if _, err := Assets.Open("dist/embed.zip"); !os.IsNotExist(err) {
		t.Fatalf("archive contains itself: %v", err)
	}
	for _, sibling := range []string{".br", ".gz"} {
		if _, err := fs.Stat(Assets, "dist/embed.zip"+sibling); err == nil {
			t.Fatalf("archive contains its own %s sibling", sibling)
		}
	}
}
