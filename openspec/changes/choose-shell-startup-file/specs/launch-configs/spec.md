## MODIFIED Requirements

### Requirement: 启动配置的混合模式开关

启动配置 SHALL 支持可选的布尔字段 `hybrid`，缺省为 `false`。

- 保存时为 `false` 的字段 SHALL 不写入文件。
- `launch-configs.json` 的版本 SHALL 保持 2；不含该字段的已有文件 SHALL 照常读取，所有配置视为未勾选。

`hybrid` 为 `true` 时，服务端 SHALL 按 `hybrid-agent-profiles` 校验 argv，校验不通过时 SHALL 返回 400 并给出原因，已有数据不变。

新安装预置的 Codex CLI、OpenCode 与 omp 配置 SHALL 为 `hybrid: true`，其 argv（含各自的自动批准参数）SHALL 通过 `hybrid-agent-profiles` 校验。修改已勾选配置的 argv 使其不再受支持时，保存 SHALL 返回 400，用户须先取消勾选。

新安装预置的下列 agent 配置 SHALL 带上各自的自动批准参数，使工具调用无需逐次批准：

- Claude Code：`--dangerously-skip-permissions`
- Codex CLI：`--dangerously-bypass-approvals-and-sandbox`
- Gemini CLI：`--yolo`
- OpenCode：`--auto`
- omp：`--auto-approve`

新安装 SHALL 同时预置 Cursor CLI（`agent`）、GitHub Copilot CLI（`copilot`）、Qwen Code（`qwen`）、Kimi Code CLI（`kimi`）和 CodeBuddy Code（`codebuddy`）。这五款配置 SHALL 保留 CLI 默认授权行为，不增加自动批准参数，且 `hybrid` 为 `false`。

Shell 配置 SHALL 不变。新安装的 agent 预置命令 SHALL 按 `defaultShell` 选择启动文件包装：`bash` 使用 `bash -ic`、`zsh` 使用 `zsh -ic`、空字符串直接执行。已存在的 `launch-configs.json` SHALL NOT 被改写或迁移；升级后旧配置保持原有的 Bash 包装或直接命令。

#### Scenario: Bash 默认的新安装预置配置

- **WHEN** 服务首次启动、`launch-configs.json` 不存在且 `defaultShell` 为 `bash`
- **THEN** 写入 Shell、Claude Code、Codex CLI、Gemini CLI、Cursor CLI、GitHub Copilot CLI、OpenCode、Qwen Code、Kimi Code CLI、CodeBuddy Code 和 omp 共 11 条配置
- **AND** omp 的 argv 为 `["bash","-ic","exec -- \"$@\"","webmux","omp","--auto-approve"]`，`hybrid` 为 `true`，`hybridSupport` 为 `{supported: true, agent: "omp"}`；Codex CLI 与 OpenCode 配置同样为混合配置
- **AND** Claude Code、Gemini CLI 带上述自动批准参数且 `hybrid` 为 `false`；其余五款 agent 不带自动批准参数且 `hybrid` 为 `false`

#### Scenario: Zsh 默认的新安装预置配置

- **WHEN** 服务首次启动、`launch-configs.json` 不存在且 `defaultShell` 为 `zsh`
- **THEN** agent 预置使用 `zsh -ic` 包装，omp 的 argv 为 `["zsh","-ic","exec -- \"$@\"","webmux","omp","--auto-approve"]`

#### Scenario: 其他 shell 的新安装预置配置

- **WHEN** 服务首次启动且 `defaultShell` 为空字符串
- **THEN** agent 预置为直接命令，不带 Bash 或 Zsh 包装

#### Scenario: 已有安装不改写

- **WHEN** 升级前已存在 `launch-configs.json`
- **THEN** 服务不修改其中任何配置，包括 `hybrid`、命令参数与原有包装 shell；新默认值只影响此后的新安装或用户手动编辑

#### Scenario: 勾选 omp 配置

- **WHEN** 用户对自己的 omp 配置把 `hybrid` 改为 `true` 并保存
- **THEN** 保存成功，摘要中 `hybrid` 为 `true`，`hybridSupport` 为 `{supported: true, agent: "omp"}`

#### Scenario: 旧文件

- **WHEN** 升级前保存的 `launch-configs.json` 不含 `hybrid` 字段
- **THEN** 服务正常读取，所有配置的 `hybrid` 为 `false`，文件不被改写

#### Scenario: 不支持的命令

- **WHEN** 用户对 Claude Code 配置提交 `hybrid: true`
- **THEN** 服务端返回 400，原因说明当前版本只支持 OpenCode 与 omp

## ADDED Requirements

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
