# launch-configs Specification

## Purpose

服务器持久化的启动配置让同一 WebMux 实例上的用户以“名称 + 完整 argv + 环境变量”保存自己的启动方式，跨浏览器、跨设备共享，是新建会话页与 CLI（`webmux run --launch-config`）的稳定输入。启动配置只描述如何启动会话，不改变 PTY、终端连接、回放或会话清理的生命周期。
## Requirements
### Requirement: 启动配置数据模型与命令快照

系统 SHALL 为每条启动配置保存服务端生成的 `id`、`name`、完整命令 `cmd`、环境变量 `env`、可选的 `icon` 与 `preview`、可选的布尔字段 `hybrid`、`createdAt` 与 `updatedAt`。`cmd` SHALL 是保存时的完整 argv；服务端 SHALL NOT 在启动时重新解析或重建命令。

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

### Requirement: 首次启动预置默认启动配置

系统 SHALL NOT 提供内置预设：服务端 SHALL NOT 读取 TOML `[[presets]]` 与 `shell`，SHALL NOT 提供 `GET /api/presets`，创建会话请求 SHALL NOT 接受 `preset` 与 `args`。

服务首次启动且 `<state_dir>/launch-configs.json` 不存在时，服务端 SHALL 写入 11 条普通启动配置：Shell、Claude Code、Codex CLI、Gemini CLI、Cursor CLI、GitHub Copilot CLI、Open Code、Qwen Code、Kimi Code CLI、CodeBuddy Code 与 omp。
这些配置 SHALL 可像其他配置一样编辑或删除；文件已存在（包括删空后的空列表）时 SHALL NOT 补写或改写。

#### Scenario: 首次启动写入默认配置

- **WHEN** 服务首次启动且状态目录中没有 `launch-configs.json`
- **THEN** 服务创建该文件并写入 11 条默认配置，列表可经 `GET /api/launch-configs` 读取

#### Scenario: 删空后不再补写

- **WHEN** 用户删除全部启动配置后重启服务
- **THEN** 启动配置列表保持为空，默认配置不会重新出现

### Requirement: 服务端持久化、文件锁与故障隔离

启动配置 SHALL 保存于 `<state_dir>/launch-configs.json`，文件权限 SHALL 为 0600，状态目录权限 SHALL 为 0700。每次变更 SHALL 通过同目录临时文件写入、`fsync`、关闭、`rename` 与目录同步原子替换；保存失败时 SHALL 回滚内存状态并返回错误。

服务端 SHALL 在启动时对 `<state_dir>/launch-configs.json.lock`（0600）取得独占非阻塞文件锁并在进程生命周期内持有，之后才读取或写入启动配置文件。无法取得锁时，本实例的启动配置功能 SHALL 不可用，SHALL NOT 读取或写入 `launch-configs.json`，服务的其他功能 SHALL 照常启动。

服务端 SHALL 在启动时加载一次，此后 CRUD 立即生效，SHALL NOT 要求重启，也 SHALL NOT 写回 `config.toml`。

状态文件无法解析或不满足数据模型时：

- 服务端 SHALL 保留原文件，SHALL NOT 覆盖、改名或重建。
- 启动配置接口 SHALL 返回 503。
- 手动命令启动会话、终端连接、WebSocket 与其他设置 SHALL NOT 因此不可用。

状态目录或文件不可读写时 SHALL 同样只禁用启动配置功能。修复文件、删除文件或停止占用锁的实例后，服务端 SHALL 需要重启才能重新加载。

#### Scenario: 两个实例共用状态目录

- **WHEN** 第一个实例已在运行，第二个实例以同一 `state_dir` 启动
- **THEN** 第二个实例的启动配置接口返回 503 并说明状态目录已被另一实例使用，`launch-configs.json` 只由第一个实例修改

#### Scenario: 损坏文件不影响会话

- **WHEN** `launch-configs.json` 内容损坏，用户打开设置页并以手动填写的命令启动会话
- **THEN** 启动配置接口返回 503 且原文件保持存在，会话仍可创建、连接和退出

#### Scenario: 保存失败回滚

- **WHEN** 创建配置时临时文件写入或重命名失败
- **THEN** 接口返回错误，内存中不出现该配置，之前已加载的配置保持可用

### Requirement: 启动配置 API

服务端 SHALL 提供以下已认证接口：

- `GET /api/launch-configs`：返回按创建顺序排列的配置摘要数组，存储不可用时返回 503；
- `GET /api/launch-configs/{id}`：返回单条完整配置；
- `POST /api/launch-configs`：接收 `{name, icon?, cmd, env?, preview?, hybrid?}`，成功时返回 201 与完整配置；
- `PUT /api/launch-configs/{id}`：以同样的字段整体替换可编辑字段，成功时返回 200 与完整配置；
- `DELETE /api/launch-configs/{id}`：成功时返回 204。

摘要 SHALL 为 `{id, name, icon?, cmd, envNames, preview, hybrid, hybridSupport, createdAt, updatedAt}`，其中 `envNames` 为按字典序排列的变量名数组，SHALL NOT 包含任何环境变量值。
完整配置 SHALL 为 `{id, name, icon?, cmd, env, preview, hybrid, hybridSupport, createdAt, updatedAt}`。`icon` 与 `preview`（`null` 或 `{lines, skip}`）的取值见 `docs/protocol/http-sessions.md`。

创建和更新 SHALL 校验数据模型中的全部限制。请求内容与更新语义：

- 缺省 `env` SHALL 视为空对象；`PUT` 为整体替换，未提供的 `env` SHALL 清空原值。
- 请求体 SHALL 拒绝未知字段且只接受一个 JSON 对象。
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

### Requirement: 设置页管理启动配置

设置页 SHALL 提供“启动配置”卡片，列出每条配置的名称、命令预览与环境变量数量，并提供新建、编辑、删除（需确认）与空状态。页面 SHALL 支持 `#launch-configs` 深链接滚动定位，并在达到 100 条上限时阻止新增并提示。

说明文字：

- 卡片 SHALL 说明启动配置保存在服务端、跨设备共享，没有内置预设。
- 页面顶部说明 SHALL 区分仅保存在当前浏览器的设置与保存在服务端的启动配置。

编辑器 SHALL 包含名称、命令与环境变量三部分。编辑已有配置时 SHALL 先读取单条完整配置，读取完成前 SHALL NOT 允许保存。

命令：

- SHALL 使用现有 `parseArgs` 解析，命令文本与 argv 的往返 SHALL 保留空参数、空白、引号、反斜杠与 `$`。
- SHALL 在命令框旁提供“加载 shell 启动文件”选项（见下文），其余命令由用户自行填写。

环境变量：

- SHALL 支持按行添加/删除。
- 名称 SHALL 使用与 `session.ValidateEnv` 相同的规则校验并拒绝 `WEBMUX_SESSION_ID`。
- 值 SHALL 默认遮挡并提供可访问名称明确的显示/隐藏控制。

前端校验 SHALL 与服务端的数量与长度限制一致。

界面 SHALL 提示启动配置保存的是创建时的命令副本，修改或删除配置不会影响已运行的会话。列表与命令预览 SHALL NOT 显示环境变量值，删除确认 SHALL NOT 包含值。

存储不可用时，设置页 SHALL 显示不可用原因与状态文件路径，禁用写入，并保持其他浏览器本地设置可用。保存失败 SHALL 保留用户输入：重名显示 409 原因，存储不可用显示 503 原因。

#### Scenario: 创建带环境变量的配置

- **WHEN** 用户在设置页新建配置，填写名称与命令、添加一条环境变量并保存
- **THEN** 列表出现该配置，环境变量值默认遮挡，数量正确，命令预览不包含值

#### Scenario: 编辑保留环境变量

- **WHEN** 用户打开一条含环境变量的配置只修改名称后保存
- **THEN** 保存请求包含原有全部环境变量，配置的 `env` 不变

#### Scenario: 命令往返

- **WHEN** 用户编辑的命令包含空格、单双引号、反斜杠、`$` 或空参数
- **THEN** 用 `joinArgs` 渲染的文本再次用 `parseArgs` 解析得到完全相同的 argv

#### Scenario: 删除与空状态

- **WHEN** 用户确认删除最后一条配置
- **THEN** 配置被删除，列表显示空状态与新建入口

#### Scenario: 存储损坏

- **WHEN** 启动配置文件损坏后用户打开设置页
- **THEN** 卡片显示存储不可用及文件路径，新建、编辑与删除不可用，快捷指令与输出预览仍可修改

### Requirement: 新建会话页选择启动配置

新建会话页 SHALL 通过“预先输入”与“手动填写”两个 Tab 区分命令来源，两者都只使用自定义命令；系统 SHALL NOT 提供内置预设。提交内容取决于所选来源：

- 预先输入：提交 SHALL 发送 `configId` 与本次选择的 `cwd` 或 `workspace`，可选发送 `name` 与 `startSide`，SHALL NOT 同时发送 `cmd` 或 `env`。
- 手动填写：提交 SHALL 发送完整 `cmd` 及本次选择的 `cwd` 或 `workspace`，该命令不写入服务端。

页面 SHALL 只请求启动配置摘要，SHALL NOT 请求环境变量值。切换 Tab SHALL 保留已选配置与手动输入，但只提交当前来源。

页面 SHALL 在当前浏览器以键 `lastLaunch`（`config:<ID>` 或 `custom`）记住上次选择、以 `customCommand` 记住手动命令，SHALL NOT 缓存配置内容。记住的配置已不存在时 SHALL NOT 选中它。

没有启动配置时，“预先输入”SHALL 显示空状态与前往 `/settings#launch-configs` 的链接；启动配置不可用时 SHALL 说明原因。启动配置的加载或失败 SHALL NOT 阻塞手动填写；加载期间启动配置选择 SHALL 不可用，避免发送空的 `configId`。

#### Scenario: 从配置启动

- **WHEN** 用户选择名为“Claude 恢复”的启动配置与目录后提交
- **THEN** 请求只包含 `configId`、目标目录与会话名，会话按该配置保存的 argv 与 env 启动

#### Scenario: 手动命令照常启动

- **WHEN** 用户在“手动填写”输入完整命令后提交
- **THEN** 请求包含 `cmd` 与目标目录，不包含 `configId`，命令不写入服务端

#### Scenario: 存储不可用时仍可启动

- **WHEN** 启动配置接口返回 503
- **THEN** “预先输入”显示不可用原因，“手动填写”仍可启动会话

### Requirement: 设备间共享与本地选择边界

启动配置 SHALL 由服务端存储并在同一实例的所有已认证浏览器与设备间共享。浏览器 SHALL 只保存上次选择与临时命令内容；SHALL NOT 在本地保存另一份可编辑的配置列表。

配置更新与删除 SHALL 在下一次读取时反映到其他设备；本期 SHALL NOT 提供实时推送、冲突合并或并发编辑提示。删除配置 SHALL NOT 影响已运行会话。

#### Scenario: 跨设备复用

- **WHEN** 用户在桌面浏览器创建启动配置，随后在手机上打开同一实例的新建会话页
- **THEN** 手机读取到该配置并可选择启动

#### Scenario: 删除后其他设备刷新

- **WHEN** 一个设备删除配置，另一个设备重新打开新建会话页
- **THEN** 已删除配置不再出现；其他设备之前记住的该配置若失效，则不再被选中

### Requirement: 启动配置的混合模式开关

启动配置 SHALL 支持可选的布尔字段 `hybrid`，缺省为 `false`。

- 保存时为 `false` 的字段 SHALL 不写入文件。
- `launch-configs.json` 的版本 SHALL 保持 2；不含该字段的已有文件 SHALL 照常读取，所有配置视为未勾选。

`hybrid` 为 `true` 时，服务端 SHALL 按 `hybrid-agent-profiles` 校验 argv，校验不通过时 SHALL 返回 400 并给出原因，已有数据不变。
校验 SHALL NOT 检查第三方连接器是否已安装。

新安装预置的 OpenCode、omp、Codex CLI 与 Claude Code 配置 SHALL 为 `hybrid: true`，其 argv（含各自的自动批准参数）SHALL 通过 `hybrid-agent-profiles` 校验。修改已勾选配置的 argv 使其不再受支持时，保存 SHALL 返回 400，用户须先取消勾选。

新安装预置的 agent 配置 SHALL 带上各自的自动批准参数，使工具调用无需逐次批准：

- Claude Code：`--dangerously-skip-permissions`
- Codex CLI：`--dangerously-bypass-approvals-and-sandbox`
- Gemini CLI：`--yolo`
- OpenCode：`--auto`
- omp：`--auto-approve`

Shell 配置 SHALL 不变。已存在的 `launch-configs.json` SHALL NOT 被改写。

#### Scenario: 新安装的预置配置

- **WHEN** 服务首次启动且 `launch-configs.json` 不存在
- **THEN** 预置的 omp 配置 argv 为 `["bash","-ic","exec -- \"$@\"","webmux","omp","--auto-approve"]`（服务端 `$SHELL` 为 Bash 时），`hybrid` 为 `true`，`hybridSupport` 为 `{supported: true, agent: "omp"}`；
  OpenCode、Codex CLI 与 Claude Code 配置同样为混合配置，Codex CLI 与 Claude Code 的 `hybridSupport` 带有各自的连接器信息；
  Gemini CLI 配置带自动批准参数但 `hybrid` 为 `false`

#### Scenario: 已有安装不改写

- **WHEN** 升级前已存在 `launch-configs.json`
- **THEN** 服务不修改其中任何配置，包括 `hybrid` 与命令参数

#### Scenario: 勾选 omp 配置

- **WHEN** 用户对自己的 omp 配置把 `hybrid` 改为 `true` 并保存
- **THEN** 保存成功，摘要中 `hybrid` 为 `true`，`hybridSupport` 为 `{supported: true, agent: "omp"}`

#### Scenario: 未安装连接器时勾选 Codex 配置

- **WHEN** 服务上没有 `codex-acp`，用户对 Codex 配置把 `hybrid` 改为 `true` 并保存
- **THEN** 保存成功；连接器检查只在启动 ACP 进程之前进行

#### Scenario: 旧文件

- **WHEN** 升级前保存的 `launch-configs.json` 不含 `hybrid` 字段
- **THEN** 服务正常读取，所有配置的 `hybrid` 为 `false`，文件不被改写

#### Scenario: 不支持的命令

- **WHEN** 用户对 Gemini CLI 配置提交 `hybrid: true`
- **THEN** 服务端返回 400，原因为“未识别到支持的 Agent”，已有数据不变

### Requirement: 启动配置的混合支持信息

启动配置的完整配置、摘要、创建与更新请求 SHALL 包含 `hybrid`。完整配置与摘要 SHALL 额外包含只读的 `hybridSupport: {supported, agent?, reason?, connector?}`，由服务端按当前 argv 计算；请求体中提供 `hybridSupport` SHALL 视为未知字段而返回 400。

`connector` 只在 `supported` 为 `true` 且档案声明了第三方连接器时出现，内容为 `{executable, title, installCommand, docsUrl}`，
SHALL 只来自档案，SHALL NOT 反映连接器是否已安装。

#### Scenario: 请求体携带只读字段

- **WHEN** 客户端创建或更新启动配置时在请求体中提供 `hybridSupport`
- **THEN** 服务端返回 400，已有数据不变

#### Scenario: Codex 配置的连接器信息

- **WHEN** 客户端调用 `POST /api/launch-configs/hybrid-support {cmd: ["codex"]}`
- **THEN** 返回 `{supported: true, agent: "codex", connector: {executable: "codex-acp", …}}`，无论服务上是否安装了 `codex-acp`

### Requirement: 设置页的混合模式勾选框

设置页的启动配置编辑器 SHALL 提供“混合模式（聊天 ↔ 终端）”勾选框。

- 勾选框是否可用 SHALL 由服务端判定，浏览器 SHALL NOT 复制档案规则。打开已有配置时使用其 `hybridSupport`；编辑命令时，设置页 SHALL 去抖调用 `POST /api/launch-configs/hybrid-support {cmd}`（已认证，只读，返回同样的 `hybridSupport`）。
- 不可用时，勾选框 SHALL 禁用，并在旁边显示原因。
- 可用且 `hybridSupport.connector` 存在时，勾选框旁 SHALL 说明“聊天端需要另外安装 `<可执行文件>`”，并显示可复制的安装命令与说明链接。
- 启动配置列表 SHALL 为已勾选的配置显示“混合”标记。

#### Scenario: Shell 配置

- **WHEN** 用户打开 Shell 配置的编辑器
- **THEN** 混合模式勾选框不可用，并显示“未识别到支持的 Agent”

#### Scenario: Codex 配置

- **WHEN** 用户打开预置的 Codex CLI 配置编辑器
- **THEN** 混合模式勾选框可用且已勾选，旁边说明聊天端需要另外安装 `codex-acp` 并显示安装命令

### Requirement: 启动文件选择与包装命令

启动配置编辑器 SHALL 在命令下方提供“加载 shell 启动文件”复选框与 shell 选择（Bash `~/.bashrc` / Zsh `~/.zshrc`），并 SHALL 只在勾选后显示并应用所选 shell。

- 保存时 SHALL 以完整 argv 记录选择：Bash 为 `["bash","-ic","exec -- \"$@\"","webmux",<命令>,<参数>…]`，Zsh 为 `["zsh","-ic","exec -- \"$@\"","webmux",<命令>,<参数>…]`。
- 编辑器 SHALL 只识别上述两种精确前缀；重新编辑 SHALL 还原为普通命令并保持勾选与所选 shell，保存 SHALL NOT 重复包装；取消勾选 SHALL 恢复直接执行。
- 包装参数 SHALL 计入 `cmd` 的 1–128 项上限；SHALL NOT 展开 `$HOME`、管道或命令替换。
- 服务端 SHALL NOT 新增持久字段，`launch-configs.json` 版本 SHALL 保持 `2`；选择由 argv 表达。

#### Scenario: Zsh 往返

- **WHEN** 用户把命令 `omp --auto-approve` 勾选“加载 shell 启动文件”、选择 Zsh 并保存，之后重新打开编辑
- **THEN** 保存的 argv 为 `["zsh","-ic","exec -- \"$@\"","webmux","omp","--auto-approve"]`；编辑器显示普通命令、保持勾选与 Zsh，再次保存不重复包装

#### Scenario: 取消勾选

- **WHEN** 用户取消已包装配置的勾选并保存
- **THEN** 保存的 argv 只有普通命令与参数

#### Scenario: 用户手写其他包装

- **WHEN** 配置的 argv 为 `["sh","-c","opencode"]` 或 `["fish","-c","opencode"]`
- **THEN** 编辑器不把它们识别为“加载 shell 启动文件”，保存与执行保持原样

### Requirement: 默认启动 Shell

服务端 SHALL 在已验证的 `GET /api/config` 响应中提供只读字段 `defaultShell`，取值为 `"bash"`、`"zsh"` 或空字符串，由服务进程 `$SHELL` 的 basename 决定：

- `bash` → `"bash"`；
- `zsh` → `"zsh"`；
- 未设置或为空 → `"bash"`，保持既有默认行为；
- 其他 shell（fish、nu、sh 等）→ 空字符串。

该字段 SHALL NOT 包含路径、环境变量值或可执行文件探测结果。设置页 SHALL 用 `defaultShell` 预选启动文件的 shell；其值为空时 SHALL 在用户勾选后回退到 Bash。打开已有包装配置时 SHALL 以 argv 中的 shell 为准，忽略 `defaultShell`。

#### Scenario: macOS launchd

- **WHEN** 服务以 `SHELL=/bin/zsh` 启动，用户新建启动配置并勾选“加载 shell 启动文件”
- **THEN** `GET /api/config` 返回 `defaultShell: "zsh"`，编辑器预选 Zsh

#### Scenario: systemd 未设置 SHELL

- **WHEN** 服务环境没有 `SHELL`
- **THEN** `defaultShell` 为 `"bash"`，编辑器与预置继续使用 Bash

#### Scenario: fish

- **WHEN** 服务环境的 `SHELL` 为 `/usr/bin/fish`
- **THEN** `defaultShell` 为空字符串；用户勾选时编辑器回退到 Bash

