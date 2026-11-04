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
- 首页各分组沿用会话的 `visibleIds` 约定：`RepositoryGroup`、`DirectSessions` 接收全部已归档记录与筛选保留的 `visibleArchiveIds`，DOM 结构不随筛选改变；共享的 `now`、`filtering`、`highlight` 与恢复/删除回调合成一个 `ArchiveActions` 属性向下传递。
- 已中断条数在折叠标题中以 `Badge variant="warning"` 加文字“N 条已中断”呈现，不仅靠颜色。

### D4 归档后的导航与定位

- `CloseSessionDialog` 对混合会话（`session.hybrid` 存在且未退出）显示归档文案，按钮非破坏性（`Button` 默认 variant），成功后调用新回调 `onarchived(session)`。
- `SessionTerminal` 把 `onarchived` 接到 `Terminal.svelte` 的 `close(sessionId)`：单窗格时 `navigate("/?archived=<id>")`，多窗格时只移除窗格。复用现有 `close`，只把单窗格分支的目标改为带参数的首页。
- 首页读取 `archived` 查询参数，等 `recoverable` 列表刷新到包含该 ID 后：
  1. 当前仓库视图不含该记录时切到“全部”（不改本机记忆的选择），展开所属仓库（沿用 `expansionOverrides`/`focusedExpanded`）与工作树的已归档区，`scrollIntoView({block: "nearest"})`；
  2. 该行改用 `outline` 外观，并在标题旁显示文字徽标“刚归档”约 4 秒；不使用动画，无需 `prefers-reduced-motion` 降级，也不只靠颜色；
  3. 通过 `aria-live="polite"` 区域播报「已归档“标题”」；
  4. 读取参数后立即用 `history.replaceState` 去掉它，刷新不再重复定位。

  记录在 5 秒内未出现（例如写入失败）时只去掉参数，不报错。
- 在首页卡片菜单归档时不导航，只做同样的定位与播报。

### D5 筛选

`SessionFilters` 的状态选项加入 `{id: "archived", label: "已归档"}`，计数为已归档记录数。搜索匹配函数复用现有会话匹配逻辑的字段集（标题、名称、agent、cwd）。筛选期间已归档区的展开状态与 `filterExpanded` 一样独立于非筛选状态，清除后还原。

## Risks / Trade-offs

- [会话关联在 Git 解析完成前就被归档，快照为空] → D2 的 `cwd` 回退会把它放到正确工作树；仅子目录跨越多个注册仓库的极端情况可能落入直接会话。
- [拥有已归档记录的工作树强制显示，可能使首页变长] → 已归档区默认折叠只占一行；用户删除记录即可让工作树回到主页选择规则。
- [`?archived=` 参数被书签保存] → 首页处理后立即 `replaceState` 去掉。
- [文案从“关闭”改为“归档”，E2E 与 Storybook 依赖旧文案] → 同一变更内更新选择器与用户故事。

## Migration Plan

无数据迁移：旧记录没有快照，按 D2 回退归位；新快照在会话下一次关联更新、关闭或中断时写入。回滚到旧版本时，旧二进制忽略未知的 `workspace` 字段：`internal/hybrid` 与 `internal/statefile` 均未使用 `DisallowUnknownFields`，回滚不会把文件判为损坏；旧版本重写文件时会丢弃快照，之后再升级按 D2 回退。
