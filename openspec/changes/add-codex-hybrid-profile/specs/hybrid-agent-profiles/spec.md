## MODIFIED Requirements

### Requirement: 内置档案与可识别的命令写法

系统 SHALL 内置 OpenCode、omp 与 Codex 三个档案，SHALL NOT 开放用户自定义档案。启动配置的 argv 只有在以下两种写法之一中，且可执行文件的基本名与某个档案匹配时，才 SHALL 视为可用于混合模式：

1. 直接执行：`[<可执行文件>, 参数…]`；
2. webmux 的 bashrc 包装：`["bash", "-ic", "exec -- \"$@\"", "webmux", <可执行文件>, 参数…]`。

其他任何写法（例如其他 shell 包装、`env` 前缀）SHALL 视为不可用，并给出原因“命令写法不受支持”。
未匹配任何档案时，原因文字 SHALL 为“未识别到支持的 Agent，当前版本只支持 OpenCode、omp 与 Codex CLI”。

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

## ADDED Requirements

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
