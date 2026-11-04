# Tasks

## 1. 后端：档案图标与 `hybrid` 字段

- [x] 1.1 `internal/agentprofile.Profile` 新增 `Icon string`，各适配器声明图标键。
  - omp 为 `pi`，OpenCode 为 `opencode`，Codex CLI 为 `codex`，Claude Code 为 `claudecode`。
  - 验证：`go test ./internal/agentprofile/...` 与 `go test ./internal/archtest` 通过（`hybrid` 与核心包不出现 agent 名称）。
- [x] 1.2 在 `internal/launch` 新增测试，守护档案图标与启动配置图标目录一致。
  - 每个已注册 agent 的 `Title` 与 `Icon` 非空，`Icon` 不是 `terminal` 且被启动配置接受。
  - 每个混合预设识别出的 agent，其 `Icon` 与该预设的 `Icon` 相同。
  - 验证：临时把某个适配器的 `Icon` 改为空串时测试失败，改回后通过。
- [x] 1.3 `session.HybridInfo` 新增 `AgentTitle`、`AgentIcon`，由 `internal/hybrid/actor.go` 从档案填入；验证：`TestCreateStartsEitherSide` 断言两者，`go test -race ./internal/hybrid ./internal/session` 通过。
- [x] 1.4 `contract.SessionHybrid` 新增 `agentTitle`、`agentIcon`，`wire.Hybrid` 拷贝；验证：`internal/wire/wire_test.go` 的期望 JSON 更新后 `go test ./internal/wire` 通过。
- [x] 1.5 重新录制 wire 快照。
  - `wire_snapshot_test.go` 中的混合会话叠加层带上真实的显示名与图标。
  - 验证：`go test ./internal/api -run TestWireSnapshots -update` 的差异只有 `hybrid` 新增的两个字段，不带 `-update` 再跑一次通过。
- [x] 1.6 `docs/protocol/http-sessions.md` 与 `docs/protocol/http-hybrid.md` 说明 `agentTitle`、`agentIcon`；验证：`make docs-check` 通过。

## 2. 前端：契约与图标组件

- [x] 2.1 `web/src/chatSchemas.ts` 的 `SessionHybridSchema` 增加必填 `agentTitle`、`agentIcon`，并补齐夹具。
  - 补齐 `web/test` 与 Storybook 中所有构造 `hybrid` 对象的数据。
  - 验证：`pnpm --dir web check:test` 与 `test/wire-contract.test.ts` 通过。
- [x] 2.2 `LaunchConfigIcon.svelte` 增加 `size` 选项并导出图标键判断函数。
  - `size` 取 `xs`/`sm`/`md`，默认 `md` 保持 `size-5`；判断函数放在 `<script module>`。
  - 验证：启动配置相关浏览器测试通过，`pnpm --dir web check:arch` 通过且基线不增长。

## 3. 前端：会话卡片与聊天页头部

- [x] 3.1 `SessionRow.svelte` 为任意所在端（`tui`、`acp`、`switching`）的混合会话显示 agent 标识。
  - 形如 `[品牌图标 xs] {agentTitle} · {hybridSideLabel}`，图标 `aria-hidden`；图标键未知时只显示文字；普通会话保持原样。
  - 验证：`web/test/browser/SessionRow.test.ts` 覆盖品牌图标替换通用聊天/终端图标、休眠文字、切换中与终端端标识、未知图标键不渲染图标。
- [x] 3.2 `TerminalHeader.svelte` 在聊天视图显示 agent 标识。
  - 条件：`hybrid?.view === "chat"` 且 `t.info?.hybrid` 存在；位置在标题与状态点之间，`shrink-0`。
  - 形式：`<span role="img">`，`aria-label` 与 `title` 为“Agent：{agentTitle}”；内含 `LaunchConfigIcon size="sm"` 与 `max-sm:hidden` 的弱化小号名称；图标键未知时始终显示名称。
  - 验证：浏览器测试覆盖聊天视图显示与可访问名称、终端视图不显示、390px 宽度下名称隐藏而图标可见且长标题被截断。
- [x] 3.3 Storybook 中混合会话卡片或会话页头部的故事补充聊天端 agent 标识；验证：Storybook 套件通过。

## 4. 用户故事

- [x] 4.1 `docs/user-stories/hybrid-sessions.md` 新增 US-Q17“识别聊天端 agent”。
  - 写明首页卡片与聊天页头部的验收要点与源码引用，并在 `docs/user-stories.md`、`docs/user-stories/coverage.md` 的索引及计数中登记。
  - 验证：ID 唯一、源码路径存在，`make docs-check` 与 `make format-check` 通过。

## 5. 集成验证

- [x] 5.1 运行 `make check` 与 `make test-fast`，全部通过。
- [x] 5.2 冒烟并截图。
  - 用 `make build` 后的二进制在独立 HOME、配置与动态端口上启动，fake agent 置于 `PATH` 前；以混合配置“从聊天开始”创建会话。
  - 确认首页卡片显示品牌小图标与“<名称> · 聊天”；聊天页头部在桌面显示图标与名称、在 Pixel 7 视口只显示图标；切到终端后标识消失。
  - 截取桌面/手机、浅色/深色截图供 PR 使用。
