package workspace

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"path"
	"path/filepath"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/rainchan/webmux/internal/hybrid"
)

const (
	// quickStartRetention is how long a finished quick start answers a
	// retry of its request ID with its outcome.
	quickStartRetention = 10 * time.Minute
	// QuickStartTimeout bounds one quick start: the checks, the worktree,
	// the agent's start, its options and the first prompt.
	QuickStartTimeout = 3 * time.Minute
)

// Quick start steps, as QuickStartError.Step.
const (
	StepPrecheck = "precheck"
	StepWorktree = "worktree"
	StepSession  = "session"
	StepConfig   = "config"
	StepPrompt   = "prompt"
)

// quickStartID is a client request ID, as hybrid chat commands take it.
var quickStartID = regexp.MustCompile(`^[A-Za-z0-9_-]{8,64}$`)

// QuickStartInput starts a chat in a new worktree in one request. Config
// maps option IDs to values (a select value, or "true"/"false" for a
// boolean); PermissionMode is ask, auto or empty for the default.
// Attachments are files uploaded to the quick start staging folder; they
// move to the new session and go with the first prompt.
type QuickStartInput struct {
	ClientRequestID string
	RepositoryID    string
	ConfigID        string
	Base            string
	Branch          string
	Text            string
	Config          map[string]string
	PermissionMode  string
	Attachments     []string
}

// QuickStartResult is a quick start that sent its first prompt.
// ConfigWarnings describe each Config item the agent did not take.
type QuickStartResult struct {
	SessionID      string
	Worktree       Worktree
	ConfigWarnings []string
}

// QuickStartError is a quick start that stopped at Step. Worktree and
// SessionID are what it had created; nothing is rolled back. Err keeps the
// error kind of the failed step.
type QuickStartError struct {
	Step      string
	Worktree  *Worktree
	SessionID string
	Err       error
}

func (e *QuickStartError) Error() string { return e.Err.Error() }
func (e *QuickStartError) Unwrap() error { return e.Err }

// quickStartFlight is one quick start by client request ID: duplicates
// wait for done and share its outcome.
type quickStartFlight struct {
	done   chan struct{}
	result QuickStartResult
	err    error
}

// quickStarts dedupes quick starts by client request ID, in memory only.
type quickStarts struct {
	mu      sync.Mutex
	flights map[string]*quickStartFlight
	// retention replaces quickStartRetention when set (tests).
	retention time.Duration
}

// QuickStart creates a worktree on a new branch, starts a chat-first
// hybrid session of a saved launch config in it, applies the requested
// agent options and permission mode, and sends the first prompt, each step
// through the same service methods a client uses one by one. What can be
// checked beforehand (repository, launch config, connector) is checked
// before anything is created. A duplicate of a running or, for
// quickStartRetention, finished request ID gets its outcome without
// running again. Failures are *QuickStartError.
func (s *Service) QuickStart(ctx context.Context, in QuickStartInput) (QuickStartResult, error) {
	if !quickStartID.MatchString(in.ClientRequestID) {
		return QuickStartResult{}, &QuickStartError{Step: StepPrecheck, Err: problemf(ErrInvalid, "clientRequestId 必须是 8 到 64 个字母、数字、- 或 _")}
	}
	s.quickStarts.mu.Lock()
	if s.quickStarts.flights == nil {
		s.quickStarts.flights = make(map[string]*quickStartFlight)
	}
	flight, running := s.quickStarts.flights[in.ClientRequestID]
	if !running {
		flight = &quickStartFlight{done: make(chan struct{})}
		s.quickStarts.flights[in.ClientRequestID] = flight
	}
	s.quickStarts.mu.Unlock()
	if running {
		select {
		case <-flight.done:
			return flight.result, flight.err
		case <-ctx.Done():
			return QuickStartResult{}, &QuickStartError{Step: StepPrecheck, Err: ctx.Err()}
		}
	}
	flight.result, flight.err = s.quickStart(ctx, in)
	close(flight.done)
	retention := s.quickStarts.retention
	if retention == 0 {
		retention = quickStartRetention
	}
	time.AfterFunc(retention, func() {
		s.quickStarts.mu.Lock()
		defer s.quickStarts.mu.Unlock()
		if s.quickStarts.flights[in.ClientRequestID] == flight {
			delete(s.quickStarts.flights, in.ClientRequestID)
		}
	})
	return flight.result, flight.err
}

func (s *Service) quickStart(ctx context.Context, in QuickStartInput) (QuickStartResult, error) {
	fail := func(step string, tree *Worktree, sessionID string, err error) (QuickStartResult, error) {
		return QuickStartResult{}, &QuickStartError{Step: step, Worktree: tree, SessionID: sessionID, Err: err}
	}
	plan, err := s.quickStartPlan(ctx, in)
	if err != nil {
		return fail(StepPrecheck, nil, "", err)
	}
	tree, err := s.CreateWorktree(ctx, in.RepositoryID, CreateRequest{Path: plan.path, Mode: "new", Branch: in.Branch, Base: in.Base})
	if err != nil {
		return fail(StepWorktree, nil, "", err)
	}
	configID := in.ConfigID
	item, err := s.CreateSession(ctx, SessionInput{
		ConfigID: &configID, StartSide: string(hybrid.SideACP),
		Workspace: &Reference{RepositoryID: in.RepositoryID, WorktreeID: tree.ID},
	})
	if err != nil {
		return fail(StepSession, &tree, "", err)
	}
	id := item.Info().ID
	warnings, err := s.applyQuickStartConfig(ctx, id, in)
	if err != nil {
		return fail(StepConfig, &tree, id, err)
	}
	attachments, err := s.adoptQuickStartUploads(id, plan.attachments)
	if err != nil {
		return fail(StepPrompt, &tree, id, err)
	}
	if _, err := s.hybrids.Prompt(ctx, id, hybrid.PromptInput{ClientRequestID: in.ClientRequestID, Text: plan.text, Attachments: attachments}); err != nil {
		return fail(StepPrompt, &tree, id, err)
	}
	return QuickStartResult{SessionID: id, Worktree: tree, ConfigWarnings: warnings}, nil
}

// quickStartPlan is a checked quick start: the worktree path, the first
// prompt's text and its staged attachments.
type quickStartPlan struct {
	path        string
	text        string
	attachments []string
}

// quickStartPlan makes the checks a quick start can make before it
// creates anything: the request, the repository's availability, a launch
// config with hybrid mode, the hybrid store and the agent's connector.
func (s *Service) quickStartPlan(ctx context.Context, in QuickStartInput) (quickStartPlan, error) {
	if strings.TrimSpace(in.Text) == "" {
		return quickStartPlan{}, problemf(ErrInvalid, "请输入消息")
	}
	if in.PermissionMode != "" && in.PermissionMode != string(hybrid.PermissionAsk) && in.PermissionMode != string(hybrid.PermissionAuto) {
		return quickStartPlan{}, problemf(ErrInvalid, "permissionMode 只能是 ask 或 auto")
	}
	leaf := path.Base(in.Branch)
	if in.Branch == "" || leaf == "." || leaf == "/" || strings.HasPrefix(leaf, "-") {
		return quickStartPlan{}, problemf(ErrInvalid, "分支名无效")
	}
	if len(in.Attachments) > hybrid.MaxAttachments {
		return quickStartPlan{}, problemf(ErrInvalid, "每条消息最多 %d 个附件", hybrid.MaxAttachments)
	}
	staged := make([]string, 0, len(in.Attachments))
	for _, p := range in.Attachments {
		clean, err := s.artifacts.StagedPath(p)
		if err != nil || slices.Contains(staged, clean) {
			return quickStartPlan{}, problemf(ErrInvalid, "附件不是本次快速开始上传的文件")
		}
		staged = append(staged, clean)
	}
	configID := in.ConfigID
	command, err := s.resolveCommand(SessionInput{ConfigID: &configID, StartSide: string(hybrid.SideACP)})
	if err != nil {
		return quickStartPlan{}, err
	}
	if s.hybrids == nil {
		return quickStartPlan{}, problemf(ErrUnavailable, "混合会话不可用")
	}
	r, err := s.repository(in.RepositoryID)
	if err != nil {
		return quickStartPlan{}, err
	}
	readCtx, cancel := context.WithTimeout(ctx, readTimeout)
	snapshot, err := s.snapshot(readCtx, r)
	cancel()
	if err != nil {
		return quickStartPlan{}, err
	}
	if snapshot.Availability != "available" {
		return quickStartPlan{}, problemf(ErrUnavailable, "%s", snapshot.Error)
	}
	text := in.Text
	if instruction := snapshot.InitInstruction; instruction != "" {
		text = instruction + "\n\n" + in.Text
	}
	if len(text) > hybrid.MaxPromptBytes {
		return quickStartPlan{}, problemf(ErrInvalid, "消息与初始化指令合计超过 %d 字节", hybrid.MaxPromptBytes)
	}
	if err := s.hybrids.CheckCreate(ctx, hybrid.CreateInput{
		Cwd: r.entry.AnchorPath, ConfigID: command.configID, Cmd: command.cmd, Env: command.env, StartSide: hybrid.SideACP,
	}); err != nil {
		return quickStartPlan{}, err
	}
	parent := filepath.Join(filepath.Dir(r.entry.AnchorPath), filepath.Base(r.entry.AnchorPath)+".worktrees")
	return quickStartPlan{path: filepath.Join(parent, leaf), text: text, attachments: staged}, nil
}

// adoptQuickStartUploads moves the staged attachments into session id's
// uploads, where its prompts take files from.
func (s *Service) adoptQuickStartUploads(id string, staged []string) ([]hybrid.Attachment, error) {
	paths, err := s.artifacts.Adopt(id, staged)
	if err != nil {
		return nil, problemf(ErrUnavailable, "附件无法交给会话：%v", err)
	}
	out := make([]hybrid.Attachment, 0, len(paths))
	for _, p := range paths {
		out = append(out, hybrid.Attachment{Path: p})
	}
	return out, nil
}

// QuickStartOptions reads the agent options launch config configID offers
// before any chat of it exists: the agent opens a throwaway session in the
// repository's main worktree and reports them (hybrid ProbeAgentOptions).
func (s *Service) QuickStartOptions(ctx context.Context, repositoryID, configID string) ([]hybrid.ConfigOption, error) {
	command, err := s.resolveCommand(SessionInput{ConfigID: &configID, StartSide: string(hybrid.SideACP)})
	if err != nil {
		return nil, err
	}
	if s.hybrids == nil {
		return nil, problemf(ErrUnavailable, "混合会话不可用")
	}
	r, err := s.repository(repositoryID)
	if err != nil {
		return nil, err
	}
	return s.hybrids.ProbeAgentOptions(ctx, hybrid.CreateInput{
		Cwd: r.entry.AnchorPath, ConfigID: command.configID, Cmd: command.cmd, Env: command.env,
	})
}

// applyQuickStartConfig sets each requested option that differs from what
// the session started with, then the permission mode. An option the agent
// does not offer or refuses is a warning, also shown in the chat; any
// other failure stops the quick start.
func (s *Service) applyQuickStartConfig(ctx context.Context, id string, in QuickStartInput) ([]string, error) {
	warnings := []string{}
	current, err := s.hybrids.Settings(ctx, id)
	if err != nil {
		return warnings, err
	}
	warn := func(text string) error {
		warnings = append(warnings, text)
		return s.hybrids.Notice(ctx, id, text)
	}
	optionIDs := make([]string, 0, len(in.Config))
	for optionID := range in.Config {
		optionIDs = append(optionIDs, optionID)
	}
	slices.Sort(optionIDs)
	for _, optionID := range optionIDs {
		want := in.Config[optionID]
		i := slices.IndexFunc(current.Config, func(o hybrid.ConfigOption) bool { return o.ID == optionID })
		if i < 0 {
			if err := warn(fmt.Sprintf("配置项 %s 不可用，未应用 %s", optionID, want)); err != nil {
				return warnings, err
			}
			continue
		}
		option := current.Config[i]
		set := hybrid.ConfigInput{ClientRequestID: derivedRequestID(in.ClientRequestID, "config", optionID), ConfigID: optionID}
		if option.Enabled != nil {
			enabled, err := strconv.ParseBool(want)
			if err != nil || (want != "true" && want != "false") {
				if err := warn(fmt.Sprintf("配置项 %s 只接受 true 或 false，未应用 %s", option.Name, want)); err != nil {
					return warnings, err
				}
				continue
			}
			if enabled == *option.Enabled {
				continue
			}
			set.Enabled = &enabled
		} else {
			if want == option.Value {
				continue
			}
			set.Value = &want
		}
		err := s.hybrids.SetConfig(ctx, id, set)
		switch {
		case err == nil:
		case errors.Is(err, hybrid.ErrInvalid), errors.Is(err, hybrid.ErrAgentCall):
			if err := warn(fmt.Sprintf("配置项 %s 未能设为 %s：%v", option.Name, want, err)); err != nil {
				return warnings, err
			}
		default:
			return warnings, err
		}
	}
	if mode := hybrid.PermissionMode(in.PermissionMode); mode != "" && mode != current.PermissionMode {
		if err := s.hybrids.SetPermissionMode(ctx, id, hybrid.PermissionModeInput{
			ClientRequestID: derivedRequestID(in.ClientRequestID, "permission", ""), Mode: mode,
		}); err != nil {
			return warnings, err
		}
	}
	return warnings, nil
}

// derivedRequestID is the client request ID of one command a quick start
// sends for request: stable, so a command is never applied twice.
func derivedRequestID(request, command, item string) string {
	sum := sha256.Sum256([]byte(request + "\x00" + command + "\x00" + item))
	return "qs-" + hex.EncodeToString(sum[:16])
}
