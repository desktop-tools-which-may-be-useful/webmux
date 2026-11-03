package api

import (
	"github.com/rainchan/webmux/internal/contract"
	"github.com/rainchan/webmux/internal/wire"
)

// pendingUpdate is the installed version awaiting a restart, or nil.
func (s *Server) pendingUpdate() *contract.PendingUpdate {
	if s.InstalledVersion == nil {
		return nil
	}
	return wire.PendingUpdate(s.InstalledVersion(), s.Launcher)
}

// tunnelInfo describes the runtime tunnel when a controller is wired in,
// otherwise only the configured mode. Named and Hostname come from the
// configuration so the page can explain a Named Tunnel even while it is off.
func (s *Server) tunnelInfo() contract.TunnelInfo {
	info := contract.TunnelInfo{
		State:    contract.TunnelOff,
		Named:    s.Config.Tunnel.Named(),
		Hostname: s.Config.Tunnel.NamedHostname(),
	}
	if s.TunnelSettings != nil {
		settings := s.TunnelSettings.Settings()
		info.Named, info.Hostname = settings.Mode == "named", settings.Hostname
	}
	if s.Tunnel == nil {
		return info
	}
	status := s.Tunnel()
	info.State = status.State
	if info.State == "" {
		info.State = contract.TunnelOff
	}
	info.PublicURL = status.PublicURL
	info.Enabled = info.State == contract.TunnelStarting || info.State == contract.TunnelReady
	return info
}
