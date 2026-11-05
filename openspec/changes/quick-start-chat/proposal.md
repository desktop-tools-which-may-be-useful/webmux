# Proposal

## Why

从首页开始一次 agent 编码任务现在要走四步：

1. 打开新建会话弹窗；
2. 选仓库并填写 worktree 分支；
3. 选启动配置并指定聊天端；
4. 等聊天就绪后再输入第一条消息。

Codex、Claude Code 网页版的做法是在一个输入框里写任务、选仓库和 agent，按发送即可开工。WebMux 已经具备仓库登记、worktree 创建、混合会话和聊天提示这些能力，只差一个把它们串成一步的入口。

另外，每个仓库常有一句固定的开场要求（例如“先读 AGENTS.md，改完跑 make check”），现在只能每次手动粘贴。

聊天入口加进来之后，首页、快速开始页和会话页需要一套统一的导航。现在会话页是全屏的，要换会话只能先回首页，而首页的仓库筛选只在宽屏出现。改成侧栏外壳、在左上角切换聊天和工作台两种模式后，任何页面都能直接跳到其他会话。

## What Changes

- 新增“快速开始聊天”页面（路由 `/new`），作为聊天模式的主区域：底部是聊天式输入框，下方一行放仓库和基准分支，以及 agent、该 agent 的会话配置（模型、思考强度、模式等）和 WebMux 权限模式；窄屏下 agent 与配置收进输入卡片。
- 按发送后，服务端一次完成：在所选仓库新建 worktree（自动生成分支名和路径）、以聊天端启动所选 agent、应用所选配置、把首条消息作为第一轮提示发送。成功后在当前页进入该会话的聊天视图。失败时停在原页面，保留输入文字，并说明失败的步骤和已创建的资源。
- 新增服务端接口 `POST /api/quick-starts`，请求带 `clientRequestId`，同一请求重复提交不会重复创建 worktree 或会话。
- 服务端记住每个支持聊天端的启动配置最近一次得到的 agent 配置项清单（名称、类别、可选值），供快速开始在 agent 启动前展示配置。清单还没有时，配置区说明“首次启动后可选”，并使用 agent 默认值。
- 仓库可设置“初始化指令”：添加仓库时可以填写，之后可在仓库菜单里修改或清空。快速开始时，初始化指令与用户输入合并为第一条消息，初始化指令在前，用空行分隔。输入框上方提示本次会附带初始化指令，点击可查看全文。
- 主界面改为左侧侧栏布局，左上角切换两种模式（参考 Claude Code 网页版）：
  - 聊天模式：侧栏是聊天历史（运行中和已归档的聊天会话，按最近活动平铺，显示 agent 图标和状态），主区域是快速开始。
  - 工作台模式：侧栏是“仓库 → worktree → 会话”导航树，取代现在的仓库筛选列表；主区域是现有的会话总览，内容和操作不变。
  - 记住上次的模式，首次使用进入工作台。会话页的侧栏跟随当前模式，在会话页切换模式只换侧栏。
  - 首页顶栏移除：连接状态与主机名、设置、主题切换移到侧栏底部。桌面端侧栏可收起成窄栏；手机端从页面顶栏左侧按钮打开抽屉，终端顶栏窄屏的返回按钮换成这个按钮。
  - 侧栏中运行中会话的行可以“修改标题”：打开首页卡片已有的同一个对话框，规则与 US-C15 相同（手动标题优先、留空恢复自动标题、非法名称拒绝），新标题经会话列表同步到各处。
- 新增 Storybook 原型 `Workspace/Quick start`：两种模式的外壳、快速开始的各种状态、带侧栏的会话页、手机抽屉，以及带初始化指令字段的添加仓库弹窗。

## Capabilities

### New Capabilities

- `quick-start-chat`：从一条任务描述一步创建 worktree、启动聊天 agent 并发送首条消息；包括页面交互、选择记忆、配置预选、幂等提交、失败报告和配置清单记忆。
- `app-shell`：首页、快速开始页和会话页的侧栏外壳，包括两种模式的切换与记忆、会话页侧栏跟随模式、侧栏底部全局入口、桌面收起和手机抽屉。
- `chat-sidebar`：聊天模式的侧栏内容，包括列表内容与排序、行状态和归档恢复。
- `workbench-navigation`：工作台模式的“仓库 → worktree → 会话”导航树，包括节点行为、展开与计数、键盘与读屏。

### Modified Capabilities

- `repository-registry`：仓库登记新增可选的初始化指令，可在添加时设置、之后修改或清空，随仓库清单持久保存。
- `terminal-header`：窄屏顶栏常驻的“返回会话列表”换成“打开侧栏”；宽屏返回按钮回到当前模式的主页。

## Impact

- 后端：
  - `internal/workspace` 的登记记录与仓库请求新增初始化指令。
  - 新增快速开始编排（worktree → 混合会话 → 配置 → 首条提示），复用 `workspace.CreateWorktree`、`workspace.CreateSession`、`hybrid.Service.Prompt`。
  - `internal/hybrid` 记录每个启动配置的配置项清单；这份新的持久状态需要在 `internal/statefile` 目录登记。
- 接口：`internal/contract`、`internal/wire`、`internal/api` 新增 `QuickStartRequest`/`QuickStartResult`、`PATCH /api/repositories/{id}`，以及启动配置的配置项清单字段；`web/src/schemas.ts` 同步更新，并重新录制 `internal/api/testdata/wire`。现有会话、混合会话和聊天接口的字段与语义不变。
- 混合模式：不受影响。快速开始只是组合现有的 worktree 创建、混合会话创建（`startSide: acp`）、设置配置和发送提示；生成的是标准混合会话，切换、休眠、恢复、关闭和归档都按 `hybrid-sessions` 的规则进行。聊天侧栏只读取现有数据。
- 前端：
  - 新增 `pages/QuickStart.svelte` 与 `/new` 路由；输入框和配置控件复用 `ChatConfig`、`ChatSelect`、`ChatPermissionMode`。
  - `AddRepositoryDialog` 增加初始化指令字段，并新增修改指令的仓库菜单项。
  - 新增 `AppShell`（含 `ModeSwitch`、`SidebarFooter`）、`ChatHistory`、`WorkbenchTree`；移除 `WorkspaceHeader` 和 `RepositorySidebar`。`App.svelte` 为首页、快速开始页和会话页包上外壳。
  - 把 `SessionList.svelte` 的会话列表订阅、仓库清单和归档记录加载抽成共享 hook，由外壳持有；不新增接口。
  - `.terminal-page` 按侧栏宽度偏移；`TerminalHeader` 窄屏的返回按钮换成打开侧栏按钮。
- 文档：在 `docs/user-stories/` 新增快速开始、侧栏与模式切换、聊天历史、工作台导航树和仓库初始化指令的故事，更新索引与覆盖表。
