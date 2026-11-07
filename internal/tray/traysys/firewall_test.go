package traysys

import (
	"errors"
	"slices"
	"strings"
	"testing"
)

// allow always targets this tray; only remove names another path, which
// must be absolute so a relative argument cannot pick an unintended rule.
func TestParseFirewall(t *testing.T) {
	for _, tc := range []struct {
		args    []string
		allow   bool
		program string
		ok      bool
	}{
		{[]string{"allow"}, true, "", true},
		{[]string{"remove"}, false, "", true},
		{[]string{"remove", `D:\Old\webmux-tray.exe`}, false, `D:\Old\webmux-tray.exe`, true},
		{[]string{"remove", `\\server\share\webmux-tray.exe`}, false, `\\server\share\webmux-tray.exe`, true},
		{[]string{"remove", `webmux-tray.exe`}, false, "", false},
		{[]string{"allow", `D:\Other\evil.exe`}, false, "", false},
		{nil, false, "", false},
		{[]string{"add"}, false, "", false},
	} {
		allow, program, err := ParseFirewall(tc.args)
		if (err == nil) != tc.ok || (tc.ok && (allow != tc.allow || program != tc.program)) {
			t.Errorf("ParseFirewall(%q) = %v, %q, %v", tc.args, allow, program, err)
		}
	}
}

// One rule per tray executable: the name follows the path (Windows paths
// compare case-insensitively), so another install never shares it.
func TestFirewallRuleNamePerExecutable(t *testing.T) {
	a := FirewallRule{Exe: `C:\Users\u\AppData\Local\Programs\WebMux\webmux-tray.exe`}
	if got := (FirewallRule{Exe: strings.ToUpper(a.Exe)}).Name(); got != a.Name() {
		t.Errorf("case changed the name: %q vs %q", got, a.Name())
	}
	if b := (FirewallRule{Exe: `D:\Tools\WebMux\webmux-tray.exe`}); b.Name() == a.Name() {
		t.Errorf("two paths share %q", a.Name())
	}
	if !strings.HasPrefix(a.Name(), "WebMux tray forwarding ") {
		t.Errorf("name %q", a.Name())
	}
}

// The rule opens inbound TCP for exactly this program and never on public
// networks.
func TestFirewallRuleAddArgs(t *testing.T) {
	r := FirewallRule{Exe: `C:\Program Files\Web Mux\webmux-tray.exe`}
	args := r.AddArgs()
	for _, want := range []string{"dir=in", "action=allow", "protocol=TCP", "profile=private,domain", "program=" + r.Exe, "name=" + r.Name()} {
		if !slices.Contains(args, want) {
			t.Errorf("AddArgs lacks %q: %q", want, args)
		}
	}
	for _, a := range args {
		if strings.HasPrefix(a, "profile=") && a != "profile=private,domain" {
			t.Errorf("AddArgs opens more than private and domain networks: %q", a)
		}
	}
}

type fakeNetsh struct {
	exists   bool
	showErr  error
	elevated [][]string
	code     int
	err      error
	// apply makes a successful elevated run change exists.
	apply bool
}

func (n *fakeNetsh) firewall() Firewall {
	return Firewall{
		Rule: FirewallRule{Exe: `C:\WebMux\webmux-tray.exe`},
		Exists: func(args []string) (bool, error) {
			if !slices.Equal(args[:4], []string{"advfirewall", "firewall", "show", "rule"}) {
				return false, errors.New("unexpected show args")
			}
			return n.exists, n.showErr
		},
		Elevate: func(args []string) (int, error) {
			n.elevated = append(n.elevated, args)
			if n.err == nil && n.code == 0 && n.apply {
				n.exists = args[2] == "add"
			}
			return n.code, n.err
		},
	}
}

func TestFirewallApply(t *testing.T) {
	for _, tc := range []struct {
		name      string
		netsh     fakeNetsh
		allow     bool
		want      int
		elevation string // the netsh verb run as administrator, or ""
	}{
		{name: "allow adds a missing rule", netsh: fakeNetsh{apply: true}, allow: true, want: FirewallDone, elevation: "add"},
		{name: "allow with the rule present asks nothing", netsh: fakeNetsh{exists: true}, allow: true, want: FirewallDone},
		{name: "remove deletes the rule", netsh: fakeNetsh{exists: true, apply: true}, allow: false, want: FirewallDone, elevation: "delete"},
		{name: "remove without a rule asks nothing", netsh: fakeNetsh{}, allow: false, want: FirewallDone},
		{name: "declined prompt", netsh: fakeNetsh{err: ErrElevationDeclined}, allow: true, want: FirewallDeclined, elevation: "add"},
		{name: "netsh fails", netsh: fakeNetsh{code: 1}, allow: true, want: FirewallFailed, elevation: "add"},
		{name: "rule still missing after netsh", netsh: fakeNetsh{}, allow: true, want: FirewallFailed, elevation: "add"},
		{name: "cannot query rules", netsh: fakeNetsh{showErr: errors.New("no netsh")}, allow: true, want: FirewallFailed},
	} {
		t.Run(tc.name, func(t *testing.T) {
			n := tc.netsh
			if got := n.firewall().Apply(tc.allow); got != tc.want {
				t.Errorf("Apply(%v) = %d, want %d", tc.allow, got, tc.want)
			}
			switch {
			case tc.elevation == "" && len(n.elevated) != 0:
				t.Errorf("asked for administrator rights: %q", n.elevated)
			case tc.elevation != "" && (len(n.elevated) != 1 || n.elevated[0][2] != tc.elevation):
				t.Errorf("elevated %q, want one %s", n.elevated, tc.elevation)
			}
		})
	}
}
