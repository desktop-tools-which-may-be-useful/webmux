package hybrid

import "testing"

func TestVersionField(t *testing.T) {
	tests := []struct{ out, want string }{
		{"codex-cli 0.165.0\n", "0.165.0"},
		{"2.1.316 (Claude Code)\n", "2.1.316"},
		{"opencode v2.1.4", "v2.1.4"},
		{"18.13.1-beta.2 (omp)\n", "18.13.1-beta.2"},
		{"no version here\n", ""},
		{"", ""},
	}
	for _, tt := range tests {
		if got := versionField(tt.out); got != tt.want {
			t.Errorf("versionField(%q) = %q, want %q", tt.out, got, tt.want)
		}
	}
}
