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

### 依据

- [会话命令与命令表](../../internal/sessioncmd/command.go)、[帮助渲染](../../internal/sessioncmd/help.go)、[元数据命令](../../internal/sessioncmd/metadata.go)、[运维命令表](../../cmd/webmux/commands.go)
- [会话详情与标签接口](../../internal/api/sessionmeta.go)、[父会话与标签](../../internal/session/relations.go)、[进程所属会话](../../internal/session/peer.go)
- [本机套接字定位](../../cmd/webmux/sessions.go)、[会话环境](../../internal/session/env.go)、[保留变量](../../internal/contract/env.go)
- [手册与简述](../../internal/agentguide/agentguide.go)、[手册主题](../../internal/agentguide/llms/connect.md)、[手册路由与说明接口](../../internal/api/guide.go)、[guide 与 dir 命令](../../internal/sessioncmd/guide.go)
- [启动命令](../../internal/sessioncmd/launch.go)、[会话列表订阅、跟随与回合跟踪](../../internal/sessioncmd/streams.go)、[仓库与主页显示](../../internal/sessioncmd/workspace.go)
- [初始化指令展开](../../internal/workspace/quickstart.go)、[快捷指令运行](../../web/src/components/SessionTerminal.svelte)、[预设与内置变量](../../web/src/quickCommands.ts)
