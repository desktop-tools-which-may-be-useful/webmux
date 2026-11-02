package agentprofile_test

import (
	"slices"
	"testing"

	. "github.com/rainchan/webmux/internal/agentprofile"
)

func TestDefaultShell(t *testing.T) {
	tests := map[string]string{
		"":                   ShellBash,
		"/bin/bash":          ShellBash,
		"/bin/zsh":           ShellZsh,
		"/usr/local/bin/zsh": ShellZsh,
		"/usr/bin/fish":      "",
		"/bin/sh":            "",
		"bash":               ShellBash,
		"zsh":                ShellZsh,
	}
	for path, want := range tests {
		if got := DefaultShell(path); got != want {
			t.Errorf("DefaultShell(%q) = %q, want %q", path, got, want)
		}
	}
}

func TestStartupArgv(t *testing.T) {
	tests := []struct {
		name string
		argv []string
		want []string
	}{
		{
			name: "bash",
			argv: StartupArgv(ShellBash, "omp", "--auto-approve"),
			want: []string{"bash", "-ic", `exec -- "$@"`, "webmux", "omp", "--auto-approve"},
		},
		{
			name: "zsh",
			argv: StartupArgv(ShellZsh, "omp", "--auto-approve"),
			want: []string{"zsh", "-ic", `exec -- "$@"`, "webmux", "omp", "--auto-approve"},
		},
		{
			name: "unsupported shell runs directly",
			argv: StartupArgv("", "omp", "--auto-approve"),
			want: []string{"omp", "--auto-approve"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if !slices.Equal(tt.argv, tt.want) {
				t.Fatalf("StartupArgv = %q, want %q", tt.argv, tt.want)
			}
		})
	}
}
