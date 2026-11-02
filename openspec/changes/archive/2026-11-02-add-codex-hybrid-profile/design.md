## Context

混合会话的档案层（`internal/agentprofile`）目前假设：ACP 由 agent 自己的可执行文件以 `acp` 子命令提供（`Command.ACPArgv` 硬编码 `"acp"`），
agent 是 PTY 的根进程（`eligibility.go` 中 `id.Executable != ident.Agent` → `not_root`），会话 ID 来自终端信号（Warp 事件或 OSC 7501 子记录）。
Codex CLI 0.165 同时打破这三点：ACP 来自第三方适配器 `codex-acp`；npm 安装时根进程是 `node`；OSC 7501 没有子记录 ID。

#208 记录了实测结论与 C1–C13 决策项，本设计按各项的建议方案定稿；评论中指出的缺口（子进程身份、`adapter_missing` 冒不出来、首次加载无值可重新应用、
分层冲突）一并处理。另外按本次需求，把“第三方连接器启动前检查并在界面提示安装”做成与 agent 无关的能力，而不是 Codex 专用分支。

现有代码事实：

- 探测失败在 `actor.probed` 中被折叠成 `probeNotListed`；从聊天开始时连接器缺失只得到 `link.connect` 的“启动 agent 失败”（502）。
- `StartSidecar` 用 `exec.Command(argv[0])`，可执行文件按服务进程的 PATH 查找，而环境来自 `sessionEnvWithVersion`（含配置覆盖）。
- `reapplyValues` 只读 `Record.Config`，记录初始为空。
- 插话调用 `CallExtension(methodSteering, PromptRequest{…}, nil)`，不带 `_meta`、忽略结果；`PromptRequest` 已有 `Meta` 字段。
- `session.Identity` 只有根进程 PID、可执行文件名与 cwd，注释声明 agent-neutral。

## Goals / Non-Goals

**Goals:**

- 新安装预置的 Codex CLI 默认混合，TUI ↔ 聊天可切换、可恢复，两端模型、推理强度与批准模式一致。
- 声明了第三方连接器的档案，在任何 ACP 启动之前确定连接器是否已安装；未安装时给出稳定原因码和可执行的安装指引，终端端不受影响。
- 保持分层：`session` 只提供 agent 无关的进程事实；Codex 语义只在 `agentprofile`/`hybrid`。

**Non-Goals:**

- 在 webmux 中实现 Codex App Server 协议（C1b）；自动安装 `codex-acp` 或 Node；`npx` 临时执行。
- `--remote`、`--worktree`、`-C` 写法的混合模式；`-c`/`--enable`/`--disable` 转为 `CODEX_CONFIG`（C9a，后续需求明确再做）。
- 延续 TUI 内 `/model` 的改动（C8b，依赖 Codex 内部线程格式）。
- 对 OpenCode/omp 自身可执行文件缺失做专门提示（它们没有第三方连接器，TUI 本身就无法启动）。

## Decisions

### D1 档案声明 ACP 启动方式与连接器（C1a、C2a）

`Profile` 新增 `acp` 描述：要么内置子命令（`subcommand: "acp"`，沿用现状），要么第三方连接器 `Connector{Executable, Title, InstallCommand, DocsURL, VerifiedVersion}`。
`ACPArgv()` 据此生成 `[<agent> acp …]` 或 `[codex-acp]`，bashrc 写法仍用 fd 3 脚本，只是被 exec 的程序换成连接器。
`Profile` 再新增 `acpEnv`（Codex：`NO_BROWSER=1`；`CODEX_PATH` 由 D3 动态解析）。

备选：在设置中配置连接器路径（C2c）——多一处状态且与“只靠档案与 PATH”的部署模型不符；`npx`（C2b）——版本不固定且需要网络。

### D2 连接器检查放在 `hybrid`，进程执行放在 `session`

新增 `hybrid` 内的 `connectorCheck`：输入档案、命令写法、会话环境与 cwd，输出 `installed | missing | error`。

- 直接执行：在会话环境的 PATH 中查找（与 D2a 一致的查找函数）。
- bashrc：经 `session` 的伴随进程能力运行 `bash -ic 'command -v -- "$1" >/dev/null 2>&1' webmux <exe>`，10 秒超时，丢弃输出，退出码 0 为已安装。

D2a：为使“检查与实际启动用同一查找规则”，`StartSidecar` 在启动前按会话环境的 PATH 解析 `argv[0]`（`exec.Command` 默认用服务 PATH，
配置覆盖 PATH 时两者不一致）。这一修正对所有伴随进程生效，是行为上的缩小差异而非新功能；检查函数与启动共用它。

调用点：`sessions()`（探测）、`connect()`（新建/切换/恢复/唤醒/重连）。`connect` 中检查失败返回新哨兵 `ErrConnectorMissing`，
携带 `*ConnectorMissingError{Connector}`；`api/errors.go` 映射为 409 `connector_missing`，`contract.HybridConflict` 增加 `connector` 字段。
启动后 `ENOENT` 或 bashrc 在 `initialize` 前退出码 127 同样归入该错误，覆盖检查与启动之间的竞态。

探测路径：`probeResult` 增加 `probeConnectorMissing`，`listedReason` 返回 `connector_missing`；资格判定在条件 1 之后、条件 3 之前检查（缓存结果，
`connector_missing` 期间每 30 秒重查，切换前必查）。这样未安装时不会每次都冷启动注定失败的探测。

备选：只在启动失败后识别 127/ENOENT——实现简单，但无法在资格判定中提前给出原因，且 bashrc 写法下 127 也可能来自 `.bashrc` 自身。

### D3 `CODEX_PATH` 指向 TUI 使用的 codex（C3a）

与连接器检查同一次解析：bashrc 写法用一个 `bash -ic` 依次输出两者的 `command -v` 结果（各一行），直接执行则在会话 PATH 中查找。
解析结果按会话缓存，随连接器检查一起刷新；配置已设置 `CODEX_PATH` 时不解析。找不到 `codex` 时不注入并在聊天视图提示。
Codex 版本由对该路径执行 `--version`（5 秒超时）取得，只用于版本提示。

### D4 会话 ID 来自线程锁，`session` 只提供进程事实（C4a+c、C5a）

`session` 新增按需方法（不进入 `Identity` 的常驻字段）：

- `RunChildren()`：根进程直接子进程的可执行文件名；
- `RunOpenFiles()`：根进程及后代打开的普通文件绝对路径（Linux `/proc/<pid>/fd` 的 `readlink`；darwin `proc_pidinfo(PROC_PIDLISTFDS)` +
  `PROC_PIDFDVNODEPATHINFO`），按平台拆分为 `proc_files_linux.go`/`proc_files_darwin.go`，上限 4096 条。

`agentprofile` 为 Codex 声明锁目录布局（`LockedSessions(paths, getenv, home)`）与 `Launcher: "node"`。`hybrid` 在档案声明时才调用上述方法，
并以 `lockTracker` 按锁出现的先后判定当前会话（实测 `/new` 保留旧锁、`resume` 另持空白会话的锁，见 Open Questions），
满足评论中“避免 `session` 理解 Codex”的分层要求；`internal/archtest` 规则不变。

兜底 C4c：读不到打开的文件或锁无法判定时，用 `session/list` 候选（更新时间晚于本次 TUI 启动；本会话在聊天端的加载早于这次启动，因而自然排除）。
多候选复用恢复时的候选选择（`SessionMissingError` 同形的 `SessionAmbiguousError`，409 `session_ambiguous`），
切换请求增加可选 `agentSessionId`，只用于切到聊天。C4d（上游在 OSC 7501 加 `id=`）不在本变更范围，另行提议。

### D5 强制 `--no-daemon`（C6a）

复用 `tuiInject`，与 OpenCode 的 `--standalone` 相同。代价：TUI 看不到 daemon 中的其他任务，记入文档。

### D6 配置播种与重新应用（C7a、C8a）

`agentprofile` 提供 `Command.SeedConfig() (values map[string]string, unsupported []string)`，内部使用未导出的 `kept`，映射表放在档案（适配器 mode ID 与 TUI 参数不同名）。
`hybrid` 在第一次打开（`session/new` 或首次 `session/load`）后，对记录中缺失的项调用 `session/set_config_option` 并写入记录；之后沿用 `ReapplyOnLoad`。
适配器插入的“resuming with …”警告不过滤（webmux 不改写 agent 内容）。

### D7 插话的空闲行为（C12a）

档案新增 `steeringIdlePrompt bool`。为真时 `steer` 带 `_meta.steering.idleBehavior = "promptRequired"`，解析 `{outcome}`：
`promptRequired` → 以队首条目重新入队；`failed` → `ErrAgentCall` 并退回草稿；`startedNewTurn` → 协议错误通知。omp 行为不变。

### D8 会话类子命令与隐藏命令（C11a）

`agentprofile` 新增原因 `session_arg`；Codex `resume`/`fork` 子命令归入。档案新增 `hiddenCommands`（Codex：`logout`），
`hybrid` 在转发 `available_commands_update` 时过滤。

### D9 外部写者（#7）

`agentFailure` 识别 JSON-RPC `-32600` 且 `data.reason == "thread_active_writer"`，映射为 `occupied_external`，走现有回滚。

### D10 前端

无按 agent 分支：`schemas.ts` 增加 `connector` 对象、新原因码；`hybrid.ts` 增加文字；新增一个复用 `ui/` 组件（Alert + 复制按钮）的 `ConnectorInstallHint`，
用于会话页提示条、新建会话弹层、聊天视图通知与设置页，避免四处各写一份。

## Risks / Trade-offs

- [线程锁路径是 Codex 未公开的内部实现] → 仅作为主路径，失败退回候选判定；fakeagent 覆盖两条路径；版本提示记录实测版本。
- [bashrc 检查每次多一个 `bash -ic`，`.bashrc` 慢时拖慢资格判定] → 结果按会话缓存，只在 `connector_missing` 期间每 30 秒与切换前重查。
- [检查通过但连接器运行时（Node）缺失] → 连接器启动后立即退出，归入既有“启动 agent 失败”并附 stderr 尾部；不伪装成未安装。
- [`StartSidecar` 改为按会话 PATH 解析影响现有 agent] → 只在配置覆盖 PATH 时结果不同，且此时新行为才与 TUI 一致；补测试。
- [适配器自带 Codex 与 TUI 版本不同写同一状态库] → D3 让两端使用同一个 codex；找不到时提示。
- [探测冷启动约 2.5 秒，最多 2 个并发] → 维持 `maxProbes=2`，文档说明。
- [Codex 启动阶段短暂上报 `working`] → 资格判定与 `tuiBusy` 增加档案级启动静默期，测试覆盖（见 tasks）。

## Migration Plan

- 只影响新安装的预置配置；已有 `launch-configs.json` 不改写，用户可手动勾选。
- wire 变更为新增字段与新原因码，旧客户端忽略未知字段；原因码文字缺失时回退显示原因码本身（现有逻辑）。
- 回滚：移除 Codex 档案后，记录中 `agent: "codex"` 的混合会话按现有“未知档案”处理为不可恢复。

## Open Questions

第 1 组任务的实测结论（Linux x86_64、Codex 0.166.0 原生安装、codex-acp 2.5.0）：

- 锁文件为 `$CODEX_HOME/thread-writer-locks/<UUIDv7>.lock`，`CODEX_HOME` 覆盖生效，原生安装时由根进程 `codex` 持有；
  会话在首条消息前就已持有锁，但 `session/list` 只在首条消息后列出。
- npm 安装（`@openai/codex` 0.166.0）的包装脚本在 Node 提供 `process.execve`（22.15 / 23.11 起）时直接替换为原生 `codex`，
  根进程即 `codex`（Node 22.23 与 26.12 实测）；更早的 Node 才以 `node` 启动器 `spawn` 原生 `codex` 子进程（据包装脚本源码，未实测），
  启动器判定为这种情况保留。
- `/new`（以及选择工作树后的新会话）打开新锁且不释放旧锁；`codex resume <id>` 同时持有被恢复会话与一个空白会话的锁。
  据此把“多个锁即无法确定”改为按出现先后判定（已更新 `agent-session-identity` 增量规格与 D4）。
- TUI 启动时先上报 `state=working:app=codex` 再上报 idle，没有 OSC 7501 子记录 ID，证实需要启动静默期与锁文件。
- codex-acp 的 `session/load` 把 `mode`、`model`、`reasoning_effort` 重置为默认值；外部客户端持有会话时
  `session/load` 与 `session/resume` 都返回 `-32600`、`data.reason = "thread_active_writer"`；插话的 `idleBehavior` 与 `outcome` 与设计一致；
  `initialize` 以 `_meta.steering.supported` 声明插话，`agentInfo.version` 是适配器版本。
- bashrc 写法与 omp 相同经 fd 3 握手；连接器检查在 `.bashrc` 有输出时仍能取到标记行（Go 测试覆盖）。
- 权限请求沿用 ACP 标准 `options`（`allow_once` 等），现有权限卡与自动批准逻辑无需改动。
- npm 包的 `package.json` 只有 bun 单文件打包脚本，安装说明仍以 `npm install -g` 为准。
