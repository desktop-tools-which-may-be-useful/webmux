// Package voicesettings owns the private, persistent web override of the
// [voice] recognition command: its argv and environment. It never starts a
// recognizer itself; callers apply the effective value.
package voicesettings

import (
	"errors"
	"fmt"
	"slices"

	"github.com/rainchan/webmux/internal/config"
	"github.com/rainchan/webmux/internal/settingsstore"
	"github.com/rainchan/webmux/internal/statefile"
)

var (
	ErrUnavailable = errors.New("voice settings storage unavailable")
	ErrInvalid     = errors.New("invalid voice settings")
)

// Pointer fields distinguish complete files from truncated or missing ones.
type storedFile struct {
	V   int       `json:"v"`
	Cmd *[]string `json:"cmd"`
	Env *[]string `json:"env"`
}

func (storedFile) UnavailableErr() error { return ErrUnavailable }

var spec = settingsstore.Spec[config.VoiceConfig, storedFile]{
	ID:     statefile.VoiceSettings,
	Decode: decodeStored,
	Normalize: func(_, next config.VoiceConfig) (config.VoiceConfig, error) {
		return Prepare(next)
	},
	Encode: func(c config.VoiceConfig) storedFile {
		cmd, env := slices.Clone(c.Cmd), slices.Clone(c.Env)
		if cmd == nil {
			cmd = []string{}
		}
		if env == nil {
			env = []string{}
		}
		return storedFile{V: 1, Cmd: &cmd, Env: &env}
	},
	// A damaged store must not run a recognizer nobody can inspect.
	FailClosed: func(c *config.VoiceConfig) { *c = config.VoiceConfig{} },
	Closed:     "voice settings service is closed",
}

// Service holds an exclusive lock for its lifetime. A fault disables writes
// and turns speech recognition off without changing the original file.
type Service = settingsstore.Store[config.VoiceConfig, storedFile]

// Open loads the override for stateDir. defaults is the validated TOML value.
func Open(stateDir string, defaults config.VoiceConfig) *Service {
	return settingsstore.Open(&spec, stateDir, defaults)
}

func decodeStored(f storedFile, _ config.VoiceConfig) (config.VoiceConfig, error) {
	if f.V != 1 || f.Cmd == nil || f.Env == nil {
		return config.VoiceConfig{}, errors.New("incomplete voice settings")
	}
	return config.VoiceConfig{Cmd: *f.Cmd, Env: *f.Env}.Validated()
}

// Prepare validates a proposed value with the checks [voice] gets.
func Prepare(next config.VoiceConfig) (config.VoiceConfig, error) {
	out, err := next.Validated()
	if err != nil {
		return config.VoiceConfig{}, fmt.Errorf("%w: %v", ErrInvalid, err)
	}
	return out, nil
}
