## MODIFIED Requirements

### Requirement: 启动配置的混合模式开关

启动配置 SHALL 支持可选的布尔字段 `hybrid`，缺省为 `false`。

- 保存时为 `false` 的字段 SHALL 不写入文件。
- `launch-configs.json` 的版本 SHALL 保持 2；不含该字段的已有文件 SHALL 照常读取，所有配置视为未勾选。

`hybrid` 为 `true` 时，服务端 SHALL 按 `hybrid-agent-profiles` 校验 argv，校验不通过时 SHALL 返回 400 并给出原因，已有数据不变。校验 SHALL NOT 检查第三方连接器是否已安装。

新安装预置的 Claude Code、Codex CLI、OpenCode 与 omp 配置 SHALL 为 `hybrid: true`，其 argv（含各自的自动批准参数）SHALL 通过 `hybrid-agent-profiles` 校验。修改已勾选配置的 argv 使其不再受支持时，保存 SHALL 返回 400，用户须先取消勾选。

新安装预置的 agent 配置 SHALL 保留各自的启动参数：

- Claude Code：`--dangerously-skip-permissions`；
- Codex CLI：`--dangerously-bypass-approvals-and-sandbox`；
- Gemini CLI：`--yolo`，`hybrid` 为 `false`；
- OpenCode：`--auto`；
- omp：`--auto-approve`。

Cursor CLI、GitHub Copilot CLI、Qwen Code、Kimi Code CLI 与 CodeBuddy Code 的预置和默认非混合行为不变。Shell 配置 SHALL 不变。已存在的 `launch-configs.json` SHALL NOT 被改写。

#### Scenario: 新安装的预置配置

- **WHEN** 服务首次启动且 `launch-configs.json` 不存在
- **THEN** 写入现有 11 条预置配置，Claude Code、Codex CLI、OpenCode 与 omp 的 `hybrid` 均为 `true`
- **AND** Claude Code 的 argv 为 `["bash","-ic","exec -- \"$@\"","webmux","claude","--dangerously-skip-permissions"]`，`hybridSupport.supported` 为 `true`，`agent` 为 `claudecode`，并带有 `claude-agent-acp` 连接器信息；Gemini CLI 与其余非混合配置保持 `hybrid=false`

#### Scenario: 已有安装不改写

- **WHEN** 升级前已存在 `launch-configs.json`
- **THEN** 服务不修改其中任何配置，包括 Claude Code 的 `hybrid` 与命令参数

#### Scenario: 未安装连接器时勾选 Claude Code 配置

- **WHEN** 服务上没有 `claude-agent-acp`，用户对 Claude Code 配置把 `hybrid` 改为 `true` 并保存
- **THEN** 保存成功；连接器检查只在启动 ACP 进程之前进行

#### Scenario: 不支持的命令

- **WHEN** 用户对不受支持的 shell 或任意自定义包装配置提交 `hybrid: true`
- **THEN** 服务端返回 400，原因说明未识别到支持的 Agent 或命令写法不受支持

### Requirement: 启动配置的混合支持信息

启动配置的完整配置、摘要、创建与更新请求 SHALL 包含 `hybrid`。完整配置与摘要 SHALL 额外包含只读的 `hybridSupport: {supported, agent?, reason?, connector?, supportedAgents}`，由服务端按当前 argv 计算；请求体中提供 `hybridSupport` SHALL 视为未知字段而返回 400。

`connector` 只在 `supported` 为 `true` 且档案声明了第三方连接器时出现，内容为 `{executable, title, installCommand, docsUrl}`，SHALL 只来自档案，SHALL NOT 反映连接器是否已安装。`supportedAgents` SHALL 包含 Claude Code 的显示名，并始终存在。

#### Scenario: Claude Code 配置的连接器信息

- **WHEN** 客户端调用 `POST /api/launch-configs/hybrid-support {cmd: ["claude"]}`
- **THEN** 返回 `{supported: true, agent: "claudecode", connector: {executable: "claude-agent-acp", installCommand: "npm install -g @agentclientprotocol/claude-agent-acp", …}}`，无论服务上是否安装了连接器

#### Scenario: 请求体携带只读字段

- **WHEN** 客户端创建或更新启动配置时在请求体中提供 `hybridSupport`
- **THEN** 服务端返回 400，已有数据不变
