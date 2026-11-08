package main

import (
	"slices"
	"sort"
	"strings"
	"sync"

	"github.com/rainchan/webmux/cmd/internal/assemble"
	"github.com/rainchan/webmux/internal/config"
	"github.com/rainchan/webmux/internal/contract"
	"github.com/rainchan/webmux/internal/voice"
	"github.com/rainchan/webmux/internal/voicesettings"
)

// voiceController applies the web-edited [voice] recognizer: it validates and
// persists an update, then hands the recognizer service its new command.
type voiceController struct {
	store *voicesettings.Service
	svc   *voice.Service
	// base supplies the working directory; its Voice is replaced by each
	// update.
	base config.Config

	// mu serializes updates and resets, so the store and the running
	// service never disagree.
	mu sync.Mutex
}

func newVoiceController(store *voicesettings.Service, svc *voice.Service, base config.Config) *voiceController {
	return &voiceController{store: store, svc: svc, base: base}
}

func (c *voiceController) options(v config.VoiceConfig) voice.Options {
	cfg := c.base
	cfg.Voice = v
	return assemble.VoiceOptions(cfg)
}

func (c *voiceController) Settings() contract.VoiceSettings {
	cfg, web := c.store.Effective()
	out := contract.VoiceSettings{
		Cmd:              slices.Clone(cfg.Cmd),
		Env:              envMap(cfg.Env),
		Enabled:          cfg.Enabled(),
		Source:           "config",
		StorageAvailable: true,
	}
	if out.Cmd == nil {
		out.Cmd = []string{}
	}
	if web {
		out.Source = "web"
	}
	if err := c.store.Unavailable(); err != nil {
		out.StorageAvailable, out.StorageError = false, err.Error()
	}
	return out
}

func (c *voiceController) Update(in contract.VoiceSettingsUpdate) (contract.VoiceSettings, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	next, err := voicesettings.Prepare(config.VoiceConfig{Cmd: in.Cmd, Env: envList(in.Env)})
	if err != nil {
		return contract.VoiceSettings{}, err
	}
	if err := c.store.Save(next); err != nil {
		return contract.VoiceSettings{}, err
	}
	c.svc.Reconfigure(c.options(next))
	return c.Settings(), nil
}

// Reset drops the web override and runs the [voice] of config.toml again.
func (c *voiceController) Reset() (contract.VoiceSettings, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	defaults, err := c.store.Reset()
	if err != nil {
		return contract.VoiceSettings{}, err
	}
	c.svc.Reconfigure(c.options(defaults))
	return c.Settings(), nil
}

// envMap turns KEY=VALUE entries into the map the API carries.
func envMap(entries []string) map[string]string {
	out := make(map[string]string, len(entries))
	for _, entry := range entries {
		if name, value, ok := strings.Cut(entry, "="); ok {
			out[name] = value
		}
	}
	return out
}

// envList turns the API's map into KEY=VALUE entries in a stable order.
func envList(env map[string]string) []string {
	out := make([]string, 0, len(env))
	for name, value := range env {
		out = append(out, name+"="+value)
	}
	sort.Strings(out)
	return out
}
