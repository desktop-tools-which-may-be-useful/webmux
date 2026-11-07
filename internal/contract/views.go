package contract

import "time"

// A session has one or two views: the terminal, and for a session started
// from a launch config with chat enabled, an ACP chat view. See the
// unify-session-surface OpenSpec change for the behaviour behind these
// shapes.

// SessionView is a view of a session.
type SessionView string

const (
	ViewTerminal SessionView = "terminal"
	ViewChat     SessionView = "chat"
)

// SessionState is the lifecycle of a session. Ordinary sessions are always
// active; a session with a chat view becomes closed or interrupted and then
// lives on only as an archive record.
type SessionState string

const (
	SessionStateActive      SessionState = "active"
	SessionStateClosed      SessionState = "closed"
	SessionStateInterrupted SessionState = "interrupted"
)

// TransitionPhase is the current step of a view change.
type TransitionPhase string

const (
	PhaseInterrupting TransitionPhase = "interrupting"
	// PhaseAwaitingChoice: the terminal UI did not return to idle within the
	// interrupt timeout; the user chooses wait, force or cancel.
	PhaseAwaitingChoice TransitionPhase = "awaiting_choice"
	PhaseCancelling     TransitionPhase = "cancelling"
	PhaseStopping       TransitionPhase = "stopping"
	PhaseStarting       TransitionPhase = "starting"
	PhaseLoading        TransitionPhase = "loading"
	PhaseRollingBack    TransitionPhase = "rolling_back"
)

// SessionTransition is SessionInfo.transition: a view change in progress.
type SessionTransition struct {
	To    SessionView     `json:"to"`
	Phase TransitionPhase `json:"phase"`
}

// SessionAgent is SessionInfo.agent: the agent behind a session's chat view.
type SessionAgent struct {
	Name string `json:"name"`
	// Title is the agent's display name.
	Title string `json:"title"`
	// Icon is the agent's brand glyph, a launch-config icon key.
	Icon string `json:"icon"`
	// SessionID is null until the agent's own session ID is known.
	SessionID *string `json:"sessionId"`
	// Work is a one-line summary of what the agent is doing (the running
	// turn's current tool call or plan step); empty outside a turn.
	Work string `json:"work,omitempty"`
}

// SessionArchive is SessionInfo.archive, present only on a closed or
// interrupted session: what its archive record adds. It never carries
// environment values, messages or queued text.
type SessionArchive struct {
	ConfigMissing           bool     `json:"configMissing"`
	PreviousAgentSessionIDs []string `json:"previousAgentSessionIds"`
	// Activity is what an interrupted session was doing: idle, turn or
	// switching; a turn or switch may not have finished.
	Activity string `json:"activity,omitempty"`
	// QueuedCount is how many queued prompts were lost when the session was
	// interrupted; the text itself is never stored.
	QueuedCount   int        `json:"queuedCount"`
	InterruptedAt *time.Time `json:"interruptedAt"`
	UpdatedAt     time.Time  `json:"updatedAt"`
}

// CloseMoved is the WebSocket close code for "the session moved to its
// other view". It is terminal: clients do not reconnect.
const CloseMoved = 4409

// Choices that answer a view change paused in PhaseAwaitingChoice.
const (
	ViewChoiceWait   = "wait"
	ViewChoiceForce  = "force"
	ViewChoiceCancel = "cancel"
)

// ViewChangeRequest is the body of POST /api/sessions/{id}/view. Choice
// answers an interrupt timeout ("wait", "force" or "cancel") instead of
// starting a change.
type ViewChangeRequest struct {
	ClientRequestID string      `json:"clientRequestId"`
	To              SessionView `json:"to,omitempty"`
	Choice          string      `json:"choice,omitempty"`
	// AgentSessionID picks one of the candidates a 409 session_ambiguous
	// offered; it is accepted only with To "chat".
	AgentSessionID string `json:"agentSessionId,omitempty"`
}

// ViewChangeResult answers a completed view change. Draft is the merged
// text of queued prompts returned to the requesting device when moving to
// the terminal; Dropped lists launch arguments the resume command did not
// keep; Unsupported lists arguments the chat view cannot apply.
type ViewChangeResult struct {
	View        SessionView `json:"view"`
	Draft       string      `json:"draft,omitempty"`
	Dropped     []string    `json:"dropped"`
	Unsupported []string    `json:"unsupported"`
}

// ResumeRequest is the body of POST /api/sessions/{id}/resume. View
// defaults to the view the session was archived on; AgentSessionID picks a
// candidate after a 409 listing them; ConfirmWithoutEnv accepts resuming
// after the launch config was deleted.
type ResumeRequest struct {
	ClientRequestID   string      `json:"clientRequestId"`
	View              SessionView `json:"view,omitempty"`
	AgentSessionID    string      `json:"agentSessionId,omitempty"`
	ConfirmWithoutEnv bool        `json:"confirmWithoutEnv,omitempty"`
}

// AgentCandidate is an agent session offered when a session's agent
// session ID can no longer be found.
type AgentCandidate struct {
	AgentSessionID string    `json:"agentSessionId"`
	Title          string    `json:"title"`
	UpdatedAt      time.Time `json:"updatedAt"`
}

// SessionConflict is the 409 body of every session, view, input,
// interrupt, resume and chat command. Reason is a stable code: switching,
// wrong_view, no_chat_view, not_active, not_archived, input_failed,
// not_eligible, answered, sent, steering_unavailable, busy, session_missing,
// session_ambiguous, connector_missing, occupied_external, config_missing or
// limit. Eligibility carries the not_eligible cause; Candidates the
// session_missing and session_ambiguous alternatives; Connector the
// connector_missing connector; Submission the input_failed receipt.
type SessionConflict struct {
	Error       string            `json:"error"`
	Reason      string            `json:"reason"`
	Eligibility string            `json:"eligibility,omitempty"`
	Candidates  []AgentCandidate  `json:"candidates,omitempty"`
	Connector   *AgentConnector   `json:"connector,omitempty"`
	Submission  *SubmissionStatus `json:"submission,omitempty"`
}

// AgentConnector is the third-party program that provides an agent's ACP
// side (Codex's codex-acp). It comes from the agent profile only: it never
// says where, or whether, the program is installed.
type AgentConnector struct {
	Executable     string `json:"executable"`
	Title          string `json:"title"`
	InstallCommand string `json:"installCommand"`
	DocsURL        string `json:"docsUrl"`
}
