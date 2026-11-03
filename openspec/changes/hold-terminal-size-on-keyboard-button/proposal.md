# Proposal

## Why

在手机上点软键栏 `⌨` 唤起软键盘时，可见视口变矮，终端宿主随之缩小，触发 fit 并向 PTY 发送 resize；收起键盘时又反向再来一次。

键盘动画期间 100 ms 防抖仍可能放过中间尺寸，全屏 TUI（agent CLI、编辑器等）因此在每次开合时整屏重绘一到多次，产生大量无效输出与闪烁。

现有“锁定尺寸”能避免这一点，但需要用户手动常开，并且会同时冻结宽度、字号和地址栏等其他尺寸变化。

## What Changes

- 触屏设备上，用户通过软键栏 `⌨` 唤起软键盘（直接模式聚焦终端、草稿模式聚焦草稿框）时，终端在聚焦前进入“键盘暂时锁定尺寸”：只改变高度的宿主尺寸变化不再 fit，也不向 PTY 发送 resize。
- 暂时锁定期间沿用“锁定尺寸”的显示方式：保持原网格并默认贴底，最新输出和底部输入行留在键盘上方，被挡住的部分可滚动查看。与手动锁定不同，宽度和字号变化仍会适配。
- 键盘收起（再次点 `⌨`、Android 返回键、输入目标失焦），或唤起后一段时间内始终没有检测到软键盘时，结束暂时锁定，并按当前宿主尺寸适配一次；尺寸与锁定前相同则不产生 resize。
- 暂时锁定期间，宽度变化（旋转、分屏、代码区）、字号变化和手动“锁定尺寸”开关仍按现有规则立即适配。
- 终端菜单新增复选项“键盘弹出时保持尺寸”，默认开启，保存在本机浏览器；关闭后恢复现在的行为：键盘开合时实时适配。
- 不改变观察端行为、服务端协议和手动“锁定尺寸”的语义。

## Capabilities

### New Capabilities

- `terminal-keyboard-size-hold`：触屏设备通过软键栏键盘按钮唤起软键盘时，暂时保持 PTY 行列不变；包括触发条件、显示方式、结束条件、与其他尺寸来源的优先级，以及用户开关。

### Modified Capabilities

无。现有规格没有覆盖软键盘与“锁定尺寸”的终端尺寸规则（`terminal-control` 只约束观察端不改变 PTY 尺寸，本变更不改变它）。

## Impact

- 前端：
  - 新增 `web/src/hooks/keyboard-size-hold.ts`（保持状态机）；`web/src/hooks/terminal.svelte.ts`（仅高度保持的 fit 判定、键盘按钮入口）。
  - `web/src/components/SessionTerminal.svelte`（`⌨` 分派、偏好状态）、`TerminalInputComposer.svelte`（草稿模式键盘入口）、`TerminalHeader.svelte`（菜单开关）。
  - `web/src/pages/Terminal.svelte` 向会话传递键盘状态与视口高度；`TerminalViewport.svelte` 复用锁定样式，不改样式表。
- 测试：新增 `web/test/keyboard-size-hold.test.ts`；`web/test/browser/TerminalTransport.test.ts`、`web/test/browser/TerminalHarness.svelte`、`web/test/e2e/softkeys.spec.ts`。
- 文档：`docs/usage.md` 的移动端段落拆到 `docs/usage/mobile.md` 并补充说明；`docs/user-stories/terminal.md` 中的 US-H06、覆盖索引与核对记录。
- 无后端、REST/WebSocket 协议或依赖变更。
