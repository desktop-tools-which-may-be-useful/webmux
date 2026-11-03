## Why

混合模式已支持 OpenCode、omp、Codex CLI、Claude Code、Qoder 与 QoderCN 六个档案，
但预置的 Qoder 与 QoderCN 启动配置只能作为普通终端会话使用。

Qoder（国际版，`qoder`）与 QoderCN（国内版，`qodercn`/`qoderclicn`）
均为支持 ACP v1 的 CLI agent：`qoder --acp` / `qodercn --acp`
在 stdin/stdout 上运行 JSON-RPC，具备以下能力：

- `session/new`、`session/load`、`session/resume`、`session/list`
- `session/close`、`session/fork`、`session/delete`
- 配置项、附件、权限、插话（`_meta.qoder.promptQueueing`）

两者 CLI 参数与 Claude Code 高度一致：
`--resume <uuid>`、`--continue`、`--model`、`--permission-mode`、
`--dangerously-skip-permissions` 等。
会话以 UUID 存储在 `~/.qoder/projects/` 或 `~/.qoder-cn/projects/` 下。

本变更让 Qoder 与 QoderCN 在同一 WebMux 会话中
双向切换原生 TUI 与 ACP 聊天视图，
复用现有混合会话、聊天队列与恢复能力。

## What Changes

- 档案数据模型扩展：新增 `ACPArg` 字段，
  允许档案声明 ACP 启动参数（默认 `"acp"` 子命令；
  Qoder/QoderCN 为 `"--acp"` 标志）。
  `ACPArgv()` 使用该字段替代硬编码的 `"acp"`。
- 新增 Qoder 档案（`qoder`，标题 "Qoder"）：
  识别 `qoder` 可执行文件，内置 ACP（`qoder --acp`），
  `--resume <uuid>` 恢复，参数分类表与 Claude Code 对齐，
  一次 Esc 中断，支持 `session/resume`、配置重应用与插话。
- 新增 QoderCN 档案（`qodercn`，标题 "QoderCN"）：
  识别 `qodercn` 与 `qoderclicn` 可执行文件，
  其余与 Qoder 档案相同，但 ACP agent 名为 `qoder-cli-cn`，
  配置目录为 `~/.qoder-cn/`。
- 会话 ID 来源：两者均不使用 OSC 7501 终端信号传递会话 ID；
  通过 `session/list` 候选匹配（按 cwd）发现当前会话。
  当候选唯一时自动选择，多个时提示用户。
- 从 argv 推导首次聊天端的 `model`、`effort` 与 `mode`
  （`--model`、`--reasoning-effort`、`--permission-mode`、
  `--dangerously-skip-permissions`）；
  冷 load/resume 后重应用 model/effort。
- 隐藏聊天菜单中的 `login` 命令。
- 新安装的 Qoder 与 QoderCN 预置配置改为 `hybrid: true`，
  默认带 `--dangerously-skip-permissions`；
  已有 `launch-configs.json` 不改写。
- 新增 `qoder` 图标：服务端白名单、前端下拉、Svelte 组件
  与 SVG 资源四处同步注册。QoderCN 复用同一图标。

## Capabilities

### New Capabilities

无。本次复用现有内置 ACP、混合会话、聊天队列与恢复能力。

### Modified Capabilities

- `hybrid-agent-profiles`：内置档案增至 6 个
  （新增 Qoder、QoderCN）；档案数据模型新增 `ACPArg` 字段
  以支持 `--acp` 标志式 ACP 启动；
  声明两者的参数、恢复、配置推导、版本与聊天能力。
- `agent-session-identity`：新增 `session/list` 候选匹配
  作为无 OSC 7501 会话 ID 档案的通用发现机制。
- `launch-configs`：新安装的 Qoder 与 QoderCN 预置改为默认混合，
  已有配置保持不变；新增 `qoder` 图标。

## Impact

- Go：
  - 新增 `internal/agentprofile/qoder/qoder.go`
  - 新增 `internal/agentprofile/qodercn/qodercn.go`
  - `internal/agentprofile/profile.go` 新增 `ACPArg` 字段
  - `internal/agentprofile/command.go` 的 `ACPArgv()` 使用 `ACPArg`
  - `internal/agentprofile/agents/agents.go` 注册两个新档案
  - `internal/launch/presets.go` 更新预置配置
  - `internal/launch/types.go` 图标白名单增加 `qoder`
- Web：
  - `web/src/launchConfigs.ts` 增加 `qoder` 图标选项与自动检测
  - `web/src/components/LaunchConfigIcon.svelte` 导入并注册图标组件
  - `web/src/icons/` 新增 Qoder SVG 图标
- 测试：扩展 agentprofile 契约测试、hybrid 切换/恢复测试、
  fakeagent 增加 Qoder/QoderCN 档案、wire 快照、E2E 启动配置。
- 部署：聊天端使用内置 `--acp`，无需额外安装连接器；
  TUI 端不依赖连接器。
- 用户故事与文档：更新混合会话相关用户故事、
  `docs/cli-agents.md` 与 `docs/hybrid-sessions.md`。
