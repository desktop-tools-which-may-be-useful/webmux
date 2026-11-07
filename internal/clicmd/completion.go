package clicmd

import (
	"context"
	"flag"
	"fmt"
	"net/http"
	"slices"
	"strings"
	"time"

	"github.com/rainchan/webmux/internal/contract"
)

// completeTimeout bounds the server queries of one completion.
const completeTimeout = time.Second

var completionCommand = &Command{
	Name: "completion", Group: GroupHelp, Args: "bash|zsh|fish", Platform: "unix", NoGlobal: true,
	Summary: "Print a shell completion script for bash, zsh or fish.",
	Description: "The script completes commands, options and enumerated values, and asks the running server for session IDs " +
		"(with titles in zsh and fish), launch config names, repositories, worktrees and topics. Without a server, or when it " +
		"does not answer within a second, only the static candidates are offered and nothing is printed as an error.",
	Arguments: []Item{{Name: "SHELL", Text: "bash, zsh or fish."}},
	Output:    "The completion script.",
	Exit:      []ExitItem{{Code: 0, Text: "Printed."}, {Code: 2, Text: "Unknown shell."}},
	Examples: []Example{
		{Command: `echo 'source <(webmux completion bash)' >> ~/.bashrc`, Text: "Enable completion in bash."},
		{Command: "webmux completion zsh > ~/.zfunc/_webmux", Text: "Install zsh completion (with ~/.zfunc in fpath before compinit)."},
		{Command: "webmux completion fish > ~/.config/fish/completions/webmux.fish", Text: "Install fish completion."},
	},
	SeeAlso: []string{"webmux help"},
	Bind: func(fs *flag.FlagSet) Action {
		return func(_ context.Context, e Env, args []string) int {
			if len(args) != 1 {
				return e.Usagef("give one shell: bash, zsh or fish")
			}
			script, ok := completionScripts[args[0]]
			if !ok {
				return e.Usagef("unknown shell %q (bash, zsh or fish)", args[0])
			}
			_, _ = fmt.Fprint(e.Stdout, script)
			return contract.ExitOK
		}
	},
}

var completionScripts = map[string]string{
	"bash": `# bash completion for webmux
_webmux() {
	local IFS=$'\n'
	local words=("${COMP_WORDS[@]:1:COMP_CWORD}")
	COMPREPLY=($(webmux __complete "${words[@]}" 2>/dev/null | cut -f1))
}
complete -o default -F _webmux webmux
`,
	"zsh": `#compdef webmux
# zsh completion for webmux
_webmux() {
	local -a candidates
	local line
	for line in "${(@f)$(webmux __complete "${words[@]:1:$((CURRENT-1))}" 2>/dev/null)}"; do
		[[ -n $line ]] && candidates+=("${line/$'\t'/:}")
	done
	_describe 'webmux' candidates
}
compdef _webmux webmux
`,
	"fish": `# fish completion for webmux
complete -c webmux -f -a '(webmux __complete (commandline -opc)[2..] (commandline -ct) 2>/dev/null)'
`,
}

var completeCommand = &Command{
	Name: "__complete", Group: GroupHelp, Hidden: true, RawArgs: true, NoGlobal: true,
	Summary: "Print completion candidates for the words after webmux (used by the completion scripts).",
	Bind: func(fs *flag.FlagSet) Action {
		return func(ctx context.Context, e Env, args []string) int {
			for _, c := range e.complete(ctx, args) {
				_, _ = fmt.Fprintln(e.Stdout, c)
			}
			return contract.ExitOK
		}
	},
}

// complete returns "value\tdescription" candidates for the last word of
// words, the arguments typed after webmux.
func (e Env) complete(ctx context.Context, words []string) []string {
	if len(words) == 0 {
		words = []string{""}
	}
	current := words[len(words)-1]
	done := words[:len(words)-1]
	// Dispatch always sets Commands; Commands() itself lists this command.
	r := e.Commands.Visible(e.Platform)
	c, rest := r.Find(done)
	var out []string
	add := func(value, text string) {
		if strings.HasPrefix(value, current) {
			out = append(out, value+"\t"+text)
		}
	}
	switch {
	case c == nil:
		for _, cmd := range r {
			if !strings.Contains(cmd.Name, " ") {
				add(cmd.Name, cmd.Summary)
			}
		}
		return out
	case c.Bind == nil:
		for _, sub := range r.Children(c.Name) {
			add(strings.TrimPrefix(sub.Name, c.Name+" "), sub.Summary)
		}
		return out
	}
	options := e.Options(c)
	if strings.HasPrefix(current, "-") {
		for _, o := range options {
			add("--"+o.Name, o.Description)
		}
		return out
	}
	// A value for the option before the current word.
	if len(rest) > 0 {
		if prev, ok := strings.CutPrefix(rest[len(rest)-1], "--"); ok {
			for _, o := range options {
				if o.Name == prev && o.Type != "bool" {
					for _, v := range e.optionValues(ctx, prev) {
						add(v[0], v[1])
					}
					return out
				}
			}
		}
	}
	for _, v := range e.argumentValues(ctx, c, rest) {
		add(v[0], v[1])
	}
	return out
}

// optionValues lists candidates for an option's value.
func (e Env) optionValues(ctx context.Context, name string) [][2]string {
	switch name {
	case "view":
		return [][2]string{{"terminal", ""}, {"chat", ""}}
	case "mode":
		return [][2]string{{"auto", "send now or queue"}, {"steer", "inject into the running turn"}}
	case "state":
		var out [][2]string
		for _, s := range listStates {
			out = append(out, [2]string{s, ""})
		}
		return out
	case "permission":
		return [][2]string{{"ask", ""}, {"auto", ""}}
	case "until", "wait", "status":
		var out [][2]string
		for _, s := range sessionStates {
			out = append(out, [2]string{s, ""})
		}
		return out
	case "key":
		var out [][2]string
		for _, k := range strings.Split(keyNames(), ", ") {
			out = append(out, [2]string{k, ""})
		}
		return out
	case "kind":
		return [][2]string{{"notify", ""}, {"done", ""}, {"error", ""}, {"question", ""}}
	case "launch-config", "config-id":
		return e.launchConfigValues(ctx)
	case "repo":
		return e.repositoryValues(ctx)
	case "worktree":
		return e.worktreeValues(ctx)
	case "parent", "session":
		return e.sessionValues(ctx)
	}
	return nil
}

// argumentValues lists candidates for a command's positional argument;
// rest are the words already typed after the command.
func (e Env) argumentValues(ctx context.Context, c *Command, rest []string) [][2]string {
	switch {
	case c.Name == "view" && positionals(rest) == 1:
		return [][2]string{{"terminal", ""}, {"chat", ""}}
	case c.Name == "resume" || c.Name == "rm" && slices.Contains(rest, "--forget"):
		return e.archivedValues(ctx)
	case c.Name == "guide":
		return topicValues()
	case c.Name == "completion":
		return [][2]string{{"bash", ""}, {"zsh", ""}, {"fish", ""}}
	case strings.HasPrefix(c.Name, "launch-config ") && strings.HasPrefix(c.Args, "NAME"):
		return e.launchConfigValues(ctx)
	case c.Name == "forward rm":
		var list []contract.Forward
		if !e.query(ctx, "/api/forwards", &list) {
			return nil
		}
		out := make([][2]string, 0, len(list))
		for _, f := range list {
			out = append(out, [2]string{fmt.Sprint(f.Port), f.Label})
		}
		return out
	case strings.Contains(c.Args, "SESSION"):
		return append([][2]string{{"self", "the current session"}}, e.sessionValues(ctx)...)
	case strings.Contains(c.Args, "REPO"):
		return e.repositoryValues(ctx)
	}
	return nil
}

func topicValues() [][2]string {
	var out [][2]string
	for _, name := range topicNames() {
		out = append(out, [2]string{name, ""})
	}
	return out
}

// query runs fn against the server within completeTimeout; any failure
// leaves the candidates empty.
func (e Env) query(ctx context.Context, path string, out any) bool {
	if e.Connect == nil {
		return false
	}
	ctx, cancel := context.WithTimeout(ctx, completeTimeout)
	defer cancel()
	client, err := e.Connect()
	if err != nil {
		return false
	}
	defer client.Close()
	return client.Do(ctx, http.MethodGet, path, nil, out) == nil
}

func (e Env) sessionValues(ctx context.Context) [][2]string {
	var list []contract.SessionInfo
	if !e.query(ctx, "/api/sessions", &list) {
		return nil
	}
	slices.SortFunc(list, func(a, b contract.SessionInfo) int { return strings.Compare(a.ID, b.ID) })
	out := make([][2]string, 0, len(list))
	for _, s := range list {
		out = append(out, [2]string{s.ID, s.Title + " (" + string(s.Status) + ")"})
	}
	return out
}

// positionals counts the words of rest that are not options; an option's
// value counts too, which is good enough for completion.
func positionals(rest []string) int {
	n := 0
	for _, w := range rest {
		if !strings.HasPrefix(w, "-") {
			n++
		}
	}
	return n
}

// archivedValues lists the closed and interrupted sessions, for resume and
// rm --forget.
func (e Env) archivedValues(ctx context.Context) [][2]string {
	var list []contract.SessionInfo
	if !e.query(ctx, "/api/sessions?state=archived", &list) {
		return nil
	}
	out := make([][2]string, 0, len(list))
	for _, s := range list {
		out = append(out, [2]string{s.ID, string(s.State) + " " + oneLine(sessionName(s), 40)})
	}
	return out
}

func (e Env) launchConfigValues(ctx context.Context) [][2]string {
	var list []contract.LaunchConfigSummary
	if !e.query(ctx, "/api/launch-configs", &list) {
		return nil
	}
	out := make([][2]string, 0, len(list))
	for _, c := range list {
		out = append(out, [2]string{c.Name, strings.Join(c.Cmd, " ")})
	}
	return out
}

func (e Env) repositoryValues(ctx context.Context) [][2]string {
	var list []contract.Repository
	if !e.query(ctx, "/api/repositories", &list) {
		return nil
	}
	out := make([][2]string, 0, len(list))
	for _, r := range list {
		out = append(out, [2]string{r.ID, r.Name + " " + r.AnchorPath})
	}
	return out
}

func (e Env) worktreeValues(ctx context.Context) [][2]string {
	var list []contract.Repository
	if !e.query(ctx, "/api/repositories", &list) {
		return nil
	}
	var out [][2]string
	for _, r := range list {
		for _, t := range r.Worktrees {
			branch := ""
			if t.Branch != nil {
				branch = *t.Branch
			}
			out = append(out, [2]string{t.Path, r.Name + " " + branch})
		}
	}
	return out
}
