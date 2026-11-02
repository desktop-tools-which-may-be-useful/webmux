package listensettings

import (
	"os"
	"path/filepath"
	"testing"
)

func TestAddress(t *testing.T) {
	for _, row := range []struct {
		configured string
		public     bool
		want       string
	}{
		{"127.0.0.1:7681", false, "127.0.0.1:7681"},
		{"127.0.0.1:7681", true, "0.0.0.0:7681"},
		{"100.64.0.5:7681", false, "100.64.0.5:7681"},
		{"100.64.0.5:7681", true, "0.0.0.0:7681"},
		{"0.0.0.0:7681", false, "127.0.0.1:7681"},
		{"0.0.0.0:7681", true, "0.0.0.0:7681"},
		{"[::]:7681", true, "[::]:7681"},
		{":7681", false, "127.0.0.1:7681"},
		{"localhost:7681", true, "0.0.0.0:7681"},
	} {
		got, err := Address(row.configured, "7681", row.public)
		if err != nil || got != row.want {
			t.Errorf("Address(%q, %v) = %q %v, want %q", row.configured, row.public, got, err, row.want)
		}
	}
	if got, _ := Address("127.0.0.1:0", "41234", true); got != "0.0.0.0:41234" {
		t.Errorf("bound port ignored: %q", got)
	}
	if _, err := Address("no-port", "1", true); err == nil {
		t.Error("accepted listen without port")
	}
}

func TestStorePersistsAndFailsClosed(t *testing.T) {
	dir := t.TempDir()
	s := Open(dir, "127.0.0.1:7681")
	if got, web := s.Effective(); got.Public || web {
		t.Fatalf("default %+v web=%v", got, web)
	}
	if err := s.Save(Settings{Public: true}); err != nil {
		t.Fatal(err)
	}
	s.Close()
	if got, web, err := Inspect(dir, "127.0.0.1:7681"); err != nil || !got.Public || !web {
		t.Fatalf("inspect %+v %v %v", got, web, err)
	}
	again := Open(dir, "127.0.0.1:7681")
	if got, web := again.Effective(); !got.Public || !web {
		t.Fatalf("reopened %+v web=%v", got, web)
	}
	if _, err := again.Reset(); err != nil {
		t.Fatal(err)
	}
	if got, web := again.Effective(); got.Public || web {
		t.Fatalf("reset %+v web=%v", got, web)
	}
	again.Close()

	if got, _ := Open(t.TempDir(), "0.0.0.0:7681").Effective(); !got.Public {
		t.Fatal("wildcard TOML default is not public")
	}

	broken := t.TempDir()
	if err := os.WriteFile(filepath.Join(broken, "listen-settings.json"), []byte(`{"v":1}`), 0o600); err != nil {
		t.Fatal(err)
	}
	b := Open(broken, "0.0.0.0:7681")
	defer b.Close()
	if got, _ := b.Effective(); got.Public || b.Unavailable() == nil {
		t.Fatalf("broken store did not fail closed: %+v", got)
	}
}
