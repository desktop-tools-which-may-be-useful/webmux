## MODIFIED Requirements

### Requirement: 插话

只有当档案声明了插话方法、agent 在初始化时声明支持、且有运行中的回合时，界面才 SHALL 显示“插话”。`steer` 模式的发送 SHALL 把文本注入当前回合，不进入队列。

不满足条件时，`steer` 请求 SHALL 返回 409，界面 SHALL NOT 显示插话入口。

档案声明插话需要空闲行为参数时（Codex），插话请求 SHALL 带上 `_meta.steering.idleBehavior = "promptRequired"`，并按返回的 `outcome` 处理：

- 注入成功：照常显示该条用户消息；
- `promptRequired`（回合已在注入前结束）：该文本 SHALL 作为新条目放回队列队首，按队列规则发送，SHALL NOT 丢失；
- `failed`：请求以 agent 错误结束，文本退回发起设备的输入草稿；
- `startedNewTurn`：视为协议错误，系统 SHALL 记录方法名与错误码，并在聊天视图提示该回合不受 webmux 跟踪。

#### Scenario: OpenCode 没有插话

- **WHEN** 用户在 OpenCode 的混合会话中有回合运行
- **THEN** 输入框旁只有发送与停止，没有插话按钮

#### Scenario: 插话时回合恰好结束

- **WHEN** 用户在 Codex 混合会话中插话，agent 在注入前结束了回合并返回 `promptRequired`
- **THEN** 该文本进入队列队首并作为下一回合发送，agent 没有自行开启 webmux 不知道的回合
