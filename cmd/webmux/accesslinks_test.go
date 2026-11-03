package main

import (
	"reflect"
	"testing"

	"github.com/rainchan/webmux/internal/config"
	"github.com/rainchan/webmux/internal/contract"
)

func TestAccessLinksReportOnlyReadyEntryPoints(t *testing.T) {
	named := &tunnelController{cfg: config.TunnelConfig{Token: "cf-token"}, state: contract.TunnelReady, url: "https://term.example.com"}
	funnel := &tailscaleController{cfg: config.TailscaleConfig{Mode: config.TailscaleFunnel}, state: contract.TailscaleReady, url: "https://node.tailnet.ts.net"}
	want := []contract.AccessLink{
		{Provider: "cloudflare", Label: "Cloudflare Tunnel", URL: "https://term.example.com", Public: true},
		{Provider: "tailscale", Label: "Tailscale Funnel", URL: "https://node.tailnet.ts.net", Public: true},
	}
	if got := accessLinks([]accessProvider{named, funnel}); !reflect.DeepEqual(got, want) {
		t.Fatalf("ready links %+v", got)
	}

	quick := &tunnelController{state: contract.TunnelReady, url: "https://abc.trycloudflare.com"}
	serve := &tailscaleController{cfg: config.TailscaleConfig{Mode: config.TailscaleServe}, state: contract.TailscaleReady, url: "https://node.tailnet.ts.net:8443"}
	want = []contract.AccessLink{
		{Provider: "cloudflare", Label: "Cloudflare Quick Tunnel", URL: "https://abc.trycloudflare.com", Public: true},
		{Provider: "tailscale", Label: "Tailscale Serve", URL: "https://node.tailnet.ts.net:8443"},
	}
	if got := accessLinks([]accessProvider{quick, serve}); !reflect.DeepEqual(got, want) {
		t.Fatalf("quick/serve links %+v", got)
	}

	starting := &tunnelController{state: contract.TunnelStarting}
	failed := &tailscaleController{cfg: config.TailscaleConfig{Mode: config.TailscaleServe}, state: contract.TailscaleFailed, url: "https://stale.ts.net"}
	if got := accessLinks([]accessProvider{starting, failed}); len(got) != 0 {
		t.Fatalf("unready entry points listed %+v", got)
	}
}
