package tailscale

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func fakeCLI(t *testing.T, status, serve string) (string, string) {
	t.Helper()
	dir := t.TempDir()
	statusPath := filepath.Join(dir, "status.json")
	servePath := filepath.Join(dir, "serve.json")
	logPath := filepath.Join(dir, "calls.log")
	if err := os.WriteFile(statusPath, []byte(status), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(servePath, []byte(serve), 0600); err != nil {
		t.Fatal(err)
	}
	script := `#!/bin/sh
case "$*" in
"status --json --peers=false") cat "$FAKE_STATUS";;
"serve status --json") cat "$FAKE_SERVE";;
*) echo "$@" >> "$FAKE_LOG";;
esac
`
	path := filepath.Join(dir, "tailscale")
	if err := os.WriteFile(path, []byte(script), 0755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("FAKE_STATUS", statusPath)
	t.Setenv("FAKE_SERVE", servePath)
	t.Setenv("FAKE_LOG", logPath)
	return path, logPath
}

const runningStatus = `{"BackendState":"Running","Self":{"DNSName":"node.tailnet.ts.net.","Online":true},"CertDomains":["node.tailnet.ts.net"]}`
const emptyServe = `{"Web":{},"AllowFunnel":{}}`

func TestDetectLinux(t *testing.T) {
	cli, _ := fakeCLI(t, runningStatus, emptyServe)
	svc := New(Options{Command: cli, Provider: "linux", IsWSL: func() bool { return false }, CacheTTL: time.Millisecond})
	got := svc.Detect(context.Background())
	if got.Provider != ProviderLinux || got.BackendState != BackendRunning {
		t.Fatalf("provider/backend = %q/%q", got.Provider, got.BackendState)
	}
	if got.DNSName != "node.tailnet.ts.net" || !got.HTTPSReady {
		t.Fatalf("dns/https = %q/%v", got.DNSName, got.HTTPSReady)
	}
}

func TestDetectNeedsLoginAndMissingCert(t *testing.T) {
	cli, _ := fakeCLI(t, `{"BackendState":"NeedsLogin","Self":{"DNSName":"node.tailnet.ts.net."},"CertDomains":[]}`, emptyServe)
	svc := New(Options{Command: cli, Provider: "linux", IsWSL: func() bool { return false }})
	got := svc.Detect(context.Background())
	if got.BackendState != BackendNeedsLogin || got.HTTPSReady {
		t.Fatalf("backend/https = %q/%v", got.BackendState, got.HTTPSReady)
	}
}

func TestServeStatusParsesEntries(t *testing.T) {
	serve := `{"Web":{"node.tailnet.ts.net:8443":{"Handlers":{"/":{"Proxy":"http://127.0.0.1:7681"}}},"node.tailnet.ts.net:443":{"Handlers":{"/":{"Proxy":"http://127.0.0.1:9000"}}}},"AllowFunnel":{"node.tailnet.ts.net:443":true}}`
	cli, _ := fakeCLI(t, runningStatus, serve)
	svc := New(Options{Command: cli, Provider: "linux", IsWSL: func() bool { return false }})
	svc.Detect(context.Background())
	got, err := svc.ServeStatus(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	info, ok := got.Lookup("node.tailnet.ts.net", 8443)
	if !ok || info.Proxy != "http://127.0.0.1:7681" || info.Handlers != 1 || info.Public {
		t.Fatalf("8443 = %+v %v", info, ok)
	}
	public, ok := got.Lookup("node.tailnet.ts.net", 443)
	if !ok || !public.Public {
		t.Fatalf("443 = %+v %v", public, ok)
	}
}

func TestApplyAndRemoveUseArgv(t *testing.T) {
	cli, logPath := fakeCLI(t, runningStatus, emptyServe)
	svc := New(Options{Command: cli, Provider: "linux", IsWSL: func() bool { return false }})
	svc.Detect(context.Background())
	if err := svc.Apply(context.Background(), "funnel", 8443, 7681); err != nil {
		t.Fatal(err)
	}
	if err := svc.Inspect(context.Background(), "linux").Remove(context.Background(), 8443); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatal(err)
	}
	lines := strings.Split(strings.TrimSpace(string(data)), "\n")
	want := []string{
		"funnel --bg --https=8443 http://127.0.0.1:7681",
		"serve --https=8443 off",
	}
	if len(lines) != len(want) {
		t.Fatalf("calls = %q", lines)
	}
	for i := range want {
		if lines[i] != want[i] {
			t.Errorf("call %d = %q, want %q", i, lines[i], want[i])
		}
	}
}

func TestRemoveTreatsMissingHandlerAsSuccess(t *testing.T) {
	dir := t.TempDir()
	cli := filepath.Join(dir, "tailscale")
	script := `#!/bin/sh
case "$*" in
"status --json --peers=false") echo '` + runningStatus + `';;
"serve status --json") echo '` + emptyServe + `';;
"serve --https=8443 off") echo "handler does not exist" >&2; exit 1;;
*) exit 1;;
esac
`
	if err := os.WriteFile(cli, []byte(script), 0755); err != nil {
		t.Fatal(err)
	}
	svc := New(Options{Command: cli, Provider: "linux", IsWSL: func() bool { return false }})
	if err := svc.Inspect(context.Background(), "linux").Remove(context.Background(), 8443); err != nil {
		t.Fatalf("missing handler was not treated as success: %v", err)
	}
}

func TestRemovePropagatesCLIError(t *testing.T) {
	dir := t.TempDir()
	cli := filepath.Join(dir, "tailscale")
	script := `#!/bin/sh
case "$*" in
"status --json --peers=false") echo '` + runningStatus + `';;
"serve status --json") echo '` + emptyServe + `';;
"serve --https=8443 off") echo "access denied" >&2; exit 1;;
*) exit 1;;
esac
`
	if err := os.WriteFile(cli, []byte(script), 0755); err != nil {
		t.Fatal(err)
	}
	svc := New(Options{Command: cli, Provider: "linux", IsWSL: func() bool { return false }})
	err := svc.Inspect(context.Background(), "linux").Remove(context.Background(), 8443)
	var cliErr *CLIError
	if !errors.As(err, &cliErr) {
		t.Fatalf("error type = %T (%v), want *CLIError", err, err)
	}
}

func TestWindowsProviderSelection(t *testing.T) {
	cli, _ := fakeCLI(t, runningStatus, emptyServe)
	svc := New(Options{
		Command: cli, Provider: "windows", CacheTTL: time.Millisecond,
		IsWSL:          func() bool { return true },
		NetworkingMode: func(context.Context) string { return "mirrored" },
	})
	got := svc.Detect(context.Background())
	if got.Provider != ProviderWindows || got.DNSName != "node.tailnet.ts.net" {
		t.Fatalf("windows detection = %+v", got)
	}
	if got.WSL == nil || got.WSL.NetworkingMode != "mirrored" {
		t.Fatalf("wsl = %+v", got.WSL)
	}
}

func TestDetectMissingCLI(t *testing.T) {
	svc := New(Options{Provider: "linux", IsWSL: func() bool { return false }, LookPath: func(string) (string, error) {
		return "", ErrNotFound
	}})
	got := svc.Detect(context.Background())
	if got.Provider != ProviderNone || got.BackendState != BackendUnavailable {
		t.Fatalf("missing CLI = %+v", got)
	}
}

func TestDetectMalformedJSON(t *testing.T) {
	cli, _ := fakeCLI(t, "not json", "{}")
	svc := New(Options{Command: cli, Provider: "linux", IsWSL: func() bool { return false }})
	got := svc.Detect(context.Background())
	if got.BackendState != BackendUnknown || !strings.Contains(got.Error, "JSON") {
		t.Fatalf("malformed JSON = %+v", got)
	}
}

func TestDetectTimeout(t *testing.T) {
	dir := t.TempDir()
	cli := filepath.Join(dir, "tailscale")
	if err := os.WriteFile(cli, []byte("#!/bin/sh\nsleep 2\n"), 0755); err != nil {
		t.Fatal(err)
	}
	svc := New(Options{Command: cli, Provider: "linux", Timeout: 50 * time.Millisecond, IsWSL: func() bool { return false }})
	got := svc.Detect(context.Background())
	if got.BackendState != BackendUnknown || !strings.Contains(got.Error, "超时") {
		t.Fatalf("timeout = %+v", got)
	}
}

func TestConfigureRefreshAndLazyRequireCLI(t *testing.T) {
	cli, logPath := fakeCLI(t, runningStatus, emptyServe)
	now := time.Now()
	svc := New(Options{
		LookPath: func(name string) (string, error) {
			if name == "tailscale" {
				return cli, nil
			}
			return "", ErrNotFound
		},
		Now: func() time.Time { return now },
	})

	// ServeStatus without prior Detect resolves the CLI via Detect.
	if _, err := svc.ServeStatus(context.Background()); err != nil {
		t.Fatalf("ServeStatus without prior Detect: %v", err)
	}
	// Cached hit within CacheTTL.
	if got := svc.Detect(context.Background()); got.BackendState != BackendRunning {
		t.Fatalf("cached Detect = %+v", got)
	}

	// Configure with same provider is a no-op; empty resets to "auto".
	svc.Configure("auto")
	svc.Configure("linux")
	svc.Configure("")
	svc.Refresh()

	// Apply after Configure/Refresh resolves the CLI lazily through requireCLI.
	if err := svc.Apply(context.Background(), "serve", 443, 7681); err != nil {
		t.Fatalf("Apply after Refresh: %v", err)
	}
	data, err := os.ReadFile(logPath)
	if err != nil || !strings.Contains(string(data), "serve --bg --https=443 http://127.0.0.1:7681") {
		t.Fatalf("Apply log = %q, err = %v", string(data), err)
	}
}

func TestMissingCLIErrorsOnOperations(t *testing.T) {
	svc := New(Options{
		Provider: "linux",
		LookPath: func(string) (string, error) { return "", ErrNotFound },
	})
	if _, err := svc.ServeStatus(context.Background()); !errors.Is(err, ErrNotFound) {
		t.Fatalf("ServeStatus err = %v, want ErrNotFound", err)
	}
	if err := svc.Apply(context.Background(), "serve", 443, 7681); !errors.Is(err, ErrNotFound) {
		t.Fatalf("Apply err = %v, want ErrNotFound", err)
	}
	node := svc.Inspect(context.Background(), "linux")
	if err := node.Remove(context.Background(), 443); !errors.Is(err, ErrNotFound) {
		t.Fatalf("Node.Remove err = %v, want ErrNotFound", err)
	}
}

func TestChooseProviderMatrix(t *testing.T) {
	tests := []struct {
		name          string
		provider      string
		inWSL         bool
		native        string
		nativeErr     error
		nativeRunning bool
		windows       string
		windowsErr    error
		wantProvider  string
		wantCLI       string
	}{
		{
			name:         "windows missing",
			provider:     "windows",
			windowsErr:   ErrNotFound,
			wantProvider: ProviderWindows,
			wantCLI:      "",
		},
		{
			name:         "linux missing",
			provider:     "linux",
			nativeErr:    ErrNotFound,
			wantProvider: ProviderLinux,
			wantCLI:      "",
		},
		{
			name:          "wsl auto prefers running native",
			provider:      "auto",
			inWSL:         true,
			native:        "/usr/bin/tailscale",
			nativeRunning: true,
			windows:       "/mnt/c/tailscale.exe",
			wantProvider:  ProviderWSLNative,
			wantCLI:       "/usr/bin/tailscale",
		},
		{
			name:          "wsl auto falls back to windows when native stopped",
			provider:      "auto",
			inWSL:         true,
			native:        "/usr/bin/tailscale",
			nativeRunning: false,
			windows:       "/mnt/c/tailscale.exe",
			wantProvider:  ProviderWindows,
			wantCLI:       "/mnt/c/tailscale.exe",
		},
		{
			name:          "wsl auto uses stopped native when windows absent",
			provider:      "auto",
			inWSL:         true,
			native:        "/usr/bin/tailscale",
			nativeRunning: false,
			windowsErr:    ErrNotFound,
			wantProvider:  ProviderWSLNative,
			wantCLI:       "/usr/bin/tailscale",
		},
		{
			name:         "wsl auto uses windows when native absent",
			provider:     "auto",
			inWSL:        true,
			nativeErr:    ErrNotFound,
			windows:      "/mnt/c/tailscale.exe",
			wantProvider: ProviderWindows,
			wantCLI:      "/mnt/c/tailscale.exe",
		},
		{
			name:         "wsl auto none when both absent",
			provider:     "auto",
			inWSL:        true,
			nativeErr:    ErrNotFound,
			windowsErr:   ErrNotFound,
			wantProvider: ProviderNone,
			wantCLI:      "",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotProvider, gotCLI := choose(tt.provider, tt.inWSL, tt.native, tt.nativeErr, tt.nativeRunning, tt.windows, tt.windowsErr)
			if gotProvider != tt.wantProvider || gotCLI != tt.wantCLI {
				t.Fatalf("choose() = (%q, %q), want (%q, %q)", gotProvider, gotCLI, tt.wantProvider, tt.wantCLI)
			}
		})
	}
}

func TestPublicURLAndPureHelpers(t *testing.T) {
	if got := PublicURL("", 443); got != "" {
		t.Fatalf("PublicURL empty = %q", got)
	}
	if got := PublicURL("node.tailnet.ts.net.", 443); got != "https://node.tailnet.ts.net" {
		t.Fatalf("PublicURL 443 = %q", got)
	}
	if got := PublicURL("node.tailnet.ts.net", 8443); got != "https://node.tailnet.ts.net:8443" {
		t.Fatalf("PublicURL 8443 = %q", got)
	}

	cliErr := &CLIError{Action: "tailscale serve", Detail: "access denied"}
	if got := cliErr.Error(); got != "tailscale serve: access denied" {
		t.Fatalf("CLIError.Error() = %q", got)
	}

	reasons := []struct {
		stderr string
		err    error
		want   string
	}{
		{"", errors.New("context deadline exceeded"), "命令超时"},
		{"Access denied: operator check failed", errors.New("exit 1"), "权限不足"},
		{"Tailscale needs login", errors.New("exit 1"), "需要登录"},
		{"tailscaled is not running", errors.New("exit 1"), "后端未运行"},
		{"unexpected failure", errors.New("exit 1"), "命令执行失败"},
	}
	for _, r := range reasons {
		if got := safeCLIReason([]byte(r.stderr), r.err); got != r.want {
			t.Fatalf("safeCLIReason(%q, %v) = %q, want %q", r.stderr, r.err, got, r.want)
		}
	}

	backends := map[string]string{
		"Running":     BackendRunning,
		"Stopped":     BackendStopped,
		"NeedsLogin":  BackendNeedsLogin,
		"needs-login": BackendNeedsLogin,
		"":            BackendUnknown,
		"Starting":    BackendUnknown,
	}
	for in, want := range backends {
		if got := mapBackend(in); got != want {
			t.Fatalf("mapBackend(%q) = %q, want %q", in, got, want)
		}
	}

	if got := funnelCapability([]string{"https://tailscale.com/cap/funnel"}, nil); got != FunnelAvailable {
		t.Fatalf("funnelCapability slice = %q", got)
	}
	if got := funnelCapability(nil, []byte(`{"https://tailscale.com/cap/funnel-ports":[443]}`)); got != FunnelAvailable {
		t.Fatalf("funnelCapability capMap = %q", got)
	}
	if got := funnelCapability([]string{"other-cap"}, []byte(`{"other":true}`)); got != FunnelUnavailable {
		t.Fatalf("funnelCapability unavailable = %q", got)
	}
	if got := funnelCapability(nil, nil); got != FunnelUnknown {
		t.Fatalf("funnelCapability empty = %q", got)
	}
}
