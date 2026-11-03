# Design

## Context

当前链路（见 proposal.md - Why）：`useViewport` 把 `visualViewport.height` 写入 `--viewport-height` → `.terminal-host` 变矮 → `terminal-renderer.ts` 的 `ResizeObserver` 防抖 100 ms 后调用 `layout`（即 `safeFit`）→ `fitAddon.fit()` → `term.onResize` → `terminal-socket` 的 `sendSize` 发送 0x01 帧。

手动“锁定尺寸”现在冻结宽、高和字号对应的行列：`isSizeLocked()` 为真且已完成首次适配（`sized`）时 `safeFit` 一律跳过 fit，字号只缩放显示。

`.terminal-host.size-locked:not(.observing)` 让宿主可滚动，`safeFit`/`updateBottom` 在锁定且跟随尾部时把宿主滚到底部，iOS 下 helper textarea 贴底。

`fitAddon.fit()` 在网格不变时不会 resize，因此“按当前区域适配”在尺寸与锁定前相同时不会产生帧。

`⌨` 由 `SoftKeys` 触发，`SessionTerminal` 按草稿模式分派到 `composer.keyboard()` 或 `t.keyboard()`；两者都包含 Android“已收起但焦点仍在”的“先 blur 再 focus”重开分支。

软键盘状态（`visible`/`hidden`/`unknown`）、`keyboardDismissed` 和视口高度目前只在 `Terminal.svelte` 的 `useViewport()` 中，按会话传给 `SessionTerminal` 的只有 `keyboardDismissed`。

## Goals / Non-Goals

**Goals:**

- `⌨` 唤起并收起一次键盘，期间页面布局没有其他变化时，0 次 resize 帧。
- 锁定只抑制高度变化；首次适配、宽度、字号和手动锁定的现有规则不变。

**Non-Goals:**

- 不改 `ResizeObserver` 防抖、观察端网格逻辑或服务端 resize 处理。
- 不覆盖不经过 `⌨` 的键盘唤起。
- 不在真机以外宣称已验证 iOS/Android 原生键盘行为。

## Decisions

### 1. 独立的“仅高度”保持参数，复用锁定的显示与贴底逻辑

`useTerminal` 末尾新增可选参数 `isHeightHeld`。

`safeFit` 记录上次适配时宿主的 `offsetWidth`（含保持时网格溢出可能产生的滚动条）与字号；保持中且已 `sized`、宽度与字号都没变时跳过 fit，否则照常适配。

贴底滚动和 `updateBottom` 把 `isSizeLocked() || isHeightHeld()` 视为“网格可超出宿主”。

`SessionTerminal` 传 `() => keyboardHeld`，并把 `sizeLocked || keyboardHeld` 作为 `TerminalViewport` 的 `sizeLocked`，复用可滚动、贴底和 iOS helper 贴底样式。

结束保持后调用 `t.fit()`：手动锁定仍开启时 `safeFit` 跳过适配，满足“手动锁定优先”。

备选：把 `sizeLocked || keyboardHeld` 直接作为 `isSizeLocked` 传入。拒绝原因：手动锁定冻结宽度与字号对应的行列，会违反“保持期间宽度与字号照常适配”。

### 2. 暂时锁定状态机放在纯 TS 模块 `web/src/hooks/keyboard-size-hold.ts`

`createKeyboardSizeHold({ canHold, onActive, release })` 每个 `SessionTerminal` 一个实例，暴露 `begin(target)`、`update(keyboard)`、`cancel()`、`dispose()`；不含 rune，便于在 Node 中用假计时器测试。

`SessionTerminal` 用 `$state` 保存 `keyboardHeld`，并以 `$effect` 在键盘状态或视口高度变化时调用 `update`。

状态：`idle` → `holding`（等待键盘出现）→ `shown`（检测到 `keyboard === "visible"`）→ `closing`（收到收起信号）→ `idle`。

- `begin(target)`：仅当 `canHold()`（开关开启且 `hasTouchInput()`）为真时进入 `holding`，在 `target` 上监听 `focusout`，并启动 1.5 s 计时；计时到期仍未进入 `shown` 时转入 `closing`。
- 收起信号（`⌨` 与系统方式统一处理，不区分来源）：
  - `target` 的 `focusout`：再次点 `⌨`、iOS“完成”、点击页面其他区域，以及 `touch.ts` 在键盘被系统收起后的下一次触摸主动 blur。
  - 若 `focusout` 时焦点已落在页面内的可编辑元素上（粘贴弹窗、查找栏的输入框），软键盘会继续显示，这不是收起信号：
    - 保持停留在 `shown`，等键盘真正收起后再结束，避免在仍被压缩的高度上适配。
    - 若输入框在 `focusout` 之后才取得焦点（例如先点粘贴按钮、再由弹窗聚焦），`closing` 中的 1 s 上限也会在检测到焦点仍在可编辑元素时顺延，不会提前释放。
  - 在 `shown` 状态下 `keyboard()` 变为 `hidden`：Android 返回键或手势、输入法收起键这类焦点仍在的系统收起。`unknown`（锁定期间旋转或捏合缩放）不算收起；旋转后 `useViewport` 以新宽度重建基线，键盘收起时高度回升，仍会报告 `hidden`。
  - 在 `holding` 状态下（尚未检测到键盘）被系统收起：由 1.5 s 计时兜底转入 `closing`。
  - 以上都转入 `closing`；已在 `closing` 时重复信号忽略。系统收起后焦点仍在、用户再点 `⌨` 走既有“先 blur 再 focus”重开分支，`beforeOpen` 在 blur 之后调用，从 `idle` 重新开始。
- `closing`：每次 `update`（键盘状态或视口高度变化）都重启 200 ms 稳定计时和 1 s 上限。
  - 稳定计时期满且键盘为 `hidden` 时调用 `onActive(false)` 与 `release`（`SessionTerminal` 中为 `tick().then(t.fit)`）。
  - `unknown` 与 `shown` 一样等待真正的 `hidden`，不按已收起处理。
  - 1 s 上限只在检测信号停止变化时兜底，避免检测异常时一直锁住；`visible` 且焦点仍在可编辑元素时会顺延，`unknown` 仍受该上限约束。
- 关闭开关调用 `cancel()`：立即回到 `idle` 并调用 `release`；组件卸载调用 `dispose()`：只清理监听和计时器。

`focusout` 不立即释放：点 `⌨` 收起时，失焦先于键盘动画发生，此时立即适配会按仍然很矮的区域发出 resize，正是要消除的抖动。

备选：只依赖 `viewport.keyboard` 的阈值检测进入锁定（不靠按钮）。拒绝原因：用户已选定仅限按钮；且检测要等高度缩小超过 120 px，之前的中间尺寸已被 fit。

### 3. 在聚焦前进入锁定

`t.keyboard()` 与 `composer.keyboard()` 增加可选参数 `beforeOpen?: (target: HTMLElement) => void`，在调用 `focus()` 之前（包括“先 blur 再 focus”的重开分支中 blur 之后）调用，参数分别为 xterm 的 `textarea` 和草稿 textarea。

收起分支（已聚焦时 blur）不调用。

`SessionTerminal` 的 `keyboard` 分派传入 `hold.begin`。

这样 `focusout` 监听只覆盖本次打开，重开分支的 blur 不会提前触发收起信号。

### 4. 视口信号按会话传递

`Terminal.svelte` 给 `SessionTerminal` 增加 `keyboardVisibility: () => KeyboardVisibility` 与 `viewportHeight: () => number` 两个 getter prop（与现有 `keyboardDismissed` 同一模式），不在每个会话内另建 `useViewport`。后者会重复设置 `documentElement` 的 overflow 和监听。

### 5. 偏好使用现有本机存储约定，并以模块级状态在所有窗格间共享

沿用 `stored`/`store`（`localStorage` 中带 `webmux.` 前缀），键为 `keyboardSizeHold`，缺省视为开启（只有存储值为 `"0"` 时关闭）。

`web/src/hooks/keyboard-size-hold-preference.svelte.ts` 在模块级保存 `$state` 并提供 getter/setter，读写同一个 `localStorage` 键。

`SessionTerminal` 读取该状态，因此分屏中任一窗格切换开关时，其他已挂载窗格（含菜单勾选状态和进行中的保持）立即跟随，无需刷新。每个窗格用 `$effect` 在开关关闭时调用 `cancel()`，结束本窗格的保持。

不走服务端设置同步：与“锁定尺寸”同类，取决于单台设备的键盘与屏幕。

## Risks / Trade-offs

- [键盘开着时顶部行被挡住] → 与手动锁定一致：宿主可滚动查看，默认贴底；提供菜单开关恢复实时适配。
- [iOS `visualViewport` 在键盘动画结束才触发 resize，或 `keyboard` 检测返回 `unknown`] → `closing` 只要求“不再 visible + 高度稳定”，另有 1 s 上限；`holding` 有 1.5 s 无键盘上限。
- [锁定期间地址栏显隐导致的高度变化被延后] → 只延后到键盘收起，收起后一次适配追上。
- [草稿模式下用户在锁定期间拖动面板] → 只延后到收起时一次适配，符合规格场景。
- [Chromium 移动模拟无法弹出原生软键盘] → 单元测试驱动注入的键盘状态，浏览器与 E2E 测试以改变宿主或视口高度模拟；真机行为单独记录为未验证边界。
- [保持中网格溢出产生的滚动条改变 `clientWidth`，被误判为宽度变化] → 宽度比较使用包含滚动条的 `offsetWidth`。

## Migration Plan

纯前端行为，默认开启；回退时可在菜单中关闭，或回滚前端构建。无数据迁移。
