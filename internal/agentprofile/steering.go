package agentprofile

import "encoding/json"

// SessionSteering is the _session/steering extension that omp introduced
// and codex-acp follows: initialize declares it in
// _meta.steering.supported. Adapters embed it to implement Steerer.
type SessionSteering struct {
	// IdlePrompt asks the agent to hand a prompt back (outcome
	// promptRequired) instead of starting a turn by itself when no turn
	// runs.
	IdlePrompt bool
}

// steeringIdlePrompt is the steering request _meta asking for
// promptRequired when the turn already ended.
var steeringIdlePrompt = json.RawMessage(`{"steering":{"idleBehavior":"promptRequired"}}`)

// SteeringMethod implements Steerer.
func (SessionSteering) SteeringMethod() string { return "_session/steering" }

// SteeringAvailable implements Steerer.
func (SessionSteering) SteeringAvailable(meta json.RawMessage) bool {
	if len(meta) == 0 {
		return false
	}
	var m struct {
		Steering *struct {
			Supported bool `json:"supported"`
		} `json:"steering"`
	}
	if json.Unmarshal(meta, &m) != nil {
		return false
	}
	return m.Steering != nil && m.Steering.Supported
}

// SteeringMeta implements Steerer.
func (s SessionSteering) SteeringMeta() json.RawMessage {
	if s.IdlePrompt {
		return steeringIdlePrompt
	}
	return nil
}
