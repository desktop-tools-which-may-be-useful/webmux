# hybrid-sessions Specification Delta

## ADDED Requirements

### Requirement: 过滤活动回合中 agent 回显的用户消息片段

系统在 `acp` 端发起 `session/prompt` 时，已经在 `turn.start` 事件之后立即向聊天时间线写入
包含原始文本与附件元数据的用户消息。因此：

- 在活动 `session/prompt` 回合进行期间（即从发出 `session/prompt` 到收到其最终响应或取消完成之间），
  若 agent 发送 `sessionUpdate: "user_message_chunk"` 通知（例如 `agy acp` 在实时回合开头回显用户消息），
  系统 SHALL 忽略该 `user_message_chunk` 通知，SHALL NOT 在时间线中生成第二条重复的用户消息。
- 在非活动回合阶段（即执行 `session/load` 回放历史会话期间），系统 SHALL 照常将 agent 发送的
  `user_message_chunk` 通知转换为时间线中的用户消息事件。

#### Scenario: agy 在实时回合中回显 user_message_chunk

- **WHEN** 用户在 `agy` 混合会话聊天端发送一条消息，`agy acp` 在处理 `session/prompt` 期间依次发送
  `user_message_chunk` 与 `agent_message_chunk`
- **THEN** 聊天时间线中该回合只包含一条用户消息与一条 agent 回复，不出现重复的用户消息气泡

#### Scenario: session/load 历史回放保留 user_message_chunk

- **WHEN** 会话从终端切到聊天并执行 `session/load`，`agy acp` 依次回放历史中的
  `user_message_chunk`、`tool_call` 与 `agent_message_chunk`
- **THEN** 聊天时间线完整还原历史中的用户消息、工具调用与 agent 回复
