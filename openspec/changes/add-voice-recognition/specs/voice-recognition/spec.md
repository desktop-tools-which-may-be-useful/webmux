## ADDED Requirements

### Requirement: 语音识别外部命令配置与能力公布

系统 SHALL 支持在配置文件的 `[voice]` 段配置可选的外部语音识别启动命令与环境变量：

- `cmd`：字符串数组，首项为可执行文件路径或名称，其余项为命令行参数，按 `argv` 直接执行且 SHALL NOT 经 shell 展开；
- `env`：`KEY=VALUE` 格式的环境变量字符串数组。

当 `cmd` 非空时：

1. `cmd[0]` 去掉首尾空白后 SHALL NOT 为空，否则配置校验失败；
2. `env` 中的每一项 SHALL 包含 `=`，变量名 SHALL 符合合法的环境变量名格式且 SHALL NOT 使用系统保留环境变量名，否则配置校验失败。

`GET /api/config` 响应 SHALL 包含布尔字段 `voice`：

- 当服务端配置了非空 `voice.cmd` 时，`voice` SHALL 为 `true`；
- 当未配置 `voice.cmd` 时，`voice` SHALL 为 `false`；
- `GET /api/config` SHALL NOT 暴露 `voice.cmd` 的具体命令参数或 `voice.env` 中的环境变量值。

#### Scenario: 已配置语音识别命令时公布能力但不泄露凭据

- **WHEN** `config.toml` 配置了 `[voice]` 且 `cmd = ["node", "/path/to/asr.js"]`、`env = ["ASR_API_KEY=secret"]`
- **THEN** 配置校验通过，`GET /api/config` 返回 `"voice": true`，且响应体中不包含 `/path/to/asr.js` 或 `ASR_API_KEY`

#### Scenario: 未配置语音识别命令时默认关闭

- **WHEN** `config.toml` 未包含 `[voice]` 段或 `cmd` 为空数组
- **THEN** `GET /api/config` 返回 `"voice": false`

#### Scenario: 非法环境变量名或空可执行文件被拒绝

- **WHEN** `config.toml` 中 `[voice]` 的 `cmd = [""]` 或 `env = ["INVALID-NAME=1"]`
- **THEN** 配置校验返回明确错误并拒绝启动

### Requirement: 基于 STDIN 与 STDOUT 的外部语音识别子进程协议

当客户端发起语音识别会话时，服务端 SHALL 按配置的 `voice.cmd` 和 `voice.env` 启动独立的外部子进程，并通过标准输入（`STDIN`）和标准输出（`STDOUT`）与其通信：

1. **音频输入流（`STDIN`）**：
   - 服务端 SHALL 将收到的 `16000 Hz`、单声道（1 channel）、16-bit 有符号小端（`s16le`）原始 PCM 字节流按顺序写入子进程 `STDIN`；
   - 当客户端发送停止录音信号时，服务端 SHALL 关闭子进程 `STDIN`（发送 `EOF`），通知子进程完成剩余音频识别。
2. **识别事件流（`STDOUT`）**：
   - 服务端 SHALL 按行读取子进程 `STDOUT` 的 UTF-8 文本；
   - 若行内容为 JSON 对象，服务端 SHALL 解析 `type` 字段并转发对应事件：
     - `{"type": "ready"}`：外部子进程已就绪；
     - `{"type": "partial", "text": "..."}`：流式中间识别文本；
     - `{"type": "final", "text": "..."}`：最终识别文本；
     - `{"type": "error", "error": "..."}`：外部子进程报告的错误；
   - 若子进程未主动输出 `{"type": "ready"}`，服务端在子进程成功启动且管道就绪后 SHALL 向客户端通知就绪，使简单脚本无需显式握手即可接收音频；
   - 若子进程输出非 JSON 的普通文本行且以退出码 `0` 正常结束，服务端 SHALL 将该普通文本汇总作为 `final` 识别结果返回；
   - 若子进程非零退出且未输出 `error` 事件，服务端 SHALL 从 `STDERR` 提取错误摘要并返回 `error` 事件。
3. **进程生命周期与清理**：
   - 当 WebSocket 连接断开、取消或识别完成时，服务端 SHALL 终止并回收对应的外部子进程，避免残留僵尸进程。

#### Scenario: NDJSON 流式识别并在关闭 STDIN 后返回最终文本

- **WHEN** 子进程启动后输出 `{"type":"ready"}`，服务端写入多帧 `s16le` PCM 字节并关闭 `STDIN`，子进程随后输出 `{"type":"partial","text":"测试"}` 与 `{"type":"final","text":"测试语音输入"}` 并以退出码 `0` 退出
- **THEN** 服务端依次向客户端推送 `ready`、`partial` 和 `final`（文本为 `"测试语音输入"`）事件，并清理子进程

#### Scenario: 纯文本脚本在 EOF 后直接打印结果

- **WHEN** 子进程在读取 `STDIN` 直至 `EOF` 后未输出 JSON，而是向 `STDOUT` 打印 `"纯文本识别结果"` 并以退出码 `0` 退出
- **THEN** 服务端将该输出封装为 `{"type":"final","text":"纯文本识别结果"}` 推送给客户端

#### Scenario: 客户端断开连接时立即终止子进程

- **WHEN** 录音过程中客户端主动关闭 `/ws/voice` WebSocket 连接
- **THEN** 服务端立即取消子进程上下文并回收该子进程资源

### Requirement: 语音识别 WebSocket 传输通道

服务端 SHALL 提供 `GET /ws/voice` WebSocket 端点，并强制执行与其他 `/ws/*` 端点一致的 Token 认证与 Origin 检查：

- 当服务端未启用语音识别（`voice == false`）时，请求 `GET /ws/voice` SHALL 返回错误状态码；
- 连接建立后，服务端 SHALL 接收客户端发送的 WebSocket Binary Frame 作为原始 PCM 音频块写入子进程 `STDIN`；
- 服务端 SHALL 接收客户端发送的 JSON Text Frame `{"type": "stop"}` 并触发关闭子进程 `STDIN`；
- 服务端 SHALL 将子进程的 `ready`、`partial`、`final` 与 `error` 事件编码为 JSON Text Frame 发送给客户端。

#### Scenario: 未认证或跨源请求被拒绝

- **WHEN** 客户端在启用 Token 的服务上未携带有效认证子协议或携带未授权的 `Origin` 请求 `GET /ws/voice`
- **THEN** 服务端分别返回 HTTP `401` 或 `403` 并拒绝升级 WebSocket

### Requirement: 聊天模式与终端模式下的点击录音交互与文本写入

当 `GET /api/config` 返回 `voice: true` 时，前端 SHALL 在聊天模式与终端模式提供语音输入按钮：

- **聊天模式位置**：位于会话聊天输入框（`ChatComposer`）与“新聊天”快速开始输入框（`QuickStart`）的操作栏旁；
- **终端模式位置**：位于终端快捷栏（底部软键栏 `SoftKeys`、桌面端隐藏软键栏时的顶栏快捷区 `TerminalHeader`，以及终端草稿输入框 `TerminalInputComposer`）内；
- 当 `voice: false` 时，前端 SHALL NOT 渲染上述语音输入按钮。

语音输入按钮 SHALL 采用点击切换（Toggle）的四阶段状态机：

1. **连接加载态（`connecting`）**：用户首次点击空闲态按钮时，按钮进入加载态，请求麦克风权限、初始化 `16000 Hz` 单声道 `s16le` 音频采集并连接 `/ws/voice`；在连接完成前再次点击 SHALL 取消本次连接并回到空闲态；
2. **录音与实时音量显示态（`recording`）**：收到服务端 `ready` 事件后，按钮进入录音激活态，持续将麦克风音频帧发送至 `/ws/voice`，并在按钮上实时显示当前麦克风音量大小；
3. **识别完成加载态（`finishing`）**：用户在录音态再次点击按钮时，前端立即停止麦克风采集并向服务端发送 `{"type": "stop"}`，按钮进入等待最终识别结果的加载态；
4. **文本写入与不自动回车**：收到服务端 `{"type": "final", "text": "..."}` 后，按钮恢复为空闲态；若识别文本非空，前端 SHALL 将文本写入当前输入目标，且 SHALL NOT 自动追加回车（`\r` 或 `\n`）或自动提交消息：
   - 在聊天模式中，文本插入聊天输入框当前光标或选区位置，不触发发送；
   - 在终端模式未开启草稿时，文本通过终端粘贴通道写入当前终端，不追加 `\r`；
   - 在终端模式已开启草稿时，文本插入终端草稿输入框当前光标或选区位置，不触发提交。

#### Scenario: 聊天模式下点击录音并将结果插入输入框且不发送

- **WHEN** 用户在聊天输入框旁点击语音按钮，经历加载态后进入录音态并看到实时音量变化，再次点击语音按钮进入完成加载态，随后服务端返回 `{"type":"final","text":"帮我重构这个函数"}`
- **THEN** `"帮我重构这个函数"` 被插入到聊天输入框中，按钮恢复空闲态，且消息不会自动发送

#### Scenario: 终端模式下点击快捷栏录音并将结果写入终端且不回车

- **WHEN** 用户在终端快捷栏点击语音按钮完成一次录音，服务端返回 `{"type":"final","text":"git status -sb"}`
- **THEN** `"git status -sb"` 被粘贴到终端当前输入行（或开启草稿模式时的草稿框）中，末尾不附加 `\r`，命令不会自动执行
