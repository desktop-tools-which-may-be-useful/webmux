## 1. 默认值解析（纯逻辑）

- [x] 1.1 在 `web/src/new-session-defaults.ts` 实现 `resolveInitialTarget`，覆盖上下文入口、主页选中仓库、无可用工作树、最近目录与默认目录；Vitest 覆盖各分支（`test/new-session-defaults.test.ts`）。
- [x] 1.2 实现 `resolveInitialLaunch`：记住的配置存在才选中、删除后为未选择、无记忆取第一个配置、记住自定义命令不等待加载；Vitest 覆盖。

## 2. 对话框组件重构

- [x] 2.1 新增 `PickerField.svelte`：桌面 Popover + Command，窄屏在抽屉内就地展开，Esc 只关闭选择器。
- [x] 2.2 新增 `LocationPicker.svelte`：按“当前仓库、最近目录、其他仓库、输入或浏览目录”分组，每个仓库分组提供“新建工作树…”，不可用与 bare 工作树禁用，直接目录显示输入与浏览。
- [x] 2.3 新增 `LaunchPicker.svelte`：启动配置与“自定义命令…”同列，切换保留输入，混合配置显示起始端与连接器提示，空状态与不可用原因在列表中显示。
- [x] 2.4 调整 `WorktreeCreateFields.svelte`：分支字段并排，起始提交与存放目录移入 `AdvancedSection`，暴露 `startBranch`，精简常驻说明。
- [x] 2.5 以 `NewSessionDialog.svelte` 取代 `NewSessionSheet.svelte`：汇总行、请求体互斥、成功后才写入记忆、工作树已创建后重试只启动会话、目录创建确认与连接器缺失流程；删除旧文件。
- [x] 2.6 保留“仅新建工作树”（默认关闭“创建后启动会话”）与分屏（方向选择、目录未改时保留工作区归属）变体。

## 3. 调用点与联动

- [x] 3.1 `SessionList.svelte` 传入 `entry` 与 `selectedRepositoryId`，移除页面内的预选推导。
- [x] 3.2 `Terminal.svelte` 以目录入口打开分屏对话框，`onworktreecreated` 行为不变。

## 4. 测试迁移

- [x] 4.1 将浏览器测试改写为 `test/browser/NewSessionDialog.test.ts`，覆盖默认值、失效记忆、加载竞态、键盘与 Esc、失败保留输入、新工作树重试、窄屏无溢出与两种变体。
- [x] 4.2 `WorktreeCreateFields.test.ts` 在交互前展开高级区。
- [x] 4.3 `test/e2e/fixtures.ts` 新增 `chooseLaunch`、`chooseLocation`、`useCustomCommand`、`useDirectory`、`nameSession`，并迁移依赖旧 Tab 文案的 E2E 用例。
- [x] 4.4 运行受影响的 E2E 规格并全部通过。

## 5. 文档与验收

- [x] 5.1 更新 `docs/user-stories/access-and-sessions.md` 的 US-B01、US-B02、US-B03、US-B06，`docs/usage.md` 与 `docs/cli-agents.md`，以及指向旧文件的依据链接。
- [x] 5.2 运行 `make check` 与 `make test-fast`，修复本变更导致的失败。
- [x] 5.3 在隔离演示数据上截取桌面与移动、浅色与深色的前后对比截图，附在 PR 中。
