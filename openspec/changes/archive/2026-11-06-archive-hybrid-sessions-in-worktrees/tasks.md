# Tasks

## 1. 记录的工作区关联快照（后端）

- [x] 1.1 在 `internal/hybrid/records.go` 的 `Record` 增加可选 `Workspace`（`workspace,omitempty`，含 `clone` 深拷贝），并验证 `go test -race ./internal/hybrid` 中新增用例：旧格式文件（无该字段）正常加载、写回后字段省略
- [x] 1.2 在 `internal/hybrid/actor.go` 与 `saveShellTitle` 同一路径上保存 `shell.Info().Workspace` 的变化（关联出现、改变、清空；关闭与中断时保留最后值），并用 race 测试验证：从工作树启动后记录带快照，服务重启转为 `interrupted` 后快照仍在，关联被解除后清空
- [x] 1.3 在 `internal/contract.HybridRecord` 增加 `Workspace *WorkspaceReference`（`omitempty`），在 `internal/wire` 转换，执行 `go test ./internal/api -run TestWireSnapshots -update` 重录 `hybrid-records.json`，并验证 `internal/wire/testdata` 与 `go test -race ./internal/api ./internal/wire` 通过
- [x] 1.4 运行持久化守卫（`make check` 中的 persistence guard）；若要求登记新字段，更新 `docs/persistence.md` 并验证守卫通过

## 2. 前端契约与归位逻辑

- [x] 2.1 在 `web/src/schemas.ts` 的混合记录 schema 增加可选 `workspace`（复用会话的工作区 schema），并验证契约测试与 `pnpm --dir web check` 通过
- [x] 2.2 新增归位纯函数（D2：快照 → 规范化路径最长前缀 → 直接会话目录分组），用 Vitest 覆盖快照命中、工作树已不在清单（进“已不可用工作区”）、仓库已移除、旧记录子目录匹配、无匹配、每条记录只出现一次
- [x] 2.3 `home-worktrees.ts` 的 `homeLayout` 接收有已归档记录的工作树 ID 并视同有会话，迁移 `RepositoryGroup` 调用；Vitest 验证未置顶但有已归档记录的工作树显示、记录清空后隐藏

## 3. 工作树内的“已归档”区（UI）

- [x] 3.1 把 `RecoverableSessions.svelte` 改造为 `ArchivedSessions.svelte`（D3）：默认折叠、标题“已归档 N”与“N 条已中断”徽标、按更新时间倒序、默认 5 条与“显示全部 N 条”、行内“恢复到聊天/恢复到终端”、更多菜单“删除记录”+确认；组件测试覆盖这些行为与键盘操作，并通过 `pnpm --dir web check:arch`
- [x] 3.2 在 `WorkspaceGroup`（工作树与“已不可用工作区”）和 `DirectSessions` 目录分组中渲染该区，移除 `SessionList.svelte` 底部的全局分组；只有已归档记录时不显示“暂无会话”；已归档记录不计入运行/等待/退出数量；浏览器测试验证 18 条分散记录各自出现在所属分组
- [x] 3.3 `SessionFilters` 增加“已归档”状态选项与计数，搜索匹配已归档记录的标题/名称/agent/目录，匹配时自动展开、清除后恢复折叠状态；Vitest 覆盖“全部 / 运行中 / 已归档”与搜索
- [x] 3.4 为 `ArchivedSessions` 补充 Storybook 故事（含已中断、长标题、窄屏），并验证 Storybook 测试通过

## 4. 归档操作与返回首页

- [x] 4.1 `CloseSessionDialog`、`TerminalHeader`、`SessionRow`（以及 `SidebarSessionMenu`）对运行中的混合会话显示“停止会话并归档”，确认框标题「停止并归档“标题”？」、说明与非破坏性的“停止并归档”按钮，普通会话文案不变；组件测试分别验证两类会话的标题、说明与按钮（#296 初版为“归档会话”，#329 改为现名）
- [x] 4.2 增加 `onarchived` 回调并接到 `pages/Terminal.svelte` 的 `close`：单窗格导航到当前模式首页（`shellUi.home`），多窗格只移除窗格并聚焦相邻窗格；Vitest/浏览器测试覆盖两种布局与失败时停留原页显示错误
- [x] 4.3 首页归档后的处理（浏览器测试 `SessionListArchived`、`ArchivedSessions` 覆盖）：
  - 最初实现的 `?archived=` 定位、“刚归档”徽标与播报已由 #303 移除；现行为是返回首页后已归档区保持折叠、数量更新，筛选驱动的展开不变。
  - `ArchivedSessions` 的“删除记录”对可删除链接 worktree 打开 `RemoveWorktreeDialog`（#329）。
- [x] 4.4 更新 `web/test/e2e/hybrid.spec.ts`：从工作树启动混合会话 → 会话页“停止会话并归档” → 回到首页且该工作树“已归档”区保持折叠、展开后可见记录 → 恢复到终端后回到活动会话；用 `pnpm --dir web exec playwright test test/e2e/hybrid.spec.ts` 验证

## 5. 文档与用户故事

- [x] 5.1 把用户故事 US-C02、US-D10、US-Q12 中的“可恢复”描述改为工作树内“已归档”与“归档会话”，验证 `make docs-check` 通过
- [x] 5.2 更新使用文档中提到“可恢复”分组的句子，验证 `make docs-check` 通过：
  - `docs/hybrid-sessions.md`、`docs/faq.md`、`docs/overview.md`、`docs/usage.md`、`docs/usage/terminal.md`；
  - `docs/deployment/install-and-upgrade.md`；
  - `docs/protocol/http-hybrid.md`（同时说明 `workspace` 字段）。

## 6. 集成验证

- [x] 6.1 运行 `make check` 与 `make test`，并在隔离实例（非开发者现有服务）用演示数据手动走查桌面/手机、浅色/深色下的已归档区与归档后定位，截图用于 PR
  - #296 的 PR 已附桌面/手机 × 浅色/深色的前后对比截图。
  - 2026-11-06 在当前 main 复验：`make check` 通过；`make test` 各阶段通过（Go race 全部包、Vitest 单元 658、浏览器 546/547、Storybook 452、E2E 335、性能 18）。
  - 浏览器唯一失败 `CodeDocument` 横向滚动条用例与本变更无关：单独运行 2 次均通过，CI 在同一提交通过。
  - 用 E2E 隔离后端与假 agent 走查：会话页“停止会话并归档”→ 返回会话总览，“已归档 3”保持折叠，展开后 3 条记录均有恢复操作；桌面与 Pixel 7、浅色与深色四种组合均已截图确认。
