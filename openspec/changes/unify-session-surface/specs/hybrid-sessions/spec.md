## MODIFIED Requirements

### Requirement: 混合会话的创建与起始端

系统 SHALL 只在使用开启了聊天视图（勾选了混合模式）的启动配置创建会话时创建混合会话，即 `views` 为 `["terminal","chat"]` 的会话。不带配置的自定义命令和未勾选的配置 SHALL 创建普通会话，行为不变。

`POST /api/sessions` 在 `configId` 指向开启了聊天视图的启动配置时 SHALL 接受可选字段 `view`，取值为 `terminal` 或 `chat`，缺省为 `terminal`。对普通配置或自定义命令提供 `view: "chat"` SHALL 返回 400；请求体中的 `startSide` SHALL 作为未知字段返回 400。

- 从终端开始：系统 SHALL 在 PTY 中执行配置保存的 argv，会话的 `view` 为 `terminal`。
- 从聊天开始：档案声明了第三方连接器时，系统 SHALL 先检查连接器已安装，未安装时 SHALL 返回 409 `connector_missing`
  并附连接器信息，SHALL NOT 创建会话或写入混合会话记录。检查通过后，系统 SHALL 按档案启动 ACP 进程，
  依次调用 `initialize` 和 `session/new {cwd}`，记录返回的 agent 会话 ID，会话的 `view` 为 `chat`。

两种起始方式 SHALL 都写入混合会话记录。

Web 端新建会话弹层 SHALL 在选中混合配置时显示“从终端开始 / 从聊天开始”选择，默认从终端开始，并按浏览器记住上一次的选择。

#### Scenario: 从聊天开始

- **WHEN** 用户选择勾选了混合模式的 omp 配置，选择“从聊天开始”并提交
- **THEN** 会话出现在列表中，`view = "chat"`，页面显示聊天视图；PTY 中没有运行任何进程

#### Scenario: 从聊天开始但连接器未安装

- **WHEN** 用户选择 Codex 混合配置、从聊天开始并提交，服务上没有 `codex-acp`
- **THEN** 服务端返回 409 `connector_missing`，会话列表不变，没有启动任何进程

#### Scenario: 对普通配置指定起始端

- **WHEN** 客户端以未勾选混合模式的配置 ID 和 `view: "chat"` 创建会话
- **THEN** 服务端返回 400，不创建会话

### Requirement: 单写者与所在端

混合会话 SHALL 在任意时刻处于且只处于以下之一：终端视图、聊天视图、切换中。会话信息中，`view` SHALL 为最近一次落定的视图（`terminal` 或 `chat`），切换中时 `transition` SHALL 为 `{to, phase}`，其他时候为 `null`。webmux 会话 ID SHALL 在混合会话的整个生命周期中保持不变。

系统 SHALL 保证同一 agent 会话 ID 在任意时刻至多被一个由 webmux 启动的进程打开。只调用 `session/list` 的临时探测进程不算打开。进入目标视图之前，系统 SHALL 已确认原视图的进程组已经全部退出。这是硬性要求：omp 在 TUI 仍持有会话租约时，另一个进程写入同一会话，会把它分叉成一个新 ID 的会话。

会话信息 SHALL NOT 包含 `hybrid` 对象。混合会话的会话信息 SHALL 含 `state`、`views: ["terminal","chat"]`、`view`、`transition` 与 `agent: {name, title, icon, sessionId, work}`（`sessionId` 尚未得知时为 `null`）。普通会话的 `agent` SHALL 为 `null`。

- `agent.title` SHALL 是 agent 档案声明的显示名，非空；
- `agent.icon` SHALL 是 agent 档案声明的图标键，取值属于启动配置的图标目录；
- 两者在会话生命周期内 SHALL 保持不变，并在会话列表快照、列表事件与单个会话信息中一致。

#### Scenario: 切换期间不存在两个进程

- **WHEN** 会话从终端视图切到聊天视图，同时用进程表持续观察
- **THEN** 在 TUI 进程退出之前，不出现由 webmux 为该会话启动的 ACP 进程

#### Scenario: 普通会话的负载不变

- **WHEN** 客户端读取普通会话的会话信息
- **THEN** 负载中没有 `hybrid` 字段，`views` 为 `["terminal"]`、`view` 为 `terminal`、`agent` 为 `null`

#### Scenario: 会话信息带出 agent 显示名与图标

- **WHEN** 用户以 OpenCode 混合配置创建会话，客户端随后读取会话列表快照
- **THEN** 该会话的 `agent` 对象含 `name: "opencode"`、`title: "OpenCode"` 与一个属于启动配置图标目录的 `icon`，单个会话信息中的取值与之相同

### Requirement: 从终端切到聊天

收到 `POST /api/sessions/{id}/view {to: "chat"}` 且会话在终端视图时，系统 SHALL 先按 `agent-session-identity` 的资格判定确认可以切换（含连接器检查），不满足时返回 409 并附原因，不改变会话。确认可以切换后，系统 SHALL 进入切换中（`transition.to` 为 `chat`），按以下顺序执行：

1. 根据终端信号判断 TUI 是否在执行回合；在执行时，向 PTY 写入档案定义的中断键，并等待信号变为空闲；
2. 结束 TUI 进程，并等待其进程组清理完成；
3. 按档案启动 ACP 进程，调用 `initialize`，再用资格判定得到的 agent 会话 ID 与 cwd 调用 `session/load`；
4. 用记录中的配置项取值补齐 agent 未返回或回到默认值的配置（按档案判断）；记录中尚无某项取值时，使用按档案从 argv 推导的值；
5. 进入聊天视图。

切换期间，所有设备的输入 SHALL 被禁用，已连接的终端客户端 SHALL 以关闭码 4409 断开并收到“已切换到聊天视图”的原因。

#### Scenario: 空闲时切换

- **WHEN** TUI 空闲，用户点击“聊天”
- **THEN** 会话在一次切换内进入聊天视图，聊天视图回放完整历史，模型与推理强度与切换前一致

#### Scenario: 回合进行中切换

- **WHEN** TUI 正在执行回合，用户确认切到聊天
- **THEN** 系统先发送中断键并等到空闲，再退出 TUI；被中断的回合以中断结束，会话之后可以继续对话

#### Scenario: Codex 首次切到聊天

- **WHEN** argv 为 `["codex","-m","gpt-5.6-luna","-s","read-only"]` 的 Codex 混合会话在 TUI 中发出首条消息后切到聊天
- **THEN** 聊天端的 `model` 为 `gpt-5.6-luna`、`mode` 为 `read-only`，并写入混合会话记录；之后每次加载都重新应用记录中的取值

### Requirement: 中断等待超时

发送中断键后 10 秒内 TUI 仍未回到空闲时，系统 SHALL 暂停切换，在切换状态中提供“继续等待 / 强制切换 / 取消”三个选项，直到用户选择。客户端 SHALL 以 `POST /api/sessions/{id}/view` 的 `choice`（`wait`、`force`、`cancel`）答复。

- 选择“继续等待”：SHALL 再等待 10 秒。
- 选择“强制切换”：SHALL 跳过等待，直接结束 TUI 继续切换。确认文案 SHALL 说明后果：被强制结束的回合可能不完整，omp 已输出的部分可能丢失。由于 OpenCode 的 TUI 以独立服务运行（见 `hybrid-agent-profiles`），强制结束后它的回合 SHALL NOT 在后台继续。
- 选择“取消”：SHALL 保持在终端视图。

#### Scenario: 用户取消

- **WHEN** 中断后 10 秒仍未空闲，用户选择“取消”
- **THEN** 会话回到终端视图（`view` 为 `terminal`、`transition` 为 `null`），TUI 进程仍在运行，终端客户端可以重新连接

### Requirement: 从聊天切到终端

收到 `POST /api/sessions/{id}/view {to: "terminal"}` 且会话在聊天视图时，系统 SHALL 进入切换中（`transition.to` 为 `terminal`），按以下顺序执行：

1. 有回合在运行时发送 `session/cancel`，并等待回合结束；挂起的权限请求 SHALL 答复为 cancelled；
2. 把队列中未发出的消息合并后退回草稿（见 `chat-prompt-queue`）；
3. agent 支持时调用 `session/close`，然后结束 ACP 进程，并等待其进程组清理完成；
4. 在 PTY 中启动档案生成的恢复命令，开始新的运行代际；
5. 进入终端视图。

回合正在运行时，Web 端 SHALL 先请求用户确认“当前回合会被中断，已输出的内容会保留”。

#### Scenario: 往返后历史完整

- **WHEN** 会话从聊天开始并对话，切到终端继续对话，再切回聊天
- **THEN** 每一次切换后，对端都能看到此前全部对话并继续

### Requirement: 切换失败自动回滚

切换的任一步骤失败时，系统 SHALL 回到原视图：原视图为终端时按档案的恢复命令重新启动 TUI，原视图为聊天时重新启动 ACP 并 `session/load`。系统 SHALL 以可读原因通知所有设备。

回滚本身也失败时，会话 SHALL 标记为 `interrupted`，并进入可恢复列表。

#### Scenario: 加载失败

- **WHEN** 切到聊天时 `session/load` 返回错误
- **THEN** 会话回到终端视图并运行恢复后的 TUI，页面显示失败原因

### Requirement: 并发切换请求

切换进行中再次收到切换（`/view`）、输入（`/input`）、中断（`/interrupt`）、关闭或恢复请求时，系统 SHALL 返回 409，响应为 `SessionConflict`，`reason` 为 `switching`，不排队执行，并 SHALL 继续向所有设备发布当前切换进度。关闭请求 SHALL 等到切换完成或回滚后才能再次提交。

#### Scenario: 两台设备同时切换

- **WHEN** 两台设备几乎同时请求切到聊天
- **THEN** 只有一次切换执行，另一台收到 409 `switching`，两台都看到同一个进度并最终显示同一视图

### Requirement: 进程意外退出

- 会话在终端视图时，TUI 进程自行退出（包括 `/exit` 与崩溃）SHALL 使会话标记为 `interrupted`，SHALL NOT 自动切换。会话页 SHALL 保留最后的终端输出，并提供“恢复到聊天 / 恢复到终端”。
- 会话在聊天视图时，ACP 进程意外退出 SHALL 触发一次自动重连：
  - 重新启动 ACP，agent 声明了 `session/resume` 且档案允许时 SHALL 使用 `session/resume`（不回放历史，事件日志保持连续），否则 SHALL 使用 `session/load` 并以新的日志代际重新回放；
  - 当前回合 SHALL 以中断结束；重连失败时，会话 SHALL 标记为 `interrupted`。

#### Scenario: ACP 崩溃

- **WHEN** 聊天视图回合进行中，ACP 进程被 SIGKILL
- **THEN** 当前回合显示为已中断，系统自动重连一次；重连成功后可以继续发送消息

### Requirement: 聊天端进程保持运行

会话处于聊天视图且 `state` 为 `active` 时，系统 SHALL NOT 因空闲时长结束 ACP 进程，使 agent 内部的后台命令、子任务或定时器在聊天视图空闲期间保持运行；系统 SHALL 仅在切换到终端视图、显式归档（关闭）会话或服务停止时结束 ACP 进程。

#### Scenario: 聊天端空闲时保持存活

- **WHEN** 聊天视图的会话没有运行中的回合且长时间未收到用户命令
- **THEN** ACP 进程保持运行，会话始终处于 `active` 状态，只有用户显式归档会话、切换到终端视图或停止服务时才结束进程

### Requirement: 过滤活动回合中 agent 回显的用户消息片段

系统在聊天视图发起 `session/prompt` 时，已经在 `turn.start` 事件之后立即向聊天时间线写入
包含原始文本与附件元数据的用户消息。因此：

- 在活动 `session/prompt` 回合进行期间（即从发出 `session/prompt` 到收到其最终响应或取消完成之间），
  若 agent 发送 `sessionUpdate: "user_message_chunk"` 通知（例如 `agy acp` 在实时回合开头回显用户消息），
  系统 SHALL 忽略该 `user_message_chunk` 通知，SHALL NOT 在时间线中生成第二条重复的用户消息。
- 在非活动回合阶段（即执行 `session/load` 回放历史会话期间），系统 SHALL 照常将 agent 发送的
  `user_message_chunk` 通知转换为时间线中的用户消息事件。

#### Scenario: agy 在实时回合中回显 user_message_chunk

- **WHEN** 用户在 `agy` 混合会话聊天视图发送一条消息，`agy acp` 在处理 `session/prompt` 期间依次发送
  `user_message_chunk` 与 `agent_message_chunk`
- **THEN** 聊天时间线中该回合只包含一条用户消息与一条 agent 回复，不出现重复的用户消息气泡

#### Scenario: session/load 历史回放保留 user_message_chunk

- **WHEN** 会话从终端切到聊天并执行 `session/load`，`agy acp` 依次回放历史中的
  `user_message_chunk`、`tool_call` 与 `agent_message_chunk`
- **THEN** 聊天时间线完整还原历史中的用户消息、工具调用与 agent 回复
