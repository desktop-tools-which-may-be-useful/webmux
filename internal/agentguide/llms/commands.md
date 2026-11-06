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

Create a terminal session and attach to it, or print its ID with --detach.

Without --launch-config, the arguments after -- are the session's argv exactly as given; nothing is expanded by a shell. A custom command inherits the caller's environment, minus terminal identity variables, with --env on top. With --launch-config, the server starts the saved Web UI launch config of that exact name, including its environment, which never reaches the client.

Arguments:

- `COMMAND [ARG]...`: The program and its arguments, after --.

Options:

- `--config-id value`: Alias for --launch-config. Conflicts with --env.
- `--cwd DIR`: Working DIR on the server (default: the current directory)
- `--detach`: Create the session and print its ID and link without attaching.
- `--env NAME=VALUE`: Set NAME=VALUE in the session, overriding the inherited environment. Repeatable. Conflicts with --launch-config.
- `--launch-config NAME`: Use a saved Web UI launch config by NAME (instead of -- COMMAND) Conflicts with --env.
- `--name NAME`: Session NAME.

Examples:

- `webmux run -- claude`: Start Claude Code in the current directory and attach; Ctrl+] detaches.
- `webmux run --detach --name tests --cwd ~/src/app -- make test`: Start a named session in another directory and print its ID and link.
- `webmux run --detach --launch-config "Claude Code"`: Start a saved Web UI launch config in the background.

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
