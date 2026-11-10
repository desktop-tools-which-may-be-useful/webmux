## Context

- 每次仓库快照的读取路径：
  - `readRepository` 先执行 `worktree list --porcelain -z`；
  - 再由 `populateWorktreeStatus` 为每个工作树执行一次 `status --porcelain=v1 -z --untracked-files=normal --ignored=matching`；
  - 并发上限是服务级的 `worktreeGate`（4）。
- `parseStatus` 只产出 `dirty`、`untracked`、`ignored` 三个布尔值，删除、释放和批量删除的判定都依赖这三个值。
- 分支接口（`GET /branches`）只执行一次 `for-each-ref`，格式为 `refname/objectname/subject`，同时供新建工作树弹窗和 CLI 使用。
- Git 进程统一由 `Service.git` 启动：带 `-C`，环境变量设置 `LC_ALL=C` 和 `GIT_TERMINAL_PROMPT=0`，输出有大小上限，超时由 `readTimeout`（10s）控制。
- 前端现状：
  - 单工作树详情 `WorktreeDetailsDialog` 只在首页分组可用；
  - `RepositoryDetailsDialog` 只有四行文字；
  - `ManageWorktreesDialog`（约 513 行）自带列表、筛选和“选择 → 预览 → 结果”三段流程；
  - 这三个弹窗都由 `AppShell` 以 `LazyDialog` 挂载。
- 现有仓库与工作树状态没有推送通道，刷新只由生命周期、重连或显式操作触发。
- `batch-worktree-management` 已完成但未归档，本变更的增量规格修改了它引入的需求。

## Goals / Non-Goals

**Goals:**

- 每次快照读取工作树详细状态时，Git 进程数量不随新增字段增加。唯一的例外是每个仓库多一次批量 `log`。
- 单工作树详情、仓库面板、批量清理三处共用同一组展示组件，数据只有一个来源。

**Non-Goals:**

- 不做任何会改变 Git 状态的操作。
- 不推送状态变更（不加 WebSocket 事件）。
- 不展示逐文件的修改清单（代码视图和 diff review 已经覆盖）。
- 不展示子模块状态。

## Decisions

### 1. 状态读取改为 `status --porcelain=v2 -z --branch`

一次调用同时得到 `# branch.oid`、`# branch.head`、`# branch.upstream`、`# branch.ab +A -B` 和逐条目记录：
- `1`/`2` 为普通/重命名条目，X 为暂存区、Y 为工作区，`.` 表示未变；
- `u` 为冲突，`?` 为未跟踪，`!` 为忽略。

由此计算：
- `staged`：X ≠ `.` 的 `1`/`2` 条目数；
- `unstaged`：Y ≠ `.` 的 `1`/`2` 条目数；
- `conflicted`：`u` 条目数；
- `untracked`：`?` 条目数。

一个条目可以同时计入 staged 和 unstaged。`dirty` 保持“存在任何已跟踪变更”（`1`/`2`/`u`），`hasUntracked`、`hasIgnored` 的含义不变，删除相关判定不受影响。

上游判定：有 `branch.upstream` 但没有 `branch.ab`，表示上游引用已不存在，记为 `upstreamGone`；没有 `branch.upstream` 表示无上游。

- 备选方案：保留 v1，另外为每个工作树执行 `rev-list --left-right --count`。这会让每个工作树多一个进程，被否决。
- 备选方案：`for-each-ref %(ahead-behind:...)` 需要 Git 2.41，当前隐含的最低版本（`--path-format=absolute` 要求 2.31）不满足，被否决。
- 注意：`-z` 模式下重命名条目（`2`）后面跟一个额外的 NUL 路径，解析器必须跳过它，否则后续记录会错位。

### 2. 最近提交按仓库批量读取

所有工作树的状态读取完成后，收集各工作树去重后的 `Head`，执行一次 `log --no-walk=unsorted --no-show-signature --format=%H%x00%s%x00%an%x00%cI%x00 <heads...> --`，按哈希回填 `lastCommit`。这个函数沿用 `RecentCommits` 中已有的 `missing` 补查写法。

失败时只把 `lastCommit` 置空，不把整个仓库标为不可用。bare、不可用或尚无提交的工作树不参与。

### 3. 契约字段

- `Worktree` 新增以下字段：
  - `upstream *string`；
  - `ahead *int`、`behind *int`：无上游或上游已删除时为 null；
  - `upstreamGone bool`；
  - `lastCommit *Commit`：复用现有 `Commit` 类型；
  - `changes *WorktreeChanges{staged, unstaged, untracked, conflicted}`：状态读取失败时为 null。
- `Repository` 新增 `lastFetchedAt *time.Time`。
- `Branch` 新增 `subject`、`committedAt`、`upstream`、`ahead`、`behind`、`upstreamGone`。
- `BranchBase` 新增 `kind`（`local` | `remote`）和 `committedAt`。分支分区的远程组由此筛选，不另设一份远程分支列表。
- 新增 `Remote{name, fetchUrl, pushUrl}` 和 `Remotes{remotes}`。

所有新增字段都只增不改，旧前端忽略它们也能工作。Zod schema 同步收紧，`wire` 快照重新录制。

### 4. 分支的上游信息

在现有 `for-each-ref` 格式后追加 `%(upstream)%00%(upstream:track,nobracket)%00%(committerdate:iso-strict)`。`LC_ALL=C` 保证 track 字段输出固定的英文词（`ahead N`、`behind N`、`ahead N, behind M`、`gone`、空），解析时按这些词匹配。

结果始终包含在 `GET /branches` 中，不另设参数。新建工作树弹窗同时受益：它可以在起始分支列表中标出领先/落后。

### 5. 上次获取时间取 `FETCH_HEAD` 的修改时间

本机实验（Git 2.47）确认 linked worktree 的 `FETCH_HEAD` 位于 `<common>/worktrees/<name>/FETCH_HEAD`。因此取 `<common>/FETCH_HEAD` 与 `<common>/worktrees/*/FETCH_HEAD` 中最新的修改时间，读文件状态即可，不启动 Git 进程。

- 已知不准的情况：`fetch --no-write-fetch-head`，以及 IDE 等外部工具只更新远程引用。界面标“近似”。
- 备选方案：读取 `logs/refs/remotes/*` 的 reflog，但 reflog 可能被关闭或过期，被否决。

### 6. 远程清单与脱敏

新增 `GET /api/repositories/{id}/remotes`，在仓库门闩下执行 `remote -v`，按 `name\turl (fetch|push)` 解析。脱敏函数集中在 `internal/workspace`：
- 包含 `://` 的 URL 交给 `net/url` 解析：有密码时用 `url.UserPassword(user, "***")` 替换；`http`/`https` 只有用户名时，整个 userinfo 换成 `***`；
- 解析失败的 URL 不原样返回，而是丢弃 `@` 之前的部分；
- scp 风格（`user@host:path`）和 `ssh://` URL 保留用户名。

服务端在返回前完成脱敏，任何日志都不记录原始 URL。

### 7. 仓库面板的组件拆分

`RepositoryDetailsDialog.svelte` 重命名为 `RepositoryPanel.svelte`，它只负责外壳：`ResponsiveDialog` 加 `Tabs variant="line"`，接收 `initialTab`。四个分区各自一个组件：

- `RepositoryOverview.svelte`；
- `RepositoryWorktrees.svelte`：从 `ManageWorktreesDialog` 迁入列表、筛选、选择模式和三段流程，`ManageWorktreesPreview.svelte` 原样复用；
- `RepositoryBranches.svelte`：进入分区时才请求 `/branches`，使用 `LatestRequest`；
- `RepositoryRemotes.svelte`：进入分区时才请求 `/remotes`。

新增 `WorktreeSyncSummary.svelte`，统一渲染上游、领先/落后、修改统计和最近提交，提供 `compact` 与 `full` 两种形态：
- 面板行和首页标题使用 `compact`；
- `WorktreeDetailsDialog` 使用 `full`。

领先/落后指示使用 tabler `arrow-up` / `arrow-down` 图标加数字，再加 `aria-label`。现有 `Badge` 变体不够时，在 `ui/badge` 中增加变体并登记到前端架构守卫，调用处不写 `class` 改样式。

`shell.svelte.ts` 的调整：
- `repositoryDetails(repository, tab?)` 成为唯一的面板入口；
- `manageWorktrees(repository)` 改为调用 `repositoryDetails(repository, "worktrees")`；
- 删除 `managingWorktrees` 状态和 `ManageWorktreesDialog.svelte`。

批量预览或执行处于 `busy` 时，Tabs 禁用。

- 备选方案：保留独立的批量弹窗，只共用行组件。这样仍有两份工作树列表和两个常驻入口，违反“同一种界面只有一个组件”，被否决。
- 备选方案：做成独立路由页 `/repositories/{id}`。现有导航模型是首页视图加弹层，新路由要连带改动侧栏和返回逻辑，本变更不做。

### 8. 侧栏工作树行菜单

复用 `SidebarRowMenu`。`WorktreeMenuItems` 增加 `ondetails`，侧栏不传 `ontogglepin`，因为置顶已在“主页显示”中提供。原来的 `删除工作树 {label}` 图标按钮删除，对应测试改为通过菜单定位。

### 9. CLI

`repo show`、`worktree ls/show`、`repo branches` 直接使用扩展后的契约字段。新增 `repo remotes` 子命令，调用新接口。

## Risks / Trade-offs

- [风险] 分支差距很大时，`--branch` 计算 ahead/behind 较慢，可能让单个工作树的状态读取超时 → 超时只影响该工作树，现有行为已显示“状态未知”。实现时在测试仓库中构造约 10k 提交的差距并测量，超过 2s 时为该调用加 `--no-ahead-behind`，并把领先/落后显示为“未计算”。
- [风险] 本地分支很多时，`for-each-ref` 的 track 计算拖慢新建工作树弹窗 → 在 500 个分支的夹具上测量，`GET /branches` 应保持在 1s 内。超出时把 track 改为面板专用的查询参数，这一处属于实现细节，不改变规格。
- [风险] porcelain v2 解析错误会让删除保护判定出错 → 解析器保留 v1 的 `dirty/untracked/ignored` 语义，并为重命名、冲突、子模块条目（`1 .M S...`）、尚无提交（`# branch.oid (initial)`）补充表驱动单测。删除相关的现有测试必须全部通过。
- [取舍] 上次获取时间是近似值 → 文案标“近似”，不用它做任何判定。
- [取舍] 合并弹窗是界面结构上的 BREAKING 改动 → 菜单文字“管理工作树…”保留，e2e helper `repository-menu.ts` 集中迁移。

## Migration Plan

1. 先归档 `batch-worktree-management`。归档后执行 `openspec validate enrich-repository-details --strict`，确认不再报告“Archive would refuse”。
2. 先落地后端字段和快照，再落地前端。字段只增不改，两步之间可以独立发布。
3. 不涉及持久化格式变化。回滚只需还原代码。
