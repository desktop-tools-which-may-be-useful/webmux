## 1. 契约与 wire

- [x] 1.1 在 `internal/contract` 中以 `state`、`views`、`view`、`transition`、`agent`、`archive` 取代 `SessionInfo.hybrid`，删除 `SessionHybrid` 与 `HybridRecord`，`RemovalPreview.archived` 与 `SessionDetails` 改用新形状
- [x] 1.2 把视图取值改为 `terminal`/`chat`，`HybridSide`/`HybridPhase`/`HybridState` 改名为视图、切换阶段与会话状态类型，删除 `switching` 取值
- [x] 1.3 定义 `CreateSessionRequest`（含 `newWorktree`、`attachments`、`agentConfig`、`permissionMode`、`clientRequestId`）、`CreateSessionResult`、`CreateSessionFailure`，删除 `QuickStart*`
- [x] 1.4 定义 `SessionInputRequest`/`SessionInputResult`、`SessionInterruptRequest`、`ViewChangeRequest`/`ViewChangeResult`、`ResumeRequest`、`SessionConflict`，删除 `ChatPromptRequest`/`ChatPromptResult`、`HybridSwitch*`、`HybridResumeRequest`、`HybridConflict`
- [x] 1.5 从 `ChatState` 删除 `side`、`phase`、`agent`、`agentSessionId`，`ChatMarker.side` 改为 `view`
- [x] 1.6 启动配置契约 `hybrid`/`hybridSupport`/`HybridSupportRequest` 改为 `chat`/`chatSupport`/`ChatSupportRequest`，`DetailsLaunchConfig.hybrid` 改为 `chat`
- [x] 1.7 更新 `internal/wire` 的全部转换，包括记录到已归档 `SessionInfo` 的转换（运行时字段取零值）
- [x] 1.8 用 `go test ./internal/api -run TestWireSnapshots -update` 重新录制快照，并逐一核对新增与删除的快照文件

## 2. 服务端

- [x] 2.1 `internal/workspace`：把快速开始编排并入会话创建，支持 `newWorktree`（含自动生成分支名）、首条提示、配置项与权限模式、按步骤失败与 10 分钟请求 ID 去重；删除 `quickstart.go` 中的独立入口
- [x] 2.2 `internal/session`：增加不依附 WebSocket 的带回执提交，以 `script` 标签取得控制，写入提交后依次写入命名按键
- [x] 2.3 `internal/hybrid`：按视图实现输入（聊天提示与队列）和中断（聊天取消、终端档案中断键）；普通会话的中断写入 Ctrl-C
- [x] 2.4 `internal/hybrid`：提供按状态列出与按 ID 读取已归档会话、删除记录（含对活动会话先关闭）的服务方法
- [x] 2.5 `internal/launch`：线上字段改名为 `chat`，存储层继续读写 `hybrid` 键与版本 2，补充回退兼容的读取测试
- [x] 2.6 `internal/api`：注册 `GET /api/sessions?state=`、`POST /api/sessions`（新请求与响应）、`/input`、`/interrupt`、`/view`、`/resume`、`DELETE ?forget=1`、`POST /api/uploads`、`POST /api/launch-configs/{id}/options`、`/api/launch-configs/chat-support`
- [x] 2.7 `internal/api`：删除 `/api/quick-starts*`、`/api/hybrid-sessions*`、`/switch`、`chat/prompts`、`chat/cancel` 路由与处理函数；`chat/*` 与 `/ws/s/{id}/chat` 对没有聊天视图的会话返回 409 `no_chat_view`
- [x] 2.8 `internal/api/errors.go`：统一映射 `SessionConflict` 原因码（`wrong_view`、`no_chat_view`、`not_active`、`not_archived`、`input_failed` 等）
- [x] 2.9 `internal/hub`：终端移动帧取值改为 `terminal`/`chat`，聊天状态帧去掉重复字段
- [x] 2.10 补充 Go 测试：输入与中断在两种视图与切换中的分派、错误视图的 409、已归档列表排序与零值字段、创建失败步骤与去重、`forget` 删除记录与临时文件夹

## 3. Web 端

- [ ] 3.1 `schemas.ts`、`chat-schemas.ts`：以新的 `SessionInfo` 字段取代 `SessionHybridSchema` 与 `HybridRecordSchema`，同步启动配置与冲突体
- [ ] 3.2 `chat-api.ts`、`api.ts`：发送改用 `/input`，取消改用 `/interrupt`，切换改用 `/view`，恢复使用 `view`
- [ ] 3.3 `quick-start.ts`、`QuickStart.svelte`：改用 `POST /api/sessions`、`/api/uploads` 与 `/api/launch-configs/{id}/options`，失败按步骤显示不变
- [ ] 3.4 `NewSessionDialog.svelte`：新建 worktree 改为随创建请求提交 `newWorktree`，重试时以 `workspace` 提交已建好的 worktree
- [ ] 3.5 `hooks/hybrid.svelte.ts`、`archived.ts`、`recoverable.ts`、`ChatHistory`、`ArchivedSessions`、`SessionList`、`AppSidebar`、`WorkbenchTree`、`TerminalHeader`、`SessionViewTabs`、`Settings`：改读 `view`、`transition`、`state`、`agent` 与 `chat`
- [ ] 3.6 更新受影响的 Vitest、浏览器与 Playwright 用例中的接口与字段，运行 `pnpm --dir web check:arch`

## 4. 文档与用户故事

- [x] 4.1 更新 `docs/protocol` 中会话与混合会话的接口说明，以及 `docs/hybrid-sessions.md` 中的接口
- [x] 4.2 更新 `docs/user-stories/hybrid-sessions.md` 中引用旧接口与字段的条目（快速开始的选项与上传接口、已归档记录、切换与恢复），以及 US-S03 的创建入口，同步覆盖索引
- [x] 4.3 运行 `make docs-check`

## 5. 验证

- [x] 5.1 与 `unify-session-cli` 一起运行 `make check` 与 `make test`
- [ ] 5.2 用真实服务进程与 `curl` 冒烟：普通会话与聊天视图会话的 `/input`、`/interrupt`、`/view`，`?state=archived` 列表、`/resume`、`?forget=1`，以及带 `newWorktree` 与 `prompt` 的创建
- [ ] 5.3 在浏览器中确认快速开始、新建会话、聊天发送与取消、切换、归档与恢复的可见行为与改动前一致
