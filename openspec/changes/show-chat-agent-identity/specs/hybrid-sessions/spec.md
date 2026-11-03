## MODIFIED Requirements

### Requirement: 单写者与所在端

混合会话 SHALL 在任意时刻处于且只处于以下之一：`tui`、`acp`、`switching`。处于 `acp` 时 SHALL 还可以带有 `dormant` 标记。webmux 会话 ID SHALL 在混合会话的整个生命周期中保持不变。

系统 SHALL 保证同一 agent 会话 ID 在任意时刻至多被一个由 webmux 启动的进程打开。只调用 `session/list` 的临时探测进程不算打开。进入目标端之前，系统 SHALL 已确认原端的进程组已经全部退出。这是硬性要求：omp 在 TUI 仍持有会话租约时，另一个进程写入同一会话，会把它分叉成一个新 ID 的会话。

会话信息 SHALL 包含 `hybrid` 对象，至少含 `side`、`phase`、`dormant`、`state`、`agent`、`agentTitle`、`agentIcon` 与 `agentSessionId`（尚未得知时为 `null`）。普通会话 SHALL NOT 含该对象。

- `agentTitle` SHALL 是 agent 档案声明的显示名，非空；
- `agentIcon` SHALL 是 agent 档案声明的图标键，取值属于启动配置的图标目录；
- 两者在会话生命周期内 SHALL 保持不变，并在会话列表快照、列表事件与单个会话信息中一致。

#### Scenario: 切换期间不存在两个进程

- **WHEN** 会话从 `tui` 切到 `acp`，同时用进程表持续观察
- **THEN** 在 TUI 进程退出之前，不出现由 webmux 为该会话启动的 ACP 进程

#### Scenario: 普通会话的负载不变

- **WHEN** 客户端读取普通会话的会话信息
- **THEN** 负载中没有 `hybrid` 字段，其余字段与本变更前一致

#### Scenario: 会话信息带出 agent 显示名与图标

- **WHEN** 用户以 OpenCode 混合配置创建会话，客户端随后读取会话列表快照
- **THEN** 该会话的 `hybrid` 对象含 `agent: "opencode"`、`agentTitle: "OpenCode"` 与一个属于启动配置图标目录的 `agentIcon`，单个会话信息中的取值与之相同
