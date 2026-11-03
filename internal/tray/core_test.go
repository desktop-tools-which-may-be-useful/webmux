package tray

import (
	"context"
	"errors"
	"net"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/rainchan/webmux/internal/contract"
	"github.com/rainchan/webmux/internal/update"
)

func TestSettings(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "tray.json")
	s, err := LoadSettings(path)
	if err != nil || !reflect.DeepEqual(s, DefaultSettings()) {
		t.Fatalf("missing file: %+v %v", s, err)
	}
	if s.Webmux != "~/.local/bin/webmux" || s.Mode != ModeAuto || !s.StartWebMux || !s.AutoRestart || s.Config != "" || !s.AutoUpdate || s.LastVersion != "" {
		t.Fatalf("defaults %+v", s)
	}
	for name, content := range map[string]string{
		"corrupt":      "{not json",
		"unknown":      `{"mode":"auto","token":"secret"}`,
		"invalid mode": `{"mode":"docker"}`,
		"relative":     `{"config":"configs/webmux.toml"}`,
		"loopback":     `{"forward":["127.0.0.1"]}`,
		"trailing":     `{"mode":"tray"} {}`,
	} {
		if err := os.WriteFile(path, []byte(content), 0600); err != nil {
			t.Fatal(err)
		}
		s, err := LoadSettings(path)
		if err == nil || !reflect.DeepEqual(s, DefaultSettings()) {
			t.Errorf("%s: %+v %v", name, s, err)
		}
		if data, _ := os.ReadFile(path); string(data) != content {
			t.Errorf("%s: file rewritten", name)
		}
	}
	custom := DefaultSettings()
	custom.Distro, custom.Mode, custom.Config, custom.Forward = "Ubuntu-24.04", ModeSystemd, "~/my configs/webmux;$(id).toml", []string{"100.64.0.9"}
	if err := SaveSettings(path, custom); err != nil {
		t.Fatal(err)
	}
	data, _ := os.ReadFile(path)
	if strings.Contains(strings.ToLower(string(data)), "token") {
		t.Fatalf("settings mention a token: %s", data)
	}
	if got, err := LoadSettings(path); err != nil || !reflect.DeepEqual(got, custom) {
		t.Fatalf("round trip %+v %v", got, err)
	}
	// A UTF-8 BOM from Notepad is accepted.
	if err := os.WriteFile(path, append([]byte("\xef\xbb\xbf"), data...), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := LoadSettings(path); err != nil {
		t.Fatalf("BOM: %v", err)
	}
}

// A settings file written before automatic updates existed keeps them on.
func TestSettingsWithoutAutoUpdate(t *testing.T) {
	path := filepath.Join(t.TempDir(), "tray.json")
	old := `{"distro":"Ubuntu","mode":"tray","webmux":"~/.local/bin/webmux","config":"","logFile":"~/w.log","startWebMux":false,"autoRestart":true,"forward":[]}`
	if err := os.WriteFile(path, []byte(old), 0600); err != nil {
		t.Fatal(err)
	}
	s, err := LoadSettings(path)
	if err != nil || !s.AutoUpdate || s.LastVersion != "" || s.Distro != "Ubuntu" || s.StartWebMux {
		t.Fatalf("old file: %+v %v", s, err)
	}
	s.AutoUpdate, s.LastVersion = false, "0.4.0"
	if err := SaveSettings(path, s); err != nil {
		t.Fatal(err)
	}
	if got, err := LoadSettings(path); err != nil || got.AutoUpdate || got.LastVersion != "0.4.0" {
		t.Fatalf("round trip %+v %v", got, err)
	}
}

// A file without updateChannel follows stable without a warning; an
// unrecognised value follows stable with one and keeps the other settings.
func TestSettingsUpdateChannel(t *testing.T) {
	path := filepath.Join(t.TempDir(), "tray.json")
	for _, tc := range []struct {
		name, field string
		want        update.Channel
		warn        bool
	}{
		{"older file", ``, update.ChannelStable, false},
		{"stable", `,"updateChannel":"stable"`, update.ChannelStable, false},
		{"dev", `,"updateChannel":"dev"`, update.ChannelDev, false},
		{"unrecognised", `,"updateChannel":"beta"`, update.ChannelStable, true},
	} {
		file := `{"distro":"Ubuntu","mode":"tray","webmux":"~/.local/bin/webmux","config":"","logFile":"~/w.log","startWebMux":true,"autoRestart":true,"forward":[]` + tc.field + `}`
		if err := os.WriteFile(path, []byte(file), 0o600); err != nil {
			t.Fatal(err)
		}
		s, err := LoadSettings(path)
		ch, warn := s.Channel()
		if err != nil || s.Distro != "Ubuntu" || ch != tc.want || (warn != nil) != tc.warn {
			t.Errorf("%s: %+v %v, channel %s warn %v", tc.name, s, err, ch, warn)
		}
	}
}

func TestRecordVersion(t *testing.T) {
	for _, tc := range []struct {
		name, last, version string
		updated             string
		changed             bool
	}{
		{"first run", "", "0.5.0", "", true},
		{"updated", "0.4.0", "0.5.0", "0.5.0", true},
		{"same", "0.5.0", "0.5.0", "", false},
		{"downgraded", "0.6.0", "0.5.0", "", true},
		{"from a development build", "dev", "0.5.0", "", true},
		{"development build", "0.5.0", "dev", "", false},
		{"to a dev build", "0.10.2", "0.10.3-dev.2", "0.10.3-dev.2", true},
		{"to the release after a dev build", "0.10.3-dev.2", "0.10.3", "0.10.3", true},
	} {
		s := DefaultSettings()
		s.LastVersion = tc.last
		next, updated, failed, changed := RecordVersion(s, tc.version)
		if updated != tc.updated || changed != tc.changed || !failed.IsZero() {
			t.Errorf("%s: updated %q failed %+v changed %v", tc.name, updated, failed, changed)
		}
		if want := map[bool]string{true: tc.version, false: tc.last}[changed]; next.LastVersion != want {
			t.Errorf("%s: lastVersion %q", tc.name, next.LastVersion)
		}
	}
}

// A recorded attempt at a newer release means its install failed and stays
// recorded; once that release or a later one runs the attempt is cleared.
func TestRecordVersionUpdateAttempt(t *testing.T) {
	at := time.Date(2026, 11, 3, 10, 0, 0, 0, time.UTC)
	for _, tc := range []struct {
		name, last, attempt, version string
		failed, changed              bool
	}{
		{"install failed", "0.4.0", "0.5.0", "0.4.0", true, false},
		{"installed", "0.4.0", "0.5.0", "0.5.0", false, true},
		{"installed a later release", "0.4.0", "0.5.0", "0.6.0", false, true},
		{"failed after an earlier update", "0.3.0", "0.5.0", "0.4.0", true, true},
		{"development build", "0.4.0", "0.5.0", "dev", false, false},
	} {
		s := DefaultSettings()
		s.LastVersion = tc.last
		s.UpdateAttempt = UpdateAttempt{Version: tc.attempt, At: at}
		next, _, failed, changed := RecordVersion(s, tc.version)
		if changed != tc.changed || (failed == s.UpdateAttempt) != tc.failed || (!tc.failed && !failed.IsZero()) {
			t.Errorf("%s: failed %+v changed %v", tc.name, failed, changed)
		}
		if keep := tc.failed || tc.version == "dev"; (next.UpdateAttempt == s.UpdateAttempt) != keep || (!keep && !next.UpdateAttempt.IsZero()) {
			t.Errorf("%s: attempt %+v", tc.name, next.UpdateAttempt)
		}
	}
}

// The attempt survives a save, and a file without one does not mention it,
// so trays that predate the field still read it.
func TestSettingsUpdateAttempt(t *testing.T) {
	path := filepath.Join(t.TempDir(), "tray.json")
	s := DefaultSettings()
	if err := SaveSettings(path, s); err != nil {
		t.Fatal(err)
	}
	if data, _ := os.ReadFile(path); strings.Contains(string(data), "updateAttempt") {
		t.Fatalf("empty attempt written: %s", data)
	}
	s.UpdateAttempt = UpdateAttempt{Version: "0.5.0", At: time.Date(2026, 11, 3, 10, 0, 0, 0, time.UTC)}
	if err := SaveSettings(path, s); err != nil {
		t.Fatal(err)
	}
	got, err := LoadSettings(path)
	if err != nil || got.UpdateAttempt.Version != "0.5.0" || !got.UpdateAttempt.At.Equal(s.UpdateAttempt.At) {
		t.Fatalf("round trip %+v %v", got.UpdateAttempt, err)
	}
}

func TestInvocationsAreArgv(t *testing.T) {
	target := Target{Distro: "Ubuntu", Webmux: "~/.local/bin/webmux", Config: "~/my configs/webmux;$(id).toml"}
	serve := target.Serve("~/logs/web mux.log", []string{"WSLENV=USERPROFILE/p:WT_SESSION"})
	want := []string{"wsl.exe", "-d", "Ubuntu", "--cd", "~", "--exec", "./.local/bin/webmux", "serve", "--log-file", "~/logs/web mux.log", "--config", "~/my configs/webmux;$(id).toml"}
	if !slices.Equal(serve.Argv, want) {
		t.Fatalf("serve argv %q", serve.Argv)
	}
	if !slices.Equal(serve.Env, []string{"WEBMUX_LAUNCHER=windows-tray", "WSLENV=USERPROFILE/p:WT_SESSION:WEBMUX_LAUNCHER"}) {
		t.Fatalf("serve env %q", serve.Env)
	}
	if env := (Target{Webmux: "/usr/bin/webmux"}).Serve("/l", nil).Env; env[1] != "WSLENV=WEBMUX_LAUNCHER" {
		t.Fatalf("empty WSLENV: %q", env)
	}
	if MergeWSLENV("A:WEBMUX_LAUNCHER/u", "WEBMUX_LAUNCHER") != "A:WEBMUX_LAUNCHER/u" {
		t.Fatal("duplicate WSLENV entry")
	}
	bridge := (Target{Webmux: "/opt/web mux/webmux"}).Bridge()
	if !slices.Equal(bridge.Argv, []string{"wsl.exe", "--cd", "~", "--exec", "/opt/web mux/webmux", "bridge"}) {
		t.Fatalf("bridge argv %q", bridge.Argv)
	}
	if got := target.Systemctl("restart").Argv; !slices.Equal(got, []string{"wsl.exe", "-d", "Ubuntu", "--exec", "systemctl", "--user", "restart", "webmux.service"}) {
		t.Fatalf("systemctl argv %q", got)
	}
	for _, inv := range []Invocation{serve, bridge, target.Status(), target.ForceStop(), target.ConfigInit(), target.Probe(), target.Version(), target.Arch(), target.Wslpath(`C:\a b;$(id)`), target.Install("/mnt/c/a b")} {
		for _, arg := range inv.Argv {
			if arg == "sh" || arg == "bash" || arg == "-c" {
				t.Fatalf("shell in argv %q", inv.Argv)
			}
		}
	}
}

// scriptedRunner answers Run by the program after --exec.
type scriptedRunner map[string]Result

func (r scriptedRunner) Run(_ context.Context, argv, _ []string) (Result, error) {
	for i, arg := range argv {
		if arg == "--exec" && i+1 < len(argv) {
			key := strings.Join(argv[i+1:], " ")
			if res, ok := r[key]; ok {
				return res, nil
			}
		}
	}
	return Result{Code: 1}, nil
}
func (scriptedRunner) Start([]string, []string) (Process, error) { return nil, errors.New("unused") }

func TestModeSelection(t *testing.T) {
	target := Target{Webmux: "webmux"}
	systemd := scriptedRunner{"cat /proc/1/comm": {Stdout: []byte("systemd\n")}, "systemctl --user cat webmux.service": {}}
	noUnit := scriptedRunner{"cat /proc/1/comm": {Stdout: []byte("systemd\n")}}
	initd := scriptedRunner{"cat /proc/1/comm": {Stdout: []byte("init\n")}}
	tests := []struct {
		runner scriptedRunner
		mode   Mode
		want   Mode
		reason string
	}{
		{systemd, ModeAuto, ModeSystemd, ""},
		{initd, ModeAuto, ModeTray, ""},
		{noUnit, ModeAuto, ModeTray, ""},
		{systemd, ModeTray, ModeTray, ""},
		{initd, ModeSystemd, "", "发行版未启用 systemd"},
		{noUnit, ModeSystemd, "", "未安装 webmux 用户单元"},
	}
	for _, tt := range tests {
		d := DetectSystemd(context.Background(), tt.runner, target)
		got, err := Resolve(tt.mode, d)
		if got != tt.want || (tt.reason == "") != (err == nil) || (err != nil && (!errors.Is(err, ErrSystemdUnavailable) || !strings.Contains(err.Error(), tt.reason))) {
			t.Errorf("%v %s: %s %v", d, tt.mode, got, err)
		}
	}
	running := func(record bool, launcher string) contract.StatusV1 {
		return contract.StatusV1{State: contract.StateRunning, Record: record, Launcher: launcher}
	}
	for _, tt := range []struct {
		status     contract.StatusV1
		configured Mode
		mode       Mode
		managed    bool
	}{
		{running(true, "windows-tray"), ModeAuto, ModeTray, true},
		{running(true, "windows-tray"), ModeTray, ModeTray, true},
		{running(true, "windows-tray"), ModeSystemd, "", false},
		{running(true, "systemd"), ModeAuto, ModeSystemd, true},
		{running(true, "systemd"), ModeTray, "", false},
		{running(true, "manual"), ModeAuto, "", false},
		{running(false, "windows-tray"), ModeAuto, "", false},
	} {
		if mode, managed := ManagedMode(tt.status, tt.configured); mode != tt.mode || managed != tt.managed {
			t.Errorf("%+v under %s: %s %v", tt.status, tt.configured, mode, managed)
		}
	}
}

func TestPlanForward(t *testing.T) {
	ifaces := []WinInterface{
		{Name: "Loopback", Up: true, Loopback: true, Addrs: []net.IP{net.ParseIP("127.0.0.1")}},
		{Name: "Wi-Fi", Up: true, Addrs: []net.IP{net.ParseIP("192.168.1.20"), net.ParseIP("fe80::1"), net.ParseIP("169.254.1.1")}},
		{Name: "vEthernet (WSL (Hyper-V firewall))", Up: true, Addrs: []net.IP{net.ParseIP("172.29.0.1")}},
		{Name: "Tailscale", Up: true, Addrs: []net.IP{net.ParseIP("100.64.0.9")}},
		{Name: "Ethernet 2", Up: false, Addrs: []net.IP{net.ParseIP("10.0.0.5")}},
	}
	candidates := []Candidate{{"100.64.0.9", "Tailscale", "tailscale"}, {"192.168.1.20", "Wi-Fi", "lan"}}
	if got := WindowsCandidates(ifaces); !reflect.DeepEqual(got, candidates) {
		t.Fatalf("candidates %+v", got)
	}
	healthy := Snapshot{HasStatus: true, Reachable: true, Status: contract.StatusV1{State: contract.StateRunning, Health: contract.Health{Status: contract.HealthOK}, TokenConfigured: true, URLs: contract.URLs{Local: "http://127.0.0.1:7681", Remote: []contract.RemoteURL{}}, Config: contract.ConfigRef{Exists: true}}}
	plan := PlanForward("nat", healthy, ifaces)
	if !plan.Needed || plan.Blocked != "" || !reflect.DeepEqual(plan.Candidates, candidates) || plan.Headline != "可通过托盘转发" {
		t.Fatalf("NAT plan %+v", plan)
	}
	if plan := PlanForward("mirrored", healthy, ifaces); plan.Needed || plan.Headline != "无需转发（镜像网络模式）" || plan.Blocked == "" {
		t.Fatalf("mirrored plan %+v", plan)
	}
	remote := healthy
	remote.Status.URLs.Remote = []contract.RemoteURL{{URL: "http://100.64.0.5:7681"}}
	if plan := PlanForward("nat", remote, ifaces); plan.Needed {
		t.Fatalf("remote links still need forwarding: %+v", plan)
	}
	noToken := healthy
	noToken.Status.TokenConfigured = false
	noToken.Status.Config.Exists = false
	if plan := PlanForward("nat", noToken, ifaces); !strings.Contains(plan.Blocked, "请先设置 Token") || !plan.OfferInit {
		t.Fatalf("no token plan %+v", plan)
	}
	stopped := healthy
	stopped.Status.State = contract.StateStopped
	if plan := PlanForward("nat", stopped, ifaces); plan.Blocked == "" {
		t.Fatal("forwarding allowed while stopped")
	}
	unreachable := healthy
	unreachable.Reachable = false
	if plan := PlanForward("nat", unreachable, ifaces); plan.Blocked == "" {
		t.Fatal("forwarding allowed while Windows cannot reach WebMux")
	}
}

func TestLogRotatesAndRedacts(t *testing.T) {
	path := filepath.Join(t.TempDir(), "WebMux", "tray.log")
	log, err := OpenLog(path, 200)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = log.Close() }()
	secret := "tok_9f8e7d6c5b4a"
	release := log.Redact(secret)
	log.Printf("copied token %s to the clipboard", secret)
	release()
	if first, _ := os.ReadFile(path); strings.Contains(string(first), secret) || !strings.Contains(string(first), "[redacted]") {
		t.Fatalf("token not redacted: %q", first)
	}
	for range 20 {
		log.Printf("health check failed: connection refused")
	}
	current, _ := os.ReadFile(path)
	previous, _ := os.ReadFile(path + ".1")
	if len(current) > 200 || len(previous) == 0 || len(previous) > 200 {
		t.Fatalf("rotation: %d / %d bytes", len(current), len(previous))
	}
	entries, _ := os.ReadDir(filepath.Dir(path))
	if len(entries) != 2 {
		t.Fatalf("expected one rotated file, have %v", entries)
	}
	all := string(current) + string(previous)
	if strings.Contains(all, secret) {
		t.Fatal("token written to the tray log")
	}
}

// probeRunner answers the version probe of a current webmux.
type probeRunner struct{}

func (probeRunner) Run(context.Context, []string, []string) (Result, error) {
	return Result{Stderr: []byte("  -log-file string\n"), Code: 1}, nil
}
func (probeRunner) Start([]string, []string) (Process, error) { return nil, errors.New("unused") }

func TestTargetServiceRetargets(t *testing.T) {
	build := func(s Settings) *WSLService {
		return &WSLService{Runner: probeRunner{}, Target: TargetOf(s), LogFile: s.LogFile}
	}
	first := DefaultSettings()
	first.Distro = "Ubuntu"
	svc := NewTargetService(build, first)
	next := first
	next.Distro = "Debian"
	// While an instance runs in Ubuntu, its stop must still reach Ubuntu.
	svc.Retarget(next, false)
	if svc.Current().Target.Distro != "Ubuntu" {
		t.Fatal("deferred retarget applied early")
	}
	if _, err := svc.Check(context.Background()); err != nil || svc.Current().Target.Distro != "Debian" {
		t.Fatalf("next start still targets %s (%v)", svc.Current().Target.Distro, err)
	}
	logs := next
	logs.LogFile = "~/other.log"
	svc.Retarget(logs, false)
	if !svc.Settle() || svc.Current().LogFile != "~/other.log" || svc.Settle() {
		t.Fatal("pending path change not settled exactly once")
	}
	svc.Retarget(first, true)
	if svc.Current().Target.Distro != "Ubuntu" {
		t.Fatal("immediate retarget ignored")
	}
}
