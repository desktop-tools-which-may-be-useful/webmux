// Package tunnelsettings owns private, persistent tunnel settings. It never
// grants public access; startup callers must honor its fail-closed result.
package tunnelsettings

import (
	"errors"
	"fmt"

	"github.com/rainchan/webmux/internal/config"
	"github.com/rainchan/webmux/internal/settingsstore"
	"github.com/rainchan/webmux/internal/statefile"
)

var (
	ErrUnavailable   = errors.New("tunnel settings storage unavailable")
	ErrInvalid       = errors.New("invalid tunnel settings")
	ErrEnableBlocked = errors.New("tunnel cannot be enabled")
	ErrNotPersisted  = errors.New("tunnel is off for this process, but the disable was not persisted; repair storage before restarting")
)

// Pointer fields distinguish complete files from truncated/missing values.
type storedFile struct {
	V        int     `json:"v"`
	Enabled  *bool   `json:"enabled"`
	Mode     string  `json:"mode"`
	Hostname *string `json:"hostname"`
	Token    string  `json:"token,omitempty"`
}

func (storedFile) UnavailableErr() error { return ErrUnavailable }

var spec = settingsstore.Spec[config.TunnelConfig, storedFile]{
	ID:     statefile.Tunnel,
	Decode: decodeStored,
	Normalize: func(_, next config.TunnelConfig) (config.TunnelConfig, error) {
		return Prepare(next, modeOf(next), next.Hostname, nil)
	},
	Encode: func(c config.TunnelConfig) storedFile {
		return storedFile{V: 1, Enabled: &c.Enabled, Mode: modeOf(c), Token: c.Token, Hostname: &c.Hostname}
	},
	FailClosed: func(c *config.TunnelConfig) { c.Enabled = false },
	Closed:     "tunnel settings service is closed",
}

func modeOf(c config.TunnelConfig) string {
	if c.Named() {
		return "named"
	}
	return "quick"
}

// Service holds an exclusive lock for its lifetime. A fault disables writes
// and startup exposure without changing the original file.
type Service = settingsstore.Store[config.TunnelConfig, storedFile]

func Open(stateDir string, defaults config.TunnelConfig) *Service {
	if defaults.Named() {
		if normalized, err := config.ValidateTunnelSettings("named", defaults.Hostname, defaults.Token); err == nil {
			defaults.Hostname = normalized.Hostname
		}
	}
	return settingsstore.Open(&spec, stateDir, defaults)
}
func decodeStored(f storedFile, defaults config.TunnelConfig) (config.TunnelConfig, error) {
	if f.V != 1 || f.Enabled == nil || f.Hostname == nil {
		return config.TunnelConfig{}, errors.New("incomplete tunnel settings")
	}
	next, err := config.ValidateTunnelSettings(f.Mode, *f.Hostname, f.Token)
	if err != nil || (f.Mode == "quick" && (f.Token != "" || *f.Hostname != "")) {
		return config.TunnelConfig{}, errors.New("invalid tunnel settings")
	}
	next.Enabled = *f.Enabled
	next.Cloudflared = defaults.Cloudflared
	next.DownloadMirrors = defaults.DownloadMirrors
	return next, nil
}

// Prepare validates an update, preserving the credential only on omission.
func Prepare(current config.TunnelConfig, mode, hostname string, token *string) (config.TunnelConfig, error) {
	credential := current.Token
	if token != nil {
		credential = *token
	}
	next, err := config.ValidateTunnelSettings(mode, hostname, credential)
	if err != nil {
		return config.TunnelConfig{}, fmt.Errorf("%w: %v", ErrInvalid, err)
	}
	current.Token, current.Hostname = next.Token, next.Hostname
	return current, nil
}
