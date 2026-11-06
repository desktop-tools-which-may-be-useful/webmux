## MODIFIED Requirements

### Requirement: 列出与打开会话

`webmux ls` SHALL 以表格列出会话（ID、名称、状态与退出码、命令、启动目录）；`--json` 时 SHALL 向标准输出写入服务端会话列表的原始 JSON 数组。`webmux open <会话 ID>` SHALL 输出该会话的本机会话链接，`--browser` 时 SHALL 同时用默认浏览器打开；会话不存在时 SHALL 以退出码 1 结束。

`webmux ls` SHALL 接受以下筛选，并映射为 `GET /api/sessions` 的查询：

- `--state STATE`（可重复）：取值为 `active`、`closed`、`interrupted`、`archived` 或 `all`，缺省只列 `active`；未知取值 SHALL 以 2 结束。
- `--archived`：等同于 `--state archived`。
- `--repo REPO`：只列出工作区关联属于该仓库的会话，仓库参数的解析规则与 `worktree` 相同。

已归档会话在表格的状态列中 SHALL 显示其 `state`（`closed` 或 `interrupted`），而不是退出码。

#### Scenario: 机器可读列表

- **WHEN** 用户执行 `webmux ls --json`
- **THEN** 标准输出恰好是服务端会话列表 JSON，可直接交给 `jq`

#### Scenario: 列出已归档会话

- **WHEN** 用户执行 `webmux ls --archived --repo .`
- **THEN** 只列出当前仓库中已关闭或已中断的会话，按归档更新时间从新到旧排列

### Requirement: 用混合配置从终端创建会话

`webmux run --launch-config <名称>` 引用勾选了混合模式的配置时，SHALL 创建带聊天视图的会话，起始视图默认为终端（`--view chat` 见 `shell-session-launch`）。行为 SHALL 与 Web 端创建的完全相同：写入混合会话记录，按资格判定决定能否切到聊天。

未指定 `--detach` 时，本地终端 SHALL 像普通会话一样附着，可以输入。附着中的本地终端 SHALL NOT 提供切换按键；切换在 Web 端或用 `webmux view` 发起。

`webmux run -- <argv…>` SHALL 始终创建普通会话，即使命令是 omp 或 opencode。

#### Scenario: 从终端启动混合配置

- **WHEN** 用户执行 `webmux run --launch-config omp-hybrid`
- **THEN** 本地终端附着并运行 omp；会话信息含 `views: ["terminal","chat"]` 与 `view: "terminal"`，Web 端列表中该会话带有终端端标记，可以在资格满足后切到聊天

#### Scenario: 直接命令不是混合会话

- **WHEN** 用户执行 `webmux run -- omp`
- **THEN** 创建普通会话，`views` 为 `["terminal"]`，Web 端不显示聊天切换

### Requirement: 会话移到另一端的关闭码

服务端 SHALL 用 WebSocket 关闭码 4409 表示“会话已移到另一端”，关闭原因中 SHALL 包含当前视图（`chat` 或 `terminal`）。4409 SHALL 写入协议文档。浏览器与本机客户端 SHALL 把 4409 视为终止状态，SHALL NOT 自动重连。

#### Scenario: 浏览器终端页收到 4409

- **WHEN** 浏览器终端页连接着一个混合会话，该会话在另一设备上切到聊天
- **THEN** 服务端以 4409 关闭终端 WebSocket，关闭原因包含 `chat`，浏览器不自动重连

### Requirement: 列表显示所在端

`webmux ls` 的表格 SHALL 为带聊天视图的会话在状态列中附加所在视图：`view` 为 `terminal` 时显示 `终端`，为 `chat` 时显示 `聊天`，`transition` 不为 `null` 时显示 `切换中`。
`webmux ls --json` SHALL 原样包含服务端会话信息中的 `view`、`views`、`transition` 与 `agent`。普通会话的表格输出 SHALL 不变。

#### Scenario: 机器可读的所在端

- **WHEN** 用户执行 `webmux ls --json`，其中一个带聊天视图的会话在聊天视图
- **THEN** 该会话的 JSON 含 `view: "chat"`、`views: ["terminal","chat"]` 与非空的 `agent`，普通会话含 `views: ["terminal"]` 与 `agent: null`，所有会话都不含 `hybrid`
