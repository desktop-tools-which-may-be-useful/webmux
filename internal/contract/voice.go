package contract

// VoiceClientMessage is a JSON control frame sent by the browser over
// GET /ws/voice. Binary frames carry raw 16kHz mono s16le PCM audio.
type VoiceClientMessage struct {
	Type string `json:"type"`
}

// VoiceServerEvent is a JSON event frame sent by the server over
// GET /ws/voice.
type VoiceServerEvent struct {
	Type  string `json:"type"`
	Text  string `json:"text,omitempty"`
	Error string `json:"error,omitempty"`
}

// VoiceSettings is the web-editable [voice] recognizer returned by
// /api/voice/settings. Env values are credentials for the recognition
// service: like launch configs, they are readable only with the login token.
type VoiceSettings struct {
	// Cmd is the recognizer's full argv, run without a shell.
	Cmd []string `json:"cmd"`
	// Env is added to the recognizer's environment.
	Env map[string]string `json:"env"`
	// Enabled reports whether a recognizer is configured, which shows the
	// microphone buttons.
	Enabled bool `json:"enabled"`
	// Source is config when the TOML values apply, web after a saved update.
	Source string `json:"source"`
	// StorageAvailable reports whether web updates can be persisted.
	StorageAvailable bool `json:"storageAvailable"`
	// StorageError explains an unavailable settings store.
	StorageError string `json:"storageError,omitempty"`
}

// VoiceSettingsUpdate replaces the recognizer. An empty Cmd turns speech
// recognition off.
type VoiceSettingsUpdate struct {
	Cmd []string          `json:"cmd"`
	Env map[string]string `json:"env"`
}
