package api

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"sync"
	"testing"

	"github.com/rainchan/webmux/internal/config"
	"github.com/rainchan/webmux/internal/contract"
	"github.com/rainchan/webmux/internal/session"
	"github.com/rainchan/webmux/internal/voicesettings"
)

type voiceControlFixture struct {
	mu    sync.Mutex
	view  contract.VoiceSettings
	err   error
	calls int
	saved contract.VoiceSettingsUpdate
}

func newVoiceControlFixture() *voiceControlFixture {
	return &voiceControlFixture{view: contract.VoiceSettings{
		Cmd: []string{}, Env: map[string]string{},
		Source: "config", StorageAvailable: true,
	}}
}

func (c *voiceControlFixture) Settings() contract.VoiceSettings {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.view
}

func (c *voiceControlFixture) Update(in contract.VoiceSettingsUpdate) (contract.VoiceSettings, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.calls++
	if c.err != nil {
		return contract.VoiceSettings{}, c.err
	}
	c.saved = in
	c.view.Cmd, c.view.Env = in.Cmd, in.Env
	c.view.Enabled, c.view.Source = len(in.Cmd) > 0, "web"
	return c.view, nil
}

func (c *voiceControlFixture) Reset() (contract.VoiceSettings, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.calls++
	if c.err != nil {
		return contract.VoiceSettings{}, c.err
	}
	c.view = newVoiceControlFixture().view
	return c.view, nil
}

func voiceAPIHandler(t *testing.T, control VoiceControl) http.Handler {
	t.Helper()
	m := session.NewManager(session.Options{DefaultCwd: t.TempDir()})
	t.Cleanup(func() { m.Shutdown(context.Background()) })
	return New(testDeps(t, Deps{Config: config.Config{Token: "login-secret"}, Manager: m, VoiceSettings: control}))
}

// The recognizer's environment holds credentials: every route needs the
// login token and a same-origin browser, and rejects what it does not know.
func TestVoiceSettingsEndpointsSecurityAndStrictBodies(t *testing.T) {
	control := newVoiceControlFixture()
	h := voiceAPIHandler(t, control)
	for _, endpoint := range []struct{ method, path, body string }{
		{"GET", "/api/voice/settings", ""},
		{"PUT", "/api/voice/settings", `{"cmd":["asr"],"env":{"KEY":"v"}}`},
		{"DELETE", "/api/voice/settings", ""},
	} {
		for _, auth := range []struct {
			token, origin string
			status        int
		}{
			{"", "", 401}, {"wrong", "", 401}, {"login-secret", "https://evil.example", 403},
		} {
			w := tunnelAPIRequest(h, endpoint.method, endpoint.path, endpoint.body, auth.token, auth.origin)
			if w.Code != auth.status {
				t.Fatalf("%s %s: %d", endpoint.method, endpoint.path, w.Code)
			}
		}
	}
	if control.calls != 0 {
		t.Fatal("unauthorized request reached the control boundary")
	}
	for _, body := range []string{
		`{"cmd":[],"env":{},"extra":1}`,
		`{"cmd":"asr","env":{}}`,
		`{"cmd":[],"env":{}} {}`,
	} {
		if w := tunnelAPIRequest(h, "PUT", "/api/voice/settings", body, "login-secret", ""); w.Code != 400 {
			t.Fatalf("body %s: %d", body, w.Code)
		}
	}
	if control.calls != 0 {
		t.Fatal("a malformed body reached the control boundary")
	}
	if w := tunnelAPIRequest(voiceAPIHandler(t, nil), "GET", "/api/voice/settings", "", "login-secret", ""); w.Code != 503 {
		t.Fatalf("without a control: %d", w.Code)
	}
}

func TestVoiceSettingsUpdateReachesControlAndMapsErrors(t *testing.T) {
	control := newVoiceControlFixture()
	h := voiceAPIHandler(t, control)
	w := tunnelAPIRequest(h, "PUT", "/api/voice/settings", `{"cmd":["node","asr.js"],"env":{"ASR_KEY":"secret"}}`, "login-secret", "")
	if w.Code != 200 || fmt.Sprint(control.saved.Cmd) != "[node asr.js]" || control.saved.Env["ASR_KEY"] != "secret" {
		t.Fatalf("update %d %s saved %+v", w.Code, w.Body.String(), control.saved)
	}
	for _, row := range []struct {
		err  error
		want int
	}{
		{fmt.Errorf("%w: voice.cmd[0] must not be empty", voicesettings.ErrInvalid), 400},
		{errors.Join(voicesettings.ErrUnavailable, errors.New("disk")), 503},
		{errors.New("boom"), 500},
	} {
		control.err = row.err
		if w := tunnelAPIRequest(h, "PUT", "/api/voice/settings", `{"cmd":[],"env":{}}`, "login-secret", ""); w.Code != row.want {
			t.Fatalf("%v: %d, want %d", row.err, w.Code, row.want)
		}
	}
}

func TestWireSnapshotsVoiceSettings(t *testing.T) {
	rec := &wireRecorder{t: t, norm: newWireNormalizer(), seen: map[string]bool{}}
	control := newVoiceControlFixture()
	h := voiceAPIHandler(t, control)
	for _, row := range []struct {
		name, method, path, body, token, origin string
		err                                     error
	}{
		{"voice-settings-get", "GET", "/api/voice/settings", "", "login-secret", "", nil},
		{"voice-settings-put", "PUT", "/api/voice/settings", `{"cmd":["node","/opt/asr.js"],"env":{"DASHSCOPE_API_KEY":"sk-test"}}`, "login-secret", "", nil},
		{"voice-settings-reset", "DELETE", "/api/voice/settings", "", "login-secret", "", nil},
		{"error-400-voice-settings", "PUT", "/api/voice/settings", `{"cmd":["   "],"env":{}}`, "login-secret", "", fmt.Errorf("%w: voice.cmd[0] must not be empty", voicesettings.ErrInvalid)},
		{"error-503-voice-settings", "PUT", "/api/voice/settings", `{"cmd":[],"env":{}}`, "login-secret", "", voicesettings.ErrUnavailable},
		{"error-401-voice-settings", "GET", "/api/voice/settings", "", "", "", nil},
	} {
		control.err = row.err
		w := tunnelAPIRequest(h, row.method, row.path, row.body, row.token, row.origin)
		rec.record(row.name, w.Code, w.Body.Bytes())
	}
}
