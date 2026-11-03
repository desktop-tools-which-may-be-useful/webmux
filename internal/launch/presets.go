package launch

import "github.com/rainchan/webmux/internal/agentprofile"

// Presets returns the launch configs seeded into a new installation: a login
// shell and common CLI agents. Agents load the startup file of the service's
// default shell; permission flags are explicit parts of the editable
// commands. Claude Code, Codex CLI, OpenCode, omp, Qoder and QoderCN start as hybrid
// sessions. They are ordinary configs the user may edit or delete. shell is
// the login shell path; empty means /bin/sh.
func Presets(shell string) []Input {
	login := shell
	if login == "" {
		login = "/bin/sh"
	}
	startup := agentprofile.DefaultShell(shell)
	command := func(program string, args ...string) []string {
		return agentprofile.StartupArgv(startup, program, args...)
	}
	return []Input{
		{Name: "Shell", Icon: "terminal", Cmd: []string{login, "-l", "-i"}},
		{Name: "Claude Code", Icon: "claudecode", Cmd: command("claude", "--dangerously-skip-permissions"), Hybrid: true},
		{Name: "Codex CLI", Icon: "codex", Cmd: command("codex", "--dangerously-bypass-approvals-and-sandbox"), Hybrid: true},
		{Name: "Gemini CLI", Icon: "geminicli", Cmd: command("gemini", "--yolo")},
		{Name: "Cursor CLI", Icon: "cursor", Cmd: command("agent")},
		{Name: "GitHub Copilot CLI", Icon: "githubcopilot", Cmd: command("copilot")},
		{Name: "Open Code", Icon: "opencode", Cmd: command("opencode", "--auto"), Hybrid: true},
		{Name: "Qwen Code", Icon: "qwen", Cmd: command("qwen")},
		{Name: "Kimi Code CLI", Icon: "kimi", Cmd: command("kimi")},
		{Name: "CodeBuddy Code", Icon: "codebuddy", Cmd: command("codebuddy")},
		{Name: "omp", Icon: "pi", Cmd: command("omp", "--auto-approve"), Env: map[string]string{"PI_FORCE_IMAGE_PROTOCOL": "kitty"}, Hybrid: true},
		{Name: "Qoder", Icon: "qoder", Cmd: command("qoder", "--dangerously-skip-permissions"), Hybrid: true},
		{Name: "QoderCN", Icon: "qoder", Cmd: command("qodercn", "--dangerously-skip-permissions"), Hybrid: true},
	}
}
