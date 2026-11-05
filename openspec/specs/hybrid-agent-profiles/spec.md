# hybrid-agent-profiles Specification

## Purpose

内置 agent 档案描述 webmux 如何识别、启动、恢复和中断某个支持 ACP 的 agent，让混合会话在 TUI 与 ACP 两端使用一致的命令、参数与环境。新增 agent 时只需新增档案，不改变会话编排。
## Requirements
### Requirement: 内置档案与可识别的命令写法

系统 SHALL 内置 OpenCode、omp、Codex、Claude Code、Qoder 与 QoderCN 六个档案，SHALL NOT 开放用户自定义档案。启动配置的 argv 只有在以下两种写法之一中，且可执行文件的基本名与某个档案匹配时，才 SHALL 视为可用于混合模式：

1. 直接执行：`[<可执行文件>, 参数…]`；
2. webmux 的 bashrc 包装：`["bash", "-ic", "exec -- \"$@\"", "webmux", <可执行文件>, 参数…]`。

其他任何写法（例如其他 shell 包装、`env` 前缀）SHALL 视为不可用，并给出原因“命令写法不受支持”。
未匹配任何档案时，原因文字 SHALL 为“未识别到支持的 Agent，当前版本只支持 OpenCode、omp、Codex CLI、Claude Code、Qoder 与 QoderCN”。

#### Scenario: 预置的 bashrc 包装

- **WHEN** 配置的 argv 为 `["bash","-ic","exec -- \"$@\"","webmux","omp"]`
- **THEN** 判定为可用，档案为 omp

#### Scenario: Codex 预置配置

- **WHEN** 配置的 argv 为 `["bash","-ic","exec -- \"$@\"","webmux","codex","--dangerously-bypass-approvals-and-sandbox"]`
- **THEN** 判定为可用，档案为 codex

#### Scenario: 自定义包装

- **WHEN** 配置的 argv 为 `["sh","-c","omp"]`
- **THEN** 判定为不可用，原因为命令写法不受支持

### Requirement: 使两端看到不同会话库或目录的参数

以下参数 SHALL 使配置不可用于混合模式：

- 让 TUI 与 ACP 看到不同会话库的参数，原因为“该参数使聊天视图无法找到会话”：
  - omp 的 `--session-dir` 只对 ACP 新建的会话生效，ACP 的列出、加载与恢复始终使用默认目录；`--no-session` 不保存会话；
  - OpenCode 的 `--server` 使 TUI 连接另一个服务及其会话库；
  - Codex 的 `--remote`、`--remote-auth-token-env` 使 TUI 连接远端 app-server。
- 在命令中指定工作目录的参数，原因为“请在新建会话时选择目录，不要在命令中写目录”：
  OpenCode 的位置参数、omp 的 `--cwd`，以及 Codex 的 `-C`/`--cd`、`--worktree`。
- 打开已有会话的子命令，原因为“该参数打开已有会话，混合配置请不要指定会话”（原因码 `session_arg`）：Codex 的 `resume`、`fork`。

omp 的 `--profile` SHALL NOT 视为不可用：它同时透传给 ACP 进程，两端使用同一个配置档的会话库。

#### Scenario: OpenCode 命令中写了目录

- **WHEN** 用户对 argv 为 `["opencode","/home/u/proj"]` 的配置勾选混合模式
- **THEN** 保存返回 400，原因提示在新建会话时选择目录

#### Scenario: Codex 命令中写了恢复子命令

- **WHEN** 用户对 argv 为 `["codex","resume","--last"]` 的配置勾选混合模式
- **THEN** 保存返回 400，原因码为 `session_arg`

### Requirement: 不启动交互界面的参数

以下参数 SHALL 使配置不可用于混合模式，原因为“该参数不启动交互界面”：

- omp 的 `-p`/`--print`、`--mode`、`--export`、`--alias`、`-h`/`--help`、`-v`/`--version`，以及首个位置参数为 omp 子命令（例如 `commit`、`acp`）的写法；
- OpenCode 的 `-h`/`--help`、`-v`/`--version`、`--completions`、`--wizard`；
- Codex 的 `-h`/`--help`、`-V`/`--version`，以及首个位置参数为以下子命令的写法：`exec`/`e`、`review`、`login`、`logout`、`mcp`、
  `plugin`、`app-server`、`remote-control`、`app`、`completion`、`update`、`doctor`、`sandbox`、`debug`、`apply`/`a`、`cloud`、
  `exec-server`、`features`、`agents`、`queue`、`archive`、`delete`、`unarchive`、`help`，以及以 `migrate-` 开头的子命令。

#### Scenario: 非交互用法

- **WHEN** 用户对 argv 为 `["omp","-p","总结改动"]` 的配置勾选混合模式
- **THEN** 保存返回 400，原因为该参数不启动交互界面

#### Scenario: Codex 非交互子命令

- **WHEN** 用户对 argv 为 `["codex","exec","修 bug"]` 的配置勾选混合模式
- **THEN** 保存返回 400，原因为该参数不启动交互界面

### Requirement: 恢复命令的生成

切换或恢复到 TUI 时，系统 SHALL 按以下方式生成恢复命令：

- 由可执行文件、档案要求注入的 TUI 参数、原 argv 中位于档案白名单内的参数、档案的恢复参数与 agent 会话 ID 组成，
  并套用与原命令相同的写法（直接执行或 bashrc 包装）。恢复参数：OpenCode 为 `-s <id>`，omp 为 `--resume <id>`，
  Codex 为子命令 `resume <id>`（全局参数位于子命令之前）。
- 白名单参数 SHALL 同时识别 `--flag value` 与 `--flag=value` 两种形式，并按档案声明的参数个数解析。
- Codex 的白名单为：`-m`/`--model`、`-p`/`--profile`、`-s`/`--sandbox`、`-a`/`--ask-for-approval`、`--approve-for-me`、
  `--dangerously-bypass-approvals-and-sandbox`、`--dangerously-bypass-hook-trust`、`-c`/`--config`、`--enable`、`--disable`、
  `--search`、`--add-dir`、`--oss`、`--local-provider`、`--no-alt-screen`、`--strict-config`、`--no-daemon`。
- 系统 SHALL 丢弃以下参数：
  - omp 与 Codex 的位置参数（提示词）；
  - 提示词与附图参数（例如 OpenCode 的 `--prompt`、Codex 的 `-i`/`--image`）；
  - 会话类参数（`-c`、`--continue`、`-r`、`--resume`、`--restore`、`-s`、`--session`、`--fork` 等，按档案区分）；
  - 白名单之外的参数。
- 被丢弃的参数 SHALL 在切换结果中列出，界面 SHALL 提示“以下参数在恢复时未保留”。

#### Scenario: 丢弃提示词与会话参数

- **WHEN** 原 argv 为 `["omp","--model","haiku","--continue","修 bug"]`，agent 会话 ID 为 `01abc`
- **THEN** 恢复命令为 `["omp","--model","haiku","--resume","01abc"]`，界面列出未保留的 `--continue` 与 `修 bug`

#### Scenario: Codex 子命令式恢复

- **WHEN** 原 argv 为 `["codex","-m","gpt-5.6-luna","-s","read-only","修 bug"]`，agent 会话 ID 为 `01a18715`
- **THEN** 恢复命令为 `["codex","--no-daemon","-m","gpt-5.6-luna","-s","read-only","resume","01a18715"]`，界面列出未保留的 `修 bug`

### Requirement: OpenCode 的 TUI 以独立服务运行

OpenCode 的 TUI 默认连接后台服务，回合在后台服务中运行，结束 TUI 并不会停止回合。因此，混合会话中 OpenCode 的每一次 TUI 启动（包括首次启动与恢复）SHALL 带上 `--standalone`，使 TUI 持有自己的服务进程，结束 TUI 即停止其回合。

- 原 argv 已包含该参数时 SHALL NOT 重复添加。
- 首次启动时追加的参数 SHALL 在会话信息的命令中可见。

#### Scenario: OpenCode 以独立服务运行

- **WHEN** 混合配置的 argv 为 `["opencode","--prompt","hi"]`，agent 会话 ID 为 `ses_1`，用户从聊天切到终端
- **THEN** 恢复命令为 `["opencode","--standalone","-s","ses_1"]`，界面列出未保留的 `--prompt hi`；之后切回聊天时，结束 TUI 即停止其回合

### Requirement: ACP 启动命令

系统 SHALL 按档案声明的 ACP 启动方式启动 ACP 进程，并套用与原命令相同的写法：

- 内置子命令：OpenCode 为 `opencode acp`，omp 为 `omp acp`；
- 第三方连接器：Codex 为 `codex-acp`，启动前 SHALL 按 `acp-connector-availability` 检查连接器已安装。

ACP 进程的参数透传按 agent 区分：

- omp：`omp acp` 解析完整 argv，并静默忽略未知参数。因此原 argv 中位于 omp 白名单内的参数 SHALL 全部透传给 ACP 进程（例如 `--model`、`--thinking`、`--profile`、`--config`、`--approval-mode`、`--auto-approve`），使两端使用相同的模型、配置档与批准策略。
- OpenCode：`opencode acp` 只接受 `--login`，因此 OpenCode 的白名单参数 SHALL NOT 透传。`--auto` 也 SHALL NOT 在聊天视图提示不支持，因为聊天端的工具批准由聊天视图的权限模式决定（默认自动批准）。
- Codex：`codex-acp` 不接受这些参数，白名单参数 SHALL NOT 透传。模型、推理强度与批准/沙箱参数按“Codex 的聊天端配置推导”延续；
  `-p`/`--profile`、`-c`/`--config`、`--enable`、`--disable`、`--search`、`--add-dir`、`--oss`、`--local-provider` 在聊天端不生效，
  SHALL 在聊天视图列为“以下参数在聊天端不生效”。

使用 bashrc 包装时，`.bashrc` 执行期间写到 stdout 的内容 SHALL NOT 进入 ACP 协议通道。

#### Scenario: bashrc 有输出

- **WHEN** 用户的 `.bashrc` 会 `echo` 一行欢迎语，混合会话从聊天开始
- **THEN** ACP 握手成功，欢迎语不导致协议错误

#### Scenario: omp 参数透传

- **WHEN** 混合配置的 argv 为 `["omp","--profile","work","--model","haiku","修 bug"]`，会话从聊天开始
- **THEN** ACP 进程的 argv 为 `["omp","acp","--profile","work","--model","haiku"]`

#### Scenario: Codex 使用连接器

- **WHEN** 混合配置的 argv 为 `["bash","-ic","exec -- \"$@\"","webmux","codex","--search","-m","gpt-5.6-luna"]`，会话从聊天开始
- **THEN** ACP 进程以 bashrc 包装执行 `codex-acp`，不带任何参数；聊天视图列出 `--search` 在聊天端不生效

### Requirement: 两端环境一致

ACP 进程、恢复后的 TUI 与最初的 TUI SHALL 使用同一套环境：

- 启动配置保存的环境变量；
- 服务进程环境，按现有规则移除宿主终端变量；
- webmux 注入的变量，包括同一个 `WEBMUX_SESSION_ID`。

`webmux run --launch-config` 创建的混合会话同样 SHALL NOT 继承调用方 shell 的环境。

传给 ACP `session/new`、`session/list`、`session/load` 与 `session/resume` 的 cwd SHALL 是解析符号链接后的绝对路径。TUI 记录的是解析后的路径，而 OpenCode 的 `session/list` 按字符串精确匹配 cwd，未解析的符号链接路径查不到 TUI 创建的会话。

#### Scenario: 符号链接目录

- **WHEN** 用户在符号链接目录 `/home/u/proj-link`（指向 `/home/u/proj`）中新建 OpenCode 混合会话，在 TUI 中发出首条消息后切到聊天
- **THEN** 资格核对与 `session/load` 都使用 `/home/u/proj`，切换成功

#### Scenario: 配置文件目录变量

- **WHEN** 混合配置设置了 `PI_CODING_AGENT_DIR=/tmp/agentdir`
- **THEN** ACP 进程与恢复后的 TUI 都能看到同样的值，并能列出同一目录中的会话

### Requirement: 档案与混合开关的校验

保存启动配置时，若 `hybrid: true` 而 argv 不满足可识别的写法，服务端 SHALL 返回 400，并给出不可用原因。启动配置摘要与完整配置 SHALL 提供只读字段 `hybridSupport`，内容为 `{supported, agent?, reason?, connector?, supportedAgents}`，供设置页决定勾选框是否可用及显示原因。

#### Scenario: 对 shell 配置勾选

- **WHEN** 客户端保存 `{name: "Shell", cmd: ["/bin/bash","-l"], hybrid: true}`
- **THEN** 服务端返回 400，原因为未识别到支持的 agent

### Requirement: 版本提示

agent 的版本来自终端信号、ACP `initialize` 返回的 `agentInfo.version`，或档案声明的版本命令。版本低于档案记录的实测版本时，系统 SHALL 在会话页显示提示，SHALL NOT 因此阻止切换；能否切换只由资格判定决定。版本无法解析时 SHALL 不提示。

- OpenCode 与 omp 的实测版本分别为 2.1.4 与 18.13.1，来自终端信号或 `agentInfo.version`。
- 使用第三方连接器的档案 SHALL 分别记录 agent 与连接器的实测版本：Codex 0.165.0（来自 `CODEX_PATH` 所指可执行文件的 `--version` 输出），
  codex-acp 2.3.0（来自 `agentInfo.version`）。两者任一低于实测版本时 SHALL 分别提示。

#### Scenario: 旧版本

- **WHEN** ACP `initialize` 返回 omp 版本 18.0.0
- **THEN** 聊天视图显示“低于已验证版本 18.13.1”的提示，其余功能照常

#### Scenario: 旧版连接器

- **WHEN** Codex 混合会话的 `initialize` 返回 `agentInfo.version` 为 2.0.0，`codex --version` 为 0.165.0
- **THEN** 聊天视图只提示 Codex ACP 适配器低于已验证版本 2.3.0

### Requirement: 连接器检查跟随启动文件 shell

混合会话在启动 ACP 进程前检查档案声明的第三方连接器时，若配置使用启动文件包装，SHALL 使用同一 shell（Bash 或 Zsh）执行 `command -v`。启动文件的输出 SHALL 先于结果标记；超时、输出上限、结果解析与“启动前以 127 退出表示连接器缺失”的判定 SHALL 与既有规则一致。

#### Scenario: Zsh 下连接器已安装

- **WHEN** 配置使用 `zsh -ic` 包装且 `~/.zshrc` 设置了包含 `codex-acp` 的 PATH
- **THEN** 连接器检查经同一 Zsh 包装找到 `codex-acp`，聊天端启动

#### Scenario: Zsh 启动文件有输出

- **WHEN** `~/.zshrc` 输出多行提示后以 127 退出且未找到连接器
- **THEN** 检查结果仍不含启动文件输出，并判定为连接器缺失

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

Claude Code 档案 SHALL 声明 `session/resume` 可用于崩溃重连，SHALL 使用 `_session/steering` 和 `_meta.steering.idleBehavior=promptRequired` 处理插话竞态。本次实测中，空闲 steering 返回 `{"outcome":"promptRequired","reason":"noRunningTurn"}`，不得启动新回合。

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

### Requirement: Codex 的 TUI 不使用共享 daemon

混合会话中 Codex 的每一次 TUI 启动（包括首次启动与恢复）SHALL 带上 `--no-daemon`。
Codex 默认把回合交给共享的 app-server daemon 执行，daemon 位于 webmux 的进程组之外，结束 TUI 不会停止回合。

- 原 argv 已包含该参数时 SHALL NOT 重复添加。
- 首次启动时追加的参数 SHALL 在会话信息的命令中可见。

#### Scenario: 结束 TUI 不残留进程

- **WHEN** Codex 混合会话从终端开始并发出一条消息，用户切到聊天
- **THEN** TUI 以 `codex --no-daemon …` 运行；切换结束后，会话的进程组中没有残留的 codex 或 app-server 进程

### Requirement: Codex 的 ACP 环境

Codex 的 ACP 进程（含资格探测）SHALL 在“两端环境一致”的环境之上额外注入：

- `CODEX_PATH`：TUI 实际使用的 `codex` 可执行文件，按与连接器检查相同的写法与环境解析；找不到时 SHALL NOT 注入，
  聊天视图 SHALL 提示“未找到 codex，聊天端使用适配器自带的 Codex”；
- `NO_BROWSER=1`：聊天端不执行浏览器登录。

启动配置已设置同名变量时，SHALL 使用配置中的值，SHALL NOT 覆盖。

#### Scenario: 两端使用同一个 codex

- **WHEN** 服务 PATH 中 `codex` 解析为 `/usr/local/bin/codex`，配置未设置 `CODEX_PATH`
- **THEN** `codex-acp` 进程的环境中 `CODEX_PATH=/usr/local/bin/codex`、`NO_BROWSER=1`

### Requirement: Codex 的聊天端配置推导

Codex 在每次 `session/load` 后都会把模型、推理强度与批准/沙箱模式恢复为默认值。因此 Codex 档案 SHALL 声明
`model`、`reasoning_effort`、`mode` 为加载后重新应用的配置项，并 SHALL 提供从 argv 推导这些值的映射：

| argv | 配置项取值 |
|---|---|
| `-m <m>`/`--model <m>` | `model = <m>` |
| `-c model_reasoning_effort=<e>`（值可带引号） | `reasoning_effort = <e>` |
| `--dangerously-bypass-approvals-and-sandbox` | `mode = agent-full-access` |
| `--approve-for-me` | `mode = agent` |
| `-s read-only` | `mode = read-only` |
| `-s workspace-write` | `mode = workspace-write` |

多个批准/沙箱参数同时出现时，按表中自上而下的顺序取第一个匹配。其他批准/沙箱组合（例如 `-s danger-full-access` 或单独的 `-a`）
SHALL NOT 推导 `mode`，并 SHALL 在聊天视图列为“以下参数在聊天端不生效”。

适配器在时间线中插入的“recorded with model … resuming with …”文本 SHALL 原样显示，系统 SHALL NOT 改写 agent 的消息内容。

#### Scenario: 预置配置的批准模式

- **WHEN** 预置的 Codex CLI 混合配置从终端开始，发出首条消息后切到聊天
- **THEN** 聊天端的 `mode` 为 `agent-full-access`，而不是适配器默认的 `agent`

### Requirement: Codex 的聊天端命令

Codex 档案 SHALL 声明在聊天视图命令菜单中隐藏的命令 `logout`：在聊天中执行它会注销服务用户的全局 Codex 登录，而聊天端不执行登录。
隐藏的命令 SHALL NOT 出现在命令菜单中；用户手动输入 `/logout` 时 SHALL 按普通文本发送给 agent。

#### Scenario: 命令菜单

- **WHEN** 用户在 Codex 混合会话的聊天视图中输入 `/`
- **THEN** 命令菜单中没有 `/logout`

