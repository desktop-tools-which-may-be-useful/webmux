# Tasks

## 1. 记录的工作区关联快照（后端）

- [x] 1.1 在 `internal/hybrid/records.go` 的 `Record` 增加可选 `Workspace`（`workspace,omitempty`，含 `clone` 深拷贝），并验证 `go test -race ./internal/hybrid` 中新增用例：旧格式文件（无该字段）正常加载、写回后字段省略
- [x] 1.2 在 `internal/hybrid/actor.go` 与 `saveShellTitle` 同一路径上保存 `shell.Info().Workspace` 的变化（关联出现、改变、清空；关闭与中断时保留最后值），并用 race 测试验证：从工作树启动后记录带快照，服务重启转为 `interrupted` 后快照仍在，关联被解除后清空
- [x] 1.3 在 `internal/contract.HybridRecord` 增加 `Workspace *WorkspaceReference`（`omitempty`），在 `internal/wire` 转换，执行 `go test ./internal/api -run TestWireSnapshots -update` 重录 `hybrid-records.json`，并验证 `internal/wire/testdata` 与 `go test -race ./internal/api ./internal/wire` 通过
- [x] 1.4 运行持久化守卫（`make check` 中的 persistence guard）；若要求登记新字段，更新 `docs/persistence.md` 并验证守卫通过

## 2. 前端契约与归位逻辑

- [x] 2.1 在 `web/src/schemas.ts` 的混合记录 schema 增加可选 `workspace`（复用会话的工作区 schema），并验证契约测试与 `pnpm --dir web check` 通过
- [x] 2.2 新增归位纯函数（D2：快照 → 规范化路径最长前缀 → 直接会话目录分组），用 Vitest 覆盖快照命中、工作树已不在清单（进“已不可用工作区”）、仓库已移除、旧记录子目录匹配、无匹配、每条记录只出现一次
- [x] 2.3 `homeWorktrees.ts` 的 `homeLayout` 接收有已归档记录的工作树 ID 并视同有会话，迁移 `RepositoryGroup` 调用；Vitest 验证未置顶但有已归档记录的工作树显示、记录清空后隐藏

## 3. 工作树内的“已归档”区（UI）

- [x] 3.1 把 `RecoverableSessions.svelte` 改造为 `ArchivedSessions.svelte`（D3）：默认折叠、标题“已归档 N”与“N 条已中断”徽标、按更新时间倒序、默认 5 条与“显示全部 N 条”、行内“恢复到聊天/恢复到终端”、更多菜单“删除记录”+确认；组件测试覆盖这些行为与键盘操作，并通过 `pnpm --dir web check:arch`
- [x] 3.2 在 `WorkspaceGroup`（工作树与“已不可用工作区”）和 `DirectSessions` 目录分组中渲染该区，移除 `SessionList.svelte` 底部的全局分组；只有已归档记录时不显示“暂无会话”；已归档记录不计入运行/等待/退出数量；浏览器测试验证 18 条分散记录各自出现在所属分组
- [x] 3.3 `SessionFilters` 增加“已归档”状态选项与计数，搜索匹配已归档记录的标题/名称/agent/目录，匹配时自动展开、清除后恢复折叠状态；Vitest 覆盖“全部 / 运行中 / 已归档”与搜索
- [x] 3.4 为 `ArchivedSessions` 补充 Storybook 故事（含已中断、长标题、窄屏），并验证 Storybook 测试通过

## 4. 归档操作与返回首页

- [x] 4.1 `CloseSessionDialog`、`TerminalHeader`、`SessionRow` 对运行中的混合会话显示“归档会话”与归档确认文案（非破坏性按钮），普通会话文案不变；组件测试分别验证两类会话的标题、说明与按钮
- [x] 4.2 增加 `onarchived` 回调并接到 `Terminal.svelte` 的 `close`：单窗格导航到 `/?archived=<id>`，多窗格只移除窗格并聚焦相邻窗格；Vitest/浏览器测试覆盖两种布局与失败时停留原页显示错误
- [x] 4.3 首页处理 `archived` 参数（D4）：等记录出现后展开仓库/工作树与已归档区、滚动到可见、以 outline 外观与“刚归档”文字徽标标出约 4 秒、`aria-live` 播报“已归档”、`replaceState` 去掉参数、5 秒未出现则静默放弃；首页卡片菜单归档时同样定位且不导航；浏览器测试验证
- [x] 4.4 更新 `web/test/e2e/hybrid.spec.ts`：从工作树启动混合会话 → 会话页归档 → 回到首页且记录在该工作树“已归档”区高亮 → 恢复到终端后回到活动会话；用 `pnpm --dir web exec playwright test test/e2e/hybrid.spec.ts` 验证

## 5. 文档与用户故事

- [x] 5.1 把用户故事 US-C02、US-D10、US-Q12 中的“可恢复”描述改为工作树内“已归档”与“归档会话”，验证 `make docs-check` 通过
- [x] 5.2 更新使用文档中提到“可恢复”分组的句子，验证 `make docs-check` 通过：
  - `docs/hybrid-sessions.md`、`docs/faq.md`、`docs/overview.md`、`docs/usage.md`、`docs/usage/terminal.md`；
  - `docs/deployment/install-and-upgrade.md`；
  - `docs/protocol/http-hybrid.md`（同时说明 `workspace` 字段）。

## 6. 集成验证

- [ ] 6.1 运行 `make check` 与 `make test`，并在隔离实例（非开发者现有服务）用演示数据手动走查桌面/手机、浅色/深色下的已归档区与归档后定位，截图用于 PR
