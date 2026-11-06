## MODIFIED Requirements

### Requirement: 发送与排队

会话处于聊天视图时，`POST /api/sessions/{id}/input` SHALL 接受 `{clientRequestId, text, attachments, mode}` 作为聊天提示，`mode` 取 `auto` 或 `steer`，缺省为 `auto`；响应为 `{view: "chat", queued, itemId, turnId, promptId}`。系统 SHALL NOT 提供 `POST /api/sessions/{id}/chat/prompts`。

- `auto` 模式：没有运行中的回合且队列未暂停时，SHALL 立即发送；否则 SHALL 追加到队尾。
- 队列中的条目 SHALL 在上一回合正常结束后自动发送，一次一条。

`text` 去除首尾空白后 SHALL 不为空，且不超过 64 KiB。会话处于终端视图时，带 `attachments` 或 `mode` 的请求 SHALL 返回 409 `wrong_view`（不带它们的输入按 `session-surface` 写入终端）；切换进行中 SHALL 返回 409 `switching`。ACP 正在自动重连时，消息 SHALL 先进入队列等待连接恢复。

#### Scenario: 运行中继续发送

- **WHEN** 回合进行中，用户先后发送 A、B 两条消息
- **THEN** A、B 依次出现在队列中；当前回合结束后 A 自动发送，A 的回合结束后 B 自动发送

#### Scenario: 终端视图拒绝插话

- **WHEN** 客户端向处于终端视图的会话发送带 `mode: "steer"` 的输入
- **THEN** 服务端返回 409，`reason` 为 `wrong_view`，终端没有收到任何输入

### Requirement: 请求去重

服务端 SHALL 按 `clientRequestId` 对聊天视图的输入（`/input`）、队列操作、中断（`/interrupt`）、答复与配置变更去重，保留最近 256 条、30 分钟。重复的请求 SHALL 返回首次处理的结果，SHALL NOT 再次执行。

#### Scenario: 重试不重复发送

- **WHEN** 手机网络中断导致客户端以同一 `clientRequestId` 重新提交发送
- **THEN** 消息只发送一次，两次请求得到相同结果
