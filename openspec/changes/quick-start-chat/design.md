# Design

## Context

能力都已存在，只是分散在三个接口里，由前端依次调用：

- `POST /api/repositories/{id}/worktrees`（`workspace.CreateWorktree`）：`{path, mode, branch, base}`，由服务端用 `git check-ref-format` 校验分支；失败不回滚。分支建议来自 `GET …/branches`（`DefaultParent`、`DefaultBase`、`Bases`）和前端的 `branchName.ts` `suggestWorktreeBranch`。
- `POST /api/sessions`（`workspace.CreateSession` → `hybrid.Service.Create`）：`configId` 加 `startSide: "acp"`，返回前已完成 ACP `initialize` 和 `session/new`。
- `POST /api/sessions/{id}/chat/prompts`（`hybrid.Service.Prompt`）：按 `clientRequestId` 幂等。

`NewSessionDialog.svelte` 现在就是在浏览器里串联前两步。agent 的配置项（`ChatState.config`）只有会话运行后才知道；`hybrid.Record.Config` 持久保存了每个会话最后使用的配置值，但没有保存配置项清单（名称、类别、可选值）。仓库登记记录（`registration`）只有 `ID/CommonDir/AnchorPath/DisplayName`。

## Goals / Non-Goals

**Goals:**

- 一个请求在服务端完成 worktree、会话、配置和首条提示，浏览器断开不影响结果。
- 启动前就能选择配置，复用聊天输入框现有的配置控件。
- 新增持久状态最少，遵守 `persistence-governance`。
- 与现有混合模式完全兼容：快速开始和聊天侧栏只是新的启动入口和显示页面，底层的会话、切换、记录和接口语义都不变。

**Non-Goals:**

- 不支持直接目录或已有 worktree 作为快速开始目标；这种情况继续使用新建会话弹窗。
- 首条消息不带附件。
- 不做语音输入、云端环境选择（参考截图中的“默认环境”）。
- 失败时不自动删除已创建的 worktree。
- 不新增会话类型，不给会话或混合会话记录加“来源”字段，也不改变 `hybrid-sessions` 的任何规则。
- 快速开始不提供“从终端开始”；需要先进终端的用户继续使用新建会话弹窗，或在会话里切过去。

## Decisions

### 0. 复用边界：只组合现有能力

快速开始等价于在浏览器里依次完成“新建 worktree → 新建混合会话（`startSide: acp`）→ 调整配置 → 发送第一条消息”。区别只在于这几步改由服务端连续执行。每一步都调用现有的服务入口：

| 步骤 | 复用的入口 | 现有规则 |
| --- | --- | --- |
| 预检 | 启动配置的混合模式判定、`agent-session-identity` 的连接器检查 | `hybrid-sessions`：未勾选返回 400，缺连接器返回 409 `connector_missing` |
| worktree | `workspace.CreateWorktree` | `worktree-lifecycle`，分支与路径校验不变 |
| 会话 | `workspace.CreateSession` → `hybrid.Service.Create` | ACP `initialize` + `session/new`，argv 播种配置 |
| 配置 | 聊天输入框使用的同一个 set-config 与权限模式服务方法 | 取值写入混合会话记录，之后的切换和恢复照常重新应用 |
| 首条消息 | `hybrid.Service.Prompt`（`ClientRequestID` 复用请求 ID） | 排队、回合、`auth_required` 引导都与手动发送相同 |
| 提示信息 | 现有的混合会话 notice | 与模式回退等提示同一种展示 |

因此：

- 快速开始返回后的会话与手动创建的会话没有区别，后续切换、休眠、恢复、关闭和归档都不需要额外分支。
- 新增的只有三样：仓库初始化指令、按启动配置记忆的配置项清单、请求 ID 去重缓存。它们都在会话之外，不写入会话或混合会话记录。
- 前端的会话页仍是 `pages/Terminal.svelte`，聊天视图仍是 `ChatView`；侧栏只是包在外面的一层布局。

### 1. 服务端编排，单一接口 `POST /api/quick-starts`

接口形状：

- 请求 `QuickStartRequest{clientRequestId, repositoryId, configId, base, branch, text, config: map[string]string, permissionMode}`，其中 `config` 是页面上显示的全部配置值（预选值加用户的修改）。
- 成功响应 `QuickStartResult{sessionId, worktree, configWarnings[]}`。
- 失败时返回结构化错误 `{step: "worktree"|"session"|"config"|"prompt", message, worktree?, sessionId?}`：状态码由 `internal/api/errors.go` 映射，失败步骤和已创建资源放在错误体中。

编排放在 `internal/workspace`（新文件 `quickstart.go`）：它已经持有 worktree 创建和 `CreateSession`，并已依赖 `internal/hybrid`（`sessions.go`、`remove.go`），设置配置和首条提示直接调用 `hybrid.Service`，不引入新的服务包或层级例外。

处理器使用 `context.WithoutCancel(r.Context())` 加独立超时（与会话创建一致），客户端断开不会中断编排。

- 备选：继续在前端串联三步。缺点是关页面会停在中间（有 worktree 无会话，或有会话无首条消息），无法满足“浏览器断开不影响”。
- 备选：给 `CreateSessionRequest` 加 `initialPrompt`。会让普通会话创建接口也承担 worktree 和配置，职责混杂。

### 2. 分支名在前端生成，路径在服务端推导

- 前端加载 `GET …/branches`，取值与现有新建工作树弹窗（`WorktreeCreateFields`）一致：
  - 基准分支选择器默认显示 `DefaultBranch`（主工作区当前分支名），请求中的 `base` 发送 `DefaultBase`（该分支的 HEAD 提交）。
  - 可选的基准与弹窗的“起始分支”相同，只列本地分支 `Branches[]`：显示 `name`，发送对应的 `head`。远程引用不提供。
  - 分支名用现有 `suggestWorktreeBranch` 生成（形如 `wt/<adjective>-<noun>`）后放进请求。这样分支命名只有一个实现。
- 服务端用 `DefaultParent/<分支名最后一段>` 推导路径，与弹窗的路径规则相同（`wt/brisk-otter` → `webmux.worktrees/brisk-otter`），沿用 `CreateWorktree` 的分支校验。
- 分支名或路径冲突时返回 `step: "worktree"` 的冲突错误；前端重新生成分支名，用新请求 ID 再发送。

### 3. 幂等：进程内按请求 ID 去重

`clientRequestId` 映射到一个 `singleflight` 风格的条目：执行中的重复请求等待同一结果；完成后的结果保留 10 分钟，然后清除。

- 去重窗口因此是“执行中及完成后 10 分钟”；窗口外用同一 ID 重发会作为新请求执行。前端只在一次发送的网络重试中复用 ID，重试远短于这个窗口。
- 不持久化，服务重启本来就会结束会话。
- 首条提示复用同一个请求 ID 作为 `Prompt` 的 `ClientRequestID`。

### 4. 配置项清单按启动配置持久记忆

- 记录：`hybrid` 在每次 ACP 会话报告配置项时（`session/new` 及后续更新），按 `configId` 记下最近的配置项清单（ID、名称、类别、类型、可选值）和当时的值。
- 保存：新的状态文件 `agent-options.json`，在 `internal/statefile/catalog.go` 登记；删除启动配置时一并清除。
- 读取：`GET /api/launch-configs` 的每个配置新增字段 `chatOptions`，结构直接复用 `ChatConfig`，因此 `ChatConfig.svelte` 可以原样渲染。

启动后的应用顺序：

1. `hybrid.Service.Create` 完成 `session/new` 和 argv 播种。
2. 服务端把请求中的全部配置值逐项与播种结果比较，只对不同的项调用现有的设置配置路径。页面显示的值因此就是新会话实际得到的值；清单的值来自该启动配置最近一次会话的最终值，通常与播种结果一致，不会产生多余的设置调用。
3. 发送首条提示。

agent 不接受的值记为 `configWarnings`，并作为聊天 notice 写入时间线；不阻断发送。

清单只是展示用的缓存：它按 `configId` 记录、来源于任何入口创建的混合会话，不写入混合会话记录，也不参与切换或恢复。

- 备选：选择 agent 时预启动一个 ACP 进程探测配置。代价是每次切换都起进程，而且 cwd 还未确定。
- 备选：只存在内存里。服务重启后首个会话之前都没有配置可选，体验差，且成本只是一份小 JSON。

### 5. 初始化指令存在仓库登记中

- 存储：`registration` 增加 `InitInstruction string`（`omitempty`）；`registry.Version` 不变，旧文件缺字段即为空。
- 接口：`RegisterRepositoryRequest` 增加可选 `initInstruction`；新增 `PATCH /api/repositories/{id}` `{initInstruction}` 用于修改或清空。
- 校验：长度上限 4000 个字符，由服务端校验。
- 重复登记：保留已有指令，响应照常返回 200 和已有条目，前端据此提示。

拼接在服务端完成：`text = instruction + "\n\n" + userText`（指令为空时只用用户文字）。这样首条提示与仓库当前保存的指令一致，前端提示只用于展示。

### 6. 前端页面

- 路由 `/new`（`route.ts` 新增 `quick-start`），`pages/QuickStart.svelte` 懒加载，作为聊天模式的主区域。
- 布局参考 Claude Code 网页版：空白主区域，底部固定输入卡片，卡片下方一行放选择控件。
  - 仓库、基准分支、agent 使用显示当前值文字的下拉：给 `ChatSelect` 增加 `labeled` 用法，不另写一套。
  - agent 配置直接用 `ChatConfig`，权限用 `ChatPermissionMode`；桌面端 agent 与配置靠右，与仓库、分支分列两侧。
  - 窄屏与聊天输入框一致：agent 和 `ChatSessionOptions` 收进卡片底部，下方一行只留仓库和分支，避免配置被截断。
- 初始化指令提示放在输入卡片顶部的一行（与命令菜单、附件同一位置），点击用 Popover 显示全文。
- 上次的仓库和 agent 存在 `localStorage`，用 Zod 校验后读取。
- 草稿：输入文字存在独立的 `localStorage` 键 `webmux.quick-start-draft`，用 Zod 校验，发送成功后清除。不复用 `terminal-draft`：它的 `StoredDraftSchema.sessionId` 只接受会话 ID，`reconcile()` 也会删除不属于存活会话的键。

### 7. 应用外壳与两种模式

参考 Claude Code 网页版：左上角两个图标按钮切换聊天和代码（这里是工作台），侧栏在页面之间常驻，全局入口放在侧栏底部。

- 挂载位置：`App.svelte` 在 `sessions`（`/`）、`quick-start`（`/new`）和 `terminal`（`/s/{id}`）三种路由外包一层 `AppShell`。设置页、代码弹出窗口和登录页不使用外壳。
  - 外壳实例不随路由切换重建，侧栏滚动位置和树的展开状态因此得以保留；主区域仍按路由懒加载，`Terminal` 继续按 `sessionId` 重建。
  - 首次渲染就是最终结构：外壳和侧栏骨架随第一帧画出，数据到达后只填充内容，不出现先无侧栏、再加侧栏的跳动。
  - `.terminal-page` 现在是 `position: fixed; left: 0; right: 0; width: 100%`。改为 `left: var(--app-sidebar-width, 0); right: 0; width: auto`，否则 `width: 100%` 会让终端超出视口一个侧栏宽度；由外壳按桌面/窄栏/手机设置这个变量，终端尺寸走现有的 resize 流程。原型用带 transform 的容器代替这个偏移。
- 模式状态：`mode: "chat" | "workbench"` 存在 `localStorage`，用 Zod 校验，无效时回退到工作台。
  - 首页路由决定模式：`/` 是工作台，`/new` 是聊天，进入时写回记住的模式。
  - 打开 `/` 且记住的是聊天时，用 `history.replaceState` 改成 `/new`，避免后退回到工作台。
  - 在会话页切换模式只改记住的模式和侧栏内容，不导航。
- 数据：外壳和首页共用一份数据，不重复订阅。
  - 把 `SessionList.svelte` 里的 `/ws/sessions` 订阅、仓库清单加载和归档记录加载抽成 rune hook：`hooks/session-list.svelte.ts`、`hooks/repositories.svelte.ts`、`hooks/archived.svelte.ts`。
  - 这些 hook 由外壳创建，通过 context 提供给首页、聊天历史和导航树；继续使用 `ws.ts` 的 `connect` 和 `api.ts`，不新增请求或重连约定。
  - 会话页原本没有会话列表订阅；外壳挂上后多一条 `/ws/sessions` 连接。它只传会话元数据，不传 PTY 字节。
- 全局区：`WorkspaceHeader` 拆掉，品牌移到侧栏顶部，连接状态与主机名、设置、主题切换移到侧栏底部。重连提示从首页顶部条移到侧栏底部的状态文字。
- 新建弹窗上移到外壳：`SessionList.svelte` 现在持有 `creating` 状态和懒加载的 `NewSessionDialog`，入口是 `start(entry?: LaunchEntry)`。
  - 把这份状态、`start()` 和启动配置加载移到外壳，通过 context 提供给首页、工作台侧栏的“新建会话”和树中仓库的“+”。
  - 会话页上也能打开同一个弹窗，不另写流程。
  - 首页卡片和仓库菜单改为调用同一个 `start()`，行为不变。
- 组件与样式：新增 `components/AppShell.svelte`、`ModeSwitch.svelte`、`SidebarFooter.svelte`。
  - 手机端使用现有 `Drawer`（左侧方向），打开按钮放在各页面的顶栏左侧；终端顶栏的返回按钮在窄屏换成这个按钮（见 `terminal-header` 的 delta）。
  - 模式切换用两个 `Button variant="ghost" size="icon-touch"`，按钮固定 variant、只切换 `aria-pressed`，符合线条优先的约定。
  - 不引入 shadcn-svelte 的 `sidebar`：它自带一套布局状态和 CSS 变量，与现有断点显隐方式重复。
  - 收起状态存在 `localStorage` 并用 Zod 校验；显隐只用 `max-sm:`/`sm:` 断点。

### 8. 聊天历史

- 列表合并运行中的混合会话与 `GET /api/hybrid-sessions` 的归档记录；同一会话 ID 同时出现时以运行中的为准。排序键：运行中取 `lastActiveAt`，归档取 `updatedAt`。
- 状态映射（与导航树共用）：
  - 等待（琥珀色）：`attention` 为 question/permission，或 `status: waiting`。
  - 工作中（绿色）：`hybrid.work` 非空，或 `status: running`。
  - 空闲（中性色）：`status: idle`。
  - 已结束（中性弱化）：`status: exited`，或归档记录。
- 组件：`components/ChatHistory.svelte`。行使用 `ListButton` 的 `navigation` 变体，以 `aria-current` 标记当前项；agent 图标用 `LaunchConfigIcon`。

### 8.1 侧栏中修改标题

- 复用 `components/RenameSessionDialog.svelte`（`PATCH /api/sessions/{id}`，`name` 为空时恢复自动标题），不改接口，也不改服务端规则。对话框只在外壳中挂载一个实例，由聊天历史和导航树的行按钮设置要修改的会话；它与首页卡片里各自挂载的实例是同一个组件。
- 新标题经 `/ws/sessions` 的列表事件回到共享的会话列表，侧栏、首页卡片和会话页顶栏一起更新，侧栏自己不保存标题。
- 行按钮用 `ListButton` 的 `file-tree-action` 变体（铅笔图标）。导航树的行本来就为它留了右侧空间；聊天历史的 `navigation` 行加右内边距留出同样的空间，避免触屏上常驻的按钮盖住状态文字。
- 只对运行中的会话显示。归档记录没有对应的运行中会话，`PATCH /api/sessions/{id}` 不适用，因此不提供。

### 9. 工作台导航树

- 取代 `RepositorySidebar`，组件 `components/WorkbenchTree.svelte`。分组与排序复用 `homeWorktrees.ts`、`homeLayout` 现有的规则，保证树与主区域顺序一致。
- 选择视图：现在的仓库选择保存在 `localStorage` 的 `homeRepository`，只有 `SessionList` 读写。改为外壳中的共享状态（仍落在同一个键上，兼容已有取值），树和首页一起读写。
- 工作树跳转：导航到 `/#wt-<worktreeId>`，首页在分组渲染后滚动到对应锚点并聚焦标题；锚点不存在时停在仓库视图顶部。
- 展开状态按节点 ID 存在 `localStorage`；当前会话所在路径自动展开，不写回存储。
- 键盘：`role="tree"`，用 roving tabindex 实现方向键导航；树的节点只占一个 Tab 停靠点，焦点所在行的“+”按钮是紧随其后的下一个停靠点。
- `RepositorySidebar.svelte` 在切换后删除；其中的计数逻辑移到树节点。
- 仓库行的“+”：调用外壳的 `start({ kind: "new-worktree", repositoryId })`，与 `RepositoryGroup` 菜单中的“新建工作树…”是同一个入口；禁用条件也相同（仓库不可用或 Git 不可用时不显示）。
  - 按钮用 `ListButton` 的 `file-tree-action` 变体：悬停或获得焦点时显示，触屏上常驻；行加 `group/row`，按钮在行内有自己的 Tab 停靠点。
- 打开会话时的模式：外壳的 `openSession(session)` 统一处理树的会话行和工作台主区域的会话卡片。
  - 读取 `session.hybrid?.side`：`acp` 时把模式设为聊天，否则保持工作台；`dormant` 不影响判断。之后导航到 `/s/{id}`。
  - 会话页的默认视图沿用现有逻辑：聊天端显示聊天，终端端显示终端，因此两种模式下右侧展示的内容都与所在端一致。
  - 只在用户点击打开时判断一次；会话页内切换端、直接打开地址或刷新页面都不改变模式。

## Risks / Trade-offs

- [侧栏在会话页占用终端/聊天宽度] → 桌面端可收起并记住；手机端默认隐藏为抽屉。
- [首页改版面大，老用户找不到原入口] → 第一次使用默认进入工作台，主区域内容与操作不变；只有顶栏的按钮搬到了侧栏。
- [会话页多一条 `/ws/sessions` 连接] → 只传元数据，体积小；外壳统一持有，页面之间切换不重连。
- [`SessionList.svelte` 数据逻辑抽离有回归风险] → 先抽 hook 并让首页改用，跑现有首页的 Vitest 和 E2E，再挂外壳（tasks 4.1 在 4.2 之前）。
- [编排耗时较长（agent 启动可能十几秒），请求一直挂着] → 处理器不依赖客户端连接；前端超时后用同一请求 ID 重试即可拿到结果。
- [配置项清单过时（agent 升级后选项变化）] → 启动后以 agent 实际报告为准，失效值只产生 warning；下一次会话会刷新清单。
- [失败遗留 worktree] → 错误体给出 worktree，并提供“在此 worktree 新建会话”入口；删除仍走现有的删除确认流程。
- [初始化指令过长挤占 agent 上下文] → 4000 字符上限，输入框上方提示让用户知道会附带。

## Migration Plan

新增字段都是可选的：旧的 `repositories.json` 直接兼容，`agent-options.json` 不存在时为空。回滚到旧版本时，旧版会忽略未知的 `initInstruction` 字段；如果旧版会在保存时丢掉该字段，指令会丢失，但不影响其他功能。
