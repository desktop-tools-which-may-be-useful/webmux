package traysys

import "testing"

func TestCommandPathAndSamePath(t *testing.T) {
	for _, tc := range []struct {
		raw  string
		want string
	}{
		{`"C:\Program Files\WebMux\webmux-tray.exe"`, `C:\Program Files\WebMux\webmux-tray.exe`},
		{`"C:\Program Files\WebMux\webmux-tray.exe" --flag`, `C:\Program Files\WebMux\webmux-tray.exe`},
		{`C:\WebMux\webmux-tray.exe --flag`, `C:\WebMux\webmux-tray.exe`},
		{`C:\Program Files\WebMux\webmux-tray.exe`, `C:\Program Files\WebMux\webmux-tray.exe`},
	} {
		if got := CommandPath(tc.raw); got != tc.want {
			t.Errorf("CommandPath(%q) = %q, want %q", tc.raw, got, tc.want)
		}
	}
	if !SamePath(`C:\Users\u\WebMux\webmux-tray.exe`, `c:\users\u\webmux\webmux-tray.exe`) {
		t.Error("SamePath rejected case-insensitive match")
	}
	if SamePath("", `C:\x`) || SamePath(`C:\x`, "") {
		t.Error("SamePath accepted empty path")
	}
}
