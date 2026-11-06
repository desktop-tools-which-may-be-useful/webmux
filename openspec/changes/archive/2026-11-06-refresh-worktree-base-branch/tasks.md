## 1. 工作区上游刷新服务

- [x] 1.1 在 `internal/workspace` 的分支结果中增加主工作区 `defaultBranch`，保持 `defaultBase` 兼容；用同 OID 多分支与 detached HEAD 测试验证初始分支不会被猜测，并运行 `go test -race ./internal/workspace`。
- [x] 1.2 实现按精确本地分支解析已配置上游、30 秒非交互 `fetch --no-tags`、远程 OID/标题读取及本地回退，不猜测同名 remote；用临时 bare remote 覆盖远端前进、无上游、remote 为 `.`、失败和超时，确认本地分支与工作区不被切换或重置，并运行对应 Go 测试。
- [x] 1.3 确保网络 fetch 在仓库操作 gate 外执行，fetch 后仅短暂重读引用；用可控慢 fetch 与并发创建测试证明创建不等待网络超时，并运行 `go test -race ./internal/workspace`。

## 2. HTTP 与前后端契约

- [x] 2.1 在 `internal/contract`、`internal/wire`、`internal/api` 增加 `POST /api/repositories/{id}/branches/refresh` 及设计字段，复用认证、Origin、JSON 校验和错误映射；测试成功、无上游/失败降级、非法或未知分支及安全拒绝，运行 `go test -race ./internal/api ./internal/workspace`。
- [x] 2.2 同步 `web/src/schemas.ts` 和现有 API 请求封装中的 `defaultBranch`、刷新请求/响应类型，重录 `go test ./internal/api -run TestWireSnapshots -update`；运行 `pnpm --dir web exec vitest run --config vitest.config.ts test/wire-contract.test.ts test/api.test.ts` 验证契约。
- [x] 2.3 更新 `docs/protocol/http-workspace.md` 的离线清单与刷新接口、降级响应和安全边界，并用文档中的请求示例核对实际路由与字段后运行 `make docs-check`。

## 3. 创建面板与竞态处理

- [x] 3.1 在 `WorktreeCreateFields.svelte` 增加本地起始分支选择、立即本地基线、自动异步刷新、远程来源标签、旋转进度和手动刷新按钮；保持 `refreshing` 独立于 `ready`，用浏览器测试验证刷新期间字段可编辑且能用当前 OID 创建，并运行相关 Vitest 浏览器测试。
- [x] 3.2 实现刷新代次、基线用户编辑版本及中止/忽略逻辑；用可控 Promise 覆盖 A→B 过期响应、请求后手动编辑、关闭面板、切换已有分支模式、无上游与失败回退，确认旧响应不覆盖新意图且已有分支模式不 fetch。
- [x] 3.3 更新 `docs/usage.md`、`docs/user-stories.md` 和 `docs/user-stories/code-and-worktrees.md` 的 US-J04，说明起始分支、已配置上游、非阻断创建和本地回退；核对 US-J05 的已有分支无 fetch 行为并保持不需要的文档不变，运行 `make docs-check`。
- [x] 3.4 在 `web/test/e2e/worktrees.spec.ts` 用隔离 bare remote 验证远端前进后自动采用上游 OID、手动刷新和失败回退；运行 `pnpm --dir web exec playwright test test/e2e/worktrees.spec.ts`，不连接已有开发服务。

## 4. 集成验收

- [x] 4.1 运行 `make check`、`pnpm --dir web check:arch` 和 `make test`，修复本变更导致的失败；不扩张任何架构 baseline。
- [x] 4.2 在隔离演示仓库的实际创建面板验证桌面/移动及浅色/深色界面，确认 spinner 不阻塞创建、远程/本地来源和警告可辨识；按仓库要求留存最终截图与可复核 URL，并确认没有实现代码之外的临时脚手架遗留。
