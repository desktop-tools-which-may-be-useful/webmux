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
