// Package hybrid runs hybrid sessions: one agent conversation that moves
// between the agent's terminal UI (a PTY run of a session shell) and an
// ACP chat (a sidecar process of the same shell), under one WebMux session
// ID. Each session is owned by an actor goroutine that drives the shell
// (session.Driver), translates ACP updates into normalized chat events,
// and keeps the session's record in hybrid-sessions.json.
//
// Content safety: nothing here logs prompts, messages, tool output,
// permission options, form content or agent stderr. Agent stderr only
// reaches the user who requested a failed start, in the returned error.
package hybrid

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"slices"
	"sync"
	"time"

	"github.com/rainchan/webmux/internal/agentprofile"
	"github.com/rainchan/webmux/internal/agentprofile/agents"
	"github.com/rainchan/webmux/internal/conversation"
	"github.com/rainchan/webmux/internal/session"
)

const (
	// defaultScrollbackBytes matches the session default when no limit is
	// configured.
	defaultScrollbackBytes = 2 << 20
	// shutdownTimeout bounds stopping the ACP processes in Close: actors
	// stop them in parallel and kill those still running after it.
	shutdownTimeout = 10 * time.Second
	// shutdownJoin bounds Close as a whole: the kill, the wait for killed
	// processes to go (goneTimeout), and a margin for the actors to end.
	shutdownJoin = shutdownTimeout + goneTimeout + time.Second
)

// Options configure a Service. Only cmd/ builds them.
type Options struct {
	// StateDir holds hybrid-sessions.json.
	StateDir string
	// Conversations is explicitly assembled by cmd; nil disables durable identity.
	Conversations *conversation.Service
	Manager       *session.Manager
	// ScrollbackBytes bounds each session's chat event log, like a
	// terminal's scrollback (config scrollback_bytes).
	ScrollbackBytes int
	// Version is the WebMux version sent as ACP clientInfo.
	Version string
	// Logger receives lifecycle events, method names and error codes only.
	Logger *slog.Logger
	// Uploads validates that path is one of session id's uploaded files and
	// returns it cleaned (artifacts.Service.UploadPath). Nil refuses every
	// attachment.
	Uploads func(id, path string) (string, error)
}

// timing holds the service's waits; tests shorten them.
type timing struct {
	// interrupt is how long a switch waits for the terminal UI to go idle
	// after the interrupt key before it asks the user (D21).
	interrupt time.Duration
	// probeDelay debounces the eligibility probe after the terminal's
	// agent session or cwd changes.
	probeDelay time.Duration
	// observe is how often a terminal side's identity is read.
	observe time.Duration
	// connectorRecheck is how often a missing connector is looked for
	// again while the terminal side runs.
	connectorRecheck time.Duration
	// bgPoll is how often a finished turn with an in_progress tool call
	// checks the agent's process tree before latching done.
	bgPoll time.Duration
}

var defaultTiming = timing{
	interrupt: 10 * time.Second, probeDelay: time.Second, observe: 500 * time.Millisecond,
	connectorRecheck: connectorRecheck, bgPoll: bgPoll,
}

// Service owns the hybrid sessions of one server.
type Service struct {
	conversations     *conversation.Service
	conversationError error
	manager           *session.Manager
	records           *records
	// options remembers each launch config's agent options for display
	// before a session starts (agent-options.json).
	options  *agentOptions
	capacity int
	version  string
	log      *slog.Logger
	timing   timing
	uploads  func(id, path string) (string, error)
	// agents recognises launch commands and terminal signals.
	agents *agentprofile.Registry
	// userHome is the service user's home directory, where agents keep
	// their state unless the session environment says otherwise.
	userHome string
	// probes holds a token per running eligibility probe (maxProbes).
	probes chan struct{}
	// resumes dedupes resume requests by client request ID.
	resumes dedupe
	// onEvicted cleans up after an auto-evicted record: its artifacts folder
	// and release ref. Nil keeps them.
	onEvicted func(context.Context, Record)

	mu       sync.Mutex
	actors   map[string]*actor
	closed   bool
	starting sync.WaitGroup
	// recovering holds the record IDs a resume or removal is working on.
	recovering map[string]bool
	// restarted holds the sessions this start found active and interrupted.
	restarted []string
}

// New opens the record store, marks sessions left active by an earlier
// server interrupted (ending their leftover processes), and reserves the
// recorded IDs in the manager. A store that cannot be opened leaves the
// service unavailable for hybrid creation; ordinary sessions keep working.
func New(o Options) *Service {
	log := o.Logger
	if log == nil {
		log = slog.New(slog.DiscardHandler)
	}
	capacity := o.ScrollbackBytes
	if capacity <= 0 {
		capacity = defaultScrollbackBytes
	}
	if o.Version == "" {
		o.Version = "dev"
	}
	userHome, _ := os.UserHomeDir()
	s := &Service{
		conversations: o.Conversations, manager: o.Manager, records: openRecords(o.StateDir), options: openAgentOptions(o.StateDir), capacity: capacity, version: o.Version, log: log,
		uploads: o.Uploads, userHome: userHome, agents: agents.Default(),
		timing: defaultTiming, probes: make(chan struct{}, maxProbes), actors: make(map[string]*actor),
		recovering: make(map[string]bool),
	}
	restarted, err := s.records.recoverAfterRestart(time.Now())
	if err != nil {
		log.Warn("hybrid sessions unavailable", "error", err)
	}
	s.conversationError = s.migrateConversations()
	if s.conversationError != nil {
		log.Warn("conversation identity migration unavailable")
	}
	s.restarted = restarted
	s.records.setLogger(log)
	s.options.setLogger(log)
	logAgentOptions(log, s.options)
	o.Manager.SetIDReserver(s.records.reserved)
	return s
}

// Unavailable reports why hybrid sessions cannot be created, or nil.
func (s *Service) Unavailable() error {
	if s.conversationError != nil {
		return fmt.Errorf("%w: conversation identity unavailable", ErrUnavailable)
	}
	return s.records.unavailable()
}

// Restarted returns the IDs of the sessions the previous server left
// running: the ones this start marked interrupted. A server that wants them
// back resumes each; sessions the user closed are not among them.
func (s *Service) Restarted() []string { return slices.Clone(s.restarted) }

// OnEvicted sets the cleanup of an auto-evicted record, called after its
// actor and shell ended. The server removes the record's artifacts folder
// and release ref there. Set it before serving.
func (s *Service) OnEvicted(fn func(context.Context, Record)) { s.onEvicted = fn }

// uploadCheck validates attachment paths against session id's uploads.
func (s *Service) uploadCheck(id string) func(string) (string, error) {
	return func(path string) (string, error) {
		if s.uploads == nil {
			return "", errors.New("上传不可用")
		}
		return s.uploads(id, path)
	}
}

func (s *Service) beginOperation() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return fmt.Errorf("%w: 服务已关闭", ErrUnavailable)
	}
	s.starting.Add(1)
	return nil
}

// Close stops every actor and its ACP process without changing records,
// so a restart finds the sessions active and marks them interrupted, then
// releases the store once the actors ended (at most shutdownJoin).
// Terminal runs are left to Manager.Shutdown.
func (s *Service) Close() {
	s.mu.Lock()
	s.closed = true
	actors := make([]*actor, 0, len(s.actors))
	for _, a := range s.actors {
		actors = append(actors, a)
	}
	s.mu.Unlock()
	ctx, cancel := context.WithTimeout(context.Background(), shutdownTimeout)
	defer cancel()
	join, cancelJoin := context.WithTimeout(context.Background(), shutdownJoin)
	defer cancelJoin()
	for _, a := range actors {
		a.post(func() { a.shutdown(ctx) })
	}
	for _, a := range actors {
		select {
		case <-a.quit:
		case <-join.Done():
			select {
			case <-a.quit:
			default:
				s.log.Warn("hybrid session did not stop in time", "id", a.id)
			}
		}
	}
	s.starting.Wait()
	s.records.close()
	s.options.close()
}

// CreateInput starts a hybrid session from a saved launch config. Cwd is
// resolved again through symlinks, as ACP cwds must be. Env reaches the
// processes only; it is never stored.
type CreateInput struct {
	Cwd       string
	Name      string
	ConfigID  string
	Cmd       []string
	Env       map[string]string
	StartSide Side
	// Cols and Rows are the initial terminal size, both zero for the default.
	Cols, Rows int
	// InitialPrompt resolves the first message for a terminal-side start once
	// the session ID is allocated. When the agent profile supports passing it
	// on the command line, InitialPromptSent is set to true so the caller does
	// not also type the message into the PTY.
	InitialPrompt     func(id string) (string, error)
	InitialPromptSent *bool
	// Attention configures completion and intervention notifications for both
	// sides of the hybrid session. Nil uses the global default policy.
	Attention *session.AttentionPolicy
}

// CheckCreate makes the checks of a Create of in before anything exists:
// the record store, the start side, the launch command's profile, the
// directory, the record limit and, for a chat-first start, the profile's
// connector, looked up as the start would in in.Cwd with in.Env. A missing
// connector is a *ConnectorMissingError. It registers nothing and starts
// only the connector lookup.
func (s *Service) CheckCreate(ctx context.Context, in CreateInput) error {
	cmd, side, cwd, err := s.admit(in)
	if err != nil || side != SideACP || cmd.Profile.Connector == nil {
		return err
	}
	probe, err := s.manager.ProbeShell(session.CreateInput{Cwd: cwd, Cmd: cmd.TUIArgv(), Env: in.Env, ConfigID: in.ConfigID})
	if err != nil {
		return shellError(err)
	}
	_, err = prepareACP(ctx, cmd, probe)
	return err
}

// admit runs the checks that precede every create.
func (s *Service) admit(in CreateInput) (agentprofile.Command, Side, string, error) {
	if err := s.Unavailable(); err != nil {
		return agentprofile.Command{}, "", "", err
	}
	side := in.StartSide
	if side == "" {
		side = SideTUI
	}
	if side != SideTUI && side != SideACP {
		return agentprofile.Command{}, "", "", fmt.Errorf("%w: startSide 只能是 tui 或 acp", ErrInvalid)
	}
	cmd, err := s.agents.Parse(in.Cmd)
	if err != nil {
		var unsupported *agentprofile.UnsupportedError
		if errors.As(err, &unsupported) {
			return agentprofile.Command{}, "", "", fmt.Errorf("%w: 该启动配置不能用于混合会话: %s", ErrInvalid, s.agents.Explain(unsupported.Reason))
		}
		return agentprofile.Command{}, "", "", fmt.Errorf("%w: %w", ErrInvalid, err)
	}
	cwd, err := filepath.EvalSymlinks(in.Cwd)
	if err == nil {
		cwd, err = filepath.Abs(cwd)
	}
	if err != nil {
		return agentprofile.Command{}, "", "", fmt.Errorf("%w: 目录: %w", ErrInvalid, err)
	}
	if !s.records.accepts(s.recoveringNow) {
		return agentprofile.Command{}, "", "", ErrLimit
	}
	s.mu.Lock()
	closed := s.closed
	s.mu.Unlock()
	if closed {
		return agentprofile.Command{}, "", "", fmt.Errorf("%w: 服务已关闭", ErrUnavailable)
	}
	return cmd, side, cwd, nil
}

// Create registers a hybrid session shell, records it, and starts its first
// side: the terminal UI (default) or the ACP chat (initialize, then
// session/new). A failed start removes the shell and the record; an ACP
// failure carries the agent's stderr tail in its message.
func (s *Service) Create(ctx context.Context, in CreateInput) (*session.Session, error) {
	if err := s.beginOperation(); err != nil {
		return nil, err
	}
	defer s.starting.Done()
	cmd, side, cwd, err := s.admit(in)
	if err != nil {
		return nil, err
	}
	a := newActor(s, cmd, cwd, in.ConfigID, side, in.Attention)
	// The shell shows the terminal command, with the options the profile
	// injects (OpenCode's --standalone); the record keeps the config's argv.
	shell, err := s.manager.CreateShell(session.ShellInput{
		CreateInput: session.CreateInput{
			Cwd: cwd, Cmd: cmd.TUIArgv(), Name: in.Name, Env: in.Env, ConfigID: in.ConfigID,
			Cols: in.Cols, Rows: in.Rows, Attention: in.Attention,
		},
		Overlay: a.overlay(),
	}, a)
	if err != nil {
		return nil, shellError(err)
	}
	info := shell.Info()
	a.id, a.shell = info.ID, shell
	if side == SideACP {
		// A missing connector refuses the session before it is recorded.
		if _, err := a.prepareACP(ctx); err != nil {
			_ = s.manager.Remove(info.ID)
			return nil, err
		}
	}
	var initialArgv []string
	if side == SideTUI && in.InitialPrompt != nil {
		if probe, ok := cmd.InitialTUIArgv("x"); ok && len(probe) > 0 {
			prompt, err := in.InitialPrompt(info.ID)
			if err != nil {
				_ = s.manager.Remove(info.ID)
				return nil, err
			}
			if argv, ok := cmd.InitialTUIArgv(prompt); ok {
				initialArgv = argv
				if in.InitialPromptSent != nil {
					*in.InitialPromptSent = true
				}
			}
		}
	}
	now := time.Now().UTC()
	rec := Record{
		ID: info.ID, Name: in.Name, Title: info.Title, ConfigID: in.ConfigID, Agent: cmd.Profile.Name, Cwd: cwd,
		Cmd: in.Cmd, Side: side, PreviousIDs: []string{}, Config: map[string]string{},
		State: StateActive, CreatedAt: now, UpdatedAt: now,
	}
	if err := s.ensureConversation(context.WithoutCancel(ctx), rec); err != nil {
		_ = s.manager.Remove(info.ID)
		return nil, err
	}
	a.recordTitle = rec.Title
	evicted, err := s.records.add(rec, s.recoveringNow)
	if err != nil {
		// The shell has no process yet: removal cannot be refused.
		if cleanupErr := s.forgetConversation(context.WithoutCancel(ctx), info.ID); cleanupErr != nil {
			s.log.Warn("conversation cleanup incomplete", "id", info.ID)
		}
		_ = s.manager.Remove(info.ID)
		return nil, err
	}
	s.mu.Lock()
	if s.closed {
		s.mu.Unlock()
		_ = s.manager.Remove(info.ID)
		if err := s.records.remove(info.ID); err != nil {
			s.log.Warn("hybrid record cleanup failed", "id", info.ID, "error", err)
		}
		if err := s.forgetConversation(context.Background(), info.ID); err != nil {
			s.log.Warn("conversation cleanup incomplete", "id", info.ID)
		}
		for _, e := range evicted {
			if err := s.records.restore(e); err != nil {
				s.log.Warn("hybrid evicted record restore failed", "id", e.ID, "error", err)
			}
		}
		return nil, fmt.Errorf("%w: 服务已关闭", ErrUnavailable)
	}
	s.actors[a.id] = a
	s.mu.Unlock()
	go a.run()
	go a.watchShell()
	go a.observeLoop()
	started := make(chan error, 1)
	a.post(func() { a.start(started, initialArgv) })
	// The start runs to completion even if ctx ends, so a cancelled request
	// never leaves a half-started session behind.
	if err := awaitStarted(a, started); err != nil {
		// The failed start removed its own record, so the dropped ones
		// go back rather than being lost for nothing.
		if err := s.forgetConversation(context.Background(), info.ID); err != nil {
			s.log.Warn("conversation cleanup incomplete", "id", info.ID)
		}
		for _, e := range evicted {
			if err := s.records.restore(e); err != nil {
				s.log.Warn("hybrid evicted record restore failed", "id", e.ID, "error", err)
			}
		}
		return nil, err
	}
	for _, e := range evicted {
		s.log.Info("hybrid record evicted", "id", e.ID)
		s.evict(ctx, e.ID)
		if err := s.forgetConversation(context.WithoutCancel(ctx), e.ID); err != nil {
			s.log.Warn("evicted conversation cleanup incomplete", "id", e.ID)
		}
		if s.onEvicted != nil {
			s.onEvicted(context.WithoutCancel(ctx), e)
		}
	}
	if err := ctx.Err(); err != nil {
		s.log.Info("hybrid session created after its request ended", "id", a.id)
	}
	return shell, nil
}

// evict ends what an archived record dropped for a new session left behind:
// its actor and its last-output shell. The record is already gone, so a
// failure is logged and never blocks the new session.
func (s *Service) evict(ctx context.Context, id string) {
	if err := s.clear(context.WithoutCancel(ctx), id); err != nil {
		s.log.Warn("hybrid evicted record cleanup failed", "id", id, "error", err)
	}
}

// shellError keeps the session start error kinds as hybrid kinds; a
// resumed ID still in use is a conflict.
func shellError(err error) error {
	switch {
	case errors.Is(err, session.ErrInvalid):
		return fmt.Errorf("%w: 启动会话失败: %w", ErrInvalid, err)
	case errors.Is(err, session.ErrShuttingDown):
		return fmt.Errorf("%w: 启动会话失败: %w", ErrUnavailable, err)
	case errors.Is(err, session.ErrExists):
		return fmt.Errorf("%w: 启动会话失败: %w", ErrConflict, err)
	}
	return fmt.Errorf("启动会话失败: %w", err)
}

func (s *Service) unregister(a *actor) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.actors[a.id] == a {
		delete(s.actors, a.id)
	}
}

func (s *Service) actor(id string) (*actor, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	a, ok := s.actors[id]
	if !ok {
		return nil, ErrNotFound
	}
	return a, nil
}

// recoveringNow reports whether id is being resumed or removed right now, so
// auto-eviction must not drop it.
func (s *Service) recoveringNow(id string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.recovering[id]
}

// Subscribe opens a gap-free view of session id's chat: a snapshot, the
// events after resume (or all cached events, with Reset when resume is no
// longer in the log), then live frames. A subscriber that falls behind is
// dropped with ErrSlow.
func (s *Service) Subscribe(id string, resume *ResumePoint) (*Subscription, error) {
	a, err := s.actor(id)
	if err != nil {
		return nil, err
	}
	return a.log.subscribe(resume)
}
