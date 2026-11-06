# Spec Delta

## MODIFIED Requirements

### Requirement: 从终端创建会话

`webmux run [--launch-config <名称>] [--name <名称>] [--cwd <目录>] [--env NAME=VALUE]... [--detach] -- <argv...>` SHALL 在本机 WebMux 中创建会话。
位置、起始端、首条消息、标签、`--json`、`--wait` 与 `--dry-run` 等选项见 `shell-session-launch`。
未指定 `--launch-config` 时，`--` 之后的参数 SHALL 原样作为自定义命令 argv，SHALL NOT 经过 shell 展开。

指定 `--launch-config`（`--config-id` 为兼容别名）时，CLI SHALL 读取 `GET /api/launch-configs` 摘要（只含环境变量名，不含值），
按区分大小写的精确名称匹配唯一的 Web 端启动配置，并以其 ID 作为 `configId` 发送；服务端 SHALL 按该 ID 解析完整 argv 与环境变量，CLI SHALL NOT 读取或复制环境变量值。

CLI SHALL NOT 把参数当作配置 ID 接受。名称不存在时 SHALL 不创建会话，在标准错误列出已保存的名称（没有任何配置时说明没有已保存配置）并以退出码 2 结束。
两种模式 SHALL 互斥：启动配置名称不得与 `--` 后命令或 `--env` 同时提供，空名称 SHALL 以退出码 2 拒绝。

未指定 `--cwd` 时 SHALL 使用调用者的当前目录；`--cwd` 为相对路径时 SHALL 相对当前目录解析为绝对路径，以 `~` 开头时 SHALL 交由服务端展开。自定义命令模式缺少命令时 SHALL 输出用法并以退出码 2 结束。

`--detach` 时 SHALL 只向标准输出写入一行 `<会话 ID>\t<本机会话链接>` 并以退出码 0 结束。
未指定 `--detach` 时，标准输入与标准输出都是终端则 SHALL 立即附着该会话：附着前在标准错误输出会话 ID、本机会话链接与分离键提示，SHALL NOT 向标准输出写入额外内容。
任一不是终端（例如在 agent 的工具调用或管道中）时，SHALL 按 `--detach` 处理，并在标准错误说明已自动分离，SHALL NOT 以用法错误结束。`--attach` SHALL 强制附着，此时没有终端仍以 2 结束。

本机会话链接 SHALL 为服务报告的本机链接加 `/s/<会话 ID>`，SHALL NOT 包含 Token。

#### Scenario: 参数不被 shell 展开

- **WHEN** 用户执行 `webmux run --detach -- printf '%s;$(id)' 'a b'`
- **THEN** 会话 argv 为 `["printf", "%s;$(id)", "a b"]`，`$(id)` 不被执行，标准输出为会话 ID 与链接

#### Scenario: 附着前告知链接

- **WHEN** 用户在 `/home/u/proj` 执行 `webmux run -- claude`
- **THEN** 会话以 cwd `/home/u/proj` 创建，标准错误先出现会话 ID 与 `http://127.0.0.1:7681/s/<ID>`，随后终端附着到该会话

#### Scenario: agent 调用时自动分离

- **WHEN** agent 在没有终端的工具调用中执行 `webmux run -- make test`
- **THEN** 会话被创建，标准输出为会话 ID 与链接，标准错误说明已自动分离，退出码 0

#### Scenario: 使用 Web 端保存的启动配置

- **WHEN** Web 端已保存名称为 `Claude Code`、ID 为 `k3m9q2x7` 的启动配置，用户执行 `webmux run --launch-config 'Claude Code' --cwd /home/u/proj --detach`
- **THEN** 客户端先读取启动配置摘要，再向 `POST /api/sessions` 发送 `configId: "k3m9q2x7"` 与 cwd，不发送 `cmd` 或 `env`；服务端按该 ID 保存的完整 argv 与环境变量创建会话

#### Scenario: 启动配置名称不存在

- **WHEN** 已保存的配置名称为 `Shell` 与 `omp`，用户执行 `webmux run --launch-config nope --detach`，或以配置 ID `k3m9q2x7` 作为名称
- **THEN** 客户端不创建会话，标准错误列出 `"Shell", "omp"`，退出码为 2

#### Scenario: 启动配置互斥

- **WHEN** 用户执行 `webmux run --launch-config omp --env API_URL=https://example.test -- true`
- **THEN** 客户端输出用法并以退出码 2 结束，且在创建任何会话前拒绝参数

### Requirement: 连接与错误约定

`run`、`attach`、`ls`、`open`，以及 `shell-session-control`、`shell-chat-control`、`shell-workspace-control`、`agent-shell-guide` 中的命令，
SHALL 经本机套接字访问本机服务。它们 SHALL NOT 需要或输出 Token，SHALL 接受 `--config <path>` 与 `--socket <path>`，并按以下顺序定位服务：

1. 显式给出的 `--socket`，直接连接该套接字；
2. 显式给出的 `--config`，按与服务相同的规则定位；
3. 环境变量 `WEBMUX_SOCKET` 指向的套接字；
4. 默认配置路径，按与服务相同的规则定位。

`--socket` 与 `--config` SHALL 互斥。`--socket` 或 `WEBMUX_SOCKET` 指定的套接字不可连接时，SHALL 视为服务未运行，SHALL NOT 回退到默认配置。
`--socket` 用于 agent 的工具 shell 过滤掉 `WEBMUX_*` 变量的情形：`{{webmux}}` 简述会给出套接字的绝对路径。

CLI 调用服务端没有的接口（404，且路径属于本变更新增的接口）时，SHALL 报告“服务端版本过旧”，
同时给出服务端与 CLI 的版本，并提示改用 `$WEBMUX_BIN`（与服务端同一个可执行文件）；SHALL NOT 只输出原始的 404。

接受会话 ID 的位置参数和选项 SHALL 接受 `self`，在客户端解析为环境变量 `WEBMUX_SESSION_ID` 的值。该变量未设置时 SHALL 以退出码 2 结束。

服务未运行时 SHALL 以退出码 3 结束并提示启动方式，SHALL NOT 隐式启动服务。
以下命令不受此限：

- `agent-shell-guide` 中读取手册的命令（不带 `--path`）输出 CLI 内嵌的手册并以 0 退出；
- `help`、`--help` 与 `completion` 不连接服务。

诊断信息 SHALL 只写标准错误。

`webmux help` SHALL 按组列出命令：服务与运维、会话、聊天、工作区、工具说明。每个新命令 SHALL 提供 `--help`。

#### Scenario: 服务未运行

- **WHEN** 没有 WebMux 运行，用户执行 `webmux run -- claude`
- **THEN** 不创建任何进程，标准错误提示服务未运行与启动方式，退出码 3

#### Scenario: 会话内连回所属实例

- **WHEN** 服务以 `--config /srv/webmux.toml` 启动，会话内的 agent 不带 `--config` 执行 `webmux ls`
- **THEN** CLI 经 `WEBMUX_SOCKET` 连接这个实例，而不是默认配置对应的实例

#### Scenario: 套接字失效不回退

- **WHEN** `WEBMUX_SOCKET` 指向的服务已经退出，而默认配置对应另一个正在运行的实例
- **THEN** CLI 以退出码 3 结束，不访问另一个实例

#### Scenario: 工具 shell 丢失环境变量

- **WHEN** agent 的工具 shell 没有 `WEBMUX_SOCKET`，agent 按简述执行 `/usr/bin/webmux ls --socket /run/user/1000/webmux/webmux.sock`
- **THEN** CLI 连接该套接字列出会话，不读取默认配置

#### Scenario: CLI 比服务端新

- **WHEN** PATH 中的 `webmux` 比运行中的服务新，用户执行一个服务端尚未支持的命令
- **THEN** 退出码 1，标准错误给出两边的版本，并提示使用 `$WEBMUX_BIN`

#### Scenario: self 在会话外

- **WHEN** 用户在普通终端执行 `webmux get self`
- **THEN** 退出码 2，标准错误说明 `self` 只能在 WebMux 会话内使用
