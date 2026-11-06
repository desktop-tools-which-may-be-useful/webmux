# Spec Delta

## MODIFIED Requirements

### Requirement: 配置分层与同步范围

系统 SHALL 把用户偏好划分为三类，并 SHALL 只把第一类放进服务端设置文档与设置 API：

- 跨客户端同步：`theme`、`preview`、`codeReader.lineNumbers`、`codeReader.diffOverflow`、`quickCommands`、`homeWorktrees`、`forceSoftKeys`。
- 仅当前设备：浏览器中保存的终端字号（`fontSize`，屏幕尺寸因设备而异）、token 输入、Web Push 订阅标识、草稿与暂存正文、`draftPersistence` 开关、`recentDirs`、`lastLaunch`、`customCommand`、`clientId`、`clientLive`。
- 部署默认与全局策略：`listen`、`token`、`state_dir`、`allowed_origins`、`tunnel.*`、`default_cwd`、`scrollback_bytes`、`idle_after_seconds`、`prompt_patterns`、`agent_protocol`、`bell_attention`、`push.*`。

设置接口 SHALL NOT 返回或接受第二类与第三类字段；PATCH 中出现 `token`、`push`、`draft`、`listen` 等未知键时 SHALL 返回 400。服务端 SHALL NOT 因为本能力修改 `config.toml` 或改变上述 TOML 项的生效方式。

已有公网访问配置 SHALL 保持独立：`tunnel.*` 提供 TOML 部署默认值，现有专用服务通过 `tunnel-settings.json` 持久化并即时应用开关、模式、凭据和主机名的覆盖，`cloudflared` 与下载镜像仍只由 TOML 管理。通用设置文档、快照和更新帧 SHALL NOT 包含隧道凭据，本能力 SHALL NOT 改变既有隧道入口或优先级。

#### Scenario: 跨设备读取同一偏好

- **WHEN** 用户在桌面浏览器把主题改为深色并新增一条快捷指令，随后在手机上打开同一实例
- **THEN** 手机在认证后取得同一份设置快照，显示深色主题与该快捷指令，不依赖手机本地曾经保存的值

#### Scenario: 设备本地项不进入设置接口

- **WHEN** 客户端读取 `GET /api/settings` 或服务端写入 `settings.json`
- **THEN** 响应与文件都不包含浏览器 token、Web Push 订阅、草稿正文、最近目录或上次启动选择

#### Scenario: TOML 策略不经 API 修改

- **WHEN** 客户端在 PATCH 中提交 `listen`、`allowed_origins`、`tunnel` 或 `push`
- **THEN** 服务端返回 400，`config.toml` 与当前服务行为保持不变

### Requirement: 设置快照、修订号与校验

`GET /api/settings` 与成功的 PATCH SHALL 返回 `{revision, updatedAt, values}`：

- `revision` SHALL 为从 0 开始的单调递增整数，每次成功 PATCH 加 1。
- `updatedAt` SHALL 为 UTC RFC 3339 时间，未写入过时为空字符串。
- `values` SHALL 始终包含 `theme`、`preview`、`codeReader`、`quickCommands`、`homeWorktrees`、`forceSoftKeys`，未设置的键为 `null`。

设置值 SHALL 满足以下约束：

- `theme`：`"light"`、`"dark"` 或 `null`；
- `preview`：`{lines, skip}`，`lines` 为 1–50 的整数，`skip` 为 0–50 的整数，或 `null`；
- `codeReader`：`{lineNumbers, diffOverflow}`，`lineNumbers` 为 `"show"` 或 `"hide"`，`diffOverflow` 为 `"scroll"` 或 `"wrap"`，或 `null`；
- `quickCommands`：0–50 条，每条 `id` 为 1–64 字符且列表内唯一，`name` 去首尾空白后不超过 40 字符，`command` 去首尾空白后非空且不超过 4096 字符，`enter` 为布尔，或 `null`；
- `homeWorktrees`：对象，最多 200 个仓库，仓库 ID 为 1–128 字符，每个仓库最多 1000 个工作树 ID，每个 ID 为 1–128 字符，全部条目合计最多 2000 个，或 `null`；
- `forceSoftKeys`：布尔或 `null`。

- 所有字符串 SHALL 是有效 UTF-8；长度上限 SHALL 按浏览器 UTF-16 code unit 计数，以保证 Go 接受的值也通过客户端 schema。
- 快捷指令保存时 SHALL 把 CRLF/CR 统一为 LF 并移除正文末尾换行；服务端 SHALL 执行相同规范化。
- PATCH 请求体 SHALL 不超过 2 MiB；设置文件 SHALL 不超过 4 MiB。
- 任意值越界、类型错误或包含未知字段时 SHALL 返回 400，且 SHALL NOT 写入任何部分内容。
- 已退役的 `artifactPrompt`（临时文件夹提示词模板，其说明已移入 `{{webmux}}`）SHALL 视为未知字段：PATCH 中出现时返回 400；旧 `settings.json` 中遗留的该键 SHALL 在读取时忽略，并在下一次写入时移除。

#### Scenario: 默认快照

- **WHEN** 设置文件不存在，客户端首次读取设置
- **THEN** 返回 `revision: 0`、`updatedAt: ""`，上述各键全部为 `null`

#### Scenario: 快捷指令的未初始化与空列表

- **WHEN** 服务端 `quickCommands` 为 `null`，用户随后在设置页删除全部预置指令并保存
- **THEN** 第一次保存写入 `[]`，之后所有客户端显示空列表，不再自动补回任何预置指令

#### Scenario: 越界与非法值

- **WHEN** PATCH 同时包含合法主题与 51 条快捷指令，或包含 `theme: "blue"`、重复的快捷指令 ID、已退役的 `artifactPrompt`
- **THEN** 服务端返回 400，修订号与所有设置保持原值，后续读取看不到部分更新

#### Scenario: 请求体上限

- **WHEN** 客户端提交超过 2 MiB 的 PATCH
- **THEN** 服务端拒绝请求且不解析为设置，已有设置不变

### Requirement: 服务端设置不可用时的降级

浏览器 SHALL 把最近一次成功同步的快照保存在本地只读缓存中，用于首屏主题，以及设置服务不可用时展示最后已知值。缓存 SHALL 包含 `v`、`revision` 与 `values`，损坏、版本不符或字段非法时 SHALL 被忽略并删除。

设置接口返回 503 或网络不可达时，页面 SHALL 使用缓存或默认值只读展示：

- 设置页 SHALL 显示不可用原因与服务端返回的状态文件路径，禁用写入并提供重新读取入口。
- 快捷指令菜单 SHALL 使用缓存列表只读展示；没有缓存时显示预置指令并说明暂不可保存。

认证失败时 SHALL 进入现有登录流程。登录页可以使用缓存主题渲染，但 SHALL NOT 展示缓存中的快捷指令。

设置不可用 SHALL NOT 阻止会话创建、终端连接、终端输入输出、草稿或主页浏览。

服务恢复后 SHALL 重新读取完整快照并覆盖缓存；SHALL NOT 自动重放离线期间的修改，避免过期意图覆盖其他设备的新值。缓存写入失败（隐私模式或配额不足）SHALL NOT 影响当前页面使用。

#### Scenario: 设置文件损坏

- **WHEN** `settings.json` 损坏，用户打开设置页和终端
- **THEN** 设置页显示不可用原因与文件路径，写入被禁用；终端仍使用缓存中的主题与本机保存的字号，快捷指令菜单可用但不可保存

#### Scenario: 离线后恢复

- **WHEN** 手机断网期间用户打开设置页，随后网络恢复
- **THEN** 页面先显示缓存值，恢复后自动重新读取服务端快照并更新显示

#### Scenario: 首次使用就没有缓存

- **WHEN** 新浏览器第一次打开页面时设置接口不可用
- **THEN** 页面使用默认主题与“恢复会话”“新会话”“WebMux 工具”三条预置指令，不因设置失败而阻塞登录后的终端
