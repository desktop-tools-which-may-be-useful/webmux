## ADDED Requirements

### Requirement: 根进程可执行文件回退识别

终端信号未识别出 agent 时（例如 Qoder、QoderCN 不发 OSC 7501 根记录），系统 SHALL 回退按会话
PTY 根进程的可执行文件识别：根进程可执行文件属于当前配置档案（含其版本化名称），或档案允许启动器、
根进程是该启动器且其直接子进程属于该档案时，SHALL 视为识别出该档案的 agent，cwd 取根进程的 cwd；
档案跟踪写锁或进程注册记录时，会话 ID 仍按该来源取得。终端信号识别出 agent 时 SHALL 以信号为准。

#### Scenario: 无 OSC 7501 时按进程识别

- **WHEN** QoderCN 混合会话的根进程可执行文件为 `qoderclicn-1.1.81`，终端没有 OSC 7501 根记录
- **THEN** 系统识别该会话的 agent 为 qodercn，不提示“未识别到支持的 Agent”

### Requirement: 无自有会话 ID 来源档案的 session/list 候选判定

档案既不跟踪写锁也不读取进程注册记录（例如 Qoder、QoderCN），且当前尚无会话 ID 时，系统 SHALL 用
`session/list` 做候选判定，SHALL NOT 直接判定为缺少会话 ID：

1. 用根进程 cwd 调用 `session/list`，取更新时间晚于本次 TUI 启动的会话作为候选；
2. 没有候选时，“聊天”按钮保持不可用，提示“请先在 TUI 中发送一条消息”；
3. 有候选时“聊天”按钮可用；切换时恰有一个候选则采用它；
4. 有多个候选时，切换 SHALL 返回 409 `session_ambiguous` 并附候选列表，由用户选择后重试。

#### Scenario: 唯一候选自动选择

- **WHEN** Qoder 混合会话切到聊天，`session/list` 在当前 cwd 下恰有一个本次 TUI 启动后更新的会话
- **THEN** 系统采用该会话 ID，切换成功

#### Scenario: 多个候选提示用户

- **WHEN** Qoder 混合会话切到聊天，`session/list` 在当前 cwd 下有多个本次 TUI 启动后更新的会话
- **THEN** 切换返回 409 `session_ambiguous`，前端显示候选列表供用户选择

#### Scenario: 首条消息之前

- **WHEN** 从终端开始的 Qoder 混合会话还没有发送过消息，`session/list` 中没有本次启动后更新的会话
- **THEN** “聊天”按钮不可用，提示“请先在 TUI 中发送一条消息”
