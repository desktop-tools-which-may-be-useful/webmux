# Antigravity CLI (`agy`) 实测基线（2026-11-03）

本文件记录 `agy 1.4.12` 在本机的 ACP JSON-RPC stdio 与 PTY TUI 实测结果，
供实现 `internal/agentprofile`、`internal/hybrid`、`internal/session` 与
`internal/acp/testdata/fakeagent` 时直接对照，无需在开发或测试环境中安装真实 `agy`。

## 1. 测试方法与环境

- **实测版本**：`agy 1.4.12`（`agentInfo.name = "Antigravity ACP Server"`，
  `agentInfo.version = "1.4.12"`）。
- **数据目录**：`agy` TUI 与 `agy acp` 默认均使用 `~/.gemini/antigravity-cli`，
  其中保存 OAuth 凭据（`antigravity-oauth-token`）、会话摘要库
  （`conversation_summaries.db`）、单会话消息库（`conversations/<uuid>.db`）与
  工作区产物（`brain/<uuid>/`）。混合模式不得覆盖 `--app-data-dir`，否则两端无法共享凭据与本地会话库。
- **测试清理**：所有实测脚本创建的临时会话均已在测试结束时从 `conversations/`、
  `brain/` 与 `conversation_summaries.db` 中清理完毕。

## 2. ACP 协议实测与报文结构（`agy acp`）

### 2.1 `initialize`

客户端发送 `{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":1,"clientCapabilities":{}}}`，
`agy acp` 返回：

```json
{
  "protocolVersion": 1,
  "agentCapabilities": {
    "loadSession": true,
    "load_session": true,
    "sessionCapabilities": {
      "resume": {},
      "close": {},
      "load": {},
      "list": {},
      "additionalDirectories": {}
    },
    "auth": { "logout": {} },
    "_meta": {
      "antigravity": { "tier": true, "systemPrompt": { "append": true } }
    }
  },
  "agentInfo": {
    "name": "Antigravity ACP Server",
    "version": "1.4.12"
  },
  "authMethods": [
    {
      "id": "oauth-personal",
      "name": "Log in with Google",
      "description": "Log in with your Google account"
    },
    {
      "id": "oauth-business",
      "name": "Log in with Gemini Enterprise",
      "description": "Log in with your Gemini Enterprise account"
    },
    {
      "id": "gemini-api-key",
      "name": "Gemini API key",
      "description": "Use an API key with Gemini Developer API"
    },
    {
      "id": "agent-platform",
      "name": "Gemini Enterprise Agent Platform",
      "description": "Use Gemini Enterprise Agent Platform (formerly Vertex AI) with Application Default Credentials"
    }
  ]
}
```

关键观察：

- `loadSession` 为 `true`，`sessionCapabilities` 同时声明 `list`、`load`、`resume`、
  `close`，满足 WebMux 混合会话的全部能力要求。
- 未声明 `promptCapabilities`（即 `image: false`、`embeddedContext: false`），
  WebMux 聊天端发送图片或文件附件时按既有降级逻辑转为文件路径并生成 `warning` 提示。
- 未声明 `_meta.steering.supported`，因此 `agy` 档案 `Steering: false`。
- 未登录或找不到凭据时，会话方法返回 JSON-RPC 错误
  `{"code": -32000, "message": "Authentication required"}`，与 WebMux
  `acp.CodeAuthRequired` 一致。

### 2.2 `session/new`、`session/load`、`session/resume` 与 `configOptions`

`session/new {"cwd": "/path/to/work", "mcpServers": []}` 返回 36 字符小写 UUID v4
格式的 `sessionId` 以及 `configOptions`：

```json
{
  "sessionId": "e0993a23-9d8d-478e-b070-349b8eb0b913",
  "models": {
    "availableModels": [
      { "modelId": "gemini-4-argon", "name": "Gemini 4 Argon", "description": " Gemini 4 Argon" },
      { "modelId": "gemini-3.8-flash", "name": "Gemini 3.8 Flash", "description": " Gemini 3.8 Flash" }
    ],
    "currentModelId": "gemini-4-argon"
  },
  "configOptions": [
    {
      "id": "model",
      "name": "Model",
      "description": "The AI model to use for this session",
      "category": "model",
      "type": "select",
      "currentValue": "gemini-4-argon",
      "options": [
        { "value": "gemini-4-argon", "name": "Gemini 4 Argon", "description": " Gemini 4 Argon" },
        { "value": "gemini-3.8-flash", "name": "Gemini 3.8 Flash", "description": " Gemini 3.8 Flash" }
      ]
    },
    {
      "id": "thought_level",
      "name": "Thinking Level",
      "description": "Controls the depth of reasoning (thinking tokens) used by the model",
      "category": "thought_level",
      "type": "select",
      "currentValue": "medium",
      "options": [
        { "value": "low", "name": "Low", "description": "Less thinking, faster responses" },
        { "value": "medium", "name": "Medium", "description": "Balanced thinking and speed" },
        { "value": "high", "name": "High", "description": "More thinking, deeper reasoning" }
      ]
    }
  ]
}
```

- **配置修改（`session/set_config_option`）**：发送
  `{"sessionId": "<uuid>", "configId": "model", "value": "gemini-3.8-flash"}` 或
  `{"sessionId": "<uuid>", "configId": "thought_level", "value": "low"}` 后，
  响应返回更新后的完整 `configOptions` 数组。
- **跨进程加载的配置重置（`ReapplyOnLoad`）**：在同一 `agy acp` 进程内再次
  `session/load` 会保留修改后的 `model` 与 `thought_level`；但当原 `agy acp` 进程退出、
  由新启动的 `agy acp` 进程执行 `session/load` 或 `session/resume` 时，
  `configOptions` 会重置为默认值（`model: "gemini-4-argon"`、
  `thought_level: "medium"`）。因此 `agy` 适配器返回
  `ReapplyOnLoad() == []string{"model", "thought_level"}`，由 WebMux 在加载或恢复后
  自动调用 `session/set_config_option` 补齐记录中的值。

### 2.3 `session/prompt`、权限请求、工具调用与 `user_message_chunk` 回显

在一次带工具调用的 `session/prompt` 回合中，`agy acp` 按以下顺序发送通知与请求：

1. `session/update`（`config_option_update`、`available_commands_update`）；
2. `session/update`（**`user_message_chunk` 回显**）：
   ```json
   {
     "sessionUpdate": "user_message_chunk",
     "content": { "type": "text", "text": "Run the shell command `echo HELLO_ACP` and reply with DONE." }
   }
   ```
   **关键差异**：`omp` 与 `opencode` 仅在 `session/load` 历史回放时发送
   `user_message_chunk`，而 `agy acp` 在实时 `session/prompt` 回合中也会回显
   `user_message_chunk`（且不带 `messageId`）。
   - 由于 WebMux 的 `actor.send()` 在回合开始（`turn.start`）时已经向时间线写入了带附件元数据的
     `RoleUser` 消息并调用了 `breakMessage()`，若 `actor.onUpdate()` 在 `a.turn != nil`
     期间继续转换 `*acp.UserMessageChunk`，会产生重复的第二条用户消息气泡。
   - 因此 `actor.onUpdate()` 在 `a.turn != nil` 时必须忽略 `*acp.UserMessageChunk`，
     仅在 `a.turn == nil`（`session/load` 回放阶段）转换它。
3. `session/update`（`session_info_update`）：
   `{"sessionUpdate": "session_info_update", "title": "Execute Shell Command Echo"}`。
4. `session/update`（`tool_call`）：
   ```json
   {
     "sessionUpdate": "tool_call",
     "toolCallId": "call_679144",
     "name": "run_command",
     "args": { "CommandLine": "echo HELLO_ACP", "WaitMsBeforeAsync": 5000, "toolSummary": "Run echo HELLO_ACP" },
     "status": "in_progress",
     "kind": "execute",
     "title": "echo HELLO_ACP",
     "rawInput": { "CommandLine": "echo HELLO_ACP", "WaitMsBeforeAsync": 5000, "toolSummary": "Run echo HELLO_ACP" }
   }
   ```
5. `session/request_permission`（未传 `--dangerously-skip-permissions` 时触发）：
   ```json
   {
     "sessionId": "e0993a23-9d8d-478e-b070-349b8eb0b913",
     "toolCall": {
       "sessionUpdate": "tool_call",
       "toolCallId": "call_679144",
       "status": "pending",
       "kind": "execute",
       "title": "echo HELLO_ACP"
     },
     "options": [
       { "optionId": "allow", "name": "Allow", "kind": "allow_once" },
       { "optionId": "deny", "name": "Deny", "kind": "reject_once" }
     ]
   }
   ```
   WebMux 默认的 `PermissionAuto` 自动选择第一个 `allow_once`（`optionId: "allow"`），
   `PermissionAsk` 则在聊天界面展示 Allow / Deny 按钮。若启动配置包含
   `--dangerously-skip-permissions` 并透传给 `agy acp`，`agy acp` 内部也会自动批准所有工具权限。
6. `session/update`（`tool_call_update`）：依次发送 `status: "pending"`、
   `status: "in_progress"` 与 `status: "completed"`，完成时携带标准 `content` 数组：
   ```json
   {
     "sessionUpdate": "tool_call_update",
     "toolCallId": "call_679144",
     "status": "completed",
     "kind": "execute",
     "title": "echo HELLO_ACP",
     "content": [
       {
         "type": "content",
         "content": { "type": "text", "text": "Command exited with code 0.\n\nCommand output:\nHELLO_ACP\r\n" }
       }
     ]
   }
   ```
7. `session/update`（`agent_thought_chunk` / `agent_message_chunk`）：均不带
   `messageId` 字段，由 WebMux `translator` 按连续角色归并并分配 `wm<N>` ID。
8. `session/prompt` 响应：
   `{"stopReason": "end_turn", "usage": {"inputTokens": 13737, "outputTokens": 43, "totalTokens": 13780}}`。

### 2.4 `session/list`、`session/load` 回放与 `session/close`

- **写盘与列出时机**：`session/new` 立即在
  `~/.gemini/antigravity-cli/conversations/<uuid>.db` 创建单会话数据库
  （此时直接调用 `session/load` 可成功返回空历史），但只有完成首轮消息交互后才会写入
  `conversation_summaries.db` 并出现在 `session/list` 结果中；TUI 新建会话同样在首条消息后写入并出现在 `session/list` 中。
- **`session/list` 条目格式**：
  ```json
  {
    "sessionId": "e0993a23-9d8d-478e-b070-349b8eb0b913",
    "cwd": "/tmp/tmpst5s82pa",
    "title": "Execute Shell Command Echo",
    "updatedAt": "2026-11-03T09:33:25Z"
  }
  ```
- **符号链接 `cwd`**：在指向 `/tmp/.../real-proj` 的符号链接 `/tmp/.../link-proj`
  中创建会话并发送消息后，分别以 `link-proj` 与 `real-proj` 调用 `session/list {cwd}`
  均能查到该会话，与 WebMux 统一先执行 `filepath.EvalSymlinks` 的规则完全兼容。
- **`session/load` 历史回放**：依次发送 `user_message_chunk`、`tool_call`
  （直接带 `"status": "completed"` 与 `"content": [...]`）、`agent_message_chunk`，
  随后返回 `LoadSessionResponse`（含 `configOptions`）。
- **`session/resume`**：不回放历史消息，直接返回 `ResumeSessionResponse`（含 `configOptions`），耗时约 15 毫秒。
- **`session/close`**：返回 `{}`，耗时约 2 毫秒。

## 3. 命令行参数与启动文件包装实测

### 3.1 `agy --help` 与 `agy acp --help` 参数对照

- **TUI 与 ACP 共用白名单（保留并透传 ACP）**：
  - 开关：`--dangerously-skip-permissions`、`--sandbox`；
  - 带值：`--agent <v>`、`--add-dir <v>`、`--log-file <v>`；
  - 处理规则：`ResumeArgv` 保留，`ACPArgv` 透传；
  - 实测依据：`agy` 与 `agy acp` 均接受且影响权限、沙箱与工作区边界。
- **仅 TUI 白名单（保留但不透传 ACP）**：
  - 带值：`--model <v>`、`--effort <v>`、`--mode <v>`；
  - 处理规则：`ResumeArgv` 保留，`ACPArgv` 不透传，`--mode` 列入聊天端 `unsupported`；
  - 实测依据：`agy acp` 使用 Cobra 严格校验参数，传入 `--model`/`--effort`/`--mode`
    会报 `unknown flag`；聊天端通过 `configOptions` 控制模型与推理等级。
- **丢弃参数（恢复时省略并提示）**：
  - 参数：位置参数（提示词）、`-c`/`--continue`、`--conversation <v>`、
    `-i`/`--prompt-interactive <v>`、`--settings <v>`、`--remote-control` 及未知参数；
  - 处理规则：`Drop`；
  - 实测依据：会话恢复或首轮提示词参数；`--settings` 丢弃以防覆盖 WebMux 注入的 `title` 钩子。
- **拒绝：不同会话库（`session_store`）**：
  - 参数：`--project <v>`、`--new-project <v>`；
  - 处理规则：`ReasonSessionStore`；
  - 实测依据：切入云端项目会话模式，忽略本地 `--conversation` 会话恢复。
- **拒绝：非交互或子命令（`non_interactive`）**：
  - 参数：`-p`/`--print`/`--prompt`、`--output-format`、`--input-format`、
    `--json-schema`、`--print-timeout`、`--disable-slash-commands`、`-h`/`--help`、
    `-v`/`--version`，以及子命令 `acp`、`agent`、`agents`、`changelog`、`help`、
    `install`、`mcp`、`mic-serve`、`models`、`plugin`、`plugins`、`remote-control`、`update`；
  - 处理规则：`ReasonNonInteractive`；
  - 实测依据：不启动交互式 TUI。

### 3.2 `.bashrc` 包装与 fd 3 重定向

实测启动命令：

```sh
bash -ic 'echo "NOISY_STDOUT_FROM_BASHRC"; exec -- "$@" 1>&3 3>&-' webmux agy acp --dangerously-skip-permissions
```

当 `Stdout` 指向 `stderr` 管道（或丢弃）、`ExtraFiles[0]`（fd 3）作为 ACP JSON-RPC
管道时，`.bashrc` 输出的噪声不会进入 fd 3，`initialize` 与 `session/new` 正常完成。

## 4. TUI 状态信号、`--settings` 会话 ID 注入与中断退出

### 4.1 原生 `OSC 7501` 信号

`agy` TUI 启动时向控制终端发送 `\x1b]7501;?\x07`（等待 100ms 超时）。WebMux 的
`OSCScanner` 立即同步写回应答
`\x1b]7501;?:states=idle,working,done,blocked,error:kinds=permission,question,auth\x1b\\`
后，`agy` TUI 原生发出以下根记录：

- 空闲：`\x1b]7501;state=idle:app=antigravity\x07`
- 回合运行中：`\x1b]7501;state=working:app=antigravity\x07`
- 回合完成：`\x1b]7501;state=done:app=antigravity:msg=<base64>\x07`
- 等待授权/提问/登录：`\x1b]7501;state=blocked:kind=permission|question|auth:app=antigravity:msg=<base64>\x07`
- 错误：`\x1b]7501;state=error:app=antigravity:msg=<base64>\x07`

### 4.2 会话级 `--settings` `title` 钩子上报 `conversation_id`

`agy` TUI 原生根记录不含会话 UUID，但支持 `--settings <json>` 会话级内联设置
（仅当次进程生效，不改写 `~/.gemini/antigravity-cli/settings.json`）。配置 `title`
命令后，`agy` 在每次状态迁移时执行 `sh -c <command>` 并向其 `stdin` 写入单行 JSON：

```json
{
  "agent_state": "working",
  "session_id": "89c9a933-60f6-46ad-977b-a8daf22f8e4d",
  "conversation_id": "89c9a933-60f6-46ad-977b-a8daf22f8e4d",
  "conversation_title": null,
  "cwd": "/tmp/tmpja8gdb7i",
  "product": "antigravity",
  "version": "1.4.12"
}
```

- 新建 TUI 会话在首条消息发送前，`conversation_id` 为 `""`；发送首条消息或通过
  `--conversation <uuid>` 恢复会话后，`conversation_id` 为 36 字符小写 UUID。
- `agy` 档案注入的 `TUIInject` 参数对为 `--settings` 与对应的 `title` 命令 JSON，
  提取 `conversation_id` 并写向 `/dev/tty`。
- 该命令不向 `stdout` 输出任何内容（保持终端标题不变），仅在提取到非空
  `conversation_id` 时向 `/dev/tty` 写入 `\x1b]7501;state=working:id=<uuid>\x07`。
- 将 `internal/session/osc.go` 的 `maxProgramIDSegment` 从 `32` 放宽到 `64` 后，
  `OSCScanner` 将该 36 字符 UUID 记入 `Identity.ProgramChildID`，同时不改变根记录状态。

### 4.3 中断键与 `SIGHUP` 退出

- **中断键**：`agy` TUI 默认键绑定中单次 `Esc`（`\x1b`）映射为
  `Stop stream, close menus, or clear prompt`（而 `Esc Esc` 在空闲时为清空输入框），
  因此 `InterruptKeys` 配置为单次 `[]byte("\x1b")`。
- **`SIGHUP` 退出**：向 `agy` TUI 进程组发送 `SIGHUP` 后，进程在 `0.21–0.52` 秒内刷盘退出，
  退出码为 `0`，之后通过 `agy acp` 执行 `session/load` 可完整回放此前对话。
