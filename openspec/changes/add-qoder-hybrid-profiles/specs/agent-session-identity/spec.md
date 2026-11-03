# agent-session-identity Delta Spec

## New Requirements

### Requirement: session/list 候选匹配

对于不通过 OSC 7501 终端信号传递会话 ID 的档案（如 Qoder、QoderCN），系统 SHALL 使用 `session/list` 作为会话发现机制：

1. 启动 ACP 连接后，调用 `session/list` 获取当前 cwd 下的所有会话；
2. 若候选唯一，自动选择该会话 ID；
3. 若候选多个且无法确定，返回 `SessionAmbiguousError`，附带候选列表供用户选择；
4. 若无候选，返回 `SessionMissingError`。

此机制 SHALL NOT 替代现有 OSC 7501、LockTracker 或 ProcessRegistry 方式；仅当档案未实现这些接口时使用。

#### Scenario: 唯一候选自动选择

- **WHEN** Qoder 混合会话切到聊天，`session/list` 在当前 cwd 下返回恰好一个会话
- **THEN** 系统自动选择该会话 ID，切换成功

#### Scenario: 多个候选提示用户

- **WHEN** Qoder 混合会话切到聊天，`session/list` 在当前 cwd 下返回多个会话
- **THEN** 系统返回 `SessionAmbiguousError`，前端显示候选列表供用户选择
