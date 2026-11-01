package codeview

import (
	"archive/zip"
	"bytes"
	"context"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
)

func mkdirs(t *testing.T, root string, dirs ...string) {
	t.Helper()
	for _, d := range dirs {
		if err := os.MkdirAll(filepath.Join(root, d), 0o755); err != nil {
			t.Fatal(err)
		}
	}
}

func read(t *testing.T, root, p string) string {
	t.Helper()
	b, err := os.ReadFile(filepath.Join(root, p))
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

func exists(root, p string) bool {
	_, err := os.Lstat(filepath.Join(root, p))
	return err == nil
}

func names(m Mutation) []string {
	out := make([]string, len(m.Nodes))
	for i, n := range m.Nodes {
		out[i] = n.Path
	}
	return out
}

// Rename and mkdir keep names to one entry of the same directory and never
// replace an existing entry: a taken name gets the " (n)" suffix.
func TestRenameAndMkdirNeverReplace(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	s, id, root := setup(t)
	mkdirs(t, root, "src/sub")
	write(t, root, "src/a.txt", "a")
	write(t, root, "src/b.txt", "b")
	c, err := s.Context(ctx, id, "")
	if err != nil || !c.Write {
		t.Fatalf("context %+v %v", c, err)
	}

	m, err := s.Rename(ctx, id, c.ID, nodeID("src/a.txt"), "b.txt")
	if err != nil || strings.Join(names(m), ",") != "src/b (2).txt" {
		t.Fatalf("rename onto taken name: %v %v", names(m), err)
	}
	if read(t, root, "src/b.txt") != "b" || read(t, root, "src/b (2).txt") != "a" || exists(root, "src/a.txt") {
		t.Fatal("rename replaced or kept the source")
	}
	if m, err = s.Rename(ctx, id, c.ID, nodeID("src/sub"), "sub"); err != nil || names(m)[0] != "src/sub" {
		t.Fatalf("same-name rename: %v %v", names(m), err)
	}
	if m, err = s.Rename(ctx, id, c.ID, nodeID("src/sub"), "b.txt"); err != nil || names(m)[0] != "src/b (3).txt" || m.Nodes[0].Kind != "directory" {
		t.Fatalf("directory rename: %+v %v", m, err)
	}
	for _, bad := range []string{"", ".", "..", ".GIT", "a/b", `a\b`, "x\x01", strings.Repeat("n", 256)} {
		if _, err := s.Rename(ctx, id, c.ID, nodeID("src/b.txt"), bad); errorKind(err) != ErrInvalid {
			t.Errorf("rename to %q: %v", bad, err)
		}
	}
	if _, err := s.Rename(ctx, id, c.ID, nodeID(""), "root"); errorKind(err) != ErrForbidden {
		t.Fatalf("root rename: %v", err)
	}

	for _, want := range []string{"src/new", "src/new (2)"} {
		if m, err = s.Mkdir(ctx, id, c.ID, nodeID("src"), "new"); err != nil || names(m)[0] != want {
			t.Fatalf("mkdir: %v %v, want %s", names(m), err, want)
		}
	}
	if _, err := s.Mkdir(ctx, id, c.ID, nodeID("src/b.txt"), "x"); errorKind(err) != ErrInvalid {
		t.Fatalf("mkdir in a file: %v", err)
	}
}

// Copy recreates files, directories and symlinks without following links,
// skips .git entries, refuses pasting into the source itself, and stays
// inside the byte budget. Move renames, keeping the source's identity.
func TestCopyAndMove(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	s, id, root := setup(t)
	mkdirs(t, root, "pkg/inner/.git", "dest")
	write(t, root, "pkg/main.go", "package main")
	write(t, root, "pkg/inner/.git/config", "secret")
	write(t, root, "pkg/inner/x.txt", "x")
	if err := os.Symlink("/etc/passwd", filepath.Join(root, "pkg/link")); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(filepath.Join(root, "pkg/main.go"), 0o750); err != nil {
		t.Fatal(err)
	}
	c, _ := s.Context(ctx, id, "")

	if _, err := s.Copy(ctx, id, c.ID, []string{nodeID("pkg/inner")}, nodeID("pkg/inner"), ModeCopy); errorKind(err) == nil {
		t.Fatal("copy into itself accepted")
	}
	// pkg/inner holds a .git entry, so it is a nested repository boundary.
	if _, err := s.Copy(ctx, id, c.ID, []string{nodeID("pkg")}, nodeID("pkg/inner"), ModeCopy); errorKind(err) != ErrInvalid {
		t.Fatalf("copy into own subtree: %v", err)
	}
	if err := os.RemoveAll(filepath.Join(root, "pkg/inner/.git")); err != nil {
		t.Fatal(err)
	}
	m, err := s.Copy(ctx, id, c.ID, []string{nodeID("pkg"), nodeID("pkg/main.go")}, nodeID(""), ModeCopy)
	if err != nil || strings.Join(names(m), ",") != "pkg (2),main.go" {
		t.Fatalf("copy: %v %v", names(m), err)
	}
	if read(t, root, "pkg (2)/main.go") != "package main" || read(t, root, "pkg (2)/inner/x.txt") != "x" {
		t.Fatal("copied contents differ")
	}
	if info, err := os.Stat(filepath.Join(root, "pkg (2)/main.go")); err != nil || info.Mode().Perm() != 0o750 {
		t.Fatalf("copied mode %v %v", info, err)
	}
	if target, err := os.Readlink(filepath.Join(root, "pkg (2)/link")); err != nil || target != "/etc/passwd" {
		t.Fatalf("symlink copy %q %v", target, err)
	}

	m, err = s.Copy(ctx, id, c.ID, []string{nodeID("main.go"), nodeID("pkg (2)")}, nodeID("dest"), ModeMove)
	if err != nil || strings.Join(names(m), ",") != "dest/main.go,dest/pkg (2)" {
		t.Fatalf("move: %v %v", names(m), err)
	}
	if exists(root, "main.go") || !exists(root, "dest/pkg (2)/inner/x.txt") {
		t.Fatal("move left the source or lost contents")
	}
	write(t, root, "main.go", "again")
	if m, err = s.Copy(ctx, id, c.ID, []string{nodeID("main.go")}, nodeID("dest"), ModeMove); err != nil || names(m)[0] != "dest/main (2).go" {
		t.Fatalf("move onto taken name: %v %v", names(m), err)
	}
	if read(t, root, "dest/main.go") != "package main" {
		t.Fatal("move replaced an existing file")
	}
	if m, err = s.Copy(ctx, id, c.ID, []string{nodeID("dest/main.go")}, nodeID("dest"), ModeMove); err != nil || names(m)[0] != "dest/main.go" {
		t.Fatalf("move into own directory: %v %v", names(m), err)
	}
	if _, err := s.Copy(ctx, id, c.ID, []string{nodeID("pkg/link")}, nodeID("dest"), ModeCopy); errorKind(err) != ErrInvalid {
		t.Fatalf("symlink node copy: %v", err)
	}

	big := filepath.Join(root, "big.bin")
	if err := os.WriteFile(big, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Truncate(big, MaxDownloadBytes+1); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Copy(ctx, id, c.ID, []string{nodeID("big.bin")}, nodeID("dest"), ModeCopy); errorKind(err) != ErrTooLarge {
		t.Fatalf("oversized copy: %v", err)
	}
	if exists(root, "dest/big.bin") {
		t.Fatal("oversized copy left a file")
	}
}

// Delete removes files and symlinks themselves, needs recursive for a
// non-empty directory, and refuses directories holding Git metadata.
func TestDelete(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	s, id, root := setup(t)
	outside := filepath.Join(t.TempDir(), "keep.txt")
	write(t, filepath.Dir(outside), "keep.txt", "keep")
	mkdirs(t, root, "empty", "tree/a/b", "repo/nested/.git")
	write(t, root, "tree/a/b/c.txt", "c")
	write(t, root, "file.txt", "f")
	if err := os.Symlink(outside, filepath.Join(root, "link")); err != nil {
		t.Fatal(err)
	}
	c, _ := s.Context(ctx, id, "")

	if u, err := s.Usage(ctx, id, c.ID, nodeID("tree")); err != nil || u.Entries != 4 || u.Bytes != 1 {
		t.Fatalf("usage %+v %v", u, err)
	}
	for _, p := range []string{"file.txt", "link", "empty"} {
		if err := s.Delete(ctx, id, c.ID, nodeID(p), false); err != nil || exists(root, p) {
			t.Fatalf("delete %s: %v", p, err)
		}
	}
	if read(t, filepath.Dir(outside), "keep.txt") != "keep" {
		t.Fatal("deleting a symlink touched its target")
	}
	if err := s.Delete(ctx, id, c.ID, nodeID("tree"), false); errorKind(err) != ErrConflict || !exists(root, "tree/a/b/c.txt") {
		t.Fatalf("non-recursive delete: %v", err)
	}
	if err := s.Delete(ctx, id, c.ID, nodeID("tree"), true); err != nil || exists(root, "tree") {
		t.Fatalf("recursive delete: %v", err)
	}
	if err := s.Delete(ctx, id, c.ID, nodeID("repo"), true); errorKind(err) != ErrConflict || !exists(root, "repo/nested/.git") {
		t.Fatalf("delete with nested repository: %v", err)
	}
	if err := s.Delete(ctx, id, c.ID, nodeID(""), true); errorKind(err) != ErrForbidden {
		t.Fatalf("root delete: %v", err)
	}
	if err := s.Delete(ctx, id, c.ID, nodeID("missing"), false); errorKind(err) != ErrNotFound {
		t.Fatalf("missing delete: %v", err)
	}
}

// Uploads appear under their final name only on commit; an abandoned or
// oversized upload leaves nothing behind.
func TestUploadCommitsOrLeavesNothing(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	s, id, root := setup(t)
	mkdirs(t, root, "in")
	write(t, root, "in/report.md", "old")
	c, _ := s.Context(ctx, id, "")

	u, err := s.Upload(ctx, id, c.ID, nodeID("in"), `C:\Users\me\report.md`)
	if err != nil {
		t.Fatal(err)
	}
	if err := u.Write(strings.NewReader("new"), 10); err != nil {
		t.Fatal(err)
	}
	m, err := u.Commit()
	u.Close()
	if err != nil || names(m)[0] != "in/report (2).md" || read(t, root, "in/report (2).md") != "new" || read(t, root, "in/report.md") != "old" {
		t.Fatalf("commit %v %v", names(m), err)
	}

	u, err = s.Upload(ctx, id, c.ID, nodeID("in"), "big.bin")
	if err != nil {
		t.Fatal(err)
	}
	if err := u.Write(bytes.NewReader(make([]byte, 11)), 10); errorKind(err) != ErrTooLarge {
		t.Fatalf("oversized upload: %v", err)
	}
	u.Close()
	entries, _ := os.ReadDir(filepath.Join(root, "in"))
	if len(entries) != 2 {
		t.Fatalf("leftover entries %v", entries)
	}
	if _, err := s.Upload(ctx, id, c.ID, nodeID("in"), ".git"); errorKind(err) != ErrInvalid {
		t.Fatalf("reserved upload name: %v", err)
	}
}

func zipNames(t *testing.T, a *Archive) []string {
	t.Helper()
	var buf bytes.Buffer
	if err := a.WriteTo(context.Background(), &buf); err != nil {
		t.Fatal(err)
	}
	r, err := zip.NewReader(bytes.NewReader(buf.Bytes()), int64(buf.Len()))
	if err != nil {
		t.Fatal(err)
	}
	var out []string
	for _, f := range r.File {
		out = append(out, f.Name)
		if f.Name == "src/a.go" {
			rc, _ := f.Open()
			b, _ := io.ReadAll(rc)
			_ = rc.Close()
			if string(b) != "package a" {
				t.Fatalf("zip content %q", b)
			}
		}
	}
	sort.Strings(out)
	return out
}

// A directory archive holds regular files and directories only, leaves out
// Git-ignored paths unless asked, and is refused above the byte budget.
func TestArchive(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	s, id, root := setup(t)
	gitTest(t, root, "init", "-q")
	mkdirs(t, root, "src/empty", "src/out")
	write(t, root, ".gitignore", "out/\n")
	write(t, root, "src/a.go", "package a")
	write(t, root, "src/out/gen.txt", "generated")
	if err := os.Symlink("/etc/passwd", filepath.Join(root, "src/link")); err != nil {
		t.Fatal(err)
	}
	c, _ := s.Context(ctx, id, "")

	a, err := s.Archive(ctx, id, c.ID, nodeID("src"), false)
	if err != nil {
		t.Fatal(err)
	}
	got := strings.Join(zipNames(t, a), ",")
	a.Close()
	if got != "src/a.go,src/empty/" {
		t.Fatalf("archive entries %s", got)
	}
	a, err = s.Archive(ctx, id, c.ID, nodeID("src"), true)
	if err != nil {
		t.Fatal(err)
	}
	got = strings.Join(zipNames(t, a), ",")
	a.Close()
	if got != "src/a.go,src/empty/,src/out/,src/out/gen.txt" {
		t.Fatalf("archive with ignored entries %s", got)
	}
	if err := os.Truncate(filepath.Join(root, "src/a.go"), MaxDownloadBytes+1); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Archive(ctx, id, c.ID, nodeID("src"), false); errorKind(err) != ErrTooLarge {
		t.Fatalf("oversized archive: %v", err)
	}
	if _, err := s.NodeRaw(ctx, id, c.ID, nodeID("src")); errorKind(err) != ErrInvalid {
		t.Fatalf("directory raw download: %v", err)
	}
}
