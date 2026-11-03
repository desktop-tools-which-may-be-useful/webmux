package api

import (
	"net"
	"net/url"
	"strings"

	"github.com/rainchan/webmux/internal/config"
)

// hostCheck establishes the authority boundary independently of Origin: a page
// loaded from a rebound hostname can supply an Origin matching that hostname.
// Loopback listeners (which allow tokenless access) restrict Host to loopback
// addresses, localhost, and configured allowed_origins; non-loopback listeners
// always require a token at startup and accept any valid Host authority so LAN
// and Tailscale MagicDNS hostnames work directly.
func hostCheck(c config.Config) func(string) bool {
	// Direct API constructors in tests can omit Listen. Startup always validates
	// a nonempty listen address before constructing the HTTP handler.
	if c.Listen == "" {
		return func(string) bool { return true }
	}
	listenHost, _, err := net.SplitHostPort(c.Listen)
	if err != nil {
		return func(string) bool { return false }
	}
	listenIP := net.ParseIP(listenHost)
	loopback := strings.EqualFold(listenHost, "localhost") || listenIP.IsLoopback()
	if !loopback {
		return func(authority string) bool {
			return authorityHostname(authority) != ""
		}
	}
	hosts := map[string]bool{strings.ToLower(listenHost): true}
	if listenIP != nil {
		hosts[listenIP.String()] = true
	}
	for _, origin := range c.AllowedOrigins {
		u, err := url.Parse(origin)
		if err != nil || u.User != nil || u.Host == "" || u.Path != "" || strings.ContainsAny(origin, "?#") || (u.Scheme != "http" && u.Scheme != "https") {
			continue
		}
		if host := authorityHostname(u.Host); host != "" {
			hosts[host] = true
		}
	}
	return func(authority string) bool {
		host := authorityHostname(authority)
		if host == "" {
			return false
		}
		if hosts[host] || host == "localhost" {
			return true
		}
		return net.ParseIP(host).IsLoopback()
	}
}

func authorityHostname(authority string) string {
	u, err := url.Parse("http://" + authority)
	if err != nil || u.User != nil || u.Host != authority || u.Path != "" || u.RawQuery != "" || u.Fragment != "" {
		return ""
	}
	host := strings.ToLower(u.Hostname())
	if ip := net.ParseIP(host); ip != nil {
		return ip.String()
	}
	return host
}
