package agents

import (
	"log/slog"
	"os"
	"slices"
	"sort"
	"strings"

	"github.com/rainchan/webmux/internal/agentprofile"
	"github.com/rainchan/webmux/internal/jsonutil"
)

// ExternalAgentsEnv names the JSON object that declares external ACP agents.
// Each stable agent name maps to process identity and complete command bindings.
const ExternalAgentsEnv = "WEBMUX_ACP_AGENTS"

const (
	maxExternalDocumentBytes = 4 << 20
	maxExternalAgents        = 100
	maxStableAgentIDBytes    = 64
	maxCommandsPerAgent      = 100
	maxCommandItems          = 128
	maxCommandItemBytes      = 8192
)

// LoadExternal extends the built-ins with valid declarations in stable name
// order. A malformed document leaves the built-ins unchanged. A bad entry is
// logged and skipped without disabling other entries. It reads no process
// state; the assembly root supplies raw after applying its launch environment.
func LoadExternal(raw string, log *slog.Logger) *agentprofile.Registry {
	if log == nil {
		log = slog.New(slog.DiscardHandler)
	}
	if len(raw) > maxExternalDocumentBytes {
		log.Warn("ignoring declared agents", "env", ExternalAgentsEnv, "error", "document too large")
		return defaultRegistry
	}
	if strings.TrimSpace(raw) == "" {
		return defaultRegistry
	}
	var specs map[string]agentprofile.Spec
	if err := jsonutil.Decode([]byte(raw), &specs); err != nil {
		log.Warn("ignoring declared agents", "env", ExternalAgentsEnv, "error", err)
		return defaultRegistry
	}
	if len(specs) > maxExternalAgents {
		log.Warn("ignoring declared agents", "env", ExternalAgentsEnv, "error", "too many agents")
		return defaultRegistry
	}
	names := make([]string, 0, len(specs))
	for name := range specs {
		names = append(names, name)
	}
	sort.Strings(names)
	accepted := defaultRegistry.Agents()
	registry := defaultRegistry
	for _, name := range names {
		spec := specs[name]
		if !validStableAgentID(name) {
			log.Warn("ignoring declared agent", "error", "invalid stable agent ID")
			continue
		}
		var err error
		switch {
		case len(spec.Commands) > maxCommandsPerAgent:
			err = os.ErrInvalid
		default:
			err = spec.Validate()
			if err == nil {
				err = validateCommandLimits(spec)
			}
		}
		if err != nil {
			log.Warn("ignoring declared agent", "name", name, "error", err)
			continue
		}
		candidate := spec.Agent(name)
		next, conflict := agentprofile.NewRegistryChecked(append(slices.Clone(accepted), candidate)...)
		if conflict != nil {
			log.Warn("ignoring declared agent", "name", name, "error", conflict)
			continue
		}
		accepted = append(accepted, candidate)
		registry = next
	}
	return registry
}

func validStableAgentID(name string) bool {
	if len(name) == 0 || len(name) > maxStableAgentIDBytes {
		return false
	}
	for i := range len(name) {
		c := name[i]
		alphaNumeric := c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' || c >= '0' && c <= '9'
		if !alphaNumeric && (i == 0 || c != '.' && c != '_' && c != '-') {
			return false
		}
	}
	return true
}

func validateCommandLimits(spec agentprofile.Spec) error {
	for _, command := range spec.Commands {
		for _, argv := range [][]string{command.Match, command.ACP, command.Resume} {
			if len(argv) > maxCommandItems {
				return os.ErrInvalid
			}
			for _, token := range argv {
				if len(token) > maxCommandItemBytes {
					return os.ErrInvalid
				}
			}
		}
	}
	return nil
}
