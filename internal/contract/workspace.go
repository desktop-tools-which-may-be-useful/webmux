package contract

import "time"

// WorkspaceCapability reports whether Git and the repository registry work.
type WorkspaceCapability struct {
	GitAvailable      bool   `json:"gitAvailable"`
	RegistryAvailable bool   `json:"registryAvailable"`
	GitError          string `json:"gitError"`
	RegistryError     string `json:"registryError"`
}

// Repository is a registered Git repository and its worktrees.
// InitInstruction prefixes the first prompt of a quick start; "" for none.
type Repository struct {
	ID              string     `json:"id"`
	Name            string     `json:"name"`
	CommonDir       string     `json:"commonDir"`
	AnchorPath      string     `json:"anchorPath"`
	Availability    string     `json:"availability"`
	Error           string     `json:"error"`
	CheckedAt       time.Time  `json:"checkedAt"`
	Worktrees       []Worktree `json:"worktrees"`
	InitInstruction string     `json:"initInstruction"`
}

type Worktree struct {
	ID           string  `json:"id"`
	RepositoryID string  `json:"repositoryId"`
	Path         string  `json:"path"`
	Kind         string  `json:"kind"`
	Branch       *string `json:"branch"`
	Head         *string `json:"head"`
	Detached     bool    `json:"detached"`
	Locked       bool    `json:"locked"`
	LockReason   string  `json:"lockReason"`
	Prunable     bool    `json:"prunable"`
	Available    bool    `json:"available"`
	Dirty        *bool   `json:"dirty"`
	HasUntracked bool    `json:"hasUntracked"`
	HasIgnored   bool    `json:"hasIgnored"`
	Error        string  `json:"error"`
}

type Branch struct {
	Name          string   `json:"name"`
	Head          string   `json:"head"`
	WorktreePaths []string `json:"worktreePaths"`
}

type BranchBase struct {
	Name    string `json:"name"`
	Head    string `json:"head"`
	Subject string `json:"subject"`
}

// Branches is the body of GET /api/repositories/{id}/branches.
type Branches struct {
	Branches           []Branch     `json:"branches"`
	Bases              []BranchBase `json:"bases"`
	DefaultBase        *string      `json:"defaultBase"`
	DefaultBaseSubject string       `json:"defaultBaseSubject"`
	DefaultBranch      *string      `json:"defaultBranch"`
	DefaultParent      string       `json:"defaultParent"`
}

// Commit is one recent commit offered as a worktree start point.
type Commit struct {
	Hash        string `json:"hash"`
	Subject     string `json:"subject"`
	Author      string `json:"author"`
	CommittedAt string `json:"committedAt"`
}

// Commits is the body of GET /api/repositories/{id}/commits?from=<oid>.
type Commits struct {
	Commits []Commit `json:"commits"`
}

// RefreshBranchBaseRequest selects the local branch whose upstream is fetched.
type RefreshBranchBaseRequest struct {
	Branch string `json:"branch"`
}

// RefreshBranchBaseResponse describes a remote base or a local fallback.
// UpstreamHead is the fetched upstream tip, also when the local base is kept
// because the branch is ahead or diverged; it is empty when no fetch succeeded.
type RefreshBranchBaseResponse struct {
	Branch       string `json:"branch"`
	Base         string `json:"base"`
	BaseSubject  string `json:"baseSubject"`
	BaseSource   string `json:"baseSource"`
	Upstream     string `json:"upstream"`
	UpstreamHead string `json:"upstreamHead"`
	FetchWarning string `json:"fetchWarning"`
}

// RegisterRepositoryRequest is the body of POST /api/repositories. An
// already registered repository keeps its instruction.
type RegisterRepositoryRequest struct {
	Path            string `json:"path"`
	InitInstruction string `json:"initInstruction,omitempty"`
}

// UpdateRepositoryRequest is the body of PATCH /api/repositories/{id}; an
// empty instruction clears it. It answers 200 with the Repository.
type UpdateRepositoryRequest struct {
	InitInstruction string `json:"initInstruction"`
}

// CreateWorktreeRequest is the body of POST /api/repositories/{id}/worktrees.
type CreateWorktreeRequest struct {
	Path   string `json:"path"`
	Mode   string `json:"mode"`
	Branch string `json:"branch"`
	Base   string `json:"base"`
}

// RemovalPreviewRequest is the empty body of a removal preview.
type RemovalPreviewRequest struct{}

// RemovalPreview describes what removing a worktree would affect.
type RemovalPreview struct {
	ConfirmationID string        `json:"confirmationId"`
	ExpiresAt      time.Time     `json:"expiresAt"`
	Worktree       Worktree      `json:"worktree"`
	Sessions       []SessionInfo `json:"sessions"`
	// Archived are the closed and interrupted hybrid records in the
	// worktree; a successful removal deletes them.
	Archived     []HybridRecord `json:"archived"`
	Blockers     []string       `json:"blockers"`
	ForceAllowed bool           `json:"forceAllowed"`
}

// RemoveWorktreeRequest confirms a previewed removal.
type RemoveWorktreeRequest struct {
	ConfirmationID string `json:"confirmationId"`
	Mode           string `json:"mode"`
	ConfirmPath    string `json:"confirmPath"`
	CloseSessions  bool   `json:"closeSessions"`
}
