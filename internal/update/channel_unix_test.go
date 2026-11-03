//go:build unix

package update

import (
	"os"
	"path/filepath"
	"syscall"
	"testing"
)

func TestReadChannel(t *testing.T) {
	uid := uint32(os.Getuid())
	dir := t.TempDir()
	write := func(name, content string, mode os.FileMode) string {
		path := filepath.Join(dir, name)
		if err := os.WriteFile(path, []byte(content), mode); err != nil {
			t.Fatal(err)
		}
		if err := os.Chmod(path, mode); err != nil {
			t.Fatal(err)
		}
		return path
	}
	cases := []struct {
		name string
		path string
		want Channel
		warn bool
	}{
		{"missing", filepath.Join(dir, "absent"), ChannelStable, false},
		{"stable", write("stable", "stable\n", 0o644), ChannelStable, false},
		{"dev", write("dev", "dev\n", 0o644), ChannelDev, false},
		{"unrecognised", write("beta", "beta\n", 0o644), ChannelStable, true},
		{"writable by others", write("open", "dev\n", 0o666), ChannelStable, true},
		{"writable by group", write("group", "dev\n", 0o664), ChannelStable, true},
	}
	for _, c := range cases {
		ch, warn := readChannel(c.path, uid)
		if ch != c.want || (warn != nil) != c.warn {
			t.Errorf("%s: %q %v", c.name, ch, warn)
		}
	}
	// A file owned by another user does not count.
	if ch, warn := readChannel(filepath.Join(dir, "dev"), uid+1); ch != ChannelStable || warn == nil {
		t.Errorf("foreign owner: %q %v", ch, warn)
	}
}

func TestWriteChannel(t *testing.T) {
	path := filepath.Join(t.TempDir(), "etc", "webmux", "update-channel")
	// Hardened hosts run sudo with umask 027; non-root readers still need
	// to reach the file.
	old := syscall.Umask(0o027)
	err := WriteChannel(path, ChannelDev)
	syscall.Umask(old)
	if err != nil {
		t.Fatal(err)
	}
	if info, err := os.Stat(filepath.Dir(path)); err != nil || info.Mode().Perm() != 0o755 {
		t.Fatalf("directory mode: %v %v", err, info)
	}
	info, err := os.Stat(path)
	if err != nil || info.Mode().Perm() != 0o644 {
		t.Fatalf("mode: %v %v", err, info)
	}
	if ch, warn := readChannel(path, uint32(os.Getuid())); ch != ChannelDev || warn != nil {
		t.Fatalf("read back: %q %v", ch, warn)
	}
	if err := WriteChannel(path, ChannelStable); err != nil {
		t.Fatal(err)
	}
	if ch, _ := readChannel(path, uint32(os.Getuid())); ch != ChannelStable {
		t.Fatalf("overwrite: %q", ch)
	}
	entries, _ := os.ReadDir(filepath.Dir(path))
	if len(entries) != 1 {
		t.Fatalf("temporary files left: %v", entries)
	}
}
