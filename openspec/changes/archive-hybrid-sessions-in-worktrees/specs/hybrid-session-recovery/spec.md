## MODIFIED Requirements

### Requirement: 混合会话记录的内容

系统 SHALL 为每个混合会话保存一条记录，内容包括：

- webmux 会话 ID、创建名称、当前会话标题、启动配置 ID、档案名、cwd、argv 快照；
- 所在端、当前 agent 会话 ID、最近至多 8 个被替换下来的 agent 会话 ID；
- 最近一次的配置项取值；
- 状态（`active`、`closed`、`interrupted`）、中断前的状态，以及当时队列中的条数（只记条数，不记内容）；
- 创建、更新、中断时间；
- 根进程的 PID 与进程启动标识；
- 最近一次的工作区关联快照（仓库 ID 与工作树 ID），会话没有仓库关联时为空。

工作区关联快照 SHALL 在会话的仓库关联变化时更新，关联被解除时 SHALL 清空；服务重启把记录改为 `interrupted` 时 SHALL 保留最后的快照。旧文件中没有该字段的记录 SHALL 照常读取。

记录 SHALL NOT 包含环境变量值、消息内容、队列、token 或终端输出。

#### Scenario: 记录不含密钥

- **WHEN** 混合配置的环境变量含 `API_KEY=secret-value`，用户创建并使用混合会话
- **THEN** `hybrid-sessions.json` 中任何位置都不出现 `secret-value`

#### Scenario: 保存工作区关联

- **WHEN** 用户从已注册仓库的 `feature-x` 工作树启动混合会话，随后服务重启
- **THEN** 该记录变为 `interrupted`，其工作区关联快照仍指向 `feature-x` 的仓库 ID 与工作树 ID

#### Scenario: 读取旧记录

- **WHEN** 服务读取升级前写入、没有工作区关联字段的 `hybrid-sessions.json`
- **THEN** 文件被正常加载，这些记录的工作区关联快照为空，混合会话创建与恢复照常可用

### Requirement: 重启与中断后的状态

服务启动时，系统 SHALL 把所有 `active` 记录改为 `interrupted`。改之前 SHALL 按记录中的 PID 与启动标识检查进程是否残留，两者都匹配时 SHALL 结束其进程组。

在会话运行期间，以下情况 SHALL 把记录改为 `interrupted`：TUI 自行退出、ACP 重连失败、切换回滚失败。用户关闭（UI 中称为“归档”）SHALL 把记录改为 `closed`。

普通 PTY 会话 SHALL 仍不跨服务重启。

#### Scenario: 服务重启

- **WHEN** 一个混合会话在聊天端时服务重启
- **THEN** 重启后它不在运行中列表，而出现在所属工作树的“已归档”区，标注“已中断”，并记录中断前在聊天端

### Requirement: 恢复到任一端

`POST /api/sessions/{id}/resume {side}` SHALL 只对 `closed` 或 `interrupted` 的记录可用，`side` 取 `tui` 或 `acp`，缺省为记录中的所在端。恢复时，系统 SHALL 按以下步骤执行：

1. 用记录的 cwd 调用 `session/list`，确认 agent 会话 ID 仍然存在。不存在时，SHALL 返回 409，并附上历史 ID 与 `session/list` 中同一 cwd 下的候选，由用户选择后重试或放弃。
2. 以记录中的 webmux 会话 ID 重新创建会话外壳。
3. 恢复到聊天端时，启动 ACP 并 `session/load`；恢复到终端端时，在 PTY 中执行档案生成的恢复命令。
4. 重新应用记录中的配置项取值。

恢复后的会话 SHALL 仍是同一个混合会话，可以继续切换。

#### Scenario: 恢复到终端

- **WHEN** 用户在工作树的“已归档”区对一条已中断的记录选择“恢复到终端”
- **THEN** 会话以原 ID 出现在运行中列表，处于 `tui` 端，TUI 显示此前的对话

#### Scenario: agent 会话已被删除

- **WHEN** 记录中的 agent 会话 ID 已在 agent 中被删除，用户选择恢复
- **THEN** 服务端返回 409，界面列出可选的候选会话，用户也可以放弃恢复

### Requirement: 移除记录

`DELETE /api/hybrid-sessions/{id}` SHALL 只对 `closed` 或 `interrupted` 的记录可用，对活动会话 SHALL 返回 409。移除 SHALL 删除记录，SHALL NOT 删除 agent 的会话库。移除后，该记录 SHALL NOT 再出现在任何列表中。UI 中该操作 SHALL 称为“删除记录”。

#### Scenario: 移除

- **WHEN** 用户对一条已关闭的记录选择“删除记录”并确认
- **THEN** 记录从所在工作树的“已归档”区消失；在 agent 的 TUI 中仍可以用其自身的恢复参数打开该会话

### Requirement: 可恢复记录的列表

`GET /api/hybrid-sessions?state=recoverable` SHALL 返回所有 `closed` 与 `interrupted` 的记录摘要，按更新时间倒序排列。每条摘要包含：

- ID、创建名称、当前会话标题、档案、cwd、所在端、状态、更新时间与中断时间；
- 中断前的状态，用于提示“上一轮可能未完成”；
- 启动配置是否仍存在；
- 工作区关联快照（`workspace: {repositoryId, worktreeId}`），没有快照时省略。

摘要 SHALL NOT 包含环境变量名或值。

首页 SHALL 按 `workspace-overview` 的“已归档”规则在所属工作树中展示这些记录。每条记录 SHALL 优先显示记录中的当前会话标题，旧记录没有标题时回退到创建名称，并提供“恢复到聊天 / 恢复到终端 / 删除记录”三个操作。会话列表事件涉及混合会话时，首页 SHALL 重新获取该列表。

#### Scenario: 首页分组

- **WHEN** 同一工作树有一条已关闭和一条已中断的记录
- **THEN** 该工作树的“已归档”区折叠显示数量 2 并标出 1 条已中断，展开后两条记录分别标注“已关闭”和“已中断”，并各有三个操作

#### Scenario: 保留会话标题

- **WHEN** agent 或终端更新了会话标题，随后会话关闭或中断
- **THEN** 记录保存更新后的标题，“已归档”区显示该标题而不是创建时的默认名称

#### Scenario: 摘要包含工作区关联

- **WHEN** 一条已关闭记录的会话曾关联 `feature-x` 工作树
- **THEN** 列表摘要中该记录的 `workspace` 为该工作树的仓库 ID 与工作树 ID，且不出现环境变量
