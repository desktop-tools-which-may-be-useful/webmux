# Tasks

## 1. 暂时锁定状态机

- [x] 1.1 新增 `web/src/hooks/keyboard-size-hold.ts`（纯 TS，便于 Node 单元测试），实现 design 决策 2 的 `idle/holding/shown/closing` 状态机、1.5 s 无键盘上限、收起后“键盘 hidden + 高度 200 ms 稳定”释放、信号停止变化后的 1 s 释放上限、关闭开关立即释放和卸载清理。
  - 验证：`pnpm --dir web exec vitest run --config vitest.config.ts test/keyboard-size-hold.test.ts` 通过。
  - 用假计时器覆盖：`⌨` 收起只释放一次；失焦（iOS“完成”/点其他区域）时键盘仍 visible 不释放，收起且高度稳定后才释放。
  - 焦点转到粘贴弹窗、查找栏等其他可编辑元素时保持 `shown`；焦点在 `focusout` 之后才进入输入框时，1 s 上限顺延，键盘收起后才释放。
  - 焦点仍在的系统收起（shown→hidden）释放；锁定期间旋转（visible→unknown→hidden）只在收起后释放；`closing` 中的 `unknown` 不提前释放。
  - holding 中被系统收起由 1.5 s 兜底释放；检测停在 visible 且信号不再变化时 1 s 上限释放；信号持续变化时上限顺延；系统收起后再次 `begin` 重新锁定；不允许保持时不进入锁定。

## 2. 接入终端与草稿

- [x] 2.1 `t.keyboard()`（`web/src/hooks/terminal.svelte.ts`）与 `TerminalInputComposer.keyboard()` 增加 `beforeOpen` 回调，仅在即将 `focus()` 时调用（重开分支在 blur 之后）；收起分支不调用。验证：`pnpm --dir web check` 无错误
- [x] 2.2 接入保持状态机，不改变手动“锁定尺寸”的语义。
  - `Terminal.svelte` 向 `SessionTerminal` 传入 `keyboardVisibility` 与 `viewportHeight` getter。
  - `useTerminal` 新增 `isHeightHeld`：仅在宽度与字号未变时跳过 fit，并复用锁定的贴底滚动。
  - `SessionTerminal` 创建保持控制器，传入 `() => keyboardHeld`，以 `sizeLocked || keyboardHeld` 作为 `TerminalViewport` 的 `sizeLocked`；`⌨` 分派传入 `hold.begin`，释放回调 `tick().then(t.fit)`。
  - 验证：`pnpm --dir web check` 与 `pnpm --dir web check:arch` 通过
- [x] 2.3 在 `web/test/browser/TerminalTransport.test.ts` 经 `TerminalHarness` 的 `heightHeld` 补充回归：保持期间只改高度不发送 0x01 帧且宿主贴底；恢复原高度后释放不发送帧；保持期间改宽度、改字号各发送一帧；释放时高度已变则发送一帧。
  - 验证：`pnpm --dir web exec vitest run --config vitest.browser.config.ts test/browser/TerminalTransport.test.ts` 通过

## 3. 菜单开关与文档

- [x] 3.1 `web/src/hooks/keyboard-size-hold-preference.svelte.ts` 以模块级 `$state` 保存偏好并读写 `localStorage`（`keyboardSizeHold`，缺省开启），所有窗格共享；`TerminalHeader` 在“锁定尺寸”下方增加“键盘弹出时保持尺寸”，关闭时立即释放各窗格的保持。
  - 验证：`web/test/e2e/softkeys.spec.ts` 的 Pixel 7 用例覆盖默认勾选、`⌨` 开合期间无 resize 帧且提示符贴底可见、取消后刷新仍未勾选、关闭后同一操作产生 resize 帧
- [x] 3.2 更新文档。验证：`make docs-check` 与 `make format-check` 通过。
  - 把 `docs/usage.md` 的移动端段落拆到 `docs/usage/mobile.md`（`usage.md` 已到 400 行上限）并补充说明。
  - 更新 `docs/user-stories/terminal.md` 中 US-H06 的验收标准与源码引用。
  - 同步 `docs/user-stories/verification-log-2026-11.md` 核对记录与 `docs/user-stories/coverage.md` 索引。

## 4. 集成验证

- [x] 4.1 运行 `make test-fast`、相关 E2E 和 `openspec validate hold-terminal-size-on-keyboard-button --strict`，记录实际结果。
  - 2026-11-03（基于 `origin/main` a0f428f）：Vitest 单元 581 项、浏览器 438 项通过；`softkeys`/`composer`/`terminal` E2E 49 项通过（11 项按项目跳过）；strict 校验通过。
  - Go race：在 WebMux 会话内运行时，`cmd/webmux` 隧道与 `internal/wslclient` 用例因会话内进程禁止控制隧道/停止服务失败；清除 `WEBMUX_SESSION_ID` 后两包通过，其余包通过。本变更不涉及 Go 代码。
- [x] 4.2 用隔离服务在 Chromium Pixel 7 视口实测全屏 TUI。
  - 在 `vim` 中点 `⌨` 后把视口缩小 300 px 再收起：开启保持时 0 个 resize 帧、0 字节重绘输出，`vim` 状态行贴底可见；关闭开关后同一操作产生 4 个 resize 帧（37→22→37 行）和 342 字节重绘输出。
  - 未验证边界：没有 iOS/Android 真机，原生软键盘动画、系统返回键与输入法收起键只由单元测试中的状态序列和视口模拟覆盖。

## Workflow follow-up

- 评审后归档变更，并核对 `openspec/specs/terminal-keyboard-size-hold/spec.md` 的 Purpose。
- UI 变更的 PR 需附终端菜单新开关的手机视口截图（亮/暗主题）。
