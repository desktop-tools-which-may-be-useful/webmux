# agent-session-identity Specification Delta

## MODIFIED Requirements

### Requirement: 切到聊天的资格

会话在 `tui` 端时，只有同时满足以下六个条件，切到聊天的操作才 SHALL 可用：

1. 终端信号识别出的 agent 有对应的档案（omp 由 Warp 事件或 `OSC 7501` `app=omp` 识别；
   OpenCode 由 `OSC 7501` `app=opencode` 识别；Codex 由 `OSC 7501` `app=codex` 识别；
   Claude Code 由 `OSC 7501` `app=claude-code` 识别；`agy` 由 `OSC 7501`
   `app=antigravity` 识别）；
2. 档案声明了第三方连接器时，连接器已安装（`acp-connector-availability`）；
3. 该 agent 进程是会话 PTY 的根进程；或者档案允许启动器、根进程是该启动器，且其直接子进程是该
   agent 的原生可执行文件（Codex 与 Claude Code 允许 npm 安装产生的 `node` 启动器）；
4. 已得到当前 agent 会话 ID（omp 取 Warp `session_id`；OpenCode 取 `ses_` 前缀的
   `OSC 7501` 子记录 `id`；Codex 取写锁文件；Claude Code 取进程注册记录；`agy` 取标准
   36 字符 UUID 格式的 `OSC 7501` 子记录 `id`；若当前运行是切换或恢复启动且尚未收到信号中的 ID，
   则回退取本次启动所恢复的 ID）；
5. 用正确的 cwd 调用 `session/list` 能查到这个 ID。cwd 的来源由档案决定（omp 取事件中的 `cwd`，
   OpenCode、Codex、Claude Code 与 `agy` 取根进程的 cwd）；
6. 这个 ID 没有被其他会话占用。

不满足时，界面 SHALL 禁用该操作并显示对应原因，依次为：

- “未识别到支持的 Agent”；
- “未安装 `<连接器显示名称>`（`<可执行文件>`）”（原因码 `connector_missing`）；
- “Agent 不是由本会话直接启动的”；
- “请先在 TUI 中发送一条消息”；
- “会话尚未保存”或“找不到会话”；
- “该会话已在另一个会话中打开”。

条件 5 SHALL 在 ID 或 cwd 变化后核对一次，并在执行切换前再核对一次；它 SHALL NOT 在每个信号到来时都执行。
核对 SHALL 只使用 `session/list`，SHALL NOT 加载或恢复该会话。

#### Scenario: 首条消息之前

- **WHEN** 从终端开始的 OpenCode 或 `agy` 混合会话还没有发送过消息
- **THEN** “聊天”按钮不可用，提示“请先在 TUI 中发送一条消息”

#### Scenario: agy 在首条消息后满足切换资格

- **WHEN** 从终端开始的 `agy` 混合会话发出 `app=antigravity` 根记录，并在首条消息后通过
  `OSC 7501` 子记录报出 UUID `89c9a933-60f6-46ad-977b-a8daf22f8e4d`，且 `agy acp` 的
  `session/list` 包含该 ID
- **THEN** 会话的 `hybrid.agentSessionId` 更新为 `89c9a933-60f6-46ad-977b-a8daf22f8e4d`，
  “聊天”按钮变为可用

#### Scenario: 在 shell 中手动启动

- **WHEN** 混合配置的 TUI 退出后，用户在同一 PTY 中又通过其他程序启动了 omp（omp 不是根进程）
- **THEN** “聊天”按钮不可用，提示 Agent 不是由本会话直接启动的

#### Scenario: npm 安装的 Codex

- **WHEN** Codex 混合会话的根进程是 `node`，其直接子进程是 codex 原生可执行文件，TUI 已发出首条消息
- **THEN** 条件 3 满足，“聊天”按钮可用

#### Scenario: 未安装连接器优先提示

- **WHEN** Codex 混合会话尚未发出消息，服务上没有 `codex-acp`
- **THEN** “聊天”按钮提示“未安装 Codex ACP 适配器（codex-acp）”，而不是“请先在 TUI 中发送一条消息”
