## MODIFIED Requirements

### Requirement: 内置档案与可识别的命令写法

系统 SHALL 内置 OpenCode、omp、Codex 与 Claude Code 四个档案，SHALL NOT 开放用户自定义档案。Claude Code 档案的稳定标识为 `claudecode`，可执行文件名为 `claude`，终端信号为 `app=claude-code`。启动配置的 argv 只有在以下两种写法之一中，且可执行文件的基本名与某个档案匹配时，才 SHALL 视为可用于混合模式：

1. 直接执行：`[<可执行文件>, 参数…]`；
2. webmux 的 bashrc 包装：`["bash", "-ic", "exec -- \"$@\"", "webmux", <可执行文件>, 参数…]`。

其他任何写法（例如其他 shell 包装、`env` 前缀）SHALL 视为不可用，并给出原因“命令写法不受支持”。未匹配任何档案时，原因文字 SHALL 列出上述四个受支持 agent。

#### Scenario: Claude Code 预置配置

- **WHEN** 配置的 argv 为 `["bash","-ic","exec -- \"$@\"","webmux","claude","--dangerously-skip-permissions"]`
- **THEN** 判定为可用，档案为 `claudecode`，并声明 `claude-agent-acp` 连接器

#### Scenario: 自定义包装

- **WHEN** 配置的 argv 为 `["sh","-c","claude"]`
- **THEN** 判定为不可用，原因为命令写法不受支持

### Requirement: 使两端看到不同会话库或目录的参数

以下参数 SHALL 使配置不可用于混合模式：

- 让 TUI 与 ACP 看到不同会话库或不保存会话的参数，原因为“该参数使聊天视图无法找到会话”：omp 的 `--session-dir`/`--no-session`、OpenCode 的 `--server`、Codex 的 `--remote`/`--remote-auth-token-env`，以及 Claude Code 的 `--no-session-persistence`、`--environment`。
- 在命令中指定或创建工作目录的参数，原因为“请在新建会话时选择目录，不要在命令中写目录”：OpenCode 的位置参数、omp 的 `--cwd`、Codex 的 `-C`/`--cd`/`--worktree`，以及 Claude Code 的 `-w`/`--worktree`/`--tmux`。
- 打开、复制或指定已有会话的参数或子命令，原因为“该参数指定了 agent 会话，混合配置请不要指定会话”（原因码 `session_arg`）：Codex 的 `resume`/`fork`，以及 Claude Code 的 `-c`/`--continue`、`-r`/`--resume`、`--fork-session`、`--session-id`、`--from-pr`、`--teleport`、`--cloud` 与 `attach`。

omp 的 `--profile` SHALL NOT 视为不可用：它同时透传给 ACP 进程，两端使用同一个配置档的会话库。Claude Code 的 `CLAUDE_CONFIG_DIR` SHALL 通过启动配置环境传递，SHALL NOT 因设置了该环境变量而拒绝混合模式。

#### Scenario: Claude Code 命令中写了恢复参数

- **WHEN** 用户对 argv 为 `["claude","--resume","01a18715"]` 的配置勾选混合模式
- **THEN** 保存返回 400，原因码为 `session_arg`

#### Scenario: Claude Code 命令中写了工作树

- **WHEN** 用户对 argv 为 `["claude","--worktree","feature"]` 的配置勾选混合模式
- **THEN** 保存返回 400，原因提示在新建会话时选择目录

### Requirement: 不启动交互界面的参数

以下参数 SHALL 使配置不可用于混合模式，原因为“该参数不启动交互界面”：

- omp 的 `-p`/`--print`、`--mode`、`--export`、`--alias`、`-h`/`--help`、`-v`/`--version`，以及首个位置参数为 omp 子命令的写法；
- OpenCode 的 `-h`/`--help`、`-v`/`--version`、`--completions`、`--wizard`；
- Codex 的 `-h`/`--help`、`-V`/`--version`，以及档案声明的非交互子命令；
- Claude Code 的 `-p`/`--print`、`--output-format`、`--input-format`、`--json-schema`、`--max-budget-usd`、`--bg`/`--background`、`--desktop`、`-h`/`--help`、`-v`/`--version`；
- Claude Code 的 `auth`、`agents`、`gateway`、`import`、`install`、`logs`、`mcp`、`doctor`、`setup-token` 子命令；
- Claude Code 的 `stop`/`kill`、`rm`、`ultrareview`、`update`/`upgrade`、`completion`、`help` 子命令。

#### Scenario: Claude Code 非交互输出

- **WHEN** 用户对 argv 为 `["claude","-p","总结改动"]` 的配置勾选混合模式
- **THEN** 保存返回 400，原因为该参数不启动交互界面

#### Scenario: Claude Code 后台命令

- **WHEN** 用户对 argv 为 `["claude","agents","list"]` 的配置勾选混合模式
- **THEN** 保存返回 400，原因为该参数不启动交互界面

### Requirement: 恢复命令的生成

切换或恢复到 TUI 时，系统 SHALL 按以下方式生成恢复命令：

- 由可执行文件、档案要求注入的 TUI 参数、原 argv 中位于档案白名单内的参数、档案的恢复参数与 agent 会话 ID 组成，并套用与原命令相同的写法。恢复参数：OpenCode 为 `-s <id>`，omp 为 `--resume <id>`，Codex 为子命令 `resume <id>`（全局参数位于子命令之前），Claude Code 为全局参数 `--resume <id>`。
- 白名单参数 SHALL 同时识别 `--flag value` 与 `--flag=value` 两种形式，并按档案声明的参数个数解析：
  - 可变参数个数选项包括 `--add-dir`、`--mcp-config`、`--allowedTools`/`--allowed-tools`、`--disallowedTools`/`--disallowed-tools`、`--tools` 与 `--betas`。
  - 这些选项 SHALL 消费至下一选项，SHALL NOT 只保留第一个值或把其余值误判为位置提示词。
- Claude Code 的白名单 SHALL 包括以下参数：
  - 模型与权限：`--model`、`--effort`、`--fallback-model`、`--permission-mode`、`--dangerously-skip-permissions`；
  - 目录与配置：`--add-dir`、`--settings`、`--setting-sources`；
  - 系统提示与 MCP：`--system-prompt`/`--system-prompt-file`、`--append-system-prompt`/`--append-system-prompt-file`、`--mcp-config`、`--strict-mcp-config`；
  - agent、插件与工具：`--agents`、`--agent`、`--plugin-dir`、`--allowedTools`/`--allowed-tools`、`--disallowedTools`/`--disallowed-tools`、`--tools`、`--betas`；
  - 精简交互界面：`--bare`。该选项只减少 hooks、插件与其他功能，仍启动 TUI，SHALL NOT 归类为非交互参数。
- 系统 SHALL 丢弃位置提示词、附图参数和白名单之外的参数；被丢弃的参数 SHALL 在切换结果中列出，界面 SHALL 提示“以下参数在恢复时未保留”。

#### Scenario: Claude Code 恢复命令

- **WHEN** 原 argv 为 `["claude","--model","opus","--effort","high","修 bug"]`，agent 会话 ID 为 `01a18715-8504-7020-9f3d-257900000000`
- **THEN** 恢复命令为 `["claude","--model","opus","--effort","high","--resume","01a18715-8504-7020-9f3d-257900000000"]`，界面列出未保留的 `修 bug`

#### Scenario: 丢弃 Claude Code 提示词

- **WHEN** 原 argv 含位置提示词与不在白名单中的参数
- **THEN** 恢复命令不包含这些参数，且切换结果逐项列出

### Requirement: ACP 启动命令

系统 SHALL 按档案声明的 ACP 启动方式启动 ACP 进程，并套用与原命令相同的写法：

- 内置子命令：OpenCode 为 `opencode acp`，omp 为 `omp acp`；
- 第三方连接器：Codex 为 `codex-acp`，Claude Code 为 `claude-agent-acp`，启动前 SHALL 按 `acp-connector-availability` 检查连接器已安装。

Claude Code 连接器 SHALL 无参数启动。WebMux 的 `initialize` 请求 SHALL 使用 ACP `protocolVersion: 1`，SHALL NOT 选择仍属实验阶段的 ACP v2。

原 argv 中的 Claude Code 参数 SHALL NOT 透传给连接器。可映射的参数按“Claude Code 的聊天端配置推导”延续，其余保留参数 SHALL 在聊天视图列为“以下参数在聊天端不生效”。

使用 bashrc 包装时，`.bashrc` 执行期间写到 stdout 的内容 SHALL NOT 进入 ACP 协议通道。

#### Scenario: Claude Code 使用连接器

- **WHEN** 混合配置的 argv 为 `["bash","-ic","exec -- \"$@\"","webmux","claude","--model","opus"]`，会话从聊天开始
- **THEN** ACP 进程以 bashrc 包装无参数执行 `claude-agent-acp`，握手协议为 v1，`model` 通过配置项设为 `opus`

#### Scenario: bashrc 有输出

- **WHEN** 用户的 `.bashrc` 会 `echo` 一行欢迎语，Claude Code 混合会话从聊天开始
- **THEN** ACP 握手成功，欢迎语不导致协议错误

### Requirement: 版本提示

agent 的版本来自终端信号、ACP `initialize` 返回的 `agentInfo.version`，或档案声明的版本命令。版本低于档案记录的实测版本时，系统 SHALL 在会话页显示提示，SHALL NOT 因此阻止切换；能否切换只由资格判定决定。版本无法解析时 SHALL 不提示，高于实测版本 SHALL 视为尚未重新验证而非自动不兼容。

- OpenCode、omp 与 Codex 的实测版本沿用各自档案。
- Claude Code 的本次实测基线为 2.1.316，连接器为 0.99.1；版本命令作用于 `CLAUDE_CODE_EXECUTABLE` 所指可执行文件，连接器版本来自 `agentInfo.version`。
- 安装命令 SHALL 为 `npm install -g @agentclientprotocol/claude-agent-acp`，以跟随 `latest`；该安装策略 SHALL NOT 被解释为对所有未来版本的兼容保证。

#### Scenario: 旧版 Claude 连接器

- **WHEN** Claude Code 混合会话的 `initialize` 返回 `agentInfo.version` 为 0.98.0
- **THEN** 聊天视图提示连接器低于已验证版本 0.99.1，其余资格判定照常

## ADDED Requirements

### Requirement: Claude Code 的 ACP 环境与认证

Claude Code 的 ACP 进程（含资格探测）SHALL 复用会话环境，并在配置未设置 `CLAUDE_CODE_EXECUTABLE` 时注入 TUI 实际使用的 `claude` 绝对路径，使两端运行同一个 Claude Code。配置已设置同名变量时 SHALL 使用配置值。

ACP 进程 SHALL 使用与 TUI 相同的服务用户、`HOME` 与 `CLAUDE_CONFIG_DIR`，复用其中已有的订阅登录、API key、OAuth token 或第三方 provider。系统 SHALL NOT 为聊天端要求一套独立凭据，SHALL NOT 在记录、负载或日志中保存凭据值，也 SHALL NOT 自动执行登录或注销。

#### Scenario: 两端使用同一个 Claude Code

- **WHEN** 会话 PATH 中 `claude` 解析为 `/home/u/.local/bin/claude`，配置未设置 `CLAUDE_CODE_EXECUTABLE`
- **THEN** `claude-agent-acp` 的环境中 `CLAUDE_CODE_EXECUTABLE=/home/u/.local/bin/claude`

#### Scenario: 复用现有订阅登录

- **WHEN** 服务用户已在同一配置目录登录 Claude 订阅，未设置额外 ACP 凭据
- **THEN** 聊天端以该身份创建和恢复会话，任何凭据内容都不进入 WebMux 记录或日志

### Requirement: Claude Code 的聊天端配置推导

Claude Code 档案 SHALL 从保留的 argv 推导首次聊天端配置：

| argv | 配置项取值 |
|---|---|
| `--model <m>` | `model = <m>` |
| `--effort <e>` | `effort = <e>` |
| `--permission-mode default`/`manual` | `mode = default` |
| `--permission-mode acceptEdits`/`plan`/`auto`/`bypassPermissions` | `mode` 为同名值 |
| `--dangerously-skip-permissions` | `mode = bypassPermissions`，优先于其他权限参数 |

Claude Code 2.1.316 的 `--permission-mode` 接受 `acceptEdits`、`auto`、`bypassPermissions`、`default`（`manual` 为别名）、`dontAsk` 与 `plan`。

`--permission-mode dontAsk` 与未知取值 SHALL NOT 静默映射为其他模式，SHALL 在聊天端列为不生效参数。

每次冷 `session/load` 或 `session/resume` 后，系统 SHALL 以适配器发布的实际配置为准。本次实测基线下，`model` 与 `effort` 均回到环境或设置默认值，档案 SHALL 在两种恢复方法后重新应用记录值；`mode` 会从 transcript 恢复，SHALL NOT 用过期 argv 覆盖适配器实际模式。

适配器因运行条件不支持而把 `auto` 或 `bypassPermissions` 回退时，界面 SHALL 显示实际生效模式与适配器通知，SHALL NOT 继续显示请求值已生效。

#### Scenario: 预置配置的权限模式

- **WHEN** 预置 Claude Code 混合配置从终端开始并切到聊天
- **THEN** 首次聊天端请求 `mode=bypassPermissions`；若适配器允许该模式，界面显示其已生效

#### Scenario: Auto 被适配器回退

- **WHEN** argv 请求 `--permission-mode auto`，当前模型不支持 Auto
- **THEN** 界面显示适配器实际回退的 `acceptEdits` 与相应通知，而不是仍显示 `auto`

#### Scenario: 当前 provider 接受 Auto 与 Bypass

- **WHEN** 本次实测的第三方 provider 会话依次请求 `mode=auto` 与 `mode=bypassPermissions`
- **THEN** 适配器分别发布实际值 `auto` 与 `bypassPermissions`，不产生 Auto 回退通知

#### Scenario: 冷恢复后重应用模型与 Effort

- **WHEN** 原会话请求 `model=sonnet`、`effort=high`、`mode=plan`，连接器冷启动后执行 `session/load` 或 `session/resume`
- **THEN** 适配器先发布其恢复值，WebMux 重应用 `model=sonnet` 与 `effort=high`，并保留适配器恢复的 `mode=plan`

### Requirement: Claude Code 的聊天端命令与回合能力

Claude Code 档案 SHALL 声明 `session/resume` 可用于重连与休眠唤醒，SHALL 使用 `_session/steering` 和 `_meta.steering.idleBehavior=promptRequired` 处理插话竞态。本次实测中，空闲 steering 返回 `{"outcome":"promptRequired","reason":"noRunningTurn"}`，不得启动新回合。

图片与嵌入资源 SHALL 按握手中的 prompt 能力沿用现有附件管线。0.99.1 对二者均声明支持，并已在同一真实 prompt 中完成 `end_turn`。不支持的能力 SHALL 按现有规则降级，SHALL NOT 假定所有 MIME 类型均可发送。

连接器 0.99.1 发布的命令菜单已经不包含 `login` 与 `logout`。档案 SHALL 继续声明隐藏这两项作为兼容保护，WebMux SHALL NOT 通过其他入口在聊天端发起认证流程或注销服务用户的全局 Claude 登录。

Claude TUI 启动后查询 OSC 7501 能力，收到 WebMux 应答后发布 `state=idle:app=claude-code`。启动阶段不产生假 `working`，档案的 `StartupQuiet` SHALL 为 0；prompt 使用 `working → done`，退出使用 `clear`。

每一次 TUI 启动 SHALL 使用一次 Esc 作为中断键。本次实测一次 Esc 使进行中的回合 `working → idle`，进程继续运行。

#### Scenario: 空闲时插话

- **WHEN** 用户发送插话时 Claude 回合恰好结束
- **THEN** 适配器返回 `promptRequired`，WebMux 按现有队列规则处理，不产生未跟踪的独立回合

#### Scenario: 认证命令不出现在菜单

- **WHEN** 用户在 Claude Code 混合会话的聊天视图中输入 `/`
- **THEN** 命令菜单中没有 `/login` 或 `/logout`

### Requirement: Claude Code 的取消与后台进程边界

`session/cancel` SHALL 只视为当前 ACP 回合的中断，SHALL NOT 描述为整个进程树的清理。档案的强制切换说明 SHALL 明确：已经启动的后台 Bash 或子任务可能在 cancel 后继续运行；强制切换必须终止 ACP 连接器并等待其受管后代进程退出，不能仅等待 prompt 返回 `cancelled`。

本次实测中，后台 `sleep` 在 cancel 返回后一秒仍存活；终止连接器后，受测 Claude、shell 与后台进程全部退出。实现 SHALL 保留进程组清理与测试，不得把这一结果推广为对任意自行 daemonize 进程的保证。

#### Scenario: cancel 留下后台进程

- **WHEN** Claude 在真实 ACP 回合中启动 `sleep 60`，客户端随后发送 `session/cancel`
- **THEN** prompt 返回 `stopReason=cancelled`，后台进程可能仍存活，界面按档案说明提示

#### Scenario: 强制切换清理受管进程

- **WHEN** 用户确认强制切换，且后台任务仍属于 ACP 连接器的受管进程树
- **THEN** 系统终止连接器并等待受管后代退出后才启动另一端，不把 cancel 当作清理完成
