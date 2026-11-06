## ADDED Requirements

### Requirement: Qoder 图标

服务端图标白名单 SHALL 接受 `"qoder"`。前端图标下拉 SHALL 提供 `{value: "qoder", label: "Qoder"}`，
启动配置图标组件 SHALL 渲染自定义的 Qoder SVG 图标；未设置图标时，argv 的可执行文件基本名为
`qoder`、`qodercn`、`qoderclicn` 或 `qoder-cn` 的配置 SHALL 自动显示 `qoder` 图标。QoderCN 复用同一图标。

#### Scenario: 保存 qoder 图标

- **WHEN** 客户端保存 `{name: "Qoder", icon: "qoder", cmd: ["qoder"]}`
- **THEN** 保存成功，摘要中 `icon` 为 `"qoder"`

#### Scenario: 图标自动检测

- **WHEN** 未设置图标的配置 argv 可执行文件为 `qodercn`
- **THEN** 前端显示 `qoder` 图标

## MODIFIED Requirements

### Requirement: 首次启动预置默认启动配置

系统 SHALL NOT 提供内置预设：服务端 SHALL NOT 读取 TOML `[[presets]]` 与 `shell`，
SHALL NOT 提供 `GET /api/presets`，创建会话请求 SHALL NOT 接受 `preset` 与 `args`。

服务首次启动且 `<state_dir>/launch-configs.json` 不存在时，服务端 SHALL 写入 13 条普通启动配置：
Shell、Claude Code、Codex CLI、Antigravity CLI、Cursor CLI、GitHub Copilot CLI、Open Code、
Qwen Code、Kimi Code CLI、CodeBuddy Code、omp、Qoder 与 QoderCN。
这些配置 SHALL 可像其他配置一样编辑或删除；文件已存在（包括删空后的空列表）时 SHALL NOT 补写或改写。

#### Scenario: 首次启动写入默认配置

- **WHEN** 服务首次启动且状态目录中没有 `launch-configs.json`
- **THEN** 服务创建该文件并写入 13 条默认配置（含 Antigravity CLI、Qoder 与 QoderCN），列表可经 `GET /api/launch-configs` 读取

#### Scenario: 删空后不再补写

- **WHEN** 用户删除全部启动配置后重启服务
- **THEN** 启动配置列表保持为空，默认配置不会重新出现

### Requirement: 启动配置的混合模式开关

启动配置 SHALL 支持可选的布尔字段 `hybrid`，缺省为 `false`。

- 保存时为 `false` 的字段 SHALL 不写入文件。
- `launch-configs.json` 的版本 SHALL 保持 2；不含该字段的已有文件 SHALL 照常读取，所有配置视为未勾选。

`hybrid` 为 `true` 时，服务端 SHALL 按 `hybrid-agent-profiles` 校验 argv，校验不通过时 SHALL 返回 400
并给出原因，已有数据不变。校验 SHALL NOT 检查第三方连接器是否已安装。

新安装预置的 Antigravity CLI（`agy`）、OpenCode、omp、Codex CLI、Claude Code、Qoder 与 QoderCN 配置 SHALL 为
`hybrid: true`，其 argv（含各自的自动批准参数）SHALL 通过 `hybrid-agent-profiles` 校验。
修改已勾选配置的 argv 使其不再受支持时，保存 SHALL 返回 400，用户须先取消勾选。

新安装预置的 agent 配置 SHALL 带上各自的自动批准参数，使工具调用无需逐次批准：

- Claude Code：`--dangerously-skip-permissions`
- Codex CLI：`--dangerously-bypass-approvals-and-sandbox`
- Antigravity CLI（`agy`）：`--dangerously-skip-permissions`
- OpenCode：`--auto`
- omp：`--auto-approve`
- Qoder 与 QoderCN：`--dangerously-skip-permissions`

预置的 Antigravity CLI 配置图标 SHALL 为 `antigravity`；预置的 Qoder 与 QoderCN 配置图标 SHALL 为 `qoder`；
已有配置文件中保存的 `geminicli` 图标值 SHALL 继续被接受并正常渲染。Shell 配置 SHALL 不变。
已存在的 `launch-configs.json` SHALL NOT 被改写。

#### Scenario: 新安装的预置配置

- **WHEN** 服务首次启动且 `launch-configs.json` 不存在
- **THEN** 预置的 Antigravity CLI 配置名称为 `"Antigravity CLI"`、图标为 `"antigravity"`、
  argv 为 `["bash","-ic","exec -- \"$@\"","webmux","agy","--dangerously-skip-permissions"]`
  （服务端 `$SHELL` 为 Bash 时）、`hybrid` 为 `true`、`hybridSupport` 为 `{supported: true, agent: "agy"}`；
  OpenCode、omp、Codex CLI 与 Claude Code 配置同样为混合配置

#### Scenario: 新安装的 Qoder 与 QoderCN 预置

- **WHEN** 服务首次启动且 `launch-configs.json` 不存在，服务端 `$SHELL` 为 Bash
- **THEN** 预置的 Qoder 配置图标为 `"qoder"`、argv 为
  `["bash","-ic","exec -- \"$@\"","webmux","qoder","--dangerously-skip-permissions"]`、`hybrid` 为 `true`、
  `hybridSupport` 为 `{supported: true, agent: "qoder"}`；QoderCN 配置图标为 `"qoder"`、
  argv 以 `qodercn` 代替 `qoder`、`hybridSupport.agent` 为 `"qodercn"`

#### Scenario: 已有安装不改写

- **WHEN** 升级前已存在 `launch-configs.json`（含旧的 `"icon": "geminicli"` 配置）
- **THEN** 服务正常加载且不修改其中任何配置，包括名称、图标、`hybrid` 与命令参数

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
