package sessioncmd

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"maps"
	"net/http"
	"net/url"
	"slices"
	"strings"
	"text/tabwriter"

	"github.com/rainchan/webmux/internal/contract"
	"github.com/rainchan/webmux/internal/localclient"
)

const hiddenValue = "<hidden>"

var launchConfigNameArgument = Item{Name: "NAME", Text: "The exact, case-sensitive name of a saved launch config."}

// launchConfig reads the full config with the exact name.
func (e Env) launchConfig(ctx context.Context, client *localclient.Client, name string) (contract.LaunchConfig, int, bool) {
	summary, err := launchConfigByName(ctx, client, name)
	if err != nil {
		if errors.Is(err, errUnknownLaunchConfig) {
			return contract.LaunchConfig{}, e.Usagef("%v", err), false
		}
		return contract.LaunchConfig{}, e.Fail(err), false
	}
	var config contract.LaunchConfig
	if err := client.Do(ctx, http.MethodGet, "/api/launch-configs/"+url.PathEscape(summary.ID), nil, &config); err != nil {
		return config, e.Fail(err), false
	}
	return config, contract.ExitOK, true
}

func sortedKeys(m map[string]string) []string {
	return slices.Sorted(maps.Keys(m))
}

var launchConfigListCommand = &Command{
	Name: "launch-config ls", Group: GroupLaunch,
	Summary: "List the saved launch configs: name, icon, command, chat view and environment variable names.",
	Output:  "A table: NAME, ICON, CHAT, ENV, COMMAND. With --json, the LaunchConfigSummary array (variable names only, never values).",
	Exit:    exitsCommon(),
	Examples: []Example{
		{Command: "webmux launch-config ls", Text: "See which configs run --launch-config accepts."},
		{Command: "webmux launch-config ls --json | jq -r '.[] | select(.chat) | .name'", Text: "List the configs that can start on the chat view."},
	},
	SeeAlso: []string{"webmux launch-config show", "webmux run --launch-config"},
	Bind: func(fs *flag.FlagSet) Action {
		asJSON := fs.Bool("json", false, "print the LaunchConfigSummary array")
		return func(ctx context.Context, e Env, args []string) int {
			if len(args) != 0 {
				return e.Usagef("launch-config ls takes no arguments")
			}
			client, code := e.connect()
			if client == nil {
				return code
			}
			defer client.Close()
			var list []contract.LaunchConfigSummary
			if err := client.Do(ctx, http.MethodGet, "/api/launch-configs", nil, &list); err != nil {
				return e.Fail(err)
			}
			if *asJSON {
				if list == nil {
					list = []contract.LaunchConfigSummary{}
				}
				writeJSON(e.Stdout, list)
				return contract.ExitOK
			}
			w := tabwriter.NewWriter(e.Stdout, 0, 4, 2, ' ', 0)
			_, _ = fmt.Fprintln(w, "NAME\tICON\tCHAT\tENV\tCOMMAND")
			for _, c := range list {
				chat, env, icon := "-", "-", c.Icon
				if c.Chat {
					chat = "yes"
				}
				if len(c.EnvNames) > 0 {
					env = strings.Join(c.EnvNames, ",")
				}
				if icon == "" {
					icon = "-"
				}
				_, _ = fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\n", c.Name, icon, chat, env, strings.Join(c.Cmd, " "))
			}
			_ = w.Flush()
			return contract.ExitOK
		}
	},
}

var launchConfigShowCommand = &Command{
	Name: "launch-config show", Group: GroupLaunch, Args: "NAME",
	Summary:   "Show one launch config; environment values stay hidden unless --reveal-env.",
	Arguments: []Item{launchConfigNameArgument},
	Output:    "Aligned key-value lines; with --json, the LaunchConfig as GET /api/launch-configs/{id} returns it, values replaced by \"<hidden>\" unless --reveal-env.",
	Exit:      exitsCommon(ExitItem{Code: 2, Text: "Also: no launch config has that name (the saved names are listed)."}),
	Examples: []Example{
		{Command: `webmux launch-config show "Claude Code"`, Text: "See a config's command and variable names."},
		{Command: `webmux launch-config show "Claude Code" --json --reveal-env`, Text: "Copy a config with its values."},
	},
	SeeAlso: []string{"webmux launch-config edit", "webmux launch-config ls"},
	Bind: func(fs *flag.FlagSet) Action {
		asJSON := fs.Bool("json", false, "print the LaunchConfig")
		reveal := fs.Bool("reveal-env", false, "print environment variable values")
		return func(ctx context.Context, e Env, args []string) int {
			if len(args) != 1 {
				return e.Usagef("exactly one name is required (quote names with spaces)")
			}
			client, code := e.connect()
			if client == nil {
				return code
			}
			defer client.Close()
			config, code, ok := e.launchConfig(ctx, client, args[0])
			if !ok {
				return code
			}
			if !*reveal {
				for name := range config.Env {
					config.Env[name] = hiddenValue
				}
			}
			if *asJSON {
				writeJSON(e.Stdout, config)
				return contract.ExitOK
			}
			w := tabwriter.NewWriter(e.Stdout, 0, 4, 2, ' ', 0)
			preview := "global default"
			if config.Preview != nil {
				preview = fmt.Sprintf("%d lines, skip %d", config.Preview.Lines, config.Preview.Skip)
			}
			chat := "no"
			if config.Chat {
				chat = "yes"
			}
			if config.ChatSupport.Supported {
				chat += " (supported: " + config.ChatSupport.Agent + ")"
			} else if config.ChatSupport.Reason != "" {
				chat += " (unsupported: " + config.ChatSupport.Reason + ")"
			}
			for _, row := range [][2]string{
				{"id", config.ID}, {"name", config.Name}, {"icon", config.Icon}, {"command", strings.Join(config.Cmd, " ")},
				{"chat", chat}, {"preview", preview},
			} {
				if row[1] != "" {
					_, _ = fmt.Fprintf(w, "%s\t%s\n", row[0], row[1])
				}
			}
			for _, name := range sortedKeys(config.Env) {
				_, _ = fmt.Fprintf(w, "env\t%s=%s\n", name, config.Env[name])
			}
			_ = w.Flush()
			return contract.ExitOK
		}
	},
}

// configFields are the flags add and edit share.
type configFields struct {
	icon                      string
	env                       envFlag
	chat                      bool
	previewLines, previewSkip int
	set                       map[string]bool
}

func bindConfigFields(fs *flag.FlagSet, f *configFields) {
	f.env = envFlag{}
	fs.StringVar(&f.icon, "icon", "", "the launch picker `ICON` key (for example claudecode, codex, terminal)")
	fs.Var(f.env, "env", "set environment variable `NAME=VALUE` for sessions started from the config")
	fs.BoolVar(&f.chat, "chat", false, "enable the chat view for a supported agent, beside its terminal")
	fs.IntVar(&f.previewLines, "preview-lines", 0, "show `N` preview lines on the home page (1-50) instead of the global default")
	fs.IntVar(&f.previewSkip, "preview-skip", 0, "skip the last `N` lines of the preview (0-50)")
}

// visited records which flags the command line set.
func visited(fs *flag.FlagSet) map[string]bool {
	set := map[string]bool{}
	fs.Visit(func(f *flag.Flag) { set[f.Name] = true })
	return set
}

// checkChat asks the server whether argv can have a chat view.
func (e Env) checkChat(ctx context.Context, client *localclient.Client, argv []string) (int, bool) {
	var support contract.ChatSupport
	if err := client.Do(ctx, http.MethodPost, "/api/launch-configs/chat-support", contract.ChatSupportRequest{Cmd: argv}, &support); err != nil {
		return e.Fail(err), false
	}
	if !support.Supported {
		_, _ = fmt.Fprintf(e.Stderr, "webmux: this command cannot have a chat view (%s); supported agents: %s\n",
			support.Reason, strings.Join(support.SupportedAgents, ", "))
		return contract.ExitError, false
	}
	if support.Connector != nil {
		_, _ = fmt.Fprintf(e.Stderr, "webmux: its chat view needs %s (install: %s)\n", support.Connector.Title, support.Connector.InstallCommand)
	}
	return contract.ExitOK, true
}

var launchConfigAddCommand = &Command{
	Name: "launch-config add", Group: GroupLaunch, Args: "NAME -- ARGV...",
	Summary: "Save a new launch config; it appears in every browser's launch picker.",
	Description: "ARGV after -- is the command exactly as given; it is never expanded by a shell (write bash -lc '...' " +
		"explicitly when you need one). Environment values travel only in the request body and are never printed. " +
		"--chat asks the server first whether the command is a supported agent, and refuses with the reason if not.",
	Arguments: []Item{launchConfigNameArgument, {Name: "ARGV", Text: "The command and its arguments."}},
	Output:    "The new config's ID; with --json, the LaunchConfig with values hidden.",
	Exit:      exitsCommon(ExitItem{Code: 1, Text: "Also: the name is taken, validation failed, or the chat view is unsupported."}),
	Examples: []Example{
		{Command: `webmux launch-config add "Claude (sandbox)" --icon claudecode --chat --env CLAUDE_CONFIG_DIR=/w/.claude -- claude --dangerously-skip-permissions`, Text: "Add an agent config with the chat view."},
		{Command: `webmux launch-config add "Dev server" --icon terminal -- bash -lc 'npm run dev'`, Text: "Add a shell command config."},
	},
	SeeAlso: []string{"webmux launch-config edit", "webmux run --launch-config"},
	Notes:   map[string]Note{"env": {Repeatable: true}},
	Bind: func(fs *flag.FlagSet) Action {
		var f configFields
		bindConfigFields(fs, &f)
		asJSON := fs.Bool("json", false, "print the LaunchConfig, values hidden")
		return func(ctx context.Context, e Env, args []string) int {
			if len(args) < 2 {
				return e.Usagef("a name and a command after -- are required")
			}
			f.set = visited(fs)
			input := contract.LaunchConfigInput{Name: args[0], Icon: f.icon, Cmd: args[1:], Env: map[string]string(f.env), Chat: f.chat}
			if code, ok := e.applyPreview(&input, f); !ok {
				return code
			}
			client, code := e.connect()
			if client == nil {
				return code
			}
			defer client.Close()
			if input.Chat {
				if code, ok := e.checkChat(ctx, client, input.Cmd); !ok {
					return code
				}
			}
			var created contract.LaunchConfig
			if err := client.Do(ctx, http.MethodPost, "/api/launch-configs", input, &created); err != nil {
				return e.Fail(err)
			}
			return e.printSaved(created, *asJSON)
		}
	},
}

func (e Env) applyPreview(input *contract.LaunchConfigInput, f configFields) (int, bool) {
	if !f.set["preview-lines"] && !f.set["preview-skip"] {
		return contract.ExitOK, true
	}
	preview := contract.PreviewSettings{Lines: 10, Skip: 1}
	if input.Preview != nil {
		preview = *input.Preview
	}
	if f.set["preview-lines"] {
		preview.Lines = f.previewLines
	}
	if f.set["preview-skip"] {
		preview.Skip = f.previewSkip
	}
	if preview.Lines < 1 || preview.Lines > 50 || preview.Skip < 0 || preview.Skip > 50 {
		return e.Usagef("--preview-lines is 1-50 and --preview-skip 0-50"), false
	}
	input.Preview = &preview
	return contract.ExitOK, true
}

func (e Env) printSaved(config contract.LaunchConfig, asJSON bool) int {
	if asJSON {
		for name := range config.Env {
			config.Env[name] = hiddenValue
		}
		writeJSON(e.Stdout, config)
	} else {
		_, _ = fmt.Fprintln(e.Stdout, config.ID)
	}
	return contract.ExitOK
}

var launchConfigEditCommand = &Command{
	Name: "launch-config edit", Group: GroupLaunch, Args: "NAME [-- ARGV...]",
	Summary: "Change some fields of a launch config; everything not given keeps its value.",
	Description: "The config is read, the given fields are changed and the whole config is saved again, so environment values " +
		"you do not mention are kept as they are. --env sets or replaces a variable, --unset-env removes one. ARGV after -- " +
		"replaces the command. Turning on --chat checks support first.",
	Arguments: []Item{launchConfigNameArgument, {Name: "ARGV", Text: "A new command and its arguments."}},
	Output:    "The config's ID; with --json, the LaunchConfig with values hidden.",
	Exit:      exitsCommon(ExitItem{Code: 1, Text: "Also: the new name is taken, validation failed, or the chat view is unsupported."}, ExitItem{Code: 2, Text: "Also: no launch config has that name."}),
	Examples: []Example{
		{Command: `webmux launch-config edit "Claude (sandbox)" --unset-env CLAUDE_CONFIG_DIR`, Text: "Remove one variable and keep everything else."},
		{Command: `webmux launch-config edit omp --rename "omp (fast)" --no-chat -- omp --model fast`, Text: "Rename a config and change its command."},
	},
	SeeAlso: []string{"webmux launch-config show", "webmux launch-config add"},
	Notes: map[string]Note{
		"env": {Repeatable: true}, "unset-env": {Repeatable: true},
		"chat": {Conflicts: []string{"no-chat"}}, "no-chat": {Conflicts: []string{"chat"}},
	},
	Bind: func(fs *flag.FlagSet) Action {
		var f configFields
		bindConfigFields(fs, &f)
		rename := fs.String("rename", "", "rename the config to `NEW`")
		var unset stringsFlag
		fs.Var(&unset, "unset-env", "remove environment variable `NAME`")
		noChat := fs.Bool("no-chat", false, "turn the chat view off")
		asJSON := fs.Bool("json", false, "print the LaunchConfig, values hidden")
		return func(ctx context.Context, e Env, args []string) int {
			if len(args) < 1 {
				return e.Usagef("a name is required")
			}
			f.set = visited(fs)
			if f.chat && *noChat {
				return e.Usagef("--chat and --no-chat cannot be combined")
			}
			client, code := e.connect()
			if client == nil {
				return code
			}
			defer client.Close()
			config, code, ok := e.launchConfig(ctx, client, args[0])
			if !ok {
				return code
			}
			input := contract.LaunchConfigInput{Name: config.Name, Icon: config.Icon, Cmd: config.Cmd, Env: config.Env, Preview: config.Preview, Chat: config.Chat}
			if input.Env == nil {
				input.Env = map[string]string{}
			}
			if f.set["rename"] {
				input.Name = *rename
			}
			if f.set["icon"] {
				input.Icon = f.icon
			}
			if len(args) > 1 {
				input.Cmd = args[1:]
			}
			maps.Copy(input.Env, f.env)
			for _, name := range unset {
				if _, ok := input.Env[name]; !ok {
					_, _ = fmt.Fprintf(e.Stderr, "webmux: %s is not set; nothing to remove\n", name)
				}
				delete(input.Env, name)
			}
			switch {
			case f.chat:
				input.Chat = true
			case *noChat:
				input.Chat = false
			}
			if code, ok := e.applyPreview(&input, f); !ok {
				return code
			}
			if input.Chat && (f.chat || len(args) > 1) {
				if code, ok := e.checkChat(ctx, client, input.Cmd); !ok {
					return code
				}
			}
			var saved contract.LaunchConfig
			if err := client.Do(ctx, http.MethodPut, "/api/launch-configs/"+url.PathEscape(config.ID), input, &saved); err != nil {
				return e.Fail(err)
			}
			return e.printSaved(saved, *asJSON)
		}
	},
}

var launchConfigRemoveCommand = &Command{
	Name: "launch-config rm", Group: GroupLaunch, Args: "NAME",
	Summary:   "Delete a launch config; sessions already started from it keep running.",
	Arguments: []Item{launchConfigNameArgument},
	Exit:      exitsCommon(ExitItem{Code: 2, Text: "Also: no launch config has that name, or no terminal to confirm and no --yes."}),
	Examples: []Example{
		{Command: `webmux launch-config rm "Dev server"`, Text: "Delete a config after confirming."},
		{Command: `webmux launch-config rm "Dev server" --yes`, Text: "Delete it in a script."},
	},
	SeeAlso: []string{"webmux launch-config restore", "webmux launch-config ls"},
	Bind: func(fs *flag.FlagSet) Action {
		yes := fs.Bool("yes", false, "do not ask for confirmation")
		return func(ctx context.Context, e Env, args []string) int {
			if len(args) != 1 {
				return e.Usagef("exactly one name is required")
			}
			client, code := e.connect()
			if client == nil {
				return code
			}
			defer client.Close()
			config, code, ok := e.launchConfig(ctx, client, args[0])
			if !ok {
				return code
			}
			if !*yes {
				if code, ok := e.confirm(fmt.Sprintf("Delete launch config %q (%s)?", config.Name, strings.Join(config.Cmd, " "))); !ok {
					return code
				}
			}
			if err := client.Do(ctx, http.MethodDelete, "/api/launch-configs/"+url.PathEscape(config.ID), nil, nil); err != nil {
				return e.Fail(err)
			}
			return contract.ExitOK
		}
	},
}

var launchConfigPresetsCommand = &Command{
	Name: "launch-config presets", Group: GroupLaunch,
	Summary: "List the built-in launch presets that restore can bring back.",
	Output:  "A table: NAME, CHAT, COMMAND; with --json, the LaunchPreset array.",
	Exit:    exitsCommon(),
	Examples: []Example{
		{Command: "webmux launch-config presets", Text: "See the built-in agent configs."},
		{Command: "webmux launch-config presets --json | jq -r '.[].name'", Text: "List their names."},
	},
	SeeAlso: []string{"webmux launch-config restore"},
	Bind: func(fs *flag.FlagSet) Action {
		asJSON := fs.Bool("json", false, "print the LaunchPreset array")
		return func(ctx context.Context, e Env, args []string) int {
			if len(args) != 0 {
				return e.Usagef("launch-config presets takes no arguments")
			}
			client, code := e.connect()
			if client == nil {
				return code
			}
			defer client.Close()
			var presets []contract.LaunchPreset
			if err := client.Do(ctx, http.MethodGet, "/api/launch-configs/presets", nil, &presets); err != nil {
				return e.Fail(err)
			}
			if *asJSON {
				writeJSON(e.Stdout, presets)
				return contract.ExitOK
			}
			w := tabwriter.NewWriter(e.Stdout, 0, 4, 2, ' ', 0)
			_, _ = fmt.Fprintln(w, "NAME\tCHAT\tCOMMAND")
			for _, p := range presets {
				chat := "-"
				if p.Chat {
					chat = "yes"
				}
				_, _ = fmt.Fprintf(w, "%s\t%s\t%s\n", p.Name, chat, strings.Join(p.Cmd, " "))
			}
			_ = w.Flush()
			return contract.ExitOK
		}
	},
}

var launchConfigRestoreCommand = &Command{
	Name: "launch-config restore", Group: GroupLaunch, Args: "[PRESET]",
	Summary:   "Restore every missing built-in preset, or the one named PRESET.",
	Arguments: []Item{{Name: "PRESET", Text: "A preset name from webmux launch-config presets."}},
	Output:    "The names of the configs now saved; with --json, the LaunchConfigSummary array.",
	Exit:      exitsCommon(ExitItem{Code: 1, Text: "Also: no preset has that name."}),
	Examples: []Example{
		{Command: "webmux launch-config restore", Text: "Bring back every deleted preset."},
		{Command: `webmux launch-config restore "Claude Code"`, Text: "Restore one preset."},
	},
	SeeAlso: []string{"webmux launch-config presets", "webmux launch-config rm"},
	Bind: func(fs *flag.FlagSet) Action {
		asJSON := fs.Bool("json", false, "print the LaunchConfigSummary array")
		return func(ctx context.Context, e Env, args []string) int {
			if len(args) > 1 {
				return e.Usagef("at most one preset is allowed")
			}
			request := contract.RestoreLaunchPresetsRequest{}
			if len(args) == 1 {
				request.Name = args[0]
			}
			client, code := e.connect()
			if client == nil {
				return code
			}
			defer client.Close()
			var list []contract.LaunchConfigSummary
			if err := client.Do(ctx, http.MethodPost, "/api/launch-configs/presets", request, &list); err != nil {
				return e.Fail(err)
			}
			if *asJSON {
				writeJSON(e.Stdout, list)
				return contract.ExitOK
			}
			for _, c := range list {
				_, _ = fmt.Fprintln(e.Stdout, c.Name)
			}
			return contract.ExitOK
		}
	},
}
