package workspace

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/rainchan/webmux/internal/hybrid"
	"github.com/rainchan/webmux/internal/launch"
	"github.com/rainchan/webmux/internal/session"
)

const (
	// createRetention is how long a finished create answers a retry of its
	// request ID with its outcome.
	createRetention = 10 * time.Minute
	// CreateTimeout bounds one create: the checks, the worktree, the
	// session's start, the agent's options and the first prompt.
	CreateTimeout = 3 * time.Minute
)

// Create steps, as CreateError.Step.
const (
	StepPrecheck = "precheck"
	StepWorktree = "worktree"
	StepSession  = "session"
	StepConfig   = "config"
	StepPrompt   = "prompt"
)

// createRequestID is a client request ID, as hybrid chat commands take it.
var createRequestID = regexp.MustCompile(`^[A-Za-z0-9_-]{8,64}$`)

// NewWorktree is a worktree Create makes before the session. Mode is new (a
// new Branch from Base) or existing (the existing Branch). An empty Branch
// of a new worktree is generated, an empty Base is the repository's default
// base, and an empty Path derives the directory from the branch.
type NewWorktree struct {
	RepositoryID string
	Mode         string
	Branch       string
	Base         string
	Path         string
}

// CreateInput describes a session to create. The location is Cwd, Workspace
// (Cwd may name a directory inside it) or NewWorktree; the command is
// ConfigID or Cmd with Env. Side (tui or acp) applies only to a config with
// hybrid mode; empty starts in the terminal. Prompt is the user's first
// message; ExpandPrompt fills its {{dir}} and {{webmux}} once the session
// exists, otherwise it is sent as written. UseInitInstruction puts the
// repository's initialization
// instruction before it: it is sent once the session is up, as the first
// chat prompt on the chat side, otherwise typed into the terminal with
// Enter. Attachments are files staged by an upload before the session
// existed, moved into the session's uploads folder and sent with the first
// chat prompt or appended as paths to the terminal message. AgentConfig
// (option ID to value: a select value, or "true"/"false" for a boolean)
// and PermissionMode (ask, auto or empty for the default) belong to the
// chat side only. A non-empty ClientRequestID dedupes retries. Cols and
// Rows are the terminal's initial size (the TUI side of a hybrid session
// too): both zero for the default, otherwise both set within 1..1000.
type CreateInput struct {
	ClientRequestID string
	Cwd             string
	Workspace       *Reference
	NewWorktree     *NewWorktree
	Cmd             []string
	Env             map[string]string
	ConfigID        *string
	Name            string
	Side            string

	Prompt             string
	UseInitInstruction bool
	ExpandPrompt       bool
	Attachments        []string
	AgentConfig        map[string]string
	PermissionMode     string
	Cols, Rows         int
}

// CreateResult is a created session. Worktree is the worktree the request
// created, nil when it used an existing location; ConfigWarnings describe
// each AgentConfig item the agent did not take.
type CreateResult struct {
	Session        *session.Session
	Worktree       *Worktree
	ConfigWarnings []string
}

// CreateError is a create that stopped at Step. Worktree and SessionID are
// what it had created; nothing is rolled back. Err keeps the error kind of
// the failed step.
type CreateError struct {
	Step      string
	Worktree  *Worktree
	SessionID string
	Err       error
}

func (e *CreateError) Error() string { return e.Err.Error() }
func (e *CreateError) Unwrap() error { return e.Err }

// createFlight is one create by client request ID: duplicates wait for done
// and share its outcome.
type createFlight struct {
	done   chan struct{}
	result CreateResult
	err    error
}

// creates dedupes creates by client request ID, in memory only.
type creates struct {
	mu      sync.Mutex
	flights map[string]*createFlight
	// retention replaces createRetention when set (tests).
	retention time.Duration
}

// Create creates a session: the worktree first when asked, then the
// session, its agent options and permission mode, and its first message,
// each step through the same service methods a client could use one by
// one. What can be checked beforehand (location, launch config, connector,
// first message) is checked before anything is created. A duplicate of a
// running or, for createRetention, finished request ID gets its outcome
// without running again. Failures are *CreateError.
func (s *Service) Create(ctx context.Context, in CreateInput) (CreateResult, error) {
	if in.ClientRequestID == "" {
		return s.create(ctx, in)
	}
	if !createRequestID.MatchString(in.ClientRequestID) {
		return CreateResult{}, &CreateError{Step: StepPrecheck, Err: problemf(ErrInvalid, "clientRequestId 必须是 8 到 64 个字母、数字、- 或 _")}
	}
	s.creates.mu.Lock()
	if s.creates.flights == nil {
		s.creates.flights = make(map[string]*createFlight)
	}
	flight, running := s.creates.flights[in.ClientRequestID]
	if !running {
		flight = &createFlight{done: make(chan struct{})}
		s.creates.flights[in.ClientRequestID] = flight
	}
	s.creates.mu.Unlock()
	if running {
		select {
		case <-flight.done:
			return flight.result, flight.err
		case <-ctx.Done():
			return CreateResult{}, &CreateError{Step: StepPrecheck, Err: ctx.Err()}
		}
	}
	flight.result, flight.err = s.create(ctx, in)
	close(flight.done)
	retention := s.creates.retention
	if retention == 0 {
		retention = createRetention
	}
	time.AfterFunc(retention, func() {
		s.creates.mu.Lock()
		defer s.creates.mu.Unlock()
		if s.creates.flights[in.ClientRequestID] == flight {
			delete(s.creates.flights, in.ClientRequestID)
		}
	})
	return flight.result, flight.err
}

func (s *Service) create(ctx context.Context, in CreateInput) (CreateResult, error) {
	fail := func(step string, tree *Worktree, sessionID string, err error) (CreateResult, error) {
		return CreateResult{}, &CreateError{Step: step, Worktree: tree, SessionID: sessionID, Err: err}
	}
	plan, err := s.createPlan(ctx, in)
	if err != nil {
		return fail(StepPrecheck, nil, "", err)
	}
	reference := in.Workspace
	var tree *Worktree
	if nw := in.NewWorktree; nw != nil {
		created, err := s.CreateWorktree(ctx, nw.RepositoryID, CreateRequest{Path: plan.path, Mode: nw.Mode, Branch: plan.branch, Base: plan.base})
		if err != nil {
			return fail(StepWorktree, nil, "", err)
		}
		tree = &created
		reference = &Reference{RepositoryID: nw.RepositoryID, WorktreeID: created.ID}
	}
	var initialPrompt func(id string) (string, error)
	var preparedPrompt string
	var promptPrepared bool
	prepareTerminalPrompt := func(id string) (string, error) {
		if promptPrepared {
			return preparedPrompt, nil
		}
		text, err := s.composeTerminalPrompt(id, plan)
		if err != nil {
			return "", err
		}
		preparedPrompt = text
		promptPrepared = true
		return preparedPrompt, nil
	}
	if plan.side != hybrid.SideACP && (plan.text != "" || plan.instruction != "") {
		initialPrompt = prepareTerminalPrompt
	}
	name := in.Name
	if name == "" && plan.side != hybrid.SideACP {
		name = plan.title
	}
	item, promptSent, err := s.startSession(ctx, in.Cwd, name, reference, plan.command, plan.side, initialPrompt, nil)
	if err != nil {
		return fail(StepSession, tree, "", err)
	}
	id := item.Info().ID
	warnings := []string{}
	if plan.side == hybrid.SideACP && (len(in.AgentConfig) > 0 || in.PermissionMode != "") {
		if warnings, err = s.applyAgentConfig(ctx, id, in); err != nil {
			return fail(StepConfig, tree, id, err)
		}
	}
	if !promptSent && (plan.text != "" || plan.instruction != "") {
		if plan.side != hybrid.SideACP {
			text, err := prepareTerminalPrompt(id)
			if err != nil {
				return fail(StepPrompt, tree, id, err)
			}
			go func() { _ = item.SendWhenReady(context.Background(), text) }()
		} else if err := s.sendFirstMessage(ctx, item, in.ClientRequestID, plan); err != nil {
			return fail(StepPrompt, tree, id, err)
		}
	}
	return CreateResult{Session: item, Worktree: tree, ConfigWarnings: warnings}, nil
}

// createPlan is a checked create: the command, the side, the new
// worktree's branch, base and path, and the first message with its staged
// attachments.
type createPlan struct {
	command resolvedCommand
	side    hybrid.Side
	branch  string
	base    string
	path    string
	// text is the user's message; instruction is the expanded
	// initialization instruction ("" for none), still holding {{dir}}.
	// expand asks for text's own {{dir}} and {{webmux}} to be filled.
	text        string
	instruction string
	expand      bool
	attachments []string
	// title is the first-prompt title of the user's text alone (no
	// instruction, no placeholders), "" for none.
	title string
}

// createPlan makes the checks a create can make before it creates
// anything: the request's shape, the launch config and side, the
// repository's availability and initialization instruction, the first
// message's size and attachments, and for a new worktree on the chat side
// the hybrid store and the agent's connector.
func (s *Service) createPlan(ctx context.Context, in CreateInput) (createPlan, error) {
	plan := createPlan{text: strings.TrimSpace(in.Prompt), expand: in.ExpandPrompt}
	plan.title = hybrid.PromptTitle(plan.text)
	if in.ExpandPrompt {
		plan.title = hybrid.PromptTitle(strings.NewReplacer(artifactsDirPlaceholder, "", webmuxPlaceholder, "").Replace(plan.text))
	}
	if in.NewWorktree != nil && (in.Cwd != "" || in.Workspace != nil) {
		return plan, problemf(ErrInvalid, "cwd、workspace 与 newWorktree 只能选择一个")
	}
	if err := session.ValidateInitialSize(in.Cols, in.Rows); err != nil {
		return plan, problemf(ErrInvalid, "%v", err)
	}
	command, err := s.resolveCommand(in)
	if err != nil {
		return plan, err
	}
	command.cols, command.rows = in.Cols, in.Rows
	plan.command = command
	plan.side = hybrid.Side(in.Side)
	if plan.side == "" {
		plan.side = hybrid.SideTUI
	}
	if plan.side != hybrid.SideACP && (len(in.AgentConfig) > 0 || in.PermissionMode != "") {
		return plan, problemf(ErrInvalid, "agentConfig 与 permissionMode 只适用于聊天视图")
	}
	if len(in.Attachments) > 0 && plan.text == "" {
		return plan, problemf(ErrInvalid, "附件需要随消息发送，请输入消息")
	}
	if in.PermissionMode != "" && in.PermissionMode != string(hybrid.PermissionAsk) && in.PermissionMode != string(hybrid.PermissionAuto) {
		return plan, problemf(ErrInvalid, "permissionMode 只能是 ask 或 auto")
	}
	if in.UseInitInstruction && in.Workspace == nil && in.NewWorktree == nil {
		return plan, problemf(ErrInvalid, "初始化指令属于仓库，请选择仓库中的位置")
	}
	if len(in.Prompt) > hybrid.MaxPromptBytes {
		return plan, problemf(ErrInvalid, "用户指令超过 %d 字节", hybrid.MaxPromptBytes)
	}
	if len(in.Attachments) > hybrid.MaxAttachments {
		return plan, problemf(ErrInvalid, "每条消息最多 %d 个附件", hybrid.MaxAttachments)
	}
	if len(in.Attachments) > 0 && s.artifacts == nil {
		return plan, problemf(ErrUnavailable, "临时文件夹不可用")
	}
	if in.ExpandPrompt {
		if strings.Contains(plan.text, artifactsDirPlaceholder) && s.artifacts == nil {
			return plan, problemf(ErrUnavailable, "临时文件夹不可用")
		}
		if strings.Contains(plan.text, webmuxPlaceholder) && (s.artifacts == nil || s.brief == nil) {
			return plan, problemf(ErrUnavailable, "WebMux 工具说明不可用")
		}
	}
	for _, p := range in.Attachments {
		clean, err := s.artifacts.StagedPath(p)
		if err != nil || slices.Contains(plan.attachments, clean) {
			return plan, problemf(ErrInvalid, "附件不是为新会话上传的文件")
		}
		plan.attachments = append(plan.attachments, clean)
	}
	if command.hybrid {
		if s.hybrids == nil {
			return plan, problemf(ErrUnavailable, "混合会话不可用")
		}
		if err := s.hybrids.Unavailable(); err != nil {
			return plan, err
		}
	}
	repositoryID := ""
	switch {
	case in.NewWorktree != nil:
		repositoryID = in.NewWorktree.RepositoryID
	case in.Workspace != nil:
		repositoryID = in.Workspace.RepositoryID
	}
	if repositoryID == "" {
		return plan, nil
	}
	r, err := s.repository(repositoryID)
	if err != nil {
		return plan, err
	}
	readCtx, cancel := context.WithTimeout(ctx, readTimeout)
	defer cancel()
	snapshot, err := s.snapshot(readCtx, r)
	if err != nil {
		return plan, err
	}
	if snapshot.Availability != "available" {
		return plan, problemf(ErrUnavailable, "%s", snapshot.Error)
	}
	if in.UseInitInstruction {
		if plan.instruction, err = s.expandRepositoryInstruction(snapshot.InitInstruction); err != nil {
			return plan, err
		}
		if strings.Contains(plan.instruction, artifactsDirPlaceholder) && s.artifacts == nil {
			return plan, problemf(ErrUnavailable, "临时文件夹不可用")
		}
		if strings.Contains(plan.instruction, webmuxPlaceholder) && (s.artifacts == nil || s.brief == nil) {
			return plan, problemf(ErrUnavailable, "WebMux 工具说明不可用")
		}
		if plan.instruction != "" && promptSize(plan.instruction, plan.text) > hybrid.MaxPromptBytes {
			return plan, problemf(ErrInvalid, "消息与初始化指令合计超过 %d 字节", hybrid.MaxPromptBytes)
		}
	}
	if in.NewWorktree == nil {
		return plan, nil
	}
	if err := s.planWorktree(readCtx, r, in.NewWorktree, &plan); err != nil {
		return plan, err
	}
	if plan.side == hybrid.SideACP {
		if err := s.hybrids.CheckCreate(ctx, hybrid.CreateInput{
			Cwd: r.entry.AnchorPath, ConfigID: command.configID, Cmd: command.cmd, Env: command.env, StartSide: plan.side,
		}); err != nil {
			return plan, err
		}
	}
	return plan, nil
}

// planWorktree settles a new worktree's branch, base and path: a new
// branch without a name gets wt/<date>-<time>, free as a branch and as a
// directory; a missing base is the repository's default base; a missing
// path is the branch's last segment under the repository's worktrees
// folder.
func (s *Service) planWorktree(ctx context.Context, r *repositoryState, nw *NewWorktree, plan *createPlan) error {
	if nw.Mode != "new" && nw.Mode != "existing" {
		return problemf(ErrInvalid, "newWorktree.mode 只能是 new 或 existing")
	}
	plan.branch, plan.base, plan.path = nw.Branch, nw.Base, nw.Path
	if nw.Mode == "existing" && plan.branch == "" {
		return problemf(ErrInvalid, "请选择已有本地分支")
	}
	parent := filepath.Join(filepath.Dir(r.entry.AnchorPath), filepath.Base(r.entry.AnchorPath)+".worktrees")
	if nw.Mode == "new" && (plan.branch == "" || plan.base == "") {
		repository, err := s.current(ctx, r)
		if err != nil {
			return err
		}
		branches, err := s.branches(ctx, r, repository)
		if err != nil {
			return err
		}
		if plan.base == "" {
			plan.base = "HEAD"
			if branches.DefaultBase != nil {
				plan.base = *branches.DefaultBase
			}
		}
		if plan.branch == "" {
			taken := make(map[string]bool, len(branches.Branches))
			for _, b := range branches.Branches {
				taken[b.Name] = true
			}
			plan.branch = generatedBranch(time.Now(), func(branch string) bool {
				if taken[branch] {
					return false
				}
				_, err := os.Lstat(filepath.Join(parent, path.Base(branch)))
				return errors.Is(err, os.ErrNotExist)
			})
		}
	}
	leaf := path.Base(plan.branch)
	if leaf == "." || leaf == "/" || strings.HasPrefix(leaf, "-") {
		return problemf(ErrInvalid, "分支名无效")
	}
	if plan.path == "" {
		plan.path = filepath.Join(parent, leaf)
	}
	return nil
}

// generatedBranch is wt/<date>-<time> at now, with a -2, -3, ... suffix
// until free accepts it.
func generatedBranch(now time.Time, free func(string) bool) string {
	stamp := now.Format("20060102-150405")
	for suffix := 1; suffix <= 1000; suffix++ {
		name := "wt/" + stamp
		if suffix > 1 {
			name = fmt.Sprintf("wt/%s-%d", stamp, suffix)
		}
		if free(name) {
			return name
		}
	}
	return fmt.Sprintf("wt/%s-%d", stamp, now.Nanosecond())
}

// resolvedCommand is the command a session start runs.
type resolvedCommand struct {
	cmd       []string
	env       map[string]string
	configID  string // empty for an explicit command
	hybrid    bool
	attention *session.AttentionPolicy
	// cols and rows are the terminal's requested initial size, zero for the
	// default.
	cols, rows int
}

func attentionPolicy(a *launch.AttentionSettings) *session.AttentionPolicy {
	if a == nil {
		return nil
	}
	return &session.AttentionPolicy{
		Mode:          session.AttentionMode(a.Mode),
		ProgramStatus: a.ProgramStatus,
		AgentProtocol: a.AgentProtocol,
		Notify:        a.Notify,
		Bell:          a.Bell,
		MinTurn:       time.Duration(a.MinTurnSeconds) * time.Second,
		DoneDelay:     time.Duration(a.DoneDelaySeconds) * time.Second,
	}
}

// resolveCommand applies a saved launch config and validates the explicit
// environment and side. It runs before any filesystem or Git work.
func (s *Service) resolveCommand(in CreateInput) (resolvedCommand, error) {
	out := resolvedCommand{cmd: in.Cmd, env: in.Env}
	if in.ConfigID != nil {
		if *in.ConfigID == "" {
			return resolvedCommand{}, problemf(ErrInvalid, "configId must not be empty")
		}
		if len(in.Cmd) > 0 || len(in.Env) > 0 {
			return resolvedCommand{}, problemf(ErrInvalid, "configId cannot be combined with cmd or env")
		}
		// A nil store answers launch.ErrUnavailable.
		saved, err := s.launches.Get(*in.ConfigID)
		if err != nil {
			return resolvedCommand{}, err
		}
		out = resolvedCommand{
			cmd: saved.Cmd, env: saved.Env, configID: saved.ID, hybrid: saved.Hybrid,
			attention: attentionPolicy(saved.Attention),
		}
	}
	switch in.Side {
	case "", string(hybrid.SideTUI):
	case string(hybrid.SideACP):
		if !out.hybrid {
			return resolvedCommand{}, problemf(ErrInvalid, "聊天视图只适用于开启了聊天视图的启动配置")
		}
	default:
		return resolvedCommand{}, problemf(ErrInvalid, "view 只能是 terminal 或 chat")
	}
	if err := session.ValidateEnv(out.env); err != nil {
		return resolvedCommand{}, problemf(ErrInvalid, "%v", err)
	}
	return out, nil
}

// startSession starts a session in cwd or the selected worktree. Launch
// config errors keep their launch error kinds; a config with hybrid mode
// starts a hybrid session, whose errors keep their hybrid kinds.
func (s *Service) startSession(ctx context.Context, cwd, name string, reference *Reference, command resolvedCommand, side hybrid.Side, initialPrompt func(id string) (string, error), identity *HandoffIdentity) (*session.Session, bool, error) {
	requestCtx := ctx
	ctx, cancel := context.WithTimeout(ctx, readTimeout)
	defer cancel()
	var selected *Worktree
	if reference != nil {
		r, err := s.repository(reference.RepositoryID)
		if err != nil {
			return nil, false, err
		}
		// Do not hold a repository operation gate while registering an in-flight
		// launch: removal holds that gate and waits for earlier launches to finish.
		snapshot, err := s.snapshot(ctx, r)
		if err != nil {
			return nil, false, err
		}
		if snapshot.Availability != "available" {
			return nil, false, problemf(ErrUnavailable, "%s", snapshot.Error)
		}
		tree, err := findTree(snapshot, reference.WorktreeID)
		if err != nil {
			return nil, false, err
		}
		if !tree.Available || tree.Kind == "bare" {
			return nil, false, problemf(ErrConflict, "所选工作树不可作为启动目录: %s", tree.Error)
		}
		selected = &tree
		if cwd == "" {
			cwd = tree.Path
		}
	}
	if cwd == "" {
		cwd = s.defaultCwd
	}
	path, err := canonical(cwd)
	if err != nil {
		return nil, false, problemf(ErrInvalid, "directory: %v", err)
	}
	stat, err := os.Stat(path)
	if err != nil {
		return nil, false, problemf(ErrInvalid, "directory: %v", err)
	}
	if !stat.IsDir() {
		return nil, false, problemf(ErrInvalid, "path is not a directory")
	}
	if selected != nil && !within(selected.Path, path) {
		return nil, false, problemf(ErrInvalid, "cwd 与所选工作树不一致")
	}
	if err := s.beginStart(path); err != nil {
		return nil, false, err
	}
	defer s.endStart(path)
	association := s.association(ctx, path)
	if selected != nil && (association == nil || association.RepositoryID != reference.RepositoryID || association.WorktreeID != reference.WorktreeID) {
		return nil, false, problemf(ErrConflict, "所选工作树身份已变化或 cwd 属于嵌套仓库，请刷新")
	}
	if err := requestCtx.Err(); err != nil {
		return nil, false, err
	}
	var item *session.Session
	var promptSent bool
	if command.hybrid {
		var runtimeID, threadID, executionID string
		if identity != nil {
			runtimeID, threadID, executionID = identity.RuntimeID, identity.ProviderThreadID, identity.ExecutionID
		}
		item, err = s.hybrids.Create(requestCtx, hybrid.CreateInput{
			RuntimeID: runtimeID, ProviderThreadID: threadID, ContextExecutionID: executionID,
			Cwd: path, Name: name, ConfigID: command.configID, Cmd: command.cmd, Env: command.env, StartSide: side,
			Cols: command.cols, Rows: command.rows, Attention: command.attention,
			InitialPrompt: initialPrompt, InitialPromptSent: &promptSent,
		})
		if err != nil {
			return nil, false, err
		}
	} else {
		item, err = s.manager.Create(session.CreateInput{
			Cwd: path, Cmd: command.cmd, Name: name, Env: command.env, ConfigID: command.configID,
			Cols: command.cols, Rows: command.rows, Attention: command.attention,
		})
		if err != nil {
			return nil, false, sessionCreateError(err)
		}
	}
	s.commitAssociation(item.Info(), association, false)
	return item, promptSent, nil
}

// composeTerminalPrompt adopts staged attachments into session id and builds
// the terminal start prompt with adopted attachment paths appended.
func (s *Service) composeTerminalPrompt(id string, plan createPlan) (string, error) {
	attachments, err := s.adoptUploads(id, plan.attachments)
	if err != nil {
		return "", err
	}
	text, err := s.composePrompt(id, plan)
	if err != nil {
		return "", err
	}
	if len(attachments) > 0 {
		paths := make([]string, len(attachments))
		for i, at := range attachments {
			paths[i] = at.Path
		}
		if text != "" {
			text += "\n\n"
		}
		text += "附件路径：\n" + strings.Join(paths, "\n")
	}
	return text, nil
}

// sendFirstMessage sends the plan's first message to a chat session just
// started with its attachments.
func (s *Service) sendFirstMessage(ctx context.Context, item *session.Session, requestID string, plan createPlan) error {
	id := item.Info().ID
	attachments, err := s.adoptUploads(id, plan.attachments)
	if err != nil {
		return err
	}
	text, err := s.composePrompt(id, plan)
	if err != nil {
		return err
	}
	if requestID == "" {
		requestID = "first-" + id
	}
	_, err = s.hybrids.Prompt(ctx, id, hybrid.PromptInput{ClientRequestID: requestID, Text: text, TitleSource: plan.title, Attachments: attachments})
	return err
}

// composePrompt is the first prompt of session id: the expanded
// instruction, whose {{dir}} becomes the session's artifacts folder, with the
// user's text at its {{user}} or else after a blank line. With ExpandPrompt
// the user's text has its own {{dir}} and {{webmux}} filled first, so what
// fillUser inserts is never read as a reference.
func (s *Service) composePrompt(id string, plan createPlan) (string, error) {
	userText := plan.text
	if plan.expand {
		var err error
		if userText, err = s.fillSessionPlaceholders(id, userText); err != nil {
			return "", err
		}
	}
	instruction := plan.instruction
	if instruction == "" {
		if len(userText) > hybrid.MaxPromptBytes {
			return "", problemf(ErrInvalid, "展开后的消息超过 %d 字节", hybrid.MaxPromptBytes)
		}
		return userText, nil
	}
	// A start with only the instruction has no message to place.
	noMessage := plan.text == ""
	instruction, err := s.fillSessionPlaceholders(id, instruction)
	if err != nil {
		return "", err
	}
	text := fillUser(instruction, userText)
	if noMessage {
		text = strings.TrimSpace(strings.ReplaceAll(instruction, userReference, ""))
	}
	if len(text) > hybrid.MaxPromptBytes {
		return "", problemf(ErrInvalid, "消息与初始化指令合计超过 %d 字节", hybrid.MaxPromptBytes)
	}
	return text, nil
}

// fillSessionPlaceholders replaces {{dir}} in text with session id's
// artifacts folder (created if missing) and {{webmux}} with its WebMux
// brief. The brief goes in last, after {{dir}}: nothing in it is read as a
// reference. Other {{name}} text stays.
func (s *Service) fillSessionPlaceholders(id, text string) (string, error) {
	if strings.Contains(text, artifactsDirPlaceholder) {
		if s.artifacts == nil {
			return "", problemf(ErrUnavailable, "临时文件夹不可用")
		}
		dir, err := s.artifacts.Ensure(id)
		if err != nil {
			return "", problemf(ErrUnavailable, "临时文件夹不可用：%v", err)
		}
		text = strings.ReplaceAll(text, artifactsDirPlaceholder, dir)
	}
	if strings.Contains(text, webmuxPlaceholder) {
		if s.artifacts == nil || s.brief == nil {
			return "", problemf(ErrUnavailable, "WebMux 工具说明不可用")
		}
		dir, err := s.artifacts.Ensure(id)
		if err != nil {
			return "", problemf(ErrUnavailable, "WebMux 工具说明不可用：%v", err)
		}
		text = strings.ReplaceAll(text, webmuxPlaceholder, s.brief(id, dir))
	}
	return text, nil
}

// expandRepositoryInstruction is a repository's initialization instruction
// with its {{name}} references replaced by quick command text ("" for none).
// A reserved or unknown reference is ErrInvalid; {{user}} and {{dir}} stay.
func (s *Service) expandRepositoryInstruction(instruction string) (string, error) {
	if instruction == "" {
		return "", nil
	}
	var commands []QuickCommand
	if s.commands != nil {
		commands = s.commands()
	}
	expanded, missing := expandInstruction(instruction, commands)
	if len(missing) > 0 {
		return "", problemf(ErrInvalid, "初始化指令引用了不存在的快捷指令：%s", strings.Join(missing, "、"))
	}
	return expanded, nil
}

// adoptUploads moves the staged attachments into session id's uploads,
// where its prompts take files from.
func (s *Service) adoptUploads(id string, staged []string) ([]hybrid.Attachment, error) {
	if len(staged) == 0 {
		return nil, nil
	}
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

// LaunchOptions reads the agent options launch config configID offers
// before any chat of it exists: the agent opens a throwaway session in the
// repository's main worktree and reports them (hybrid ProbeAgentOptions).
// A config without the chat view fails with hybrid.ErrNoChatView.
func (s *Service) LaunchOptions(ctx context.Context, configID, repositoryID string) ([]hybrid.ConfigOption, error) {
	command, err := s.resolveCommand(CreateInput{ConfigID: &configID})
	if err != nil {
		return nil, err
	}
	if !command.hybrid {
		return nil, hybrid.ErrNoChatView
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

// applyAgentConfig sets each requested option that differs from what the
// session started with, then the permission mode. An option the agent does
// not offer or refuses is a warning, also shown in the chat; any other
// failure stops the create.
func (s *Service) applyAgentConfig(ctx context.Context, id string, in CreateInput) ([]string, error) {
	warnings := []string{}
	current, err := s.hybrids.Settings(ctx, id)
	if err != nil {
		return warnings, err
	}
	warn := func(text string) error {
		warnings = append(warnings, text)
		return s.hybrids.Notice(ctx, id, text)
	}
	requestID := in.ClientRequestID
	if requestID == "" {
		requestID = id
	}
	optionIDs := make([]string, 0, len(in.AgentConfig))
	for optionID := range in.AgentConfig {
		optionIDs = append(optionIDs, optionID)
	}
	slices.Sort(optionIDs)
	for _, optionID := range optionIDs {
		want := in.AgentConfig[optionID]
		i := slices.IndexFunc(current.Config, func(o hybrid.ConfigOption) bool { return o.ID == optionID })
		if i < 0 {
			if err := warn(fmt.Sprintf("配置项 %s 不可用，未应用 %s", optionID, want)); err != nil {
				return warnings, err
			}
			continue
		}
		option := current.Config[i]
		set := hybrid.ConfigInput{ClientRequestID: derivedRequestID(requestID, "config", optionID), ConfigID: optionID}
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
			ClientRequestID: derivedRequestID(requestID, "permission", ""), Mode: mode,
		}); err != nil {
			return warnings, err
		}
	}
	return warnings, nil
}

// derivedRequestID is the client request ID of one command a create sends
// for request: stable, so a command is never applied twice.
func derivedRequestID(request, command, item string) string {
	sum := sha256.Sum256([]byte(request + "\x00" + command + "\x00" + item))
	return "qs-" + hex.EncodeToString(sum[:16])
}
