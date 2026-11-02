// Package listensettings owns the persistent network-exposure switch: whether
// the TCP listener binds every interface or stays on the configured address.
// A storage fault keeps the listener off the network (fail closed).
package listensettings

import (
	"errors"
	"net"

	"github.com/rainchan/webmux/internal/settingsstore"
	"github.com/rainchan/webmux/internal/statefile"
)

var ErrUnavailable = errors.New("network exposure settings storage unavailable")

// Settings is the desired exposure. Public binds every interface on the
// configured port; otherwise the configured listen address is used, or
// loopback when that address is itself a wildcard.
type Settings struct {
	Public bool
}

// Pointer fields distinguish complete files from truncated/missing values.
type storedFile struct {
	V      int   `json:"v"`
	Public *bool `json:"public"`
}

func (storedFile) UnavailableErr() error { return ErrUnavailable }

var spec = settingsstore.Spec[Settings, storedFile]{
	ID: statefile.ListenSettings,
	Decode: func(f storedFile, _ Settings) (Settings, error) {
		if f.V != 1 || f.Public == nil {
			return Settings{}, errors.New("incomplete network exposure settings")
		}
		return Settings{Public: *f.Public}, nil
	},
	Normalize:  func(_, next Settings) (Settings, error) { return next, nil },
	Encode:     func(s Settings) storedFile { return storedFile{V: 1, Public: &s.Public} },
	FailClosed: func(s *Settings) { s.Public = false },
	Closed:     "network exposure settings service is closed",
}

// Service holds an exclusive lock for its lifetime. A fault disables writes
// and keeps startup off the network without changing the original file.
type Service = settingsstore.Store[Settings, storedFile]

// Open loads the override for stateDir. configured is the TOML (or flag)
// listen address, which supplies the default.
func Open(stateDir, configured string) *Service {
	return settingsstore.Open(&spec, stateDir, Settings{Public: Wildcard(configured)})
}

// Inspect reads the stored override without taking the lock.
func Inspect(dir, configured string) (Settings, bool, error) {
	return settingsstore.Inspect(&spec, dir, Settings{Public: Wildcard(configured)})
}

// Wildcard reports whether listen binds every interface.
func Wildcard(listen string) bool {
	host, _, err := net.SplitHostPort(listen)
	if err != nil {
		return false
	}
	if host == "" {
		return true
	}
	ip := net.ParseIP(host)
	return ip != nil && ip.IsUnspecified()
}

// Address resolves the listen address for the desired exposure. configured
// is the TOML (or flag) address and port the bound port, which differs from
// the configured one only when it was 0. A public request on a wildcard
// configuration keeps that wildcard (for example [::]); a private request
// on one falls back to IPv4 loopback.
func Address(configured string, port string, public bool) (string, error) {
	host, _, err := net.SplitHostPort(configured)
	if err != nil {
		return "", err
	}
	switch {
	case public && Wildcard(configured):
	case public:
		host = "0.0.0.0"
	case Wildcard(configured):
		host = "127.0.0.1"
	}
	return net.JoinHostPort(host, port), nil
}
