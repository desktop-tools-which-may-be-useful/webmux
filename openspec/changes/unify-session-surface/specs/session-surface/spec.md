## Purpose

规定所有会话共用的对外契约：一个会话只有一种形状，终端与聊天是会话的视图而不是会话种类，
输入、中断、切换、恢复与结束都作用于会话本身，由服务端按当前视图分派，使脚本、CLI 与 Web 端不必先判断会话种类再选择接口。

## ADDED Requirements

### Requirement: 会话信息的统一形状

`SessionInfo` SHALL 对所有会话使用同一形状，SHALL NOT 包含 `hybrid` 对象，并 SHALL 包含以下字段：

- `state`：`active`、`closed` 或 `interrupted`。普通会话 SHALL 恒为 `active`，包括进程已退出、`status` 为 `exited` 的普通会话。
- `views`：会话支持的视图。普通会话为 `["terminal"]`；用开启了聊天视图的启动配置创建的会话为 `["terminal","chat"]`。
- `view`：最近一次落定的视图，取 `terminal` 或 `chat`。切换进行中 SHALL 仍为来源视图。
- `transition`：切换进行中时为 `{to, phase}`，`to` 为目标视图，`phase` 为切换阶段；其他时候为 `null`。
- `agent`：`views` 含 `chat` 时为 `{name, title, icon, sessionId, work}`，`sessionId` 在尚未得知时为 `null`，`work` 只在回合进行中非空；否则为 `null`。
- `archive`：只在 `state` 为 `closed` 或 `interrupted` 时出现，为 `{configMissing, previousAgentSessionIds, activity, queuedCount, interruptedAt, updatedAt}`。

`state` 为 `closed` 或 `interrupted` 的会话信息中，运行时字段 SHALL 取零值：`generation` 为空字符串、`status` 为 `exited`、`exitCode` 为 `null`、
`clients` 为 0、`cols` 与 `rows` 为 0、`preview` 为空字符串、`listeningPorts` 为空数组，`lastActiveAt` SHALL 等于 `archive.updatedAt`。

#### Scenario: 普通会话

- **WHEN** 客户端读取一个用自定义命令创建的会话
- **THEN** 会话信息含 `state: "active"`、`views: ["terminal"]`、`view: "terminal"`、`transition: null`、`agent: null`，不含 `archive` 与 `hybrid`

#### Scenario: 切换进行中

- **WHEN** 一个处于终端视图的会话正在切到聊天，处于中断阶段
- **THEN** 会话信息含 `view: "terminal"` 与 `transition: {to: "chat", phase: "interrupting"}`

#### Scenario: 已归档会话

- **WHEN** 客户端读取一个已关闭的带聊天视图的会话
- **THEN** 会话信息含 `state: "closed"`、`status: "exited"`、`generation: ""`、`clients: 0` 与 `archive`，`lastActiveAt` 等于 `archive.updatedAt`

### Requirement: 视图命名

所有 HTTP 请求与响应、WebSocket 帧与 CLI 输出 SHALL 用 `terminal` 与 `chat` 表示视图，SHALL NOT 出现 `tui`、`acp` 或 `switching` 作为视图取值。
表示视图的字段 SHALL 命名为 `view`（或在切换请求中为 `to`），SHALL NOT 使用 `side` 或 `startSide`。

#### Scenario: 旧取值被拒绝

- **WHEN** 客户端以 `{"configId": "...", "startSide": "acp"}` 创建会话
- **THEN** 服务端以 400 拒绝未知字段 `startSide`，不创建会话

#### Scenario: 终端移动帧

- **WHEN** 已连接终端 WebSocket 的会话切到聊天
- **THEN** 终端连接收到的移动通知取值为 `chat`，并以关闭码 4409 结束

### Requirement: 按状态列出会话

`GET /api/sessions` SHALL 接受可选查询参数 `state`，取值为 `active`、`closed`、`interrupted`、`archived`（等于 `closed` 与 `interrupted`）或 `all`，可重复，缺省为 `active`。
`active` 会话的排序 SHALL 与原有列表一致；`closed` 与 `interrupted` 会话 SHALL 按 `archive.updatedAt` 倒序；`all` SHALL 先列 `active` 再列其他。
未知的 `state` 取值 SHALL 返回 400。`GET /api/sessions/{id}` SHALL 对任何状态的会话返回会话信息，ID 不存在时返回 404。

`/ws/sessions` 的快照与事件 SHALL 只描述 `active` 会话；会话变为 `closed` 或 `interrupted` 时 SHALL 发送 `removed` 事件。

系统 SHALL NOT 提供 `GET /api/hybrid-sessions` 与 `GET /api/hybrid-sessions/{id}`。

#### Scenario: 缺省只列运行中

- **WHEN** 客户端请求 `GET /api/sessions`，服务中有两个运行中会话和一个已关闭的会话
- **THEN** 响应只含两个运行中会话

#### Scenario: 列出已归档

- **WHEN** 客户端请求 `GET /api/sessions?state=archived`
- **THEN** 响应只含 `closed` 与 `interrupted` 的会话，按 `archive.updatedAt` 从新到旧排列

#### Scenario: 读取已归档会话

- **WHEN** 客户端请求一个已中断会话的 `GET /api/sessions/{id}`
- **THEN** 返回 200 与 `state: "interrupted"` 的会话信息

### Requirement: 一次完成的会话创建

`POST /api/sessions` SHALL 是唯一的会话创建接口，请求体字段为：

- 位置，三者取其一：`cwd`、`workspace`（已登记 worktree）、`newWorktree`（`{repositoryId, mode, branch, base, path}`，`mode` 取 `new` 或 `existing`）。
  `mode` 为 `new` 且省略 `branch` 时，系统 SHALL 自动生成不与已有分支或目录冲突的分支名与路径。
- 命令，两者取其一：`configId`，或 `cmd` 与可选的 `env`。
- 可选：`name`、`labels`、`view`、`prompt`、`useInitInstruction`、`attachments`、`agentConfig`、`permissionMode`、`clientRequestId`。

`view` 缺省为 `terminal`；`view` 为 `chat` 时 `configId` SHALL 指向开启了聊天视图的启动配置，否则 SHALL 返回 400。
`agentConfig` 与 `permissionMode` SHALL 只在 `view` 为 `chat` 时接受；`attachments` SHALL 要求 `prompt` 非空（在终端视图将移入会话上传目录后的路径附在输入消息后）。
`useInitInstruction` SHALL 要求位置为 `workspace` 或 `newWorktree`。

系统 SHALL 按以下顺序执行，并在任一步失败时停止后续步骤：预检、新建 worktree（有 `newWorktree` 时）、创建会话、应用 `agentConfig` 与 `permissionMode`、发送 `prompt`。
`prompt` 在聊天视图作为第一条提示由服务端发送；在终端视图作为输入写入并提交。
成功时 SHALL 返回 201 与 `{session, worktree, configWarnings}`，`worktree` 只在本次新建时出现，`configWarnings` 列出 agent 没有接受的配置项。

失败时 SHALL 返回 `{error, step, reason, eligibility, connector, worktree, sessionId}`（未设置的字段省略），`step` 取
`precheck`、`worktree`、`session`、`config` 或 `prompt`，HTTP 状态码按失败原因映射；已完成步骤创建的资源 SHALL 保留，SHALL NOT 回滚。

带 `clientRequestId` 的请求在第一次执行进行中或完成后 10 分钟内重复提交时，系统 SHALL 等待并返回第一次执行的结果，SHALL NOT 再创建 worktree、会话或提示。

系统 SHALL NOT 提供 `POST /api/quick-starts`。

#### Scenario: 新 worktree 中直接开始聊天

- **WHEN** 客户端以 `newWorktree: {repositoryId: "r1", mode: "new"}`、聊天视图启动配置、`view: "chat"`、`prompt: "修复 lint"` 与 `agentConfig: {"model": "sonnet"}` 创建会话
- **THEN** 系统新建分支名自动生成的 worktree，在其中以聊天视图启动会话，设置模型后发送提示，返回 201 与会话信息、新 worktree 和空的 `configWarnings`

#### Scenario: 首条提示失败

- **WHEN** 会话已创建，但发送首条提示失败
- **THEN** 返回失败体，`step` 为 `prompt`，含 `worktree` 与 `sessionId`；worktree 与会话都保留

#### Scenario: 终端视图拒绝聊天专有字段

- **WHEN** 客户端以 `view: "terminal"` 和 `permissionMode: "auto"` 创建会话
- **THEN** 返回 400，不创建任何资源

#### Scenario: 重复提交

- **WHEN** 同一 `clientRequestId` 在 5 分钟内提交了两次
- **THEN** 只创建一个 worktree 和一个会话，两次都得到同一结果

### Requirement: 创建前的上传与选项探测

系统 SHALL 提供 `POST /api/uploads`，暂存创建会话时使用的附件，返回值与会话上传接口相同；
`POST /api/launch-configs/{id}/options`，请求体为 `{repositoryId}`，SHALL 让该启动配置的 agent 在仓库主工作区报告它提供的配置项，响应为配置项数组。
启动配置未开启聊天视图时 SHALL 返回 409 `no_chat_view`。
系统 SHALL NOT 提供 `POST /api/quick-starts/uploads` 与 `POST /api/quick-starts/options`。

#### Scenario: 探测配置项

- **WHEN** 客户端对一个开启了聊天视图的启动配置请求 `POST /api/launch-configs/{id}/options`
- **THEN** 返回该 agent 的配置项、类型、当前值与可选值

#### Scenario: 终端启动配置

- **WHEN** 客户端对未开启聊天视图的启动配置请求选项
- **THEN** 返回 409，`reason` 为 `no_chat_view`

### Requirement: 按视图分派的输入

系统 SHALL 提供 `POST /api/sessions/{id}/input`，请求体为 `{clientRequestId, text, submit, keys, attachments, mode}`，并按会话当前视图分派：

- 终端视图：`text` 与 `submit`（缺省 `false`，为 `true` 时追加提交键）作为一次带回执的提交写入终端，随后按顺序写入 `keys`
  （`backspace`、`ctrl-c`、`ctrl-d`、`down`、`enter`、`esc`、`left`、`right`、`tab`、`up`）。服务端 SHALL 在提交写入完成后答复 200 `{view: "terminal"}`；
  提交部分写入、失败或会话已无运行进程时 SHALL 返回 409 `input_failed` 并附回执状态。输入 SHALL 像浏览器输入一样取得终端控制，控制者标签为 `script`，
  调用者在会话内时附上来源会话 ID。带 `attachments` 或 `mode` 时 SHALL 返回 409 `wrong_view`，不写入任何内容。
- 聊天视图：`text` 与 `attachments` 作为提示，`mode` 取 `auto`（缺省，空闲时发送，否则排队）或 `steer`（插入进行中的回合）。
  响应 SHALL 为 `{view: "chat", queued, itemId, turnId, promptId}`，含义与聊天提示队列一致。`submit` SHALL 被忽略；带 `keys` 时 SHALL 返回 409 `wrong_view`。

切换进行中 SHALL 返回 409 `switching`；`state` 不是 `active` 时 SHALL 返回 409 `not_active`；`text` 为空且没有 `keys` 与 `attachments` 时 SHALL 返回 400。
同一 `clientRequestId` 的重复请求 SHALL 只生效一次。

系统 SHALL NOT 提供 `POST /api/sessions/{id}/chat/prompts`。

#### Scenario: 向终端视图输入命令

- **WHEN** 客户端向一个 shell 会话发送 `{text: "make test", submit: true}`
- **THEN** 终端收到 `make test` 与回车，响应为 `{view: "terminal"}`，控制指示器显示 `script`

#### Scenario: 向聊天视图发送提示

- **WHEN** 客户端向空闲的聊天视图会话发送 `{text: "下一步"}`
- **THEN** 提示立即发送，响应含 `queued: false`、`turnId` 与 `promptId`

#### Scenario: 用错视图

- **WHEN** 客户端向聊天视图的会话发送带 `keys: ["esc"]` 的输入
- **THEN** 返回 409，`reason` 为 `wrong_view`，聊天中没有新提示

### Requirement: 按视图分派的中断

系统 SHALL 提供 `POST /api/sessions/{id}/interrupt`，请求体为 `{clientRequestId}`：

- 聊天视图：SHALL 取消进行中的回合；没有进行中的回合时 SHALL 不做任何事并返回 204。
- 终端视图：会话的 agent 有档案时 SHALL 写入档案定义的中断键，否则 SHALL 写入 Ctrl-C（`\x03`）。

切换进行中 SHALL 返回 409 `switching`；`state` 不是 `active` 时 SHALL 返回 409 `not_active`。
系统 SHALL NOT 提供 `POST /api/sessions/{id}/chat/cancel`。

#### Scenario: 中断聊天回合

- **WHEN** 聊天视图的会话有回合进行中，客户端请求中断
- **THEN** 回合以 `cancelled` 结束，返回 204

#### Scenario: 中断 shell 命令

- **WHEN** 一个普通 shell 会话正在运行 `sleep 100`，客户端请求中断
- **THEN** 终端收到 Ctrl-C，`sleep` 结束

### Requirement: 视图切换接口

系统 SHALL 提供 `POST /api/sessions/{id}/view`，请求体为 `{clientRequestId, to, choice, agentSessionId}`：`to` 发起切换，`choice`（`wait`、`force`、`cancel`）答复等待选择的切换，
`agentSessionId` 在候选冲突后选择 agent 会话且只在 `to` 为 `chat` 时接受。成功时 SHALL 返回 `{view, draft, dropped, unsupported}`。
`views` 不含 `chat` 的会话 SHALL 返回 409 `no_chat_view`。系统 SHALL NOT 提供 `POST /api/sessions/{id}/switch`。

#### Scenario: 切到聊天

- **WHEN** 客户端对终端视图的会话请求 `{to: "chat"}`，切换成功
- **THEN** 返回 `{view: "chat", dropped: [], unsupported: []}`，会话信息的 `view` 为 `chat`、`transition` 为 `null`

#### Scenario: 普通会话

- **WHEN** 客户端对普通 shell 会话请求切到聊天
- **THEN** 返回 409，`reason` 为 `no_chat_view`

### Requirement: 恢复、停止、移除与删除记录

- `POST /api/sessions/{id}/resume` 的请求体 SHALL 为 `{clientRequestId, view, agentSessionId, confirmWithoutEnv}`，`view` 缺省为会话归档时的视图；
  只对 `closed` 或 `interrupted` 的会话有效，对 `active` 会话 SHALL 返回 409 `not_archived`。成功时返回会话信息，`state` 为 `active`。
- `DELETE /api/sessions/{id}` SHALL 停止会话当前视图的进程：普通会话保留在运行中列表并显示为已退出；带聊天视图的会话变为 `closed` 并离开运行中列表。
- `DELETE /api/sessions/{id}?remove=1` SHALL 先停止再从运行中列表移除；带聊天视图的会话 SHALL 保留归档。
- `DELETE /api/sessions/{id}?forget=1` SHALL 隐含 `remove=1`，并删除归档记录与会话临时文件夹，SHALL NOT 删除 agent 自己的会话库；之后该 ID 在任何状态下都返回 404。

系统 SHALL NOT 提供 `DELETE /api/hybrid-sessions/{id}`。

#### Scenario: 恢复到聊天

- **WHEN** 客户端对一个已关闭的会话请求 `{view: "chat"}` 恢复
- **THEN** 会话以原 ID 回到运行中列表，`view` 为 `chat`

#### Scenario: 删除记录

- **WHEN** 客户端对一个已中断的会话请求 `DELETE /api/sessions/{id}?forget=1`
- **THEN** 该会话不再出现在 `state=all` 的列表中，`GET /api/sessions/{id}` 返回 404

### Requirement: 统一的冲突响应

所有会话、视图、输入、中断、恢复与聊天命令的 409 响应 SHALL 使用同一形状 `{error, reason, eligibility, candidates, connector, submission}`（未设置的字段省略）。
`reason` SHALL 是稳定的代码，至少包括：`switching`、`wrong_view`、`no_chat_view`、`not_active`、`not_archived`、`input_failed`、`not_eligible`、`answered`、`sent`、
`steering_unavailable`、`session_missing`、`session_ambiguous`、`connector_missing`、`occupied_external`、`config_missing`、`limit`。
`views` 不含 `chat` 的会话调用 `/api/sessions/{id}/chat/*` 或 `/ws/s/{id}/chat` 时 SHALL 返回 409 `no_chat_view`。

#### Scenario: 普通会话的聊天命令

- **WHEN** 客户端对普通会话请求 `PUT /api/sessions/{id}/chat/permission-mode`
- **THEN** 返回 409，`reason` 为 `no_chat_view`

### Requirement: 聊天通道只承载对话状态

`/ws/s/{id}/chat` 的状态对象 SHALL NOT 包含 `side`、`phase`、`agent` 或 `agentSessionId`；所在视图、切换阶段与 agent 身份 SHALL 只由会话信息提供。
时间线标记中表示视图的字段 SHALL 命名为 `view`。

#### Scenario: 切换中的聊天状态

- **WHEN** 已连接聊天通道的客户端在会话切换期间收到状态帧
- **THEN** 状态帧不含 `side` 与 `phase`，客户端从会话信息的 `transition` 得知切换阶段

### Requirement: 一次性切换

本变更的旧接口、旧字段、旧取值与旧 CLI 子命令 SHALL 在同一版本中删除，SHALL NOT 保留别名、转发或弃用期。
`launch-configs.json` 与 `hybrid-sessions.json` 的文件名、版本与键 SHALL 保持不变，使回退到旧版本时数据仍可读取。

#### Scenario: 旧接口

- **WHEN** 客户端请求 `POST /api/quick-starts`
- **THEN** 返回 404

#### Scenario: 回退

- **WHEN** 用户从本版本回退到上一版本
- **THEN** 上一版本照常读取启动配置与混合会话记录
