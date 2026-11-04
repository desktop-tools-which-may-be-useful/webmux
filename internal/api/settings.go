package api

import (
	"net/http"

	"github.com/rainchan/webmux/internal/contract"
	"github.com/rainchan/webmux/internal/hub"
	"github.com/rainchan/webmux/internal/settings"
	"github.com/rainchan/webmux/internal/wire"
)

func (s *Server) settingsRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/settings", func(w http.ResponseWriter, r *http.Request) {
		v, err := s.Settings.Snapshot()
		if err != nil {
			failure(w, settingsStatus(err), err)
			return
		}
		reply(w, 200, wire.SettingsSnapshot(v))
	})
	mux.HandleFunc("PATCH /api/settings", func(w http.ResponseWriter, r *http.Request) {
		var req contract.SettingsPatch
		if !decodeStrict(w, r, &req, 2<<20, settings.ErrInvalid) {
			return
		}
		changes, err := wire.SettingsChanges(req)
		if err != nil {
			failure(w, 400, err)
			return
		}
		v, err := s.Settings.Patch(*req.BaseRevision, changes)
		if err != nil {
			failure(w, settingsStatus(err), err)
			return
		}
		reply(w, 200, wire.SettingsSnapshot(v))
	})
	mux.HandleFunc("GET /ws/settings", func(w http.ResponseWriter, r *http.Request) {
		sub, err := s.Settings.Subscribe()
		if err != nil {
			failure(w, settingsStatus(err), err)
			return
		}
		defer sub.Close()
		c := s.accept(w, r)
		if c == nil {
			return
		}
		hub.Settings(r.Context(), sub, c)
	})
}
