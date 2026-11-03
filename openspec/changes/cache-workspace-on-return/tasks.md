# Tasks

## 1. 缓存模块与首页接入

- [x] 1.1 `web/src/repository-cache.ts` 改名为 `web/src/home-cache.ts`，保存仓库、会话与设置并导出 `clearHomeCache`；`App.svelte` 的 `webmux:auth` 改调 `clearHomeCache`。
  - 验证：`pnpm --dir web check` 与 `pnpm --dir web check:arch` 通过
- [x] 1.2 `SessionList.svelte` 以缓存初始化会话、设置、`loading`、`initializing` 与 `setupSettled`；快照与增量事件后写会话缓存，`/api/config` 成功后写设置缓存。
  - 验证：`pnpm --dir web check` 通过

## 2. 测试与文档

- [x] 2.1 `web/test/e2e/frontend-quality.spec.ts` 新增用例：返回首页时挂起 `/api/config` 与 `/api/repositories`，仍显示搜索框与上次会话且没有骨架屏。
  - 验证：`pnpm --dir web exec playwright test test/e2e/frontend-quality.spec.ts` 通过
- [x] 2.2 更新 `docs/user-stories/access-and-sessions.md`（US-C01、US-C02）、`docs/usage.md` 与 `verification-log-2026-11.md`。
  - 验证：`make docs-check` 通过，`openspec validate cache-workspace-on-return --strict` 通过

## Workflow follow-up

- 评审后归档变更，并核对 `openspec/specs/workspace-return-cache/spec.md` 的 Purpose。
