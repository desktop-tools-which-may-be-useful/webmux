package contract

import "time"

// ForwardPrefix is what the gateway sends upstream: "keep" sends
// /forward/{port}/... unchanged, "strip" removes /forward/{port}.
type ForwardPrefix string

const (
	ForwardPrefixKeep  ForwardPrefix = "keep"
	ForwardPrefixStrip ForwardPrefix = "strip"
)

// ForwardStatus is "listening" when the loopback port accepts connections,
// otherwise "unreachable".
type ForwardStatus string

// ForwardOptions are the compatibility rewrites of one forward.
type ForwardOptions struct {
	RewriteHost      bool          `json:"rewriteHost"`
	RewriteOrigin    bool          `json:"rewriteOrigin"`
	ForwardedHeaders bool          `json:"forwardedHeaders"`
	RewriteResponse  bool          `json:"rewriteResponse"`
	Prefix           ForwardPrefix `json:"prefix"`
}

// Forward is one forwarded loopback port, served at Path.
type Forward struct {
	Port int    `json:"port"`
	Path string `json:"path"`
	// Label is the optional display name.
	Label string `json:"label"`
	// Session is the ID of the session the port was detected in, or "".
	Session   string         `json:"session"`
	Process   string         `json:"process"`
	Options   ForwardOptions `json:"options"`
	Status    ForwardStatus  `json:"status"`
	CreatedAt time.Time      `json:"createdAt"`
}

// ForwardRequest is the body of POST /api/forwards; omitted options take the
// defaults.
type ForwardRequest struct {
	Port    int             `json:"port"`
	Label   string          `json:"label,omitempty"`
	Session string          `json:"session,omitempty"`
	Process string          `json:"process,omitempty"`
	Options *ForwardOptions `json:"options,omitempty"`
}

// ForwardUpdate is the body of PATCH /api/forwards/{port}; omitted fields
// stay unchanged.
type ForwardUpdate struct {
	Label   *string         `json:"label,omitempty"`
	Options *ForwardOptions `json:"options,omitempty"`
}

// ForwardTicket is a one-time code the browser posts to /forward/_auth to
// receive the forwarding cookie.
type ForwardTicket struct {
	Code      string    `json:"code"`
	ExpiresAt time.Time `json:"expiresAt"`
}

// ListeningPort is a TCP port a session's process tree listens on.
type ListeningPort struct {
	Port    int    `json:"port"`
	Process string `json:"process"`
}
