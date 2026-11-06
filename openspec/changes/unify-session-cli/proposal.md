## Why

shell 命令按“普通会话 / 混合会话”和“终端端 / 聊天端”分成两套动词：输入是 `send` 与 `chat send`，中断是 `send --key esc` 与 `chat cancel`，
创建是 `run` 与 `chat new`，列表是 `ls` 与 `chat ls`，结束与清理是 `rm` 与 `chat forget`，恢复与切换只在 `chat` 组里。
脚本和 agent 必须先查清会话种类和所在端，再选对命令，选错时以 1 退出；`agentguide` 也因此要分别教两套用法。

`unify-session-surface` 让服务端按视图分派输入与中断，并把列表、创建、恢复与删除记录统一到会话接口上。
本变更在同一 PR 中把 shell 命令收敛为作用于会话的动词，使 CLI 与契约使用同一套名词。

## What Changes

- **BREAKING** `run` 吸收 `chat new`：
  - `--new-worktree` 由取值改为开关，分支名由 `--branch` 给出，省略时按 `wt/<日期>-<时间>` 自动生成；`--repo`、`--base`、`--existing-branch`、`--worktree-path`、
    `--refresh-base`、`--no-pin` 保留。
  - `--start-side` 改为 `--view terminal|chat`；新增 `--set ID=VALUE`、`--permission ask|auto`、`--attach-file PATH`、`--reply`。
  - 创建只发送一次 `POST /api/sessions`，失败按步骤报告并保留已创建的资源。
  - 删除 `chat new`。
- **BREAKING** `send` 调用 `POST /api/sessions/{id}/input`，由服务端按视图分派；新增 `--attach-file`、`--mode steer`、`--reply`
  （跟踪本次输入开启的回合并输出最终回复，取代 `chat send --wait`）；删除 `chat send`。
- **BREAKING** 新增 `interrupt`、`view`、`resume`，取代 `chat cancel`、`chat switch`、`chat resume`。
- **BREAKING** `ls` 增加 `--state`、`--archived`、`--repo`，取代 `chat ls`；`rm` 增加 `--forget` 与 `--yes`，取代 `chat forget`。
- **BREAKING** `chat` 命令组只保留聊天视图专有的操作：`status`、`watch`、`answer`、`set`、`permission`、`queue ...`。
- **BREAKING** `launch-config add|edit` 的 `--hybrid`/`--no-hybrid` 改为 `--chat`/`--no-chat`；`launch-config options` 改用 `POST /api/launch-configs/{id}/options`。
- **BREAKING** `ls` 的表格与 `--json` 按新的会话信息显示视图，`--json` 不再含 `hybrid`。
- **BREAKING** 一次性切换：删除的子命令与选项按未知命令或未知选项以 2 结束，不保留别名。
- `agentguide` 的 `launch`、`chat`、`sessions`、`recipes` 主题按新命令改写。
- **BREAKING** llms 手册去掉与 `--help` 重复的部分：`/llms/commands.md` 从全部命令帮助的副本改为一行一个命令的命令目录，运行时由命令表渲染，
  仓库中不再保存生成的 `commands.md`；手写主题删除选项清单、按键清单与单命令示例，改为指向 `--help`；命令帮助不再复述手册中的概念说明。
  一致性测试增加对手册中选项与重复示例的检查。

## Capabilities

### New Capabilities

（无）

### Modified Capabilities

- `shell-session-launch`：`run` 的位置、视图、首条消息、配置项、附件与 `--reply`；删除 `chat new`；`launch-config options` 改用新接口。
- `shell-session-control`：`send` 按视图分派；`rm --forget`；新增 `interrupt`、`view`、`resume`。
- `shell-chat-control`：只保留聊天视图专有的命令，删除发送、取消、切换与历史命令。
- `terminal-session-cli`：`ls` 的筛选、视图显示与 JSON 字段；附着时的移动提示取值。
- `shell-cli-help`：示例改用新命令；llms 的命令目录在运行时由命令表渲染，不再保存生成副本；帮助不复述手册中的概念说明。
- `shell-workspace-control`：示例改用新命令，`launch-config add|edit` 的 `--hybrid` 改为 `--chat`。
- `agent-shell-guide`：主题划分、示例与配方改用新命令；新增“手册与命令帮助的分工”：命令目录取代完整命令参考，手写主题不重复命令帮助，并有测试保证。

## Impact

- CLI：`internal/sessioncmd`（`launch.go`、`control.go`、`chat.go`、`chat_more.go`、`launchconfig.go`、`completion.go`、`worktree.go`、`streams.go`、`help.go`）、
  `internal/attach` 的移动提示、`cmd/webmux` 端到端测试。
- 手册：`internal/agentguide`（改为由 `cmd/` 注入命令目录的 `Guide` 值，删除 `llms/commands.md`）、`internal/api/guide.go`、`internal/sessioncmd/help.go` 与 `guide.go`、
  `cmd/webmux` 的装配与 `commands_test.go`，以及 `internal/agentguide/llms/*` 的手写主题。
- 文档：`docs/usage/shell-control.md`，以及 `docs/user-stories/shell-control.md`（US-S04、US-S05、US-S07、US-S08、US-S09、US-S11、US-S12、US-S14、US-S15、US-S16、US-S17、US-S18）
  与 `docs/user-stories/hybrid-sessions.md`（US-Q 中引用 `run --start-side`、`chat switch|send|status|answer` 的条目）。
- 依赖：`unify-session-surface` 的接口与字段；两个变更必须一起发布。
- 兼容：写在脚本或 agent 提示中的 `webmux chat new|send|cancel|switch|ls|resume|forget`、`run --start-side`、`run --new-worktree BRANCH`、
  `launch-config --hybrid` 会失效。
