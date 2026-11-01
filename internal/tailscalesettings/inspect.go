package tailscalesettings

import (
	"github.com/rainchan/webmux/internal/config"
	"github.com/rainchan/webmux/internal/settingsstore"
)

func Inspect(dir string, defaults config.TailscaleConfig) (config.TailscaleConfig, bool, error) {
	return settingsstore.Inspect(&spec, dir, defaults)
}
