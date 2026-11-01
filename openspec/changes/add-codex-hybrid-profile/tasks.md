## 1. 实测待定项

- [x] 1.1 在 Linux 上实测 Codex 原生安装与 npm 安装：锁文件路径、`CODEX_HOME` 覆盖、锁由哪个进程持有、`/new`/`/resume` 后锁是否切换；结论回写 design.md Open Questions
- [x] 1.2 实测 bashrc 写法下 `codex-acp` 经 fd 3 握手（含 `.bashrc` 有输出），以及 `command -v` 检查在同一包装下的结果
- [x] 1.3 记录 `codex-acp` 权限请求的 `options` 种类，确认与现有权限卡及自动批准逻辑兼容；确认发布方式是否提供单文件二进制

## 2. 档案模型与 Codex 档案（internal/agentprofile）

- [x] 2.1 `Profile` 增加第三方 `Connector{Executable, Title, InstallCommand, DocsURL, VerifiedVersion}` 与 `ACPEnv`；`ACPArgv` 按声明生成，bashrc 仍走 fd 3
- [x] 2.2 新增 `Codex = "codex"` 档案的命令解析：参数分类表、位置参数为提示词、非交互子命令、`resume`/`fork` → 新原因 `session_arg`
- [x] 2.3 Codex 档案的运行声明：`tuiInject: --no-daemon`、恢复参数 `resume <id>`、一次 Esc、`Resume`、`Steering` 与空闲行为、`Launcher: node`
- [x] 2.4 Codex 档案的聊天端声明：`ReapplyOnLoad: model, reasoning_effort, mode`、聊天端不生效参数、`HiddenCommands: logout`、连接器 `codex-acp`
- [x] 2.5 实现 `Command.ChatConfig()`（argv → `model`/`reasoning_effort`/`mode` 映射与不生效参数列表）与 `Profile.LockedSessions`
- [x] 2.6 `Identify` 识别 `ProgramApp == "codex"`；更新 `Reason.Message()` 中的支持列表；`Support` 增加连接器信息
- [x] 2.7 补充参数表、恢复 argv、ACP argv、播种映射与锁文件解析的表驱动测试

## 3. 进程事实（internal/session）

- [x] 3.1 新增按需的直接子进程可执行文件名与后代进程打开文件路径读取，Linux（`/proc`）与 darwin（`proc_pidinfo`）分平台实现，读取失败视为无结果，条目上限 4096
- [x] 3.2 `StartSidecar` 按会话环境的 PATH 解析 `argv[0]`，并导出供连接器检查复用的 `LookPath`；补配置覆盖 PATH 的测试
- [x] 3.3 补 Linux/darwin 打开文件读取测试（自建子进程打开临时锁文件）

## 4. 连接器检查与资格（internal/hybrid）

- [x] 4.1 实现连接器检查（直接执行查 PATH；bashrc 经伴随进程执行 `command -v`，10 秒超时）与 `CODEX_PATH` 解析、`--version` 读取
- [x] 4.2 新增 `ErrConnectorMissing`/`ConnectorMissingError`；`connect()` 与探测在启动前检查，启动后 `ENOENT` 或 `initialize` 前退出码 127 同样归入
- [x] 4.3 资格判定加入 `connector_missing`（条件 2）、启动器判定（条件 3）、Codex 会话 ID 来源（锁文件优先，`session/list` 候选兜底）；`connector_missing` 期间每 30 秒重查
- [x] 4.4 新增 `session_ambiguous` 冲突与切换请求的可选 `agentSessionId`；`thread_active_writer` 映射为 `occupied_external` 并走回滚
- [x] 4.5 首次打开后播种配置并写入记录，未提供的取值以通知列出
- [x] 4.6 插话带 `idleBehavior: promptRequired` 并按 outcome 处理（重新入队队首 / agent 错误 / 协议错误通知）
- [x] 4.7 ACP 进程注入档案环境（不覆盖配置中的同名变量）；转发命令列表时过滤 `HiddenCommands`
- [x] 4.8 Codex 启动阶段的 `working` 不使切换误判为回合进行中；空闲时不发送 Esc；补测试

## 5. 契约与控制器（internal/contract、internal/wire、internal/api）

- [x] 5.1 `contract` 增加 `AgentConnector`，`HybridSupport.connector`、`ChatEligibility.connector`、`HybridConflict.connector`/候选、`ChatState.outdated`/`connectorMissing`，新原因码 `connector_missing`、`session_ambiguous`、`occupied_external`、`session_arg`
- [x] 5.2 `wire` 转换；`api/errors.go` 把新错误映射为 409 并附原因码
- [x] 5.3 API 与 wire 快照：从聊天开始缺连接器返回 409 且不创建会话、`hybrid-support` 返回连接器信息；重录快照

## 6. 启动配置（internal/launch）

- [x] 6.1 预置 Codex CLI 配置改为 `Hybrid: true`；更新 `store_test.go` 的预置断言，已有文件不改写的测试保持

## 7. Web

- [x] 7.1 `schemas.ts` 增加连接器对象与新原因码；`hybrid.ts` 增加原因文字（连接器原因用连接器显示名称与可执行文件名拼接）
- [x] 7.2 新增复用 `ui/` 组件的 `ConnectorInstallHint`（安装命令可复制、说明链接、Tabler 图标、无障碍名称）
- [x] 7.3 会话页：聊天按钮原因与可关闭提示条；新建会话弹层：从聊天开始旁的说明、`connector_missing` 失败时保持打开并提供“改为从终端开始”
- [x] 7.4 聊天视图：恢复/唤醒/重连因 `connector_missing` 失败时的提示与“切到终端”；`session_ambiguous` 复用候选选择；设置页勾选框旁的连接器说明
- [x] 7.5 补浏览器组件测试与 Storybook 状态；运行 `pnpm --dir web check:arch`

## 8. 测试运行时

- [x] 8.1 fakeagent 增加 codex 档案：OSC 7501 `app=codex`、打开锁文件、`thread_active_writer`、加载后配置回到默认、插话 `idleBehavior`、启动阶段 `working`
- [x] 8.2 `Makefile` 与 `.github/scripts/package-e2e-runtime.mjs` 额外复制 `codex` 与 `codex-acp`；E2E 增加“未安装连接器”（PATH 中不放 `codex-acp`）与 Codex 切换往返用例

## 9. 文档与用户故事

- [x] 9.1 更新 `docs/hybrid-sessions.md`、`docs/deployment.md`（安装 `codex-acp`、服务 PATH、探测成本、外部占用检测）与 `docs/protocol/http-hybrid.md` 的原因码与字段
- [x] 9.2 更新 US-Q01、US-Q03 的验收要点（Codex、外部占用），新增 US-Q16“第三方连接器安装提示”，同步 `docs/user-stories.md` 与 `docs/user-stories/coverage.md` 的索引与计数
- [x] 9.3 运行 `make docs-check`、`make check` 与 `make test`
