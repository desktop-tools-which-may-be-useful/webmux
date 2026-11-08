package api

import (
	"errors"
	"net/http"

	"github.com/rainchan/webmux/internal/contract"
)

// VoiceControl is injected by cmd/. HTTP handlers never construct the
// settings store or touch its file.
type VoiceControl interface {
	Settings() contract.VoiceSettings
	Update(contract.VoiceSettingsUpdate) (contract.VoiceSettings, error)
	Reset() (contract.VoiceSettings, error)
}

func (s *Server) voiceSettingsRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/voice/settings", func(w http.ResponseWriter, _ *http.Request) {
		if s.voiceControl(w) == nil {
			return
		}
		reply(w, http.StatusOK, s.VoiceSettings.Settings())
	})
	mux.HandleFunc("PUT /api/voice/settings", func(w http.ResponseWriter, r *http.Request) {
		if s.voiceControl(w) == nil {
			return
		}
		var req contract.VoiceSettingsUpdate
		if !decode(w, r, &req) {
			return
		}
		view, err := s.VoiceSettings.Update(req)
		if err != nil {
			failure(w, voiceSettingsStatus(err), err)
			return
		}
		reply(w, http.StatusOK, view)
	})
	mux.HandleFunc("DELETE /api/voice/settings", func(w http.ResponseWriter, _ *http.Request) {
		if s.voiceControl(w) == nil {
			return
		}
		view, err := s.VoiceSettings.Reset()
		if err != nil {
			failure(w, voiceSettingsStatus(err), err)
			return
		}
		reply(w, http.StatusOK, view)
	})
}

func (s *Server) voiceControl(w http.ResponseWriter) VoiceControl {
	if s.VoiceSettings == nil {
		failure(w, http.StatusServiceUnavailable, errors.New("voice settings unavailable"))
	}
	return s.VoiceSettings
}
