// Package tailscalesettings owns the private, persistent web override of the
// [tailscale] mode, port and provider. It never applies an entry itself;
// startup callers must honor its fail-closed result.
package tailscalesettings

import (
	"errors"
	"fmt"

	"github.com/rainchan/webmux/internal/config"
	"github.com/rainchan/webmux/internal/settingsstore"
	"github.com/rainchan/webmux/internal/statefile"
)

var (
	ErrUnavailable = errors.New("tailscale settings storage unavailable")
	ErrInvalid     = errors.New("invalid tailscale settings")
	// ErrEnableBlocked rejects an entry this process cannot safely publish.
	ErrEnableBlocked = errors.New("tailscale entry cannot be enabled")
	// ErrFunnelUnconfirmed rejects a switch to Funnel without the explicit
	// public-exposure confirmation.
	ErrFunnelUnconfirmed = errors.New("enabling Funnel exposes WebMux to the public internet; confirm the risk to continue")
)

// Pointer fields distinguish complete files from truncated/missing values.
type storedFile struct {
	V        int     `json:"v"`
	Mode     *string `json:"mode"`
	Port     *int    `json:"port"`
	Provider *string `json:"provider"`
}

func (storedFile) UnavailableErr() error { return ErrUnavailable }

var spec = settingsstore.Spec[config.TailscaleConfig, storedFile]{
	ID:     statefile.TailscaleSettings,
	Decode: decodeStored,
	Normalize: func(defaults, next config.TailscaleConfig) (config.TailscaleConfig, error) {
		return Prepare(defaults, next.Mode, next.Port, next.Provider)
	},
	Encode: func(c config.TailscaleConfig) storedFile {
		return storedFile{V: 1, Mode: &c.Mode, Port: &c.Port, Provider: &c.Provider}
	},
	FailClosed: func(c *config.TailscaleConfig) { c.Mode = config.TailscaleOff },
	Closed:     "tailscale settings service is closed",
}

// Service holds an exclusive lock for its lifetime. A fault disables writes
// and forces the effective mode off without changing the original file.
type Service = settingsstore.Store[config.TailscaleConfig, storedFile]

// Open loads the override for stateDir. defaults is the validated TOML value.
func Open(stateDir string, defaults config.TailscaleConfig) *Service {
	return settingsstore.Open(&spec, stateDir, defaults)
}

func decodeStored(f storedFile, defaults config.TailscaleConfig) (config.TailscaleConfig, error) {
	if f.V != 1 || f.Mode == nil || f.Port == nil || f.Provider == nil {
		return config.TailscaleConfig{}, errors.New("incomplete tailscale settings")
	}
	return config.ValidateTailscaleSettings(defaults, *f.Mode, *f.Port, *f.Provider)
}

// Prepare validates an update against current, keeping the TOML command.
func Prepare(current config.TailscaleConfig, mode string, port int, provider string) (config.TailscaleConfig, error) {
	next, err := config.ValidateTailscaleSettings(current, mode, port, provider)
	if err != nil {
		return config.TailscaleConfig{}, fmt.Errorf("%w: %v", ErrInvalid, err)
	}
	return next, nil
}
