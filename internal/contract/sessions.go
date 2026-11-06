package contract

import "time"

// The REST and WebSocket shapes below mirror web/src/schemas.ts. Field
// names, omitempty and nil-versus-empty behavior are part of the wire
// contract and are pinned by internal/api/testdata/wire.

// SessionStatus is the display state of a session.
type SessionStatus string

const (
	SessionStatusRunning SessionStatus = "running"
	SessionStatusWaiting SessionStatus = "waiting"
	SessionStatusIdle    SessionStatus = "idle"
	SessionStatusExited  SessionStatus = "exited"
)

// AttentionKind names the signal that a session needs the user.
type AttentionKind string

const (
	AttentionKindQuestion   AttentionKind = "question"
	AttentionKindPermission AttentionKind = "permission"
	AttentionKindDone       AttentionKind = "done"
	AttentionKindError      AttentionKind = "error"
	AttentionKindNotify     AttentionKind = "notify"
	AttentionKindBell       AttentionKind = "bell"
)

// Valid reports whether k is one of the named kinds.
func (k AttentionKind) Valid() bool {
	switch k {
	case AttentionKindQuestion, AttentionKindPermission, AttentionKindError, AttentionKindDone, AttentionKindNotify, AttentionKindBell:
		return true
	}
	return false
}

// AttentionSource identifies the component that raised an attention signal.
type AttentionSource string

const (
	AttentionSourceAgent    AttentionSource = "agent"
	AttentionSourceTerminal AttentionSource = "terminal"
)

// SessionInfo is one session in GET /api/sessions, POST /api/sessions,
// /ws/sessions events and the 0x02 terminal frame.
type SessionInfo struct {
	ID           string              `json:"id"`
	Generation   string              `json:"generation"`
	Title        string              `json:"title"`
	Cwd          string              `json:"cwd"`
	Cmd          []string            `json:"cmd"`
	Status       SessionStatus       `json:"status"`
	ExitCode     *int                `json:"exitCode"`
	Preview      string              `json:"preview"`
	CreatedAt    time.Time           `json:"createdAt"`
	LastActiveAt time.Time           `json:"lastActiveAt"`
	Cols         uint16              `json:"cols"`
	Rows         uint16              `json:"rows"`
	Clients      int                 `json:"clients"`
	Workspace    *SessionWorkspace   `json:"workspace"`
	Attention    *SessionAttention   `json:"attention"`
	ReportedCwd  *SessionReportedCwd `json:"reportedCwd"`
	// ConfigID is the saved launch config used at creation, if any. It is
	// omitted for sessions started from an explicit command.
	ConfigID string `json:"configId,omitempty"`
	// Hybrid describes a hybrid session; ordinary sessions omit it.
	Hybrid *SessionHybrid `json:"hybrid,omitempty"`
	// ListeningPorts are the loopback or wildcard TCP ports the session's
	// process tree listens on, ascending; empty where detection is
	// unsupported.
	ListeningPorts []ListeningPort `json:"listeningPorts"`
	// ParentID is the session whose process created this one through the
	// local API; sessions created elsewhere omit it.
	ParentID string `json:"parentId,omitempty"`
	// Labels are caller-defined key-value pairs; empty is omitted.
	Labels map[string]string `json:"labels,omitempty"`
}

// SessionWorkspace is the registered worktree a session was started in.
type SessionWorkspace struct {
	RepositoryID string `json:"repositoryId"`
	WorktreeID   string `json:"worktreeId"`
	WorktreePath string `json:"worktreePath"`
}

// SessionAttention is a latched signal that the session needs the user.
type SessionAttention struct {
	Kind   AttentionKind   `json:"kind"`
	Source AttentionSource `json:"source"`
	Since  time.Time       `json:"since"`
}

// SessionReportedCwd is the directory a program reported through OSC 7.
type SessionReportedCwd struct {
	Path   string    `json:"path"`
	Host   string    `json:"host"`
	Remote bool      `json:"remote"`
	Since  time.Time `json:"since"`
}

// WorkspaceReference selects a registered worktree.
type WorkspaceReference struct {
	RepositoryID string `json:"repositoryId"`
	WorktreeID   string `json:"worktreeId"`
}

// CreateSessionRequest is the body of POST /api/sessions. ConfigID excludes
// Cmd and Env. StartSide (tui or acp, default tui) is only valid with a
// config that has hybrid mode. omitempty only shapes what Go clients send;
// the server treats an absent field like its zero value.
type CreateSessionRequest struct {
	Cwd       string              `json:"cwd"`
	Cmd       []string            `json:"cmd,omitempty"`
	Name      string              `json:"name,omitempty"`
	Workspace *WorkspaceReference `json:"workspace,omitempty"`
	Env       map[string]string   `json:"env,omitempty"`
	ConfigID  *string             `json:"configId,omitempty"`
	StartSide HybridSide          `json:"startSide,omitempty"`
	// Prompt is the user's first message, sent once the session is up: as
	// the first chat prompt on the chat side, otherwise typed into the
	// terminal with Enter. UseInitInstruction puts the repository's
	// initialization instruction before it; it needs Workspace.
	Prompt             string `json:"prompt,omitempty"`
	UseInitInstruction bool   `json:"useInitInstruction,omitempty"`
	// Labels are set on the new session; invalid labels reject the request
	// before anything starts.
	Labels map[string]string `json:"labels,omitempty"`
}

// LabelsUpdate is the body of PATCH /api/sessions/{id}/labels: Set adds or
// replaces labels, then Remove deletes keys, as one change. It answers 200
// with the SessionInfo.
type LabelsUpdate struct {
	Set    map[string]string `json:"set,omitempty"`
	Remove []string          `json:"remove,omitempty"`
}

// SessionDetails is GET /api/sessions/{id}/details: the session's metadata
// gathered from every service. It never carries the token, launch
// environment values or another session's output.
type SessionDetails struct {
	Session             SessionInfo          `json:"session"`
	Link                *string              `json:"link"`
	Repository          *DetailsRepository   `json:"repository"`
	Worktree            *DetailsWorktree     `json:"worktree"`
	LaunchConfig        *DetailsLaunchConfig `json:"launchConfig"`
	LaunchConfigMissing bool                 `json:"launchConfigMissing,omitempty"`
	Hybrid              *HybridRecord        `json:"hybrid"`
	// ArtifactsDir is the session's artifacts folder when it exists; the
	// endpoint never creates it.
	ArtifactsDir *string           `json:"artifactsDir"`
	Forwards     []Forward         `json:"forwards"`
	ParentID     string            `json:"parentId,omitempty"`
	Labels       map[string]string `json:"labels"`
	// Children are the sessions whose ParentID is this session, oldest
	// first.
	Children []string `json:"children"`
}

// DetailsRepository names the registered repository of a session.
type DetailsRepository struct {
	ID   string `json:"id"`
	Name string `json:"name"`
	Path string `json:"path"`
}

// DetailsWorktree names the registered worktree of a session.
type DetailsWorktree struct {
	ID     string  `json:"id"`
	Branch *string `json:"branch"`
	Path   string  `json:"path"`
	Main   bool    `json:"main"`
}

// DetailsLaunchConfig names the saved launch config a session came from.
type DetailsLaunchConfig struct {
	ID     string `json:"id"`
	Name   string `json:"name"`
	Icon   string `json:"icon,omitempty"`
	Hybrid bool   `json:"hybrid"`
}

// RenameSessionRequest is the body of PATCH /api/sessions/{id}. An empty
// name hands the title back to the terminal or agent.
type RenameSessionRequest struct {
	Name string `json:"name"`
}
