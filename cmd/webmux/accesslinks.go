package main

import (
	"github.com/rainchan/webmux/internal/config"
	"github.com/rainchan/webmux/internal/contract"
)

// accessProvider is a remote-access system this process manages, such as
// Cloudflare Tunnel or Tailscale Serve/Funnel. To add a forwarding system,
// implement AccessLinks and append the controller to the providers passed to
// startRuntime: /local/v1/status, `webmux status` and the Windows tray's
// “复制链接” menu then list its links without further changes.
type accessProvider interface {
	// AccessLinks returns the ready entry points without a token, path or
	// query. It is called for every status request and must not block on
	// detection or network I/O.
	AccessLinks() []contract.AccessLink
}

// accessLinks collects the links of every provider in order.
func accessLinks(providers []accessProvider) []contract.AccessLink {
	var links []contract.AccessLink
	for _, p := range providers {
		links = append(links, p.AccessLinks()...)
	}
	return links
}

// AccessLinks reports the public tunnel link while cloudflared is connected.
func (tc *tunnelController) AccessLinks() []contract.AccessLink {
	tc.mu.Lock()
	defer tc.mu.Unlock()
	if tc.state != contract.TunnelReady || tc.url == "" {
		return nil
	}
	label := "Cloudflare Quick Tunnel"
	if tc.cfg.Named() {
		label = "Cloudflare Tunnel"
	}
	return []contract.AccessLink{{Provider: "cloudflare", Label: label, URL: tc.url, Public: true}}
}

// AccessLinks reports the Serve or Funnel link once it is applied. Unlike
// Status it never runs Tailscale detection.
func (tc *tailscaleController) AccessLinks() []contract.AccessLink {
	tc.mu.Lock()
	defer tc.mu.Unlock()
	if tc.state != contract.TailscaleReady || tc.url == "" {
		return nil
	}
	funnel := tc.cfg.Mode == config.TailscaleFunnel
	label := "Tailscale Serve"
	if funnel {
		label = "Tailscale Funnel"
	}
	return []contract.AccessLink{{Provider: "tailscale", Label: label, URL: tc.url, Public: funnel}}
}
