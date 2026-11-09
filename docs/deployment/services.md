# 服务管理与平台差异

把 WebMux 作为后台服务运行：服务管理命令、systemd、launchd、Windows（WSL）指引、agent 的 PATH 与平台差异。

## 服务管理命令

先将二进制放到稳定位置（源码构建可执行 `make install`），再以日常用户运行，不要使用 sudo：

```sh
webmux service install
webmux service status
webmux service logs            # 最近 100 行
webmux service logs --follow   # 持续查看，Ctrl-C 退出；也支持 -f
webmux service stop
webmux service start
webmux service restart
webmux service uninstall
```

- `install` 写入用户服务定义并立即启用、启动；缺少默认配置时与首次 `serve` 共用同一生成逻辑，创建带随机 token 的 0600 配置后重新读取并校验。显式 `--config PATH` 指定的文件必须已经存在且有效，不会自动创建。
- 服务使用当前可执行文件和配置文件的绝对路径，不复制二进制。移动二进制后，需在新位置执行 `service install --force` 更新定义。
- 已有服务定义需要 `service install --force` 才能替换，并会重启服务。`service install --dry-run` 只输出服务定义，不写文件、不创建配置、不操作服务；可与 `--config` 配合使用。
- `stop`、`restart`、覆盖安装和卸载会结束当前所有终端会话，滚屏不会在重启后恢复。`uninstall` 保留二进制、配置、日志和状态目录。
- 安装会保存当前 PATH 中的绝对目录，并补充常见 agent 安装目录，只保存 PATH、shell、locale 和 `XDG_STATE_HOME` 等指定环境变量。更新 agent 安装路径后可重新执行 `install --force`。
- 服务工作目录为用户主目录；配置中的 `default_cwd`、`state_dir` 建议使用绝对路径或支持的 `~` 路径；自定义启动命令的可执行程序使用绝对路径或 PATH 中的命令名，避免依赖安装时的工作目录。
- Linux 需要可用的 systemd 用户管理器；macOS 需要已登录的 GUI 会话。服务启用成功不代表 HTTP 端口已成功监听，遇到端口冲突等启动失败时查看 `status` 和 `logs`。

`webmux help` 显示命令概览；`webmux service install --help` 等显示子命令参数。

## systemd 用户服务（Linux）

推荐使用 `webmux service install`，生成位置为 `$XDG_CONFIG_HOME/systemd/user/webmux.service`（默认 `~/.config/systemd/user/webmux.service`）。

通过 deb / rpm 安装时也可以直接启用包内单元：`systemctl --user enable --now webmux`（见 [Linux 安装包](install-and-upgrade.md#linux-安装包deb--rpm)）。

也可手工使用模板：

```sh
make install
mkdir -p ~/.config/systemd/user
cp deploy/webmux.service ~/.config/systemd/user/
systemctl --user daemon-reload
systemctl --user enable --now webmux
journalctl --user -u webmux -f
```

用户未登录时也保持运行：

```sh
loginctl enable-linger "$USER"
```

服务使用 `KillMode=mixed`：WebMux 先自行清理会话，超时后由 systemd 清理整个 cgroup。

## launchd 用户代理（macOS）

推荐使用 `webmux service install`，自动生成 `~/Library/LaunchAgents/com.webmux.plist` 并填写绝对路径，日志位于 `~/Library/Logs/webmux.log`。也可手工使用模板：

```sh
make install
mkdir -p ~/Library/LaunchAgents ~/Library/Logs
sed "s#/Users/YOU#$HOME#g" deploy/com.webmux.plist > ~/Library/LaunchAgents/com.webmux.plist
plutil -lint ~/Library/LaunchAgents/com.webmux.plist
launchctl bootstrap "gui/$(id -u)" ~/Library/LaunchAgents/com.webmux.plist
tail -f ~/Library/Logs/webmux.log
```

修改 plist 后重新加载：

```sh
launchctl bootout "gui/$(id -u)/com.webmux"
launchctl bootstrap "gui/$(id -u)" ~/Library/LaunchAgents/com.webmux.plist
```

- launchd 不展开 `~` 和 `$HOME`，plist 中必须使用绝对路径。
- plist 设置了 `LANG=en_US.UTF-8`；缺少 UTF-8 locale 时中文会显示为转义字符。
- 代理在用户登录后运行，`KeepAlive` 在异常退出后重启。
- 系统睡眠会暂停所有会话进程；长时间运行 agent 时可使用 `caffeinate` 防止睡眠。

## Windows（WSL）

WebMux 运行在 WSL 2 发行版中，由 Windows 托盘程序或 `webmux-windows` 命令行客户端管理。前置条件、安装与托盘设置、启动方式、远程转发、Token 与剪贴板及故障诊断见 [Windows（WSL）部署](windows-wsl.md)。

## agent 的 PATH

systemd 和 launchd 都不会加载交互 shell 的初始化文件。

- 需要从 `~/.bashrc` 或 `~/.zshrc` 获取 PATH 与环境变量时，在自定义启动配置中勾选“加载 shell 启动文件”并选择对应 shell。
- 不使用启动文件包装时，可直接指定程序绝对路径或调整服务的 PATH。服务模板的 `PATH` 已包含 `~/.local/bin`、pnpm / npm 全局目录（macOS 另含 Homebrew）。

如果 agent 安装在 nvm 等其他位置：

```sh
which claude codex opencode
systemctl --user edit webmux    # Linux：添加 drop-in
```

```ini
[Service]
Environment=PATH=/home/YOU/.local/bin:/home/YOU/.nvm/versions/node/v24.20.0/bin:/usr/local/bin:/usr/bin:/bin
```

macOS 直接编辑 plist 的 `EnvironmentVariables` 后重新加载。

Windows 托盘经 `wsl.exe --exec` 启动 WebMux，同样不经过登录 shell；托盘启动的实例会在 `PATH` 前补上服务模板中的 `~/.local/bin`、`~/.local/share/pnpm`、`~/.npm-global/bin`（已存在的保持原位）。安装在其他位置的 agent 请改用 systemd 启动方式并按上面添加 drop-in，或把它链接到 `~/.local/bin`。

## 平台差异

Linux 与 macOS 同等支持。进程状态在 Linux 读取 `/proc`，在 macOS 使用 `sysctl` 与 `proc_pidinfo`。

macOS 上：

- 没有 subreaper，脱离父进程的后代由 launchd 回收。
- 内核不公开其他进程的环境变量，`WEBMUX_SESSION_ID` 无法用于清理；关闭会话时按终端会话 ID 与进程树查找后代。先 `setsid` 再让父进程退出的自守护程序不会被清理。
- 临时目录位于 `/var → /private/var` 符号链接下，WebMux 报告规范化后的真实路径。
- Apple 自带的系统 Bash 版本为 3.2；跨平台配置建议使用 `/usr/bin/env bash`。如需 zsh，请在自定义启动命令中显式填写 `/bin/zsh -l -i`。
