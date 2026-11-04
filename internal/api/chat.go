package api

import (
	"errors"
	"net/http"

	"github.com/rainchan/webmux/internal/contract"
	"github.com/rainchan/webmux/internal/hub"
	"github.com/rainchan/webmux/internal/hybrid"
	"github.com/rainchan/webmux/internal/launch"
	"github.com/rainchan/webmux/internal/wire"
)

// chatRoutes registers a hybrid session's chat channel: the event stream
// and the commands. Commands answer 204 unless they return a result.
func (s *Server) chatRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /ws/s/{id}/chat", s.chatWS)
	const base = "/api/sessions/{id}/chat/"
	serveIn(mux, "POST "+base+"prompts", http.StatusOK, chatFailure, func(r *http.Request, req contract.ChatPromptRequest) (contract.ChatPromptResult, error) {
		h, err := s.hybrids()
		if err != nil {
			return contract.ChatPromptResult{}, err
		}
		result, err := h.Prompt(r.Context(), r.PathValue("id"), wire.ChatPromptInput(req))
		return wire.ChatPromptResult(result), err
	})
	serveInDone(mux, "PATCH "+base+"queue/{qid}", chatFailure, chatCommand(s, func(h *hybrid.Service, r *http.Request, req contract.ChatQueueRequest) error {
		return h.EditQueued(r.Context(), r.PathValue("id"), r.PathValue("qid"), wire.ChatQueueEdit(req))
	}))
	serveInDone(mux, "DELETE "+base+"queue/{qid}", chatFailure, chatCommand(s, func(h *hybrid.Service, r *http.Request, req contract.ChatQueueRequest) error {
		return h.RemoveQueued(r.Context(), r.PathValue("id"), req.ClientRequestID, r.PathValue("qid"))
	}))
	serveInDone(mux, "POST "+base+"queue/{qid}/send-now", chatFailure, chatCommand(s, func(h *hybrid.Service, r *http.Request, req contract.ChatQueueRequest) error {
		return h.SendQueuedNow(r.Context(), r.PathValue("id"), req.ClientRequestID, r.PathValue("qid"))
	}))
	serveInDone(mux, "POST "+base+"queue/resume", chatFailure, chatCommand(s, func(h *hybrid.Service, r *http.Request, req contract.ChatQueueRequest) error {
		return h.ResumeQueue(r.Context(), r.PathValue("id"), req.ClientRequestID)
	}))
	serveInDone(mux, "POST "+base+"queue/clear", chatFailure, chatCommand(s, func(h *hybrid.Service, r *http.Request, req contract.ChatQueueRequest) error {
		return h.ClearQueue(r.Context(), r.PathValue("id"), req.ClientRequestID)
	}))
	serveInDone(mux, "POST "+base+"cancel", chatFailure, chatCommand(s, func(h *hybrid.Service, r *http.Request, req contract.ChatCommandRequest) error {
		return h.Cancel(r.Context(), r.PathValue("id"), req.ClientRequestID)
	}))
	serveInDone(mux, "POST "+base+"requests/{requestId}", chatFailure, chatCommand(s, func(h *hybrid.Service, r *http.Request, req contract.ChatAnswerRequest) error {
		answer, err := wire.ChatAnswer(req)
		if err != nil {
			return &badRequest{err}
		}
		return h.Answer(r.Context(), r.PathValue("id"), req.ClientRequestID, r.PathValue("requestId"), answer)
	}))
	serveInDone(mux, "PUT "+base+"config/{configId}", chatFailure, chatCommand(s, func(h *hybrid.Service, r *http.Request, req contract.ChatConfigRequest) error {
		return h.SetConfig(r.Context(), r.PathValue("id"), wire.ChatConfigInput(r.PathValue("configId"), req))
	}))
	serveInDone(mux, "PUT "+base+"permission-mode", chatFailure, chatCommand(s, func(h *hybrid.Service, r *http.Request, req contract.ChatPermissionModeRequest) error {
		return h.SetPermissionMode(r.Context(), r.PathValue("id"), wire.ChatPermissionModeInput(req))
	}))
	// hybridSwitch moves a hybrid session to its other side, or answers a
	// switch waiting for the user's choice. It answers once the switch is
	// over; a draft returned to the terminal goes to this requester only.
	serveIn(mux, "POST /api/sessions/{id}/switch", http.StatusOK, chatFailure, func(r *http.Request, req contract.HybridSwitchRequest) (contract.HybridSwitchResult, error) {
		h, err := s.hybrids()
		if err != nil {
			return contract.HybridSwitchResult{}, err
		}
		result, err := h.Switch(r.Context(), r.PathValue("id"), wire.HybridSwitchInput(req))
		return wire.HybridSwitchResult(result), err
	})
	// hybridResume resumes a closed or interrupted hybrid session under its
	// ID and answers with the running session. Choices the user must make
	// first (another agent session, resuming without the deleted config's
	// environment) are 409s.
	serveIn(mux, "POST /api/sessions/{id}/resume", http.StatusOK, sessionFailure, func(r *http.Request, req contract.HybridResumeRequest) (contract.SessionInfo, error) {
		if _, err := s.hybrids(); err != nil {
			return contract.SessionInfo{}, err
		}
		item, err := s.Workspace.ResumeHybrid(r.Context(), r.PathValue("id"), wire.HybridResumeInput(req))
		if err != nil {
			return contract.SessionInfo{}, err
		}
		return wire.Session(item.Info()), nil
	})
	// hybridRecords lists the hybrid session records, most recently updated
	// first; state=recoverable keeps the closed and interrupted ones.
	serve(mux, "GET /api/hybrid-sessions", http.StatusOK, chatFailure, func(r *http.Request) ([]contract.HybridRecord, error) {
		h, err := s.hybrids()
		if err != nil {
			return []contract.HybridRecord{}, err
		}
		recoverable := false
		switch r.URL.Query().Get("state") {
		case "":
		case "recoverable":
			recoverable = true
		default:
			return []contract.HybridRecord{}, &badRequest{errors.New("state 只能是 recoverable")}
		}
		list, err := h.Records(recoverable)
		return wire.HybridRecords(list, s.configMissing), err
	})
	serve(mux, "GET /api/hybrid-sessions/{id}", http.StatusOK, chatFailure, func(r *http.Request) (contract.HybridRecord, error) {
		h, err := s.hybrids()
		if err != nil {
			return contract.HybridRecord{}, err
		}
		rec, err := h.Record(r.PathValue("id"))
		if err != nil {
			return contract.HybridRecord{}, err
		}
		return wire.HybridRecord(rec, s.configMissing(rec.ConfigID)), nil
	})
	// removeHybridRecord deletes a closed or interrupted record and its
	// exited shell; the agent keeps its own session.
	serveDone(mux, "DELETE /api/hybrid-sessions/{id}", chatFailure, func(r *http.Request) error {
		h, err := s.hybrids()
		if err != nil {
			return err
		}
		id := r.PathValue("id")
		if err := h.RemoveRecord(r.Context(), id); err != nil {
			return err
		}
		// The folder outlived the shell for a possible resume; the record
		// is gone, so nothing can come back for it.
		s.Artifacts.Remove(id)
		return nil
	})
}

// hybrids returns the hybrid service, or hybrid.ErrNotFound: without one no
// session has a chat channel.
func (s *Server) hybrids() (*hybrid.Service, error) {
	if s.Hybrid == nil {
		return nil, hybrid.ErrNotFound
	}
	return s.Hybrid, nil
}

// chatCommand adapts a command on the hybrid service to a serveInDone
// handler.
func chatCommand[Req any](s *Server, run func(h *hybrid.Service, r *http.Request, req Req) error) func(*http.Request, Req) error {
	return func(r *http.Request, req Req) error {
		h, err := s.hybrids()
		if err != nil {
			return err
		}
		return run(h, r, req)
	}
}

func (s *Server) chatWS(w http.ResponseWriter, r *http.Request) {
	var sub *hybrid.Subscription
	err := hybrid.ErrNotFound
	if s.Hybrid != nil {
		sub, err = s.Hybrid.Subscribe(r.PathValue("id"), hub.ChatResume(r.URL.Query()))
	}
	c := s.accept(w, r)
	if c == nil {
		if sub != nil {
			sub.Close()
		}
		return
	}
	if err != nil {
		_ = c.Close(4404, "session not found")
		return
	}
	hub.Chat(r.Context(), sub, c)
}

// configMissing reports whether launch config id was deleted; a store that
// cannot be read cannot tell and says no.
func (s *Server) configMissing(id string) bool {
	_, err := s.Launch.Get(id)
	return errors.Is(err, launch.ErrNotFound)
}
