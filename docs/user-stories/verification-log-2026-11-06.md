# 核对记录：2026-11-06 起

本文收录 2026-11-06 起各次局部核对的记录：对应的故事、已运行的验证、范围限制与未核对的部分，条目按日期升序排列。前一份记录见 [verification-log-2026-11](verification-log-2026-11.md)。

本次局部核对（2026-11-06）：实现 OpenSpec 变更 `release-archived-worktree-space`（已归档工作树释放空间与恢复前重建），新增 US-J13，更新 US-Q12、US-Q19、US-C15 与覆盖索引。

- 本地检查：`go build ./...`、`go vet`（hybrid、workspace、api、wire、contract）、`gofmt`、`svelte-check`、Oxlint、`check:arch`、格式检查与 `make docs-check`。
- 快照：用 `go test ./internal/api -run TestWireSnapshots -update` 重新录制 wire 快照。
- CI：Go race、Vitest 单元与浏览器、Storybook、性能与四个 E2E 分片在 PR #344 上通过。
- 隔离演示服务（临时状态目录与 `/tmp/demo` 仓库，Claude Code 记录不启动 agent）上实际走通：
  - 侧栏“释放空间”预览与释放：目录删除、分支保留、私有引用建立，记录标注“已释放空间”并归入“已不可用工作区”；
  - `rebuild-preview`/`rebuild` 接口在原路径检出原分支，并清理私有引用；
  - 截取了侧栏与首页的桌面、Pixel 7、明暗主题前后对比截图。
- 限制：未在本机运行测试套件；未在演示中点击“重建并恢复”启动真实 agent；未做真机核对；未重新核验其余故事能力。

本次局部核对（2026-11-06）：删除 worktree 保留已归档记录并标为已释放、删除与释放前的推送检查、侧栏树显示已归档记录；更新 US-J08、US-J09、US-J13、US-Q12、US-C17 与覆盖索引，不新增故事。

- 本地：`go test -race` 覆盖 workspace、hybrid、wire、contract，api 的强制删除用例与 `TestWireSnapshots`；`svelte-check`、Oxlint、`check:arch`、格式与文档检查；受影响的浏览器用例（树、对话框、外壳）。
- 隔离实例（假 ACP agent、本地 bare 远程）：未推送时两个预览均为 `unpushed`，推送后为 `pushed`；删除后记录为已释放并出现在首页与侧栏树，点击后重建并恢复成功；无提交的 worktree 删除成功、记录原样保留。
- 限制：完整套件与 E2E 交给 CI；未做真机核对。

本次局部核对（2026-11-07）：侧栏收起为窄栏时只保留展开按钮和当前模式的主要操作，聊天/工作台模式切换仅在展开的侧栏中显示，对应更新 US-C16 与 `app-shell` 主规格，不新增故事或改变覆盖索引。

- 范围：桌面端与平板窄栏；展开后的侧栏、手机抽屉与模式记忆不变。
- 依据：`AppShell` 浏览器组件测试（窄栏保留“展开侧栏”和“新建会话”，不渲染模式按钮；展开后模式按钮恢复）通过；`pnpm --dir web check`、`lint`、`check:arch`、格式检查与 `make docs-check` 通过。
- 限制：未运行全量 E2E 与 Storybook；未在真机核对；未重新核验其余故事能力。
