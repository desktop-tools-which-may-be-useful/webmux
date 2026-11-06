package api

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/rainchan/webmux/internal/contract"
	"github.com/rainchan/webmux/internal/hybrid"
	"github.com/rainchan/webmux/internal/launch"
	"github.com/rainchan/webmux/internal/session"
	"github.com/rainchan/webmux/internal/wire"
)

// sessionMetaRoutes serves the details and labels of a session.
func (s *Server) sessionMetaRoutes(mux *http.ServeMux) {
	serve(mux, "GET /api/sessions/{id}/details", http.StatusOK, renameFailure, s.details)
	serveIn(mux, "PATCH /api/sessions/{id}/labels", http.StatusOK, renameFailure, func(r *http.Request, req contract.LabelsUpdate) (contract.SessionInfo, error) {
		info, err := s.Manager.UpdateLabels(r.PathValue("id"), req.Set, req.Remove)
		return wire.Session(info), err
	})
}

// callerSession is the session whose process sent r over the local socket,
// or "" for the TCP listener, a caller outside every session, or one whose
// session cannot be named.
func (s *Server) callerSession(r *http.Request) string {
	if !s.local {
		return ""
	}
	pid, ok := r.Context().Value(peerKey{}).(int)
	if !ok || pid <= 0 {
		return ""
	}
	id, _ := s.Manager.ProcessSession(pid)
	return id
}

// adopt records the caller as the parent of a session it just created and
// applies the labels the create request carried.
func (s *Server) adopt(r *http.Request, id string, labels map[string]string) (session.Info, error) {
	s.Manager.SetParent(id, s.callerSession(r))
	if len(labels) > 0 {
		return s.Manager.UpdateLabels(id, labels, nil)
	}
	item, err := s.Manager.Get(id)
	if err != nil {
		return session.Info{}, err
	}
	return item.Info(), nil
}

// detailsTimeout bounds the repository snapshot a details request reads.
const detailsTimeout = 5 * time.Second

func (s *Server) details(r *http.Request) (contract.SessionDetails, error) {
	item, err := s.Manager.Get(r.PathValue("id"))
	if err != nil {
		return contract.SessionDetails{}, err
	}
	info := item.Info()
	out := contract.SessionDetails{
		Session:  wire.Session(info),
		ParentID: info.ParentID,
		Labels:   info.Labels,
		Children: s.Manager.Children(info.ID),
		Forwards: []contract.Forward{},
	}
	if out.Labels == nil {
		out.Labels = map[string]string{}
	}
	if s.LocalURL != "" {
		link := strings.TrimSuffix(s.LocalURL, "/") + "/s/" + info.ID
		out.Link = &link
	}
	if info.Workspace != nil {
		ctx, cancel := context.WithTimeout(r.Context(), detailsTimeout)
		s.detailsWorkspace(ctx, info.Workspace, &out)
		cancel()
	}
	if info.ConfigID != "" && s.Launch != nil {
		config, err := s.Launch.Get(info.ConfigID)
		switch {
		case err == nil:
			out.LaunchConfig = &contract.DetailsLaunchConfig{ID: config.ID, Name: config.Name, Icon: config.Icon, Hybrid: config.Hybrid}
		case errors.Is(err, launch.ErrNotFound):
			out.LaunchConfigMissing = true
		}
	}
	if s.Hybrid != nil {
		if record, err := s.Hybrid.Record(info.ID); err == nil {
			value := wire.HybridRecord(record, s.configMissing(record.ConfigID))
			out.Hybrid = &value
		} else if !errors.Is(err, hybrid.ErrNotFound) && !errors.Is(err, hybrid.ErrUnavailable) {
			return contract.SessionDetails{}, err
		}
	}
	if s.Artifacts != nil {
		if dir, err := s.Artifacts.Existing(info.ID); err == nil {
			out.ArtifactsDir = &dir
		}
	}
	if s.Forward != nil {
		for _, f := range s.Forward.List() {
			if f.Session == info.ID {
				out.Forwards = append(out.Forwards, wire.Forward(f))
			}
		}
	}
	return out, nil
}

// detailsWorkspace names the repository and worktree of a session. A
// repository that cannot be read leaves both unset.
func (s *Server) detailsWorkspace(ctx context.Context, ref *session.Workspace, out *contract.SessionDetails) {
	repositories, err := s.Workspace.List(ctx)
	if err != nil {
		return
	}
	for _, repository := range repositories {
		if repository.ID != ref.RepositoryID {
			continue
		}
		out.Repository = &contract.DetailsRepository{ID: repository.ID, Name: repository.Name, Path: repository.AnchorPath}
		for _, tree := range repository.Worktrees {
			if tree.ID == ref.WorktreeID {
				out.Worktree = &contract.DetailsWorktree{ID: tree.ID, Branch: tree.Branch, Path: tree.Path, Main: tree.Kind == "main"}
			}
		}
		return
	}
}
