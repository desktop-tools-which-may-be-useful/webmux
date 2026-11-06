## MODIFIED Requirements

### Requirement: 恢复到任一端

`POST /api/sessions/{id}/resume {clientRequestId, view, agentSessionId, confirmWithoutEnv}` SHALL 只对 `state` 为 `closed` 或 `interrupted` 的会话可用，对 `active` 会话 SHALL 返回 409 `not_archived`。`view` 取 `terminal` 或 `chat`，缺省为记录中的所在视图；
请求体中的 `side` SHALL 作为未知字段返回 400。恢复时，系统 SHALL 按以下步骤执行：

1. 用记录的 cwd 调用 `session/list`，确认 agent 会话 ID 仍然存在。不存在时，SHALL 返回 409，并附上历史 ID 与 `session/list` 中同一 cwd 下的候选，由用户选择后以 `agentSessionId` 重试或放弃。
2. 以记录中的 webmux 会话 ID 重新创建会话外壳。
3. 恢复到聊天视图时，启动 ACP 并 `session/load`；恢复到终端视图时，在 PTY 中执行档案生成的恢复命令。
4. 重新应用记录中的配置项取值。

恢复后的会话 SHALL 仍是同一个混合会话，`state` 为 `active`，可以继续切换。

#### Scenario: 恢复到终端

- **WHEN** 用户在工作树的“已归档”区对一条已中断的记录选择“恢复到终端”
- **THEN** 会话以原 ID 出现在运行中列表，`view` 为 `terminal`，TUI 显示此前的对话

#### Scenario: agent 会话已被删除

- **WHEN** 记录中的 agent 会话 ID 已在 agent 中被删除，用户选择恢复
- **THEN** 服务端返回 409，界面列出可选的候选会话，用户也可以放弃恢复

#### Scenario: 恢复运行中的会话

- **WHEN** 客户端对一个 `state` 为 `active` 的会话请求恢复
- **THEN** 服务端返回 409，`reason` 为 `not_archived`，会话不变

### Requirement: 移除记录

`DELETE /api/sessions/{id}?forget=1` SHALL 删除会话的混合会话记录，SHALL NOT 删除 agent 的会话库。对 `closed` 或 `interrupted` 的会话，系统 SHALL 直接删除记录；对 `active` 的混合会话，系统 SHALL 先按“关闭混合会话”的流程关闭，再删除记录。删除后，
该会话 SHALL NOT 再出现在任何状态的列表中，`GET /api/sessions/{id}` SHALL 返回 404。系统 SHALL NOT 提供 `DELETE /api/hybrid-sessions/{id}`。UI 中该操作 SHALL 称为“删除记录”，且 SHALL 只对已归档会话提供。

#### Scenario: 移除

- **WHEN** 用户对一条已关闭的记录选择“删除记录”并确认
- **THEN** 记录从所在工作树的“已归档”区消失；在 agent 的 TUI 中仍可以用其自身的恢复参数打开该会话

### Requirement: 可恢复记录的列表

`GET /api/sessions?state=archived` SHALL 返回所有 `closed` 与 `interrupted` 的混合会话，按 `archive.updatedAt` 倒序排列。每一项 SHALL 是会话信息（见 `session-surface`），记录摘要的内容由以下字段表达：

- ID、创建名称（`name`）、当前会话标题（`title`）、档案（`agent.name`）、cwd、所在视图（`view`）、状态（`state`）、更新时间（`archive.updatedAt`）与中断时间（`archive.interruptedAt`）；
- 中断前的状态（`archive.activity`），用于提示“上一轮可能未完成”；
- 启动配置是否仍存在（`archive.configMissing`）；
- 工作区关联快照，以会话信息的 `workspace` 字段表达（含仓库 ID 与工作树 ID），没有快照时为 `null`。

该列表 SHALL NOT 包含环境变量名或值。系统 SHALL NOT 提供 `GET /api/hybrid-sessions`。

首页 SHALL 按 `workspace-overview` 的“已归档”规则在所属工作树中展示这些会话。每条记录 SHALL 优先显示记录中的当前会话标题，旧记录没有标题时回退到创建名称，并提供“恢复到聊天 / 恢复到终端 / 删除记录”三个操作。会话列表事件涉及混合会话（包括 `removed`）时，首页 SHALL 重新获取该列表。

#### Scenario: 首页分组

- **WHEN** 同一工作树有一条已关闭和一条已中断的记录
- **THEN** 该工作树的“已归档”区折叠显示数量 2 并标出 1 条已中断，展开后两条记录分别标注“已关闭”和“已中断”，并各有三个操作

#### Scenario: 保留会话标题

- **WHEN** agent 或终端更新了会话标题，随后会话关闭或中断
- **THEN** 记录保存更新后的标题，“已归档”区显示该标题而不是创建时的默认名称

#### Scenario: 摘要包含工作区关联

- **WHEN** 一条已关闭记录的会话曾关联 `feature-x` 工作树
- **THEN** `GET /api/sessions?state=archived` 中该会话的 `workspace` 为该工作树的仓库 ID 与工作树 ID，且不出现环境变量
