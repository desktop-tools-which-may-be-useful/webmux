## MODIFIED Requirements

### Requirement: 启动配置数据模型与命令快照

系统 SHALL 为每条启动配置保存服务端生成的 `id`、`name`、完整命令 `cmd`、环境变量 `env`、可选的 `icon` 与 `preview`、可选的布尔字段 `chat`（`launch-configs.json` 中以键 `hybrid` 保存）、`createdAt` 与 `updatedAt`。`cmd` SHALL 是保存时的完整 argv；服务端 SHALL NOT 在启动时重新解析或重建命令。

`name` 去除首尾空白后 SHALL 为 1–64 个 Unicode 字符，并在启动配置之间唯一；比较 SHALL 使用去空白后的精确值。

- `cmd` SHALL 包含 1–128 项，第一项 SHALL 非空，单项 SHALL NOT 超过 8 KiB。
- `env` SHALL 最多 64 项，值 SHALL NOT 超过 8 KiB，名称与值 SHALL 满足 `session.ValidateEnv` 的现有规则，包括 `WEBMUX_SESSION_ID` 保留。

`id` 与 `createdAt` SHALL 由服务端生成且客户端不可修改；`updatedAt` SHALL 在每次成功更新时由服务端刷新。时间 SHALL 以 UTC RFC 3339 表示。每条配置的 `env` 在序列化时 SHALL 为对象，空映射 SHALL 表示为 `{}`。单个存储 SHALL 最多保存 100 条配置。

#### Scenario: 保存后不随外部变化

- **WHEN** 用户保存启动配置 `["claude", "--resume"]`，之后卸载或升级本机的 `claude`
- **THEN** 该启动配置仍保存并执行创建时的完整 argv `["claude", "--resume"]`

#### Scenario: 名称与数量边界

- **WHEN** 用户提交去空白后为空的名称或第 101 条配置
- **THEN** 服务端以 400 拒绝，已有配置不变

#### Scenario: 重名

- **WHEN** 用户提交与已有启动配置去空白后同名的配置
- **THEN** 服务端以 409 拒绝，已有配置不变

#### Scenario: 空参数保留

- **WHEN** 用户保存的命令文本包含一个合法的空参数
- **THEN** 保存的 `cmd` 保留该空字符串，启动时按 argv 原样传给 PTY

### Requirement: 启动配置 API

服务端 SHALL 提供以下已认证接口：

- `GET /api/launch-configs`：返回按创建顺序排列的配置摘要数组，存储不可用时返回 503；
- `GET /api/launch-configs/{id}`：返回单条完整配置；
- `POST /api/launch-configs`：接收 `{name, icon?, cmd, env?, preview?, chat?}`，成功时返回 201 与完整配置；
- `PUT /api/launch-configs/{id}`：以同样的字段整体替换可编辑字段，成功时返回 200 与完整配置；
- `DELETE /api/launch-configs/{id}`：成功时返回 204。

摘要 SHALL 为 `{id, name, icon?, cmd, envNames, preview, chat, chatSupport, createdAt, updatedAt}`，其中 `envNames` 为按字典序排列的变量名数组，SHALL NOT 包含任何环境变量值。
完整配置 SHALL 为 `{id, name, icon?, cmd, env, preview, chat, chatSupport, createdAt, updatedAt}`。`icon` 与 `preview`（`null` 或 `{lines, skip}`）的取值见 `docs/protocol/http-sessions.md`。

创建和更新 SHALL 校验数据模型中的全部限制。请求内容与更新语义：

- 缺省 `env` SHALL 视为空对象；`PUT` 为整体替换，未提供的 `env` SHALL 清空原值。
- 请求体 SHALL 拒绝未知字段且只接受一个 JSON 对象；请求体中的 `hybrid` SHALL 视为未知字段。
- 更新 SHALL NOT 修改 `id` 与 `createdAt`。

错误状态码：

- 名称冲突 SHALL 返回 409。
- `id` 不存在 SHALL 返回 404。
- 字段缺失、为空或非法 SHALL 返回 400。
- 落盘失败 SHALL 返回 500 并保持原数据。
- 存储不可用、文件损坏或锁冲突 SHALL 返回 503，错误文本 SHALL 包含原因与状态文件路径。

这些接口 SHALL 继承 `/api/*` 现有的认证、Host 与 Origin 校验，并 SHALL NOT 在错误信息、日志或诊断输出中记录环境变量值。

#### Scenario: 创建与更新

- **WHEN** 用户创建 `{name: "Claude 恢复", cmd: ["claude"], env: {}}`，随后整体更新名称与命令
- **THEN** 创建返回 201，更新返回 200，`id` 与 `createdAt` 不变，`updatedAt` 刷新，列表中顺序不变

#### Scenario: 列表不含环境变量值

- **WHEN** 已保存配置的 `env` 为 `{"API_KEY": "secret-value"}`，客户端读取 `GET /api/launch-configs`
- **THEN** 响应包含 `envNames: ["API_KEY"]`，响应正文任何位置都不出现 `secret-value`；读取 `GET /api/launch-configs/{id}` 才返回该值

#### Scenario: 重名与未知 ID

- **WHEN** 用户提交与已有配置去空白后同名的配置，或读取、更新不存在的 `id`
- **THEN** 分别返回 409 与 404，已有数据不被修改

#### Scenario: 认证与损坏存储

- **WHEN** 未认证请求访问启动配置接口，或文件损坏时已认证用户读取列表
- **THEN** 未认证请求按现有规则返回 401/403；已认证请求在文件损坏时返回 503 及包含状态文件路径的错误

### Requirement: 新建会话页选择启动配置

新建会话对话框 SHALL 通过单一命令选择器选择命令来源：列表包含已保存的启动配置与末尾的“自定义命令…”，二者都只使用自定义命令；系统 SHALL NOT 提供内置预设，SHALL NOT 再以“预先输入 / 手动填写”Tab 区分。提交内容取决于所选来源：

- 保存的启动配置：提交 SHALL 发送 `configId` 与本次选择的 `cwd`、`workspace` 或 `newWorktree`，可选发送 `name` 与 `view`，SHALL NOT 同时发送 `cmd` 或 `env`。
- 自定义命令：提交 SHALL 发送完整 `cmd` 及本次选择的 `cwd`、`workspace` 或 `newWorktree`，该命令不写入服务端。

页面 SHALL 只请求启动配置摘要，SHALL NOT 请求环境变量值。在配置与自定义命令之间切换 SHALL 保留已选配置与已输入命令，但只提交当前来源。

页面 SHALL 在当前浏览器以键 `lastLaunch`（`config:<ID>` 或 `custom`）记住最近一次成功创建所用的启动方案、以 `customCommand` 记住自定义命令，SHALL NOT 缓存配置内容；这些键的格式 SHALL 保持兼容。记住的配置已不存在时 SHALL NOT 选中它。默认填入与失效处理的对话框行为见 `new-session-dialog` 规格。

没有启动配置时，命令选择器 SHALL 显示空状态与前往 `/settings#launch-configs` 的链接，并默认选中“自定义命令…”；启动配置不可用时 SHALL 说明原因。启动配置的加载或失败 SHALL NOT 阻塞自定义命令；加载期间保存的配置不可选，避免发送空的 `configId`。

#### Scenario: 从配置启动

- **WHEN** 用户选择名为“Claude 恢复”的启动配置与目录后提交
- **THEN** 请求只包含 `configId`、目标目录与会话名，会话按该配置保存的 argv 与 env 启动

#### Scenario: 手动命令照常启动

- **WHEN** 用户在命令选择器选择“自定义命令…”并输入完整命令后提交
- **THEN** 请求包含 `cmd` 与目标目录，不包含 `configId`，命令不写入服务端

#### Scenario: 存储不可用时仍可启动

- **WHEN** 启动配置接口返回 503
- **THEN** 命令选择器显示不可用原因，“自定义命令…”仍可选择并启动会话

### Requirement: 启动配置的混合模式开关

启动配置 SHALL 支持可选的布尔字段 `chat`（界面称为“混合模式”），缺省为 `false`。HTTP 请求与响应 SHALL 使用 `chat`；`launch-configs.json` 中 SHALL 继续以键 `hybrid` 保存，由存储层映射。

- 保存时为 `false` 的字段 SHALL 不写入文件。
- `launch-configs.json` 的版本 SHALL 保持 2；不含该键的已有文件 SHALL 照常读取，所有配置视为未勾选。

`chat` 为 `true` 时，服务端 SHALL 按 `hybrid-agent-profiles` 校验 argv，校验不通过时 SHALL 返回 400
并给出原因，已有数据不变。校验 SHALL NOT 检查第三方连接器是否已安装。

新安装预置的 Antigravity CLI（`agy`）、OpenCode、omp、Codex CLI 与 Claude Code 配置 SHALL 为
`chat: true`，其 argv（含各自的自动批准参数）SHALL 通过 `hybrid-agent-profiles` 校验。
修改已勾选配置的 argv 使其不再受支持时，保存 SHALL 返回 400，用户须先取消勾选。

新安装预置的 agent 配置 SHALL 带上各自的自动批准参数，使工具调用无需逐次批准：

- Claude Code：`--dangerously-skip-permissions`
- Codex CLI：`--dangerously-bypass-approvals-and-sandbox`
- Antigravity CLI（`agy`）：`--dangerously-skip-permissions`
- OpenCode：`--auto`
- omp：`--auto-approve`

预置的 Antigravity CLI 配置图标 SHALL 为 `antigravity`；已有配置文件中保存的 `geminicli` 图标值
SHALL 继续被接受并正常渲染。Shell 配置 SHALL 不变。已存在的 `launch-configs.json` SHALL NOT 被改写。

#### Scenario: 新安装的预置配置

- **WHEN** 服务首次启动且 `launch-configs.json` 不存在
- **THEN** 预置的 Antigravity CLI 配置名称为 `"Antigravity CLI"`、图标为 `"antigravity"`、
  argv 为 `["bash","-ic","exec -- \"$@\"","webmux","agy","--dangerously-skip-permissions"]`
  （服务端 `$SHELL` 为 Bash 时）、`chat` 为 `true`、`chatSupport` 为 `{supported: true, agent: "agy"}`；
  OpenCode、omp、Codex CLI 与 Claude Code 配置同样为混合配置；文件中这些配置以 `"hybrid": true` 保存

#### Scenario: 已有安装不改写

- **WHEN** 升级前已存在 `launch-configs.json`（含旧的 `"icon": "geminicli"` 配置）
- **THEN** 服务正常加载且不修改其中任何配置，包括名称、图标、`hybrid` 键与命令参数

#### Scenario: 勾选 omp 配置

- **WHEN** 用户对自己的 omp 配置把 `chat` 改为 `true` 并保存
- **THEN** 保存成功，摘要中 `chat` 为 `true`，`chatSupport` 为 `{supported: true, agent: "omp"}`，文件中该配置为 `"hybrid": true`

#### Scenario: 未安装连接器时勾选 Codex 配置

- **WHEN** 服务上没有 `codex-acp`，用户对 Codex 配置把 `chat` 改为 `true` 并保存
- **THEN** 保存成功；连接器检查只在启动 ACP 进程之前进行

#### Scenario: 旧文件

- **WHEN** 升级前保存的 `launch-configs.json` 不含 `hybrid` 键
- **THEN** 服务正常读取，所有配置的 `chat` 为 `false`，文件不被改写

#### Scenario: 不支持的命令

- **WHEN** 用户对 Gemini CLI 配置提交 `chat: true`
- **THEN** 服务端返回 400，原因为“未识别到支持的 Agent”，已有数据不变

#### Scenario: 请求体使用旧字段名

- **WHEN** 客户端创建启动配置时在请求体中提供 `hybrid: true`
- **THEN** 服务端以未知字段返回 400，已有数据不变

### Requirement: 启动配置的混合支持信息

启动配置的完整配置、摘要、创建与更新请求 SHALL 包含 `chat`。完整配置与摘要 SHALL 额外包含只读的 `chatSupport: {supported, agent?, reason?, connector?}`，由服务端按当前 argv 计算；请求体中提供 `chatSupport` 或 `hybridSupport` SHALL 视为未知字段而返回 400。

`connector` 只在 `supported` 为 `true` 且档案声明了第三方连接器时出现，内容为 `{executable, title, installCommand, docsUrl}`，
SHALL 只来自档案，SHALL NOT 反映连接器是否已安装。

#### Scenario: 请求体携带只读字段

- **WHEN** 客户端创建或更新启动配置时在请求体中提供 `chatSupport`
- **THEN** 服务端返回 400，已有数据不变

#### Scenario: Codex 配置的连接器信息

- **WHEN** 客户端调用 `POST /api/launch-configs/chat-support {cmd: ["codex"]}`
- **THEN** 返回 `{supported: true, agent: "codex", connector: {executable: "codex-acp", …}}`，无论服务上是否安装了 `codex-acp`

### Requirement: 设置页的混合模式勾选框

设置页的启动配置编辑器 SHALL 提供“混合模式（聊天 ↔ 终端）”勾选框，对应启动配置的 `chat` 字段。

- 勾选框是否可用 SHALL 由服务端判定，浏览器 SHALL NOT 复制档案规则。打开已有配置时使用其 `chatSupport`；编辑命令时，设置页 SHALL 去抖调用 `POST /api/launch-configs/chat-support {cmd}`（已认证，只读，返回同样的 `chatSupport`）。系统 SHALL NOT 提供 `POST /api/launch-configs/hybrid-support`。
- 不可用时，勾选框 SHALL 禁用，并在旁边显示原因。
- 可用且 `chatSupport.connector` 存在时，勾选框旁 SHALL 说明“聊天端需要另外安装 `<可执行文件>`”，并显示可复制的安装命令与说明链接。
- 启动配置列表 SHALL 为已勾选的配置显示“混合”标记。

#### Scenario: Shell 配置

- **WHEN** 用户打开 Shell 配置的编辑器
- **THEN** 混合模式勾选框不可用，并显示“未识别到支持的 Agent”

#### Scenario: Codex 配置

- **WHEN** 用户打开预置的 Codex CLI 配置编辑器
- **THEN** 混合模式勾选框可用且已勾选，旁边说明聊天端需要另外安装 `codex-acp` 并显示安装命令
