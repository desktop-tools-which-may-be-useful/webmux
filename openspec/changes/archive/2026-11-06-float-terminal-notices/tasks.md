# Tasks

## 1. 悬浮提示

- [x] 1.1 `SessionTerminal.svelte` 在 `terminalPane` 片段内放置绝对定位悬浮层（`z-30`），承载 `TerminalNotices` 与连接器安装提示。
  - 验证：`pnpm --dir web check` 与 `pnpm --dir web check:arch` 通过。

## 2. 文档

- [x] 2.1 更新 `docs/usage/terminal.md`、`docs/user-stories/terminal.md` 的 US-D05 与 `docs/user-stories/verification-log-2026-11.md`。
  - 验证：`make docs-check` 通过。

## 3. 集成验证

- [x] 3.1 运行 `make docs-check`、`make format-check`、`openspec validate float-terminal-notices --strict`、`pnpm --dir web check` 与 `check:arch`，均通过；浏览器 Vitest 全量 451 项通过，`TerminalGraphics` 一项在并发负载下偶发失败，单独重跑 18 项通过。
