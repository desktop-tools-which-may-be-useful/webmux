package voice

import (
	"context"
	"os/exec"
	"reflect"
	"strings"
	"testing"
	"time"
)

func helperService(t *testing.T, script string, extraEnv ...string) *Service {
	t.Helper()
	return New(Options{
		Cmd:           []string{"/bin/sh", "-c", script},
		Env:           extraEnv,
		ReadyDelay:    20 * time.Millisecond,
		FinishTimeout: 200 * time.Millisecond,
	})
}

func collectEvents(ch <-chan Event) []Event {
	var out []Event
	for ev := range ch {
		out = append(out, ev)
	}
	return out
}

func TestNDJSONStreamingProtocol(t *testing.T) {
	script := `printf '{"type":"ready"}\n'; bytes=$(wc -c | tr -d ' '); printf '{"type":"partial","text":"bytes:%s"}\n' "$bytes"; printf '{"type":"final","text":"%s:%s"}\n' "$CUSTOM_VOICE_TAG" "$bytes"`
	svc := helperService(t, script, "CUSTOM_VOICE_TAG=qwen")
	if !svc.Enabled() {
		t.Fatal("expected service enabled")
	}
	sess, err := svc.Start(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	defer sess.Close()

	first := <-sess.Events()
	if first.Type != EventReady {
		t.Fatalf("first event = %+v, want ready", first)
	}

	if err := sess.WriteAudio(make([]byte, 640)); err != nil {
		t.Fatal(err)
	}
	if err := sess.WriteAudio(make([]byte, 640)); err != nil {
		t.Fatal(err)
	}
	sess.StopInput(2 * time.Second)

	rest := collectEvents(sess.Events())
	want := []Event{
		{Type: EventPartial, Text: "bytes:1280"},
		{Type: EventFinal, Text: "qwen:1280"},
	}
	if !reflect.DeepEqual(rest, want) {
		t.Fatalf("events = %+v, want %+v", rest, want)
	}
}

func TestPlainTextFallbackProtocol(t *testing.T) {
	script := `bytes=$(wc -c | tr -d ' '); printf 'plain result (%s bytes)\n' "$bytes"`
	svc := helperService(t, script)
	sess, err := svc.Start(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	defer sess.Close()

	first := <-sess.Events()
	if first.Type != EventReady {
		t.Fatalf("first event = %+v, want implicit ready", first)
	}
	if err := sess.WriteAudio(make([]byte, 320)); err != nil {
		t.Fatal(err)
	}
	sess.StopInput(2 * time.Second)

	rest := collectEvents(sess.Events())
	want := []Event{
		{Type: EventFinal, Text: "plain result (320 bytes)"},
	}
	if !reflect.DeepEqual(rest, want) {
		t.Fatalf("events = %+v, want %+v", rest, want)
	}
}

func TestErrorJSONAndStderrExtraction(t *testing.T) {
	t.Run("json error event", func(t *testing.T) {
		svc := helperService(t, `printf '{"type":"error","error":"upstream quota exceeded"}\n'; exit 1`)
		sess, err := svc.Start(context.Background())
		if err != nil {
			t.Fatal(err)
		}
		defer sess.Close()
		events := collectEvents(sess.Events())
		if len(events) == 0 || events[len(events)-1].Type != EventError || events[len(events)-1].Error != "upstream quota exceeded" {
			t.Fatalf("events = %+v", events)
		}
	})
	t.Run("stderr non-zero exit", func(t *testing.T) {
		svc := helperService(t, `printf 'fatal: api key invalid\n' >&2; exit 2`)
		sess, err := svc.Start(context.Background())
		if err != nil {
			t.Fatal(err)
		}
		defer sess.Close()
		events := collectEvents(sess.Events())
		if len(events) == 0 || events[len(events)-1].Type != EventError || !strings.Contains(events[len(events)-1].Error, "api key invalid") {
			t.Fatalf("events = %+v", events)
		}
	})
}

func TestHungSubprocessTerminatedOnTimeoutAndClose(t *testing.T) {
	svc := helperService(t, `printf '{"type":"ready"}\n'; exec sleep 30`)
	sess, err := svc.Start(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	first := <-sess.Events()
	if first.Type != EventReady {
		t.Fatalf("first event = %+v", first)
	}
	sess.StopInput(50 * time.Millisecond)
	done := make(chan struct{})
	go func() {
		for range sess.Events() {
		}
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("hung subprocess did not terminate after StopInput timeout")
	}
	sess.Close()
}

func TestExtractScriptIsReusedAndRunnable(t *testing.T) {
	dir := t.TempDir()
	first, err := ExtractScript(dir)
	if err != nil {
		t.Fatal(err)
	}
	second, err := ExtractScript(dir)
	if err != nil || second != first {
		t.Fatalf("second extract = %q, %v; want %q", second, err, first)
	}
	if _, err := exec.LookPath("node"); err != nil {
		t.Skip("node is not installed")
	}
	// An unknown provider must fail fast with the usage text, proving the
	// extracted file is a complete program, not a truncated copy.
	out, err := exec.Command("node", first, "nope").CombinedOutput()
	if err == nil || !strings.Contains(string(out), "unknown provider: nope") {
		t.Fatalf("output = %q, err = %v", out, err)
	}
}
