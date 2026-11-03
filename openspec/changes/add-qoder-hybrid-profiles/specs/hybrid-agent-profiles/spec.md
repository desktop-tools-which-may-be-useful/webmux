# hybrid-agent-profiles Delta Spec

## Purpose

新增 Qoder 与 QoderCN 两个内置档案，
并扩展档案数据模型以支持 `--acp` 标志式 ACP 启动
（区别于现有 `acp` 子命令式）。

## New Requirements

### Requirement: Qoder 档案

系统 SHALL 内置 Qoder 档案（`Name: "qoder"`，`Title: "Qoder"`），
识别 `qoder` 可执行文件。

- ACP 启动：`qoder --acp`（标志式，非子命令），
  由 `Profile.ACPArg = "--acp"` 声明。
- 恢复：`qoder --resume <uuid>`，`ResumeFlag = "--resume"`。
- 中断：一次 Esc（`\x1b`）。
- 隐藏命令：`login`、`logout`。
- 参数分类表与 Claude Code 对齐：
  - Keep：`--model`、`--reasoning-effort`、`--thinking`、
    `--permission-mode`、`--system-prompt`、`--append-system-prompt`、
    `--agents`、`--agent`、`--plugin-dir`、`--settings`、
    `--setting-sources`、`--mcp-config`、`--allowed-tools`、
    `--disallowed-tools`、`--tools`、`--add-dir`、`--strict-mcp-config`、
    `--bare`、`--fallback-model`、`--allowed-mcp-server-names`、
    `--max-output-tokens`、`--context-window`、`--thinking-budget`、
    `--output-style`、`--max-model-request-retries`。
  - Drop：`-c`/`--continue`、`-r`/`--resume`（无参数形式）、
    `--attachment`、`-i`/`--prompt-interactive`、`-n`/`--name`。
  - Session：`--fork-session`、`--session-id`、`--from-pr`、
    `--remote-session`、`--remote-control`、`--remote`。
  - Directory：`-w`/`--worktree`、`--cwd`。
  - NonInteractive：`-p`/`--print`、`-h`/`--help`、`-v`/`--version`、
    `-o`/`--output-format`、`--input-format`、`--list-models`、
    `--list-sessions`、`--no-session-persistence`、`--delete-session`。
  - Store：`--config-dir`。
- 子命令（NonInteractive）：`mcp`、`hooks`/`hook`、`rollback`、
  `update`、`config`、`plugins`/`plugin`、`commit`、`agents`/`agent`、
  `skills`/`skill`、`feedback`、`security`、`login`、`status`、
  `remote-control`、`sites`、`wiki`。
- ChatSide 推导：`--model` → `model`、`--reasoning-effort` → `effort`、
  `--permission-mode`/`--dangerously-skip-permissions` → `mode`。
- 可选能力：`ResumesSessions() = true`、
  `ReapplyOnLoad() = ["model", "effort"]`、
  `SessionSteering{IdlePrompt: true}`。

#### Scenario: Qoder 预置配置

- **WHEN** 配置的 argv 为
  `["bash","-ic","exec -- \"$@\"","webmux","qoder","--dangerously-skip-permissions"]`
- **THEN** 判定为可用，档案为 qoder

#### Scenario: Qoder ACP 启动

- **WHEN** 混合会话从聊天开始，配置档案为 qoder
- **THEN** ACP 进程 argv 为 `["qoder","--acp"]`
  （或 bashrc 包装形式）

### Requirement: QoderCN 档案

系统 SHALL 内置 QoderCN 档案（`Name: "qodercn"`，`Title: "QoderCN"`），
识别 `qodercn`、`qoderclicn` 与 `qoder-cn` 可执行文件。

- 除 ACP agent 名（`qoder-cli-cn`）与配置目录（`~/.qoder-cn/`）外，
  其余行为与 Qoder 档案相同。
- `Identify()` 匹配 `s.ProgramApp == "qoder-cli-cn"` 或 `"qodercn"`。

#### Scenario: QoderCN 预置配置

- **WHEN** 配置的 argv 为
  `["bash","-ic","exec -- \"$@\"","webmux","qodercn","--dangerously-skip-permissions"]`
- **THEN** 判定为可用，档案为 qodercn

### Requirement: ACP 启动参数声明

`Profile` SHALL 新增 `ACPArg string` 字段。`ACPArgv()` 生成
内置 ACP 命令时：

- `ACPArg` 非空：使用 `ACPArg` 作为 ACP 启动参数（如 `"--acp"`）；
- `ACPArg` 为空：使用默认值 `"acp"`（子命令式，兼容现有档案）。

#### Scenario: 子命令式 ACP（现有行为不变）

- **WHEN** 档案的 `ACPArg` 为空（如 OpenCode、omp）
- **THEN** ACP argv 为 `["opencode","acp"]` 或 `["omp","acp"]`

#### Scenario: 标志式 ACP（Qoder/QoderCN）

- **WHEN** 档案的 `ACPArg` 为 `"--acp"`
- **THEN** ACP argv 为 `["qoder","--acp"]` 或 `["qodercn","--acp"]`

## Modified Requirements

### Requirement: 内置档案与可识别的命令写法

系统 SHALL 内置 OpenCode、omp、Codex、Claude Code、Qoder
与 QoderCN 六个档案。

未匹配任何档案时，原因文字 SHALL 为
"未识别到支持的 Agent，当前版本只支持 OpenCode、omp、Codex CLI、
Claude Code、Qoder 与 QoderCN"。
