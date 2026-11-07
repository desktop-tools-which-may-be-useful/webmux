## Context

WebMux 用户经常在手机或桌面浏览器中向 CLI agent 发送包含中英混合技术术语的长提示词，或在终端中输入命令。目前浏览器端仅支持键盘打字、粘贴与快捷指令。

在引入语音识别（ASR）能力时，存在以下核心约束：

1. **厂商中立与凭据隔离**：
   - 不同用户习惯的语音识别提供商不同（例如千问 Qwen ASR、微信输入法 WeType、豆包、百度、讯飞、本地 Whisper.cpp 等），且大多数云服务需要用户提供自己的 API Key 或自定义鉴权逻辑；
   - WebMux 作为通用终端与 Agent 工作台，不应在后端内置特定云厂商的协议实现，也不应在 WebMux 数据库或状态文件中托管用户的第三方云 API Key。
2. **音频链路效率**：
   - 主流流式语音识别服务均以 `16000 Hz`、单声道（Mono）、16-bit 有符号小端（`s16le`）PCM 作为基准输入格式（每秒仅 `32 KB`）；
   - 浏览器通过 Web Audio API（`AudioContext({ sampleRate: 16000 })` + `AudioWorklet`）可以直接在前端完成 `16kHz s16le` 采集与重采样，并以 `20ms`（`640 Bytes`）二进制帧经 WebSocket 发送到 Go 后端，无需在 Go 后端引入 `ffmpeg` 或 CGO 音频转码依赖（保持 `CGO_ENABLED=0` 构建）。
3. **架构分层（`internal/archtest` R0–R11）**：
   - Controller 层（`internal/api`）不得直接调用 `os/exec`（R3），且仅能编码/解码 `internal/contract` 中的具名类型（R8）；
   - 子进程管理必须封装在独立的 Service 层包（`internal/voice`）中，由 `cmd/internal/assemble` 构造并注入 `api.Deps`。
4. **双模式 UI 与设计系统（`webmux-ui`）**：
   - 语音输入需要同时在聊天模式（输入框旁）和终端模式（快捷栏内）可用；
   - 交互遵循点击切换（Toggle）的四阶段状态机：点击进入加载 $\rightarrow$ 连接成功显示实时音量 $\rightarrow$ 再次点击进入完成加载 $\rightarrow$ 写入文本且不自动回车；
   - 视觉实现严格对齐同排按钮尺寸与线条优先的激活态规范。

## Goals / Non-Goals

**Goals:**

- 提供极简的 `[voice]` 配置（仅含 `cmd` 与 `env`），通过 `STDIN / STDOUT` 管道协议驱动任意外部语音识别可执行文件（如 `node /path/to/script.js`）。
- 在文档中清晰定义 `STDIN`（`16kHz / 1ch / s16le` 裸 PCM 流 + `EOF` 结束信号）与 `STDOUT`（NDJSON 事件流 + 纯文本输出兼容）协议细节，使用户可以用任何语言对接自己的 ASR 服务。
- 实现 `/ws/voice` 认证 WebSocket 通道与 `internal/voice` 子进程生命周期管理，确保连接断开或识别结束时干净回收子进程。
- 在聊天模式（`ChatComposer`、`QuickStart`）与终端模式（`SoftKeys`、`TerminalHeader`、`TerminalInputComposer`）提供统一的四阶段点击录音组件 `VoiceButton.svelte`，实时渲染麦克风音量，并在识别完成后将结果安全写入输入框或终端（不自动发送回车）。

**Non-Goals:**

- 不在 WebMux 仓库中内置任何特定云厂商的 ASR 脚本或 SDK（用户自定义的测试脚本保存在仓库外，通过 `voice.cmd` 的外部路径引用）。
- 不提供网页端的第三方 API Key 管理界面；API Key 的传入方式完全由用户的可执行文件决定（通过 `voice.cmd` 参数或 `voice.env` 环境变量传入）。
- 不支持语音唤醒（Wake Word）或自动回车执行（识别结果始终留给用户确认后再发送或回车）。

## Decisions

### D1 采用 `STDIN / STDOUT` 一次性子进程模型，而非 HTTP Webhook 或常驻 Daemon

- **决策**：每次用户在浏览器点击开始录音时，`internal/voice` 按 `voice.cmd` 启动一个独立的外部子进程；录音期间将收到的 `16kHz s16le` 二进制流写入子进程 `STDIN`；用户再次点击结束录音时关闭 `STDIN`（发送 `EOF`），等待子进程输出 `final` 结果并退出。
- **原因与对比**：
  - 相比要求用户额外部署一个常驻 HTTP/gRPC 微服务，`STDIN / STDOUT` 子进程零端口占用、零服务编排负担，用户只需写一个单文件脚本（例如 `node /path/to/qwen.js`）即可即插即用。
  - 每次录音对应一次进程生命周期，天然隔离不同录音会话的状态，即使外部脚本异常崩溃也不会影响下一次录音或主服务稳定性。

### D2 `STDOUT` 同时支持 NDJSON 流式事件与纯文本回退

- **决策**：
  - 标准模式：子进程向 `STDOUT` 按行输出 UTF-8 JSON 对象（`{"type":"ready"}`、`{"type":"partial","text":"..."}`、`{"type":"final","text":"..."}`、`{"type":"error","error":"..."}`）。
  - 兼容模式：若子进程启动后 150ms 内未输出 `{"type":"ready"}` 且进程仍在运行，`internal/voice` 自动向前端发出一次 `ready` 事件；若子进程输出非 JSON 的普通文本行且最终以退出码 `0` 退出，`internal/voice` 将普通文本合并作为 `final` 事件发回前端。
- **原因**：
  - 对于连接云端流式 WebSocket ASR 的脚本（如 Node.js 脚本），显式输出 `{"type":"ready"}` 可以让前端按钮准确停留在“加载中（`connecting`）”直到云端握手完成，随后再进入“录音中（`recording`）”并展示音量；
  - 对于使用本地命令行工具（如 `whisper-cli`）或极简脚本的用户，无需编写 JSON 序列化代码，只需从 `stdin` 读完音频后 `print(text)` 即可工作。

### D3 浏览器端统一输出 `16kHz Mono s16le` 裸 PCM，不在后端做音频转码或压缩

- **决策**：
  - 浏览器使用 `AudioContext({ sampleRate: 16000 })` 与 `AudioWorkletProcessor`（在不支持上下文级 `16kHz` 的浏览器上由 Worklet 内联完成降采样与单声道混合）；
  - 每 `20ms`（320 个采样点 = `640 Bytes`）打包为一帧 `Int16LE` 二进制消息发送给 `/ws/voice`，同时在前端计算该帧的均方根（RMS）音量以驱动 UI 音量指示。
- **原因**：
  - `16kHz Mono s16le` 码率仅 `256 kbps`（`32 KB/s`），在局域网或远程连接上带宽开销极小，直接传裸 PCM 避免了浏览器端 WASM Opus 编码与后端解码的延迟和依赖；
  - 外部子进程从 `STDIN` 拿到的直接就是所有主流 ASR 接口通用的标准 `16kHz s16le` 字节流。

### D4 双模式挂载位置与“不自动回车”写入语义

- **决策**：
  1. **聊天模式**：在 `ChatComposer.svelte`（会话内聊天）和 `QuickStart.svelte`（首页快速开始）的输入框底部操作栏右侧（快捷指令菜单与发送按钮旁）放置 `VoiceButton`（`size="icon-touch"`）。识别完成后，将文本插入当前光标/选区位置并聚焦输入框，**不调用 `submit()` / `send()`**。
  2. **终端模式**：
     - 在移动端或开启底部软键栏时，放置于 `SoftKeys.svelte` 的 `trailing` 区域（与草稿开关、快捷指令并列）；
     - 在桌面端未显示底部软键栏时，放置于 `TerminalHeader.svelte` 的 `.quick-commands-header` 容器内（`size="icon-sm"`，与 `quickMenu` 共享相同的 CSS 显隐规则，确保软键栏隐藏时顶栏可见、软键栏显示时顶栏不重复显示）；
     - 在开启终端草稿模式时，`TerminalInputComposer.svelte` 的操作栏同样提供入口。
     - 识别完成后统一调用 `SessionTerminal.svelte` 的 `insertText(text)`：未开启草稿模式时走 `t.paste(text)` 写入终端（支持 bracketed paste，且**绝不追加 `\r`**）；开启草稿模式时插入草稿框光标处。

## Risks / Trade-offs

- **[子进程卡死或未响应 EOF]** → 在 `internal/voice` 中为 `StopInput()` 后的等待阶段设置超时（15 秒），超时或 WebSocket 断开时立即取消子进程 `context` 并强制回收进程。
- **[外部脚本在远程握手完成前就收到音频帧]** → 外部脚本若需要远程 WebSocket 握手，可在握手期间先将 `process.stdin` 的数据缓存在内存数组，握手成功后再输出 `{"type":"ready"}` 并冲刷缓冲，确保用户开口瞬间不吞字。
- **[非安全上下文（HTTP 非 localhost）下浏览器禁用 `navigator.mediaDevices.getUserMedia`]** → 前端捕获 `navigator.mediaDevices` 缺失或权限拒绝错误，立即重置按钮状态并通过现有的错误提示区域告知用户需要 HTTPS / localhost 或授予麦克风权限。
