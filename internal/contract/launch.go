package contract

import "time"

// PreviewSettings selects the session output preview rows. A null config
// preview inherits the browser's global preview setting.
type PreviewSettings struct {
	Lines int `json:"lines"`
	Skip  int `json:"skip"`
}

// HybridSupport reports whether a command can run as a hybrid session.
// Reason is a stable code (unknown_agent, unsupported_form, session_store,
// directory_arg, non_interactive, session_arg) set when Supported is false.
// Connector names the separately installed ACP connector a supported agent
// needs for its chat side.
type HybridSupport struct {
	Supported bool            `json:"supported"`
	Agent     string          `json:"agent,omitempty"`
	Reason    string          `json:"reason,omitempty"`
	Connector *AgentConnector `json:"connector,omitempty"`
	// SupportedAgents are the display names of the agents hybrid mode
	// supports.
	SupportedAgents []string `json:"supportedAgents"`
}

// HybridSupportRequest is the body of POST /api/launch-configs/hybrid-support.
type HybridSupportRequest struct {
	Cmd []string `json:"cmd"`
}

// LaunchConfig is one saved launch config with its environment values.
type LaunchConfig struct {
	ID            string            `json:"id"`
	Name          string            `json:"name"`
	Icon          string            `json:"icon,omitempty"`
	Cmd           []string          `json:"cmd"`
	Env           map[string]string `json:"env"`
	Preview       *PreviewSettings  `json:"preview"`
	Hybrid        bool              `json:"hybrid"`
	HybridSupport HybridSupport     `json:"hybridSupport"`
	CreatedAt     time.Time         `json:"createdAt"`
	UpdatedAt     time.Time         `json:"updatedAt"`
}

// LaunchConfigSummary is a list entry; it names environment variables only.
// ChatOptions are the agent options the config's latest chat session
// reported, with their last values; [] before any did.
type LaunchConfigSummary struct {
	ID            string           `json:"id"`
	Name          string           `json:"name"`
	Icon          string           `json:"icon,omitempty"`
	Cmd           []string         `json:"cmd"`
	EnvNames      []string         `json:"envNames"`
	Preview       *PreviewSettings `json:"preview"`
	Hybrid        bool             `json:"hybrid"`
	HybridSupport HybridSupport    `json:"hybridSupport"`
	ChatOptions   []ChatConfig     `json:"chatOptions"`
	CreatedAt     time.Time        `json:"createdAt"`
	UpdatedAt     time.Time        `json:"updatedAt"`
}

// LaunchConfigInput is the body of a launch config create or update. The
// read-only hybridSupport is not part of it, so sending it is rejected as an
// unknown field.
type LaunchConfigInput struct {
	Name    string            `json:"name"`
	Icon    string            `json:"icon,omitempty"`
	Cmd     []string          `json:"cmd"`
	Env     map[string]string `json:"env"`
	Preview *PreviewSettings  `json:"preview"`
	Hybrid  bool              `json:"hybrid"`
}

// LaunchPreset describes one built-in launch preset that can be restored.
type LaunchPreset struct {
	Name     string   `json:"name"`
	Icon     string   `json:"icon,omitempty"`
	Cmd      []string `json:"cmd"`
	EnvNames []string `json:"envNames"`
	Hybrid   bool     `json:"hybrid"`
}

// RestoreLaunchPresetsRequest is the body of POST /api/launch-configs/presets.
// An empty Name restores every missing preset; a set Name restores that
// preset alone.
type RestoreLaunchPresetsRequest struct {
	Name string `json:"name,omitempty"`
}
