## 1. 真实基线与已固定行为

- [x] 1.1 在本机隔离工作目录中使用 Claude Code 2.1.316、Node 26.13.0 与连接器 0.99.1，完成真实 initialize/new/list/load/resume/prompt/cancel 帧并断言 ACP v1；未输出凭据或正文日志
- [x] 1.2 复用当前服务用户的 API key/第三方 provider 完成真实 round-trip；确认 `CLAUDE_CODE_EXECUTABLE` 指向同一原生 Claude，凭据不进入探测输出
- [x] 1.3 实测 Linux 原生根进程注册记录、procStart/pidDomain/cwd、首条消息前后的 list 可见性、`/new`、TUI `--resume`、外部重复注册与退出删除
- [x] 1.4 实测冷 load 完整回放、冷 resume 不回放；固定 model/effort 在 load/resume 后重应用、mode 由 transcript 恢复，以及当前 provider 的 auto/bypassPermissions 实际行为
- [x] 1.5 实测权限拒绝、图片与嵌入资源、idle steering、OSC 7501、StartupQuiet=0、一次 Esc，以及 cancel 后后台进程存活和连接器退出后的受管进程清理
- [x] 1.6 把实测矩阵与行为回写 delta specs 和 design；本机实现不再依赖待完成的 Claude 行为探测，未覆盖平台与部署矩阵明确保留边界

## 2. Claude Code 档案与参数（internal/agentprofile）

- [x] 2.1 为参数分类增加通用 `ArgSession`，让选项形式的会话参数复用 `session_arg`；增加消费至下一选项的可变参数个数 arity，补原因文案、argv 解析和回归测试
- [x] 2.2 新增 `internal/agentprofile/claudecode`，实现 Agent/Resumer/ConfigReapplier/Steerer，在 agents.Default 注册并加入 archtest 分层清单
- [x] 2.3 建立 Claude 参数表：直接/bashrc 写法、session/store/directory/non-interactive、保留与丢弃参数、位置提示词及子命令；覆盖 `--flag value` 与 `--flag=value`
- [x] 2.4 实现 `claude`/`app=claude-code` 识别、node 启动器、`--resume <id>` 恢复 argv、一次 Esc 和 `StartupQuiet=0`；补首次启动、OSC 状态与恢复测试
- [x] 2.5 声明 `claude-agent-acp` 连接器、无参数 ACP argv、`CLAUDE_CODE_EXECUTABLE`、实测版本、隐藏 login/logout；force-switch 明确 cancel 不清理后台任务、强制切换等待受管后代退出，覆盖 bashrc fd3 和同 PATH 解析
- [x] 2.6 实现 argv 到 model/effort/mode 的 ChatSide 映射、manual→default、权限优先级和不生效参数列表；model/effort 在冷 load 与 resume 后重应用，mode 不从过期 argv 覆盖，测试实际回退不会显示为请求值已生效
- [x] 2.7 接入通用 agenttest 契约与 registry 测试，确认 supportedAgents、混合支持原因和其他三个档案行为不变，通用 Go/Web 不出现 Claude 名称

## 3. 进程事实与 Claude 会话身份

- [x] 3.1 在 `internal/session` 增加 agent-neutral 的根/直接子进程 PID、executable、cwd、启动标识与 PID domain 查询，分别实现 Linux/macOS 并测试退出、权限不足和 PID 复用
- [x] 3.2 在 agentprofile 增加进程注册记录可选能力；Claude adapter 实现配置目录解析、有界 JSON 读取、UUID/PID/cwd 验证，Linux 按 stat ticks 与 machine-id/PID namespace 验证 procStart/pidDomain，不解析 transcript JSONL
- [x] 3.3 在 hybrid 身份观察中接入当前注册记录，支持原生/node 启动器、TUI 内 ID 变化、陈旧记录拒绝和首条消息前不可切换；补定时与状态发布测试
- [x] 3.4 注册记录不可用时接入 cwd + runStart 候选降级，覆盖零候选、单候选、多候选 409 `session_ambiguous` 与用户选择后重试
- [x] 3.5 扫描有效注册记录实现外部 Claude 占用检测，排除当前/退出/陈旧进程并映射 `occupied_external`；覆盖资格禁用、切换失败回滚及不可读时的降级
- [x] 3.6 验证普通会话不读取注册记录，读取频率/大小/条目有界，PID、路径、记录正文和凭据不进入 REST/WS、混合记录或日志

## 4. 混合编排、API 与启动配置

- [x] 4.1 复用现有连接器检查覆盖 Claude 的探测、新建、切换、恢复、唤醒和重连；缺连接器不创建会话/记录、不影响 TUI，并复用现有 `connector_missing` 负载
- [x] 4.2 接入首次配置播种、load/resume 后 model/effort 重应用与实际 mode 同步、steering、附件和认证失败路径；若实现需要通用协议修复，先同步更新对应 delta spec 与设计
- [x] 4.3 将新安装 Claude Code 预置改为 `Hybrid: true`，保持现有 11 条预置和 Gemini/其他非混合配置不变；测试已有 launch-configs.json 不改写
- [x] 4.4 更新 hybrid-support、切换、冲突和恢复 API 测试；优先保持线上形状不变，若确有通用字段变化则同步 contract/wire/Zod/errors 并重录 wire 快照
- [x] 4.5 增加 Claude hybrid 记录恢复、进程退出、服务重启、关闭后恢复、休眠唤醒与自动重连测试，保持单写者、退出发布顺序和 gap-free 事件续接

## 5. Web 与可访问性

- [x] 5.1 复用动态 supportedAgents、ConnectorInstallHint、候选选择、配置通知和聊天视图，不新增 Claude 前端分支；如有通用展示调整，先使用 ui/ 组件 variant/size
- [x] 5.2 补 Claude 的 Storybook/浏览器状态：默认混合、连接器缺失、安装后重查、首条消息前、候选、外部占用、权限回退、隐藏认证命令与恢复失败
- [x] 5.3 验证窄屏会话选项、键盘/触摸操作、tooltip/accessible name 和深浅主题；运行 `pnpm --dir web check:arch`，准备隔离 demo 数据的桌面/移动前后截图与可审查 URL

## 6. Fakeagent 与 E2E 回归

- [x] 6.1 扩展 fakeagent 的 Claude TUI：OSC 查询/idle/working/done/clear、注册记录写入/更新、node 启动器、首条消息时点、外部占用、StartupQuiet=0 和一次 Esc 中断
- [x] 6.2 扩展 fake ACP：v1 能力、list/load/resume、model/effort/mode 更新与回退、权限/form、附件、steering `promptRequired`、auth 和 connector 故障注入
- [x] 6.3 更新 Makefile、E2E global-setup/server 与打包脚本的 `claude`/`claude-agent-acp` 测试别名和运行时复制，保证缺连接器用例使用隔离 PATH
- [x] 6.4 增加 Claude E2E：从聊天/终端开始、双向 round-trip、回合中切换、长回放、队列/插话、权限、附件、候选、外部占用、休眠/重启恢复和缺连接器；E2E 不访问真实账号
- [x] 6.5 运行相关 Go race、agentprofile/hybrid/acp、Vitest/browser 和 Playwright 定向测试，消除时序抖动后再进入完整门禁

## 7. 文档、用户故事与最终验证

- [x] 7.1 更新 `docs/hybrid-sessions.md`、部署和 `docs/protocol/http-hybrid.md`：Node 22+、连接器安装、latest 风险、认证/订阅条款提醒、注册记录与外部占用边界、后台任务和恢复限制
- [x] 7.2 更新 US-Q01、US-Q03、US-Q04、US-Q06、US-Q07、US-Q08、US-Q10、US-Q12、US-Q14、US-Q15、US-Q16 中受影响的 Claude 验收要点，同步 `docs/user-stories.md` 与 coverage 索引/计数，保持 ID 稳定
- [x] 7.3 按已固定的本机基线完成实现后的真实双向 round-trip，记录版本、平台、认证类型、场景和未覆盖限制；不把 fakeagent/E2E 写成真实账号验证，整理 UI 截图证据并核对所有链接
- [x] 7.4 运行 `openspec validate add-claude-code-hybrid-profile --strict`、`make format`、`make docs-check` 和 `make check`，修复所有格式、文档、架构与 lint 问题
- [x] 7.5 运行完整 `make test`，确认 Go race、Vitest/browser、Storybook、Playwright E2E 与性能套件通过；交付摘要列出受影响 US-Q ID、真实/模拟验证边界和已知限制
