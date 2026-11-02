// Package omp is the hybrid adapter for omp (oh-my-pi). Its option and
// subcommand tables follow the measured baseline in
// openspec/changes/archive/2026-10-29-add-acp-hybrid-sessions/baseline.md.
package omp

import (
	ap "github.com/rainchan/webmux/internal/agentprofile"
)

// Name identifies omp on the wire and in hybrid session records.
const Name = "omp"

// Agent is the omp adapter.
var Agent ap.Agent = adapter{SessionSteering: ap.SessionSteering{}}

type adapter struct {
	ap.SessionSteering
}

var profile = &ap.Profile{
	Name:            Name,
	Title:           "omp",
	VerifiedVersion: "18.13.1",
	ResumeFlag:      "--resume",
	// omp acp parses the full argv and ignores unknown options, so every
	// kept option configures both sides the same way.
	PassToACP: true,
	// omp names models by display name, which several providers share; the
	// chat view shows the value instead so the choices stay distinct.
	ModelChoiceValues: true,
	Positional:        ap.PositionalPrompt,
	Options:           options(),
	Subcommands: ap.SetOf("acp", "agents", "anonymize", "auth", "auth-broker",
		"auth-gateway", "bench", "browser-relay", "cleanse", "clip", "collab",
		"commit", "completions", "compress", "config", "dry-balance", "find",
		"gallery", "gc", "git", "grep", "grievances", "if-bench", "images",
		"install", "join", "login", "models", "play", "plugin", "predict", "ps",
		"read", "render", "say", "search", "session", "setup", "share", "shell",
		"skill", "ssh", "stats", "stream", "tiny-models", "token", "toks",
		"ttsr", "update", "usage", "worktree"),
}

func options() map[string]ap.Option {
	o := map[string]ap.Option{}
	ap.Options(o, ap.ArgDrop, ap.ArityNone, "-c", "--continue", "--plan-yolo",
		"--from-claude", "--from-codex", "--wait-async", "--print-thoughts", "--no-ui")
	ap.Options(o, ap.ArgDrop, ap.ArityOptional, "-r", "--resume")
	ap.Options(o, ap.ArgDrop, ap.ArityOne, "--restore", "--goal", "--plan-yolo-into")
	ap.Options(o, ap.ArgStore, ap.ArityOne, "--session-dir")
	ap.Options(o, ap.ArgStore, ap.ArityNone, "--no-session")
	ap.Options(o, ap.ArgDirectory, ap.ArityOne, "--cwd")
	ap.Options(o, ap.ArgNonInteractive, ap.ArityNone, "-p", "--print", "-h", "--help", "-v", "--version")
	ap.Options(o, ap.ArgNonInteractive, ap.ArityOne, "--mode", "--export", "--alias")
	ap.Options(o, ap.ArgKeep, ap.ArityOne, "--model", "--smol", "--slow", "--plan",
		"--prewalk-into", "--provider", "--api-key", "--system-prompt",
		"--system-prompt-template", "--append-system-prompt", "--profile",
		"--config", "--add-dir", "--models", "--tools", "--thinking",
		"--service-tier", "--hook", "-e", "--extension", "--skills",
		"--max-time", "--warn-cost", "--max-cost", "--approval-mode",
		"--plugin-dir")
	ap.Options(o, ap.ArgKeep, ap.ArityNone, "--prewalk", "--no-prewalk", "--allow-home",
		"--no-tools", "--no-lsp", "--no-pty", "--hide-thinking", "--advisor",
		"--external-thinking", "--no-extensions", "--no-skills", "--no-rules",
		"--no-context-files", "--no-title", "--auto-approve")
	return o
}

func (adapter) Profile() *ap.Profile { return profile }

// Identify reads omp's Warp cli-agent events, or its OSC 7501 root record.
func (adapter) Identify(s ap.Signals) (ap.Identity, bool) {
	if s.WarpAgent != Name && s.ProgramApp != Name {
		return ap.Identity{}, false
	}
	return ap.Identity{Agent: Name, Version: s.WarpPluginVersion, AgentSessionID: s.WarpSessionID, Cwd: s.WarpCwd}, true
}

func (adapter) InterruptKeys() [][]byte { return [][]byte{[]byte("\x1b")} }

func (adapter) ResumesSessions() bool { return true }

// ReapplyOnLoad: omp's session/load does not restore the mode.
func (adapter) ReapplyOnLoad() []string { return []string{"mode"} }

// ForceSwitchNote: a killed omp turn loses what it has not saved.
func (adapter) ForceSwitchNote() string { return "omp 已输出的部分可能丢失" }
