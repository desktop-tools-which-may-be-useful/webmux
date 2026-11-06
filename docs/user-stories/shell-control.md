# 脚本与 agent 控制

本组描述用 `webmux` 命令行在脚本中、或由运行在会话里的 CLI agent 控制 WebMux 的能力。命令经本机套接字访问服务，不需要 Token；使用说明见 [用脚本和 agent 控制 WebMux](../usage/shell-control.md)。

## S. 脚本与 agent 控制

- **US-S01 会话内找到所属实例**：作为在会话里运行的脚本或 agent，我希望不依赖 `PATH` 或默认配置就能连回启动我的 WebMux，以便控制的总是同一个实例。
  - 每个会话进程，包括混合会话聊天端的 ACP agent，都有 `WEBMUX_SESSION_ID`、`WEBMUX_SOCKET`（本机 API 套接字绝对路径）与 `WEBMUX_BIN`（服务端正在运行的可执行文件）；
  - 三个变量属于服务端自有变量：创建请求或启动配置中出现时以 400 拒绝，`webmux run` 继承调用方环境时跳过；服务不修改会话的 `PATH`；
  - CLI 按 `--socket`、`--config`、`WEBMUX_SOCKET`、默认配置的顺序定位服务，`--socket` 与 `--config` 互斥；指定的套接字连不上时以退出码 3 结束，不回退到其他实例；
  - 接受会话 ID 的命令都接受 `self`；会话外使用 `self` 以退出码 2 结束；
  - 旧版 CLI 调用服务端没有的新接口时提示“服务端版本过旧”，给出两边的版本并建议使用 `$WEBMUX_BIN`；
  - 本机状态（`status --json`）包含 `bin`。
- **US-S02 读取会话元数据**：作为脚本或 agent，我希望一次取得会话所在的仓库、分支、启动配置、混合状态、链接、父子会话与标签，以便判断自己的处境和要操作的对象。
  - `GET /api/sessions/{id}/details` 返回 `SessionDetails`，不含 Token、启动环境变量值或其他会话的输出；临时文件夹只在已存在时报告，接口不创建它；
  - `webmux get [SESSION]` 在会话内默认 `self`，会话外必须给出 ID；默认输出对齐的键值，`--json` 输出 `SessionDetails`；
  - `--field PATH` 按点分路径输出单个值（字符串不带引号，对象与数组为 JSON），值为空或不存在时以 1 结束；`--json` 与 `--field` 互斥。
- **US-S03 标记并找到自己创建的会话**：作为编排脚本，我希望知道哪些会话由我创建并能给会话打标签，以便事后筛选和清理。
  - 经本机 API 从会话内创建的会话（`POST /api/sessions` 与 `POST /api/quick-starts`）自动记录 `parentId`；服务端按套接字对端进程判断，请求体无法指定，经 TCP 创建的会话没有父会话；
  - 创建请求可带 `labels`，非法标签以 400 拒绝且不创建会话；`PATCH /api/sessions/{id}/labels` 以 `{set, remove}` 原子修改；
  - 标签键匹配 `[a-z0-9][a-z0-9._-]{0,62}`，值最多 256 个字符且不含控制字符，每个会话最多 16 个；父会话与标签只在内存中，经 `/ws/sessions` 同步，Web 端暂不显示；
  - `webmux label SESSION KEY=VALUE... KEY-...` 设置或删除标签；`webmux ls` 支持 `--label KEY=VALUE`、`--label KEY`、`--parent SESSION` 与 `--status STATE`，条件同时满足，`--json` 输出筛选后的原始对象。
- **US-S04 完整的命令帮助**：作为第一次使用某个命令的人或 agent，我希望每个命令都有完整、可机读的帮助，以便不必猜测选项。
  - 所有命令（包括运维命令）在一张命令表中声明；`--help` 包含用法、说明、参数、选项与默认值、环境变量、输出、退出码、至少两个示例和相关命令，英文，80 列换行；
  - `-h`、`--help`、`webmux help <命令>` 写到标准输出并以 0 退出，不连接服务；用法错误以 2 退出并提示 `--help`；只给分组命令（如 `webmux config`）时把子命令列表写到标准错误并以 2 退出；
  - `webmux help --json [命令]` 输出命令树（选项、默认值、互斥关系、退出码、示例）；
  - 选项可以写在位置参数之后，`--` 之后的参数原样保留；`run` 在第一个位置参数处停止解析；
  - 顶层帮助保留服务端选项一行，Windows 客户端据 `-log-file` 判断 WSL 内的版本；
  - 测试保证每个可见命令有摘要和至少两个可解析的示例，每个选项有说明，运维命令的选项与实现一致。

- **US-S05 按需阅读 WebMux 工具手册**：作为 CLI agent，我希望从一个固定地址按需加载与当前服务版本一致的 shell 工具说明，以便只读需要的部分就能正确使用 `webmux`。
  - 服务端按 llms.txt 约定提供 `/llms.txt`（H1、摘要、命令前缀说明与主题链接，含 `## Optional`）、`/llms/<主题>.md` 与 `/llms-full.txt`；内容只在内存中渲染，不写入临时文件夹或其他文件；
  - 主题包括连接与身份（可执行文件与套接字的定位顺序、排错）、会话、启动、临时文件夹（原“临时文件夹”预设的全部说明）、命令参考、退出码、操作守则与常用流程；命令参考由命令表生成，测试保证与 `--help` 一致；
  - TCP 上不需要 Token 但检查 Host，内容不含版本、主机、路径或会话信息，示例不写死安装路径；本机套接字上附加本实例的版本、可执行文件与套接字；
  - `webmux guide [TOPIC] [--full]` 经本机套接字读取，服务未运行时输出内嵌副本并在标准错误说明；`--path` 输出服务端可执行文件与套接字路径；
  - `webmux dir [SESSION]` 输出会话临时文件夹路径（按需创建），会话内默认 `self`。
- **US-S06 在指令中插入 WebMux 工具说明**：作为使用者，我希望在快捷指令或仓库初始化指令中写 `{{webmux}}`，以便 agent 拿到本会话的手册地址、命令路径和交付文件位置。
  - `POST /api/sessions/{id}/agent-guide` 确保临时文件夹存在并返回说明：会话 ID、`webmux` 可执行文件与套接字的绝对路径（环境变量丢失时的用法）、`/llms.txt` 地址与 `webmux guide`、本会话临时文件夹路径与“交付文件放这里而不是项目仓库”的规则；不含 Token；
  - 初始化指令中的 `{{webmux}}` 不再被拒绝：会话创建后在 `{{dir}}` 之后插入说明，说明中的文字不再被当作变量；说明不可用时预检返回 503 且不创建 worktree；
  - 快捷指令正文中的 `{{webmux}}` 在选择时经该接口展开，失败时提示“WebMux 工具说明不可用”且不插入；
  - 设置页初始化指令的“可插入”提供内置的“WebMux 说明”按钮，快速开始预览说明 `{{webmux}}` 会在会话创建后替换；快捷指令名称仍不能是 `webmux`。

- **US-S07 从 shell 启动、跟随或等待会话**：作为脚本或 agent，我希望 `run` 具备 Web 端新建会话对话框的全部位置选择，并能不经终端跟随输出或等待状态，以便把启动、等待和取结果写进脚本。
  - 位置：
    - 当前目录所在的已登记 worktree 自动关联；
    - `--worktree REPO:WORKTREE|PATH` 选已有 worktree，`--cwd` 须在其中；
    - `--new-worktree BRANCH`（`--base`、`--existing-branch`、`--worktree-path`、`--repo`、`--refresh-base`）先建 worktree 再启动，默认像 Web 端一样加入主页显示（`--no-pin` 不加入）；
    - 会话创建失败时保留 worktree 并给出路径；
  - 混合会话：`--start-side terminal|chat`、`--prompt TEXT|-`、`--init`（需要已登记 worktree）；
  - 输出与等待：`--json` 输出 `{session, link}`，`--wait STATE --timeout DUR` 等会话第一次进入该状态（超时 124），`--label` 设置标签，`--dry-run` 只打印请求且隐藏环境变量值；
  - 没有终端时自动分离，`--attach` 强制附着；
  - `--follow` 以观察者身份把输出写到标准输出（`--strip` 去除控制序列），以程序的退出码退出（信号为 128+N）；SIGINT/SIGTERM 只停止跟随（130/143），`--kill-on-interrupt` 先结束程序；连接中断以 1 退出并提示。
- **US-S08 一条命令完成快速开始**：作为编排者，我希望从 shell 把任务交给新 worktree 中的 agent 并取回结果，以便并行分派工作。
  - `webmux chat new TEXT|-`（消息必填）调用快速开始接口：仓库默认是当前目录所属的已登记仓库，`--agent` 选混合启动配置（只有一条时可省略），分支名默认按 `wt/<日期>-<时间>` 生成并在冲突时重新生成，起始提交默认取仓库默认基点；
  - `--set ID=VALUE`、`--permission ask|auto`、`--init/--no-init`（仓库有初始化指令时默认使用）、`--attach PATH`（先上传）、`--terminal`（与 `--set`、`--permission`、`--attach`、`--wait` 互斥）、`--label`；
  - 失败按步骤报告并列出已创建的 worktree 与会话，不回滚；配置警告写到标准错误；
  - `--wait` 跟踪第一轮，输出 agent 的最终回复（`--json` 为回合事件），`end_turn` 以 0 退出，其他结束原因或出现授权、表单请求以 1 退出，`--auto-wait-requests` 继续等待，超时 124；
  - `webmux launch-config options NAME` 列出 agent 提供的配置项与可选值，供 `--set` 使用。

- **US-S09 向会话输入并等待结果**：作为编排脚本，我希望向终端会话输入并可靠地等到它有反应，以便把交互程序串进流程。
  - `webmux send SESSION [TEXT|-] [--enter] [--key NAME]...` 以带回执的提交写入文字，收到 `written` 才以 0 退出，`partial`、`failed`、`missing` 以 1 退出；命名按键在文字之后发送，未知按键以 2 结束；
  - 发送像浏览器输入一样取得控制权，控制指示器显示 `script`，在会话内附上来源会话 ID；混合会话在聊天端时以 1 结束并提示 `chat send`；
  - `send --wait STATE` 先订阅会话列表再输入，只接受输入之后的变化；STATE 可以是逗号分隔的多个状态；离开订阅时的状态后再回到它（如 waiting、running、waiting）也算变化；
  - `webmux wait SESSION... --until STATES [--changed] [--stable DUR] [--any] [--timeout DUR] [--json]`：`attention` 表示有锁存的提醒（agent 完成一回合后也会锁存），超时 124，会话被移除或在未等待 `exited` 时结束以 1 退出；
  - 状态语义：`waiting` 表示需要用户介入（agent 的授权、提问、未读的完成或出错，或没有 agent 信号时的提示符启发式，所以停在提示符的 shell 是 `waiting`），`idle` 只表示长期无活动，不是“命令或回合已结束”的信号；等 shell 命令结束用 `--wait waiting,idle`，等 agent 一回合结束用 `--until attention`。
- **US-S10 读取会话的输出与画面**：作为脚本或 agent，我希望读取会话输出，并看到全屏 TUI 当前显示的内容，以便判断下一步。
  - `webmux logs SESSION` 输出回放缓冲区原始字节，`--strip` 去除控制序列（CRLF、单独 CR、退格按行处理），`--tail N` 取最后 N 行（与 `--follow` 互斥），`--follow` 以观察者身份持续输出且不抢控制权；
  - `webmux screen SESSION` 以观察者身份取回放，在 CLI 本地按会话网格渲染，输出可见画面的纯文本，`--json` 含尺寸、光标与是否备用屏幕；回放被截断时在标准错误提示；聊天端以 1 结束；服务端不做终端模拟。
- **US-S11 通用 API、会话生命周期与事件**：作为脚本，我希望结束、移除、改名会话，订阅会话事件，并调用没有专用命令的接口。
  - `webmux kill`（结束进程，会话保留为已结束）、`webmux rm`（移除，运行中的先结束）、`webmux rename SESSION [NAME]`（空名称交还给终端标题）；
  - `webmux events [--session ID]...` 把 `/ws/sessions` 事件逐行输出为 JSON，可按会话筛选，断线后重连并输出新的快照；
  - `webmux api [-X METHOD] PATH [-d DATA|@FILE|-]` 只接受 `/api/` 与 `/local/v1/` 路径并原样输出响应，`-d` 须为 JSON，非 2xx 以 1 退出，WebSocket 路径以 2 拒绝；会话内的现有限制照常生效。
- **US-S12 shell 补全**：作为命令行使用者，我希望按 Tab 补全命令、选项与会话 ID，以便少记少打。
  - `webmux completion bash|zsh|fish` 输出补全脚本；隐藏的 `__complete` 给出命令、子命令、选项、枚举值，并向服务查询会话 ID（附标题）、启动配置、仓库、worktree 与手册主题；
  - 查询超过 1 秒或服务未运行时只给静态候选，不输出错误，也不输出 Token 或环境变量值；Windows 客户端不提供补全。
- **US-S13 从脚本提醒用户**：作为在会话里跑长任务的脚本或 agent，我希望任务结束或需要决定时直接提醒用户，以便用户不必盯着终端。
  - `webmux notify [--kind notify|done|error|question] [MESSAGE]` 默认作用于当前会话，会话外需 `--session`；消息最多 200 个字符、不含控制字符，超出时 CLI 以 2 拒绝；
  - 提醒来源为 `cli`，覆盖当前锁存的提醒而不受来源分层限制；会话徽章、页面内弹窗与后台推送照常出现，消息作为弹窗说明行与推送正文，只保存在内存中；
  - 之后按原有规则清除：用户输入、程序的下一次状态报告（混合会话聊天端为 agent 的新状态或新一轮）；`notify --clear` 与 `send` 的输入同样清除，并同步到所有设备；已退出的会话以 1 拒绝。
- **US-S14 从脚本驱动混合会话聊天**：作为编排脚本，我希望向另一个 agent 发送提示、取回回复并代为处理它的请求，以便不打开浏览器也能完成多 agent 协作。
  - `chat send` 立即发送或进入服务端队列，可附带上传的文件或以 `--mode steer` 插话；`--wait` 先订阅再发送，按 `promptId`（排队时按 `queueItemId`）跟踪这条提示自己的回合；
  - `end_turn` 以 0 退出并输出最终回复（`--json` 为该回合事件），其他结束原因以 1 退出；遇到授权或提问时输出请求 ID 与选项并以 1 退出，
    `--auto-wait-requests` 继续等待；排队的提示被删除时以 1 退出；超时 124 且不取消回合；
  - `chat status` 列出所在端、回合、队列、权限模式、待回答请求（ID、选项、表单字段）与 agent 设置，`chat watch` 持续输出消息、工具调用、请求与回合（`--json` 为 NDJSON 帧，默认跳过历史）；
  - `chat answer`（`--option`、`--content`、`--decline`、`--cancel`）、`chat cancel`、`chat set ID=VALUE`（校验设置与可选值，失败的项报告后以 1 结束）、
    `chat permission ask|auto`、`chat queue ls|rm|edit|send-now|clear|resume` 使用与 Web 端相同的接口并同步到所有设备；
  - 目标不是混合会话、或需要聊天端却在终端端时以 1 结束并给出 `chat switch` 提示；每次写操作带新的 `clientRequestId`。
- **US-S15 从脚本切换端与恢复历史会话**：作为脚本，我希望在终端与聊天之间切换混合会话，并找回、恢复或清理已结束的 agent 会话。
  - `chat switch --to chat|terminal` 等到切换结束；终端 TUI 忙时 `--force` 强制中断，否则取消切换并以 1 退出；服务端列出 agent 会话候选时用 `--agent-session` 重试；
  - `chat ls` 按状态（`--state`、`--archived`）与仓库筛选混合会话记录，`chat resume` 按记录原来的端或 `--side` 恢复，终端端且有终端时附着、否则输出 ID 与链接，
    启动配置已删除时需要 `--without-env`；
  - `chat forget` 删除已结束或已中断的记录（agent 自己的历史保留），终端上先确认，非交互时需要 `--yes`，进行中的记录以 1 拒绝。
- **US-S16 从脚本管理 worktree 与主页显示**：作为编排脚本，我希望为并行任务创建、查看和删除 worktree，并决定它们是否出现在主页，以便 Web 端与脚本看到同一个工作区。
  - `worktree ls|show|create|rm`：`REPO` 为 ID、路径或 `.`，worktree 为 ID、分支或路径，找不到时以 2 结束并列出候选；`create` 默认放在 `<仓库>.worktrees/<分支末段>`，
    `--refresh-base` 先取回上游；`rm` 先在标准错误输出预览，有阻止原因时需要 `--force`，终端上要求输入路径确认，非交互时需要 `--yes`；
  - 主页状态（`pinned`、`sessions`、`hidden`）与 Web 端规则一致：从未设置的仓库只显示主工作区，有会话或归档记录的 worktree 总会显示；
  - `worktree create` 与 `run --new-worktree` 默认加入主页显示（`--no-pin`）；`worktree pin|unpin|home [--set|--reset]` 只改该仓库的 `homeWorktrees` 条目，
    409 时重新读取并重放（最多 3 次），清理已不存在的 worktree，并同步到所有浏览器。
- **US-S17 从脚本登记仓库与修改仓库设置**：作为在新克隆的仓库里工作的脚本，我希望登记仓库、查看分支与提交，并设置快速开始用的初始化指令。
  - `repo ls|add|rm|show|refresh|branches|commits|base`：`add` 幂等，`rm` 只取消登记并清理主页显示设置，`commits --from` 接受分支、基点或完整提交 ID，`base` 的取回失败只警告；
  - `repo settings` 对应仓库设置对话框：输出初始化指令、长度与引用；`--init TEXT|-|@FILE`、`--clear-init`、`--edit`；超过 4000 个字符在本地以 2 拒绝，
    引用不存在的快捷指令时给出与设置页相同含义的警告并仍然保存。
- **US-S18 从脚本管理启动配置与端口转发**：作为配置开发环境的脚本，我希望增删改启动配置并为开发服务器开转发。
  - `launch-config ls|show|add|edit|rm|presets|restore`：`show` 默认把变量值显示为 `<hidden>`，`--reveal-env` 才输出；`add` 的命令原样保存不经 shell，
    `--hybrid` 先做支持检查；`edit` 只改给出的字段，未提及的变量值保持不变；名称不存在时以 2 结束并列出已保存的名称；变量值不出现在输出或诊断中；
  - `forward ls|add|rm`：`add` 输出转发地址，`--session self` 关联当前会话，`--no-compat` 关闭兼容改写；转发关闭、端口非法或重复时给出原因并以 1 结束。

### 依据

- [会话命令与命令表](../../internal/sessioncmd/command.go)、[帮助渲染](../../internal/sessioncmd/help.go)、[元数据命令](../../internal/sessioncmd/metadata.go)、[运维命令表](../../cmd/webmux/commands.go)
- [会话详情与标签接口](../../internal/api/sessionmeta.go)、[父会话与标签](../../internal/session/relations.go)、[进程所属会话](../../internal/session/peer.go)
- [本机套接字定位](../../cmd/webmux/sessions.go)、[会话环境](../../internal/session/env.go)、[保留变量](../../internal/contract/env.go)
- [手册与简述](../../internal/agentguide/agentguide.go)、[手册主题](../../internal/agentguide/llms/connect.md)、[手册路由与说明接口](../../internal/api/guide.go)、[guide 与 dir 命令](../../internal/sessioncmd/guide.go)
- [会话控制命令](../../internal/sessioncmd/control.go)、[补全](../../internal/sessioncmd/completion.go)
- [notify 命令](../../internal/sessioncmd/notify.go)、[脚本提醒](../../internal/session/notify.go)、[推送正文](../../internal/push/alert.go)
- [worktree 与主页显示命令](../../internal/sessioncmd/worktree.go)、[仓库命令](../../internal/sessioncmd/repo.go)、[启动配置命令](../../internal/sessioncmd/launchconfig.go)、[转发命令](../../internal/sessioncmd/forward.go)
- [聊天命令](../../internal/sessioncmd/chat.go)、[聊天历史、设置与队列命令](../../internal/sessioncmd/chat_more.go)、[聊天接口](../../internal/api/chat.go)
- [启动命令](../../internal/sessioncmd/launch.go)、[会话列表订阅、跟随与回合跟踪](../../internal/sessioncmd/streams.go)、[仓库与主页显示](../../internal/sessioncmd/workspace.go)
- [初始化指令展开](../../internal/workspace/quickstart.go)、[快捷指令运行](../../web/src/components/SessionTerminal.svelte)、[预设与内置变量](../../web/src/quickCommands.ts)
