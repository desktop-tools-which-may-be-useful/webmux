## Why

当前新建会话对话框用多层分段控件组织表单：

- 位置用“直接目录 / 已有工作树 / 新工作树”Tab 区分；
- 命令来源用“预先输入 / 手动填写”Tab 区分；
- 混合配置还有第三层“终端 / 聊天”分段控件。

新工作树模式下字段纵向堆叠，小屏必须滚动才能找到提交按钮，常驻说明文字多达五处以上。最常见的操作——在当前仓库的主工作区用上次的启动方式开一个会话——仍要走完整张表单。

首页已经有“当前选中的仓库”，浏览器也已记住上次启动配置，但对话框没有把两者作为一等默认值呈现：用户打开就能创建。需要把对话框重构为“一句话汇总”式单页，让默认值就是用户想要的结果。

## What Changes

- 新建会话对话框改为汇总式单页：主体只有“在 [位置] 运行 [命令]”两个选择行，移除“启动位置”与“启动命令来源”两层 Tab。
- 位置选择器合并最近目录、当前仓库及其他仓库的工作树、直接输入/浏览目录，以及“新建工作树…”入口；选择新建工作树时在原位展开精简的分支区域，起始提交与存放路径折叠到“高级”。
- 命令选择器合并保存的启动配置与“自定义命令…”，选中自定义时才出现命令输入框；选中混合配置时在同一行显示“终端 / 聊天”起始端。
- 默认位置固定优先级：显式上下文入口 > 主页当前选中的仓库（其可用主工作区）> 最近目录 > 服务默认目录；对话框内的选择不反向改变主页选中状态。
- 默认命令沿用并强化“最近一次启动方案”：打开即填入上次的启动配置或自定义命令及混合起始端，无需任何点击即可提交；记住项失效时不静默替换。
- 会话名称、起始提交、存放路径、使用已有分支等低频项收进“高级”；常驻说明改为按需提示，底部固定显示“将在 X 运行 Y”汇总与主按钮。
- 新建工作树与分屏入口复用同一对话框：前者预选“新建工作树…”并保留“创建后启动会话”开关，后者额外提供分屏方向并预填当前会话目录。
- 请求体与后端契约不变。浏览器本地键 `lastLaunch`、`customCommand`、`hybridStartSide`、`recentDirs` 的格式保持兼容，不做迁移。

## Capabilities

### New Capabilities

- `new-session-dialog`：新建会话对话框的布局、位置与命令选择、默认值来源（主页选中仓库、最近启动方案）、新建工作树与分屏变体、提交与可访问性行为。

### Modified Capabilities

- `launch-configs`：“新建会话页选择启动配置”由“预先输入 / 手动填写”两个 Tab 改为单一命令选择器；请求内容、`lastLaunch` / `customCommand` 记忆规则与失效处理保持。
- `workspace-overview`：“提供全局和上下文创建入口”把预选行为细化为带优先级的默认值规则，并把“表单允许选择”改为对话框内的位置选择器。

## Impact

- 前端：重写 `web/src/components/NewSessionSheet.svelte`，并按位置选择、命令选择、工作树字段拆分子组件；调整 `WorktreeCreateFields.svelte` 的折叠结构；`web/src/pages/SessionList.svelte` 与 `web/src/pages/Terminal.svelte` 的调用参数随之更新（含向对话框传递主页选中仓库）。
- 测试：`web/test/browser/NewSessionSheet.test.ts`、`WorktreeCreateFields.test.ts`，以及 `web/test/e2e` 中依赖 Tab 文案的 `session-creation`、`launch-configs`、`hybrid`、`worktrees`、`lifecycle`、`page-stability` 用例与 `fixtures.ts`。
- 文档：`docs/user-stories/access-and-sessions.md` 的 US-B01、US-B03、US-B06、US-B08 及相关索引、`docs/usage.md`；UI PR 需附亮/暗、桌面/移动截图。
- 不涉及：后端、`internal/contract`、wire 快照、启动配置的服务端存储与设置页、按仓库分别记忆启动方案、会话名称记忆、命令面板式入口。
