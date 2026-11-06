## Context

动机见 proposal.md，服务端契约见 `unify-session-surface`（`design.md` 与 `specs/session-surface/spec.md`）。与 CLI 设计相关的现状：

- 命令由 `internal/sessioncmd` 的命令表定义，帮助、补全、`--help` 与 `internal/agentguide/llms/commands.md` 都由命令表生成，一致性测试要求每个选项有说明与默认值。
- `webmux send` 在客户端读取会话、检查所在端，再拨 `/ws/s/{id}` 发送带回执的提交；`chat send` 走 `chat/prompts`，`--wait` 先连接 `/ws/s/{id}/chat` 再发送，按 `promptId` 或 `queueItemId` 跟踪回合。
- `run --new-worktree BRANCH` 先在客户端调用 `POST /api/repositories/{id}/worktrees`，再创建会话，最后按 `--no-pin` 决定是否加入主页显示；
  `chat new` 调用 `POST /api/quick-starts`，分支名在客户端生成，失败按服务端返回的步骤报告。
- `run` 已有 `--attach` 开关，表示附着终端。

## Goals / Non-Goals

**Goals:**

- 每个意图只有一个命令；命令只按会话 ID 操作，不要求调用方先判断会话种类或所在视图。
- 视图不匹配时的错误统一：服务端 409 `wrong_view`、`no_chat_view` 映射为退出码 1 并给出下一步命令。
- `chat` 组只剩聊天视图真正独有的操作。

**Non-Goals:**

- 不改变 `wait`、`logs`、`screen`、`events`、`attach`、`kill`、`rename`、`label`、`notify` 的语义。
- 不改变退出码约定（0 成功、1 失败、2 用法错误、124 超时、130/143 信号）。
- 不提供旧命令的别名或迁移提示命令。

## Decisions

### C1 命令对照

| 现在 | 之后 |
|---|---|
| `run --new-worktree BRANCH`、`--start-side`、`chat new` | `run --new-worktree [--repo R] [--branch B] [--base REF]`、`--view`、`--set`、`--permission`、`--attach-file`、`--prompt`、`--reply` |
| `send` / `chat send` | `send [--enter] [--key K]... [--attach-file P]... [--mode steer] [--wait STATE \| --reply] [--auto-wait-requests] [--timeout D] [--json]` |
| `send --key esc` / `chat cancel` | `interrupt SESSION` |
| `chat switch --to` | `view SESSION terminal\|chat [--force] [--agent-session ID]` |
| `chat ls` / `ls` | `ls [--state STATE]... [--archived] [--repo REPO]` |
| `chat resume RECORD --side` | `resume SESSION [--view] [--agent-session ID] [--without-env] [--detach]` |
| `chat forget` / `rm` | `rm SESSION [--forget] [--yes]` |
| `chat status/watch/answer/set/permission/queue` | 保留在 `chat` 组 |
| `launch-config add\|edit --hybrid\|--no-hybrid` | `--chat\|--no-chat` |

备选：保留 `chat` 组作为“agent 会话”的全部入口，只把 `send` 做成分派。否决：创建、列表、恢复与清理仍然各有两套，脚本仍要先判断会话种类。

### C2 `--reply` 与 `--wait` 分开

`--wait STATE` 对两种视图都等待会话状态（状态在聊天视图上已由回合与请求推导）。`--reply` 只对聊天视图有意义：先连接聊天流再发送，
跟踪本次输入开启的回合并输出最终回复，退出码规则沿用 `chat send --wait`；在终端视图上不写入任何内容，以 1 结束并提示改用 `--wait STATE`。两者互斥。

备选：让 `--wait` 在聊天视图上自动输出回复。否决：同一个选项在两种视图上输出不同类型的结果，脚本无法依赖 `--json` 的形状。

### C3 `run` 一次请求完成创建

`run` 把位置、命令、视图、首条消息、配置项、权限模式与附件放进一次 `POST /api/sessions`，带新的 `clientRequestId`，同一次调用内的重试复用它。
`--attach-file` 先经 `POST /api/uploads` 上传。`--new-worktree` 时分支名在省略时由服务端生成；创建成功后，除非 `--no-pin`，CLI 再把新 worktree 加入主页显示。
失败时按服务端的 `step` 报告并列出已创建的 worktree 与会话。

`--init` 保持显式开启：`chat new` 原来在仓库有初始化指令时默认附带，`run` 不改变自己的默认值，迁移说明中写明需要加 `--init`。
`--view chat` 隐含 `--detach`；`--reply` 要求 `--view chat` 与 `--prompt`。附件选项命名为 `--attach-file`，因为 `--attach` 已表示附着终端。

### C4 视图不匹配的错误

- 会话没有聊天视图：`chat *`、`view ... chat`、`--reply` 以 1 结束，说明该会话没有聊天视图。
- 会话在终端视图而命令需要聊天视图：`chat *` 中需要聊天端的命令以 1 结束，提示 `webmux view SESSION chat`。
- `send` 的选项与当前视图不符：服务端 409 `wrong_view`，CLI 以 1 结束并报告原因，不写入任何内容。

### C5 生成物

补全的已归档会话来源改为 `GET /api/sessions?state=archived`；`worktree` 命令中对已归档会话的计数同样改读该接口。
`launch`、`chat`、`sessions`、`recipes` 主题与 `agentguide.go` 的主题摘要手工改写，并按 C6 去掉与命令帮助重复的部分。

### C6 手册只保留命令帮助给不了的内容

现状：`/llms/commands.md` 是全部 `--help` 的副本（67 KB，占 `llms-full.txt` 88 KB 的 76%），作为生成文件提交进仓库，靠 `TestGuideCommandReference` 与
`WEBMUX_UPDATE_GUIDE=1` 保持同步；手写主题又重复了选项、按键清单与单命令示例（例如 `send … --enter --wait waiting,idle` 同时出现在 `send` 的帮助、
`sessions`、`recipes` 与 `docs/usage/shell-control.md`）；`TestHelpReferencesResolve` 只检查命令名，不检查选项。

分工：命令表是单个命令用法的唯一来源（`--help`、`help --json`、补全、命令目录）；手册只写能力索引、概念、选择命令的依据、跨命令流程与本实例信息。

- `commands` 主题改为命令目录：按分组每个命令一行（用法与摘要，与 `webmux help` 概览同源），页首指向 `--help` 与 `help --json`。
  读手册的 agent 一定有 shell 可以运行 `--help`，且 `--help` 与二进制版本一致，完整副本不提供额外信息。预计 `llms-full.txt` 降到约 25 KB。
- 目录在运行时渲染，不再提交生成文件。`agentguide` 是 shared 层、`sessioncmd` 是 client 层，二者不能互相依赖，因此由 `cmd/webmux` 在启动时用
  `sessioncmd.Commands()` 渲染目录文本，构造 `agentguide.Guide`（持有目录文本，提供 `Index`、`Page`、`Full`）并注入 `api.Deps`；
  `webmux guide` 的离线副本由 `sessioncmd` 用自己的命令表渲染。删除 `TestGuideCommandReference`、`WEBMUX_UPDATE_GUIDE` 与 `Env.Reference`。
- 手写主题删除选项清单、按键与枚举清单、默认值和单命令示例，改为写命令名并指向 `--help`；`recipes` 保留跨命令流程。
  会话状态、`self`、临时文件夹只在手册中完整说明，命令帮助的 Description 缩为一句定义加 See also。
- `TestHelpReferencesResolve` 扩展为：页面中 `webmux <命令> … --<选项>` 的选项必须属于该命令或全局选项，覆盖 `docs/usage/shell-control.md`；
  除 `recipes` 外的主题不得包含与命令表示例相同的命令行；命令目录列出全部可见命令且不含帮助小节。

备选：保留完整命令参考，只加选项校验。否决：重复本身没有消除，`llms-full.txt` 的大部分仍是 `--help` 的副本。

备选：把 `--help` 翻译成中文，与手册合并。否决：会多出一份需要同步的说明；英文帮助、中文手册的分工在去重后不再产生重复内容。

## Risks / Trade-offs

- [脚本与 agent 提示中的旧命令失效] → 同一二进制内 CLI 与服务端同版本；`agentguide` 随版本更新；发布说明列出 C1 对照表。
- [`run --new-worktree` 由取值改为开关，旧写法 `--new-worktree fix/lint` 会把 `fix/lint` 当作位置参数] → `run` 在没有 `--` 时拒绝位置参数并以 2 结束，提示改用 `--branch`。
- [`rm --forget` 对运行中的会话也会删除记录] → 终端上要求确认，非交互时必须 `--yes`。

## Migration Plan

与 `unify-session-surface` 在同一 PR 中实现与发布；先完成契约与服务端，再切换 CLI，最后重新生成手册。回滚即回退整个版本。
