package hybrid

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/rainchan/webmux/internal/session"
)

func (h *harness) claudeInput(side Side, pathDir string, args ...string) CreateInput {
	h.t.Helper()
	exe := agentPath(h.t, "claude")
	if pathDir == "" {
		pathDir = filepath.Dir(exe)
	}
	in := h.input("claude", side, "FAKEAGENT_PROFILE=", "CLAUDE_CONFIG_DIR="+h.t.TempDir(), "PATH="+pathDir+":/usr/bin:/bin")
	in.Cmd = append([]string{exe}, args...)
	return in
}

func TestClaudeRoundTrip(t *testing.T) {
	h := newHarness(t, 0)
	h.fast()
	in := h.claudeInput(SideTUI, "", "--dangerously-skip-permissions", "--model", "sonnet", "--effort", "high", "--bare")
	s := h.create(in)
	id := s.Info().ID
	waitFor(t, "session from process registration", func() bool { return s.Info().Hybrid.AgentSessionID != nil })
	h.waitReason(id, ReasonNoSessionID)
	tuiTurn(t, s, "one")
	h.waitEligible(id)
	sid := *s.Info().Hybrid.AgentSessionID

	result := h.mustSwitch(id, SideACP)
	if !slices.Equal(result.Unsupported, []string{"--bare"}) {
		t.Fatalf("unsupported %q", result.Unsupported)
	}
	st := h.chatState(id)
	if configValue(st.Config, "model") != "sonnet" || configValue(st.Config, "effort") != "high" || configValue(st.Config, "mode") != "bypassPermissions" {
		t.Fatalf("chat config %+v", st.Config)
	}
	for _, hidden := range []string{"login", "logout"} {
		if slices.ContainsFunc(st.Commands, func(c Command) bool { return c.Name == hidden }) {
			t.Fatalf("commands include %s: %+v", hidden, st.Commands)
		}
	}
	log, err := readEventsLog(h.store)
	if err != nil || !strings.Contains(log, "CLAUDE_CODE_EXECUTABLE="+agentPath(t, "claude")) {
		t.Fatalf("connector environment not logged: %v", err)
	}
	chatTurn(t, h, id, "two")

	h.mustSwitch(id, SideTUI)
	waitFor(t, "resumed transcript", func() bool { return strings.Contains(string(s.Buffer()), "> two") })
	tuiTurn(t, s, "three")
	h.waitEligible(id)
	if got := *s.Info().Hybrid.AgentSessionID; got != sid {
		t.Fatalf("resumed terminal names %s, want %s", got, sid)
	}
	h.mustSwitch(id, SideACP)
	if got := h.replayedUsers(id); !slices.Equal(got, []string{"one", "two", "three"}) {
		t.Fatalf("replay %q", got)
	}
	st = h.chatState(id)
	if configValue(st.Config, "model") != "sonnet" || configValue(st.Config, "effort") != "high" || configValue(st.Config, "mode") != "bypassPermissions" {
		t.Fatalf("config after cold load %+v", st.Config)
	}
	if strings.Contains(h.file(), in.Env["CLAUDE_CONFIG_DIR"]) || strings.Contains(h.file(), "secret-value") || strings.Contains(h.logs.String(), "secret-value") {
		t.Fatal("registry path or credentials leaked into records or logs")
	}
}

func TestClaudeNodeLauncher(t *testing.T) {
	h := newHarness(t, 0)
	h.fast()
	in := h.claudeInput(SideTUI, "")
	native := in.Cmd[0]
	script := filepath.Join(t.TempDir(), "claude")
	if err := os.WriteFile(script, []byte("#!"+agentPath(t, "node")+"\n"), 0o700); err != nil {
		t.Fatal(err)
	}
	in.Cmd = []string{script}
	in.Env["FAKEAGENT_NATIVE"] = native
	s := h.create(in)
	id := s.Info().ID
	waitFor(t, "child process registration", func() bool { return s.Info().Hybrid.AgentSessionID != nil })
	tuiTurn(t, s, "through node")
	h.waitEligible(id)
	if !actorState(h, id, func(a *actor) bool {
		return a.registry.processOK && a.registry.process.PID != s.Identity().PID && a.registry.process.Executable == "claude"
	}) {
		t.Fatal("registration did not select the native child below node")
	}
	h.mustSwitch(id, SideACP)
	if got := h.replayedUsers(id); !slices.Equal(got, []string{"through node"}) {
		t.Fatalf("replay %q", got)
	}
}

func TestClaudeVersionedNativeRoot(t *testing.T) {
	h := newHarness(t, 0)
	cmd, err := h.svc.agents.Parse([]string{"claude"})
	if err != nil {
		t.Fatal(err)
	}
	a := &actor{cmd: cmd}
	if !a.agentIsRoot(session.Identity{PID: 42, Executable: "2.1.316"}) {
		t.Fatal("versioned native Claude is not accepted as the terminal root")
	}
}

func TestClaudeSessionChangeAndFirstMessage(t *testing.T) {
	h := newHarness(t, 0)
	h.fast()
	s := h.create(h.claudeInput(SideTUI, ""))
	id := s.Info().ID
	waitFor(t, "initial registration", func() bool { return s.Info().Hybrid.AgentSessionID != nil })
	old := *s.Info().Hybrid.AgentSessionID
	tuiTurn(t, s, "one")
	h.waitEligible(id)
	typeLine(t, s, "/new")
	waitFor(t, "registration follows /new", func() bool {
		current := s.Info().Hybrid.AgentSessionID
		return current != nil && *current != old
	})
	h.waitReason(id, ReasonNoSessionID)
	tuiTurn(t, s, "two")
	h.waitEligible(id)
}

func TestClaudeCandidateFallbackAndChoice(t *testing.T) {
	h := newHarness(t, 0)
	h.fast()
	in := h.claudeInput(SideTUI, "")
	s := h.create(in)
	id := s.Info().ID
	waitFor(t, "initial registration", func() bool { return s.Info().Hybrid.AgentSessionID != nil })
	tuiTurn(t, s, "one")
	h.waitEligible(id)
	sid := *s.Info().Hybrid.AgentSessionID

	otherInput := h.claudeInput(SideACP, "")
	otherInput.Cwd = in.Cwd
	otherID := h.create(otherInput).Info().ID
	chatTurn(t, h, otherID, "other")
	rootPID := s.Identity().PID
	record := filepath.Join(in.Env["CLAUDE_CONFIG_DIR"], "sessions", fmt.Sprintf("%d.json", rootPID))
	if err := os.Remove(record); err != nil {
		t.Fatal(err)
	}
	waitFor(t, "candidate fallback after invalid registration", func() bool {
		return actorState(h, id, func(a *actor) bool {
			return a.registry.unknown && a.probe.key.sid == "" && a.chat.Eligibility.Available
		})
	})
	h.waitEligible(id)
	_, err := h.switchTo(id, SideACP)
	var ambiguous *SessionAmbiguousError
	if !errors.As(err, &ambiguous) || len(ambiguous.Candidates) != 2 {
		t.Fatalf("switch with two candidates: %+v %v", ambiguous, err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), testTimeout)
	defer cancel()
	if _, err := h.svc.Switch(ctx, id, SwitchInput{ClientRequestID: rid(), To: SideACP, AgentSessionID: sid}); err != nil {
		t.Fatalf("switch with selected candidate: %v", err)
	}
	if got := h.record(id).AgentSessionID; got != sid {
		t.Fatalf("selected candidate = %s, want %s", got, sid)
	}
}

func TestClaudeExternalWriter(t *testing.T) {
	h := newHarness(t, 0)
	h.fast()
	in := h.claudeInput(SideTUI, "")
	s := h.create(in)
	id := s.Info().ID
	waitFor(t, "initial registration", func() bool { return s.Info().Hybrid.AgentSessionID != nil })
	tuiTurn(t, s, "one")
	h.waitEligible(id)
	sid := *s.Info().Hybrid.AgentSessionID
	h.closeSession(id)
	other, err := h.manager.Create(session.CreateInput{Cwd: in.Cwd, Cmd: []string{in.Cmd[0], "--resume", sid}, Env: in.Env})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = h.manager.Close(other.Info().ID) })
	waitFor(t, "outside client registration", func() bool { return strings.Contains(string(other.Buffer()), "> one") })
	ctx, cancel := context.WithTimeout(context.Background(), testTimeout)
	defer cancel()
	if _, err := h.svc.Resume(ctx, id, ResumeInput{ClientRequestID: rid(), Side: SideACP, Env: in.Env}); !errors.Is(err, ErrExternalWriter) {
		t.Fatalf("resume while held: %v", err)
	}
	if rec := h.record(id); rec.State != StateClosed {
		t.Fatalf("record after refused resume %+v", rec)
	}
}

func TestClaudeConnectorMissing(t *testing.T) {
	h := newHarness(t, 0)
	h.fast()
	dir := t.TempDir()
	if err := os.Symlink(agentPath(t, "claude"), filepath.Join(dir, "claude")); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), testTimeout)
	defer cancel()
	_, err := h.svc.Create(ctx, h.claudeInput(SideACP, dir))
	var missing *ConnectorMissingError
	if !errors.As(err, &missing) || missing.Connector.Executable != "claude-agent-acp" {
		t.Fatalf("chat-first create: %v", err)
	}
	if records, _ := h.svc.Records(false); len(records) != 0 || len(h.manager.List()) != 0 {
		t.Fatalf("refused create left records %v sessions %v", records, h.manager.List())
	}
	s := h.create(h.claudeInput(SideTUI, dir))
	id := s.Info().ID
	h.waitReason(id, ReasonConnectorMissing)
	if err := os.Symlink(agentPath(t, "claude-agent-acp"), filepath.Join(dir, "claude-agent-acp")); err != nil {
		t.Fatal(err)
	}
	tuiTurn(t, s, "one")
	h.waitEligible(id)
	h.mustSwitch(id, SideACP)
}

func TestClaudeAttachments(t *testing.T) {
	h := newHarness(t, 0)
	in := h.claudeInput(SideACP, "")
	in.Env["FAKEAGENT_PROMPT_IMAGE"] = "1"
	in.Env["FAKEAGENT_EMBEDDED_CONTEXT"] = "1"
	id := h.create(in).Info().ID
	st := h.follow(id)
	image := upload(t, "shot.png", pngBytes())
	text := upload(t, "note.txt", []byte("hello"))
	ctx, cancel := context.WithTimeout(context.Background(), testTimeout)
	defer cancel()
	_, err := h.svc.Prompt(ctx, id, PromptInput{ClientRequestID: rid(), Text: "look", Attachments: []Attachment{{Path: image}, {Path: text}}})
	if err != nil {
		t.Fatal(err)
	}
	st.until("turn end", hasKind(EventTurnEnd))
	var imageSent, resourceSent bool
	for _, block := range userBlocks(t, h.store) {
		imageSent = imageSent || block.Type == "image"
		resourceSent = resourceSent || block.Type == "resource"
	}
	if !imageSent || !resourceSent {
		t.Fatalf("Claude prompt blocks: image=%v resource=%v", imageSent, resourceSent)
	}
}

func TestClaudeWakeReappliesConfig(t *testing.T) {
	h := sleepyHarness(t, 1)
	in := h.claudeInput(SideACP, "", "--model", "sonnet", "--effort", "high", "--permission-mode", "plan")
	id := h.create(in).Info().ID
	chatTurn(t, h, id, "hello")
	waitFor(t, "dormant", func() bool { return h.dormant(id) })
	st := h.follow(id)
	st.until("resumed marker", hasMarker(MarkerResumed))
	waitFor(t, "awake with restored config", func() bool {
		config := h.chatState(id).Config
		return !h.dormant(id) && configValue(config, "model") == "sonnet" && configValue(config, "effort") == "high" && configValue(config, "mode") == "plan"
	})
}

func TestClaudeSteeringAndModeFallback(t *testing.T) {
	h := newHarness(t, 0)
	in := h.claudeInput(SideACP, "", "--permission-mode", "auto")
	in.Env["FAKEAGENT_MODE_FALLBACK"] = "1"
	in.Env["FAKEAGENT_DELAY"] = "_session/steering=300ms"
	id := h.create(in).Info().ID
	st := h.follow(id)
	if !st.state.Steering {
		t.Fatal("Claude chat does not offer steering")
	}
	if got := configValue(st.state.Config, "mode"); got != "acceptEdits" {
		t.Fatalf("requested auto remained displayed instead of actual fallback: %q", got)
	}
	prompt(t, h, id, "busy")
	if _, err := h.svc.Prompt(ctxT(t), id, PromptInput{ClientRequestID: rid(), Text: "nudge", Mode: ModeSteer}); err != nil {
		t.Fatalf("steer: %v", err)
	}
	st.until("nudge as the next turn", func(events []Event) bool {
		return strings.Join(userTexts(events), ",") == "busy,nudge" && count(EventTurnEnd, 2)(events)
	})
}
