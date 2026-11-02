## Context

当前混合会话已经具备 ACP v1、单写者切换、第三方连接器检查、候选选择、配置播种、事件续接和恢复机制。#230 后的 `agentprofile.Agent` 与可选能力接口是正确扩展点；本设计以当前实现和未归档的 `add-codex-hybrid-profile` 为共同基线，不回退到主 specs 中“仅 OpenCode/omp”的旧状态。

调研依据：

- #181 记录了 Claude Code 首期暂缓的原因，包括旧适配器认证失败；#208/#217 建立了第三方连接器模式，#222/#230 建立了 agent 适配器边界。
- Claude Code 官方文档提供 Agent SDK 与 headless 接口，未提供原生 ACP server。当前连接器为 `@agentclientprotocol/claude-agent-acp`，旧 `@zed-industries/claude-code-acp` 已被 npm 标记 deprecated。
- 本次核查的稳定版为 0.99.1，要求 Node 22+，可执行文件为 `claude-agent-acp`。它默认按客户端版本协商 ACP v1/v2；WebMux 继续请求 v1。v2 仍是 experimental/draft，不纳入本次实现。
- 本机 Claude Code 为 2.1.316（原生 Linux binary），Node 为 26.13.0，连接器为 0.99.1；真实请求复用当前 API key/第三方 provider，实际模型为 k3.1-pro。CLI 未显示原生 ACP 参数，支持 `--resume`、model/effort/permission-mode 等参数。以上版本是实测基线，不是最低版本或未来兼容承诺。
- 连接器的 v1 能力包括 list/load/resume、config options、权限、图片/嵌入资源和 steering。`session/load` 回放完整历史，`session/resume` 不回放；model/effort/mode 使用 `session/set_config_option`，不是假设独立的 `session/set_model`。
- Claude CLI 的 `<CLAUDE_CONFIG_DIR>/sessions/<pid>.json` 注册记录包含 sessionId、PID、cwd、procStart、pidDomain 等字段。连接器用它补充列表活动状态；该格式未文档化，因此必须验证并保留降级路径。
  - Linux 实测 procStart 是 `/proc/<pid>/stat` 第 22 字段的十进制 ticks 字符串，不是 `ps lstart`。
  - pidDomain 是 `linux:<machine-id>:<pid-namespace>`，不是 boot ID。

主要来源：

- [混合会话设计 #181](https://github.com/mzdluo123/webmux/issues/181)、[Codex 设计 #208](https://github.com/mzdluo123/webmux/issues/208)、[适配器重构 #222](https://github.com/mzdluo123/webmux/issues/222)
- [Claude ACP 连接器](https://github.com/agentclientprotocol/claude-agent-acp)、[0.99.1 release](https://github.com/agentclientprotocol/claude-agent-acp/releases/tag/v0.99.1)
- [ACP v1 session setup](https://agentclientprotocol.com/protocol/v1/session-setup.md)、[Claude CLI reference](https://code.claude.com/docs/en/cli-reference.md)、[Agent SDK sessions](https://code.claude.com/docs/en/agent-sdk/sessions.md)
- [连接器 live registry](https://github.com/agentclientprotocol/claude-agent-acp/blob/v0.99.1/src/session-index/live-registry.ts)、[Agent SDK 使用条款提醒](https://code.claude.com/docs/en/agent-sdk/quickstart.md)

## Measured Baseline

2026-11-02 在本机完成真实探测。所有用例使用隔离的空工作目录，不连接已有开发者会话，不输出凭据或对话正文。探测父进程本身运行于 Claude Code 中，因此子进程移除 `CLAUDE_CODE_CHILD_SESSION` 等会话标记；继承该标记会让 TUI 明确关闭 transcript 保存，不能作为正常 WebMux 服务行为。

- initialize 请求与响应均为 ACP v1，agentInfo 为连接器 0.99.1；声明 list/load/resume、image/embeddedContext、notices 与 steering 所需能力，真实 new/prompt 成功。
- 原生 TUI 使用根 PID 注册；ACP 使用连接器的直接 Claude 子进程。记录 PID、cwd、procStart、pidDomain 均与上述 Linux 事实匹配。
- 首条消息前注册记录已存在但 list 不含 ID；首条 prompt 后包含。`/new` 更新 UUID，新 ID 在首次 prompt 前仍不在 list 中，即使元数据 transcript 已非空。TUI `--resume` 保持 UUID，退出删除记录。
- 两个原生 TUI 可同时以不同 PID 注册同一 UUID，外部占用检测不能省略。
- TUI 在约 0.44 秒发出 OSC 7501 查询，应答后立即发布 idle/app；启动无假 working。prompt 为 working → done，一次 Esc 为 working → idle，退出为 clear。
- 在当前 provider 上，set mode auto 与 bypassPermissions 均发布请求值，无 Auto 回退；其他模型的条件回退仍以适配器实际配置和通知为准。
- 冷恢复配置实测：请求 sonnet/high/plan 并完成 prompt 后，冷 load 与冷 resume 都回到 opus/medium，但恢复 plan。因此 model 与 effort 必须在 load 和 resume 后重应用，mode 不应由过期 argv 覆盖。这里的 opus/medium 是当前环境默认值，不是所有部署的固定值。
- load 在响应前回放 user/thought/agent 历史，resume 无历史 update；load/resume 响应都发布 configOptions。
- 空闲 steering 返回 promptRequired/noRunningTurn；cancel 返回 cancelled。图片与嵌入资源在同一真实 prompt 中完成 end_turn。
- default 模式的 Bash 写文件请求触发 permission，拒绝后文件未创建。连接器发布的 48 个命令中已无 login/logout。
- 后台 sleep 在 cancel 后一秒仍运行；终止连接器后受测后代全部退出。force-switch 文案和进程组清理必须反映这一边界。
- 未把 npm launcher、macOS、订阅登录或任意自行 daemonize 进程写成已实测结论；它们不阻碍按本机基线实现，后续平台测试不得冒称已覆盖。

## Goals / Non-Goals

**Goals:**

- 新安装的 Claude Code 默认混合；已有安装可手动开启，两端按同一 agent session ID 接续对话。
- 复用通用连接器检查、候选、切换、恢复、队列、权限、附件和通知，不增加 Claude 专用前端工作流。
- 以可验证的进程注册记录跟踪 TUI 会话，失效时安全降级；尽力避免外部客户端同时写入。
- 两端使用同一 Claude Code、配置目录与现有认证，模型、effort 和权限模式的实际状态清晰可见。
- 以真实 round-trip 和故障验收支撑 fakeagent/E2E，不能把协议声明等同于产品可用。

**Non-Goals:**

- 不实现 Claude Agent SDK 或原生 ACP server，不自动安装 Node/连接器，不使用 `npx` 临时下载执行。
- 不实现 ACP v2，不升级现有 ACP 协议栈，不支持任意 shell 包装、会话参数、worktree 或远程/后台启动方式。
- 不解析 Claude transcript JSONL，不把 WebMux 变成 durable agent session store。
- 不新增 agent-owned display terminal 子系统；首版使用现有工具卡和标准输出，未识别的 terminal metadata 不阻塞其他事件。
- 不保证浮动 `latest` 的所有未来版本兼容，不承诺发现配置目录之外或无法验证的外部客户端。
- 不对多用户托管、向第三方终端用户提供订阅额度等部署方式作法律或许可结论。

## Decisions

### D1 使用当前连接器，安装跟随 latest

`claudecode` 档案声明连接器 `claude-agent-acp`、安装命令 `npm install -g @agentclientprotocol/claude-agent-acp`、说明仓库和本次实测版本 0.99.1。安装不固定版本，这是本次明确决策，与现有 Codex 安装策略一致。

`VerifiedVersion` 只驱动旧版本提示，不阻断新旧版本；真实资格仍由连接器检查、ACP 握手和会话核对决定。升级实测基线时必须重新运行兼容套件。

备选：安装命令固定 `@0.99.1`，可减少漂移，但本次选择跟随 latest。使用旧 `@zed-industries/claude-code-acp` 或 preview 均被排除。

### D2 固定 ACP v1，并让两端使用同一个 Claude Code

WebMux 的 initialize 继续发送 `protocolVersion: 1`，连接器在默认 `auto` 下据此进入 v1；不通过升级 WebMux 协议或强制 v2 获取功能。测试须断言真实协商结果，防止后续版本改变默认行为。

复用 `AgentPathEnv` 机制注入 `CLAUDE_CODE_EXECUTABLE`，路径按会话环境和命令包装解析。这样连接器不会默认使用 Agent SDK 自带的另一补丁版本。配置已显式设置该变量时保留用户值，并通过版本提示暴露差异。

备选：使用 SDK bundled CLI，部署简单，但会与 TUI 形成版本和状态格式矩阵，不采用。

### D3 复用现有 Claude 身份，不在聊天端登录

TUI 与 ACP 使用同一服务用户、HOME、CLAUDE_CONFIG_DIR 和环境优先级，支持现有订阅登录、API key、OAuth token 与第三方 provider。WebMux 不新增 token 字段，不记录凭据，不自动调用登录/注销，隐藏聊天菜单中的 login/logout，认证失败沿用“切到终端登录”。

部署文档说明 Linux 凭据文件、macOS Keychain、服务账号权限和订阅使用条款需由部署者确认。技术复用不等于为其他分发形态授予订阅使用权。

备选：只允许 API key/provider，可缩小条款与凭据范围，但不符合本次“复用现有登录”的决策。

### D4 新增 Claude adapter，差异不进入通用编排

新增 `internal/agentprofile/claudecode` 并注册到 `agents.Default`，稳定 ID 为 `claudecode`。档案声明：

- executable `claude`、OSC app `claude-code`、允许 npm `node` 启动器；
- connector、`CLAUDE_CODE_EXECUTABLE`、resume `--resume <id>`、一次 Esc；
- `session/resume`、带 `promptRequired` 的 steering、隐藏 login/logout；
- `StartupQuiet=0`，force-switch 说明明确 cancel 后后台任务可能存活，强制切换终止连接器并等待受管后代退出；不注入 daemon/standalone 参数。

`session`、`acp`、通用 `hybrid` 和前端不得出现 Claude 名称。新包加入 `internal/archtest` 分层清单，并复用 `agenttest` 契约。

参数表新增通用 `ArgSession` 分类，使选项形式的 `--resume`、`--continue`、`--session-id` 等也能复用现有 `session_arg`，而不伪装成非交互参数。其他参数规则如下：

- 新增可变参数个数 arity，消费至下一选项，用于 add-dir、mcp-config、工具列表与 betas。
- 工作树、不持久化和非交互参数分别复用 directory/store/non-interactive 原因。
- `--bare` 是保留的精简 TUI 参数，不是非交互参数。
- CLI permission-mode 的 default（manual 为别名）直接映射为 ACP 的 default。

### D5 进程注册记录是主身份来源，候选是降级

`session` 只新增 agent-neutral 的根/直接子进程事实：PID、executable、cwd、启动标识和必要的 PID domain。`agentprofile` 新增按进程注册记录识别会话的可选能力；Claude adapter 负责配置目录、记录格式和验证，通用 hybrid 负责观察、候选、占用和切换时机。

直接运行时读取根 PID；node 启动器读取其直接 Claude 子进程 PID。CLAUDE_CONFIG_DIR 非空时按会话环境解析，否则使用 `~/.claude`；相对路径以实际 agent cwd 为基准，比较前规范化路径。读取必须有大小和条目上限，不得输出记录内容。

接受记录前验证 PID、UUID、cwd、procStart、pidDomain 与当前进程事实：

- Linux procStart 读取 `/proc/<pid>/stat` 第 22 字段并按十进制字符串比较，pidDomain 使用 machine-id 与 PID namespace；不得改用 `ps lstart` 或 boot ID。
- 当前记录按既有观察节奏重读，以跟踪 `/new` 和 `/resume`。
- 扫描同配置目录的其他有效记录可发现外部占用，排除当前进程、已退出进程和陈旧记录，映射为 `occupied_external`。

记录缺失或不可信时使用 cwd + runStart 的 `session/list` 候选，多候选复用 409 `session_ambiguous`。首条消息前即使已有 ID，只要 list 没有可加载 transcript，仍不可切换。

备选：直接解析 transcript JSONL，或简单选择 cwd 最新会话。前者绑定更大的私有格式，后者在同目录并发时容易加载错误对话，均不作为主路径。

### D6 配置从 argv 播种，以适配器实际状态为准

`ChatSide` 映射 `--model`、`--effort` 与权限参数到 `model`、`effort`、`mode`。`default` 与别名 `manual` 映射为 `default`，危险跳过权限参数优先映射 `bypassPermissions`；`dontAsk` 和未知值不静默改名。其他保留但不透传的参数列入聊天端不生效提示。

首次 new/load 后播种；冷 load 与冷 resume 实测都会丢失 model 与 effort，因此两者都在这两种恢复后重应用记录值。mode 会从 transcript 恢复，不列入 reapply；new 会话仍使用 argv 播种值。auto/bypassPermissions 被运行条件回退时，记录与界面采用实际值并显示适配器通知；当前 provider 实测两者均直接生效。

模型标签沿用服务端/ACP 通用数据与最新 adapter label 机制，不在 Web 添加 Claude 特例。其他会话配置、表单、权限和附件继续使用现有协议与 reducer。

### D7 首版只消费已有聊天能力

v1 的 list/load/resume、config options、permission/form、image/embeddedContext 和 steering 足以完成首版。steering 使用现有 `IdlePrompt` 结果处理，空闲时回到队列，不能允许连接器默认启动额外回合。

agent-owned terminal metadata、子代理私有扩展和 v2 专属更新不纳入首版。未知 ACP update 继续按现有容错规则忽略；若标准工具内容不足成为实测阻塞项，应另开通用协议变更，而不是在翻译层添加 Claude 名称。

### D8 本地真实 spike 已固定基线，fakeagent 是回归层

第一组真实探测已在隔离配置使用方式、临时工作目录和当前 provider 下完成，不连接已有开发者会话，不输出凭据或正文日志。Measured Baseline 与 delta specs 已记录 v1 帧、认证、注册记录、list 可见性、OSC、配置恢复、权限、附件、steering、cancel 和进程清理结果，实现阶段不再重复探测本机行为。

fakeagent 按这些固定行为模拟，E2E 不访问真实账号。实现完成后仍执行一次真实双向 round-trip 验收，以验证 WebMux 集成而非重新调研 Claude；npm/macOS 与订阅等未覆盖矩阵单独标注，不阻塞本机基线的实现。

### D9 优先不扩展线上契约

现有 connector、candidate、occupied_external、chat config 和错误映射足以表达本方案。前端只消费 supportedAgents、connector 和通用状态，不新增 Claude 分支。

若实测确需新增通用概念，必须同步 contract、wire、API errors、Zod、hub/frontend 处理、消费者和 wire 快照；不得只放宽 JSON 或绕开架构守卫。

## Risks / Trade-offs

- [跟随 latest 引入未测试变更] → 保留实测版本提示与 v1 握手断言；升级时运行真实兼容套件，文档不承诺未来版本。
- [注册记录是未文档化格式] → 严格防伪、大小限制和候选降级；不解析更大的 transcript 格式，版本升级复测。
- [旧适配器曾认证失败] → 当前 API key/第三方 provider 的真实 new/prompt 已通过；订阅矩阵未宣称覆盖，失败仍提供 TUI 登录和 connector/auth 提示，不静默改凭据。
- [订阅登录的分发条款不随技术能力自动解决] → 限定本文结论为单用户本机自用，文档要求部署者确认实际使用方式，不作法律判断。
- [强制 CLAUDE_CODE_EXECUTABLE 带来 SDK/CLI 版本差异] → 两端一致优先，记录版本矩阵并显示旧版提示；找不到路径时沿用通用路径解析错误，不伪造成功。
- [cancel 后后台 Bash 或子任务可能存活] → 已实测 cancel 后 sleep 存活；force-switch 明确提示并终止连接器、等待受管后代退出，不把 cancel 描述为整个进程树的 kill。
- [长 transcript 回放超时或重复] → 验证 load 响应前完整回放、120 秒边界、事件 generation/seq 和背压；需要改通用协议时另列任务。
- [auto/bypassPermissions 不可用] → 以适配器实际配置和通知为准，不隐藏回退。
- [外部注册记录不可读或不完整] → 保留 WebMux 内部检测和候选，不明确承诺无外部写者。
- [普通会话扫描造成隐私或性能问题] → 只对声明该能力的混合会话按需读取，限制频率/大小/条目，路径与正文不入日志或负载。

## Migration Plan

1. 新安装播种时将 Claude Code 的 `Hybrid` 改为 true；已有 launch-configs.json 不改写，已有用户手动勾选。
2. 部署聊天端前安装 Node 22+ 与连接器，并确保检查和实际启动使用同一会话 PATH；缺连接器只影响 ACP 端。
3. 不迁移混合记录格式或线上字段。Claude 记录使用稳定 agent ID `claudecode`，沿用现有恢复记录。
4. 回滚时先把新安装预置恢复为非混合，再移除档案；已有勾选配置需先取消混合。被移除档案的既有记录按现有未知档案规则处理，不删除 Claude transcript。

## Open Questions

本机实现基线没有待行为探测的开放问题。以下仅是不冒称已覆盖的部署边界，不要求实现者重新检查本机 Claude Code：

- npm launcher 与 macOS 的进程启动标识/domain 由对应平台实现和测试验证；Linux 原生路径严格按本设计，不用未实测平台结果改写。
- 订阅登录未单独实测；当前 API key/第三方 provider 已验证，认证失败继续沿用通用错误路径。
- form elicitation、超长历史 120 秒边界与任意 daemonize 后台进程不属于本次已测矩阵；通用管线保持现有行为，交付说明不得冒称专项覆盖。
