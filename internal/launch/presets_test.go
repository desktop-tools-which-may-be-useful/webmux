package launch

import (
	"slices"
	"testing"

	"github.com/rainchan/webmux/internal/agentprofile/agents"
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

// The browser shows a hybrid session's agent with the profile's Icon from the
// launch-config icon catalog, so every agent needs a title and a brand glyph
// in that catalog, the same one its hybrid preset carries.
func TestAgentIconsComeFromLaunchCatalog(t *testing.T) {
	registry := agents.Default()
	for _, a := range registry.Agents() {
		p := a.Profile()
		if p.Title == "" || p.Icon == "" || p.Icon == "terminal" {
			t.Errorf("agent %q: title %q, icon %q; want a title and a brand glyph", p.Name, p.Title, p.Icon)
			continue
		}
		if _, err := normalize(Input{Name: p.Title, Icon: p.Icon, Cmd: []string{p.Name}}); err != nil {
			t.Errorf("agent %q icon %q is not a launch-config icon: %v", p.Name, p.Icon, err)
		}
	}
	for _, preset := range Presets("/bin/bash") {
		if !preset.Hybrid {
			continue
		}
		support := registry.Check(preset.Cmd)
		a, ok := registry.Lookup(support.Agent)
		if !ok {
			t.Errorf("hybrid preset %q runs no registered agent: %+v", preset.Name, support)
			continue
		}
		if got := a.Profile().Icon; got != preset.Icon {
			t.Errorf("agent %q icon %q, its preset %q has %q", support.Agent, got, preset.Name, preset.Icon)
		}
	}
}
