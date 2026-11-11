package agentprofile

import (
	"slices"
	"strings"
	"testing"
)

func TestDeclaredCompleteCommands(t *testing.T) {
	agent := Spec{
		Title:              "Acme Agent",
		ProcessExecutables: []string{"acme-native"},
		Commands: []CommandSpec{
			{
				Match:  []string{"acme-acp"},
				ACP:    []string{"acme-acp", "serve", "--stdio"},
				Resume: []string{"acme-acp", "--resume"},
			},
			{
				Match:    []string{"isolate-run", "acme-acp", "work"},
				ACP:      []string{"isolate-run", "acme-acp", "work", "--acp"},
				Resume:   []string{"isolate-run", "acme-acp", "work", "--resume"},
				Launcher: "sandbox-root",
			},
		},
	}.Agent("acme")
	registry := NewRegistry(agent)

	tests := []struct {
		name       string
		input      []string
		wantTUI    []string
		wantACP    []string
		wantResume []string
		launcher   string
	}{
		{
			name:       "direct path",
			input:      []string{"/opt/bin/acme-acp"},
			wantTUI:    []string{"/opt/bin/acme-acp"},
			wantACP:    []string{"acme-acp", "serve", "--stdio"},
			wantResume: []string{"acme-acp", "--resume", "session-1"},
		},
		{
			name:       "opaque custom command",
			input:      []string{"isolate-run", "acme-acp", "work"},
			wantTUI:    []string{"isolate-run", "acme-acp", "work"},
			wantACP:    []string{"isolate-run", "acme-acp", "work", "--acp"},
			wantResume: []string{"isolate-run", "acme-acp", "work", "--resume", "session-1"},
			launcher:   "sandbox-root",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cmd, err := registry.Parse(tt.input)
			if err != nil {
				t.Fatal(err)
			}
			if got := cmd.TUIArgv(); !slices.Equal(got, tt.wantTUI) {
				t.Fatalf("TUIArgv() = %q, want %q", got, tt.wantTUI)
			}
			if got, unsupported := cmd.ACPArgv(); !slices.Equal(got, tt.wantACP) || unsupported != nil {
				t.Fatalf("ACPArgv() = %q, %q, want %q, nil", got, unsupported, tt.wantACP)
			}
			if got, dropped := cmd.ResumeArgv("session-1"); !slices.Equal(got, tt.wantResume) || dropped != nil {
				t.Fatalf("ResumeArgv() = %q, %q, want %q, nil", got, dropped, tt.wantResume)
			}
			if got := cmd.Launcher(); got != tt.launcher {
				t.Fatalf("Launcher() = %q, want %q", got, tt.launcher)
			}
			if !cmd.RunsProcess("acme-native") || cmd.RunsProcess("acme-acp") {
				t.Fatal("process identity was inferred from the launch command")
			}
		})
	}
	if _, ok := registry.ForExecutable("acme-native"); ok {
		t.Fatal("native process name became an implicit launch command")
	}
}

func TestDeclaredCommandStartupForm(t *testing.T) {
	connector := &Connector{
		Executable:      "acme-bridge",
		Title:           "Acme ACP bridge",
		InstallCommand:  "install acme-bridge",
		DocsURL:         "https://example.invalid/acme-bridge",
		VerifiedVersion: "2.0.0",
	}
	agent := Spec{
		ProcessExecutables: []string{"acme-native"},
		Commands: []CommandSpec{{
			Match:     []string{"isolate-run", "acme-acp", "work"},
			ACP:       []string{"/opt/acme/other-acp", "server", "", "one argument"},
			Resume:    []string{"isolate-run", "acme-acp", "work", "resume"},
			Connector: connector,
		}},
	}.Agent("acme")
	registry := NewRegistry(agent)
	for _, input := range [][]string{
		{"/usr/bin/env", "bash", "-ic", startupScript, startupName, "/opt/bin/isolate-run", "acme-acp", "work"},
		{"bash", "-ic", startupScript, startupName, "/opt/bin/isolate-run", "acme-acp", "work"},
		{"zsh", "-ic", startupScript, startupName, "/opt/bin/isolate-run", "acme-acp", "work"},
	} {
		t.Run(strings.Join(input[:min(2, len(input))], " "), func(t *testing.T) {
			cmd, err := registry.Parse(input)
			if err != nil {
				t.Fatal(err)
			}
			if got := cmd.TUIArgv(); !slices.Equal(got, input) {
				t.Fatalf("TUIArgv() = %q, want original %q", got, input)
			}
			wantACP := slices.Clone(input[:len(input)-3])
			wantACP[len(wantACP)-2] = startupACPScript
			wantACP = append(wantACP, "/opt/acme/other-acp", "server", "", "one argument")
			if got, _ := cmd.ACPArgv(); !slices.Equal(got, wantACP) {
				t.Fatalf("ACPArgv() = %q, want %q", got, wantACP)
			}
			wantResume := slices.Clone(input[:len(input)-3])
			wantResume = append(wantResume, "isolate-run", "acme-acp", "work", "resume", "session-1")
			if got, _ := cmd.ResumeArgv("session-1"); !slices.Equal(got, wantResume) {
				t.Fatalf("ResumeArgv() = %q, want %q", got, wantResume)
			}
			if !cmd.ProtocolOnFD3() {
				t.Fatal("startup-file ACP did not retain fd 3 transport")
			}
			if got := cmd.Connector(); got == nil || *got != *connector {
				t.Fatalf("Connector() = %+v, want %+v", got, connector)
			}
			lookup := cmd.Lookup(connector.Executable)
			wantLookup := append(slices.Clone(input[:len(input)-3]), connector.Executable)
			if len(lookup) != len(wantLookup) {
				t.Fatalf("Lookup() = %q, want wrapper length %d", lookup, len(wantLookup))
			}
			wantLookup[len(wantLookup)-3] = lookup[len(lookup)-3]
			if !strings.Contains(lookup[len(lookup)-3], LookupMarker) || !slices.Equal(lookup, wantLookup) {
				t.Fatalf("Lookup() = %q, want original wrapper %q", lookup, wantLookup)
			}
		})
	}
}

func TestDeclaredCommandMatchesExactly(t *testing.T) {
	agent := Spec{ProcessExecutables: []string{"acme-native"}, Commands: []CommandSpec{{
		Match:  []string{"isolate-run", "acme-acp", "work"},
		ACP:    []string{"acme-acp", "--acp"},
		Resume: []string{"acme-acp", "--resume"},
	}}}.Agent("acme")
	registry := NewRegistry(agent)
	for _, argv := range [][]string{
		{"isolate-run", "acme-acp", "other"},
		{"isolate-run", "acme-acp", "work", "--version"},
		{"sh", "-c", "isolate-run acme-acp work"},
		{"acme-acp"},
		{"echo", "codex"},
		{"echo", "label=omp"},
	} {
		if got := registry.Check(argv); got.Supported || got.Reason != ReasonUnknownAgent {
			t.Fatalf("Check(%q) = %+v, want unknown agent", argv, got)
		}
	}
}

func TestDeclaredSpecValidation(t *testing.T) {
	valid := Spec{ProcessExecutables: []string{"agent-native"}, Commands: []CommandSpec{{Match: []string{"agent", ""}, ACP: []string{"agent-acp", ""}, Resume: []string{"agent", "--resume", ""}}}}
	if err := valid.Validate(); err != nil {
		t.Fatal(err)
	}
	for name, spec := range map[string]Spec{
		"no commands":  {ProcessExecutables: []string{"agent-native"}},
		"no processes": {Commands: valid.Commands},
		"empty match":  {ProcessExecutables: valid.ProcessExecutables, Commands: []CommandSpec{{ACP: []string{"a"}, Resume: []string{"r"}}}},
		"empty acp":    {ProcessExecutables: valid.ProcessExecutables, Commands: []CommandSpec{{Match: []string{"a"}, Resume: []string{"r"}}}},
		"empty resume": {ProcessExecutables: valid.ProcessExecutables, Commands: []CommandSpec{{Match: []string{"a"}, ACP: []string{"p"}}}},
		"path match":   {ProcessExecutables: valid.ProcessExecutables, Commands: []CommandSpec{{Match: []string{"/bin/a"}, ACP: []string{"p"}, Resume: []string{"r"}}}},
		"bad process":  {ProcessExecutables: []string{"/bin/a"}, Commands: valid.Commands},
		"unknown icon": {Icon: "unknown", ProcessExecutables: valid.ProcessExecutables, Commands: valid.Commands},
		"duplicate": {ProcessExecutables: valid.ProcessExecutables, Commands: []CommandSpec{
			{Match: []string{"a"}, ACP: []string{"p"}, Resume: []string{"r"}},
			{Match: []string{"a"}, ACP: []string{"q"}, Resume: []string{"s"}},
		}},
	} {
		t.Run(name, func(t *testing.T) {
			if err := spec.Validate(); err == nil {
				t.Fatal("Validate() unexpectedly succeeded")
			}
		})
	}
}
