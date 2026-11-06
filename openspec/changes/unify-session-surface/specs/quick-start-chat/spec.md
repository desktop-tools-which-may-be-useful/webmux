## MODIFIED Requirements

### Requirement: 启动前展示 agent 会话配置

选定 agent 后，系统 SHALL 在输入框下方展示该启动配置最近一次聊天会话报告的配置项（如模型、思考强度、模式）及其可选值，默认值为该次会话最后使用的值。

- 还没有记录时，配置区 SHALL 说明“首次启动后可选”。
- 页面显示的配置值 SHALL 就是新会话实际得到的值：会话按 argv 播种配置之后，系统把显示的每一项（预选值或用户改过的值）与播种结果比较，对不同的项应用显示的值。
- 权限模式由 WebMux 提供，始终可选。
- 页面需要探测配置项时 SHALL 调用 `POST /api/launch-configs/{id}/options {repositoryId}`；需要在会话创建前暂存附件时 SHALL 调用 `POST /api/uploads`。页面 SHALL NOT 调用 `/api/quick-starts/options` 或 `/api/quick-starts/uploads`。

#### Scenario: 有配置清单
- **WHEN** 用户选择以前用过的 agent，并把“思考”改为 high
- **THEN** 新会话启动后、首条消息发送前，思考配置已是 high

#### Scenario: 没有配置清单
- **WHEN** 用户选择一个从未以聊天端启动过的 agent
- **THEN** 配置区只显示权限模式和“首次启动后可选”说明，发送仍可用，会话按 argv 播种和 agent 默认值配置

#### Scenario: 记住的配置值已失效
- **WHEN** 预选的配置值不在 agent 本次启动报告的可选值内
- **THEN** 系统保留 agent 的当前值，照常发送首条消息，并在聊天中显示一条说明哪项配置未能应用的提示

### Requirement: 一次发送完成创建与开工

用户发送非空消息后，页面 SHALL 发出一次 `POST /api/sessions` 请求，携带 `newWorktree: {repositoryId, mode: "new", base}`（省略 `branch`）、所选启动配置的 `configId`、`view: "chat"`、`prompt`、`attachments`、`agentConfig`、`permissionMode` 与 `clientRequestId`。
系统 SHALL 依次完成：在所选仓库基于基准分支新建 worktree（自动生成不冲突的分支名和路径）、以聊天视图创建混合会话、应用预选配置和权限模式、把首条消息作为第一轮提示发送。首条提示由服务端发送，浏览器在过程中断开也不影响。全部成功后，页面在当前标签页进入新会话的聊天视图。

#### Scenario: 成功开工
- **WHEN** 用户输入“修复首页排序”，选择仓库 webmux 和 agent Claude Code 后按发送
- **THEN** webmux 多出一个基于基准分支的新 worktree，新会话以聊天视图运行在其中，第一条用户消息已发出，页面进入该会话的聊天视图

#### Scenario: 发送过程中关闭页面
- **WHEN** 用户按发送后、页面跳转前关闭了浏览器
- **THEN** worktree、会话和首条消息照常完成，用户稍后在首页的该仓库下能看到正在工作的会话

#### Scenario: 会话名称
- **WHEN** 快速开始创建会话
- **THEN** 会话不带显式名称，标题按现有规则跟随 agent 报告的标题

### Requirement: 快速开始只是混合会话的另一个入口

快速开始创建的会话 SHALL 是标准混合会话，与通过 `POST /api/sessions` 以“新建 worktree + 混合配置 + 聊天视图”创建、再手动发送第一条消息的结果一致；快速开始 SHALL NOT 有独立的服务端接口。会话列表、会话详情和混合会话记录 SHALL NOT 带有“由快速开始创建”的字段或其他差异；
之后的切换、恢复、关闭、归档和删除 worktree 都按现有规则进行。

#### Scenario: 切到终端
- **WHEN** 用户在快速开始创建的会话中切到终端，再切回聊天
- **THEN** 切换按 `hybrid-sessions` 的规则进行，配置与聊天记录的表现和其他混合会话相同

#### Scenario: 首页与归档
- **WHEN** 快速开始创建的会话运行一段时间后被关闭
- **THEN** 它运行时出现在首页对应仓库的 worktree 分组里，关闭后作为归档记录出现在原处，可以按现有流程恢复

#### Scenario: agent 需要登录
- **WHEN** 首条提示发出后 agent 返回 `auth_required`
- **THEN** 快速开始视为已完成并进入聊天视图，由聊天视图按现有规则显示提示和“切到终端登录”按钮

### Requirement: 重复提交不产生重复资源

每次快速开始的 `POST /api/sessions` 请求 SHALL 带客户端生成的 `clientRequestId`。同一请求 ID 在第一次执行进行中或完成后 10 分钟内重复提交时，系统 SHALL 等待并返回第一次执行的结果，不得再创建 worktree、会话或提示。超出这个窗口的同 ID 请求按新请求执行。
发送期间输入框和发送按钮禁用，并显示“正在创建 worktree 并启动 agent”的进行中状态。

#### Scenario: 网络重试
- **WHEN** 首次请求已在服务端执行，响应丢失，前端用同一请求 ID 重试
- **THEN** 服务端返回同一个会话，仓库中只有一个新 worktree

### Requirement: 失败时报告进度并保留输入

任一步骤失败时，系统 SHALL 停止后续步骤，页面保留在快速开始并保留输入文字，按 `POST /api/sessions` 失败体的 `step`（`precheck`、`worktree`、`session`、`config`、`prompt`）与原因说明失败的步骤。
已完成步骤创建的资源 SHALL 保留且不自动删除：已创建的 worktree（失败体的 `worktree`）在错误中可直接打开；会话已启动但首条提示发送失败时（失败体含 `sessionId`），提供进入该会话的入口，文字留在输入框中供手动发送。

#### Scenario: 创建 worktree 失败
- **WHEN** 基准分支已被删除，worktree 创建失败
- **THEN** 页面显示“创建 worktree 失败”和 Git 给出的原因，没有启动会话，输入文字保留

#### Scenario: agent 启动失败
- **WHEN** worktree 已创建，但 agent 的 ACP 进程在初始化阶段退出
- **THEN** 页面说明 agent 启动失败及原因，并显示已创建的 worktree 路径和“在此 worktree 新建会话”入口；worktree 不被删除

### Requirement: 快速开始沿用安全边界

快速开始使用的 `POST /api/sessions` SHALL 使用现有认证和 Origin 校验。仓库、启动配置、基准分支和配置项都按标识或数据传递，不拼接成命令；快速开始页面的请求 SHALL 只用 `configId` 指定 agent，SHALL NOT 携带 `cmd` 或 `env`；
`view` 为 `chat` 时服务端 SHALL 要求 `configId` 指向开启了聊天视图的已保存启动配置。能在创建前判断的条件（仓库可用、启动配置勾选了混合模式、第三方连接器已安装）SHALL 在创建 worktree 之前校验。

#### Scenario: 未授权请求
- **WHEN** 未通过认证或 Origin 不被允许的客户端提交快速开始
- **THEN** 请求被拒绝，不创建 worktree、会话或提示

#### Scenario: 未勾选混合模式的启动配置
- **WHEN** 请求以 `view: "chat"` 指定的启动配置没有勾选混合模式
- **THEN** 请求被拒绝并说明原因，不创建 worktree

#### Scenario: 连接器未安装
- **WHEN** 所选 agent 需要第三方 ACP 连接器但尚未安装
- **THEN** 请求按 `hybrid-sessions` 的规则返回 `connector_missing`，页面显示安装指引，不创建 worktree
