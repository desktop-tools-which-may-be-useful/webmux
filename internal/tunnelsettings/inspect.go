package tunnelsettings

import (
	"github.com/rainchan/webmux/internal/config"
	"github.com/rainchan/webmux/internal/settingsstore"
)

func Inspect(dir string, defaults config.TunnelConfig) (config.TunnelConfig, bool, error) {
	return settingsstore.Inspect(&spec, dir, defaults)
}
