## Context

现状（见 proposal.md 的 Why）：

- 已归档记录是 `hybrid.Record`，持久化在 `<state_dir>/hybrid-sessions.json`。它只保存 `Cwd` 和 `Workspace{repositoryId, worktreeId}`，不记录提交。
- 恢复（`hybrid.Service.resume`）直接在 `rec.Cwd` 调用 `CreateShell`，不涉及 Git。
- `workspace.Service` 已经具备以下能力：
  - 删除预览与确认：`PreviewRemoval` / `removeWorktree`，带限时 `confirmations`、文件身份校验 `identify`、子模块与嵌套仓库检查 `unsupportedContents`、路径预留和仓库锁；
  - 受限的 Git 执行器 `Service.git`；
  - 依赖 `hybrids`，用于 `targetRecords` 和 `forgetRecords`。
- 现有删除流程删除工作树后，会通过 `forgetRecords` 删除记录。释放必须走一条不删记录的独立路径。
- 已验证：工作树中只有被忽略的 `node_modules/` 时，非强制的 `git worktree remove` 能成功删除目录。所以释放无需 `--force`，未提交修改和未跟踪文件仍由 Git 兜底拒绝。
- `workspace.CreateRequest` 只支持 `new` 和 `existing` 两种模式，没有 detached 检出。

## Goals / Non-Goals

**Goals:**

- 由 `workspace.Service` 统一负责释放与重建的 Git 操作和并发协调。`hybrid` 只负责记录字段，以及在记录已释放时拒绝恢复。
- 提交在释放期间始终可达，不依赖分支是否存在。
- 释放与重建都使用“预览 → 带确认标识执行 → 重新校验”的流程，和删除保持一致。

**Non-Goals:**

- CLI 子命令、批量释放整个仓库、自动按时间释放。
- 恢复被删除的忽略文件（依赖、`.env`），由用户自行重新生成。
- 强制释放；用户需要丢弃修改时，走现有的强制删除。

## Decisions

### D1 数据模型：记录上的 `Released`

`hybrid.Record` 新增可选字段 `Released *Release`，内容如下：

```go
type Release struct {
    ID           string    // 一次释放共享的标识
    RepositoryID string
    Path         string    // 原规范化路径
    Commit       string    // 完整 OID
    Branch       string    // 空表示 detached
    ReleasedAt   time.Time
}
```

`hybrid.Service` 新增两个方法：

- `MarkReleased(ctx, ids, Release)`：立即持久化；
- `ClearReleased(ctx, releaseID)`：清除该次释放下所有记录的标记，返回受影响的 ID。

`resume` 在 `Released != nil` 时返回 `hybrid.ErrConflict`，并说明需要先重建工作树。字段可选，旧文件无需迁移。

wire 层的 `contract.HybridRecord` 增加 `released?: {id, path, commit, branch?, releasedAt}`，`chatSchemas.ts` 同步修改。

不考虑另建 `released-worktrees.json` 状态文件：它会多出一个持久化面，并且需要与记录做一致性维护。释放信息天然属于记录，删除记录时会一起消失。

### D2 用私有引用保住提交

释放前执行：

```
git update-ref refs/webmux/released/<releaseID> <commit>
```

引用写在仓库共享 refs 下，所有 worktree 都能看到。

- 只靠分支不够：分支可能被删除或强推，detached 工作树本身也没有分支。
- 不用 tag：tag 会出现在用户的 `git tag` 输出里，并可能被 `--tags` 推送。`refs/webmux/*` 默认不会被 fetch 或 push。

### D3 释放顺序与回滚

在仓库锁和路径预留下，按以下顺序执行：

1. 用确认标识重新取得 `removalState` 并比较 `sameTarget`、`sameSessions`。
2. 执行 `update-ref` 建立私有引用。
3. 调用 `MarkReleased`。
4. 执行 `git worktree remove -- <path>`（非强制）。
5. 刷新仓库。

第 4 步失败时，先重新读取仓库：工作树仍登记在原路径，才依次执行 `ClearReleased` 和 `update-ref -d`。Git 的 `worktree remove` 过了检查后即使目录内某个文件删不掉也会继续并移除登记，这时保留标记与引用，保证提交可达；读取失败同样按“已移除”处理。

先标记再删除的理由：标记后，即使删除未发生，状态仍然安全，因为恢复会被拒绝，而重建预览检测到原路径已是保存提交上的 worktree 时会直接复用。反过来先删除再标记，一旦标记写入失败，就会出现“工作树已删除、记录却不知道提交”的不可恢复状态。

### D4 阻止条件复用 `removalState`

释放与删除共享阻止规则，唯一区别是忽略文件不阻止释放。实现方式：

- 把 `removalState` 中的阻止项拆成带类型的 `blocker{kind, message}`；
- 释放时过滤掉 `ignored` 类型，并额外检查 HEAD 是否可以解析为提交；
- 删除预览的线上文案保持不变。

`confirmation` 增加 `purpose`（`remove` 或 `release`）字段，确认标识不能跨用途使用。

### D5 忽略项清单与大小

- 用 `git status --porcelain=v1 -z --ignored=traditional --untracked-files=normal` 取顶层的忽略条目（目录会被折叠）。
- 计算大小时用 `lstat` 遍历，不跟随符号链接。总时限 2 秒，条目上限 200,000。
- 超出任一限制时，`sizeComplete=false`，界面显示“未知”或“至少 X”。
- 清单最多返回 50 项，其余以计数说明。

### D6 重建

`hybrid` 负责记录，`workspace` 负责 Git，二者分开实现：

- **重建预览**：根据记录取得 `Release`，再找到仓库（未登记或不可用时为阻止项），然后：
  1. 原路径存在：
     - 是该仓库的 linked worktree 且 HEAD 等于 `Commit` 时，mode 为 `reuse`；
     - 否则加入阻止项。
  2. 原路径不存在：
     - `Branch` 存在、未被其他 worktree 检出、且尖端等于 `Commit` 时，mode 为 `branch`；
     - 否则 mode 为 `detached`，附带原因 `branchMissing` / `branchMoved` / `branchCheckedOut`。
- **执行重建**：请求携带 `releaseId`、`mode`、`commit`。服务重新计算，mode 不一致时返回 `workspace.ErrConflict`。
  - `branch`：执行 `git worktree add -- <path> <branch>`；
  - `detached`：执行 `git worktree add --detach -- <path> <commit>`；
  - `reuse`：不执行 Git 操作。

  之后调用 `ClearReleased` 并删除私有引用。引用删除失败只记录日志，不影响结果，由 D8 的清扫兜底。

- detached 模式在 `workspace` 内部新增一个 `addDetached` 辅助函数，不扩展公开的 `CreateRequest` 模式，避免影响新建工作树对话框的契约。

### D7 接口

| 方法 | 路径                                                      | 说明                                                                                                                                                      |
| ---- | --------------------------------------------------------- | --------------------------------------------------------------------------------------------------------------------------------------------------------- |
| POST | `/api/repositories/{repo}/worktrees/{wt}/release-preview` | 返回 `ReleasePreview{confirmationId, expiresAt, worktree, commit, archived, ignored[{path, bytes?}], ignoredMore, ignoredBytes?, sizeComplete, blockers}` |
| POST | `/api/repositories/{repo}/worktrees/{wt}/release`         | 请求体 `{confirmationId}`，返回 `{releaseId, recordIds}`                                                                                                  |
| POST | `/api/hybrid-sessions/{id}/rebuild-preview`               | 返回 `{releaseId, path, commit, branch?, mode, reason?, blockers}`                                                                                        |
| POST | `/api/hybrid-sessions/{id}/rebuild`                       | 请求体 `{releaseId, mode, commit}`，返回重建后的 worktree                                                                                                 |

- 这些接口由 `internal/api` 中的 workspace handler 调用 `workspace.Service`。错误沿用 `workspace` 的哨兵错误，在 `errors.go` 中映射。
- 前端的恢复流程依次调用：重建预览 → 确认 → 重建 → `resumeHybrid`。这里不合并成一个服务端接口，以免控制器层编排两个服务；重建成功、恢复失败时，按 spec 保留工作树。

### D8 引用清理

- `DELETE /api/hybrid-sessions/{id}` 删除已释放的记录后，api handler 调用 `workspace.DropRelease(ctx, release)`。若已没有记录引用该 `releaseID`，就删除私有引用。
- 记录还可能因 `MaxRecords` 淘汰而丢失，因此 `workspace` 在每次释放或重建时，以及记录成功加载后的首次仓库刷新时，清扫该仓库中无记录引用的 `refs/webmux/released/*`。
- 记录加载失败时不清扫。

### D9 前端

- `SidebarSessionMenu` 的 archived target 增加 `onrelease?`。
- `ArchivedSessions` 菜单增加同名项；是否显示由 `archived.ts` 中的 `releasableRecordWorktree` 判断（复用 `removableRecordWorktree` 的条件，并要求 `!record.released`）。
- 新增 `ReleaseWorktreeDialog.svelte` 和 `RebuildWorktreeDialog.svelte`，基于 `ConfirmDialog` 和现有 ui 组件，按 `webmux-ui` 技能实现。
- `AppSidebar.resume` 和 `ArchivedSessions` 的恢复入口遇到 `record.released` 时，先打开重建对话框。
- “已释放空间”标记使用现有 Badge 的弱化变体，并配 tabler 图标。

## Risks / Trade-offs

- [忽略文件中的 `.env` 等不可再生文件被删除] → 预览逐项列出顶层忽略项，确认文案明确说明重建无法找回这些文件。
- [私有引用泄漏，导致对象无法被 gc 回收] → 依靠 D8 的删除与清扫；引用只锁定提交本身，不保留工作树中的文件，体积可以忽略。
- [释放过程中外部进程写入目录] → 非强制的 `git worktree remove` 会拒绝存在未跟踪或修改文件的目录，失败后按 D3 回滚。
- [原路径被其他工具占用，无法重建] → 显示阻止原因。用户清理后重试；本次不提供换路径重建。
- [仓库在释放后被“从列表移除”] → 重建被阻止。记录仍可删除；重新登记仓库后可以继续重建，因为 `Release` 保存的是路径和提交，不依赖仓库 ID 的连续性。

## Migration Plan

- 新字段为可选字段。`internal/statefile` 和 `internal/hybrid` 解码时不使用 `DisallowUnknownFields`，旧版本读到新文件会忽略该字段。
- 回滚到旧版本后，已释放的记录会表现为“worktree 已删除、无法恢复”，私有引用残留。可以用 `git update-ref -d` 手动清理。
