package api

import (
	"errors"
	"net/http"
	"strings"

	"github.com/rainchan/webmux/internal/agentguide"
	"github.com/rainchan/webmux/internal/contract"
)

// guideRoutes serves the shell guide llms.txt-style and the {{webmux}}
// brief. The guide pages lie outside /api/, so like the static app they
// need no token on the TCP listener; there they carry nothing about the
// running instance. The local socket adds its version, executable and
// socket.
func (s *Server) guideRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /llms.txt", func(w http.ResponseWriter, r *http.Request) {
		guideText(w, "text/plain; charset=utf-8", agentguide.Index(s.guideInstance()))
	})
	mux.HandleFunc("GET /llms-full.txt", func(w http.ResponseWriter, r *http.Request) {
		guideText(w, "text/plain; charset=utf-8", agentguide.Full(s.guideInstance()))
	})
	mux.HandleFunc("GET /llms/{page}", func(w http.ResponseWriter, r *http.Request) {
		name, ok := strings.CutSuffix(r.PathValue("page"), ".md")
		page, found := agentguide.Page(name)
		if !ok || !found {
			http.NotFound(w, r)
			return
		}
		guideText(w, "text/markdown; charset=utf-8", page)
	})
	mux.HandleFunc("POST /api/sessions/{id}/agent-guide", s.agentGuide)
}

// guideInstance is the running instance on the local socket, nil on TCP.
func (s *Server) guideInstance() *agentguide.Instance {
	if !s.local {
		return nil
	}
	return s.Guide
}

func guideText(w http.ResponseWriter, contentType, text string) {
	w.Header().Set("Content-Type", contentType)
	w.Header().Set("Cache-Control", "no-store")
	_, _ = w.Write([]byte(text))
}

func (s *Server) agentGuide(w http.ResponseWriter, r *http.Request) {
	if s.Guide == nil {
		failure(w, http.StatusServiceUnavailable, errGuideUnavailable)
		return
	}
	id := r.PathValue("id")
	if _, err := s.Manager.Get(id); err != nil {
		failure(w, http.StatusNotFound, err)
		return
	}
	dir, err := s.Artifacts.Ensure(id)
	if err != nil {
		artifactsFailure(w, err)
		return
	}
	out := contract.AgentGuidePrompt{
		Prompt:   s.Guide.Brief(agentguide.BriefInput{SessionID: id, Dir: dir}),
		GuideURL: s.Guide.GuideURL(),
		Dir:      dir,
	}
	if bin := s.Guide.Bin; bin != "" {
		out.Bin = &bin
	}
	if socket := s.Guide.Socket(); socket != "" {
		out.Socket = &socket
	}
	reply(w, http.StatusOK, out)
}

var errGuideUnavailable = errors.New("WebMux 工具说明不可用")
