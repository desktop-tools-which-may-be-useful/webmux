# Tasks

## 1. 档案声明与适配器

- [ ] 1.1 在 `internal/agentprofile` 增加 `Relocation` 声明、可选的适配器方法与 `Command.RelocateResumeArgv`；`agenttest` 契约覆盖“声明了终端端参数的档案生成的命令包含这些参数且恢复参数在最后”，`go test -race ./internal/agentprofile/...` 通过
- [ ] 1.2 Codex 适配器声明两端，终端端注入 `-c tui.resume_cwd="current"`；单测按 spec 场景断言完整 argv，并断言普通 `ResumeArgv` 不含该参数
- [ ] 1.3 Claude 适配器实现 `PrepareStore`（复用 `configDir`，目录名替换、200 字符与 `CLAUDE_CODE_PROJECT_DIR_NAME` 拒绝、目标已存在拒绝、`<id>/` 回滚）；临时目录单测覆盖成功移动、每种拒绝条件下不移动文件、子目录移动失败时回滚
- [ ] 1.4 omp 适配器只声明终端端，其他档案不声明；`agenttest` 断言各档案的 `Sides` 与 spec 一致

## 2. 记录与恢复服务

- [ ] 2.1 `hybrid.Record` 增加 `RelocatedFrom`，旧文件读取用例通过；如持久化守卫要求，更新 `docs/persistence.md` 并让 `make check` 中的守卫通过
- [ ] 2.2 `hybrid.ResumeInput` 增加目标 cwd，`resume` 在目标目录创建外壳与 actor；`verify` 在原 cwd（或 `RelocatedFrom` 回退）核对；按 D5 写入与清除 `RelocatedFrom`；端不支持时返回 `ErrRelocationUnsupported`
  - 验证：`internal/hybrid` 单测用 fakeagent 覆盖成功搬迁后的记录 cwd 与来源、原 cwd 下的候选、端不支持、搬迁来源回退、在目标目录列出后清除来源
- [ ] 2.3 扩展 `internal/acp/testdata/fakeagent`，按 D9 模拟 codex、claude、omp 的跨目录行为；fakeagent 自身测试覆盖每种行为
- [ ] 2.4 `workspace.ResumeHybrid` 校验原目录缺失、目标为已存在目录、同仓库工作树，用目标目录 `beginStart`/`association`，Claude 搬迁在启动前调用 `PrepareStore`；`internal/workspace` 单测覆盖三类 400 与成功后的关联
- [ ] 2.5 `RemoveWorktree` 不再移除记录，删除响应与预览改为列出保留的记录；更新 `remove_test.go` 中原先断言记录被移除的用例，改为断言记录与临时文件夹都保留

## 3. API 与契约

- [ ] 3.1 `contract.HybridResumeRequest` 增加 `cwd`，`contract.HybridRecord` 增加 `cwdMissing`、`relocatableSides`，`wire` 转换；`errors.go` 映射 `relocation_unsupported`（409，附 `sides`）。`internal/api/hybrid_test.go` 覆盖成功、三类 400、409，认证与 Origin 拒绝用例继续通过
- [ ] 3.2 同步 `web/src/schemas.ts` 与删除预览/响应的 schema，重录 `go test ./internal/api -run TestWireSnapshots -update`，`wire-contract.test.ts` 通过

## 4. 前端

- [ ] 4.1 `ArchivedSessions.svelte` 按 spec 显示“恢复到其他工作树…”或禁用说明；浏览器测试覆盖可搬迁、不可搬迁、原目录存在三种记录
- [ ] 4.2 新增 `RelocateHybridDialog.svelte`（`Dialog` + `LocationPicker` + 端选择 + `Alert` 风险说明），从 `HybridResumeActions` 抽出共享的恢复提交与错误处理；浏览器测试覆盖目标只列同仓库工作树、端只列支持的端、409 候选与 `relocation_unsupported` 在对话框内显示、焦点回到触发按钮
- [ ] 4.3 `RemoveWorktreeDialog.svelte` 更新已归档记录说明，更新对应浏览器测试与 Storybook 故事；`pnpm --dir web check:arch` 通过
- [ ] 4.4 E2E：用 fakeagent 创建 Codex 混合会话→归档→删除工作树→在“已不可用工作区”恢复到主工作区的聊天端，断言会话进入主工作区；桌面与 Pixel 7 两个项目都通过

## 5. 文档与故事

- [ ] 5.1 更新 US-Q12、US-J08、US-J09（`docs/user-stories/`）及覆盖索引，更新 `docs/usage/worktrees.md` 的删除说明与搬迁恢复说明；`make docs-check` 通过

## 6. 集成验证

- [ ] 6.1 隔离实例中人工验证真实 agent：Codex（已登录）终端端与聊天端、Claude 聊天端与终端端、omp 终端端（含拒绝搬迁后再次恢复），在 PR 中记录版本与结果；`tui.resume_cwd` 不生效时按 design 风险项从 Codex 的终端端声明中移除并同步 spec
- [ ] 6.2 `make check` 与 `make test-fast` 通过；UI 截图（桌面/手机、浅色/深色）附在 PR 中
