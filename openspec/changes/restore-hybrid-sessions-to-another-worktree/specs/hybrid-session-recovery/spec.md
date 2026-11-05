## ADDED Requirements

### Requirement: 删除工作树后保留记录

在 WebMux 中安全或强制删除工作树成功后，系统 SHALL 保留该目录下已关闭或已中断的混合会话记录及其临时文件夹：

- 删除 SHALL NOT 移除这些记录；删除操作关闭的混合会话 SHALL 同样以已关闭记录保留；
- 删除响应与删除预览 SHALL 列出这些记录，说明它们将保留并可恢复到其他工作树；
- 记录 SHALL 只能通过 `DELETE /api/hybrid-sessions/{id}` 删除。

#### Scenario: 删除工作树后记录仍在

- **WHEN** 工作树 `feature-x` 下有一条已关闭的混合会话记录，用户在 WebMux 中安全删除该工作树
- **THEN** 删除成功后该记录仍出现在可恢复列表中，其临时文件夹保留，服务重启后的清扫也不删除该文件夹

#### Scenario: 删除预览说明保留

- **WHEN** 用户打开含已归档记录的工作树的删除预览
- **THEN** 预览列出这些记录，说明删除后它们保留、可恢复到同仓库的其他工作树，不再说明它们会被移除

### Requirement: 记录摘要标明原目录缺失与可搬迁的端

`GET /api/hybrid-sessions?state=recoverable` 的每条摘要 SHALL 增加 `cwdMissing`（记录的 cwd 在请求时不存在或不是目录时为真）与 `relocatableSides`（该记录的档案支持搬迁恢复的端，取值为 `tui`、`acp` 的子集，不支持时为空数组）。`relocatableSides` SHALL 只由档案决定，与 `cwdMissing` 无关。

#### Scenario: 原目录已删除的 Codex 记录

- **WHEN** 一条 Codex 记录的 cwd 已被删除
- **THEN** 其摘要的 `cwdMissing` 为真，`relocatableSides` 为 `["tui","acp"]`

#### Scenario: 不支持搬迁的档案

- **WHEN** 一条 OpenCode 记录的 cwd 已被删除
- **THEN** 其摘要的 `cwdMissing` 为真，`relocatableSides` 为空数组

### Requirement: 恢复到其他目录

恢复请求 SHALL 接受可选的 `cwd`，表示把会话恢复到该目录。携带 `cwd` 时系统 SHALL 依次校验：

- 记录的原 cwd 已不存在，否则返回 400；
- 目标是已存在的目录，按规范化（解析符号链接）后的绝对路径比较，否则返回 400；
- 记录有工作区关联快照时，目标位于同一仓库的某个工作树内，否则返回 400；
- 档案在所请求的端支持搬迁，否则返回 409 `relocation_unsupported` 并附该档案支持的端。

校验通过后，会话 SHALL 沿用原 WebMux 会话 ID 与原 agent 会话 ID，会话外壳、终端进程与 ACP 进程 SHALL 都以目标目录为 cwd。

#### Scenario: 恢复到主工作区

- **WHEN** 一条 Codex 记录的原工作树已删除，用户以 `{side: "acp", cwd: <主工作区>}` 恢复
- **THEN** 会话以原 ID 出现在运行中列表，聊天端显示此前的对话，会话的 cwd 与代码工作区都是主工作区

#### Scenario: 原目录仍存在

- **WHEN** 记录的原 cwd 仍然存在，客户端携带另一个目录的 `cwd` 恢复
- **THEN** 服务端返回 400，不创建会话，记录不变

#### Scenario: 目标属于其他仓库

- **WHEN** 记录关联仓库 A，客户端携带仓库 B 中某个工作树的目录恢复
- **THEN** 服务端返回 400，不创建会话，记录不变

#### Scenario: 端不支持搬迁

- **WHEN** 一条 omp 记录的原目录已删除，客户端以 `{side: "acp", cwd: <主工作区>}` 恢复
- **THEN** 服务端返回 409 `relocation_unsupported`，附支持的端 `["tui"]`，不创建会话，记录不变

### Requirement: 搬迁恢复时的会话核对

携带 `cwd` 恢复时，系统 SHALL 用记录的原 cwd 调用 `session/list` 确认 agent 会话仍然存在；不存在时 SHALL 按现有规则返回 409 并附原 cwd 下的候选。占用检查（其他 WebMux 会话或外部进程正在使用该 agent 会话）SHALL 与普通恢复相同。

#### Scenario: 原目录下的候选

- **WHEN** 记录中的 agent 会话已被删除，用户携带目标目录恢复
- **THEN** 服务端返回 409，候选来自原 cwd 下的会话列表，用户选择候选后携带同一目标目录重试

### Requirement: 搬迁结果与搬迁来源

搬迁恢复成功启动后，系统 SHALL 把记录的 cwd 改为目标目录，按目标目录重算工作区关联快照，并把原 cwd 保存为搬迁来源。agent 会话在目标目录的 `session/list` 中出现后，系统 SHALL 清除搬迁来源。

记录带有搬迁来源时，后续恢复的会话核对 SHALL 先查目标目录；查不到时 SHALL 再查搬迁来源，在那里找到即视为存在，并按搬迁恢复执行。

#### Scenario: 记录回到目标工作树

- **WHEN** 原工作树已删除的记录被恢复到工作树 `main` 后再次归档
- **THEN** 该记录出现在 `main` 的已归档区，不再出现在“已不可用工作区”

#### Scenario: 用户在终端拒绝搬迁

- **WHEN** omp 记录以终端端搬迁恢复到主工作区，用户在 omp 的搬迁询问中回答否，TUI 退出
- **THEN** 记录变为已中断，cwd 为主工作区且保留搬迁来源；下一次恢复在主工作区查不到该会话，在搬迁来源下找到，按搬迁恢复再次启动
