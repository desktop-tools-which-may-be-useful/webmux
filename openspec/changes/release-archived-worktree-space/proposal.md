## Why

已归档聊天所在的 linked worktree 常常带着 `node_modules`、构建产物等大体积忽略文件。用户现在只有两个选择：保留 worktree，磁盘一直被占用；或者走“删除记录”，把 worktree 和已归档记录一起删掉，之后无法恢复。需要一个中间选项：释放磁盘空间，同时保留恢复聊天的能力。

## What Changes

- 已归档记录的更多菜单（聊天侧栏“已归档”分组，以及首页工作树的“已归档”区）新增“释放空间”。只对仍存在、可安全清理的 linked worktree 显示。
- 释放前先预览，列出阻止原因、受影响的已归档记录、将被删除的顶层忽略项及估算大小。未提交修改、未跟踪文件、运行中会话、锁定、主工作区、子模块或嵌套仓库时，系统拒绝释放。忽略文件**不**阻止释放，会随目录一起删除。
- 释放成功后：
  - worktree 目录与 Git 登记被删除，分支保留；
  - 该目录下的已归档记录**保留**，并记下释放时的提交 OID、分支名和原路径；
  - 系统建立一个 WebMux 私有引用，让该提交不被 `git gc` 回收。
- 已释放的记录显示“已释放空间”标记。点“恢复聊天”或“恢复到终端”时，先弹出二次确认，说明将在原路径重建 worktree，以及是检出原分支还是以 detached 方式检出保存的提交。确认后先重建 worktree，再恢复会话。取消则不做任何改动。
- 同一 worktree 的所有已释放记录共享一次释放。重建后它们都回到普通已归档状态，私有引用随之删除；最后一条引用该释放的记录被删除时，私有引用也被删除。
- 后端新增释放预览、释放、重建预览、重建接口；混合会话记录增加释放信息字段（持久化与 wire 契约）。

## Capabilities

### New Capabilities

- `archived-worktree-release`：为已归档聊天释放 linked worktree 的磁盘空间，保留提交以便之后在原路径重建，并在恢复聊天时二次确认重建。

### Modified Capabilities

- `chat-sidebar`：打开已归档聊天时，已释放空间的记录先确认重建 worktree 再恢复；已归档菜单增加“释放空间”。
- `workspace-overview`：已归档记录的更多菜单增加“释放空间”，并展示已释放状态；恢复已释放记录时需要确认重建。

## Impact

- 后端：
  - `internal/workspace`：释放、重建与私有引用管理，复用删除预览的确认机制与身份校验；
  - `internal/hybrid`：记录增加释放信息，且尚未重建时拒绝直接恢复；
  - `internal/api` 路由与错误映射；
  - `internal/contract`、`internal/wire`、wire 快照。
- 前端：
  - 代码：`chat-schemas.ts`、`recoverable.ts`、`archived.ts`、`SidebarSessionMenu.svelte`、`ArchivedSessions.svelte`、`AppSidebar.svelte`、`ChatHistory.svelte`；
  - 新增释放确认和重建确认对话框。
- 持久化：`hybrid-sessions.json` 增加可选字段。旧记录没有该字段，视为未释放，不需要迁移。
- 用户故事：US-Q12、US-Q19、US-C15、US-J08。
- 不涉及 CLI（`webmux worktree` 子命令），后续需要时另行提出。
