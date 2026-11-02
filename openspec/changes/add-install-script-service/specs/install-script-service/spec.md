# Spec Delta

## Purpose

让 Linux 与 macOS 用户在运行 `install.sh` 时选择是否同时把 WebMux 安装为登录后自动启动的用户服务，并在已有服务、无法交互或服务安装失败时给出不破坏现有会话的明确结果。

## ADDED Requirements

### Requirement: 选择是否安装用户服务

`install.sh` SHALL 按环境变量 `WEBMUX_SERVICE` 决定二进制安装成功后是否安装用户服务：`1` 安装，`0` 不安装。其他取值 SHALL 在下载前报错退出，且不改动任何文件。

未设置时，脚本能打开终端 SHALL 询问“是否安装为登录后自动启动的用户服务”，直接回车视为是；无法打开终端时 SHALL 不安装服务，并像以前一样提示 `webmux service install` 命令。

#### Scenario: 显式安装服务

- **WHEN** 用户执行 `curl … | WEBMUX_SERVICE=1 sh`，且没有已安装的用户服务
- **THEN** 二进制安装完成后执行 `webmux service install`，服务启动并在下次登录时自动启动

#### Scenario: 交互确认

- **WHEN** 用户在终端中执行 `curl … | sh`，在询问处直接回车
- **THEN** 脚本安装用户服务

#### Scenario: 无法交互

- **WHEN** 在没有终端的 CI 中执行 `curl … | sh`，未设置 `WEBMUX_SERVICE`
- **THEN** 只安装二进制，输出中包含 `service install` 提示，不调用服务管理命令

#### Scenario: 无效取值

- **WHEN** 用户设置 `WEBMUX_SERVICE=yes`
- **THEN** 脚本在下载前以非零退出码结束并说明可用取值，安装目录不变

### Requirement: 安装用户服务

选择安装时，脚本 SHALL 以刚安装的可执行文件运行 `service install`，沿用其行为：缺少默认配置时生成带随机 Token 的配置，写入 systemd 用户服务（Linux）或 launchd 用户代理（macOS）并立即启动。

成功后 SHALL 提示查看状态与日志的命令；在 Linux 上 SHALL 提示用 `loginctl enable-linger` 让服务在未登录时也运行。

#### Scenario: Linux 安装成功

- **WHEN** Linux 用户选择安装服务，`systemctl --user` 可用
- **THEN** 输出说明服务已安装并给出 `webmux service status`、`webmux service logs` 与 `loginctl enable-linger` 提示，脚本以退出码 0 结束

### Requirement: 已有用户服务不被改动

已存在 WebMux 用户服务定义时，脚本 SHALL NOT 改写、重启或停止该服务，因为重启会结束所有终端会话。

服务运行的是本次安装的可执行文件时，脚本 SHALL 提示新版本在执行 `webmux service restart` 后生效；运行的是其他可执行文件时，SHALL 改为提示用 `webmux service install --force` 切换到本次安装的路径（同样会重启服务），不给出不会加载新版本的 restart 提示。

#### Scenario: 升级时已有服务

- **WHEN** 用户已用 `webmux service install` 安装服务且服务正在运行，再次以 `WEBMUX_SERVICE=1` 运行脚本
- **THEN** 二进制被替换，服务定义与运行中的服务不变，输出提示用 `webmux service restart` 使新版本生效

#### Scenario: 服务指向其他路径

- **WHEN** 已有服务定义运行 `/usr/local/bin/webmux`，脚本把二进制安装到 `~/.local/bin/webmux`
- **THEN** 服务不变，输出提示可用 `~/.local/bin/webmux service install --force` 切换，不提示 `service restart`

### Requirement: 服务安装失败

`service install` 失败时，脚本 SHALL 保留已安装的二进制，输出失败原因及手动重试的命令，并以非零退出码结束。

#### Scenario: 没有可用的 systemd 用户管理器

- **WHEN** Linux 上 `systemctl --user` 不可用，用户选择安装服务
- **THEN** 二进制已安装且可执行，输出包含 `service install` 的错误与重试命令，脚本退出码非 0
