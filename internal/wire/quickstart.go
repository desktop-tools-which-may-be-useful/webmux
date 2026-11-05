package wire

import (
	"github.com/rainchan/webmux/internal/contract"
	"github.com/rainchan/webmux/internal/workspace"
)

func QuickStartInput(r contract.QuickStartRequest) workspace.QuickStartInput {
	return workspace.QuickStartInput{
		ClientRequestID: r.ClientRequestID, RepositoryID: r.RepositoryID, ConfigID: r.ConfigID,
		Base: r.Base, Branch: r.Branch, Text: r.Text, Config: r.Config, PermissionMode: string(r.PermissionMode), UseInitInstruction: r.UseInitInstruction, StartSide: string(r.StartSide),
		Attachments: each(r.Attachments, func(a contract.ChatAttachment) string { return a.Path }),
	}
}

func QuickStartResult(r workspace.QuickStartResult) contract.QuickStartResult {
	return contract.QuickStartResult{SessionID: r.SessionID, Worktree: Worktree(r.Worktree), ConfigWarnings: required(r.ConfigWarnings)}
}

// QuickStartWorktree converts the worktree a failed quick start created;
// nil stays nil.
func QuickStartWorktree(w *workspace.Worktree) *contract.Worktree {
	if w == nil {
		return nil
	}
	out := Worktree(*w)
	return &out
}
