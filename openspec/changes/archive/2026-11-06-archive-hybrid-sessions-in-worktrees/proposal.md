# Proposal

## Why

首页底部的“可恢复”分组把所有已关闭、已中断的混合会话平铺在一个列表里，记录一多（例如 18 条），用户必须逐条比对目录才能找到某个工作树的那段对话；而用户本来就是按工作树思考工作的。同时“关闭”一个混合会话实际上并不丢失对话，名称却暗示结束，关闭后页面还停留在已经失效的会话页上。

## What Changes

- 把“可恢复”更名为“已归档”：已关闭与已中断的混合会话记录统称已归档记录，状态徽标仍区分“已关闭”“已中断”。
- 已归档记录不再集中在首页底部，而是显示在其所属工作树分组内、活动会话下方的“已归档”折叠区；无仓库关联的记录进入“直接会话”对应目录分组，原工作树已不在 Git 清单中的进入该仓库的“已不可用工作区”。底部全局分组移除。**BREAKING**（UI 位置变化）
- 混合会话记录新增工作区关联快照（仓库 ID、工作树 ID），`GET /api/hybrid-sessions` 摘要返回它；没有快照的旧记录按 `cwd` 与已知工作树路径匹配归位。
- 拥有已归档记录的工作树与拥有活动会话的工作树一样保留在首页，不被“选择主页工作树”隐藏。
- 混合会话的“关闭会话”操作更名为“停止会话并归档”（确认按钮“停止并归档”），确认文案说明 agent 进程停止、对话保留、可在所属工作树恢复；归档成功后会话页（单窗格时）返回当前模式的首页，不自动展开或高亮记录（#303 移除了最初的定位与高亮，#329 改为现名）。分屏时只移除该窗格。
- 首页搜索词同时匹配已归档记录的标题、agent 与目录；状态筛选新增“已归档”，选择其他状态时隐藏已归档记录。
- “移除”已归档记录改为“删除记录”，放进记录的更多菜单并保留确认，避免与“恢复”并列造成误触；记录位于可删除的链接 worktree 时，改为打开删除 worktree 的确认流程，把 worktree 与记录一起删除（#329）。
- 非混合（普通 PTY）会话的“关闭会话 / 移除会话”语义保持不变：它们无法恢复，不进入已归档。

## Capabilities

### New Capabilities

（无）

### Modified Capabilities

- `workspace-overview`：“可恢复分组”需求替换为工作树内的“已归档”区；首页层级、主页工作树保留规则与筛选覆盖已归档记录；三种移除操作的命名加入“归档会话”。
- `hybrid-session-recovery`：记录内容增加工作区关联快照；列表摘要返回该快照；首页展示与移除场景改为“已归档”术语与位置。
- `hybrid-sessions`：关闭混合会话在 UI 上称为“归档”，完成后会话页返回首页并定位到所属工作树。

## Impact

- 后端：`internal/hybrid` 记录结构与更新路径、`internal/contract` 的 `HybridRecord`、`internal/wire` 转换、`internal/api/testdata/wire` 快照；`hybrid-sessions.json` 增加可选字段（向后兼容读取旧文件）。
- 前端：
  - 契约：`web/src/schemas.ts`。
  - 首页：`RecoverableSessions.svelte`（改为工作树内的已归档区组件）、`WorkspaceGroup.svelte`、`RepositoryGroup.svelte`、`DirectSessions.svelte`、`SessionList.svelte`、`SessionFilters`。
  - 布局：`home-worktrees.ts`（`homeLayout` 计入已归档记录）。
  - 归档入口：`CloseSessionDialog.svelte`、`TerminalHeader.svelte`、`SessionRow.svelte`、`Terminal.svelte`。
- 测试：Go race（hybrid、api wire 快照）、Vitest 组件/布局测试、`web/test/e2e/hybrid.spec.ts`。
- 文档与用户故事：US-C02、US-D10、US-Q12 及 `docs/hybrid-sessions.md` 等提到“可恢复”分组的说明。
