package main

import (
	"context"
	"errors"
	"reflect"
	"testing"
	"time"

	"github.com/rainchan/webmux/internal/config"
	"github.com/rainchan/webmux/internal/contract"
	"github.com/rainchan/webmux/internal/voice"
	"github.com/rainchan/webmux/internal/voicesettings"
)

func newTestVoiceController(t *testing.T, toml config.VoiceConfig) (*voiceController, *voice.Service) {
	t.Helper()
	dir := t.TempDir()
	store := voicesettings.Open(dir, toml)
	t.Cleanup(store.Close)
	svc := voice.New(voice.Options{Cmd: toml.Cmd, Env: toml.Env})
	return newVoiceController(store, svc, config.Config{StateDir: dir, Voice: toml}), svc
}

// recognize runs one recording against the service's current recognizer and
// returns the text it reported.
func recognize(t *testing.T, svc *voice.Service) string {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	sess, err := svc.Start(ctx)
	if err != nil {
		t.Fatalf("start: %v", err)
	}
	defer sess.Close()
	sess.StopInput(5 * time.Second)
	for ev := range sess.Events() {
		if ev.Type == voice.EventFinal {
			return ev.Text
		}
		if ev.Type == voice.EventError {
			t.Fatalf("recognizer error: %s", ev.Error)
		}
	}
	t.Fatal("no final result")
	return ""
}

func shellASR(say string) []string {
	return []string{"sh", "-c", `cat >/dev/null; echo "` + say + `"`}
}

// Saving applies at once: the next recording uses the new command and
// environment, and a reset goes back to config.toml, without a restart.
func TestVoiceControllerUpdateAndResetSwitchTheRunningRecognizer(t *testing.T) {
	toml := config.VoiceConfig{Cmd: shellASR("from-toml:$A"), Env: []string{"A=1"}}
	c, svc := newTestVoiceController(t, toml)

	view := c.Settings()
	if view.Source != "config" || !view.Enabled || !reflect.DeepEqual(view.Cmd, toml.Cmd) || view.Env["A"] != "1" || !view.StorageAvailable {
		t.Fatalf("initial view %+v", view)
	}
	if got := recognize(t, svc); got != "from-toml:1" {
		t.Fatalf("toml recognizer said %q", got)
	}

	view, err := c.Update(contract.VoiceSettingsUpdate{Cmd: shellASR("from-web:$B$C"), Env: map[string]string{"B": "2", "C": "3"}})
	if err != nil {
		t.Fatal(err)
	}
	if view.Source != "web" || !view.Enabled || len(view.Env) != 2 || view.Env["C"] != "3" {
		t.Fatalf("updated view %+v", view)
	}
	if got := recognize(t, svc); got != "from-web:23" {
		t.Fatalf("web recognizer said %q", got)
	}

	view, err = c.Update(contract.VoiceSettingsUpdate{})
	if err != nil || view.Enabled || svc.Enabled() {
		t.Fatalf("empty update: %+v %v enabled=%v", view, err, svc.Enabled())
	}

	view, err = c.Reset()
	if err != nil || view.Source != "config" || !view.Enabled {
		t.Fatalf("reset: %+v %v", view, err)
	}
	if got := recognize(t, svc); got != "from-toml:1" {
		t.Fatalf("after reset the recognizer said %q", got)
	}
}

func TestVoiceControllerRejectsInvalidUpdatesBeforeApplying(t *testing.T) {
	c, svc := newTestVoiceController(t, config.VoiceConfig{Cmd: shellASR("from-toml")})
	for _, bad := range []contract.VoiceSettingsUpdate{
		{Cmd: []string{"   "}},
		{Cmd: []string{"asr"}, Env: map[string]string{"WEBMUX_SESSION_ID": "x"}},
	} {
		if _, err := c.Update(bad); !errors.Is(err, voicesettings.ErrInvalid) {
			t.Fatalf("Update(%+v) = %v", bad, err)
		}
	}
	if got := recognize(t, svc); got != "from-toml" {
		t.Fatalf("a rejected update changed the recognizer: %q", got)
	}
	if c.Settings().Source != "config" {
		t.Fatal("a rejected update was stored")
	}
}
