# 命令参考

本页由命令表生成，与 `webmux <命令> --help` 一致（帮助正文为英文）。退出码见 [退出码](/llms/exit-codes.md)。

## webmux serve

`webmux serve [options]`

Run the server in the foreground (the default without a command).

A missing default configuration file is created with a generated token. The server stops on SIGINT or SIGTERM, which ends every terminal session.

Options:

- `--config PATH`: TOML configuration PATH. Default: ~/.config/webmux/config.toml.
- `--listen HOST:PORT`: HTTP listen HOST:PORT, overriding the configuration.
- `--log-file PATH`: Write stdout and stderr to this PATH, keeping the previous one as .1.
- `--token TOKEN`: Shared access TOKEN, overriding the configuration.
- `--tunnel`: Expose WebMux through a Cloudflare Tunnel.

Examples:

- `webmux serve`: Serve with ~/.config/webmux/config.toml.
- `webmux serve --listen 127.0.0.1:8080 --log-file ~/webmux.log`: Serve on another port and log to a file.

## webmux start

`webmux start [options]`

Run the server in the background; stop it with webmux stop.

The child is a normal serve process in its own session with a log file. When an instance already runs, start prints its status instead.

Options:

- `--config PATH`: TOML configuration PATH. Default: ~/.config/webmux/config.toml.
- `--listen HOST:PORT`: HTTP listen HOST:PORT, overriding the configuration.
- `--log-file PATH`: Write stdout and stderr to this PATH, keeping the previous one as .1.
- `--token TOKEN`: Shared access TOKEN, overriding the configuration.
- `--tunnel`: Expose WebMux through a Cloudflare Tunnel.

Examples:

- `webmux start`: Start a background server with the default configuration.
- `webmux start --config ~/work/webmux.toml`: Start an instance with another configuration.

## webmux version

`webmux version [options]`

Print the version; --json adds the control contract.

Options:

- `--json`: Print the version and control contract as JSON.

Examples:

- `webmux version`: Print the version.
- `"$WEBMUX_BIN" version --json`: Check the version of the executable a session's server runs.

## webmux status

`webmux status [options]`

Show server health, sessions and access links.

Options:

- `--config PATH`: TOML configuration PATH. Default: ~/.config/webmux/config.toml.
- `--json`: Print the status contract v1 as JSON.
- `--reveal-token`: Include the token in the JSON output.

Examples:

- `webmux status`: Show whether WebMux runs and its links.
- `webmux status --json | jq -r .urls.local`: Print the local link for a script.

## webmux stop

`webmux stop [options]`

Stop the server through its local control API (ends all terminal sessions).

A server managed by systemd or launchd is stopped only with --force; use webmux service stop instead. Processes inside a WebMux session cannot stop the server.

Options:

- `--config PATH`: TOML configuration PATH. Default: ~/.config/webmux/config.toml.
- `--force`: Stop an instance managed by systemd or launchd.
- `--json`: Print the stop result as JSON.
- `--timeout DURATION`: Wait this DURATION before SIGKILL. Default: 15s.

Examples:

- `webmux stop`: Stop a server started with webmux start.
- `webmux stop --timeout 30s --json`: Allow sessions more time to exit and print the result as JSON.

## webmux bridge

`webmux bridge [options]`

Bridge the local API socket over standard input and output.

The Windows console client and SSH use it to reach the local API of a remote or WSL instance: ssh host webmux bridge carries every session command without a token.

Options:

- `--config PATH`: TOML configuration PATH. Default: ~/.config/webmux/config.toml.

Examples:

- `webmux bridge`: Serve one local API connection over stdio.
- `webmux bridge --config ~/work/webmux.toml`: Bridge the instance of another configuration.

## webmux config path

`webmux config path [options]`

Print the configuration file path and whether it exists.

Options:

- `--config PATH`: TOML configuration PATH. Default: ~/.config/webmux/config.toml.
- `--json`: Print JSON.

Examples:

- `webmux config path`: Print the default configuration path.
- `webmux config path --json`: Print the path and whether it exists as JSON.

## webmux config show

`webmux config show [options]`

Show the effective configuration without secrets.

Options:

- `--config PATH`: TOML configuration PATH. Default: ~/.config/webmux/config.toml.
- `--json`: Print the safe configuration view as JSON.
- `--live`: Read the running process instead of disk.

Examples:

- `webmux config show`: Show the configuration on disk.
- `webmux config show --live --json`: Show what the running server uses, as JSON.

## webmux config validate

`webmux config validate [options]`

Validate the configuration file (alias: config check).

Options:

- `--config PATH`: TOML configuration PATH. Default: ~/.config/webmux/config.toml.
- `--json`: Print JSON.

Examples:

- `webmux config validate`: Check the default configuration.
- `webmux config check --config ./webmux.toml --json`: Check another file and print JSON.

## webmux config init

`webmux config init [options]`

Write a new configuration file.

Options:

- `--config PATH`: TOML configuration PATH. Default: ~/.config/webmux/config.toml.
- `--force`: Keep an existing file as .bak and replace it.
- `--listen string`: Listen address to write.

Examples:

- `webmux config init`: Create the default configuration.
- `webmux config init --listen 0.0.0.0:7681 --force`: Replace it, listening on every interface.

## webmux config listen

`webmux config listen [options] [ADDR]`

Change the listen address in the configuration file.

Arguments:

- `ADDR`: The HOST:PORT to listen on; the same as --listen.

Options:

- `--config PATH`: TOML configuration PATH. Default: ~/.config/webmux/config.toml.
- `--listen string`: Listen address to write.

Examples:

- `webmux config listen 127.0.0.1:8080`: Move the server to port 8080 after the next restart.
- `webmux config listen --listen 0.0.0.0:7681`: Listen on every interface.

## webmux service install

`webmux service install [options]`

Install and start a service for the current user.

Options:

- `--config string`: TOML configuration path (default is created if missing) Default: ~/.config/webmux/config.toml.
- `--dry-run`: Print the service definition without changing anything.
- `--force`: Replace the service definition and restart (ends terminal sessions)

Examples:

- `webmux service install`: Install and start the user service.
- `webmux service install --dry-run`: Show the unit or plist without installing it.

## webmux service uninstall

`webmux service uninstall [options]`

Stop and remove the user service; keep configuration.

Examples:

- `webmux service uninstall`: Stop and remove the user service; keep configuration.
- `webmux service uninstall && webmux status`: Then show the server status.

## webmux service start

`webmux service start [options]`

Start the installed user service.

Examples:

- `webmux service start`: Start the installed user service.
- `webmux service start && webmux status`: Then show the server status.

## webmux service stop

`webmux service stop [options]`

Stop the user service (ends all terminal sessions).

Examples:

- `webmux service stop`: Stop the user service (ends all terminal sessions).
- `webmux service stop && webmux status`: Then show the server status.

## webmux service restart

`webmux service restart [options]`

Restart the user service (ends all terminal sessions).

Examples:

- `webmux service restart`: Restart the user service (ends all terminal sessions).
- `webmux service restart && webmux status`: Then show the server status.

## webmux service status

`webmux service status [options]`

Show the user service manager's status.

Examples:

- `webmux service status`: Show the user service manager's status.
- `webmux service status && webmux status`: Then show the server status.

## webmux service logs

`webmux service logs [options]`

Show recent service logs; --follow streams them.

Options:

- `-f, --follow`: Stream log output until interrupted.

Examples:

- `webmux service logs`: Show recent logs.
- `webmux service logs -f`: Stream logs until Ctrl+C.

## webmux update check

`webmux update check [options]`

Check for a newer signed release.

Options:

- `--json`: Print the result as JSON.

Examples:

- `webmux update check`: Check the configured channel.
- `webmux update check --json | jq .available`: Check from a script.

## webmux update apply

`webmux update apply [options]`

Install a newer deb or rpm release (root; used by webmux-update.timer).

Examples:

- `sudo webmux update apply`: Install the latest release now.
- `systemctl start webmux-update.service`: Let the update service run it.

## webmux update channel

`webmux update channel [options] [stable|dev]`

Show the update channel, or set it to stable or dev (root).

Arguments:

- `stable|dev`: The channel to switch to; omit it to print the current one.

Examples:

- `webmux update channel`: Print the current channel.
- `sudo webmux update channel dev`: Follow development builds.

## webmux help

`webmux help [options] [COMMAND...]`

Show help for all commands or one command; --json prints the command tree.

Arguments:

- `COMMAND...`: A command path such as ls or update check.

Options:

- `--json`: Print the commands, options and examples as JSON.

Examples:

- `webmux help get`: Show the help of webmux get.
- `webmux help --json | jq -r '.commands[].path | join(" ")'`: List every command path.

## webmux run

`webmux run [options] [-- COMMAND [ARG]...]`

Create a terminal session; attach to it, detach, or follow its output.

Without --launch-config, the arguments after -- are the session's argv exactly as given; nothing is expanded by a shell. A custom command inherits the caller's environment, minus terminal identity and WEBMUX_* variables, with --env on top. With --launch-config, the server starts the saved Web UI launch config of that exact name, including its environment, which never reaches the client.

Location matches the Web UI's new session dialog: the current directory (associated with the registered worktree it lies in, as the server does for any session), an existing worktree (--worktree), or a new one (--new-worktree), which is pinned to the home page like the Web UI does.

Without a terminal (an agent's tool call, a pipe) run detaches instead of failing; --attach insists on attaching. --follow streams the output to standard output without a terminal and exits with the program's exit status; Ctrl+C only stops following unless --kill-on-interrupt.

Arguments:

- `COMMAND [ARG]...`: The program and its arguments, after --.

Options:

- `--attach`: Attach even when that needs a terminal this process lacks (exit 2) Conflicts with --detach, --follow.
- `--base REF`: Start the new branch at REF (default: the repository's default base) Requires --new-worktree.
- `--config-id value`: Alias for --launch-config. Conflicts with --env.
- `--cwd DIR`: Working DIR on the server (default: the current directory)
- `--detach`: Create the session and print its ID and link without attaching. Conflicts with --attach, --follow.
- `--dry-run`: Print the request without creating anything; environment values are hidden.
- `--env NAME=VALUE`: Set NAME=VALUE in the session, overriding the inherited environment. Repeatable. Conflicts with --launch-config.
- `--existing-branch`: Check out an existing branch instead of creating it.
- `--follow`: Stream the output to standard output and exit with the program's status. Conflicts with --detach, --attach.
- `--init`: Put the repository's initialization instruction before the first message. Requires --launch-config.
- `--json`: Print {session, link} as JSON when detached.
- `--kill-on-interrupt`: With --follow, end the program on SIGINT or SIGTERM.
- `--label KEY=VALUE`: Set label KEY=VALUE on the session. Repeatable.
- `--launch-config NAME`: Use a saved Web UI launch config by NAME (instead of -- COMMAND) Conflicts with --env.
- `--name NAME`: Session NAME.
- `--new-worktree BRANCH`: Create a worktree for BRANCH and start in it. Conflicts with --worktree, --cwd.
- `--no-pin`: Do not show the new worktree on the home page.
- `--prompt TEXT`: First message TEXT (- reads standard input); typed with Enter on the terminal side.
- `--refresh-base`: Fetch the base branch's upstream first and start from it.
- `--repo string`: Repository of the new worktree: an ID, a path or . (default: the current directory's)
- `--start-side SIDE`: Start a hybrid launch config on SIDE: terminal or chat (chat implies --detach)
- `--strip`: With --follow, remove terminal control sequences.
- `--timeout DURATION`: Give up --wait after this DURATION (exit 124); 0 waits forever.
- `--wait STATE`: Detach, then wait until the session is first in STATE (see webmux wait) Conflicts with --attach, --follow.
- `--worktree REPO:WORKTREE`: Start in a registered worktree, as REPO:WORKTREE or its path. Conflicts with --new-worktree.
- `--worktree-path DIR`: Put the new worktree at DIR (default: <repository>.worktrees/<branch leaf>)

Examples:

- `webmux run -- claude`: Start Claude Code in the current directory and attach; Ctrl+] detaches.
- `webmux run --detach --name tests --cwd ~/src/app -- make test`: Start a named session in another directory and print its ID and link.
- `webmux run --follow --strip -- go test ./...`: Run tests in a persistent session, stream plain text and exit with their status.
- `webmux run --new-worktree fix/lint --base main --launch-config "Claude Code" --start-side chat --init --prompt "Fix lint" --json`: Create a worktree and start an agent there with a first task.
- `webmux run --dry-run --label run=7 -- make build`: Show the request run would send, without creating anything.

## webmux attach

`webmux attach [options] SESSION`

Attach this terminal to a session; Ctrl+] detaches and the session keeps running.

The terminal takes control of the session's input and size unless --observe is given. A hybrid session on its chat side cannot be attached.

Arguments:

- `SESSION`: A session ID, or "self" for the session this command runs in ($WEBMUX_SESSION_ID).

Options:

- `--observe`: Watch without taking control, sending input or resizing.

Examples:

- `webmux attach abcd2345`: Take over session abcd2345 in this terminal.
- `webmux attach --observe abcd2345`: Watch the session without sending input or resizing it.

## webmux ls

`webmux ls [options]`

List terminal sessions, optionally filtered by label, parent or status.

Filters combine: a session is listed only when it matches all of them. --label KEY=VALUE matches a label value, --label KEY only requires the label to exist.

Options:

- `--json`: Print the server's session list as JSON.
- `--label KEY=VALUE`: Only sessions with label KEY=VALUE, or with label KEY. Repeatable.
- `--parent SESSION`: Only sessions created by SESSION (or self)
- `--status STATE`: Only sessions in STATE: running, waiting, idle or exited. Repeatable.

Examples:

- `webmux ls`: Show every session.
- `webmux ls --parent self --json`: List the sessions this session created, as JSON.
- `webmux ls --label run=42 --status exited`: Show finished sessions of one scripted run.

## webmux open

`webmux open [options] SESSION`

Print a session's local link, and open it in the browser with --browser.

Arguments:

- `SESSION`: A session ID, or "self" for the session this command runs in ($WEBMUX_SESSION_ID).

Options:

- `--browser`: Also open the link in the default browser.

Examples:

- `webmux open abcd2345`: Print the link of session abcd2345.
- `webmux open --browser self`: Open the current session in the default browser.

## webmux get

`webmux get [options] [SESSION]`

Show a session's metadata: repository, branch, launch config, hybrid state, links, parent, children and labels.

Inside a WebMux session SESSION defaults to self. The details are gathered by the server in one request (GET /api/sessions/{id}/details); they never include the token or launch environment values. The artifacts folder is reported only when it exists; get never creates it.

Arguments:

- `SESSION`: A session ID, or "self" for the session this command runs in ($WEBMUX_SESSION_ID).

Options:

- `--field PATH`: Print only the value at dotted PATH, such as worktree.branch. Conflicts with --json.
- `--json`: Print the SessionDetails object as JSON. Conflicts with --field.

Examples:

- `webmux get`: Inside a session, show where it runs and what started it.
- `branch=$(webmux get --field worktree.branch)`: Read the current session's branch without jq.
- `webmux get abcd2345 --json`: Print the full details of another session as JSON.

## webmux label

`webmux label [options] SESSION KEY=VALUE|KEY-...`

Set or remove labels on a session; KEY- removes KEY.

Labels are key-value pairs for scripts to mark and find sessions, kept in memory with the session. Keys match [a-z0-9][a-z0-9._-]{0,62}; values hold at most 256 characters and no control characters; a session holds at most 16 labels. All changes apply together or not at all.

Arguments:

- `SESSION`: A session ID, or "self" for the session this command runs in ($WEBMUX_SESSION_ID).
- `KEY=VALUE`: Set label KEY to VALUE.
- `KEY-`: Remove label KEY.

Options:

- `--json`: Print the updated SessionInfo as JSON.

Examples:

- `webmux label self task=lint run=42`: Mark the current session.
- `webmux label abcd2345 task-`: Remove the task label from a session.

## webmux dir

`webmux dir [options] [SESSION]`

Print a session's artifacts folder, creating it when needed.

The artifacts folder is where the user and the programs of a session exchange files: uploads land in its uploads/ subdirectory, and files saved there show in the code panel's Temporary folder. Inside a session SESSION defaults to self.

Arguments:

- `SESSION`: A session ID, or "self" for the session this command runs in ($WEBMUX_SESSION_ID).

Examples:

- `cp report.html "$(webmux dir self)/"`: Hand a report to the user.
- `ls "$(webmux dir)/uploads"`: List the files the user uploaded to this session.

## webmux guide

`webmux guide [options] [TOPIC]`

Print the WebMux shell guide for scripts and agents (the llms.txt index, a topic, or everything).

The guide is served by the running server at /llms.txt, /llms/<topic>.md and /llms-full.txt; this command reads it over the local socket, so it matches the server's version and needs no token. When the server is not running, the copy built into this executable is printed with a note on standard error.

Arguments:

- `TOPIC`: One of: connect, sessions, launch, artifacts, commands, exit-codes, rules, recipes.

Options:

- `--full`: Print every topic after the index (/llms-full.txt) Conflicts with --path.
- `--path`: Print the server's executable and local API socket. Conflicts with --full.

Examples:

- `webmux guide`: Print the index of the guide.
- `webmux guide artifacts`: Read where to put files for the user.
- `webmux guide --full > ~/.claude/skills/webmux/reference.md`: Save the whole guide as a reference for a skill.

## webmux chat new

`webmux chat new [options] TEXT|-`

Quick start from the shell: a new worktree, an agent on its chat side, its settings and the first message.

The same as the Web UI's New chat (POST /api/quick-starts): one call creates a worktree with a generated branch, starts the agent of a hybrid launch config, applies --set options and the permission mode, and sends TEXT with the repository's initialization instruction. A failure reports the step that failed and what was already created; nothing is rolled back.

--wait then waits for the first turn and prints the agent's final reply, like webmux chat send --wait.

Arguments:

- `TEXT`: The first message; - reads standard input. Required, also with --init.

Options:

- `--agent NAME`: Hybrid launch config NAME (default: the only one)
- `--attach PATH`: Attach the file at PATH to the first message. Repeatable. Conflicts with --terminal.
- `--auto-wait-requests`: With --wait, keep waiting while a permission request or form is open.
- `--base REF`: Start the branch at REF (default: the repository's default base)
- `--branch NAME`: New branch NAME (default: generated like the Web UI, wt/...)
- `--init`: Send the repository's initialization instruction first (the default when it has one) Conflicts with --no-init.
- `--json`: Print JSON.
- `--label KEY=VALUE`: Set label KEY=VALUE on the new session. Repeatable.
- `--no-init`: Do not send the initialization instruction. Conflicts with --init.
- `--permission MODE`: Permission MODE: ask or auto. Conflicts with --terminal.
- `--repo string`: Repository: an ID, a path or . (default: the current directory's)
- `--set ID=VALUE`: Agent option ID=VALUE (see webmux launch-config options) Repeatable. Conflicts with --terminal.
- `--terminal`: Start on the terminal side and type the message with Enter. Conflicts with --set, --permission, --attach, --wait.
- `--timeout DURATION`: Give up --wait after this DURATION (exit 124); 0 waits forever.
- `--wait`: Wait for the first turn and print the agent's final reply. Conflicts with --terminal.

Examples:

- `webmux chat new --agent "Claude Code" "Add a POST /books endpoint"`: Start Claude Code on a new worktree of the current repository.
- `webmux chat new --agent Codex --label task=lint --wait "Fix all lint and summarize the changes"`: Delegate a task and print the agent's reply when it is done.
- `webmux chat new --repo app --agent omp --set model=smart --permission ask --attach design.png -`: Pick a model, ask before tools run, attach a file and read the task from standard input.

## webmux launch-config options

`webmux launch-config options [options] NAME`

List the options (model, mode, ...) the agent of a hybrid launch config offers.

The agent reports its options from a throwaway session in a repository's main worktree (POST /api/quick-starts/options). Use the IDs and values with webmux chat new --set ID=VALUE.

Arguments:

- `NAME`: The exact name of a launch config with hybrid mode.

Options:

- `--json`: Print the ChatConfig array as JSON.
- `--repo string`: Repository whose main worktree the agent runs in: an ID, a path or . (default: the current directory's)

Examples:

- `webmux launch-config options "Claude Code"`: Show the agent's options and their values.
- `webmux launch-config options Codex --json | jq '.[] | select(.id=="model") | .options[].value'`: List the models to pass to chat new --set model=...

## webmux api

`webmux api [options] PATH`

Send one request to the local API and print the response body unchanged.

The escape hatch for endpoints without a dedicated command: any /api/ or /local/v1/ path, over the local socket, with no token. The method is GET, or POST when -d is given. The body must be JSON; multipart uploads are not supported. Restrictions on callers inside a session (stopping the server, reading the token, tunnel and Tailscale) still apply. WebSocket paths are refused: use webmux events, logs, screen or chat watch.

Arguments:

- `PATH`: An /api/ or /local/v1/ path, with an optional query string.

Options:

- `--X METHOD`: HTTP METHOD (default GET, or POST with -d)
- `--d DATA`: JSON request body: literal DATA, @FILE, or - for standard input.

Examples:

- `webmux api /api/launch-configs`: List the saved launch configs as JSON.
- `webmux api -X PATCH /api/sessions/abcd2345 -d '{"name":"review"}'`: Rename a session through the raw API.
- `webmux api -X DELETE /api/launch-configs/k3m9q2x7`: Delete a launch config by ID.

## webmux kill

`webmux kill [options] SESSION`

End a session's process; the session stays listed as exited.

The process group gets SIGHUP, then SIGKILL if it does not exit. Use rm to also remove the session.

Arguments:

- `SESSION`: A session ID, or "self" for the session this command runs in ($WEBMUX_SESSION_ID).

Examples:

- `webmux kill abcd2345`: Stop a runaway command.
- `webmux ls --parent self --status running --json | jq -r '.[].id' | xargs -n1 webmux kill`: End every running session this session created.

## webmux rm

`webmux rm [options] SESSION`

Remove a session; a running one is ended first.

The session disappears from every client. Its artifacts folder is deleted unless a hybrid record can still resume it.

Arguments:

- `SESSION`: A session ID, or "self" for the session this command runs in ($WEBMUX_SESSION_ID).

Examples:

- `webmux rm abcd2345`: Remove a session.
- `webmux ls --label run=42 --json | jq -r '.[].id' | xargs -n1 webmux rm`: Clean up every session of one scripted run.

## webmux rename

`webmux rename [options] SESSION [NAME]`

Name a session; an empty NAME hands the title back to the terminal or agent.

Arguments:

- `SESSION`: A session ID, or "self" for the session this command runs in ($WEBMUX_SESSION_ID).
- `NAME`: At most 512 bytes, no control characters.

Examples:

- `webmux rename self "lint fixes"`: Name the current session.
- `webmux rename abcd2345`: Clear the name so the terminal title shows again.

## webmux send

`webmux send [options] SESSION [TEXT|-]`

Type text and keys into a terminal session, confirmed by the server.

TEXT goes in one acknowledged submission (the same path as the Web UI's draft send); with --enter, Enter is part of it. The command exits 0 only once the server reports the input written to the PTY. --key sends named keys after the text. Sending takes control of the terminal, as typing does in the browser; the control indicator names this client "script" and, inside a session, that session.

--wait subscribes to the session list before sending and returns on the first change into STATE after the input, so a stale idle state cannot end the wait early. A hybrid session on its chat side takes webmux chat send instead.

Arguments:

- `SESSION`: A session ID, or "self" for the session this command runs in ($WEBMUX_SESSION_ID).
- `TEXT`: The text to type; - reads standard input.

Options:

- `--enter`: Press Enter after the text, in the same submission.
- `--json`: With --wait, print the SessionInfo that matched.
- `--key KEY`: Send the named KEY after the text: backspace, ctrl-c, ctrl-d, down, enter, esc, left, right, tab, up. Repeatable.
- `--timeout DURATION`: Give up --wait after this DURATION (exit 124); 0 waits forever. Requires --wait.
- `--wait STATE`: Wait for the first change into STATE after sending (see webmux wait)

Examples:

- `webmux send abcd2345 'make test' --enter --wait idle --timeout 10m`: Run a command in a shell session and wait until it is idle again.
- `webmux send abcd2345 --key esc`: Interrupt an agent's turn.
- `git diff | webmux send abcd2345 - --enter`: Paste standard input into an agent and submit it.

## webmux wait

`webmux wait [options] SESSION...`

Wait until sessions reach a state: running, waiting, idle, exited or attention.

Subscribes to the session list (/ws/sessions) and returns when every session (or with --any, one) is in one of the --until states; attention means the session has a latched attention signal. Without --changed the current state counts; with --changed only a change after subscribing does. --stable requires the condition to hold that long, which filters an agent's brief idle moments between tool calls.

Arguments:

- `SESSION`: A session ID, or "self" for the session this command runs in ($WEBMUX_SESSION_ID).

Options:

- `--any`: Return when one session matches instead of all.
- `--changed`: Ignore the state at subscription; wait for a change into the states.
- `--json`: Print the matching SessionInfo.
- `--stable DURATION`: Require the condition to hold for this DURATION.
- `--timeout DURATION`: Give up after this DURATION (exit 124); 0 waits forever.
- `--until STATES`: Comma-separated STATES to wait for (required)

Examples:

- `webmux wait abcd2345 --until attention,idle --changed --timeout 30m`: Wait until an agent needs you or finishes its turn.
- `webmux wait abcd2345 --until exited --json | jq .exitCode`: Wait for a command and read its exit code.
- `webmux wait a1 b2 c3 --until idle --stable 5s --any`: Return when the first of three agents has been idle for five seconds.

## webmux logs

`webmux logs [options] SESSION`

Print a session's terminal output; --follow keeps streaming as an observer.

Prints the session's retained output (GET /api/sessions/{id}/buffer) as raw bytes. --strip removes terminal control sequences and keeps line text, which suits line-oriented programs but not full-screen TUIs; use webmux screen for those. --follow connects as an observer, prints the replay and then new output, never taking control or resizing, until the session exits.

Arguments:

- `SESSION`: A session ID, or "self" for the session this command runs in ($WEBMUX_SESSION_ID).

Options:

- `--follow`: Keep printing new output until the session exits. Conflicts with --tail.
- `--strip`: Remove terminal control sequences.
- `--tail N`: Print only the last N lines. Conflicts with --follow.

Examples:

- `webmux logs abcd2345 --strip --tail 50`: Show the last 50 lines as plain text.
- `webmux logs self --follow --strip | grep --line-buffered ERROR`: Watch a session's output for errors.

## webmux screen

`webmux screen [options] SESSION`

Print the text a terminal session shows right now, rendered on the client.

Connects as an observer, replays the session into a terminal emulator inside this command at the controller's size, and prints the visible screen; the server does no terminal emulation. Use it to see a full-screen TUI's state, such as an agent waiting on a permission prompt. A truncated replay may leave the screen incomplete; full-screen programs usually redraw.

Arguments:

- `SESSION`: A session ID, or "self" for the session this command runs in ($WEBMUX_SESSION_ID).

Options:

- `--cursor`: Write the cursor position to standard error.
- `--json`: Print size, cursor, alternate-screen state and lines as JSON.

Examples:

- `webmux screen abcd2345`: See what an agent's terminal shows.
- `webmux screen abcd2345 --json | jq -r '.lines[-1]'`: Read the last line of the screen.

## webmux events

`webmux events [options]`

Print session list events as NDJSON until interrupted.

Each /ws/sessions event (snapshot, created, updated, removed, forwards) is written as one JSON line, unchanged. With --session, only events about those sessions are written and the snapshot keeps only them. After a lost connection the command reconnects and writes a new snapshot.

Options:

- `--session SESSION`: Only events about SESSION (or self) Repeatable.

Examples:

- `webmux events | jq -c 'select(.type=="updated") | {id: .session.id, status: .session.status}'`: Print every status change.
- `webmux events --session abcd2345`: Follow one session's events.

## webmux completion

`webmux completion [options] bash|zsh|fish`

Print a shell completion script for bash, zsh or fish.

The script completes commands, options and enumerated values, and asks the running server for session IDs (with titles in zsh and fish), launch config names, repositories, worktrees and topics. Without a server, or when it does not answer within a second, only the static candidates are offered and nothing is printed as an error.

Arguments:

- `SHELL`: bash, zsh or fish.

Examples:

- `echo 'source <(webmux completion bash)' >> ~/.bashrc`: Enable completion in bash.
- `webmux completion zsh > ~/.zfunc/_webmux`: Install zsh completion (with ~/.zfunc in fpath before compinit).
- `webmux completion fish > ~/.config/fish/completions/webmux.fish`: Install fish completion.
