# Proposal

## Why

终端页面顶部的状态提示（观察中、上传进度、待复制、混合会话提示、错误、进程已退出、连接器安装提示等）原先位于顶栏与终端之间的文档流中。提示出现或点“知道了”关闭时，终端区域随之变高或变矮，触发 fit 并向 PTY 发送 resize，全屏 TUI 因此整屏重绘，产生无用输出与闪烁。

## What Changes

- 终端顶部的所有状态提示改为悬浮在终端区域上方，不再占用文档流高度；显示、关闭或内容变化都不改变终端宿主尺寸，也不向 PTY 发送 resize。
- 提示的内容、顺序、样式和关闭方式不变；悬浮期间会遮住终端最上面几行，关闭后恢复可见。
- 不改变服务端协议、REST/WebSocket 契约和其他尺寸来源（窗口、键盘、锁定尺寸）的规则。

## Capabilities

### New Capabilities

- `terminal-notice-overlay`：终端顶部提示悬浮显示，不改变终端尺寸。

### Modified Capabilities

无。

## Impact

- 前端：`web/src/components/SessionTerminal.svelte` 把 `TerminalNotices` 与连接器安装提示放入高度为 0 的定位容器；`TerminalNotices.svelte` 本身不变。
- 文档：`docs/usage/terminal.md`、`docs/user-stories/terminal.md` 的 US-D05、核对记录。
- 无后端、协议或依赖变更。
