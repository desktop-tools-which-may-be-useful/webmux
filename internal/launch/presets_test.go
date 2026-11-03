package launch

import (
	"slices"
	"testing"
)

func TestPresetsFollowDefaultShell(t *testing.T) {
	tests := []struct {
		name      string
		shell     string
		wantLogin []string
		wantOMP   []string
	}{
		{
			name:      "bash",
			shell:     "/bin/bash",
			wantLogin: []string{"/bin/bash", "-l", "-i"},
			wantOMP:   []string{"bash", "-ic", `exec -- "$@"`, "webmux", "omp", "--auto-approve"},
		},
		{
			name:      "zsh",
			shell:     "/bin/zsh",
			wantLogin: []string{"/bin/zsh", "-l", "-i"},
			wantOMP:   []string{"zsh", "-ic", `exec -- "$@"`, "webmux", "omp", "--auto-approve"},
		},
		{
			name:      "unset falls back to bash",
			shell:     "",
			wantLogin: []string{"/bin/sh", "-l", "-i"},
			wantOMP:   []string{"bash", "-ic", `exec -- "$@"`, "webmux", "omp", "--auto-approve"},
		},
		{
			name:      "other shell runs directly",
			shell:     "/usr/bin/fish",
			wantLogin: []string{"/usr/bin/fish", "-l", "-i"},
			wantOMP:   []string{"omp", "--auto-approve"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			presets := Presets(tt.shell)
			if len(presets) != 13 || presets[0].Name != "Shell" || !slices.Equal(presets[0].Cmd, tt.wantLogin) {
				t.Fatalf("login shell preset = %+v", presets[:min(1, len(presets))])
			}
			for _, preset := range presets {
				if preset.Name != "omp" {
					continue
				}
				if !slices.Equal(preset.Cmd, tt.wantOMP) {
					t.Fatalf("omp argv = %q, want %q", preset.Cmd, tt.wantOMP)
				}
				return
			}
			t.Fatal("omp preset missing")
		})
	}
}
