package api

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"

	"github.com/rainchan/webmux/internal/contract"
	"github.com/rainchan/webmux/internal/wire"
	"github.com/rainchan/webmux/internal/workspace"
)

// quickStartRoutes serves POST /api/quick-starts. The quick start runs to
// its end (or workspace.QuickStartTimeout) even if the client goes away; a
// retry with the same clientRequestId gets its outcome. Every failure is a
// contract.QuickStartFailure.
func (s *Server) quickStartRoutes(mux *http.ServeMux) {
	mux.HandleFunc("POST /api/quick-starts", func(w http.ResponseWriter, r *http.Request) {
		var req contract.QuickStartRequest
		if err := decodeQuickStart(w, r, &req); err != nil {
			reply(w, http.StatusBadRequest, contract.QuickStartFailure{Error: err.Error(), Step: workspace.StepPrecheck})
			return
		}
		ctx, cancel := context.WithTimeout(context.WithoutCancel(r.Context()), workspace.QuickStartTimeout)
		defer cancel()
		result, err := s.Workspace.QuickStart(ctx, wire.QuickStartInput(req))
		if err != nil {
			quickStartFailure(w, err)
			return
		}
		reply(w, http.StatusCreated, wire.QuickStartResult(result))
	})
}

// decodeQuickStart reads the body like decode, with room for a prompt of
// hybrid.MaxPromptBytes, but leaves the failure body to the caller: every
// quick start failure is a contract.QuickStartFailure.
func decodeQuickStart(w http.ResponseWriter, r *http.Request, req *contract.QuickStartRequest) error {
	data, err := io.ReadAll(http.MaxBytesReader(w, r.Body, chatBodyBytes))
	if err != nil {
		return err
	}
	d := json.NewDecoder(bytes.NewReader(data))
	d.DisallowUnknownFields()
	if err := d.Decode(req); err != nil {
		return err
	}
	if len(bytes.TrimSpace(data[d.InputOffset():])) > 0 {
		return errors.New("expected one JSON object")
	}
	return nil
}
