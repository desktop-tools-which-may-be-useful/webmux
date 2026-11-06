# session-launch-environment Specification

## Purpose

约定会话启动时的进程环境：调用方只能显式追加或覆盖经过校验的环境变量，或引用服务端保存的启动配置，服务端不自行复制调用方的环境，变量值也不出现在会话信息中。Linux/macOS 的 `webmux run` 自定义命令默认把调用方环境作为显式变量发送。

## Requirements

### Requirement: 显式启动环境

会话创建请求 SHALL 接受可选的 `env` 对象（变量名到值）。服务端 SHALL 以服务进程环境为基础创建会话，并只追加或覆盖 `env` 中显式列出的变量（包括覆盖默认设置的 `TERM` 等）；SHALL NOT 复制调用方的其他环境。`env` SHALL NOT 出现在会话信息中。

变量名 SHALL 匹配 `[A-Za-z_][A-Za-z0-9_]*`，值 SHALL NOT 含 NUL；`WEBMUX_SESSION_ID` SHALL 被拒绝。不合法的 `env` SHALL 以 400 拒绝且不创建会话。

`webmux run --env NAME=VALUE`（可重复）SHALL 映射为该字段；格式不是 `NAME=VALUE` 时 SHALL 以退出码 2 结束；服务端不支持该字段时客户端 SHALL 报告服务端错误，SHALL NOT 静默忽略。

Linux/macOS 上的 `webmux run` 自定义命令 SHALL 默认把调用方进程环境放入 `env`，`--env` 的值 SHALL 覆盖同名继承值。

继承时 SHALL 跳过以下变量，使会话仍以 WebMux 终端身份与请求的目录启动：

- 服务端自有的变量：`TERM`、`COLORTERM`、`TERM_PROGRAM`、`TERM_PROGRAM_VERSION`、`WEBMUX_SESSION_ID`、`WEBMUX_LAUNCHER`，以及 tmux、screen、Kitty、Warp、iTerm、WezTerm、Ghostty、Windows Terminal、VTE、Zellij 等宿主终端身份变量。
- 描述调用方进程而非会话的 `PWD`、`OLDPWD` 与 `_`。
- 名称不合法或值含 NUL 的变量。

`--launch-config` SHALL NOT 继承调用方环境；Windows 客户端 SHALL NOT 继承 Windows 环境。

`webmux run --launch-config <名称>`（`--config-id` 为兼容别名）SHALL 按区分大小写的精确名称从 `GET /api/launch-configs` 摘要中解析出配置 ID，再映射为 `configId` 字段，并 SHALL 允许同时指定 `--cwd`、`--name` 与 `--detach`。

被拒绝的情形：

- 名称不存在：SHALL 以退出码 2 拒绝并列出已保存的名称。
- 空名称：SHALL 在本地以退出码 2 拒绝。
- 与 `--` 后命令或 `--env` 同时使用：二者互斥，CLI SHALL NOT 同时发送 `cmd` 或 `env`。

#### Scenario: 虚拟环境

- **WHEN** 用户执行 `webmux run --env VIRTUAL_ENV=/w/.venv --env PATH=/w/.venv/bin:/usr/bin -- python app.py`
- **THEN** 会话中 `PATH` 与 `VIRTUAL_ENV` 为给定值，覆盖继承自调用方的同名变量

#### Scenario: 继承调用方环境

- **WHEN** 用户在 tmux 中、`/home/u/proj/sub` 目录、已导出 `ANTHROPIC_API_KEY` 时执行 `webmux run --cwd /home/u/proj -- claude`
- **THEN** 会话中 `ANTHROPIC_API_KEY` 与调用方相同，`TMUX` 不存在，`TERM` 为 `xterm-256color`，`TERM_PROGRAM` 为 `webmux`，`PWD` 为 `/home/u/proj`

#### Scenario: 保留变量

- **WHEN** 请求的 `env` 包含 `WEBMUX_SESSION_ID`
- **THEN** 服务端返回 400，不创建会话

### Requirement: 通过启动配置创建会话

`POST /api/sessions` SHALL 接受可选的 `configId` 字符串字段。提供该字段时，其他字段 SHALL 满足互斥约束，违反时 SHALL 以 400 拒绝且不创建会话：

- `cmd` 为缺省或空数组。
- `env` 为缺省、`null` 或空对象 `{}`。

空字符串 `configId` SHALL 以 400 拒绝，SHALL NOT 静默回退到自定义命令。

服务端 SHALL 按 `configId` 读取已保存的完整 argv 与环境变量，并以二者启动会话：

- 未知 ID SHALL 返回 404；启动配置存储不可用 SHALL 返回 503；二者都不创建会话。
- 互斥、读取配置与环境变量校验 SHALL 在任何目录或 Git 操作之前完成。
- 保存的 `env` SHALL 在启动前再次经过 `session.ValidateEnv`；不合法时 SHALL 返回 400 且不创建会话。
- `cwd`、`workspace` 与 `name` 的语义 SHALL 保持不变，包括目录不存在、工作树失效与目标变化对应的既有错误。

通过启动配置创建的会话 SHALL 在会话信息中记录完整 `cmd` 与 `configId`；`env` SHALL NOT 出现在会话信息中。启动后更新或删除该配置 SHALL NOT 改变运行中的会话。

显式 `cmd`/`env` 创建方式 SHALL 保持可用；创建请求 SHALL NOT 接受 `preset` 与 `args`，系统不提供内置预设。

#### Scenario: 使用配置的命令与环境变量

- **WHEN** 用户以 `{"configId": "k3m9q2x7", "cwd": "/home/u/proj"}` 创建会话，配置的 `cmd` 为 `["claude", "--resume"]` 且包含 `ANTHROPIC_BASE_URL`
- **THEN** 新会话执行该 argv，程序环境包含该变量，会话信息记录完整 `cmd` 与 `configId`

#### Scenario: 字段互斥

- **WHEN** 请求同时包含 `configId` 与非空的 `cmd` 或 `env` 中任意一项
- **THEN** 服务端返回 400，且不创建会话、不进行目录或 Git 操作

#### Scenario: 空对象环境变量不算冲突

- **WHEN** 请求包含有效的 `configId` 与 `"env": {}`
- **THEN** 服务端按该配置保存的环境变量启动会话

#### Scenario: 未知配置

- **WHEN** 请求引用不存在的 `configId`
- **THEN** 服务端返回 404，且不创建会话

#### Scenario: 删除配置后会话继续

- **WHEN** 用户通过某配置启动会话，随后删除该配置
- **THEN** 已运行会话继续使用启动时的 argv 与 env，终端连接与退出不受影响

### Requirement: WebMux 自有的会话变量

服务端 SHALL 在每个会话的环境中设置以下变量，使会话内的程序不依赖 `PATH` 或默认配置就能找到并连接本实例：

- `WEBMUX_SOCKET`：本实例本机 API 套接字的绝对路径。本机 API 不可用时 SHALL NOT 设置。
- `WEBMUX_BIN`：可以执行 `webmux` 子命令的可执行文件的绝对路径。按下文规则解析不到时 SHALL NOT 设置。

注入要求：

- 注入 SHALL 发生在构造会话进程环境的唯一入口：PTY 主进程，以及混合会话聊天端的 ACP agent 等辅助进程，都 SHALL 得到相同的值。
- 两个变量 SHALL 属于服务端自有变量：
  - 创建请求的 `env` 包含它们时，SHALL 以 400 拒绝；
  - 保存包含它们的启动配置时，SHALL 以 400 拒绝；
  - `webmux run` 继承调用方环境时，SHALL 跳过它们。
- 它们 SHALL NOT 出现在会话信息中。
- 服务端 SHALL NOT 为此修改会话的 `PATH`。

`WEBMUX_BIN` SHALL 是服务进程自身的可执行文件路径，即服务启动时 `os.Executable()` 的结果，原样保存在内存中，不另做配置或改写。
`os.Executable()` 返回错误时 SHALL NOT 设置该变量，并记录一条警告日志。

- 会话里执行的就是正在运行的服务端那一个文件，CLI 与服务端因此版本一致。
- 软件包原地升级后，同一路径指向新版本的文件，变量 SHALL 继续有效。
- 以 `go run` 启动时，该路径是 Go 构建出的临时可执行文件；它在服务运行期间一直存在，变量同样有效。

#### Scenario: PATH 中没有 webmux

- **WHEN** 服务以 `/opt/webmux/bin/webmux serve` 启动，会话的 `PATH` 不含 `/opt/webmux/bin`
- **THEN** 会话中 `"$WEBMUX_BIN" ls` 能列出本实例的会话，`PATH` 保持原样

#### Scenario: 聊天端 agent 同样可见

- **WHEN** 混合会话处于聊天端，ACP agent 执行 `echo "$WEBMUX_BIN" "$WEBMUX_SOCKET"`
- **THEN** 输出与同一会话终端端相同的两个绝对路径

#### Scenario: 开发构建

- **WHEN** 服务由 `go run ./cmd/webmux` 启动
- **THEN** 会话中的 `WEBMUX_BIN` 是 Go 构建出的临时可执行文件，`"$WEBMUX_BIN" ls` 在服务运行期间可用

#### Scenario: 请求覆盖被拒绝

- **WHEN** 创建请求的 `env` 包含 `WEBMUX_SOCKET=/tmp/other.sock` 或 `WEBMUX_BIN=/tmp/fake`
- **THEN** 服务端返回 400，不创建会话

#### Scenario: 跨实例继承被跳过

- **WHEN** 在实例 A 的会话里，用户用 `webmux run --config b.toml -- bash` 在实例 B 创建会话
- **THEN** 新会话的 `WEBMUX_SOCKET` 与 `WEBMUX_BIN` 都是实例 B 的值
