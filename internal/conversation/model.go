// Package conversation owns durable provider-neutral identities and history.
// Runtime sessions and provider processes remain ephemeral.
package conversation

import (
	"errors"
	"time"
)

var (
	ErrUnavailable = errors.New("conversation storage unavailable")
	ErrNotFound    = errors.New("conversation object not found")
	ErrConflict    = errors.New("conversation conflict")
	ErrInvalid     = errors.New("invalid conversation input")
)

type Conversation struct {
	ID, ParentID, Relationship    string
	Cwd, RepositoryID, WorktreeID string
	ActiveProviderThreadID        string
	NextRunOrdinal                int64
	HistoryOrigin                 string
	CreatedAt, UpdatedAt          time.Time
}

type ProviderThread struct {
	ID, ConversationID, ConfigID, AgentName, AgentSessionID, Status string
	LastDeliveredRunOrdinal                                         int64
	ContextUsage, ContextWindow, AutoCompactThreshold               int64
	Config                                                          map[string]string
	CreatedAt, UpdatedAt                                            time.Time
}

type Run struct {
	ID, ConversationID, ProviderThreadID, PromptID, Status string
	Ordinal                                                int64
	CreatedAt, UpdatedAt                                   time.Time
}

type ContextHandoff struct {
	ID, SourceConversationID, TargetConversationID, TargetProviderThreadID string
	Strategy                                                               string
	FromRunOrdinal, ToRunOrdinal, Revision                                 int64
	SelectedItemIDs, OmittedItemIDs                                        []string
	CreatedAt                                                              time.Time
}

type HandoffDelivery struct {
	ID, HandoffID, TargetProviderThreadID, NativeThreadID, Status, ErrorClass string
	ItemIDs                                                                   []string
	UpdatedAt                                                                 time.Time
}

type CommandReceipt struct {
	ConversationID, ClientRequestID, RequestHash         string
	HandoffID, DeliveryID, RuntimeID, Result, ErrorClass string
	CreatedAt                                            time.Time
}

// RuntimeInput contains no argv, environment or message bodies. Existing
// hybrid records import identity only; their pre-upgrade history is unknown.
type RuntimeInput struct {
	RuntimeID, Cwd, RepositoryID, WorktreeID    string
	ConfigID, AgentName, AgentSessionID, Status string
	Config                                      map[string]string
	HistoryOrigin                               string
	CreatedAt                                   time.Time
}

type Binding struct{ ConversationID, ProviderThreadID string }

type HandoffInput struct {
	SourceConversationID, TargetConversationID, TargetProviderThreadID string
	Strategy                                                           string
	FromRunOrdinal, ToRunOrdinal                                       int64
	SelectedItemIDs, OmittedItemIDs                                    []string
	ClientRequestID, RequestHash                                       string
}
