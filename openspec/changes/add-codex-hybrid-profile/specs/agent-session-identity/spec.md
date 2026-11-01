## MODIFIED Requirements

### Requirement: 身份字段的可见范围与根进程信息

这些字段 SHALL 只保存在服务端内存中。它们 SHALL NOT 出现在普通会话的会话信息、日志或推送中；对混合会话，只有经档案解释后的 agent 名与 agent 会话 ID 出现在 `hybrid` 对象中。会话退出时 SHALL 清空这些字段。

系统 SHALL 另外记录会话 PTY 根进程的可执行文件名与当前工作目录（Linux 读取 `/proc`，macOS 使用系统进程信息），供资格判定使用。

对档案要求的混合会话，系统 SHALL 还能按需提供：

- 根进程的直接子进程的可执行文件名，供“启动器”判定使用；
- 根进程及其后代进程当前打开的普通文件的绝对路径，供档案从中解释 agent 会话 ID。

这两项 SHALL 只在资格判定需要时读取，`session` 层 SHALL NOT 理解其中任何 agent 的含义；
读取失败（权限不足、进程已退出）时 SHALL 视为没有结果，SHALL NOT 使会话出错。路径同样 SHALL NOT 出现在任何负载或日志中。

#### Scenario: 普通会话不泄露

- **WHEN** 普通 shell 会话中运行 omp 并发出带 `session_id` 的事件
- **THEN** 该会话的 REST 与 WebSocket 负载中不出现该 ID

#### Scenario: 不读取普通会话的打开文件

- **WHEN** 普通会话中运行 codex
- **THEN** 系统不读取该会话进程打开的文件

### Requirement: 切到聊天的资格

会话在 `tui` 端时，只有同时满足以下六个条件，切到聊天的操作才 SHALL 可用：

1. 终端信号识别出的 agent 有对应的档案；
2. 档案声明了第三方连接器时，连接器已安装（`acp-connector-availability`）；
3. 该 agent 进程是会话 PTY 的根进程；或者档案允许启动器、根进程是该启动器，且其直接子进程是该 agent 的原生可执行文件
   （Codex 允许 npm 安装产生的 `node` 启动器）；
4. 已得到当前 agent 会话 ID；
5. 用正确的 cwd 调用 `session/list` 能查到这个 ID。cwd 的来源由档案决定（omp 取事件中的 `cwd`，OpenCode 与 Codex 取根进程的 cwd）；
6. 这个 ID 没有被其他会话占用。

不满足时，界面 SHALL 禁用该操作并显示对应原因，依次为：

- “未识别到支持的 Agent”；
- “未安装 `<连接器显示名称>`（`<可执行文件>`）”（原因码 `connector_missing`）；
- “Agent 不是由本会话直接启动的”；
- “请先在 TUI 中发送一条消息”；
- “会话尚未保存”或“找不到会话”；
- “该会话已在另一个会话中打开”。

条件 5 SHALL 在 ID 或 cwd 变化后核对一次，并在执行切换前再核对一次；它 SHALL NOT 在每个信号到来时都执行。核对 SHALL 只使用 `session/list`，SHALL NOT 加载或恢复该会话。

#### Scenario: 首条消息之前

- **WHEN** 从终端开始的 OpenCode 混合会话还没有发送过消息
- **THEN** “聊天”按钮不可用，提示“请先在 TUI 中发送一条消息”

#### Scenario: 在 shell 中手动启动

- **WHEN** 混合配置的 TUI 退出后，用户在同一 PTY 中又通过其他程序启动了 omp（omp 不是根进程）
- **THEN** “聊天”按钮不可用，提示 Agent 不是由本会话直接启动的

#### Scenario: npm 安装的 Codex

- **WHEN** Codex 混合会话的根进程是 `node`，其直接子进程是 codex 原生可执行文件，TUI 已发出首条消息
- **THEN** 条件 3 满足，“聊天”按钮可用

#### Scenario: 未安装连接器优先提示

- **WHEN** Codex 混合会话尚未发出消息，服务上没有 `codex-acp`
- **THEN** “聊天”按钮提示“未安装 Codex ACP 适配器（codex-acp）”，而不是“请先在 TUI 中发送一条消息”

## ADDED Requirements

### Requirement: Codex 会话 ID 的来源

Codex 的终端信号不带会话 ID。对 Codex 混合会话，系统 SHALL 按以下顺序取得当前 agent 会话 ID：

1. 在根进程及其后代进程打开的文件中，查找位于 `<CODEX_HOME>/thread-writer-locks/` 下、名为 `<id>.lock`（`<id>` 为 UUID 形式）的文件。
   `CODEX_HOME` 取会话环境中的值，未设置时为服务用户主目录下的 `.codex`，比较前 SHALL 解析符号链接。
   实测 Codex 在 `/new` 后不释放旧会话的锁，`codex resume <id>` 还会另外持有一个空白会话的锁，因此系统 SHALL 按锁出现的先后判定：
   - TUI 启动后的静默期内（或尚未判定出当前会话时），本次运行恢复的会话持有锁则取它，否则恰有一个锁时取它；
   - 之后新出现恰好一个锁时，取新出现的锁（新会话）；当前会话的锁消失且只剩一个锁时取剩下的锁；
   - 其余情况（例如同时新出现多个锁）视为无法确定，转入第 2 步。
2. 打开的文件无法读取或无法确定时，使用 `session/list` 候选判定：取 cwd 下更新时间晚于本次 TUI 启动的会话
   （本会话在聊天端加载造成的更新早于这次 TUI 启动，因而被排除）。
   恰有一个候选时采用它；有多个候选时，切换请求 SHALL 返回 409，原因码 `session_ambiguous` 并附候选列表，
   界面 SHALL 复用恢复时的候选选择让用户指定，带着选中的 ID 再次发起切换。

ID 变化（例如 TUI 中执行 `/new`、`/resume`、`/fork`）SHALL 按“跟踪 agent 会话 ID 的变化”处理。
锁对应的会话尚未保存（发出首条消息之前）时，资格原因 SHALL 为“请先在 TUI 中发送一条消息”。

#### Scenario: 启动即得到 ID

- **WHEN** Codex 混合会话从终端开始，TUI 打开了 `~/.codex/thread-writer-locks/01a18715-8504-7020-9f3d-257900000000.lock`
- **THEN** `hybrid.agentSessionId` 为 `01a18715-8504-7020-9f3d-257900000000`；发出首条消息前，“聊天”按钮提示“请先在 TUI 中发送一条消息”

#### Scenario: 新会话保留旧锁

- **WHEN** TUI 执行 `/new` 后同时持有旧会话与新会话的锁
- **THEN** 当前 agent 会话 ID 变为新会话的 ID，会话页提示 ID 已变化

#### Scenario: 多个候选

- **WHEN** 无法读取进程打开的文件，同一 cwd 下本次 TUI 启动后有两个 Codex 会话被更新，用户点击“聊天”
- **THEN** 服务端返回 409 `session_ambiguous` 与两个候选，界面让用户选择后再切换

### Requirement: 外部 Codex 客户端占用

系统 SHALL 把 Codex 的 `thread_active_writer` 错误映射为原因码 `occupied_external`，界面文字为“该会话正被另一个 Codex 客户端使用”。
Codex 在 `session/load` 或 `session/resume` 时以该错误拒绝被其他 Codex 客户端（Codex App、IDE、webmux 之外的终端）占用的会话：

- 切到聊天时出现，SHALL 按“切换失败自动回滚”回到终端端，SHALL NOT 新建或分叉会话；
- 恢复到聊天或从休眠唤醒时出现，请求 SHALL 返回 409 `occupied_external`，会话保持原状态。

用户文档 SHALL 说明：Codex 能发现 webmux 之外的占用，而其他 agent 不能。

#### Scenario: 外部客户端占用

- **WHEN** 用户在 webmux 之外打开了同一 Codex 会话，然后在混合会话中点击“聊天”
- **THEN** 切换失败并回到终端端，所有设备看到“该会话正被另一个 Codex 客户端使用”，没有产生新的 agent 会话
