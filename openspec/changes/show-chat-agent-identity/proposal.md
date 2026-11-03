# Proposal

## Why

混合会话进入聊天端后，页面只剩通用的“聊天”字样和对话内容，用户无法一眼看出当前对话由哪个 agent（omp、OpenCode、Codex CLI、Claude Code）承接。同时开着多个聊天会话时，首页列表和聊天页都缺少这条最基本的身份信息，用户只能靠标题猜测或切到终端确认。

## What Changes

- 混合会话的会话信息 `hybrid` 对象新增 `agentTitle`（agent 显示名，如 `OpenCode`）与 `agentIcon`（图标键，取值来自现有启动配置图标目录，如 omp 为 `pi`）。两者由服务端按 agent 档案给出，前端不内置任何 agent 名称。
- agent 档案新增图标键属性；每个已注册 agent 声明自己的图标。
- 首页会话列表：处于聊天端（含休眠、切换中）的混合会话卡片，在“所在端”信息处用 agent 小图标 + 显示名替换通用聊天图标，形如 `[图标] OpenCode · 聊天 · 2 个设备已连接 · 刚刚`。终端端卡片保持不变。
- 聊天页头部：在会话标题之后、状态点之前显示一个低强调的 agent 标识（小图标 + 显示名）。窄屏只显示图标，保留可访问名称与悬停提示；标题空间不足时先截断标题，agent 标识保持完整。
- 只有显示，没有交互；不改变切换、关注、推送和预览逻辑。

## Capabilities

### New Capabilities

（无）

### Modified Capabilities

- `hybrid-sessions`：会话信息中的 `hybrid` 对象新增 `agentTitle` 与 `agentIcon` 字段。
- `chat-view`：新增会话列表卡片与聊天页头部的 agent 标识展示要求。

## Impact

- 后端：
  - `internal/agentprofile`：`Profile` 新增图标键，各适配器子包声明取值；`internal/launch` 的测试校验图标键属于启动配置图标目录。
  - `internal/session.HybridInfo` 与 `internal/hybrid/actor.go`：填充显示名与图标。
  - `internal/contract/hybrid.go`、`internal/wire/wire.go`：新增字段；重新录制 wire 快照 `internal/api/testdata/wire`。
- 前端：`web/src/chatSchemas.ts`（`SessionHybridSchema`）、`web/src/components/SessionRow.svelte`、`web/src/components/TerminalHeader.svelte`、`web/src/components/LaunchConfigIcon.svelte`（新增尺寸选项以复用品牌图标）。
- 文档：`docs/protocol/http-sessions.md`、`docs/protocol/http-hybrid.md` 的 `hybrid` 字段说明；`docs/user-stories/hybrid-sessions.md` 与两份索引。
- 兼容性：只新增字段，旧客户端忽略即可；`internal/archtest` 禁止 `web/src` 出现 agent 名称的约束保持不变。
