package voicesettings

import (
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/rainchan/webmux/internal/config"
)

var tomlDefaults = config.VoiceConfig{Cmd: []string{"toml-asr"}, Env: []string{"DASHSCOPE_API_KEY=toml-key"}}

const fileName = "voice-settings.json"

func TestSaveOverridesTOMLAcrossRestartAndResetRestores(t *testing.T) {
	dir := t.TempDir()
	s := Open(dir, tomlDefaults)
	if got, web := s.Effective(); !reflect.DeepEqual(got, tomlDefaults) || web {
		t.Fatalf("fresh store = %+v web=%v", got, web)
	}
	web := config.VoiceConfig{Cmd: []string{"node", "/opt/asr.js", "--fast"}, Env: []string{"ASR_KEY=web-key"}}
	if err := s.Save(web); err != nil {
		t.Fatal(err)
	}
	s.Close()
	info, err := os.Stat(filepath.Join(dir, fileName))
	if err != nil || info.Mode().Perm() != 0600 {
		t.Fatalf("settings file %v %v", info, err)
	}

	s = Open(dir, tomlDefaults)
	defer s.Close()
	if got, fromWeb := s.Effective(); !reflect.DeepEqual(got, web) || !fromWeb {
		t.Fatalf("reopened = %+v web=%v; want %+v from web", got, fromWeb, web)
	}
	reset, err := s.Reset()
	if err != nil || !reflect.DeepEqual(reset, tomlDefaults) {
		t.Fatalf("reset = %+v %v", reset, err)
	}
	if _, err := os.Stat(filepath.Join(dir, fileName)); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("reset left the override: %v", err)
	}
}

// An empty update is a valid way to turn recognition off in the web even
// when config.toml turns it on.
func TestEmptyUpdateDisablesRecognition(t *testing.T) {
	s := Open(t.TempDir(), tomlDefaults)
	defer s.Close()
	if err := s.Save(config.VoiceConfig{}); err != nil {
		t.Fatal(err)
	}
	if got, web := s.Effective(); got.Enabled() || !web {
		t.Fatalf("effective %+v web=%v", got, web)
	}
}

func TestInvalidUpdatesAreRejectedWithoutChangingTheStore(t *testing.T) {
	s := Open(t.TempDir(), tomlDefaults)
	defer s.Close()
	for _, bad := range []config.VoiceConfig{
		{Cmd: []string{"   "}},
		{Cmd: []string{"asr"}, Env: []string{"BAD-NAME=1"}},
		{Cmd: []string{"asr"}, Env: []string{"WEBMUX_SESSION_ID=1"}},
	} {
		if err := s.Save(bad); !errors.Is(err, ErrInvalid) {
			t.Fatalf("Save(%+v) = %v, want ErrInvalid", bad, err)
		}
	}
	if got, web := s.Effective(); !reflect.DeepEqual(got, tomlDefaults) || web {
		t.Fatalf("a rejected update changed the store: %+v web=%v", got, web)
	}
}

// A damaged file must not leave a recognizer running with credentials nobody
// can inspect, and must not be overwritten.
func TestDamagedFileFailsClosedAndStaysUntouched(t *testing.T) {
	dir := t.TempDir()
	const damaged = `{"v":1,"cmd":["asr"]`
	path := filepath.Join(dir, fileName)
	if err := os.WriteFile(path, []byte(damaged), 0600); err != nil {
		t.Fatal(err)
	}
	s := Open(dir, tomlDefaults)
	defer s.Close()
	if got, _ := s.Effective(); got.Enabled() {
		t.Fatalf("damaged store runs %+v", got)
	}
	if err := s.Unavailable(); !errors.Is(err, ErrUnavailable) {
		t.Fatalf("Unavailable() = %v", err)
	}
	if err := s.Save(config.VoiceConfig{Cmd: []string{"asr"}}); !errors.Is(err, ErrUnavailable) {
		t.Fatalf("Save on a damaged store = %v", err)
	}
	if b, _ := os.ReadFile(path); string(b) != damaged {
		t.Fatalf("damaged file rewritten: %s", b)
	}
}
