package api

import (
	"net/http"

	"github.com/rainchan/webmux/internal/contract"
	"github.com/rainchan/webmux/internal/wire"
)

func (s *Server) workspaceRoutes(mux *http.ServeMux) {
	const repo = "/api/repositories/{id}"
	serve(mux, "GET /api/repositories", http.StatusOK, workspaceFailure, func(r *http.Request) ([]contract.Repository, error) {
		repositories, err := s.Workspace.List(r.Context())
		return wire.Repositories(repositories), err
	})
	mux.HandleFunc("POST /api/repositories", func(w http.ResponseWriter, r *http.Request) {
		var req contract.RegisterRepositoryRequest
		if !decode(w, r, &req) {
			return
		}
		repository, created, err := s.Workspace.Register(r.Context(), req.Path)
		if err != nil {
			workspaceFailure(w, err)
			return
		}
		status := http.StatusOK
		if created {
			status = http.StatusCreated
		}
		reply(w, status, wire.Repository(repository))
	})
	serveDone(mux, "DELETE "+repo, workspaceFailure, func(r *http.Request) error {
		return s.Workspace.Unregister(r.PathValue("id"))
	})
	serveIn(mux, "PATCH "+repo, http.StatusOK, workspaceFailure, func(r *http.Request, req contract.UpdateRepositoryRequest) (contract.Repository, error) {
		repository, err := s.Workspace.SetInitInstruction(r.Context(), r.PathValue("id"), req.InitInstruction)
		return wire.Repository(repository), err
	})
	serve(mux, "GET "+repo+"/worktrees", http.StatusOK, workspaceFailure, func(r *http.Request) (contract.Repository, error) {
		repository, err := s.Workspace.Refresh(r.Context(), r.PathValue("id"))
		return wire.Repository(repository), err
	})
	serve(mux, "GET "+repo+"/branches", http.StatusOK, workspaceFailure, func(r *http.Request) (contract.Branches, error) {
		branches, err := s.Workspace.Branches(r.Context(), r.PathValue("id"))
		return wire.Branches(branches), err
	})
	serve(mux, "GET "+repo+"/commits", http.StatusOK, workspaceFailure, func(r *http.Request) (contract.Commits, error) {
		commits, err := s.Workspace.RecentCommits(r.Context(), r.PathValue("id"), r.URL.Query()["from"])
		return wire.Commits(commits), err
	})
	serveIn(mux, "POST "+repo+"/branches/refresh", http.StatusOK, workspaceFailure, func(r *http.Request, req contract.RefreshBranchBaseRequest) (contract.RefreshBranchBaseResponse, error) {
		result, err := s.Workspace.RefreshBranchBase(r.Context(), r.PathValue("id"), req.Branch)
		return wire.RefreshBranchBase(result), err
	})
	serveIn(mux, "POST "+repo+"/worktrees", http.StatusCreated, workspaceFailure, func(r *http.Request, req contract.CreateWorktreeRequest) (contract.Worktree, error) {
		tree, err := s.Workspace.CreateWorktree(r.Context(), r.PathValue("id"), wire.CreateWorktree(req))
		return wire.Worktree(tree), err
	})
	serveIn(mux, "POST "+repo+"/worktrees/{worktreeId}/removal-preview", http.StatusOK, workspaceFailure, func(r *http.Request, _ contract.RemovalPreviewRequest) (contract.RemovalPreview, error) {
		preview, err := s.Workspace.PreviewRemoval(r.Context(), r.PathValue("id"), r.PathValue("worktreeId"))
		return wire.Preview(preview, s.archived), err
	})
	serveInDone(mux, "POST "+repo+"/worktrees/{worktreeId}/remove", workspaceFailure, func(r *http.Request, req contract.RemoveWorktreeRequest) error {
		forgotten, err := s.Workspace.RemoveWorktree(r.Context(), r.PathValue("id"), r.PathValue("worktreeId"), wire.RemoveWorktree(req))
		// Records of sessions in the removed directory are gone, so their
		// folders cannot come back either.
		for _, id := range forgotten {
			s.Artifacts.Remove(id)
		}
		return err
	})
}
