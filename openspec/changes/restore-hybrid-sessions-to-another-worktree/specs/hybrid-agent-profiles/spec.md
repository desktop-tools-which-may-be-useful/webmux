## ADDED Requirements

### Requirement: 档案声明搬迁恢复方式

每个档案 SHALL 声明在哪些端支持把 agent 会话恢复到与其记录目录不同的目录（搬迁恢复），以及每端的做法。系统 SHALL 只按档案声明执行搬迁恢复，SHALL NOT 为未声明的档案或端尝试。本变更的声明为：

- Codex：终端端与聊天端；
- Claude Code：终端端与聊天端；
- omp：仅终端端；
- OpenCode、Antigravity CLI、Qoder、QoderCN：不支持。

#### Scenario: 支持的端出现在记录摘要

- **WHEN** 客户端读取 Codex、omp、OpenCode 各一条可恢复记录
- **THEN** 三条摘要的 `relocatableSides` 分别为 `["tui","acp"]`、`["tui"]`、`[]`

### Requirement: Codex 的搬迁恢复

Codex 的搬迁恢复 SHALL 按端执行：

- 聊天端：以目标目录为 cwd 启动 ACP 进程，并以目标目录调用 `session/load`（或档案选用的 `session/resume`）；
- 终端端：恢复命令在全局参数中加入 `-c` `tui.resume_cwd="current"`，使 Codex 不再询问使用哪个目录，并在目标目录中继续。

该参数 SHALL 只用于搬迁恢复，SHALL NOT 出现在普通恢复或切换命令中。

#### Scenario: Codex 终端端搬迁恢复命令

- **WHEN** 原 argv 为 `["codex","-m","gpt-5.6-luna"]`，agent 会话 ID 为 `01a1`，搬迁恢复到终端端
- **THEN** 恢复命令为 `["codex","--no-daemon","-m","gpt-5.6-luna","-c","tui.resume_cwd=\"current\"","resume","01a1"]`，PTY 的 cwd 为目标目录

#### Scenario: Codex 聊天端搬迁恢复

- **WHEN** Codex 记录的原工作树已删除，用户搬迁恢复到聊天端
- **THEN** ACP 进程的 cwd 为目标目录，`session/load` 返回原对话，之后目标目录的 `session/list` 包含该会话

### Requirement: Claude Code 的搬迁恢复

Claude Code 搬迁恢复（任一端）启动 agent 进程之前，系统 SHALL 在 Claude 配置目录的 `projects/` 下移动 transcript，再按普通恢复启动：

- 配置目录为启动配置环境中的 `CLAUDE_CONFIG_DIR`，未设置时为服务用户的 `~/.claude`；
- 移动对象为原 cwd 对应项目目录中的 `<agent会话ID>.jsonl` 及同名子目录，目标为目标目录对应的项目目录；
- 项目目录名 SHALL 为把绝对路径中每个非字母数字字符替换为 `-` 的结果。

#### Scenario: Claude 聊天端搬迁恢复

- **WHEN** Claude 记录的原工作树 `/w/feat` 已删除，用户搬迁恢复到聊天端，目标为 `/w/main`
- **THEN** `projects/-w-feat/<id>.jsonl` 被移到 `projects/-w-main/<id>.jsonl`，ACP `session/load` 以 `/w/main` 成功并显示原对话

### Requirement: Claude Code 搬迁的拒绝条件

以下情况 SHALL 返回 409 `relocation_unsupported` 并说明原因，SHALL NOT 移动任何文件：

- 任一项目目录名超过 200 个字符；
- 启动配置环境设置了 `CLAUDE_CODE_PROJECT_DIR_NAME`；
- 目标项目目录已有同名 transcript；
- 原项目目录中找不到该 transcript。

移动 SHALL 使用同一文件系统内的重命名，SHALL NOT 读取、改写或记录 transcript 内容。

#### Scenario: 目录名过长

- **WHEN** 目标目录的绝对路径替换后超过 200 个字符
- **THEN** 服务端返回 409 `relocation_unsupported` 并说明路径过长，Claude 配置目录中没有文件被移动

### Requirement: omp 的搬迁恢复

omp 的搬迁恢复只支持终端端：系统 SHALL 在目标目录的 PTY 中执行普通恢复命令（`--resume <id>`），由 omp 自己询问是否把会话移到当前目录，用户在终端中回答。系统 SHALL NOT 代替用户向终端输入回答。

omp 的聊天端在原目录与目标不同的情况下会拒绝加载会话，因此搬迁恢复到聊天端 SHALL 返回 409 `relocation_unsupported`。

#### Scenario: omp 终端端搬迁恢复

- **WHEN** omp 记录的原工作树已删除，用户搬迁恢复到终端端，目标为主工作区
- **THEN** 终端显示 omp 的搬迁询问，用户确认后 omp 显示原对话，之后主工作区的 `session/list` 包含该会话，可以切到聊天
