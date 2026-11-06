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
	// Read-only: the settings page asks whether an edited command can have
	// the chat view, so the browser never mirrors the profile rules.
	serveIn(mux, "POST "+base+"/chat-support", http.StatusOK, launchFailure, func(_ *http.Request, req contract.ChatSupportRequest) (contract.ChatSupport, error) {
		return wire.ChatSupport(agents.Default().Check(req.Cmd)), nil
	})
	// options asks the config's agent which options it offers, before any
	// chat of it exists.
	serveIn(mux, "POST "+base+"/{id}/options", http.StatusOK, sessionFailure, func(r *http.Request, req contract.LaunchOptionsRequest) ([]contract.ChatConfig, error) {
		options, err := s.Workspace.LaunchOptions(r.Context(), r.PathValue("id"), req.RepositoryID)
		return wire.ChatConfigs(options), err
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
