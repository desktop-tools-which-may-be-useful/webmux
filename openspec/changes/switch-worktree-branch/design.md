## Context

- 现在只有 `worktree add` 会检出分支，系统里没有 `checkout`/`switch`。
- 删除与释放已经有成熟的两阶段框架，切换直接复用以下部分：
  - `removalStateFromTree` 中的阻止条件模式；
  - `targetSessions(path)`：收集目标路径及其子目录下的会话；
  - `confirm` / `takeConfirmation`：按用途（`purpose`）绑定、5 分钟有效、一次性消费的确认凭证；
  - 仓库门闩 `repositoryState.gate`；
  - 写操作之后用 `context.Background()` 重读状态。
- 会话与工作树按路径关联（`session.Workspace`、`hybrid.WorkspaceRef` 只保存 ID 和路径，不保存分支），工作树 ID 由“仓库 + 路径”生成。原地切换后，关联、主页置顶与归档记录的归位都不受影响。
- 本机 Git 2.47 实测结果：
  - `git switch --no-guess --end-of-options <name>` 能正确拒绝 `-x` 这类名称；
  - 切到已被其他工作树检出的分支时，Git 自身会报错；
  - `--end-of-options` 从 Git 2.24 开始支持，低于当前隐含的最低版本 2.31。
- 依赖 `enrich-repository-details` 提供的 `changes` 计数、仓库面板“分支”分区，以及共享的 `WorktreeMenuItems`。

## Goals / Non-Goals

**Goals:**

- 切换要么完成，要么不改变工作区；不完整的状态只可能来自 Git 自身的失败，并在刷新后如实展示。
- 与删除共用同一套会话归属规则和确认凭证机制，不另起一套约定。

**Non-Goals:**

- 不做 stash 管理、不携带修改切换、不做 merge/rebase、不更新子模块。
- 新建工作树流程不设置上游，保持现有规格。
- 不为“切换”增加浏览器之间的实时推送；其他客户端按现有刷新规则看到结果。

## Decisions

### 1. API 形状：沿用两阶段

- `POST /api/repositories/{id}/worktrees/{wid}/switch-preview`
  - 请求：`{target: {kind: "local"|"remote"|"new"|"detach", branch?, ref?, name?, startPoint?, commit?}}`
  - 返回 `SwitchPreview`，包含：
    - `confirmationId`、`expiresAt`；
    - `worktree`（当前状态）；
    - `target`：`{kind, branch, commit, subject, createsBranch, upstream}`；
    - `sessions`：`[]SessionInfo`；
    - `blockers`：`[{code, message, path?}]`；
    - `warnings`：`[{code, message}]`。
- `POST /api/repositories/{id}/worktrees/{wid}/switch`
  - 请求：`{confirmationId, acknowledgeSessions}`
  - 返回刷新后的 `Worktree`。

有阻止条件时，预览仍然返回 200，但不签发凭证（`confirmationId` 为空），前端据此禁用按钮并展示原因。执行阶段新出现的阻止条件返回 409 `ErrConflict`。

- 备选方案：一步完成的 `POST /switch`，只在有会话时要求预览。这样两条路径的竞态语义不一致，也无法稳定绑定目标提交，被否决。

### 2. argv 与目标解析

预览阶段把目标解析成精确的 OID。远程目标在预览时就算出本地分支名。名称都先经过 `check-ref-format --branch`，并拒绝以 `-` 开头的名称。

| 目标 | argv（`Service.git` 自动加 `-C <path>`） |
|---|---|
| local | `switch --no-guess --end-of-options <branch>` |
| remote | `switch --no-guess --track --create <name> --end-of-options refs/remotes/<ref>` |
| new | `switch --no-guess --no-track --create <name> --end-of-options <startOID>` |
| detach | `switch --detach --end-of-options <commitOID>` |

禁止使用 `--force`、`--discard-changes`、`--merge`、`--orphan`、`--ignore-other-worktrees`。

远程目标使用完整的 `refs/remotes/...`，避免与同名本地分支或标签歧义，并让 `--track` 推导出正确的 `branch.<name>.remote/merge`。

### 3. 阻止条件与检测

- 已跟踪修改：使用 `enrich-repository-details` 的 porcelain v2 计数，`staged + unstaged + conflicted > 0` 即阻止。
- 进行中的操作：通过一次 `rev-parse --git-path` 取得 `MERGE_HEAD`、`CHERRY_PICK_HEAD`、`REVERT_HEAD`、`rebase-merge`、`rebase-apply`、`BISECT_LOG` 的路径，再用 `os.Lstat` 判断是否存在。
  - Git 返回的可能是相对于工作树目录的路径（实测主工作区输出 `.git/MERGE_HEAD`），需要先与工作树路径拼接；
  - 路径按工作树解析，各工作树互不干扰。
- 分支占用：`worktree list` 中其他条目的 `branch` 与目标相同即阻止，并在 `blocker.path` 中返回占用者的路径，供前端提供“转到该工作树”。
- 同一目标：本地目标与当前分支相同，或 detach 目标与当前 HEAD 相同且当前已是 detached。
- 警告（不阻止）：
  - 存在未跟踪文件；
  - 工作树处于 locked 状态（锁只限制 prune/move/remove，不限制切换，但锁的原因值得提示）；
  - 存在 `.gitmodules`（子模块不会自动更新）；
  - 有运行会话。
- 被未跟踪文件阻挡：识别 Git 的 `would be overwritten by checkout` 错误输出，取出路径列表（最多 50 条），以 `ErrConflict` 返回。

### 4. 会话确认与竞态

凭证用途为 `purposeSwitch`，绑定以下内容：

- 工作树 ID 与路径；
- 预览时的 `branch` 和 `head`；
- 目标解析出的 OID；本地目标同时记录分支尖端；
- `targetSessions` 中未退出会话的 ID 集合（排序后保存）。

执行步骤：

1. 获取仓库门闩；
2. 重新读取单个工作树（`currentTree`）及其状态；
3. 逐项比对绑定内容和阻止条件，有任何变化即返回 409，并附带“请重新预览”；
4. 会话集合非空时，要求 `acknowledgeSessions == true`；
5. 执行 `git switch`。

切换不调用 `reserve`，因为切换不会删除目录，期间启动新会话不会落到已删除的路径。新会话的出现由第 3 步的集合比对发现。

### 5. 执行时限与取消

`git switch` 被中途杀掉会留下不一致的索引和工作区，因此执行使用 `context.WithoutCancel(ctx)`，另设独立上限 `switchTimeout = 3min`，客户端断开不会中止 Git。超时后报告 504，并用 `context.Background()` 重读状态返回实际情况。这和创建失败的规则一致：以实际 Git 状态为准，不做隐式回滚。

- 备选方案：沿用 30s 的 `writeTimeout`。大仓库切换时更可能被截断，留下残缺状态，被否决。

### 6. 前端

`SwitchBranchDialog.svelte` 基于 `ResponsiveDialog`，结构如下：

- 顶部用 `SegmentedControl`（`size="form" wrap`）选择三种模式：已有分支 / 新建分支 / 分离到提交；
- 已有分支模式复用 `PickerField` 加 `BranchCommandList`，分“本地”“远程”两组，当前分支和已被占用的分支标记后禁用；
- 新建分支模式复用 `branch-name.ts` 的校验和默认名生成，起点选择沿用 `WorktreeCreateFields` 的起始分支选择；
- 分离模式使用 `/commits` 列表。

选择变化后自动请求预览，用 `LatestRequest` 丢弃过期的响应。预览区依次展示“当前 → 目标”、阻止原因、警告和会话列表。有会话时显示确认复选框，与批量强制删除授权的模式一致。

按钮的安排：

- 主按钮“切换”使用 `default` 样式；
- 受阻（修改或会话）时，右下角提供 `success` 样式的“改为在新工作树打开”；
- 分支被占用时，提供“转到该工作树”。

“在新工作树打开”通过 `shell.start({kind: "new-worktree", repositoryId, preset})` 打开现有弹窗。`NewSessionDialog` / `WorktreeCreateFields` 新增 `preset = {mode, branch?, startBranch?, name?}`，只用于设置初始值，后续交互与现有流程相同。

切换成功后调用 `repositories.refresh(true)` 并关闭弹窗。当前首页正收窄在该工作树时保持收窄（ID 不变），标题随新分支更新。

### 7. CLI

在 `worktree.go` 中新增 `switch` 子命令，参数按第 1 节映射成 `target`。确认流程复用 `worktree rm` 的终端判定与 `--yes` 约定。

## Risks / Trade-offs

- [风险] agent 正在写文件时切换分支，会让它的后续写入落在新分支上 → 预览中对“工作中”状态的会话单独高亮并要求确认。系统不替用户做决定，也不中断会话，这与“会话关闭不会自行触发 worktree 删除”的分离原则一致。
- [风险] 代码视图的快照缓存（`internal/codeview`，`Lifetime=10min`）可能在切换后仍展示旧内容 → 实现时核查快照键：如果只按路径缓存，切换成功后由前端让该工作树下已打开的代码视图重新读取；如果键中包含内容标识，则不需要处理。
- [风险] `--track` 依赖 `remote.<name>.fetch` 映射；映射不规范时可能推导不出上游 → 执行后读取 `branch.<name>.merge`，未设置时在结果中给出警告，不回滚。
- [取舍] 不支持携带修改切换，而这恰好是 Git 默认允许的行为 → 这是保守选择。用户可以在终端自行切换，WebMux 刷新后如实展示结果；需要并行处理时可改用“在新工作树打开”。

## Migration Plan

先归档 `enrich-repository-details`，再实现本变更。没有持久化结构变化；回滚只需还原代码，未使用的 `purposeSwitch` 凭证会随服务重启清空。
