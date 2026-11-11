package agents

import (
	"encoding/json"
	"fmt"
	"strings"
	"testing"

	"github.com/rainchan/webmux/internal/agentprofile"
)

func declaration(process, match string) string {
	return fmt.Sprintf(`{"processExecutables":[%q],"commands":[{"match":[%q],"acp":[%q,"serve"],"resume":[%q,"resume"]}]}`, process, match, match, match)
}

func TestLoadExternalStrictDocument(t *testing.T) {
	for name, raw := range map[string]string{
		"invalid JSON":      `{`,
		"invalid UTF-8":     string([]byte{'{', 0xff, '}'}),
		"trailing JSON":     `{}{}`,
		"unknown field":     `{"acme":{"processExecutables":["acme"],"commands":[],"unknown":true}}`,
		"duplicate field":   `{"acme":{"processExecutables":["acme"],"processExecutables":["other"],"commands":[]}}`,
		"case-folded field": `{"acme":{"processExecutables":["acme"],"commands":[],"Commands":[]}}`,
		"duplicate agent":   `{"acme":` + declaration("acme", "acme") + `,"acme":` + declaration("other", "other") + `}`,
	} {
		t.Run(name, func(t *testing.T) {
			if got := LoadExternal(raw, nil); got != Default() {
				t.Fatalf("malformed document changed the built-in registry: %v", got.Titles())
			}
		})
	}
}

func TestLoadExternalStableAgentID(t *testing.T) {
	for _, name := range []string{"a", "Agent01", "acme.agent_2-v3", strings.Repeat("a", maxStableAgentIDBytes)} {
		t.Run("accept "+name, func(t *testing.T) {
			raw, err := json.Marshal(map[string]json.RawMessage{name: json.RawMessage(declaration("acme-native", "acme"))})
			if err != nil {
				t.Fatal(err)
			}
			if _, ok := LoadExternal(string(raw), nil).Lookup(name); !ok {
				t.Fatalf("valid stable ID %q was rejected", name)
			}
		})
	}
	for _, name := range []string{"", ".hidden", "-leading", "_leading", "with/slash", "with space", "with\ncontrol", "with\x00nul", "unicode-é", strings.Repeat("a", maxStableAgentIDBytes+1)} {
		t.Run("reject "+fmt.Sprintf("%q", name), func(t *testing.T) {
			raw, err := json.Marshal(map[string]json.RawMessage{
				name:   json.RawMessage(declaration("bad-native", "bad")),
				"good": json.RawMessage(declaration("good-native", "good")),
			})
			if err != nil {
				t.Fatal(err)
			}
			registry := LoadExternal(string(raw), nil)
			if _, ok := registry.Lookup(name); ok {
				t.Fatalf("unsafe stable ID %q was loaded", name)
			}
			if _, ok := registry.Lookup("good"); !ok {
				t.Fatal("invalid stable ID disabled its valid sibling")
			}
		})
	}
}

func TestLoadExternalSkipsInvalidAndConflictingEntries(t *testing.T) {
	raw := `{
		"a-valid":` + declaration("a-native", "a-command") + `,
		"b-invalid":{"processExecutables":[],"commands":[{"match":["bad"],"acp":["bad"],"resume":["bad"]}]},
		"c-duplicate":` + declaration("c-native", "a-command") + `,
		"d-shadow":` + declaration("d-native", "codex") + `,
		"e-valid":` + declaration("e-native", "e-command") + `
	}`
	registry := LoadExternal(raw, nil)
	for _, name := range []string{"a-valid", "e-valid"} {
		if _, ok := registry.Lookup(name); !ok {
			t.Fatalf("valid agent %q was not loaded", name)
		}
	}
	for _, name := range []string{"b-invalid", "c-duplicate", "d-shadow"} {
		if _, ok := registry.Lookup(name); ok {
			t.Fatalf("invalid or conflicting agent %q was loaded", name)
		}
	}
	if support := registry.Check([]string{"a-command"}); !support.Supported || support.Agent != "a-valid" {
		t.Fatalf("valid binding support = %+v", support)
	}
}

func TestLoadExternalBounds(t *testing.T) {
	t.Run("document", func(t *testing.T) {
		raw := strings.Repeat(" ", maxExternalDocumentBytes+1)
		if got := LoadExternal(raw, nil); got != Default() {
			t.Fatal("oversize document changed the built-in registry")
		}
	})
	t.Run("agents reject document", func(t *testing.T) {
		specs := make(map[string]agentprofile.Spec, maxExternalAgents+1)
		for i := 0; i <= maxExternalAgents; i++ {
			name := fmt.Sprintf("agent-%03d", i)
			specs[name] = agentprofile.Spec{ProcessExecutables: []string{name}, Commands: []agentprofile.CommandSpec{{Match: []string{name}, ACP: []string{name}, Resume: []string{name}}}}
		}
		raw, err := json.Marshal(specs)
		if err != nil {
			t.Fatal(err)
		}
		if got := LoadExternal(string(raw), nil); got != Default() {
			t.Fatal("too many agents changed the built-in registry")
		}
	})
	for name, field := range map[string]string{"match items": "match", "ACP items": "acp", "resume items": "resume"} {
		t.Run(name, func(t *testing.T) {
			items := make([]string, maxCommandItems+1)
			for i := range items {
				items[i] = fmt.Sprintf("arg-%d", i)
			}
			command := agentprofile.CommandSpec{Match: []string{"bad"}, ACP: []string{"bad"}, Resume: []string{"bad"}}
			switch field {
			case "match":
				command.Match = items
			case "acp":
				command.ACP = items
			case "resume":
				command.Resume = items
			}
			raw, _ := json.Marshal(map[string]agentprofile.Spec{
				"bad":  {ProcessExecutables: []string{"bad-native"}, Commands: []agentprofile.CommandSpec{command}},
				"good": {ProcessExecutables: []string{"good-native"}, Commands: []agentprofile.CommandSpec{{Match: []string{"good"}, ACP: []string{"good"}, Resume: []string{"good"}}}},
			})
			registry := LoadExternal(string(raw), nil)
			if _, ok := registry.Lookup("bad"); ok {
				t.Fatal("oversize argv entry was loaded")
			}
			if _, ok := registry.Lookup("good"); !ok {
				t.Fatal("valid sibling of oversize argv entry was skipped")
			}
		})
	}
	t.Run("commands skip only their agent", func(t *testing.T) {
		commands := make([]agentprofile.CommandSpec, maxCommandsPerAgent+1)
		for i := range commands {
			executable := fmt.Sprintf("bad-%d", i)
			commands[i] = agentprofile.CommandSpec{Match: []string{executable}, ACP: []string{executable}, Resume: []string{executable}}
		}
		raw, _ := json.Marshal(map[string]agentprofile.Spec{
			"bad":  {ProcessExecutables: []string{"bad-native"}, Commands: commands},
			"good": {ProcessExecutables: []string{"good-native"}, Commands: []agentprofile.CommandSpec{{Match: []string{"good"}, ACP: []string{"good"}, Resume: []string{"good"}}}},
		})
		registry := LoadExternal(string(raw), nil)
		if _, ok := registry.Lookup("bad"); ok {
			t.Fatal("agent with too many commands was loaded")
		}
		if _, ok := registry.Lookup("good"); !ok {
			t.Fatal("valid sibling of command-heavy agent was skipped")
		}
	})
	t.Run("token skips only its entry", func(t *testing.T) {
		oversize := strings.Repeat("x", maxCommandItemBytes+1)
		raw := `{"bad":{"processExecutables":["bad"],"commands":[{"match":["bad"],"acp":["bad",` + fmt.Sprintf("%q", oversize) + `],"resume":["bad"]}]},"good":` + declaration("good-native", "good") + `}`
		registry := LoadExternal(raw, nil)
		if _, ok := registry.Lookup("bad"); ok {
			t.Fatal("oversize entry was loaded")
		}
		if _, ok := registry.Lookup("good"); !ok {
			t.Fatal("valid sibling of oversize entry was skipped")
		}
	})
}

func TestLoadExternalRejectsIconsOutsideCatalog(t *testing.T) {
	for name, icon := range map[string]string{
		"unknown":  "unknown",
		"oversize": strings.Repeat("x", 1<<20),
	} {
		t.Run(name, func(t *testing.T) {
			raw, err := json.Marshal(map[string]agentprofile.Spec{
				"bad": {
					Icon: icon, ProcessExecutables: []string{"bad-native"},
					Commands: []agentprofile.CommandSpec{{Match: []string{"bad"}, ACP: []string{"bad-acp"}, Resume: []string{"bad-resume"}}},
				},
				"good": {
					Icon: "terminal", ProcessExecutables: []string{"good-native"},
					Commands: []agentprofile.CommandSpec{{Match: []string{"good"}, ACP: []string{"good-acp"}, Resume: []string{"good-resume"}}},
				},
			})
			if err != nil {
				t.Fatal(err)
			}
			registry := LoadExternal(string(raw), nil)
			if _, ok := registry.Lookup("bad"); ok {
				t.Fatal("invalid icon entry was loaded")
			}
			if _, ok := registry.Lookup("good"); !ok {
				t.Fatal("valid sibling was skipped")
			}
		})
	}
}

func TestLoadExternalSortsStableNames(t *testing.T) {
	registry := LoadExternal(`{"z-agent":`+declaration("z-native", "z-command")+`,"a-agent":`+declaration("a-native", "a-command")+`}`, nil)
	titles := registry.Titles()
	builtins := len(Default().Titles())
	if got := titles[builtins:]; strings.Join(got, ",") != "a-agent,z-agent" {
		t.Fatalf("external order = %q", got)
	}
}

func TestDefaultIgnoresProcessEnvironment(t *testing.T) {
	t.Setenv(ExternalAgentsEnv, `{"acme":`+declaration("acme-native", "acme")+`}`)
	if _, ok := Default().Lookup("acme"); ok {
		t.Fatal("Default read external declarations from process environment")
	}
}
