# Proposal

## Why

混合会话的记录绑定在创建时的 cwd 上，工作树删除后这段 agent 对话就无法在 WebMux 中继续：

- 在 WebMux 内删除工作树时，记录被一并移除（#306）；
- 在终端里用 `git worktree remove` 删除时，恢复会因目录不存在而失败。

实际工作中，任务分支合并、工作树删除之后，经常还需要在主工作区或另一个工作树里接着同一段对话追问、补修。实测表明 Codex、Claude Code、omp 都能把会话接到新目录，只是做法不同，WebMux 可以按 agent 分别提供。

## What Changes

- 在 WebMux 内删除工作树时，该目录下已关闭或已中断的混合会话记录与其临时文件夹 SHALL 保留，不再随删除移除；删除预览与确认文案改为说明“记录保留，可恢复到其他工作树”。**BREAKING**（相对 #306 的行为）：删除工作树后“已不可用工作区”中会出现这些记录，需要用户自己删除不再需要的记录。
- 记录的 cwd 已不存在时，记录摘要标明原目录缺失，并列出该 agent 支持搬迁恢复的端。
- `POST /api/sessions/{id}/resume` 增加可选的目标目录 `cwd`：仅在原目录已不存在时可用；目标必须是已存在的目录，记录有仓库关联时必须位于同一仓库的某个工作树内。恢复仍沿用原 WebMux 会话 ID 与原 agent 会话 ID。
- 档案声明各自的搬迁恢复方式：
  - Codex：两端都支持，聊天端以新 cwd `session/load`，终端端在恢复命令中注入 `-c tui.resume_cwd="current"`；
  - Claude Code：两端都支持，恢复前由 WebMux 把 transcript 从原目录对应的项目目录移到新目录对应的项目目录；
  - omp：只支持终端端，由 omp 在终端里询问是否搬迁、用户确认；
  - OpenCode、Antigravity CLI、Qoder、QoderCN：本变更不支持。
- 搬迁成功后，记录的 cwd 与工作区关联改为目标目录，记录从“已不可用工作区”回到目标工作树。
- 首页“已不可用工作区”与直接会话中 cwd 缺失的已归档记录提供“恢复到其他工作树…”操作，在对话框中选择目标工作树与端。

## Capabilities

### New Capabilities

（无）

### Modified Capabilities

- `hybrid-session-recovery`：新增“删除工作树后保留记录”“恢复到其他目录”“搬迁来源与确认”“记录摘要标明原目录缺失与可搬迁的端”等要求。
- `hybrid-agent-profiles`：新增“档案声明搬迁恢复方式”的要求，规定 Codex、Claude Code、omp 的具体做法与其他档案不支持。
- `workspace-overview`：新增已归档记录“恢复到其他工作树…”的入口、对话框与风险提示要求。

## Impact

- Go：
  - `internal/hybrid`：`Record` 新增搬迁来源，`Resume` 接受目标 cwd，`verify` 在原 cwd 查询；
  - `internal/workspace`：`ResumeHybrid` 校验目标并重算关联，`RemoveWorktree` 不再调用 `forgetRecords`，删除预览语义改变；
  - `internal/agentprofile` 及 `codex`/`claudecode`/`omp` 适配器；
  - `internal/contract`/`internal/wire`，以及 `internal/api/errors.go` 的新错误映射。
- 前端：`web/src/schemas.ts`、`ArchivedSessions.svelte`、`HybridResumeActions.svelte`、`RemoveWorktreeDialog.svelte`、新增搬迁恢复对话框（复用 `LocationPicker`）。
- 测试夹具：`internal/acp/testdata/fakeagent` 需要模拟三种 agent 的跨目录行为；重录 `internal/api/testdata/wire`。
- 状态文件：`hybrid-sessions.json` 记录新增可选字段，旧文件照常读取，`docs/persistence.md` 按守卫要求登记。
- 文档：用户故事 US-Q12、US-J08、US-J09，`docs/usage/worktrees.md`。
- 外部依赖：WebMux 首次写入 Claude Code 的会话目录（移动 transcript），依赖其文档化的存储位置与目录命名规则。
