# Design

## Context

- 混合会话关闭走 `DELETE /api/sessions/{id}`：`internal/hybrid/actor.go` 的 `finishClose` 把记录设为 `closed` 并 `manager.Remove` 外壳，会话从列表消失。会话页的 `CloseSessionDialog` 只在“移除已退出会话”时回调 `onremoved`，所以归档运行中的混合会话后页面停在已失效的会话上。
- 记录（`internal/hybrid/records.go` 的 `Record`、`internal/contract.HybridRecord`）只有 `cwd`，没有仓库/工作树关联。会话的关联由 `internal/workspace` 用 Git 解析 `cwd` 后经 `session.Manager.SetWorkspace` 写入 `session.Info.Workspace`，可能在会话启动后异步到达。
- 首页 `SessionList.svelte` 在底部渲染 `RecoverableSessions.svelte`；工作树分组由 `RepositoryGroup` → `WorkspaceGroup`，直接会话由 `DirectSessions`；主页工作树显隐由 `homeWorktrees.ts` 的 `homeLayout`（置顶或有会话）决定。

## Goals / Non-Goals

**Goals:**

- 已归档记录按工作树就近展示，归位规则在前端纯函数中集中实现、可单测。
- 归档→回到首页→看见记录形成一个连续反馈闭环。

**Non-Goals:**

- 普通 PTY 会话不归档、不跨重启，“关闭/移除会话”不变。
- 不做撤销归档（恢复即撤销，且恢复要重启 agent，不能伪装成瞬时撤销）；不新增全局 toast 系统。
- 不改变 `hybrid-sessions.json` 的 200 条上限、恢复流程与 HTTP 路径。
- 折叠状态不做本机持久化（避免新增浏览器存储项）。

## Decisions

### D1 服务端保存工作区关联快照，而不是列表时实时解析

`Record` 增加可选 `Workspace *WorkspaceRef{RepositoryID, WorktreeID}`（JSON `workspace,omitempty`）。actor 在已有的 `saveShellTitle` 同一路径上（shell `Info` 更新、关闭、中断）比较 `info.Workspace` 并在变化时写入，关联被清空时置空。

- 备选：`GET /api/hybrid-sessions` 时对每条记录用 Git 解析 `cwd`。最多 200 次 Git 调用、慢且受 Git 可用性影响；服务重启后 actor 不存在也无法取到会话关联。否决。
- 备选：只在前端按 `cwd` 前缀匹配。子目录、符号链接、已删除工作树（需要保留“已不可用工作区”身份）都会误判。仅作为旧记录回退（D2）。
- 字段可选、`omitempty`，旧文件按现有解码规则读取；`statefile` catalog 版本号不变。若持久化守卫要求为新字段登记，按守卫提示更新 `docs/persistence.md`。
- `contract.HybridRecord` 新增 `Workspace *WorkspaceReference`（复用创建会话时的 `{repositoryId, worktreeId}` 类型；`SessionWorkspace` 另含路径，记录不保存），`wire` 转换，重录 `internal/api/testdata/wire`。
- actor 在原保存标题的同一路径（`saveShellInfo`）与服务关机时保存关联；恢复时以记录的值为基准，避免关联尚未解析时清空快照。

### D2 归位在前端一个纯函数中完成

新增 `web/src/archived.ts`（或并入 `recoverable.ts`）的 `placeArchived(records, repositories)`，返回三组：

- `byWorktree: Map<worktreeId, HybridRecord[]>`；
- `missingByRepo: Map<repoId, HybridRecord[]>`（“已不可用工作区”）；
- `direct: HybridRecord[]`，按 `cwd` 分目录，分组键与 `DirectSessions` 现有目录分组一致。

规则即 spec 三步：快照优先 → 旧记录按规范化路径最长前缀匹配 `worktree.path` → 其余进直接会话。函数保证每条记录只落一处；单测覆盖每个分支与“仓库读取失败时不丢失”。

### D3 组件结构

- `RecoverableSessions.svelte` 改为 `ArchivedSessions.svelte`，接收一个分组的记录：
  - 折叠区用 `Collapsible` + `Badge`，行用 `Item`，恢复操作复用 `HybridResumeActions`；
  - “删除记录”放入 `DropdownMenu`（`variant="destructive"`），确认复用 `ConfirmDialog`；
  - 不新建设计系统组件；行的弱化外观用 `Item` 现有 variant，缺少时为 `Item` 增加 variant，而不是 `class` 覆写。
- `WorkspaceGroup` 增加 `archived` 属性并在会话列表下渲染；仅有已归档记录时，原“暂无会话”空状态改为不显示（避免“暂无会话”与“已归档 3”矛盾）。
- `homeLayout(home, repo, sessions, archived)`：`archived` 是该仓库按工作树分组的已归档记录，有记录的工作树视为 `active`。调用方 `RepositoryGroup` 一起迁移；主页工作树选择框仍只按会话标记“有会话”。
- 首页各分组沿用会话的 `visibleIds` 约定：`RepositoryGroup`、`DirectSessions` 接收全部已归档记录与筛选保留的 `visibleArchiveIds`，DOM 结构不随筛选改变；共享的 `now`、`filtering` 与恢复/删除回调合成一个 `ArchiveActions` 属性向下传递。
- 已中断条数在折叠标题中以 `Badge variant="warning"` 加文字“N 条已中断”呈现，不仅靠颜色。

### D4 归档后的导航

- `CloseSessionDialog` 对活动的混合会话（`session.hybrid.state === "active"` 且未退出）显示标题「停止并归档“标题”？」与归档文案，按钮“停止并归档”非破坏性（`Button` 默认 variant），成功后调用回调 `onarchived(session)`；失败时留在确认框内显示错误。
- `SessionTerminal` 把 `onarchived` 接到 `pages/Terminal.svelte` 的 `close(sessionId)`：单窗格时 `navigate(shellUi.home)`（工作台模式 `/`，聊天模式 `/new`），多窗格时只移除窗格并聚焦相邻窗格；`AppShell` 同时刷新已归档记录列表。
- 首页不再自动展开、滚动、高亮或播报刚归档的记录，已归档区保持原折叠状态。最初实现的 `?archived=<id>` 参数、“刚归档”徽标与 `aria-live` 播报已由 #303 移除。
- 记录的“删除记录”经 `removableRecordWorktree`（`web/src/archived.ts`）判断：所属工作树为可用且未待清理的链接 worktree 时打开 `RemoveWorktreeDialog`，否则使用只删除记录的 `ConfirmDialog`（#329）。

### D5 筛选

`SessionFilters` 的状态选项加入 `{id: "archived", label: "已归档"}`，计数为已归档记录数。搜索匹配函数复用现有会话匹配逻辑的字段集（标题、名称、agent、cwd）。筛选期间已归档区的展开状态与 `filterExpanded` 一样独立于非筛选状态，清除后还原。

## Risks / Trade-offs

- [会话关联在 Git 解析完成前就被归档，快照为空] → D2 的 `cwd` 回退会把它放到正确工作树；仅子目录跨越多个注册仓库的极端情况可能落入直接会话。
- [拥有已归档记录的工作树强制显示，可能使首页变长] → 已归档区默认折叠只占一行；用户删除记录即可让工作树回到主页选择规则。
- [归档后不再自动定位，用户需自行展开已归档区] → 折叠区标题显示数量，变化可见；自动展开在多次归档时会让首页跳动，#303 因此移除。
- [文案从“关闭”改为“归档”，E2E 与 Storybook 依赖旧文案] → 同一变更内更新选择器与用户故事。

## Migration Plan

无数据迁移：旧记录没有快照，按 D2 回退归位；新快照在会话下一次关联更新、关闭或中断时写入。回滚到旧版本时，旧二进制忽略未知的 `workspace` 字段：`internal/hybrid` 与 `internal/statefile` 均未使用 `DisallowUnknownFields`，回滚不会把文件判为损坏；旧版本重写文件时会丢弃快照，之后再升级按 D2 回退。
