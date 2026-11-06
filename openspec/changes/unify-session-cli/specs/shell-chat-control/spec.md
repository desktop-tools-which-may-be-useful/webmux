## MODIFIED Requirements

### Requirement: 聊天命令的共同约定

`webmux chat <子命令>` SHALL 只包含聊天视图专有的操作：`status`、`watch`、`answer`、`set`、`permission` 与 `queue …`，并遵守 `shell-session-control` 的输出与退出码约定。
发送输入、中断、切换视图、列出、恢复与删除记录对所有会话通用，见 `shell-session-control` 与 `terminal-session-cli`；创建会话见 `shell-session-launch`。

- 目标会话没有聊天视图（`views` 不含 `chat`，或服务端返回 409 `no_chat_view`）时 SHALL 以 1 结束并说明原因。
- 需要聊天视图的子命令在会话处于终端视图时 SHALL 以 1 结束，并提示 `webmux view SESSION chat`。
- 每个写操作 SHALL 生成新的 `clientRequestId`，同一次调用内的重试 SHALL 复用它，使服务端去重生效。

#### Scenario: 普通终端会话

- **WHEN** 用户对一个 bash 会话执行 `webmux chat status <ID>`
- **THEN** 退出码 1，标准错误说明该会话没有聊天视图

#### Scenario: 会话在终端视图

- **WHEN** 带聊天视图的会话处于终端视图，用户执行 `webmux chat permission <ID> auto`
- **THEN** 退出码 1，标准错误提示 `webmux view <ID> chat`

### Requirement: 观察聊天

- `webmux chat watch SESSION [--json] [--history]` SHALL 连接聊天 WebSocket，直到被中断：
  - `--json` 时 SHALL 把每个服务端帧作为一行 JSON 输出；
  - 默认 SHALL 跳过连接时回放的历史事件，`--history` 时包含它们；
  - 默认 SHALL 以文本形式输出用户与 agent 消息、工具调用标题与状态，以及回合开始和结束。
- `webmux chat status SESSION [--json]` SHALL 输出一次快照后退出，内容包括当前视图（取自会话信息的 `view` 与 `transition`）、当前回合状态、队列长度和待回答请求列表（ID、类型、摘要、选项）。

#### Scenario: 列出待回答请求

- **WHEN** agent 正在等待授权，用户执行 `webmux chat status <ID> --json`
- **THEN** 输出中包含该请求的 ID 与可选的 `optionId`

### Requirement: 控制回合与请求

- `webmux chat answer SESSION REQUEST_ID (--option ID | --content JSON | --decline | --cancel)` SHALL 调用回答接口：
  授权请求用 `--option` 选择选项；问题表单用 `--content` 提交字段值对象、`--decline` 拒绝；`--cancel` 对两者都以取消作答。
  选项与请求类型不符时 SHALL 以 2 结束；请求已被回答或不存在时 SHALL 以 1 结束。
  （`--json` 在所有命令中都是输出开关，因此结构化内容用 `--content`。）
- 结束当前回合使用 `webmux interrupt SESSION`，切换视图使用 `webmux view SESSION terminal|chat`（见 `shell-session-control`）。

#### Scenario: 代为批准一次

- **WHEN** 用户执行 `webmux chat answer <ID> <请求 ID> --option allow_once`
- **THEN** agent 继续执行，所有设备上的请求卡片同时关闭

#### Scenario: 切到聊天后发送

- **WHEN** 会话在终端视图，脚本先执行 `webmux view <ID> chat`，再执行 `webmux send <ID> 继续`
- **THEN** 切换完成后提示被发送，`webmux attach` 中的客户端按 US-Q13 被动分离

## REMOVED Requirements

### Requirement: 发送提示

**Reason**: 输入由服务端按视图分派，`webmux send` 对终端与聊天视图通用，不再需要聊天专用的发送命令。

**Migration**: `webmux chat send SESSION TEXT [--attach PATH] [--mode MODE] [--wait]` 改为 `webmux send SESSION TEXT [--attach-file PATH] [--mode steer] [--reply]`；`--auto-wait-requests`、`--timeout`、`--json` 同名保留。

### Requirement: 混合会话历史

**Reason**: 已归档会话并入统一的会话列表，恢复与删除记录成为所有会话的通用命令。

**Migration**: `webmux chat ls [--state S] [--archived] [--repo R]` 改为 `webmux ls [--state S] [--archived] [--repo R]`；
`webmux chat resume RECORD --side chat|terminal` 改为 `webmux resume SESSION --view chat|terminal`；`webmux chat forget RECORD [--yes]` 改为 `webmux rm SESSION --forget [--yes]`。
