# 混合会话（聊天 ↔ 终端）

混合会话让 OpenCode、omp、Codex CLI、Claude Code、Antigravity CLI（`agy`）、Qoder 或 QoderCN 的同一段对话在两种界面之间切换：原生 TUI（终端视图）适合在电脑上精细操作，聊天视图适合在手机上阅读和回复。

切换前后是同一个 webmux 会话与同一个 agent 会话，历史由 agent 自己保存。接口见 [混合会话协议](protocol/http-hybrid.md)。

## 开启

新安装首次启动时预置的 OpenCode、omp、Codex CLI、Claude Code、Antigravity CLI、Qoder 与 QoderCN 配置已默认勾选 **“混合模式（聊天 ↔ 终端）”**，可通过以下两种方式直接开启：

1. **方式一：快速开始（聊天模式 `/new`）** — 切到聊天模式后在快速开始页选择仓库、基准分支与支持混合会话的 agent，输入首条任务（可附带文件或勾选仓库初始化指令）并发送，WebMux 会自动创建独立 worktree 并直接进入聊天视图。
2. **方式二：新建会话对话框** — 选择位置与开启了混合模式的启动配置，在两张单选卡片中选择 **“从终端开始”** 或 **“从聊天开始”**（默认终端，浏览器按每个 agent 分别记住上一次的选择），也可直接填写首条 **“用户指令”** 与勾选 **“使用初始化指令”**。

自定义或其他配置可在“设置 → 启动配置”中编辑并勾选 **“混合模式（聊天 ↔ 终端）”**；勾选框是否可用由服务端判定，不可用时旁边会显示原因。已有安装的配置不会被自动改写。

### 启动命令兼容要求

能开启混合模式的命令需满足以下条件：

- 直接执行 `opencode …` / `omp …` / `codex …` / `claude …` / `agy …` / `qodercli …`，或用“加载 shell 启动文件”保存的 Bash/Zsh 包装；其他 shell 包装与 `env` 前缀不受支持。
- 不能在命令里指定工作目录（OpenCode 的位置参数、omp 的 `--cwd`、Codex 的 `-C`/`--cd` 与 `--worktree`、Claude Code 的 `-w`/`--worktree` 与 `--tmux`），请在新建会话时选择目录。
- 不能使用让两端看到不同会话库的参数：omp 的 `--session-dir`、`--no-session`，OpenCode 的 `--server`，Codex 的 `--remote`，Claude Code 的 `--no-session-persistence`、`--environment`，Antigravity CLI 的 `--project`、`--new-project`。
- 不能使用不启动交互界面的参数（`-p`/`--print`、`--version`、非交互子命令等）或打开已有会话的参数（`codex resume`/`fork`、Claude Code 的 `--continue`/`--resume`/`--session-id` 等）。
- Claude Code 的 `CLAUDE_CONFIG_DIR` 可在启动配置环境中设置，两端会使用同一个配置目录。

已验证的版本为 OpenCode 2.1.4（2.2.0 参数相同）、omp 18.13.1、Codex CLI 0.165.0 与 codex-acp 2.3.0、Claude Code 2.1.316 与 claude-agent-acp 0.99.1、Antigravity CLI 1.4.12；更低版本只在聊天视图显示提示，不阻止切换，更高版本不自动视为已验证。

### Codex CLI 与 codex-acp

Codex CLI 没有自己的 ACP 模式，聊天端由第三方适配器 [codex-acp](https://github.com/agentclientprotocol/codex-acp) 提供，需要在服务器上另外安装（需要 Node）：

```sh
npm install -g @agentclientprotocol/codex-acp
```

- webmux 不会自动安装或临时下载它。每次启动聊天端之前（从聊天开始、切到聊天、恢复到聊天与重连）都按启动配置的写法检查它能否找到：直接执行时查启动配置环境的 `PATH`，“加载 shell 启动文件”时经同一 Bash/Zsh 包装执行 `command -v`。
- 没有安装时不启动任何聊天端进程，页面显示“未安装 Codex ACP 适配器（codex-acp）”、可复制的安装命令与说明链接；终端端照常使用。安装后 30 秒内自动重新检测，无需刷新。
- 聊天端通过 `CODEX_PATH` 使用终端端同一个 `codex`，并设置 `NO_BROWSER=1`；启动配置已设置同名变量时以配置为准。
- 首次切到聊天时，按启动参数设置模式、模型与推理强度，之后每次加载都重新应用：
  - `--dangerously-bypass-approvals-and-sandbox` → 完全访问，`--approve-for-me` → 自动审查，`-s read-only` / `-s workspace-write` → 同名模式；
  - `-m` → 模型，`-c model_reasoning_effort=…` → 推理强度；
  - `--search`、`--profile`、`--enable` 等参数在聊天端不生效，页面会列出。
- 聊天视图的命令菜单不提供 `/logout`（它会注销服务用户的全局 Codex 登录）。

### Claude Code 与 claude-agent-acp

Claude Code 没有原生 ACP 模式，聊天端由第三方连接器 [claude-agent-acp](https://github.com/agentclientprotocol/claude-agent-acp) 提供。服务器需要 Node.js 22 或更高版本，并另外安装连接器：

```sh
npm install -g @agentclientprotocol/claude-agent-acp
```

- webmux 不会自动安装 Node 或连接器，也不会用 `npx` 临时下载。每次启动聊天端之前都按启动配置的 PATH 与 Bash 包装检查 `claude-agent-acp`；未安装时不启动 ACP 进程，终端端照常使用，页面显示可复制的安装命令，安装后 30 秒内自动重新检测。
- 安装命令跟随 npm `latest`，不是固定版本。本次实测基线是 Claude Code 2.1.316 与连接器 0.99.1，不代表最低版本或对未来 `latest` 的兼容承诺；旧版只提示，新版仍需重新验证。WebMux 与连接器使用 ACP v1，不启用实验性的 v2。
- 聊天端通过 `CLAUDE_CODE_EXECUTABLE` 使用终端端同一个 `claude`；启动配置已设置该变量时以配置为准。两端还复用同一服务用户、`HOME`、`CLAUDE_CONFIG_DIR` 与环境优先级。
- 现有订阅登录、API key、OAuth token 与第三方 provider 可继续由 Claude Code 自己使用；webmux 不新增或保存凭据，不在聊天端登录或注销，命令菜单隐藏 `/login` 与 `/logout`。认证失败时切到终端登录。
- 技术上复用现有登录不等于授予其他分发形态的订阅使用权。部署者应确认服务账号对 Linux 凭据文件或 macOS Keychain 的访问权限，并自行确认实际使用方式符合 Claude 订阅与服务条款；webmux 不作法律或许可判断。
- 首次进入聊天端时，从 `--model`、`--effort` 与权限参数设置模型、推理强度和模式；`--permission-mode manual` 对应 ACP 的 `default`，`--dangerously-skip-permissions` 优先映射为 `bypassPermissions`。其他保留但不能映射的参数会列为“聊天端不生效”。
- 冷 `session/load` 与 `session/resume` 后都重新应用记录的模型和推理强度；模式以连接器从会话恢复的实际值为准，不用旧启动参数覆盖。连接器回退 `auto` 或 `bypassPermissions` 时，界面显示实际模式与通知。
- 0.99.1 声明图片、嵌入资源和插话能力，沿用通用附件与队列管线；空闲时插话返回 `promptRequired`，消息回到队列而不会启动 webmux 未跟踪的回合。
- 行为基线来自 Linux 原生 Claude Code 与当前 API key/第三方 provider 的真实探测。npm 启动器、macOS 与订阅登录未在该基线中实测，不应视为已完成这些平台或账号矩阵的验收。

## 切换

会话页头部的“聊天 | 终端”控件用于切换，窄屏只显示图标。

**切到聊天**需要同时满足以下条件，不满足时目标端禁用并显示原因：

- 识别出受支持的 agent。
- agent 需要第三方连接器时（Codex 的 codex-acp、Claude Code 的 claude-agent-acp），连接器已安装。
- 它是会话 PTY 的根进程（不是从 shell 里再启动的；Codex 与 Claude Code 也接受 npm 安装的 `node` 启动器直接拉起的原生 agent）。
- 已得到 agent 会话 ID：
  - OpenCode、omp、Codex、Claude Code、Antigravity CLI、Qoder 与 QoderCN 都要在发出首条消息后，`session/list` 才包含可加载的会话。
  - Codex 的会话 ID 取自它打开的写锁文件（`$CODEX_HOME/thread-writer-locks/`），Claude Code 取自进程注册记录，Antigravity CLI 由注入的会话级 `--settings` `title` 钩子经 `OSC 7501` 子记录上报。
  - 读不到时按本次 TUI 启动后更新过的会话判定，有多个候选时让你选择。
  - 由 webmux 切换或恢复打开的 TUI 直接沿用它恢复的会话 ID，不必先发消息就能切回。
- agent 的会话列表能查到它。
- 没有被另一个会话占用。

为了核对，webmux 会短暂启动一次 agent 只列出会话（omp 每次启动会写 presence 文件并可能检查更新，OpenCode 会拉起私有服务，Codex 的 codex-acp 冷启动约 2.5 秒），只在会话 ID 变化后与切换前进行。

Claude Code 的主身份来源是 `<CLAUDE_CONFIG_DIR>/sessions/<pid>.json`（未设置时为 `~/.claude`），不是 transcript JSONL。

该注册格式未文档化，因此采用前会核对实际 agent 的 PID、UUID、规范化 cwd、进程启动标识与 PID domain；原生 Linux 直接核对根进程，npm `node` 启动器核对直接 Claude 子进程。原生安装器的磁盘可执行文件可能以版本号命名（例如 `2.1.316`），进程匹配覆盖这种名称，但启动命令仍按 `claude` 识别。

注册记录随身份观察重读，以跟踪 TUI 内的 `/new` 与 `/resume`。首条消息前即使已有 UUID，也仍受 `session/list` 门槛限制。

注册记录缺失、不可读或验证失败时，改用同一 cwd 下、本次 TUI 启动后更新的 `session/list` 候选；没有候选时不能切换，多个候选时由你选择。webmux 也会扫描同配置目录的有效注册记录，尽力发现其他仍在运行的 Claude 进程是否持有同一会话；当前进程、已退出进程与启动标识不匹配的陈旧记录不算占用。

TUI 正在执行回合时，webmux 先发送中断键并等待空闲；10 秒仍未空闲时让你选择继续等待、强制切换或取消。

- 中断键：omp、Codex、Claude Code 与 Antigravity CLI 为一次 Esc；OpenCode 为两次相隔 150 毫秒的 Esc，一次写入的两个 Esc 会被当作 Alt+Esc。
- Codex 启动后的几秒内会报告“工作中”，这段时间不按回合处理；Claude Code 没有这段启动静默期，OSC 7501 应答后报告实际空闲或回合状态。
- Claude Code 的取消只中断当前回合，已经启动的后台 Bash 或子任务可能继续运行。强制切换会终止连接器并等待其受管后代退出，但不保证清理任意自行 daemonize、已脱离受管进程树的程序。
- 切换期间所有设备的输入暂时禁用，已附着的原生终端以“已切换到聊天视图”断开。

**切到终端**时，正在运行的回合会被取消（页面会先确认）；排队中尚未发送的消息合并后退回发起切换的设备的输入草稿，不会自动发给 TUI。

终端用恢复命令重新打开同一个 agent 会话（Claude Code 为 `claude --resume <ID>`，Antigravity CLI 为 `agy --conversation <ID>`）。只保留模型、配置、批准方式等参数；提示词与 `--continue`/`--resume`/`--conversation`/`--settings` 等会话参数会在页面列出为“未保留”。

Claude 的 `session/cancel` 不等于后台任务已清理。切换还会结束连接器并等待受管后代退出，自行脱离进程树的任务不在保证范围内。

两个方向的切换都在原窗格显示进度：切到聊天依次为中断、退出、启动、加载历史；切到终端依次为中断、退出、启动、连接终端，进度一直显示到新的 TUI 在本页输出第一屏内容（最多等 10 秒），终端不会先显示空白。进度至少显示 0.4 秒，切换完成后新窗格淡入；系统开启“减少动态效果”时不播放动画。

切换失败会自动回到原来的视图并显示原因。

## 聊天视图

- 消息、思考（默认折叠）、工具调用（可展开，连续的读取与搜索合并为“已浏览 N 项”，diff 显示增删行数并可在代码面板查看）、计划、用量、压缩与提示按时间线显示。
  - 点击 agent 消息中的代码位置链接（`file://` URI、绝对或相对路径，支持 `#L10-L20`、`:10-20` 行范围与 `#标题` 片段）或工具调用中的文件位置 / 资源链接，会在当前会话的代码浏览器（或已弹出的独立代码窗口）中打开对应文件或目录并跳转到目标行。
- 权限请求与表单在输入框位置作答；多台设备同时打开时，第一个答复生效，其余设备随即看到结果。
- 输入框上方只读展示当前会话所属的仓库名称与 worktree 分支名（每次进入会话时刷新当前检出分支），直接目录会话不显示。
- 输入框下方是 agent 提供的配置项（模型、模式等），输入 `/` 打开命令菜单。
- 输入框下方一行是会话选项：权限与 agent 的配置项（模型、模式等）以图标显示。
  - 悬停提示、可访问名与菜单保留完整名称和当前值；宽度不足时横向滚动而不换行。
  - 窄屏（<640px）不显示这一行，改为上传按钮旁的“会话选项”按钮，点开弹层按 agent 实际提供的选项逐项列出（下拉或开关）。
  - OMP 的模型选项以完整的“提供商/模型”ID 显示，以区分不同提供商的同名模型。
  - “权限”选择决定工具调用的授权方式：
    - “全部接受”（默认）：webmux 直接选择允许（优先“允许一次”，其次“始终允许”），不再提醒，时间线也不记录。
    - “每次询问”：逐个显示权限请求。
  - 从每次询问切到全部接受时，正在等待的权限请求也随即批准；没有允许选项的请求和表单仍需作答。
  - 该设置属于会话本身，所有设备共享，切换两端、重连与重启后恢复都会保留。

- 输入框右侧的圆环显示上下文已用比例；点击弹出 Token 使用情况：已用、上下文窗口、剩余 token 与本会话费用（agent 报告时）。
- 回合进行中继续发送会进入服务端队列，所有设备共享；队列可编辑、上移、删除或立即发送。回合被停止或出错时队列暂停，可选择继续或清空。omp、Codex 与 Claude Code 还支持“插话”，把文字注入当前回合；插话到达时回合恰好结束的，这条消息排到队首作为下一回合发送。
- 输入框左侧的回形针、拖放到聊天视图或粘贴文件 / 图片会走与终端相同的上传接口，可多选；上传中显示文件名、序号与百分比并可取消，完成后以可移除的 chip 显示在输入框上方（图片附件显示缩略小图与文件名），不改动草稿文本。上传与待发送附件属于会话页，切换两端不会中断或丢失，刷新页面后未发送的附件不保留。
- 聊天视图的快捷指令（⚡）位于输入框底部、发送按钮左侧（聊天视图没有底部软键栏，顶栏不再显示该入口）：
  - 菜单标题为“插入消息”，选择指令只把正文插入输入框光标处，不论是否设置了回车都不自动发送；
  - 正文含 `{{dir}}` 或 `{{webmux}}` 的指令（如系统预设“WebMux 工具”）会先确保临时文件夹存在，再替换为真实路径或 WebMux 工具说明。
- 图片在 agent 声明 `promptCapabilities.image` 时作为原生图片块发送，其他文件在声明 `embeddedContext` 时作为嵌入资源发送；不支持时按服务器路径发送并在时间线提示降级，附件不会静默丢弃。
  - 发送后的用户消息气泡中，图片附件同样显示缩略小图与文件名（无法加载时回退为图片图标）。
  - 附件不写入混合会话记录，重启后仅保留上传临时目录的既有清理策略。
- agent 需要登录时，回合以错误结束并显示“切到终端登录”；聊天视图不执行登录流程。
- 会话卡片与 Web Push 沿用现有的提醒：等待授权、需要回答、本轮完成与出错停止。
- 聊天端回合进行中，首页卡片在最新消息上方显示一行当前工作，等待授权时仍显示（工具尚未执行），回合结束后消失：
  - 优先显示当前进行中的工具调用标题，本轮工具调用刚完成而尚无新工具时保留最近完成的工具调用；
  - 工具没有标题时按种类与首个位置显示（如“编辑 /repo/main.go”）；
  - 本轮尚无工具调用时显示计划中进行中的步骤。

聊天端会话保持 agent 进程常驻，不会因空闲自动休眠，因此 agent 内部的后台命令、子任务与定时任务不会在空闲期间被中止；仅在切换到终端端、显式归档会话或服务停止时结束该进程。

## 关闭、中断与恢复

在会话页、首页卡片或侧栏菜单选择“停止会话并归档”并确认后，agent 进程停止，会话变为已归档记录：

- 位置：首页所属工作树（无仓库时为“直接会话”中的目录分组）活动会话下方的“已归档”，默认折叠并显示数量。
- 归档之后：单窗格会话页自动返回首页，“已归档”保持折叠，不自动展开或定位；分屏时只移除该窗格。
- 已中断：服务重启、TUI 自行退出（包括 `/exit` 与崩溃）、ACP 进程反复崩溃或切换回滚失败时，会话标记为“已中断”，同样出现在“已归档”中，折叠标题另外标出已中断条数。
- 操作：每条记录可“恢复到聊天”“恢复到终端”，或在更多菜单中选“删除记录”：若关联的 linked worktree 仍存在，则打开删除 worktree 确认流程一并清理工作树与该目录下的已归档记录；若工作树已不存在、位于主工作区或属于直接目录，则确认后只删除 webmux 的记录与本机草稿，不删除 agent 的会话。
- 查找：首页搜索与“已归档”状态筛选同样作用于这些记录。

- 恢复时若原 agent 会话已不存在，页面列出同一目录下的其他会话供选择。
- 启动配置已被删除时，确认后用原命令恢复，但不再带有配置保存的环境变量。
- 服务重启后队列丢失，恢复后的页面会提示丢失的条数；浏览器中的草稿不受影响。

## 命令行

命令行只用会话 ID 操作，不必先判断会话种类或所在视图：输入、中断、切换、恢复与清理由服务端按当前视图分派。

- 创建：`webmux run --launch-config <名称>` 引用混合配置时创建混合会话，默认从终端开始；本地终端可以照常输入，附着中的终端不提供切换按键。
  - `--view chat` 从聊天开始（隐含 `--detach`），`--prompt` 给出首条提示，`--reply` 等回合结束后输出 agent 的最终回复。
  - `--new-worktree [--branch B]` 先新建 worktree，省略分支名时自动生成；`--set ID=VALUE`、`--permission ask|auto`、`--attach-file PATH` 只在 `--view chat` 时可用。
- 输入与中断：`webmux send <ID> TEXT` 在终端视图写入终端，在聊天视图作为提示发送（`--attach-file`、`--mode steer`、`--reply` 只用于聊天视图）；`webmux interrupt <ID>` 在聊天视图取消回合，在终端视图发送中断键。
- 切换：`webmux view <ID> terminal|chat` 发起切换并等到结束；终端 TUI 忙时 `--force` 强制切换，否则取消切换并以 1 退出。
- 已归档会话：`webmux ls --archived`（或 `--state closed|interrupted|all`、`--repo`）列出，`webmux resume <ID> [--view chat|terminal]` 恢复，`webmux rm <ID> --forget --yes` 删除记录。
- 聊天视图专有操作留在 `webmux chat` 组：`status`、`watch`、`answer`、`set`、`permission` 与 `queue …`；对没有聊天视图的会话以 1 结束，会话在终端视图时提示 `webmux view <ID> chat`。
- `webmux attach <ID>` 在会话处于聊天视图或切换中时拒绝附着，以退出码 1 结束；附着期间被切到聊天视为被动分离，恢复终端、提示后以 0 结束。切回终端后需重新 `attach`。
- `webmux ls` 在状态列附加所在视图（终端、聊天、切换中），`--json` 原样包含 `views`、`view`、`transition` 与 `agent`，不再含 `hybrid` 对象。
- 启动配置：`webmux launch-config add|edit --chat|--no-chat` 开启或关闭混合模式（旧选项 `--hybrid` 已删除）。

完整用法以 `webmux <命令> --help` 为准，见 [用脚本和 agent 控制 WebMux](usage/shell-control.md)。

## 注意事项

- **不要在 webmux 之外同时打开同一个 agent 会话。** webmux 保证自己启动的进程不会同时打开它，但无法保证察觉所有外部进程：
  - omp 在会话被另一进程占用时写入会把它分叉成新会话；
  - Codex 拒绝打开被其他 Codex 客户端（Codex App、IDE、外部终端）占用的会话，切换会回到终端端、恢复返回错误，页面提示“该会话正被另一个 Agent 客户端使用”；
  - Claude Code 允许两个 TUI 注册同一 UUID，webmux 依据有效注册记录尽力返回 `occupied_external`。配置目录不可读、记录不完整或外部客户端不在可验证记录中时，仍可能漏检，不能据此承诺没有外部写者。
- Claude 注册记录只由声明该能力的混合会话按既有观察节奏读取，普通 Shell 中手动运行 Claude 不会触发扫描；读取有频率、64 KiB 单条与 256 条扫描上限。注册路径、PID、进程启动标识、记录正文与凭据不进入 REST/WebSocket 负载、混合记录或日志，客户端只看到解释后的 agent 名、会话 ID 与通用错误。
- OpenCode 的 TUI 在混合会话中以 `--standalone` 运行，Codex 以 `--no-daemon` 运行（命令中可见），结束 TUI 即停止它的回合；代价是启动稍慢，并且看不到后台服务中其他客户端的实时事件。
- 记录保存在 `<state_dir>/hybrid-sessions.json`（0600），不含环境变量值、消息、队列或 token；文件损坏或被另一实例锁定时只禁用混合会话的创建与恢复。
- **回退到旧版本之前，先关闭混合会话并取消所有启动配置的混合勾选。** 旧版本严格读取 `launch-configs.json`，遇到勾选过的配置会报告文件损坏并使启动配置不可用；旧版本会忽略 `hybrid-sessions.json`。
