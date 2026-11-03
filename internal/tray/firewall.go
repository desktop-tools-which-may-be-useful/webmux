package tray

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"strings"
)

// Exit codes of `webmux-tray firewall`.
const (
	FirewallDone     = 0 // the rule is in the requested state
	FirewallFailed   = 1 // netsh failed or the rule did not change
	FirewallUsage    = 2 // invalid arguments
	FirewallDeclined = 3 // the user declined the administrator prompt
)

// ErrElevationDeclined is returned by Firewall.Elevate when the user
// declines the administrator prompt.
var ErrElevationDeclined = errors.New("administrator prompt declined")

// ParseFirewall parses the arguments after `firewall`: `allow`, or
// `remove` with an optional absolute path of the tray whose rule to remove
// (the installer records it, and the install directory may have changed).
// program is "" for this executable.
func ParseFirewall(args []string) (allow bool, program string, err error) {
	switch {
	case len(args) == 1 && args[0] == "allow":
		return true, "", nil
	case len(args) == 1 && args[0] == "remove":
		return false, "", nil
	case len(args) == 2 && args[0] == "remove" && isAbsWindowsPath(args[1]):
		return false, args[1], nil
	}
	return false, "", errors.New("firewall takes allow, or remove with an optional absolute tray path")
}

// isAbsWindowsPath reports a drive-absolute (C:\...) or UNC path.
func isAbsWindowsPath(p string) bool {
	if strings.HasPrefix(p, `\\`) {
		return len(p) > 2
	}
	return len(p) > 3 && p[1] == ':' && (p[2] == '\\' || p[2] == '/') &&
		(p[0] >= 'A' && p[0] <= 'Z' || p[0] >= 'a' && p[0] <= 'z')
}

// FirewallRule is the inbound Windows Defender Firewall rule that lets
// other devices reach the tray's forwarding listeners. It belongs to one
// tray executable: the name carries a digest of its path, so installs in
// different directories never share or remove each other's rule, and a
// silent self-update in place keeps it.
type FirewallRule struct {
	Exe string
}

// Name is the rule's display name.
func (r FirewallRule) Name() string {
	sum := sha256.Sum256([]byte(strings.ToLower(r.Exe)))
	return "WebMux tray forwarding " + hex.EncodeToString(sum[:6])
}

// ShowArgs are the netsh arguments that succeed only when the rule exists.
// They need no administrator rights.
func (r FirewallRule) ShowArgs() []string {
	return []string{"advfirewall", "firewall", "show", "rule", "name=" + r.Name()}
}

// AddArgs add the rule: inbound TCP for the tray executable on private and
// domain networks only, never public ones.
func (r FirewallRule) AddArgs() []string {
	return []string{
		"advfirewall", "firewall", "add", "rule",
		"name=" + r.Name(),
		"dir=in", "action=allow", "protocol=TCP",
		"program=" + r.Exe,
		"profile=private,domain",
		"enable=yes",
		"description=Lets trusted devices reach WebMux through the tray's forwarding; added by the WebMux installer.",
	}
}

// DeleteArgs remove the rule.
func (r FirewallRule) DeleteArgs() []string {
	return []string{"advfirewall", "firewall", "delete", "rule", "name=" + r.Name()}
}

// Firewall brings Rule to the requested state through netsh. Exists runs
// netsh without administrator rights; Elevate runs it as administrator and
// returns its exit code, or ErrElevationDeclined.
type Firewall struct {
	Rule    FirewallRule
	Exists  func(args []string) (bool, error)
	Elevate func(args []string) (int, error)
}

// Apply adds (allow) or removes the rule and returns an exit code. It asks
// for administrator rights only when the rule is not already in that state.
func (f Firewall) Apply(allow bool) int {
	exists, err := f.Exists(f.Rule.ShowArgs())
	if err != nil {
		return FirewallFailed
	}
	if exists == allow {
		return FirewallDone
	}
	args := f.Rule.DeleteArgs()
	if allow {
		args = f.Rule.AddArgs()
	}
	code, err := f.Elevate(args)
	switch {
	case errors.Is(err, ErrElevationDeclined):
		return FirewallDeclined
	case err != nil || code != 0:
		return FirewallFailed
	}
	if exists, err = f.Exists(f.Rule.ShowArgs()); err != nil || exists != allow {
		return FirewallFailed
	}
	return FirewallDone
}
