# Design

## Context

动机见 `proposal.md - Why`，完整实测报文与耗时数据见 `baseline.md`。本设计基于对官方
Antigravity CLI 文档（`antigravity.google/docs`）及本机实测 `agy 1.4.12`
（`agy --help`、`agy acp --help`、ACP JSON-RPC stdio 握手与 PTY 交互会话）的核对结果：

1. **ACP 协议与会话能力**：
   - `agy acp` 原生启动 ACP v1 stdio 服务，`initialize` 声明 `loadSession: true` 以及
     `sessionCapabilities: {resume: {}, close: {}, load: {}, list: {}, additionalDirectories: {}}`，
     `agentInfo` 为 `{name: "Antigravity ACP Server", version: "1.4.12"}`；未声明
     `promptCapabilities`（`image: false`、`embeddedContext: false`）与
     `_meta.steering.supported`。
   - `session/new`、`session/load`、`session/resume` 返回 36 字符小写 UUID v4 格式的
     `sessionId` 以及 `configOptions`（`model` 与 `thought_level`，均为 `type: "select"`）。
   - **跨进程加载的配置重置**：在同一 `agy acp` 进程内调用 `session/set_config_option`
     后再次 `session/load` 会保留修改值；但当原 `agy acp` 进程退出、由新启动的 `agy acp`
     进程调用 `session/load` 或 `session/resume` 时，`model` 与 `thought_level`
     会重置为默认值（如 `gemini-4-argon` 与 `medium`）。
   - **实时回合回显 `user_message_chunk`**：与 `omp`、`opencode` 只在 `session/load`
     回放历史时发送 `user_message_chunk` 不同，`agy acp` 在处理实时 `session/prompt`
     请求时也会先发送一条不带 `messageId` 的 `user_message_chunk` 通知，随后再发送
     `session_info_update`、`tool_call`、`tool_call_update` 与 `agent_message_chunk`。
   - `agy` TUI 与 `agy acp` 共享同一个本地会话库（默认位于 `~/.gemini/antigravity-cli`）：
     对话在完成首轮消息后写入 `conversation_summaries.db` 并出现在 `agy acp` 的
     `session/list` 中，可经 `session/load` 回放历史或由 `agy --conversation <id>` 在 TUI 中恢复。
2. **命令行参数差异**：
   - `agy` TUI 支持 `--dangerously-skip-permissions`、`--sandbox`、`--agent`、
     `--add-dir`、`--log-file`、`--model`、`--effort`、`--mode` 等交互参数，以及
     `--conversation <id>` 恢复指定会话。
   - `agy acp` 使用 Cobra 校验子命令参数，接受 `--dangerously-skip-permissions`、
     `--sandbox`、`--agent`、`--add-dir`、`--log-file`，但不接受 `--model`、
     `--effort`、`--mode`（模型与推理等级通过 ACP `configOptions` 控制）。
   - `--project` 与 `--new-project` 会将会话切入云端项目模式，忽略 `--conversation` 本地会话恢复。
3. **TUI 状态与会话身份信号**：
   - 在应答 `ESC ] 7501 ; ? BEL` 的终端中，`agy` TUI 原生发出 `OSC 7501` 根记录
     （`state=idle:app=antigravity`、`state=working:app=antigravity`、
     `state=done:app=antigravity:msg=...`、
     `state=blocked:kind=permission|question|auth:app=antigravity:msg=...`、
     `state=error:app=antigravity:msg=...`），但根记录不包含当前对话的 UUID。
   - `agy` 官方支持会话级 `--settings <json>`（只对当次会话生效，不写回磁盘
     `settings.json`），其中 `title` 命令会在每次状态迁移（`authenticating`、
     `initializing`、`idle`、`working`、`tool_use`、`reviewing`、`error`）时执行，
     并通过标准输入接收包含 `conversation_id`（首条消息前为 `""`，首条消息后或
     `--conversation` 恢复时为 36 字符 UUID）、`cwd`、`agent_state`、`version`、
     `product: "antigravity"` 的 JSON。
   - 当 `title` 命令不向标准输出打印内容而将
     `ESC ] 7501 ; state=working:id=<conversation_id> BEL` 写入 `/dev/tty` 时，
     既不会改动终端窗口标题，又能让 WebMux 的 `OSCScanner` 采集到当前对话 ID。

## Goals / Non-Goals

**Goals:**
- 在不改变 `internal/hybrid` 单写者状态机架构的前提下，扩展 `internal/agentprofile`
  与 `internal/hybrid` 支持 `agy` 混合会话（聊天 ↔ 终端双向切换、空闲休眠唤醒、异常重连与重启恢复）。
- 在规格中完整给出 `agy` 档案配置、`--settings` 注入字符串、协议差异处理与
  `fakeagent` 行为定义，使实现者无需依赖本地 `agy` 环境即可完成开发与测试。
- 将新安装预置的 `Gemini CLI` 启动配置更新为默认开启混合模式的 `Antigravity CLI`
  （`agy --dangerously-skip-permissions`），提供本地打包的 `antigravity` 图标，
  同时保证升级前含 `geminicli` 图标的 `launch-configs.json` 零迁移兼容。

**Non-Goals:**
- 不支持 `agy` 云端项目会话（`--project`、`--new-project`）的混合模式切换。
- 不修改已有用户磁盘上的 `launch-configs.json`。

## Decisions

### 1. `agy` 适配器定义与按参数透传（`internal/agentprofile` 与 `internal/agentprofile/agy`）

在 `internal/agentprofile` 的 `Option` 结构中增加 `PassToACP bool`（保留 `Parser.PassToACP`
作为全量透传开关），并在 `Parser.TUIInject` 中支持多 token 切片 `[]string`。
`internal/agentprofile/agy` 适配器的精确配置如下：

- `Name` / `Title` / `Executable`：`"agy"` / `"Antigravity CLI"` / `"agy"`
- `VerifiedVersion`：`"1.4.12"`
- `InterruptKeys`：`[][]byte{[]byte("\x1b")}`（单次 `Esc`）
- `ResumesSessions()` / `Steers()`：`true` / `false`
- `ReapplyOnLoad()`：`[]string{"model", "thought_level"}`
- `ResumeFlag`：`"--conversation"`
- `TUIInject`：`[]string{"--settings", TUISettingsJSON}`（若原 argv 未含完全相同的设置对，
  则在首次启动与恢复时注入，并剔除用户传入的 `--settings` 覆盖值）
- 白名单且透传 ACP（`Keep, PassToACP: true`）：
  - 开关：`--dangerously-skip-permissions`、`--sandbox`
  - 带 1 个值：`--agent`、`--add-dir`、`--log-file`
- 白名单仅保留于 `ResumeArgv`（`Keep, PassToACP: false`）：
  - 带 1 个值：`--model`、`--effort`、`--mode`；`ChatSide` 将 `--model` 与 `--effort`
    分别转换为 `model` 与 `thought_level` 配置种子，`--mode` 记入聊天端 `unsupported`。
- 丢弃参数（`Drop`）：
  - 位置参数（`PosPrompt`）；开关：`-c`、`--continue`、`--remote-control`；
  - 带 1 个值：`--conversation`、`-i`、`--prompt-interactive`、`--settings`；
  - 以及任何未知参数
- 拒绝：不同会话库（`SessionStore`）：带 1 个值：`--project`、`--new-project`
- 拒绝：非交互或子命令（`NonInteractive`）：
  - 开关：`--disable-slash-commands`、`-h`、`--help`、`-v`、`--version`；
  - 带 1 个值：`-p`、`--print`、`--prompt`、`--output-format`、`--input-format`、
    `--json-schema`、`--print-timeout`；
  - 子命令：`acp`、`agent`、`agents`、`changelog`、`help`、`install`、`mcp`、
    `mic-serve`、`models`、`plugin`、`plugins`、`remote-control`、`update`

### 2. 通过注入会话级 `--settings` + `/dev/tty` `OSC 7501` 子记录上报 TUI 会话 ID

1. `TUISettingsJSON` 常量定义为：
   ```go
   const TUISettingsJSON = `{"title":{"type":"command","command":"id=$(sed -n 's/.*\"conversation_id\":\"\\([0-9a-f-]*\\)\".*/\\1/p'); [ -n \"$id\" ] && printf '\\033]7501;state=working:id=%s\\007' \"$id\" >/dev/tty 2>/dev/null || true"}}`
   ```
   `title.command` 从标准输入提取 `"conversation_id":"<uuid>"`，非空时向 `/dev/tty`
   写入 `\033]7501;state=working:id=<uuid>\007`，且不向 stdout 输出任何字符。
   原命令中的 `--settings` 归为 `Drop` 并在首次启动与恢复时剔除，避免覆盖该钩子。
2. 将 `internal/session/osc.go` 中的 `maxProgramIDSegment` 从 `32` 调整为 `64`
   （总长上限 `maxProgramID = 128` 与最多 8 级保持不变），使 36 字符的标准 UUID 能通过
   `programIDFirstSegment` 校验并写入 `Identity.ProgramChildID`。
3. `agy.Agent.Identify` 在 `s.ProgramApp == "antigravity"` 时返回 `Agent: "agy"`，
   `Cwd: s.RootCwd`；当 `s.ProgramChildID` 为合法的 36 字符小写十六进制 UUID
   （`8-4-4-4-12` 格式）时作为 `AgentSessionID`。

### 3. 跨进程加载补齐 `model` 与 `thought_level`（`ReapplyOnLoad`）

- **做法**：`agy` 适配器设置 `ReapplyOnLoad: []string{"model", "thought_level"}`。
  当会话从 TUI 切到聊天、从休眠唤醒、异常重连或服务重启恢复时，`actor.openACP()`
  读取会话记录中的 `Record.Config`，若新 `agy acp` 进程返回的 `model` 或
  `thought_level` 与记录值不同，则自动调用 `session/set_config_option` 恢复用户此前选定的模型与推理等级。
- **依据**：实测 `agy acp` 在跨进程执行 `session/load` 或 `session/resume` 时总是返回默认
  `model`（`gemini-4-argon`）与默认 `thought_level`（`medium`）。

### 4. 过滤活动回合中 `agy acp` 回显的 `user_message_chunk`

- **做法**：在 `internal/hybrid/actor.go` 的
  `actor.onUpdate(epoch int, n acp.SessionNotification)` 中，当 `a.turn != nil`
  （即存在正在执行的 `session/prompt` 回合）且 `n.Update` 为 `*acp.UserMessageChunk` 时，
  直接忽略该通知；仅在 `a.turn == nil`（即 `session/load` 历史回放阶段）将
  `*acp.UserMessageChunk` 交给 `a.tr.update()` 转换为 `RoleUser` 消息事件。
- **依据**：`actor.send()` 在发出 `session/prompt` 前已通过
  `a.tr.userMessage(text, attachments)` 将用户提示词及附件元数据写入事件流并调用
  `a.tr.breakMessage()`。`agy acp` 在 `session/prompt` 期间回显的不带 `messageId` 的
  `user_message_chunk` 若不忽略，会被 `translator` 分配新的 `wm<N>` 消息 ID，
  导致每次发言在聊天视图中出现两条用户消息气泡。

### 5. 启动配置图标新增 `antigravity` 并保留 `geminicli` 兼容

- 从固定提交 `eb3e353a5cf3720ef83d52058966ed51b10f10fa` 的 `lobehub/lobe-icons` 引入
  `web/src/icons/lobehub/antigravity.svg`。
- 新安装预置配置改为 `{Name: "Antigravity CLI", Icon: "antigravity", Cmd: command("agy", "--dangerously-skip-permissions"), Hybrid: true}`。
- 前端图标选择器展示 `Antigravity CLI`（值 `antigravity`），自动匹配对可执行文件 `agy`
  返回 `antigravity`，对 `gemini` 仍返回 `geminicli`；后端 `launch.normalize` 与前端
  `LaunchConfigIcon.svelte` 同时保留 `geminicli` 与 `antigravity`，确保已有
  `launch-configs.json` 不因未知图标值而返回 503。

## Risks / Trade-offs

- **用户在混合会话中自定义 `--settings` 被丢弃** → `--settings` 归入 `Drop`，
  切换结果明确列出未保留的参数；长期设置仍可通过 `~/.gemini/antigravity-cli/settings.json`
  配置（命令行 `--settings` 只覆盖内联提供的 `title` 键）。
- **`title` 命令依赖 POSIX `sed` 与 `/dev/tty`** → WebMux 仅支持 Linux 与 macOS，
  两者的 `/bin/sh` 与 `sed` 均支持该基本正则，且 `session` 启动 PTY 时始终设置
  `Setsid: true, Setctty: true`，`/dev/tty` 恒可用；即使重定向失败也有
  `2>/dev/null || true` 兜底，不影响 TUI 运行。
- **已有安装的 `launch-configs.json` 不会自动把 `Gemini CLI` 改名为 `Antigravity CLI`** →
  遵循 `launch-configs` 规格“已存在的 `launch-configs.json` 不被改写”的原则，
  老用户可在设置页自行编辑或新建。

## Migration Plan

- 无需数据迁移：`launch-configs.json` 保持 v2，保留 `geminicli` 合法性；
  新安装自动获得 `Antigravity CLI` 混合启动配置。
- 回滚：若需回退到不支持 `agy` 的旧版本，先在设置页取消 `agy` 启动配置的混合模式勾选
  并将图标改回旧版支持的值，避免旧版校验 `launch-configs.json` 时报错 503。
