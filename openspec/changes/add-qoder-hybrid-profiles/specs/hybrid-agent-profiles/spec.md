## ADDED Requirements

### Requirement: Qoder 档案

系统 SHALL 内置 Qoder 档案（`Name: "qoder"`，`Title: "Qoder"`），
识别 `qoder` 与 `qodercli` 可执行文件及其版本化名称（`<可执行文件>-<后缀>`，例如 `qodercli-1.1.81`）。

- 终端信号：OSC 7501 根记录 `app` 为 `qoder` 或 `qoder-cli` 时识别为 Qoder，cwd 取根记录的 cwd；
  会话 ID 不来自终端信号，按 `agent-session-identity` 的候选判定取得。
- ACP 启动：`qoder --acp`（标志式，非子命令），原 argv 中的参数 SHALL NOT 透传。
- 恢复：`qoder --resume <uuid>`；中断：一次 Esc（`\x1b`）。
- 聊天菜单隐藏 `login`、`logout`；实测版本 1.1.81。
- 支持 `session/resume`；冷 load/resume 后 SHALL 重应用 `model` 与 `effort`；插话空闲行为为 `promptRequired`。

#### Scenario: Qoder 预置配置

- **WHEN** 配置的 argv 为
  `["bash","-ic","exec -- \"$@\"","webmux","qoder","--dangerously-skip-permissions"]`
- **THEN** 判定为可用，档案为 qoder

#### Scenario: Qoder 版本化可执行文件

- **WHEN** Qoder 混合会话的 PTY 根进程可执行文件为 `qodercli-1.1.81`
- **THEN** 该进程被视为 Qoder 进程

### Requirement: Qoder 恢复命令保留的模型与权限参数

Qoder 的恢复命令 SHALL 保留以下模型、推理、权限与调试参数：`-m`/`--model`、`--reasoning-effort`、
`--thinking`、`--thinking-budget`、`--context-window`、`--fallback-model`、`--max-output-tokens`、
`--max-model-request-retries`、`--permission-mode`，以及开关 `--dangerously-skip-permissions`/`--yolo`、
`-d`/`--debug`。

#### Scenario: Qoder 白名单参数进入恢复命令

- **WHEN** 混合配置的 argv 为 `["qoder","--model","gpt-4"]`，会话切回 TUI 恢复会话 `abc-123`
- **THEN** 恢复命令包含 `--model gpt-4` 与 `--resume abc-123`

### Requirement: Qoder 恢复命令保留的提示词、工具与配置参数

Qoder 的恢复命令 SHALL 保留以下参数：`--system-prompt`、`--append-system-prompt`、`--output-style`、
`--agents`、`--agent`、`--plugin-dir`、`--settings`、`--setting-sources`、`--mcp-config`、
`--allowed-mcp-server-names`、`--allowed-tools`、`--disallowed-tools`、`--tools`、`--add-dir`，
以及开关 `--strict-mcp-config`、`--bare`。

#### Scenario: Qoder 工具参数进入恢复命令

- **WHEN** 混合配置的 argv 为 `["qoder","--add-dir","/srv/lib","--strict-mcp-config"]`，会话切回 TUI 恢复会话 `abc-123`
- **THEN** 恢复命令包含 `--add-dir /srv/lib`、`--strict-mcp-config` 与 `--resume abc-123`

### Requirement: Qoder 丢弃的参数

系统 SHALL 在 Qoder 的恢复与 ACP 命令中丢弃位置参数（提示词）、`-c`/`--continue`、
`-r`/`--resume`（值可选）、`--attachment`、`-i`/`--prompt-interactive` 与 `-n`/`--name`，
这些参数 SHALL NOT 使配置不可用。

#### Scenario: Qoder 恢复参数被丢弃

- **WHEN** 配置的 argv 为 `["qoder","--resume","abc-123"]`
- **THEN** 判定为可用，档案为 qoder，`--resume abc-123` 不进入恢复命令与 ACP 命令

### Requirement: Qoder 打开会话、指定目录与改变会话库的参数

以下 Qoder 参数 SHALL 使配置不可用于混合模式：

- 打开已有会话（`session_arg`）：`--fork-session`、`--session-id`、`--from-pr`、`--remote-session`、
  `--remote-control`、`--remote`，以及首个位置参数为 `fork-session`；
- 指定工作目录（`directory_arg`）：`-w`/`--cwd`、`--worktree`；
- 使聊天视图无法找到会话（`session_store`）：`--config-dir`。

#### Scenario: Qoder 指定 worktree

- **WHEN** 用户对 argv 为 `["qoder","--worktree","feature"]` 的配置勾选混合模式
- **THEN** 保存返回 400，原因提示在新建会话时选择目录

### Requirement: Qoder 不启动交互界面的参数

以下 Qoder 参数 SHALL 使配置不可用于混合模式，原因为“该参数不启动交互界面”：
`-p`/`--print`、`-h`/`--help`、`-v`/`--version`、`--list-models`、`--list-sessions`、
`--no-session-persistence`、`-o`/`--output-format`、`--input-format`、`--delete-session`，
以及首个位置参数为子命令 `mcp`、`hooks`/`hook`、`rollback`、`update`、`config`、
`plugins`/`plugin`、`commit`、`agents`/`agent`、`skills`/`skill`、`feedback`、`security`、
`login`、`status`、`remote-control`、`sites`、`wiki` 的写法。

#### Scenario: Qoder 非交互子命令

- **WHEN** 用户对 argv 为 `["qoder","commit"]` 或 `["qoder","-p","fix bug"]` 的配置勾选混合模式
- **THEN** 保存返回 400，原因为该参数不启动交互界面

### Requirement: Qoder 的聊天端配置推导

首次在聊天端打开 Qoder 混合会话时，系统 SHALL 从 argv 推导配置种子：

- `-m`/`--model` → `model`；`--reasoning-effort` → `effort`；
- `--dangerously-skip-permissions` 或 `--yolo` → `mode=bypass_permissions`，且优先于 `--permission-mode`；
- `--permission-mode` 取值为 `default`、`accept_edits`/`acceptEdits`、`bypass_permissions`/`bypassPermissions`/`yolo`、
  `dont_ask`/`dontAsk`、`auto`、`plan` 之一时 SHALL 归一为 snake_case 写入 `mode`，其他取值 SHALL 列为不生效；
- 其余白名单参数 SHALL 在聊天视图列为“以下参数在聊天端不生效”。

#### Scenario: Qoder 权限模式别名

- **WHEN** 混合配置的 argv 为 `["qoder","--permission-mode","acceptEdits"]`，会话从聊天开始
- **THEN** 聊天配置种子包含 `mode=accept_edits`

#### Scenario: Qoder 未知权限模式

- **WHEN** 混合配置的 argv 为 `["qoder","--permission-mode","invalid"]`，会话从聊天开始
- **THEN** 不生成 `mode` 种子，`unsupported` 包含 `["--permission-mode invalid"]`

### Requirement: QoderCN 档案

系统 SHALL 内置 QoderCN 档案（`Name: "qodercn"`，`Title: "QoderCN"`），
识别 `qodercn`、`qoderclicn` 与 `qoder-cn` 可执行文件及其版本化名称
（例如 `qoderclicn-1.1.81`），SHALL NOT 把 `qoder` 或 `qodercli` 视为 QoderCN 进程。

- OSC 7501 根记录 `app` 为 `qodercn`、`qoder-cli-cn` 或 `qoderclicn` 时识别为 QoderCN。
- 其余行为（ACP 启动参数 `--acp`、恢复、中断、参数分类、聊天端配置推导、隐藏命令、
  实测版本与可选能力）SHALL 与 Qoder 档案相同。

#### Scenario: QoderCN 预置配置

- **WHEN** 配置的 argv 为
  `["bash","-ic","exec -- \"$@\"","webmux","qodercn","--dangerously-skip-permissions"]`
- **THEN** 判定为可用，档案为 qodercn

#### Scenario: QoderCN 由程序 id 识别

- **WHEN** OSC 7501 根记录的 `app` 为 `qoder-cli-cn`
- **THEN** 识别为 qodercn，cwd 取根记录的 cwd

#### Scenario: QoderCN ACP 启动

- **WHEN** 混合配置的 argv 为 `["qodercn","--model","qwen-max"]`，会话从聊天开始
- **THEN** ACP 进程的 argv 为 `["qodercn","--acp"]`

## MODIFIED Requirements

### Requirement: 内置档案与可识别的命令写法

系统 SHALL 内置 OpenCode、omp、Codex、Claude Code、Qoder、QoderCN 与 Antigravity CLI（`agy`）七个档案，
SHALL NOT 开放用户自定义档案。启动配置的 argv 只有在以下写法之一中，且可执行文件的基本名与某个档案匹配时，
才 SHALL 视为可用于混合模式：

1. 直接执行：`[<可执行文件>, 参数…]`；
2. webmux 的启动文件包装：`["bash", "-ic", "exec -- \"$@\"", "webmux", <可执行文件>, 参数…]`
   或 `["zsh", "-ic", "exec -- \"$@\"", "webmux", <可执行文件>, 参数…]`。

其他任何写法（例如其他 shell 包装、`env` 前缀）SHALL 视为不可用，并给出原因“命令写法不受支持”。
未匹配任何档案时，服务端 SHALL 在 `hybridSupport.supportedAgents` 中返回当前注册表支持的
agent 显示名称列表。

#### Scenario: 预置的 bashrc 包装

- **WHEN** 配置的 argv 为 `["bash","-ic","exec -- \"$@\"","webmux","omp"]`
- **THEN** 判定为可用，档案为 omp

#### Scenario: Codex 预置配置

- **WHEN** 配置的 argv 为 `["bash","-ic","exec -- \"$@\"","webmux","codex","--dangerously-bypass-approvals-and-sandbox"]`
- **THEN** 判定为可用，档案为 codex

#### Scenario: agy 的 bashrc 包装

- **WHEN** 配置的 argv 为 `["bash","-ic","exec -- \"$@\"","webmux","agy","--dangerously-skip-permissions"]`
- **THEN** 判定为可用，档案为 `agy`

#### Scenario: QoderCN 的另一可执行文件名

- **WHEN** 配置的 argv 为 `["qoderclicn"]`
- **THEN** 判定为可用，档案为 qodercn

#### Scenario: 自定义包装

- **WHEN** 配置的 argv 为 `["sh","-c","omp"]`
- **THEN** 判定为不可用，原因为命令写法不受支持

### Requirement: ACP 启动命令

系统 SHALL 按档案声明的 ACP 启动方式启动 ACP 进程，并套用与原命令相同的写法：

- 内置子命令：OpenCode 为 `opencode acp`，omp 为 `omp acp`，`agy` 为 `agy acp`；
- 内置标志：档案声明了 ACP 启动参数时以该参数代替默认的 `acp` 子命令，
  Qoder 为 `qoder --acp`，QoderCN 为 `qodercn --acp`；
- 第三方连接器：Codex 为 `codex-acp`，Claude Code 为 `claude-agent-acp`，启动前 SHALL 按
  `acp-connector-availability` 检查连接器已安装。

ACP 进程的参数透传按 agent 区分：

- omp：`omp acp` 解析完整 argv，并静默忽略未知参数。因此原 argv 中位于 omp 白名单内的参数 SHALL 全部透传给
  ACP 进程（例如 `--model`、`--thinking`、`--profile`、`--config`、`--approval-mode`、`--auto-approve`），
  使两端使用相同的模型、配置档与批准策略。
- OpenCode：`opencode acp` 只接受 `--login`，因此 OpenCode 的白名单参数 SHALL NOT 透传。`--auto` 也
  SHALL NOT 在聊天视图提示不支持，因为聊天端的工具批准由聊天视图的权限模式决定（默认自动批准）。
- Codex：`codex-acp` 不接受这些参数，白名单参数 SHALL NOT 透传。模型、推理强度与批准/沙箱参数按
  “Codex 的聊天端配置推导”延续；`-p`/`--profile`、`-c`/`--config`、`--enable`、`--disable`、
  `--search`、`--add-dir`、`--oss`、`--local-provider` 在聊天端不生效，SHALL 在聊天视图列为
  “以下参数在聊天端不生效”。
- Claude Code：`claude-agent-acp` 不接受 CLI 参数，模型、推理强度与权限模式按“Claude Code 的聊天端配置推导”延续，
  其余保留参数在聊天视图列为“以下参数在聊天端不生效”。
- `agy`：`agy acp` 校验其子命令标志，原 argv 中位于 `agy` 白名单且 `agy acp` 接受的参数
  （`--dangerously-skip-permissions`、`--sandbox`、`--agent`、`--add-dir`、`--log-file`）SHALL 透传给
  `agy acp`；仅 TUI 接受的 `--model`、`--effort`、`--mode` SHALL NOT 透传给 `agy acp`。
  `--model` 与 `--effort` SHALL 分别作为 `model` 与 `thought_level` 配置种子延续到聊天端，
  `--mode` SHALL 在聊天视图的 `unsupported` 列表中提示。
- Qoder 与 QoderCN：白名单参数 SHALL NOT 透传给 `--acp` 进程；模型、推理强度与权限模式按
  “Qoder 档案”的聊天端配置推导延续，其余保留参数在聊天视图列为“以下参数在聊天端不生效”。

使用启动文件包装时，`.bashrc`/`.zshrc` 执行期间写到 stdout 的内容 SHALL NOT 进入 ACP 协议通道。

#### Scenario: bashrc 有输出

- **WHEN** 用户的 `.bashrc` 会 `echo` 一行欢迎语，混合会话从聊天开始
- **THEN** ACP 握手成功，欢迎语不导致协议错误

#### Scenario: omp 参数透传

- **WHEN** 混合配置的 argv 为 `["omp","--profile","work","--model","haiku","修 bug"]`，会话从聊天开始
- **THEN** ACP 进程的 argv 为 `["omp","acp","--profile","work","--model","haiku"]`

#### Scenario: Codex 使用连接器

- **WHEN** 混合配置的 argv 为 `["bash","-ic","exec -- \"$@\"","webmux","codex","--search","-m","gpt-5.6-luna"]`，
  会话从聊天开始
- **THEN** ACP 进程以 bashrc 包装执行 `codex-acp`，不带任何参数；聊天视图列出 `--search` 在聊天端不生效

#### Scenario: agy 参数按子集透传

- **WHEN** 混合配置的 argv 为 `["agy","--dangerously-skip-permissions","--sandbox","--model","gemini-3.8-flash","--mode","plan"]`，
  会话从聊天开始
- **THEN** ACP 进程的 argv 为 `["agy","acp","--dangerously-skip-permissions","--sandbox"]`，
  聊天配置种子包含 `model=gemini-3.8-flash`，`unsupported` 包含 `["--mode"]`

#### Scenario: Qoder 标志式 ACP

- **WHEN** 混合配置的 argv 为 `["bash","-ic","exec -- \"$@\"","webmux","qoder","--dangerously-skip-permissions"]`，
  会话从聊天开始
- **THEN** ACP 进程以 bashrc 包装执行 `qoder --acp`（协议走 fd 3），不带其他参数；
  聊天配置种子包含 `mode=bypass_permissions`
