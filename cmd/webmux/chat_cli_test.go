//go:build linux || darwin

package main

import (
	"bufio"
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/rainchan/webmux/internal/contract"
)

func TestChatCommandsDriveAHybridSession(t *testing.T) {
	t.Parallel()
	agent := fakeAgent(t)
	f := newFixture(t, "")
	f.start(t, nil, true, "--config", f.config)
	repo := gitRepo(t, f.dir)
	client := f.local(t)
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	if err := client.Do(ctx, http.MethodPost, "/api/repositories", contract.RegisterRepositoryRequest{Path: repo}, nil); err != nil {
		t.Fatal(err)
	}
	config := contract.LaunchConfigInput{Name: "Fake", Cmd: []string{agent}, Hybrid: true, Env: map[string]string{
		"FAKEAGENT_STORE": t.TempDir(), "FAKEAGENT_TURN_DELAY": "5ms", "FAKEAGENT_PERMISSION": "1",
	}}
	if err := client.Do(ctx, http.MethodPost, "/api/launch-configs", config, nil); err != nil {
		t.Fatal(err)
	}
	stdout, stderr, code := f.cli(t, "chat", "new", "--repo", repo, "--agent", "Fake", "--wait", "--timeout", "40s", "start")
	if code != 0 {
		t.Fatalf("chat new: %d %q %q", code, stdout, stderr)
	}
	sessions, _, _ := f.cli(t, "ls", "--json")
	var list []contract.SessionInfo
	if err := json.Unmarshal([]byte(sessions), &list); err != nil || len(list) != 1 {
		t.Fatalf("sessions %q: %v", sessions, err)
	}
	id := list[0].ID

	// Not a hybrid session: a clear refusal.
	plain := detachedID(t, f, "sleep", "30")
	if _, stderr, code := f.cli(t, "chat", "send", plain, "hi"); code != contract.ExitError || !strings.Contains(stderr, "not a hybrid session") {
		t.Fatalf("plain session: %d %q", code, stderr)
	}

	// Auto permission: --wait returns the reply of the prompt's own turn,
	// and watch shows it live without the earlier history.
	watch := f.command(nil, "chat", "watch", "--config", f.config, id, "--json")
	pipe, _ := watch.StdoutPipe()
	if err := watch.Start(); err != nil {
		t.Fatal(err)
	}
	lines := make(chan string, 100)
	go func() {
		scanner := bufio.NewScanner(pipe)
		scanner.Buffer(nil, 1<<20)
		for scanner.Scan() {
			lines <- scanner.Text()
		}
		close(lines)
	}()
	next := func() contract.ChatFrame {
		t.Helper()
		select {
		case line := <-lines:
			var frame contract.ChatFrame
			if err := json.Unmarshal([]byte(line), &frame); err != nil {
				t.Fatalf("watch line %q: %v", line, err)
			}
			return frame
		case <-time.After(10 * time.Second):
			t.Fatal("watch printed nothing")
		}
		return contract.ChatFrame{}
	}
	if frame := next(); frame.Type != contract.ChatFrameSnapshot {
		t.Fatalf("first watch frame %+v", frame)
	}
	stdout, stderr, code = f.cli(t, "chat", "send", id, "hello there", "--wait", "--timeout", "30s")
	if code != 0 || strings.TrimSpace(stdout) == "" {
		t.Fatalf("send --wait: %d %q %q", code, stdout, stderr)
	}
	var users []string
	for ended := false; !ended; {
		for _, ev := range next().Items {
			if ev.Message != nil && ev.Message.Role == "user" {
				users = append(users, ev.Message.Text)
			}
			ended = ended || ev.TurnEnd != nil && ev.TurnEnd.StopReason == "end_turn"
		}
	}
	_ = watch.Process.Kill()
	_ = watch.Wait()
	if strings.Join(users, "|") != "hello there" {
		t.Fatalf("watch saw user messages %q; want only the new one", users)
	}
	if _, _, code := f.cli(t, "chat", "cancel", id); code != 0 {
		t.Fatalf("cancel while idle: %d", code)
	}

	// Settings: a select takes one of its values; refusals name the cause.
	settings := chatState(t, f, id).Config
	if len(settings) == 0 || len(settings[0].Options) < 2 {
		t.Fatalf("the fake agent offers no select setting: %+v", settings)
	}
	setting, value := settings[0].ID, settings[0].Options[len(settings[0].Options)-1].Value
	if _, stderr, code := f.cli(t, "chat", "set", id, setting+"="+value); code != 0 {
		t.Fatalf("chat set: %d %q", code, stderr)
	}
	eventuallyCLI(t, "the new setting", func() bool { return chatState(t, f, id).Config[0].Value == value })
	if _, stderr, code := f.cli(t, "chat", "set", id, "nope=1"); code != contract.ExitError || !strings.Contains(stderr, setting) {
		t.Fatalf("unknown setting: %d %q", code, stderr)
	}
	if _, _, code := f.cli(t, "chat", "set", id, setting+"=nope"); code != contract.ExitError {
		t.Fatalf("unknown value: %d", code)
	}
	if _, _, code := f.cli(t, "chat", "set", id, "novalue"); code != contract.ExitUsage {
		t.Fatalf("malformed setting: %d", code)
	}

	// Ask: --wait stops at the request; status lists it; answer resolves it.
	if _, _, code := f.cli(t, "chat", "permission", id, "ask"); code != 0 {
		t.Fatalf("permission ask: %d", code)
	}
	_, stderr, code = f.cli(t, "chat", "send", id, "needs a tool", "--wait", "--timeout", "30s")
	if code != contract.ExitError || !strings.Contains(stderr, "chat answer") {
		t.Fatalf("send --wait at a request: %d %q", code, stderr)
	}
	state := chatState(t, f, id)
	if len(state.Requests) != 1 || len(state.Requests[0].Options) == 0 || !state.Turn.Running {
		t.Fatalf("status: %+v", state)
	}
	request := state.Requests[0]

	// A prompt sent during the turn is queued; the queue commands edit it.
	stdout, _, code = f.cli(t, "chat", "send", id, "later")
	var queued contract.ChatPromptResult
	if err := json.Unmarshal([]byte(stdout), &queued); err != nil || code != 0 || !queued.Queued || queued.ItemID == "" {
		t.Fatalf("queued send: %d %q", code, stdout)
	}
	if _, _, code := f.cli(t, "chat", "queue", "edit", id, queued.ItemID, "later, edited"); code != 0 {
		t.Fatalf("queue edit: %d", code)
	}
	if out, _, _ := f.cli(t, "chat", "queue", "ls", id); !strings.Contains(out, queued.ItemID+"\tlater, edited") {
		t.Fatalf("queue ls: %q", out)
	}
	if _, _, code := f.cli(t, "chat", "queue", "rm", id, queued.ItemID); code != 0 {
		t.Fatalf("queue rm: %d", code)
	}
	if _, _, code := f.cli(t, "chat", "queue", "rm", id, queued.ItemID); code != contract.ExitError {
		t.Fatalf("queue rm twice: %d", code)
	}

	if _, _, code := f.cli(t, "chat", "answer", id, request.RequestID, "--content", "{}"); code != contract.ExitUsage {
		t.Fatalf("--content on a permission: %d", code)
	}
	if _, _, code := f.cli(t, "chat", "answer", id, request.RequestID, "--option", request.Options[0].OptionID); code != 0 {
		t.Fatalf("answer: %d", code)
	}
	if _, _, code := f.cli(t, "chat", "answer", id, request.RequestID, "--cancel"); code != contract.ExitError {
		t.Fatalf("answering twice: %d", code)
	}
	eventuallyCLI(t, "the answered turn to end", func() bool { return !chatState(t, f, id).Turn.Running })

	// A queued prompt removed before it is sent ends --wait with 1.
	if _, _, code := f.cli(t, "chat", "send", id, "blocks on a tool"); code != 0 {
		t.Fatalf("send: %d", code)
	}
	eventuallyCLI(t, "the next request", func() bool { return len(chatState(t, f, id).Requests) == 1 })
	waiting := f.command(nil, "chat", "send", "--config", f.config, id, "never sent", "--wait", "--timeout", "30s")
	var waitErr strings.Builder
	waiting.Stderr = &waitErr
	if err := waiting.Start(); err != nil {
		t.Fatal(err)
	}
	eventuallyCLI(t, "the queued prompt", func() bool { return len(chatState(t, f, id).Queue.Items) == 1 })
	if _, _, code := f.cli(t, "chat", "queue", "clear", id); code != 0 {
		t.Fatalf("queue clear: %d", code)
	}
	_ = waiting.Wait()
	if waiting.ProcessState.ExitCode() != contract.ExitError || !strings.Contains(waitErr.String(), "removed") {
		t.Fatalf("waiting for a removed prompt: %d %q", waiting.ProcessState.ExitCode(), waitErr.String())
	}
	if _, _, code := f.cli(t, "chat", "cancel", id); code != 0 {
		t.Fatalf("cancel: %d", code)
	}
	eventuallyCLI(t, "the cancelled turn to end", func() bool { return !chatState(t, f, id).Turn.Running })

	// History: the record is active, so forget refuses; without a
	// terminal it needs --yes anyway.
	stdout, _, _ = f.cli(t, "chat", "ls", "--repo", repo, "--json")
	var records []contract.HybridRecord
	if err := json.Unmarshal([]byte(stdout), &records); err != nil || len(records) != 1 || records[0].ID != id {
		t.Fatalf("chat ls: %q %v", stdout, err)
	}
	if out, _, _ := f.cli(t, "chat", "ls", "--archived", "--json"); strings.TrimSpace(out) != "[]" {
		t.Fatalf("chat ls --archived: %q", out)
	}
	if _, _, code := f.cli(t, "chat", "forget", id, "--yes"); code != contract.ExitError {
		t.Fatalf("forget an active record: %d", code)
	}

	// Switch to the terminal and back.
	if _, stderr, code := f.cli(t, "chat", "switch", id, "--to", "terminal", "--force"); code != 0 {
		t.Fatalf("switch to terminal: %d %q", code, stderr)
	}
	if _, stderr, code := f.cli(t, "chat", "send", id, "hi"); code != contract.ExitError || !strings.Contains(stderr, "--to chat") {
		t.Fatalf("send on the terminal side: %d %q", code, stderr)
	}
	if _, stderr, code := f.cli(t, "chat", "switch", id, "--to", "chat"); code != 0 {
		t.Fatalf("switch to chat: %d %q", code, stderr)
	}
	if out, _, _ := f.cli(t, "chat", "status", id); !strings.Contains(out, "side") || !strings.Contains(out, "acp") {
		t.Fatalf("status after switching back: %q", out)
	}

	// Closing keeps the record for resume; forget removes it.
	if _, stderr, code := f.cli(t, "rm", id); code != 0 {
		t.Fatalf("rm: %d %q", code, stderr)
	}
	eventuallyCLI(t, "the closed record", func() bool {
		out, _, _ := f.cli(t, "chat", "ls", "--archived", "--json")
		return strings.Contains(out, id)
	})
	// Resume brings it back under the same ID on its chat side.
	stdout, stderr, code = f.cli(t, "chat", "resume", id, "--detach")
	if code != 0 || !strings.HasPrefix(stdout, id+"\t") {
		t.Fatalf("resume: %d %q %q", code, stdout, stderr)
	}
	if got := chatState(t, f, id); got.Side != contract.HybridSideACP {
		t.Fatalf("resumed side %q", got.Side)
	}
	if _, stderr, code := f.cli(t, "rm", id); code != 0 {
		t.Fatalf("rm after resume: %d %q", code, stderr)
	}
	eventuallyCLI(t, "the closed record again", func() bool {
		out, _, _ := f.cli(t, "chat", "ls", "--state", "closed", "--json")
		return strings.Contains(out, id)
	})
	if _, stderr, code := f.cli(t, "chat", "forget", id); code != contract.ExitUsage || !strings.Contains(stderr, "--yes") {
		t.Fatalf("forget without a terminal: %d %q", code, stderr)
	}
	if _, stderr, code := f.cli(t, "chat", "forget", id, "--yes"); code != 0 {
		t.Fatalf("forget: %d %q", code, stderr)
	}
	if out, _, _ := f.cli(t, "chat", "ls", "--json"); strings.Contains(out, id) {
		t.Fatalf("forgotten record still listed: %q", out)
	}
}

func chatState(t *testing.T, f *fixture, id string) contract.ChatState {
	t.Helper()
	stdout, stderr, code := f.cli(t, "chat", "status", id, "--json")
	var state contract.ChatState
	if err := json.Unmarshal([]byte(stdout), &state); err != nil || code != 0 {
		t.Fatalf("chat status: %d %q %q", code, stdout, stderr)
	}
	return state
}
