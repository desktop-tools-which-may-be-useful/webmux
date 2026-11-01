// Package opencode is the hybrid adapter for OpenCode. Its option table
// follows the measured baseline in
// openspec/changes/archive/2026-10-29-add-acp-hybrid-sessions/baseline.md.
package opencode

import (
	"strings"

	ap "github.com/rainchan/webmux/internal/agentprofile"
)

// Name identifies OpenCode on the wire and in hybrid session records.
const Name = "opencode"

// sessionPrefix marks OpenCode session IDs. OpenCode borrows the OSC 7501
// child record id for its session.
const sessionPrefix = "ses_"

// Agent is the OpenCode adapter.
var Agent ap.Agent = adapter{}

type adapter struct{}

var profile = &ap.Profile{
	Name:            Name,
	Title:           "OpenCode",
	VerifiedVersion: "2.1.4",
	ResumeFlag:      "-s",
	// The default TUI is a client of the background service, where a turn
	// keeps running after the TUI exits; a private server stops with it.
	TUIInject:  []string{"--standalone"},
	Positional: ap.PositionalDirectory,
	Options:    options(),
}

func options() map[string]ap.Option {
	o := map[string]ap.Option{}
	ap.Options(o, ap.ArgKeep, ap.ArityNone, "--standalone", "--auto", "--print-logs")
	ap.Options(o, ap.ArgKeep, ap.ArityOne, "--log-level")
	ap.Options(o, ap.ArgDrop, ap.ArityNone, "-c", "--continue")
	ap.Options(o, ap.ArgDrop, ap.ArityOne, "-s", "--session", "--prompt")
	ap.Options(o, ap.ArgStore, ap.ArityOne, "--server")
	ap.Options(o, ap.ArgNonInteractive, ap.ArityNone, "-h", "--help", "-v", "--version", "--wizard")
	ap.Options(o, ap.ArgNonInteractive, ap.ArityOne, "--completions")
	return o
}

func (adapter) Profile() *ap.Profile { return profile }

// Identify reads OpenCode's OSC 7501 records: the root's app, and the
// child id as the session when it has the session prefix.
func (adapter) Identify(s ap.Signals) (ap.Identity, bool) {
	if s.ProgramApp != Name {
		return ap.Identity{}, false
	}
	id := ""
	if strings.HasPrefix(s.ProgramChildID, sessionPrefix) {
		id = s.ProgramChildID
	}
	return ap.Identity{Agent: Name, AgentSessionID: id, Cwd: s.RootCwd}, true
}

// InterruptKeys: OpenCode needs two separate Esc presses; a single
// "\x1b\x1b" read reaches it as one Alt+Esc.
func (adapter) InterruptKeys() [][]byte { return [][]byte{[]byte("\x1b"), []byte("\x1b")} }

func (adapter) ResumesSessions() bool { return true }
