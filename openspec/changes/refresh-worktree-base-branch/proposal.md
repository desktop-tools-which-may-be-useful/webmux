## Why

#268 指出，创建 worktree 时固定使用主工作区本地 HEAD，可能在主分支落后远端时从旧提交创建分支。用户需要明确选择起始分支，并在不阻塞创建操作的前提下获取其上游最新提交作为默认基线。

## What Changes

- “创建新分支”模式增加起始分支选择；初始默认分支和用户后续选择的分支都会触发一次异步上游 fetch，并立即展示该本地分支的当前提交作为可用基线。
- fetch 成功后，在分支选择仍一致且用户未在请求期间手动修改基线时，将默认起始提交更新为所选分支上游的最新提交；界面明确区分本地与远程基线。
- fetch 期间显示进度动画，但不禁用分支、提交、路径或创建操作；提供手动“刷新上游”按钮。用户可基于当前可见提交继续创建，不必等待网络完成。
- 未配置上游、fetch 失败或超时均不阻断创建：保留本地分支尖端并显示非阻断说明，不隐式选择其他 remote，也不把失败伪装成最新远程状态。
- 后端新增按指定本地分支刷新其已配置上游的接口；现有分支查询保持只读、无网络语义。fetch 不切换工作区分支、不重置本地分支、不修改用户已输入的新分支名称或目标路径。
- 防止过期异步响应覆盖新分支选择或用户手动输入；关闭创建面板后取消或忽略未完成刷新。

## Capabilities

### New Capabilities

无。

### Modified Capabilities

- `worktree-lifecycle`：创建新分支时由固定本地基线扩展为可选起始分支、非阻断上游刷新、远程最新默认基线及本地回退；调整原先“不自动 fetch”的要求。

## Impact

- 后端：`internal/workspace` 的分支与默认基线查询、Git 上游解析和有超时的 fetch；`internal/api` 的仓库分支刷新路由及错误映射。
- 契约：`internal/contract`、`internal/wire`、`web/src/schemas.ts`、API 客户端与 wire 快照。
- 前端：`WorktreeCreateFields.svelte` 及相关状态/浏览器测试，覆盖起始分支选择、异步状态、手动刷新、回退和过期响应。
- 文档：`docs/usage.md`、`docs/protocol/http-workspace.md`、`docs/user-stories/code-and-worktrees.md` 的 US-J04 / US-J05。
- 不涉及：已有本地分支创建模式、worktree 删除语义、自动 rebase/pull/push、远程凭据管理或定时后台 fetch。
