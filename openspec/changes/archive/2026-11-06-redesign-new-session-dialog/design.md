# Design

## Context

旧的 `NewSessionSheet.svelte`（686 行）同时承担“新建会话”“仅新建工作树”“分屏新建”三种意图，用 `intent`、`split`、`treeOnly`、`mode`、`commandSource` 多组状态交织控制显示。

调用点有两处：

- `SessionList.svelte` 通过 `start(mode, repositoryId, worktreeId)` 自行推导预选，再传入 `initialMode / initialRepositoryId / initialWorktreeId`；
- `Terminal.svelte` 传入 `initialCwd / initialWorkspace / split`。

浏览器本地已有 `recentDirs`、`lastLaunch`、`customCommand`、`hybridStartSide`。请求契约（`CreateSessionRequestSchema`、`CreateWorktreeRequestSchema`）与后端无需改变。

仓库 UI 规范要求优先复用 `$lib/components/ui`（已有 `popover`、`command`、`collapsible`、`drawer`），不得新增 eslint-disable 或扩张 `check:arch` 基线。动机见 proposal.md 的 Why。

## Goals / Non-Goals

**Goals:**

- 默认值即结果：主页选中仓库与最近一次启动方案共同决定初始状态，常见路径零点击提交。
- 一个对话框、两个选择行、一句汇总；去掉两层 Tab。
- 把超大组件拆成职责清晰、可单测的部分，状态模型不再依赖 `intent` 分支堆叠。
- 手机与桌面能力一致。

**Non-Goals:**

- 不改后端、契约、wire 快照、启动配置存储与设置页。
- 不按仓库分别记忆启动方案，不记住会话名称。
- 不做命令面板式入口或多步向导。
- 不改变工作树创建、上游刷新与冲突处理的行为（沿用 `worktree-lifecycle`）。

## Decisions

### 1. 上下文入口与默认值解析是纯函数

`web/src/new-session-defaults.ts` 定义入口类型 `LaunchEntry`：

- `{ kind: "worktree", repositoryId, worktreeId }`：工作树的“新建会话”；
- `{ kind: "new-worktree", repositoryId }`：仓库菜单的“新建工作树…”；
- `{ kind: "directory", cwd, workspace }`：分屏源会话。

`resolveInitialTarget({ entry, selectedRepositoryId, repositories, recent, defaultCwd })` 返回 `{ kind, cwd, repositoryId, worktreeId }`，实现 spec 的优先级。`worktreeId` 为空的工作树目标就是“未选择”状态：仓库保留，主按钮不可用。`SessionList` 只传入口与 `selectedRepository`，不再自行推导预选。

对话框的 prop 命名为 `entry` 而不是 `context`：`vitest-browser-svelte` 的 `render` 会把 `context` 当作 Svelte 挂载选项。

备选：保留 `mode` 枚举与 Tab，只换外观。否决：状态耦合不变，达不到“大改”的目的。

### 2. 命令来源用 `source` 三态并保持存储兼容

`source` 取 `"config" | "custom" | "unset"`，另存 `configId` 与 `command`。

`resolveInitialLaunch(lastLaunch, launchConfigs)` 的规则：

- 记住 `custom`：立即得到自定义命令，不等待配置加载；
- 记住 `config:<ID>`：加载完成后存在则选中，否则得到 `unset`；
- 没有或无法识别的记忆：第一个配置，没有配置时为自定义命令；
- 加载中且需要配置：返回 `null`，对话框暂不决定。

对话框只在用户尚未操作命令行时应用该结果；之后若所选配置从列表消失（被删除或存储变为不可用），`source` 变为 `unset`，不静默替换。这与旧代码把失效项改为自定义命令的行为有意不同，符合“不得静默替换”。存储键与格式不变，仅在创建成功后写入，无需迁移。

备选：新增 `lastSession` 键，同时存位置与命令。否决：位置默认值应跟随主页选中仓库，持久化位置会与之冲突。

### 3. 选择行：桌面浮层、窄屏就地展开

`PickerField.svelte` 是两个选择行共用的外壳：

- 桌面（`min-width: 640px`，与 `ResponsiveDialog` 同一断点）：`Popover` 内放 `Command` 列表，宽度跟随触发按钮；
- 窄屏：`Collapsible` 在抽屉内就地展开同一列表，Esc 只收起该面板并把焦点还给触发按钮。

触发按钮通过 `<Label for>` 取得“位置”“运行”作为可访问名称，当前值经 `aria-describedby` 关联。列表项共用 `ChoiceDetails.svelte`（标题、等宽命令或路径、环境变量数量），避免两个选择器各写一份。

备选：沿用 `Select`。否决：无法在同一列表中分组承载搜索、工作树与“新建工作树…”，且窄屏抽屉中会出现浮层。

### 4. 位置选择器的分组与新建工作树入口

`LocationPicker.svelte` 的列表顺序：当前仓库分组、最近目录、其他仓库分组、“其他 → 输入或浏览目录…”。

- 每个仓库分组列出工作树，并在末尾提供该仓库的“新建工作树…”。因此目标仓库由所选分组决定，对话框不再另放仓库下拉框。
- 本次已创建但父组件列表尚未刷新的工作树并入其仓库分组，失败重试与改选后仍可选回。
- 选中直接目录时，选择行下方显示工作目录输入与 `DirectoryPicker`；选“输入或浏览目录…”会聚焦该输入。

### 5. 新工作树字段就地精简

`WorktreeCreateFields.svelte` 原地调整，不再包一层新组件：

- 起始分支与新分支名称在 `sm` 以上并排；
- 起始提交与存放目录移入 `AdvancedSection`（“高级：起始提交与存放目录”，已有分支模式为“高级：存放目录”），标题旁显示短提交号与路径摘要；
- 去掉不影响提交结果的常驻说明，“尚无可用提交”仍在折叠区外显示；
- 新增可绑定的 `startBranch`，供位置行显示“新分支 ← 起始分支”。

`AdvancedSection.svelte` 同时用于对话框的“高级：会话名称”。

### 6. 组件边界

- `NewSessionDialog.svelte`：持有位置、命令、名称、错误与提交状态，负责请求与记忆写入，替代 `NewSessionSheet.svelte`（调用点同步改名，无别名）。
- `LocationPicker.svelte`、`LaunchPicker.svelte`：受控选择器，通过绑定值回传选择；`LaunchPicker` 另以 `onchoose` 告知用户已做选择。
- “仅新建工作树”与分屏仍由 `intent`、`split` 区分，差异限定在命令行是否显示、方向选择与主按钮文案。

### 7. 汇总行

底部汇总由当前位置与命令派生，如“将在 webmux · 主工作区 · main 运行 Claude Code”；位置或命令无效时改为“请选择位置”“请选择启动方式”等提示，主按钮同时不可用。汇总不设为实时播报区，避免输入命令时反复朗读；主按钮以 `aria-describedby` 关联它。

## Risks / Trade-offs

- **隐藏的细节**：低频项折叠后可发现性下降。缓解：折叠标题旁显示已填内容摘要。
- **浮层可访问性**：焦点与 Esc 分层易出错。缓解：复用设计系统的 Popover/Command；浏览器测试覆盖 Esc 只关闭选择器与键盘选择。
- **失效记忆不再静默改为自定义命令**：与旧行为不同，多一次点击。取舍：与“不得静默替换”一致，避免误用其他配置。
- **范围较大的重写**：测试依赖旧 Tab 文案。缓解：E2E 通过 `fixtures.ts` 的 `chooseLaunch`、`chooseLocation`、`useCustomCommand`、`useDirectory`、`nameSession` 统一操作对话框。
- **加载竞态**：启动配置与仓库异步到达。缓解：默认命令只在用户尚未操作命令行时应用。

## Migration Plan

无数据迁移：本地存储键格式不变，服务端无变化。回滚即还原前端组件与测试，不留兼容层。

## Open Questions

无。已作为假设记录：最近一次启动方案对所有位置通用；对话框内的位置选择不回写主页选中仓库。
