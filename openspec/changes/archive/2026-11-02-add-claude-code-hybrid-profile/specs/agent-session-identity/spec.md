## MODIFIED Requirements

### Requirement: 身份字段的可见范围与根进程信息

这些字段 SHALL 只保存在服务端内存中。它们 SHALL NOT 出现在普通会话的会话信息、日志或推送中；对混合会话，只有经档案解释后的 agent 名与 agent 会话 ID 出现在 `hybrid` 对象中。会话退出时 SHALL 清空这些字段。

系统 SHALL 另外记录会话 PTY 根进程的可执行文件名与当前工作目录，供资格判定使用。对档案要求的混合会话，系统 SHALL 还能按需提供：

- 根进程及允许启动器的直接子进程的 PID、可执行文件名与进程启动标识；
- 根进程及其后代进程当前打开的普通文件的绝对路径，供档案从中解释 agent 会话 ID。

这些进程事实 SHALL 只在资格判定需要时读取，`session` 层 SHALL NOT 理解其中任何 agent 的含义；读取失败时 SHALL 视为没有结果，SHALL NOT 使会话出错。Claude 注册记录的路径、PID、进程启动标识与文件内容同样 SHALL NOT 出现在客户端负载或日志中。

#### Scenario: 普通会话不读取 Claude 注册记录

- **WHEN** 普通 shell 会话中手动运行 Claude Code
- **THEN** 系统不读取该进程的 Claude 注册记录，REST 与 WebSocket 负载中不出现其 agent 会话 ID

#### Scenario: 普通会话不泄露

- **WHEN** 普通 shell 会话中运行 omp 并发出带 `session_id` 的事件
- **THEN** 该会话的 REST 与 WebSocket 负载中不出现该 ID

### Requirement: 切到聊天的资格

会话在 `tui` 端时，只有同时满足以下六个条件，切到聊天的操作才 SHALL 可用：

1. 终端信号识别出的 agent 有对应的档案；
2. 档案声明了第三方连接器时，连接器已安装；
3. 该 agent 进程是会话 PTY 的根进程；或者档案允许启动器、根进程是该启动器，且其直接子进程是该 agent 的原生可执行文件；
4. 已得到当前 agent 会话 ID；
5. 用正确的 cwd 调用 `session/list` 能查到这个 ID。cwd 的来源由档案决定：omp 取事件中的 cwd，OpenCode、Codex 与 Claude Code 取实际 agent 进程的 cwd；
6. 这个 ID 没有被其他会话或档案可识别的外部客户端占用。

不满足时，界面 SHALL 禁用该操作并显示对应原因，依次为“未识别到支持的 Agent”、“未安装连接器”、“Agent 不是由本会话直接启动的”、“请先在 TUI 中发送一条消息”、“会话尚未保存或找不到会话”与“该会话已在另一个会话中打开”。

条件 5 SHALL 在 ID 或 cwd 变化后核对一次，并在执行切换前再核对一次；它 SHALL NOT 在每个信号到来时都执行。核对 SHALL 只使用 `session/list`，SHALL NOT 加载或恢复该会话。

#### Scenario: Claude 首条消息之前

- **WHEN** Claude 注册记录已经给出会话 ID，但 `session/list` 中尚没有可加载的 transcript
- **THEN** “聊天”按钮不可用，提示“请先在 TUI 中发送一条消息”，SHALL NOT 尝试 `session/load`

#### Scenario: 在 shell 中手动启动

- **WHEN** 混合配置的 TUI 退出后，用户在同一 PTY 中又通过其他程序启动了 Claude Code
- **THEN** “聊天”按钮不可用，提示 Agent 不是由本会话直接启动的

### Requirement: 跨会话占用检测

判定占用条件时，系统 SHALL 检查所有 WebMux 会话的身份字段（包括普通会话）以及所有活动混合会话的记录。只要有其他会话报出相同的 agent 会话 ID，就 SHALL 视为占用。

档案具备外部占用来源时，系统 SHALL 同时按该来源检测：Codex 使用活动写者错误，Claude Code 使用本需求新增的进程注册记录。没有外部占用来源或来源不可读时，系统 SHALL 保留 WebMux 内部占用检测，并在用户文档中说明无法保证发现所有外部客户端，SHALL NOT 宣称外部检测始终完整。

#### Scenario: 普通会话中恢复了同一会话

- **WHEN** 用户在一个普通 PTY 会话中执行 `claude --resume <id>`，该 ID 正被一个混合会话使用，且进程身份可识别
- **THEN** 该混合会话的“聊天”按钮不可用，提示该会话已在另一个会话中打开

## ADDED Requirements

### Requirement: Claude 会话 ID 的进程注册记录来源

Claude Code 的 OSC 7501 根记录只提供 `app=claude-code`，不提供会话 ID。对 Claude Code 混合会话，系统 SHALL 按以下规则取得会话 ID：

- 读取实际 agent 进程对应的 `<CLAUDE_CONFIG_DIR>/sessions/<pid>.json` 注册记录；
- 未设置非空 `CLAUDE_CONFIG_DIR` 时，使用服务用户主目录下的 `.claude`；
- SHALL NOT 通过解析 Claude transcript JSONL 推导当前会话。

采用注册记录前 SHALL 验证：

- 文件名 PID、记录内 PID 与实际 agent 进程一致；本次实测的原生 Linux 运行直接使用根 PID，npm `node` 启动器使用其直接 Claude 子进程，而不是启动器 PID；
- `sessionId` 是合法 UUID，记录 cwd 与资格判定使用的规范化 cwd 一致；
- Linux 记录中的 `procStart` SHALL 是 `/proc/<pid>/stat` 第 22 字段的十进制启动 ticks 字符串，`pidDomain` SHALL 为 `linux:<machine-id>:<pid-namespace>`；二者 SHALL 与当前进程事实一致。其他平台使用各自可验证的进程启动标识与 domain；无法证明记录属于当前运行、记录已过期或 JSON 无效时 SHALL 拒绝采用；
- 读取与解析 SHALL 有界，注册记录路径和内容 SHALL NOT 泄露到负载或日志。

系统 SHALL 按现有身份观察节奏重新读取记录，使 `/new`、`/resume` 或其他 TUI 内会话切换更新当前 ID，并沿用既有 ID 变化提示和历史记录规则。本次实测中，注册记录在首条消息前已经存在，TUI `--resume <id>` 保持请求的 UUID，正常退出删除注册记录。

首条消息与 `/new` 的可见性 SHALL 以 `session/list` 为准，SHALL NOT 以 transcript 文件是否存在代替：首条消息前 list 不含当前 ID，首条真实 prompt 完成后包含；`/new` 后即使已有非空元数据 transcript，新 ID 在首次 prompt 前仍不出现在 list 中。

#### Scenario: 启动后取得 Claude 会话 ID

- **WHEN** 当前 Claude 进程的注册记录包含匹配的 PID、进程启动标识、cwd 与 UUID `01a18715-8504-7020-9f3d-257900000000`
- **THEN** `hybrid.agentSessionId` 更新为该 UUID，资格判定继续核对该 ID 是否已列出

#### Scenario: PID 复用产生的陈旧记录

- **WHEN** 注册记录的文件名 PID 与当前进程相同，但进程启动标识不匹配
- **THEN** 系统拒绝采用该记录，不加载其中的会话 ID

#### Scenario: TUI 内切换会话

- **WHEN** 同一 Claude 进程的注册记录从旧 UUID 更新为新 UUID
- **THEN** WebMux 更新当前 ID、保留旧 ID 历史并显示既有 ID 变化提示

#### Scenario: /new 后尚未发送消息

- **WHEN** `/new` 已更新注册记录并产生非空元数据 transcript，但尚未发送新会话的首条 prompt
- **THEN** `session/list` 仍不含新 ID，“聊天”按钮保持不可用

#### Scenario: TUI 恢复已有会话

- **WHEN** 干净 TUI 以 `claude --resume <id>` 启动
- **THEN** 根进程注册记录使用请求的 UUID，不另建新 ID，list 可见性沿用该会话已有历史

#### Scenario: TUI 正常退出

- **WHEN** 当前 Claude 进程退出并完成进程清理
- **THEN** 其注册记录被删除，不再计为当前身份或外部占用

### Requirement: Claude 会话 ID 的候选降级与外部占用

注册记录不存在、不可读或无法通过验证时，系统 SHALL 使用 `session/list` 候选判定：取相同规范化 cwd 下、更新时间晚于本次 TUI 启动的会话。恰有一个候选时采用它；有多个候选时 SHALL 返回 409 `session_ambiguous` 并附候选列表，由用户选择后重试；没有候选时 SHALL 保持不可切换。

系统 SHALL 同时利用同一配置目录下的有效注册记录，尽力识别其他仍在运行的 Claude 进程是否持有当前会话。本次实测两个原生 TUI 可同时以不同 PID 注册同一 UUID，因此不能依赖 Claude 自身阻止双写。占用判定 SHALL 遵守以下规则：

- 发现其他进程持有同一 ID 时返回 `occupied_external`；切换过程中发现时按现有失败回滚处理，SHALL NOT 加载或分叉会话；
- 当前 WebMux TUI 对应的记录 SHALL 排除在外；
- 已退出或启动标识不匹配的记录 SHALL NOT 计为占用。

#### Scenario: 多个 Claude 候选

- **WHEN** 注册记录不可用，同一 cwd 下本次 TUI 启动后有两个 Claude 会话被更新，用户点击“聊天”
- **THEN** 服务端返回 409 `session_ambiguous` 与两个候选，界面让用户选择后再切换

#### Scenario: 外部 Claude 客户端占用

- **WHEN** 另一仍在运行且进程启动标识匹配的 Claude 进程注册为当前会话的持有者，用户点击“聊天”
- **THEN** 操作以 `occupied_external` 结束，原端保持不变，没有产生新会话

#### Scenario: 注册记录不可用

- **WHEN** 配置目录不可读，且 WebMux 内部没有发现占用
- **THEN** 系统继续候选与 WebMux 内部占用判定，并依据文档说明外部检测可能不完整，不伪造“没有外部客户端”的保证
