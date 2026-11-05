package contract

// QuickStartRequest is the body of POST /api/quick-starts: in one request
// the server creates a worktree on new branch Branch from Base, starts a
// chat-first hybrid session of launch config ConfigID in it, applies Config
// (option ID to value: the select value, or "true"/"false" for a boolean)
// and PermissionMode ("" keeps the default), then sends Text as the first
// prompt, after the repository's initialization instruction if it has one.
// ClientRequestID dedupes retries and is the first prompt's request ID.
type QuickStartRequest struct {
	ClientRequestID string             `json:"clientRequestId"`
	RepositoryID    string             `json:"repositoryId"`
	ConfigID        string             `json:"configId"`
	Base            string             `json:"base"`
	Branch          string             `json:"branch"`
	Text            string             `json:"text"`
	Config          map[string]string  `json:"config"`
	PermissionMode  ChatPermissionMode `json:"permissionMode"`
	// Attachments are files returned by POST /api/quick-starts/uploads;
	// only their path is read. They go with the first prompt.
	Attachments []ChatAttachment `json:"attachments,omitempty"`
}

// QuickStartOptionsRequest is the body of POST /api/quick-starts/options:
// the agent of launch config ConfigID reports the options it offers in a
// throwaway session in repository RepositoryID's main worktree. The 200
// body is []ChatConfig, also remembered as the config's chatOptions.
type QuickStartOptionsRequest struct {
	RepositoryID string `json:"repositoryId"`
	ConfigID     string `json:"configId"`
}

// QuickStartResult is the 201 body of POST /api/quick-starts.
// ConfigWarnings name each Config item the agent did not take.
type QuickStartResult struct {
	SessionID      string   `json:"sessionId"`
	Worktree       Worktree `json:"worktree"`
	ConfigWarnings []string `json:"configWarnings"`
}

// QuickStartFailure is the body of every failed POST /api/quick-starts.
// Step is precheck (nothing was created), worktree, session, config or
// prompt. Reason, Eligibility and Connector are those of a HybridConflict;
// Reason is conflict for a branch or path that already exists. Worktree and
// SessionID name what had been created; nothing is rolled back.
type QuickStartFailure struct {
	Error       string          `json:"error"`
	Step        string          `json:"step"`
	Reason      string          `json:"reason,omitempty"`
	Eligibility string          `json:"eligibility,omitempty"`
	Connector   *AgentConnector `json:"connector,omitempty"`
	Worktree    *Worktree       `json:"worktree,omitempty"`
	SessionID   string          `json:"sessionId,omitempty"`
}
