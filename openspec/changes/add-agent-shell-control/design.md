# Design

## Context

- 本机 API：`internal/api/local.go` 的 `NewLocal` 在 Unix 套接字上挂载与 TCP 相同的路由，外加 `/local/v1/*`。
  认证靠套接字对端 UID，不检查 Token、Host、Origin。会话内调用者（`InSession(pid)`）被禁止四件事：停服务、读 Token、控制 tunnel、改 Tailscale。
- 现有 CLI：`internal/sessioncmd` 实现 `run`、`attach`、`ls`、`open`，经 `localclient.Client` 访问套接字，
  Windows 控制台客户端通过 `webmux bridge` 复用同一套命令。
- 终端 WebSocket：`FrameInput` 与 `FrameSubmit` 都会调用 `s.Claim(sub)`，即“输入即夺取控制”。
  `FrameSubmit` 会得到 `SubmissionStatus` 回执（received、written、partial、failed、missing）。
- 状态：`SessionInfo.status`（running、waiting、idle、exited）与锁存的 `attention`。
  提醒按来源分层：`OSC 7501` 根记录高于 Warp 事件，高于 `OSC 9`/`777`，高于响铃。
- 快捷指令与初始化指令：`{{dir}}` 已能展开为会话临时文件夹，`{{user}}` 是首条消息的位置，`{{webmux}}` 保留但会被拒绝。
  预置快捷指令“临时文件夹”承载了临时文件夹说明。

## Goals / Non-Goals

**Goals**

- 脚本和 agent 只用 `webmux` 命令就能完成编排：创建、输入、等待、读取、结束会话，驱动混合会话聊天，准备 worktree，开端口转发，发出提醒。
- 输出可组合：机器可读输出用 JSON 或 NDJSON，数据只写标准输出，诊断只写标准错误，退出码稳定。
- agent 能按 skill 的方式发现这些工具：指令里只放一段简述，需要时再读取完整手册，手册与正在运行的服务版本一致。

**Non-Goals**

- 不新开 TCP 控制入口，远程控制用 `ssh host webmux …`。
- 不新增会话内权限限制，也不放宽现有限制。
- 不在服务端渲染屏幕文本，不为代码视图、上传、推送订阅、设置项做专用命令。
- 不实现 MCP 服务端。

## Decisions

### D1 命令放在 `sessioncmd`，传输只用本机 API

新命令与 `run`、`attach` 一样经 `Env.Connect` 获取 `localclient.Client`，Windows 客户端因此自动获得同样的命令。
任何命令都不读 Token，也不走 TCP。备选方案“在 TCP 上加控制 Token”被否决：
会多出一条需要保护的凭据，而远程场景用 SSH 加本地 CLI 已经足够。

### D2 `webmux api` 只开放 `/api/` 与 `/local/v1/`

`api` 是逃生口，让暂时没有专用命令的接口也能脚本化。路径必须以 `/api/` 或 `/local/v1/` 开头。
`/ws/` 路径以退出码 2 拒绝，并提示改用 `events`、`logs` 或 `chat watch`。
请求体只支持 JSON，multipart 上传不在范围内。方法默认 GET，给了 `-d` 时默认 POST，与 curl、`gh api` 的习惯一致。
服务端的会话内限制照常生效，CLI 不做额外判断。

### D3 发现：`WEBMUX_SOCKET` 优先于默认配置

服务端创建会话时设置 `WEBMUX_SOCKET=<本实例套接字>`，并把它加入 `contract.ServerOwnedEnv`，
因此 `run` 不会把调用方的值转发进新会话，创建请求也不能覆盖它。

CLI 的定位顺序是：显式 `--socket`，然后显式 `--config`，然后 `WEBMUX_SOCKET`，最后默认配置（`--socket` 的用途见 D14）。`WEBMUX_SOCKET` 指向的套接字不可达时，
CLI 以退出码 3 结束，不回退到默认配置。回退可能连到另一个实例，替调用方操作错误的会话。

`self` 在客户端解析为 `$WEBMUX_SESSION_ID`。变量未设置时以退出码 2 结束。

### D4 `send` 用提交帧，默认夺取控制

`send` 把文字作为一次 `FrameSubmit` 发出：正文是文字，提交键在 `--enter` 时为 `\r`，否则为空。
CLI 等到 `written` 才以 0 退出，`partial`、`failed`、`missing` 以 1 退出，从而知道输入确实进入了 PTY。
`--key` 指定的命名按键（`enter`、`esc`、`tab`、`ctrl-c`、`ctrl-d`、`up`、`down`、`left`、`right`、`backspace`）
在文字之后作为 `FrameInput` 发出。

控制权沿用“输入即夺取控制”：浏览器会变成观察者，控制指示器显示连接标签。
`send` 的标签固定为 `script`，在会话内运行时为 `script · <来源会话 ID>`，用户能看出是谁接管了终端。
已决定沿用抢占语义，不做“注入但不夺权”（见“决策记录”）。

会话处于混合会话的聊天端或切换中时，`send` 以 1 退出，提示改用 `chat send`。

### D5 等待要避开“状态还没变”的竞态

`send` 之后立刻 `wait --until idle` 有竞态：agent 可能还没进入 running，`wait` 看到旧的 idle 就立即返回。设计两层防护：

- `wait --changed`：忽略订阅时的初始状态，只接受订阅之后发生的状态转变。
- `send --wait STATE`：先订阅 `/ws/sessions`，再提交输入，只接受提交之后的转变。推荐编排脚本使用这种方式。

`--stable DUR` 要求状态保持一段时间再返回，用于过滤 agent 在工具调用之间短暂的 idle。
超时以退出码 124（`ExitTimeout`，与 GNU `timeout` 一致）结束。会话被移除时以 1 结束。等到 `exited` 时退出码为 0，`--json` 输出包含会话的 `exitCode`。

### D6 `logs` 默认输出原始字节

服务端不做终端模拟，`GET /buffer` 返回带控制序列的原始回放。`logs` 默认原样输出，可以交给 `less -R` 或其他工具。
`--strip` 在客户端尽力去除 CSI、OSC、DCS 和其他 ESC 序列，把 `\r\n` 规范为 `\n`，丢弃单独的 `\r` 之前被覆盖的同一行内容。
这对行式输出的 shell 足够，对全屏 TUI 不可靠，手册会引导改用 `chat`。

`--follow` 以 `attach --observe` 的同一观察者通路连接，不夺取控制、不改变尺寸，把输出写到非 TTY 的标准输出。

### D7 `notify` 走接口而不是控制终端

最初考虑过向 `/dev/tty` 写 `OSC 9`。但 agent 的工具子进程常常没有控制终端，而且 `OSC 7501` 报告过状态的会话会忽略 `OSC 9`，
Claude Code、Codex 正是这样，而它们恰恰是最常见的调用方。因此改为新接口
`POST /api/sessions/{id}/attention`，请求体为 `{kind, message?}`：

- `kind` 取 `notify`、`done`、`error`、`question`，来源记为新的 `cli`。
- CLI 提醒不参与来源分层，总是覆盖当前锁存的提醒，之后由原有规则清除（用户查看、`OSC 7501` 状态变化等）。
- `message` 最长 200 个字符，只用于推送通知正文和提醒提示，只保存在内存中，不写入磁盘，也不出现在会话列表的其他字段里。
  这与“终端输出里的正文从不保存”并不冲突，因为这段文字是调用方明确交给 WebMux 的。
- 默认目标是 `self`。

### D8 聊天命令复用聊天接口，`send --wait` 输出最终回复

`chat send` 调用 `POST .../chat/prompts`，`clientRequestId` 由 CLI 生成，附件先经会话上传接口上传再引用。
`--wait` 先连接 `/ws/s/{id}/chat`，再发送提示，跟踪对应 `promptId` 的回合：
回合结束时输出 agent 消息的最终文本（`--json` 时输出该回合的事件数组），按 `stopReason` 决定退出码：
`end_turn` 为 0，`cancelled` 与其他原因为 1。回合中出现权限或问题请求时，
默认以 1 退出并输出请求 ID，方便上层用 `chat answer` 处理。`--auto-wait-requests` 则一直等到用户在 Web 端回答。

### D9 手册：llms.txt，不写任何文件

手册按 llms.txt 约定放在站点根路径：`/llms.txt` 是索引（H1、引用块摘要、按组的链接列表、`## Optional`），
`/llms/<主题>.md` 是分主题正文，`/llms-full.txt` 是全文。内容来自 `internal/agentguide` 用 `go:embed` 嵌入的 Markdown，
只在内存中渲染，不写入临时根，也不写入会话临时文件夹。

选择 llms.txt 而不是 skill 文件，原因如下：

- **渐进加载**：agent 先读一页索引，再只读需要的主题，比一次读完整的 `SKILL.md` 更省上下文。
- **没有文件生命周期问题**：不用担心系统清理临时目录、权限或版本漂移；每次读取都来自正在运行的服务。
- **入口通用**：有 HTTP 工具的 agent 直接读地址，只有 shell 的 agent 用 `webmux guide` 或 `curl --unix-socket`。
  想装成 skill 的用户可以用 `webmux guide --full` 自行生成。

访问边界：`/llms*` 不在 `/api/`、`/ws/` 之下，按现有中间件只受 Host 检查约束，不需要 Token，与静态页面相同。
因此 TCP 上的内容必须是纯静态说明：不含版本、主机、路径或任何会话数据。公网入口上的未认证访问者能看到的，
与开源仓库文档以及登录页已经暴露的信息相同。本机套接字上的版本在摘要后附加一行版本号。
路由 SHALL 在 `GET /` 的 SPA 回退之前注册。

### D10 `{{webmux}}` 的简述与临时文件夹

`{{webmux}}` 展开为固定格式的简述，会话相关的值在展开时填入：

```text
本会话运行在 WebMux 中（会话 ID：<id>）。
- WebMux 命令：<bin>（套接字：<socket>）
  `webmux` 不在 PATH 中时用这个绝对路径；环境变量 WEBMUX_* 丢失时加 `--socket <socket>`，并用上面的会话 ID 代替 self。
- WebMux shell 工具手册（llms.txt）：<本机链接>/llms.txt
  也可以运行 `<bin> guide`，不需要 Token。需要查看或控制会话、启动或等待 agent、驱动混合会话聊天、
  准备 worktree、端口转发或发送提醒时，先读索引再按需读主题；任务与此无关时不必读取。
- 本会话的临时文件夹：<dir>
  交付给我的截图、报告、导出数据等文件保存到这里而不是项目仓库，规则见 /llms/artifacts.md。
```

临时文件夹说明原先是一条完整的预置快捷指令，现在只在简述里保留路径和一句强制规则，其余移入 `/llms/artifacts.md`。这样做的原因：

- 交付物规则对几乎每个任务都适用，留一句在简述里，agent 即使不读手册也不会把文件放进仓库；
- 上传位置、在代码栏查看、汇报相对路径、会话删除后清理这些细节只在需要时读取；
- `{{dir}}` 仍然可用，用户可以继续写自己的临时文件夹说明。

简述由 `POST /api/sessions/{id}/agent-guide` 生成：确保临时文件夹存在，再返回 `{prompt, guideUrl, dir}`。
生成逻辑放在 `artifacts.Service.Brief(id)`，调用 `internal/agentguide` 的模板函数。快速开始和快捷指令都经它展开，服务端和浏览器的文字因此一致。

### D11 安全

会话内的 agent 本来就能经套接字结束、操控其他会话。同一个 UID 不构成安全边界（agent 可以直接 `kill` 进程），
所以本变更不增加新的禁止项。做三件事降低误用：

- 控制指示器和 CLI 提醒标明来源会话；
- 手册的“操作守则”要求：只操作自己创建或用户点名的会话；不要结束用户的会话；
  不要读取或输出 Token；不要向已开启自动批准的 agent 转发未经核实的外部内容；
- 服务端不记录 `send` 的内容或 `logs` 的输出，沿用“从不记录终端输入输出”。

### D12 启动命令向 Web 端对齐

Web 端现在有两个创建入口，CLI 各给一个对应：

| Web 端 | 能力 | CLI |
|---|---|---|
| 新建会话对话框 | 位置（最近目录、已有 worktree、新建 worktree）、命令（启动配置或自定义）、起始端、会话名称 | `webmux run` |
| 新聊天（快速开始） | 仓库、起始提交、自动分支、agent、起始端、模型等配置项、权限模式、初始化指令、附件、首条消息 | `webmux chat new` |

`run` 的增强：

- `--worktree` 与 `--new-worktree` 对应对话框中的位置选择；
- `cwd` 位于已登记 worktree 内时自动关联，与 Web 端选择该 worktree 的结果一致；
- `--start-side`、`--prompt`、`--init` 使用创建请求已有的字段，不改后端；
- `--json`、`--wait`、`--dry-run`、`--label` 服务于脚本。

另外修改一处现有行为：没有终端时 `run` 自动分离，而不是以用法错误结束。agent 的工具调用几乎都没有终端，
原行为会让最常见的调用直接失败。需要附着时用 `--attach` 强制。

`chat new` 直接调用 `POST /api/quick-starts`，失败语义（按 `step` 报告、不回滚）与 Web 端相同。
`launch-config options` 让脚本能发现 `--set` 可用的键与值。Web 端记住的“上次启动方案”保存在浏览器本地，CLI 不读取：
脚本应当显式写出参数，这样结果不会因为某台浏览器的使用记录而改变。

### D13 会话元数据：详情接口、父会话、标签

只有 `SessionInfo` 不够：脚本还需要知道仓库与分支、启动配置名称、混合会话的 agent 会话 ID、临时文件夹、转发，
以及“哪些会话是我创建的”。

- **详情放在服务端拼装**（`GET /api/sessions/{id}/details`），而不是由 CLI 调多个接口自己拼。
  这样一次请求就能拿到一致的快照，Web 端以后也能复用。接口不创建临时文件夹，`artifactsDir` 只报告已存在的文件夹。
- **父会话自动记录，不接受请求体指定**。用本机套接字对端 PID 判断调用者所在的会话，复用 `ContainsProcess` 的进程树逻辑，返回会话 ID。
  不可伪造，编排者因此可以安全地用 `ls --parent self` 清理自己的子会话。
- **标签是纯内存的键值对**：数量、长度和字符集都有限制，只用于脚本筛选；本变更不在 Web 端显示。
- `get` 增加 `--field`，在没有 `jq` 的环境中也能取单个值；在会话内默认为 `self`。

### D14 可执行文件定位：把服务端自己的路径告诉会话，不改 PATH

手册和简述里的命令全是 `webmux …`，但会话中的 `webmux` 不一定能找到，或者找到的不是同一个版本。常见情形：

- 服务以绝对路径启动（systemd、launchd、`/opt/...`），而 agent 的 shell 用的是精简过的 `PATH`；
- 开发时用 `go run` 或 `bin/webmux` 启动，`PATH` 里是另一个旧版本，甚至根本没有；
- agent 的工具 shell 按策略过滤环境变量（例如 Codex 的 `shell_environment_policy`），`WEBMUX_*` 全部丢失；
- 聊天端的 ACP agent 由服务直接启动，不经过用户的登录 shell。

这些情形里，服务端自己的可执行文件都可用，而且版本正确，问题只是会话不知道它在哪里。所以要做的只是把这个路径告诉会话。

决策：

- **注入 `WEBMUX_BIN`，不改 `PATH`**。把服务目录加进 `PATH` 可能遮住用户自己的同名命令或其他工具，且对过滤环境变量的 shell 同样无效。
  绝对路径加环境变量足够明确，也不改变会话里其他程序的行为。
- **路径就是服务进程自身**：`os.Executable()` 的结果原样注入，不加配置项，也不额外解析。
  Linux 上它读的是 `/proc/self/exe`，本来就是真实路径，Go 也已经去掉原地升级后出现的 ` (deleted)` 后缀；
  `go run` 构建的临时文件在服务运行期间一直存在。会话里执行的因此总是正在运行的服务端那一个文件。
- **注入点唯一**：在 `sessionEnvWithVersion` 中注入，PTY 主进程和 ACP 等辅助进程都经过它，聊天端因此自动得到同样的变量。
- **多重兜底写进文本**：简述里写出 `bin`、`socket` 和会话 ID 的字面值，环境变量丢失也能用；
  CLI 新增 `--socket`；手册的 `connect` 主题给出排错顺序，最后一步是 `curl --unix-socket`。
- **版本偏差**：`WEBMUX_BIN` 与服务是同一个文件，原地升级后两边版本一致；`PATH` 里的旧 CLI 碰到新接口 404 时，提示改用 `$WEBMUX_BIN`。
- **公开内容不含路径**：TCP 层的手册只写 `webmux` 与 `$WEBMUX_BIN`，具体路径只出现在本机套接字版本、简述和 `guide --path` 中。

### D15 `screen`：客户端终端模拟

全屏 TUI 的当前画面无法从原始字节可靠地还原成文本，而 agent 编排者经常需要看到它，比如判断子 agent 是否停在权限选择框上。
架构约束是“服务端不做终端模拟”，所以把模拟放到 CLI 一侧：

- 用观察者连接取回放快照和控制者的网格尺寸，在内存中用终端模拟器重放，输出可见画面后立即断开；
- 模拟库候选为 `charmbracelet/x/vt` 与 `hinshun/vt10x`，实现时按许可证、维护状态、`CGO_ENABLED=0` 兼容性和体积选定；
- WebMux 是单一可执行文件，库会被链接进去，但只能由客户端包引用。新增架构规则：服务端与共享包禁止导入该库；
- 回放有字节上限，被截断时画面可能不完整。全屏 TUI 通常会重绘，影响有限，CLI 在标准错误给出提示。

### D16 帮助：一张命令表生成全部说明

命令数量从 14 个增长到 60 个左右，手写帮助一定会和实现、手册不一致。决策：

- 在 `sessioncmd`（以及 `cmd/webmux` 的运维命令）中声明命令表，选项解析、`--help`、`help --json`、补全脚本、llms 命令参考都由它生成；
- 测试保证：每个选项都有说明和默认值，每个命令至少有两个示例，示例能被该命令的解析器接受，llms 主题覆盖全部命令；
- `--help` 统一写到标准输出并返回 0；用法错误统一返回 2，并提示 `--help`。现有 `run`、`attach`、`ls`、`open` 的行为随之修正；
- 帮助用英文，与现有 CLI 和 Windows 客户端一致。手册（llms）用中文，命令参考部分直接嵌入英文用法与选项；
- 动态补全经隐藏的 `__complete` 查询本机 API，超时 1 秒后退回静态候选，不在补全过程中报错。

### D17 启动配置的完整增删改

按决策提供 `add`、`edit`、`rm`、`presets`、`restore`。需要注意：

- `edit` 是“读取、修改、整体保存”，未给出的字段保持原值，特别是环境变量的值。CLI 先读取完整配置（含值），只在内存中合并；
- 环境变量的值是敏感数据：`show` 默认隐藏，需要 `--reveal-env` 才输出；诊断信息与 `--dry-run` 都不输出它们；
- `--hybrid` 先调用支持检查，避免保存一条无法以混合模式启动的配置。

### D18 仓库设置与主页显示

- “仓库设置”在 Web 端是一个对话框，目前只有初始化指令。CLI 用 `repo settings` 承接，而不是单独的 `repo init`，以后新增设置项时不用再加命令。
- “主页显示”保存在服务端设置 `homeWorktrees` 中，而不是仓库数据里，Web 端在新建 worktree 后由客户端写入。CLI 遵循同一规则，在客户端写入：
  - 只修改目标仓库的条目，带 `baseRevision` 提交，409 时重读重放，不覆盖浏览器同时做的修改；
  - 判定规则（默认只含主工作区、有会话即显示、清理失效 ID）与 `web/src/homeWorktrees.ts` 保持一致，用对照测试防止两边分叉；
  - 没有把规则搬到服务端：那需要改动 Web 端的写入路径，超出本变更范围。若以后两边规则需要同时修改，再考虑新增服务端接口。

### 决策记录（2026-11-05 与用户确认）

| 问题 | 决定 |
|---|---|
| `send` 与控制权 | 抢占控制，沿用“输入即夺取控制”，标签标明来源 |
| `/llms*` 在 TCP 上的认证 | 公开，不需要 Token，内容只有静态说明 |
| 会话内调用者的权限 | 不新增限制，保持现有四项禁止 |
| `notify` 的文字 | 允许最多 200 字，只保存在内存中 |
| 无终端时的 `run` | 自动分离，`--attach` 强制附着 |
| TUI 画面 | 增加 `webmux screen`，在客户端模拟 |
| Web 端显示父会话与标签 | 本次不显示，只提供数据 |
| 启动配置命令 | 完整增删改 |
| 补充命令 | `repo`、混合会话历史与运行时配置、`run --follow`、`help --json`、shell 补全、`notify --clear` |
| 不纳入 | 等待端口（`wait --until listening`）、`ls -q` 与 `ls --field` |
| 超时退出码 | 124，与 GNU `timeout` 一致 |
| PR 拆分 | 按主题拆成多个 PR（见 tasks.md 第 0 节） |
| `--help` 语言 | 英文，与现有 CLI 和 Windows 客户端一致；llms 手册用中文 |
| 仓库设置与主页显示 | `repo settings` 承接仓库设置；主页显示由 CLI 写入 `homeWorktrees`，规则与 Web 端一致 |

## Risks / Trade-offs

- `send` 夺取控制会打断正在浏览器里输入的用户。缓解：标签标明来源，手册要求只操作自己创建的会话。
- `--strip` 对 TUI 输出不可靠。缓解：在手册与帮助中写明，并引导使用 `chat`。
- agent 沙箱可能禁止访问套接字或执行外部程序。缓解：手册要求在这种情况下向用户说明，不要反复重试。
- `/llms*` 在公网入口上不需要 Token。缓解：内容只有静态说明，不含版本与实例信息，并有测试保证。
- 自动分离改变了 `run` 在无终端时的行为。缓解：只把原来的用法错误变成成功，交互终端中的行为不变。
- 命令数量明显增加。缓解：所有命令共用一套参数解析、连接和输出约定，帮助文字按组列出。

## Open Questions

- `notify` 是否应该在 Windows 托盘显示？托盘目前只转发推送，本变更不涉及。
- 以后是否在 Web 端显示父会话与标签，以及是否提供 PowerShell 补全，留待后续变更。
