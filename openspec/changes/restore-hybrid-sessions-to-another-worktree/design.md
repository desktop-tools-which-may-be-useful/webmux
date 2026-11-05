# Design

## Context

- 动机见 proposal.md。恢复路径现在处处使用记录的 cwd：`workspace.ResumeHybrid`（`beginStart`/`association`）、`hybrid.Service.resume`（`newActor`、`CreateShell`）、`actor.verify`（`session/list`）、`openACP`（`session/load`/`resume` 的 `Cwd`）。`HybridResumeInput` 没有目录参数。
- `workspace.RemoveWorktree` 在删除成功后调用 `forgetRecords`，用 `within(path, rec.Cwd)` 删除该目录下的可恢复记录（#306）；删除预览的 `archived` 字段列出这些记录。
- 首页归位（`web/src/archived.ts` 的 `placeArchived`）已经把工作树不在 Git 清单中的记录归入该仓库的“已不可用工作区”。这部分规则由尚未归档的变更 `archive-hybrid-sessions-in-worktrees` 描述、代码已实现；本变更的 delta 只用 ADDED，避免与它的 MODIFIED 冲突。
- Claude 配置目录的解析已有实现：`internal/agentprofile/claudecode/registry.go` 的 `configDir`。

### 实测结论（2026-11-05，本机）

固定装置：一个仓库加一个 linked worktree A，B 为主工作区，在 B 中启动连接器。

- **codex-acp 2.5.0**
  - 原目录存在：以 cwd B `session/load`、`resume` 成功，回放历史，进程 cwd 为 B；
  - rollout 随之追加 B 的 cwd，此后 `session/list` 在 B 列出、在 A 不再列出；
  - 删除 A 后：A 的 `session/list` 仍列出该会话，以 cwd B `session/load` 成功。
- **claude-agent-acp 0.100.0（内置 CLI 2.1.316）**
  - 原目录存在：以 cwd B `session/load`、`resume` 成功，回放历史，子进程 cwd 为 B；B 的 `session/list` 不列出；
  - 删除 A 后：A 的 `session/list` 仍列出；以 cwd B `session/load` 先回放、再返回 `Resource not found`；
  - 原因：CLI 按“当前项目及其工作树”查找，A 已不是工作树；`claude -p --resume <id>` 同样找不到；
  - 把 transcript 放进 B 的项目目录后，B 的 `session/list` 列出该会话，`session/load` 成功。
- **omp 18.16.0 ACP**
  - 无论 A 是否存在，`session/resume`/`load`/`fork` 以 cwd B 都返回 `ACP session load was cancelled`；
  - 源码：记录 cwd 与当前 cwd 不同时，`switchSession` 抛出 `sessionCwdChangeRejected`；
  - 删除 A 后，A 的 `session/list` 仍列出。
- **omp 18.16.0 TUI**
  - 原目录存在：`omp --resume <id>` 会 chdir 回原目录 A（源码中的 `je()`）；
  - 删除 A 后在 B 中执行：询问 `Move (re-root) it into the current directory? [Y/n]`，回答 y 后会话文件移到 B 的会话目录。

Codex 的 `tui.resume_cwd` 出自官方 CLI 参考，二进制中能找到该配置键；终端端的效果没有实测（本机 Codex 未登录）。Claude 的项目目录命名与 200 字符截断规则出自官方文档 “Manage sessions”。

## Goals / Non-Goals

**Goals:**

- 原目录已删除的记录能在同仓库的另一个工作树中继续同一个 agent 会话（同一 agent 会话 ID）。
- 每个 agent 的做法集中在其档案适配器中，hybrid 只按声明调用。

**Non-Goals:**

- 原目录仍存在时的搬迁或分叉：omp 与 Claude 的 TUI 会切回原目录，WebMux 记录的 cwd 与进程的实际 cwd 会不一致；分叉需要新的 agent 会话 ID 与新记录，另立变更。
- OpenCode、Antigravity CLI、Qoder、QoderCN 的搬迁：未实测。
- 运行中会话的目录迁移（例如 omp 的 `/move`、`/wt`）。

## Decisions

### D1 只在原目录不存在时允许搬迁

`ResumeHybrid` 收到 `cwd` 时先 `os.Stat(rec.Cwd)`，存在则返回 `ErrInvalid`。这样排除了“两个目录都在、agent 自己选目录”的情况；实测中所有 agent 在原目录删除后的行为都是确定的。

- 备选：任何时候都允许。被否决，见 Non-Goals。

### D2 档案接口：可选方法声明搬迁方式

在 `agentprofile` 中定义：

```go
type Relocation struct {
    Sides []Side                    // 支持的端
    TUIArgs func(kept []string) []string // 终端端额外注入的全局参数，Codex 用
    PrepareStore func(RelocateRequest) error // 启动前在 agent 存储中搬迁，Claude 用
}
```

适配器通过可选方法 `Relocation() ap.Relocation` 提供，与现有的 `ResumesSessions()`、`ReapplyOnLoad()` 写法一致；没有实现的档案视为不支持。

- `Command` 增加 `RelocateResumeArgv(sid)`，在 `injected + kept` 之后、恢复参数之前插入 `TUIArgs`；
- `RelocateRequest` 含原 cwd、目标 cwd、agent 会话 ID、启动环境的 `getenv` 与服务用户 HOME。

- 备选：在 hybrid 里按档案名写 switch。被否决：档案细节会散到 hybrid 中，与现有档案驱动的写法不一致。

### D3 Claude：WebMux 在启动前移动 transcript

`PrepareStore` 复用 `configDir` 解析配置目录，计算两个项目目录名（非字母数字替换为 `-`），按 spec 中的拒绝条件检查，然后对 `<id>.jsonl` 和可能存在的 `<id>/` 子目录做 `os.Rename`；`<id>/` 移动失败时把已移动的 `.jsonl` 移回去。只做重命名，不读取内容。

- 备选 A：终端端用文档化的 `claude --resume <transcript路径>`。只解决终端端；聊天端只接受会话 ID，无法使用。
- 备选 B：用 Claude 自己的 `/cd` 命令。需要先在原目录启动，而原目录已经不存在。
- 备选 C：依赖跨项目按 ID 查找（文档称 2.1.223 起支持）。实测 2.1.316 在 A 删除后找不到，被否决。

### D4 omp：只支持终端端，由用户在终端确认

omp 的 ACP 拒绝 cwd 变化，没有开关；TUI 在原目录不存在时会询问搬迁。WebMux 只在目标目录执行普通的 `--resume <id>`，由用户自己回答。

- 备选：WebMux 自动向终端写入 `y`。被否决：依赖提示文本和时序，与“不注入按键”的约定冲突。
- 备选：离线 `omp session pack`/`unpack --cwd`。会产生同一 ID 的两份副本，所有权语义不明确。

### D5 会话核对在原目录进行，记录保存搬迁来源

`verify` 改为接受 `listCwd`：搬迁恢复时传原 cwd（实测三种 agent 在原目录删除后仍能按原 cwd 列出）。

恢复启动成功后写入 `Record.Cwd = 目标`、`Record.RelocatedFrom = 原 cwd`，`Workspace` 由 `commitAssociation` 按目标重算。`RelocatedFrom` 在两种情况下清除：

- actor 下一次在目标目录的 `session/list` 中看到该 ID（切换资格探测或恢复核对时）；
- 下一次恢复在目标目录核对成功。

普通恢复的核对顺序为：先查 `rec.Cwd`；查不到且 `RelocatedFrom` 非空时再查 `RelocatedFrom`，找到即把这次恢复当作搬迁恢复（端也要通过 `Relocation().Sides` 检查）。这样覆盖了 omp 用户拒绝搬迁、或启动后立刻崩溃的情况。

- 备选：等确认迁移完成后才改 `Record.Cwd`。终端端没有可靠的“已搬迁”信号（omp 只能从后续事件推断），而且外壳已经运行在目标目录，记录必须与外壳一致。

### D6 删除工作树不再移除记录

去掉 `RemoveWorktree` 中的 `forgetRecords`；`targetRecords` 保留，用于删除预览的 `archived` 列表，预览文案改为“将保留，可恢复到其他工作树”。临时文件夹按 `session-artifacts` 的现有规则随记录保留。删除响应中原来的被移除 ID 列表改为保留的 ID 列表，字段名与 wire 快照一起调整。

### D7 API 与契约

- `contract.HybridResumeRequest` 增加 `Cwd string \`json:"cwd,omitempty"\``；`wire.HybridResumeInput`、`workspace.HybridResumeInput`、`hybrid.ResumeInput` 同步增加。
- `contract.HybridRecord` 增加 `CwdMissing bool`、`RelocatableSides []HybridSide`（非 nil）；`CwdMissing` 在列表请求时由 `workspace` 对每条记录 `os.Stat`（最多 200 次本地 stat，不调用 Git）。
- 新增 `hybrid.ErrRelocationUnsupported`（409，`reason: relocation_unsupported`，附 `sides`），在 `internal/api/errors.go` 中映射。目标校验失败归入现有 `ErrInvalid`（400）。
- `Record.RelocatedFrom string \`json:"relocatedFrom,omitempty"\``，旧文件照常读取；持久化守卫要求时更新 `docs/persistence.md`。

### D8 前端

- `schemas.ts` 同步新字段；`ArchivedSessions.svelte` 按 spec 显示入口或禁用说明。
- 新组件 `RelocateHybridDialog.svelte`：用 `Dialog`，位置用 `LocationPicker`（限定为记录所属仓库的工作树），端用现有分段控件；风险说明用 `Alert`。请求复用 `api.ts`，错误处理复用 `HybridResumeActions` 中的候选与 `config_missing` 流程（抽出共享的提交函数，不复制）。
- `RemoveWorktreeDialog.svelte` 更新已归档记录的说明文案。

### D9 测试装置

`internal/acp/testdata/fakeagent` 按 profile 模拟实测行为：codex 允许以新 cwd 加载并追加 cwd；claude 在原目录不是工作树时按 ID 加载失败、transcript 在新项目目录时成功；omp 的 ACP 拒绝 cwd 变化，TUI 在原目录缺失时打印搬迁询问并等待输入。

## Risks / Trade-offs

- [Claude 的存储布局是内部实现，可能变化] → 只依赖文档化的项目目录命名，拒绝超长路径与自定义项目名；移动前校验源文件存在、目标不存在；失败时不启动 agent，记录不变。`VerifiedVersion` 更新时复查。
- [对话历史引用旧工作树的绝对路径与分支] → 对话框显示风险说明；Codex 的沙箱可写范围跟随新 cwd（openai/codex#4791），这正是预期的目标目录。
- [保留记录会让“已不可用工作区”累积条目] → 每条记录都能直接删除；200 条上限不变，达到上限时按现有规则提示。
- [`tui.resume_cwd` 未实测] → 实现阶段在已登录的 Codex 上用 E2E 之外的人工步骤验证，并在 PR 中记录；不可用时 Codex 终端端从 `Sides` 中去掉。
- [omp 用户拒绝搬迁后记录的 cwd 已改变] → D5 的搬迁来源回退保证下次仍能找到会话。

## Migration Plan

- 记录新字段可选，旧服务读取新文件会忽略 `relocatedFrom`（Go 解码默认行为），回滚安全。
- 回滚后，删除工作树时又会移除记录；本变更期间保留的记录在回滚后仍可删除。

## Open Questions

- Claude 的 `<id>/` 子目录在当前版本中的确切内容（subagent transcript、工具结果）不影响做法：整体移动即可；实现时用真实会话确认存在与否。
