## Why

混合模式（TUI ↔ ACP 聊天视图）目前只内置 OpenCode 与 omp 两个档案，预置的 “Codex CLI” 启动配置只能当作普通终端会话使用。
Codex CLI 没有 `codex acp` 子命令，ACP 只能经第三方适配器 `codex-acp` 获得（#208）。
适配器需要用户另外安装；若不在启动 ACP 进程之前检查，缺少适配器时用户只会看到笼统的“启动 agent 失败”，不知道该装什么。

本变更按 #208 各决策项的建议方案（C1a、C2a、C3a、C4a+c、C5a、C6a、C7a、C8a、C9b、C10a、C11a、C12a、C13 双版本）
让 Codex CLI 默认支持混合模式，并为“第三方 ACP 连接器”建立统一的安装检查与界面提示。

## What Changes

- 新增 Codex 档案（`codex`，标题 “Codex CLI”）：参数分类表、`resume <id>` 子命令式恢复参数、TUI 强制注入 `--no-daemon`、
  一次 Esc 中断、支持 `session/resume` 与插话、加载后重新应用 `model`/`reasoning_effort`/`mode`。
- 档案数据模型扩展：ACP 启动命令不再固定为 `<可执行文件> acp`，改由档案声明（内置子命令或第三方连接器）；
  档案可声明 ACP 进程额外注入的环境变量（Codex 为 `CODEX_PATH`、`NO_BROWSER=1`）。
- **第三方 ACP 连接器的安装检查（新能力）**：档案声明的连接器在每次启动 ACP 进程之前（资格探测、从聊天开始、
  切到聊天、恢复到聊天、唤醒）按与实际启动相同的写法检查是否已安装；未安装时 SHALL NOT 启动 ACP 进程，
  以稳定原因码 `connector_missing` 返回，界面显示连接器名称、安装命令与说明；终端端照常使用。
- 启动配置的 `hybridSupport` 增加只读的 `connector` 信息（来自档案，不探测 PATH），设置页与新建会话弹层据此提前提醒“聊天端需另外安装”。
- 资格判定新增原因 `connector_missing`；Codex 的 `thread_active_writer` 错误映射为 `occupied`（被外部 Codex 客户端占用）。
- 会话 ID 来源：`session` 暴露与 agent 无关的“PTY 根进程及其后代进程打开的文件路径”；
  `agentprofile` 把 `$CODEX_HOME/thread-writer-locks/<id>.lock` 解释为 Codex 会话 ID；读不到时退回 `session/list` 候选判定。
- 根进程判定允许 npm 安装的 `node` 启动器，前提是其直接子进程是档案对应的原生可执行文件。
- 首次 TUI → 聊天时，从 argv 推导 `mode`、`model`、`reasoning_effort` 并写入记录（播种），之后每次加载重新应用。
- 插话对 Codex 带上 `_meta.steering.idleBehavior = "promptRequired"`，空闲时不产生 webmux 不知道的回合。
- 新安装预置的 Codex CLI 启动配置改为 `hybrid: true`；已有 `launch-configs.json` 不改写。
- 版本提示对第三方连接器记录两个已验证版本（Codex 0.165.0、codex-acp 2.3.0）。

## Capabilities

### New Capabilities

- `acp-connector-availability`：第三方 ACP 连接器的声明、启动前安装检查、`connector_missing` 原因码，以及设置页、新建会话、会话页与聊天视图中的安装提示。

### Modified Capabilities

- `hybrid-agent-profiles`：内置档案增至 3 个；ACP 启动命令与额外环境由档案声明；新增 Codex 的参数、恢复命令、`--no-daemon` 与版本提示。
- `agent-session-identity`：根进程判定允许启动器；新增后代进程打开文件这一身份来源；资格条件新增连接器已安装与外部写者占用。
- `hybrid-sessions`：从聊天开始、切到聊天时的连接器检查失败结果；首次加载前从 argv 播种配置。
- `chat-prompt-queue`：插话在 agent 支持时声明空闲行为，空闲时退回队列。
- `launch-configs`：Codex CLI 预置配置默认混合；`hybridSupport` 增加 `connector`；设置页显示连接器提示。

## Impact

- Go：`internal/agentprofile`（档案模型、Codex 表、连接器声明、argv → 配置映射）、`internal/session`（后代进程与打开文件，Linux `/proc`、darwin `proc_pidinfo`）、
  `internal/hybrid`（连接器检查、资格原因、播种、插话 outcome、`thread_active_writer`）、`internal/launch`（预置配置、`hybridSupport.connector`）、
  `internal/contract`/`internal/wire`/`internal/api`（`connector_missing` 冲突体与状态映射）。
- Web：`web/src/schemas.ts`、`web/src/hybrid.ts`、新建会话弹层、会话页聊天按钮、聊天视图通知、设置页启动配置编辑器。
- 测试：fakeagent 增加 Codex 档案与可缺失的连接器、wire 快照、E2E 运行时多复制 `codex` 与 `codex-acp`。
- 部署：使用 Codex 混合模式需在服务 PATH 中提供 `codex-acp`（npm 安装时还需 Node）；webmux 不自动安装。
