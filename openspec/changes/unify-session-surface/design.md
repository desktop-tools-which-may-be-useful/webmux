## Context

动机见 proposal.md。与设计相关的现状：

- `session.Manager` 持有全部运行中的会话外壳；混合会话是同一个外壳加上实现 `session.Driver` 的 `hybrid.actor`，ID 相同。
  聊天端时，`actor.overlay()` 把回合、请求与预览映射到会话的 `status`、`attention`、`preview` 与标题，因此 `webmux wait` 与提醒推送已经对两端通用。
- `hybrid.records`（`hybrid-sessions.json`）只为混合会话保存记录；会话关闭或中断后只存在于记录中，经 `/api/hybrid-sessions` 读取。
- 终端输入只有 WebSocket 一条路：`Session.Submit` 需要一个订阅者，带回执，并让该订阅者取得控制。`webmux send` 在客户端判断所在端后拨 `/ws/s/{id}`。
- 快速开始的编排在 `workspace/quickstart.go`：预检、新建 worktree、创建会话、应用配置、发送首条提示，带 10 分钟的请求 ID 去重与按步骤的失败体。
- `contract.HybridConflict` 已被聊天、切换、恢复与快速开始共用；原因码是稳定的。

约束：只允许 `cmd/` 构造服务；JSON 形状只在 `internal/contract` 定义、经 `internal/wire` 转换；Origin 与认证检查不变；PTY 数据保持二进制。

## Goals / Non-Goals

**Goals:**

- 对外只有一个会话名词和一套作用于会话的动词，视图差异由服务端分派，用错视图时给出统一的 409。
- 一个会话只有一种形状（`SessionInfo`），在运行中与已归档两种状态下都适用。
- 线上命名与 CLI、界面一致：`terminal`/`chat`、`view`、`SessionConflict`。
- 一次性切换，删除旧接口、旧命令、旧字段及其测试。

**Non-Goals:**

- 不给普通会话增加持久化或恢复；“服务重启即结束”的定位不变。已归档列表因此只会出现带聊天视图的会话。
- 不改变 Web 端的可见行为、交互和文案（包括“混合会话”“聊天模式”等概念名）。
- 不改变切换状态机、单端占用约束、队列、授权与表单的语义。
- 不重命名内部 Go 包（`internal/hybrid` 等）与持久化文件或键（`hybrid-sessions.json`、`launch-configs.json` 的 `hybrid`）。

## Decisions

### D1 视图是能力，`state` 是所有会话共有的生命周期

`SessionInfo` 新增 `state`、`views`、`view`、`transition`、`agent`、`archive`，删除 `hybrid`。

- 普通会话：`state` 恒为 `active`（已退出的普通会话仍是 `active`，`status` 为 `exited`），`views=["terminal"]`，`view="terminal"`，`agent=null`。
- 带聊天视图的会话：`views=["terminal","chat"]`；`view` 是最近一次落定的视图，切换期间仍是来源视图，`transition={to, phase}` 表示进行中；
  `agent={name, title, icon, sessionId, work}`。
- `closed`/`interrupted` 的会话：运行时字段取零值（`generation=""`、`status="exited"`、`exitCode=null`、`clients=0`、尺寸 0、`listeningPorts=[]`），
  `lastActiveAt` 取记录的更新时间，`archive={configMissing, previousAgentSessionIds, activity, queuedCount, interruptedAt, updatedAt}`。

备选：保留 `hybrid` 只改名为 `agent`。否决：`side`、`state` 依旧只属于一种会话，客户端仍要按“有没有这个对象”分支。

备选：`view` 在切换期间取 `null`。否决：客户端在切换中仍需要知道从哪一端来（回滚、进度文案），`transition.to` 与 `view` 一起即可表达。

### D2 列表与读取：一个接口、按状态筛选，WebSocket 语义不变

`GET /api/sessions?state=active|closed|interrupted|archived|all`，缺省 `active`，`archived` 等于 `closed`+`interrupted`。
`active` 部分的排序不变；已归档部分按 `archive.updatedAt` 倒序；`all` 先列 `active` 再列已归档。`GET /api/sessions/{id}` 对已归档的 ID 也返回会话信息。

`/ws/sessions` 仍只描述 `active` 会话：会话被关闭或中断而离开运行中列表时发送 `removed`，已归档列表由客户端按需读取。
这与现在 Web 端在 `removed` 后刷新可恢复记录的做法一致，避免把最多 200 条记录塞进每次快照。

实现：`internal/api` 合并 `Manager.List()` 与 `hybrid.Service` 的记录列表，由 `internal/wire` 把记录转换为 `SessionInfo`。

### D3 创建：快速开始并入 `POST /api/sessions`

请求体：

```
{clientRequestId?, name?, labels?,
 cwd? | workspace? | newWorktree?{repositoryId, mode, branch?, base?, path?},
 configId? | cmd?+env?,
 view?, prompt?, attachments?, agentConfig?, permissionMode?, useInitInstruction?}
```

- 位置三选一，命令二选一，规则同现在。`newWorktree.mode` 取 `new`/`existing`；`mode=new` 且省略 `branch` 时按快速开始的规则自动生成不冲突的分支名与路径。
- `view` 缺省 `terminal`；`chat` 只对带聊天视图的启动配置有效。`attachments`、`agentConfig`、`permissionMode` 只在 `view=chat` 时有效，
  否则 400；`attachments` 随首条提示发送，要求 `prompt` 非空（与现在的快速开始一致）。
- 响应 201：`{session, worktree, configWarnings}`；`worktree` 只在本次新建时出现。
- 失败体 `{error, step, reason?, eligibility?, connector?, worktree?, sessionId?}`，`step` 取 `precheck`、`worktree`、`session`、`config`、`prompt`，
  不回滚。单步请求（不含 `newWorktree`、`prompt`）失败时 `step` 为 `precheck` 或 `session`，HTTP 状态码映射不变。
- `clientRequestId` 的去重窗口沿用快速开始的 10 分钟；缺省时不去重。

编排从 `workspace/quickstart.go` 迁到 `workspace` 的创建入口，旧的 `CreateSession` 与 `QuickStart` 合并为一个函数。
新建会话对话框与 `webmux run --new-worktree` 也改用 `newWorktree`，失败重试时把已建好的 worktree 作为 `workspace` 传入，满足“只启动会话、不重复创建”。

上传暂存改为 `POST /api/uploads`，供尚无会话 ID 的创建使用；会话内上传仍是 `POST /api/sessions/{id}/upload`。
选项探测改为 `POST /api/launch-configs/{id}/options {repositoryId}`，挂在启动配置下，因为它描述的是配置的 agent。

备选：保留 `/api/quick-starts` 作为调用 `POST /api/sessions` 的薄封装。否决：两个入口、两种失败体，正是要消除的分叉。

### D4 输入与中断由服务端分派

`POST /api/sessions/{id}/input`：

```
{clientRequestId, text, submit?, keys?, attachments?, mode?}
-> 200 {view, queued?, itemId?, turnId?, promptId?}
```

- 终端视图：`text` 与 `submit`（缺省 `false`，`true` 时追加档案或终端的提交键）作为一次带回执的提交写入 PTY，随后依次写入 `keys`
  （`backspace`、`ctrl-c`、`ctrl-d`、`down`、`enter`、`esc`、`left`、`right`、`tab`、`up`）。服务端在回执为 `written` 后答复；
  `partial`、`failed`、`missing` 以 409 `input_failed` 答复并带回执状态。`attachments` 或 `mode` 返回 409 `wrong_view`。
- 聊天视图：`text` 与 `attachments` 作为提示，`mode` 取 `auto`/`steer`，语义同现在的 `chat/prompts`；`submit` 被忽略（提示总是提交）；`keys` 返回 409 `wrong_view`。
- 切换进行中返回 409 `switching`；`state` 不是 `active` 返回 409 `not_active`。

终端写入复用 `Session.Submit` 的回执与去重，但需要一个不依附 WebSocket 的写入者。`internal/session` 增加以服务端客户端身份提交的入口：
它以 `script`（调用者在会话内时附上来源会话 ID）为控制者标签取得控制，与现在 `webmux send` 经 WebSocket 时的可见结果相同。

`POST /api/sessions/{id}/interrupt {clientRequestId}`：聊天视图等同现在的 `chat/cancel`（空闲时无操作）；终端视图有档案时写入档案中断键，否则写入 `\x03`。

备选：保留 `chat/prompts`，只在 CLI 分派。否决：分派逻辑会在 CLI、Web 与其他客户端里各写一份，服务端是唯一知道当前视图且无竞态的地方。

### D5 视图切换、恢复与结束

- `POST /api/sessions/{id}/view {clientRequestId, to, choice?, agentSessionId?}` 取代 `/switch`，结果 `{view, draft, dropped, unsupported}`。
- `POST /api/sessions/{id}/resume {clientRequestId, view?, agentSessionId?, confirmWithoutEnv?}`，只对 `closed`/`interrupted` 有效；`active` 会话（含所有普通会话）为 409 `not_archived`。
- `DELETE /api/sessions/{id}`：停止当前视图的进程。普通会话留在列表中显示为已退出；带聊天视图的会话变为 `closed` 并离开运行中列表（语义不变）。
- `DELETE /api/sessions/{id}?remove=1`：停止并从运行中列表移除；带聊天视图的会话保留归档记录（语义不变）。
- `DELETE /api/sessions/{id}?forget=1`：隐含 `remove`，并删除归档记录与临时文件夹；对 `active` 的带聊天视图会话先关闭再删除。
  这比现在的 `DELETE /api/hybrid-sessions/{id}`（对活动会话 409）更宽，CLI 用 `--yes` 确认保护。

### D6 统一冲突体与命名

`SessionConflict{error, reason, eligibility?, candidates?, connector?, submission?}` 用于所有会话命令的 409。原因码保持稳定，改名 `wrong_side` → `wrong_view`，
新增 `no_chat_view`（对没有聊天视图的会话调用 `chat/*` 或视图切换）、`not_active`、`not_archived`、`input_failed`。

取值改名：`tui` → `terminal`，`acp` → `chat`；字段 `side`/`startSide` → `view`。`HybridPhase` 的取值不变，改名为 `TransitionPhase`。
启动配置 `hybrid`/`hybridSupport` → `chat`/`chatSupport`；持久化键不变，由 `internal/launch` 的存储层映射。

### D7 聊天通道只承载对话

`/ws/s/{id}/chat` 与 `/api/sessions/{id}/chat/{requests,queue,config,permission-mode}` 保留。`ChatState` 删除 `side`、`phase`、`agent`、`agentSessionId`；
需要这些信息的客户端读取 `SessionInfo`（Web 端已经从 `/ws/sessions` 与终端帧拿到）。`ChatMarker.side` 改名 `view`。

终端 WebSocket 的移动关闭码 4409 与移动帧保留，帧中的取值改为 `chat`/`terminal`。

### D8 CLI 在姊妹变更中收敛

shell 命令的动词与选项见同一 PR 中的 `unify-session-cli`。为满足文档治理对单个变更规格目录数的上限，CLI 的规格增量放在那个变更中；两者必须一起实现与发布。

### D9 Web 端迁移

Web 端只换接口与字段，不改界面：`schemas.ts` 删除 `SessionHybridSchema` 与 `HybridRecordSchema`，以 `SessionInfoSchema` 的新字段与 `archive` 取代；
`useHybrid` 从 `info.view`/`info.transition` 推导 `view`；已归档列表、聊天历史与恢复改读 `GET /api/sessions?state=archived`；
聊天发送与取消改用 `input`/`interrupt`；快速开始改用 `POST /api/sessions`，上传改用 `/api/uploads`，选项改用 `/api/launch-configs/{id}/options`。

## Risks / Trade-offs

- [旧 CLI、旧脚本与 agent 提示中的命令失效] → 同一二进制内 CLI 与服务端同版本；`agentguide` 随版本更新；发布说明列出命令对照表。
- [Windows 控制台客户端与托盘读取到新字段形状] → 它们只展示 ID、名称、状态、命令与目录，这些字段不变；在 `windows-tray-server-compat` 的验证中覆盖一次。
- [`DELETE ?forget=1` 对活动会话的语义比旧接口宽] → CLI 非交互时要求 `--yes`；Web 端只对已归档会话提供“删除记录”。
- [已归档会话的运行时字段取零值，客户端误把它当作已退出的普通会话] → 客户端以 `state` 区分，`archive` 只在已归档时出现；wire 快照覆盖两种形状。
- [服务端终端输入取得控制，可能打断浏览器中的输入] → 与现在 `webmux send` 的结果相同，控制指示器显示来源，不新增行为。
- [变更面大，wire 快照、前端 schema 与 CLI 必须同步] → 按 tasks.md 的顺序：先契约与 wire 快照，再服务端，再 CLI，最后 Web 端；每一步都跑对应的契约测试。

## Migration Plan

1. 契约、wire 与快照一次性切到新形状；服务端同时删除旧路由。
2. CLI 与 `agentguide` 在同一 PR 中按 `unify-session-cli` 切换。
3. Web 端在同一 PR 中迁移；`make test` 通过后发布。
4. 回滚：回退到旧版本即可，持久化文件的格式与键没有变化。
