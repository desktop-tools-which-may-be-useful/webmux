# Proposal

## Why

用户希望用脚本或 CLI agent 控制 WebMux：例如让一个 agent 在新 worktree 里启动另一个 agent、发出任务、等它完成再读取结果，
或者在长任务结束时通知手机。

通道其实已经有了：本机 Unix 套接字挂着与浏览器相同的 `/api/*`、`/ws/*` 路由，靠对端 UID 认证，不需要 Token。
但 shell 侧还有三类缺口：

- **只有 REST、没有命令**：删除、重命名会话，启动配置，worktree，端口转发，混合会话聊天。用 curl 能做，
  但调用方要自己找到套接字、拼 JSON。
- **只有 WebSocket**：curl 做不了，这是真正卡住编排的地方。包括向会话输入并确认已写入、跟随输出、
  等待状态变化（agent 完成或需要介入）、订阅聊天事件。
- **发现机制**：会话只注入了 `WEBMUX_SESSION_ID`。服务端用非默认配置启动时，会话里的 `webmux` 可能找不到实例或连错实例。

另外，初始化指令和快捷指令里预留了 `{{webmux}}` 变量，目前写了就会被拒绝。agent 也没有途径得知 WebMux 提供了哪些 shell 工具。
临时文件夹说明只是一条预置快捷指令，没有和其他 WebMux 能力放在一起。

## What Changes

- **P0 会话控制命令**，全部放在 `internal/sessioncmd`，Linux/macOS CLI 与 Windows 控制台客户端都能用：
  - `webmux api`：通用逃生口，经本机套接字发送任意 `/api/` 请求，原样输出响应；
  - `webmux kill`、`rm`、`rename`：补齐会话生命周期；
  - `webmux send`：以带回执的提交帧写入文字和命名按键，可选 `--wait` 等待状态转变；
  - `webmux wait`：订阅会话列表事件，等到指定状态（running、waiting、idle、exited、attention）；
  - `webmux logs`：读取回放缓冲区，`--follow` 以观察者身份跟随，`--strip` 尽力去除控制序列；
  - `webmux events`：以 NDJSON 输出 `/ws/sessions` 事件；
  - `webmux dir`：输出会话临时文件夹的路径（按需创建）；
  - `webmux screen`：在客户端用终端模拟器把回放渲染成当前屏幕文本，服务端仍不做模拟。
- **P0 发现与身份**：
  - 每个会话（包括聊天端的 ACP agent）注入服务端自有变量 `WEBMUX_SOCKET`（本机 API 套接字路径）与 `WEBMUX_BIN`（可执行文件绝对路径），不修改 `PATH`；
  - `WEBMUX_BIN` 就是服务进程自身的可执行文件（`os.Executable()`）；
  - CLI 定位顺序：`--socket`，然后 `--config`，然后 `WEBMUX_SOCKET`，最后默认配置；
  - 所有接受会话 ID 的命令都接受 `self`，表示 `$WEBMUX_SESSION_ID`；
  - 新增退出码 124 表示等待超时（与 GNU `timeout` 一致）。
- **P0 启动命令向 Web 端对齐**：
  - `webmux run` 支持新建会话对话框中的全部位置选择：`--worktree`、`--new-worktree`，`cwd` 位于已登记 worktree 内时自动关联；
  - 新增 `--start-side`、`--prompt`、`--init`、`--label`、`--json`、`--wait`、`--dry-run`；
  - 没有终端时自动分离，不再以用法错误结束；
  - `webmux chat new` 对应 Web 端“新聊天”，一条命令完成 worktree、agent、配置项、权限模式、初始化指令、附件和首条消息；
  - `webmux run --follow`：不需要终端地跟随持久会话，输出流到标准输出，以程序的退出码退出，Ctrl+C 只分离；
  - `webmux launch-config options` 列出 agent 配置项，供 `--set` 使用。
- **P0 会话元数据**：
  - `GET /api/sessions/{id}/details` 集中返回仓库与分支、启动配置、混合会话记录、临时文件夹、转发、父会话、子会话和标签；
  - `webmux get [SESSION] [--field PATH]`，在会话内默认为 `self`；
  - 经本机 API 从会话内创建的会话自动记录 `parentId`；
  - 会话标签：`run --label`、`webmux label`、`ls --label/--parent/--status`。
- **P1 结构化编排**：
  - `webmux chat send|watch|status|cancel|answer|switch`：经 ACP 结构化通道控制混合会话，`chat send --wait` 输出 agent 的最终回复；
  - `webmux chat ls|resume|forget`（混合会话历史）与 `chat set|permission|queue`（运行时配置与队列）；
  - `webmux notify`：经新接口 `POST /api/sessions/{id}/attention` 发出提醒，`--clear` 清除提醒，不依赖控制终端；
  - `webmux repo ls|add|rm|show|refresh|branches|commits|base` 与 `webmux repo settings`（对应 Web 端“仓库设置”，目前是初始化指令）；
  - `webmux worktree ls|show|create|rm`，以及主页显示 `worktree pin|unpin|home`（与 Web 端的“主页显示”共用服务端设置 `homeWorktrees`）；
  - `webmux launch-config ls|show|add|edit|rm|presets|restore`（完整增删改，变量值默认隐藏）；
  - `webmux forward ls|add|rm`。
- **完整帮助与补全**：
  - 所有命令（包括现有命令）由一张命令表声明，`--help`、`help --json`、shell 补全与 llms 命令参考都由它生成；
  - 每个 `--help` 包含用法、参数、选项与默认值、环境变量、输出、退出码、至少两个示例和相关主题；
  - `--help` 写到标准输出并返回 0，修正现有命令把它当作用法错误的行为；
  - `webmux completion bash|zsh|fish`，支持会话 ID、启动配置、仓库等动态补全。
- **`{{webmux}}` 变为 WebMux 工具说明**：
  - 手册按 llms.txt 约定在站点根路径提供：`/llms.txt` 索引、`/llms/<主题>.md` 分主题、`/llms-full.txt` 全文。
    只在内存中渲染，不写入任何文件；`webmux guide` 经本机套接字读取同一份内容；
  - 手册不假设 `webmux` 在 `PATH` 中：给出“命令前缀”说明与排错顺序，公开内容不含任何安装路径；
  - `{{webmux}}` 展开为一段简述：会话 ID、可执行文件与套接字的绝对路径、`/llms.txt` 地址与 `guide` 读取方式、何时需要读取，以及本会话临时文件夹的路径与一句强制规则；
  - 初始化指令和快捷指令都可以使用 `{{webmux}}`；
  - 现有临时文件夹说明（上传位置、交付物保存规则、在代码栏查看、汇报相对路径、会话删除后清理）移入 `/llms/artifacts.md`；
  - 预置快捷指令“临时文件夹”替换为“WebMux 工具”，正文为 `{{webmux}}`。已保存的列表不迁移。
- **明确不做**：代码视图、文件补全、mkdir、上传、推送订阅、设置项的专用命令（用 `webmux api` 或直接操作文件系统）；等待端口与 `ls -q`；Web 端显示父会话与标签；
  会话内读取 Token、控制 tunnel 或 Tailscale（保持现有禁止）；服务端渲染屏幕文本（违反“服务端不做终端模拟”）；MCP 服务端（后续可在这些命令上薄封装）。

## Capabilities

### New Capabilities

- `shell-session-control`：通用 API 命令、会话生命周期命令、`send`、`wait`、`logs`、`screen`、`events`、`dir`、`notify`，以及共同的输出与退出码约定。
- `shell-session-launch`：`run` 的位置、起始端、首条消息、输出与等待，`chat new` 与 `launch-config options`。
- `session-metadata`：会话详情接口、`webmux get`、父会话与会话标签。
- `shell-cli-help`：命令表、完整 `--help`、帮助的调用约定、`help --json`、shell 补全，以及 llms 命令参考与帮助的一致性。
- `shell-chat-control`：混合会话聊天、历史、运行时配置与队列的 shell 命令。
- `shell-workspace-control`：仓库与仓库设置、worktree 与主页显示、启动配置（完整增删改）与端口转发命令。
- `agent-shell-guide`：llms.txt 形式的手册、访问边界、内容要求、`webmux guide` 与 `{{webmux}}` 的展开规则。

### Modified Capabilities

- `terminal-session-cli`：连接约定加入 `--socket`、`WEBMUX_SOCKET` 与 `self`，旧 CLI 碰到新接口时提示版本过旧，帮助列出新命令；`run` 在没有终端时自动分离。
- `session-launch-environment`：服务端为每个会话设置 `WEBMUX_SOCKET` 与 `WEBMUX_BIN`，并把它们列为服务端自有变量。
- `terminal-quick-commands`：预置指令改为恢复会话、新会话、WebMux 工具三条；快捷指令正文支持 `{{webmux}}`；快捷指令菜单去掉固定项“插入临时文件夹提示词”。
- `server-settings-sync`：退役 `artifactPrompt` 设置项，PATCH 中出现时按未知字段 400，旧文件中的该键读取时忽略。
- `session-artifacts`：移除已失效的“临时文件夹提示词模板”与“从快捷指令菜单插入提示词”两条需求，新增 shell 访问与“手册不写入临时文件夹”。

## Impact

- 后端：
  - 新增共享包 `internal/agentguide`（仅标准库，嵌入手册 Markdown 并渲染简述），在 `internal/archtest/layers_test.go` 中登记为 Shared；
  - `internal/artifacts` 提供 `Brief(id)` 生成简述；
  - `internal/session` 记录 `parentId` 与标签，并能由对端 PID 找到所在会话；
  - `internal/workspace` 在快速开始中展开 `{{webmux}}`，取消保留变量的拒绝；
  - `internal/session` 支持外部提醒；
  - `internal/api` 新增 `/llms.txt`、`/llms-full.txt`、`/llms/{topic}.md`，以及 `POST /api/sessions/{id}/agent-guide`、`GET /api/sessions/{id}/details`、
    `PATCH /api/sessions/{id}/labels`、`POST|DELETE /api/sessions/{id}/attention`；
  - `cmd/webmux` 取 `os.Executable()` 作为 `WEBMUX_BIN`，与 `WEBMUX_SOCKET` 一起注入会话选项，并注册新子命令。
- 接口：`internal/contract`、`internal/wire`、`web/src/schemas.ts` 新增 `AgentGuidePrompt`、`SessionDetails`、`LabelsUpdate`、`AttentionRequest`；
  `SessionInfo` 新增 `parentId`、`labels`，`CreateSessionRequest` 新增 `labels`，`AttentionSource` 新增 `cli`；重新录制 `internal/api/testdata/wire`。
- CLI：`internal/sessioncmd` 引入命令表与帮助生成；`screen` 引入一个纯 Go 终端模拟库，只允许客户端包引用（新增架构规则）。
  `cmd/webmux` 的运维命令迁入同一命令表。
- 前端：`quickCommands.ts` 把 `{{webmux}}` 当作内置变量；`SessionTerminal` 与聊天草稿在插入前调用手册接口；
  初始化指令字段和快速开始页去掉“保留变量”提示；更换预置指令。
- 安全：不新增权限边界。会话内调用者已能经套接字做同样的事，现有四项禁止保持不变。
  控制指示器与提醒标明来源会话，手册写明操作守则。
- 文档：新增用户故事分组（shell 与 agent 控制），更新 US-Q13 与快捷指令、临时文件夹相关故事，更新索引与覆盖表。
