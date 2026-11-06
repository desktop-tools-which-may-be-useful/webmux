# Tasks

## 1. 默认值

- [x] 1.1 `web/src/ws.ts` 的 `wsCompression()`：存储为 `on`/`off` 时直接返回，否则返回 `(pointer: coarse)` 匹配结果（无 `matchMedia` 视为关闭）。
  - 验证：`pnpm --dir web exec vitest run --config vitest.config.ts test/ws.test.ts` 通过，新增默认值与显式选择用例。
- [x] 1.2 `NetworkSettings.svelte` 说明文字注明手机与平板默认开启。

## 2. 文档

- [x] 2.1 更新 `docs/user-stories/terminal-network.md`（US-H10 与取舍）、`docs/protocol/websocket.md`、`docs/user-stories/coverage.md` 与核对记录。
  - 验证：`make docs-check` 通过。

## 3. 集成验证

- [x] 3.1 运行前端检查、`openspec validate default-mobile-ws-compression --strict`。2026-11-03：svelte-check、Vitest 单元 590 项、`docs-check`、docguard/archtest 与 strict 校验通过；未做真机移动网络测速。
