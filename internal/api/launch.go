package api

import (
	"net/http"

	"github.com/rainchan/webmux/internal/agentprofile/agents"
	"github.com/rainchan/webmux/internal/contract"
	"github.com/rainchan/webmux/internal/hybrid"
	"github.com/rainchan/webmux/internal/wire"
)

// launchRoutes serves the saved launch configs. The list carries environment
// names only; values leave the server only for a single config.
func (s *Server) launchRoutes(mux *http.ServeMux) {
	const base = "/api/launch-configs"
	serve(mux, "GET "+base, http.StatusOK, launchFailure, func(*http.Request) ([]contract.LaunchConfigSummary, error) {
		list, err := s.Launch.List()
		return wire.LaunchSummaries(list, s.chatOptions), err
	})
	serve(mux, "GET "+base+"/presets", http.StatusOK, launchFailure, func(*http.Request) ([]contract.LaunchPreset, error) {
		list, err := s.Launch.Presets()
		return wire.LaunchPresets(list), err
	})
	serveIn(mux, "POST "+base+"/presets", http.StatusOK, launchFailure, func(_ *http.Request, req contract.RestoreLaunchPresetsRequest) ([]contract.LaunchConfigSummary, error) {
		list, err := s.Launch.RestorePresets(req.Name)
		return wire.LaunchSummaries(list, s.chatOptions), err
	})
	// Read-only: the settings page asks whether an edited command can use
	// hybrid mode, so the browser never mirrors the profile rules.
	serveIn(mux, "POST "+base+"/hybrid-support", http.StatusOK, launchFailure, func(_ *http.Request, req contract.HybridSupportRequest) (contract.HybridSupport, error) {
		return wire.HybridSupport(agents.Default().Check(req.Cmd)), nil
	})
	serve(mux, "GET "+base+"/{id}", http.StatusOK, launchFailure, func(r *http.Request) (contract.LaunchConfig, error) {
		item, err := s.Launch.Get(r.PathValue("id"))
		return wire.LaunchConfig(item), err
	})
	serveIn(mux, "POST "+base, http.StatusCreated, launchFailure, func(_ *http.Request, req contract.LaunchConfigInput) (contract.LaunchConfig, error) {
		item, err := s.Launch.Create(wire.LaunchInput(req))
		return wire.LaunchConfig(item), err
	})
	serveIn(mux, "PUT "+base+"/{id}", http.StatusOK, launchFailure, func(r *http.Request, req contract.LaunchConfigInput) (contract.LaunchConfig, error) {
		item, err := s.Launch.Update(r.PathValue("id"), wire.LaunchInput(req))
		return wire.LaunchConfig(item), err
	})
	serveDone(mux, "DELETE "+base+"/{id}", launchFailure, func(r *http.Request) error {
		return s.Launch.Delete(r.PathValue("id"))
	})
}

// chatOptions returns the agent options remembered for launch config id;
// none without hybrid sessions or when agent-options.json is unusable
// (hybrid.New logged why), so the list itself never fails for it.
func (s *Server) chatOptions(id string) []hybrid.ConfigOption {
	if s.Hybrid == nil {
		return nil
	}
	options, _ := s.Hybrid.AgentOptions(id)
	return options
}
