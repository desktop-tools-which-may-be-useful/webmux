# Design

## Context

`SessionTerminal` 的根节点是纵向 flex：`TerminalHeader`、`TerminalNotices`（多条 `shrink-0` 的条带）、混合会话连接器提示、`TerminalWorkspace`。条带占用高度，`TerminalWorkspace` 的 `flex-1` 随之伸缩；终端宿主的 `ResizeObserver` 在防抖后 fit 并发送 0x01 resize 帧。

## Goals / Non-Goals

**Goals:**

- 提示显示、关闭、更换内容时终端宿主的宽高不变。

**Non-Goals:**

- 不重做提示的视觉样式、文案或触发条件。
- 不改变 `TerminalNotices` 的属性与行为。

## Decisions

### 1. 终端窗格内的绝对定位悬浮层

在 `SessionTerminal` 的 `terminalPane` 片段外包一层 `relative h-full`，把 `TerminalNotices` 与连接器提示放进其内的 `absolute inset-x-0 top-0 z-30 flex flex-col bg-background` 层。

悬浮层不参与布局，终端宿主尺寸与有无提示无关；没有提示时层高为 0。层只覆盖终端窗格（含聊天视图），不覆盖旁边的代码窗格，因此代码工作区的工具栏不被遮挡。`bg-background` 让半透明的错误条带不透出终端文字。`z-30` 高于查找栏与拖放遮罩的 `z-20`，使提示按钮始终可点。

备选：包裹整个 `TerminalWorkspace`。拒绝原因：全宽层会遮住代码窗格顶部工具栏。

备选：保留文档流并给终端预留固定高度。拒绝原因：浪费空间且无法适应不定数量的提示。

## Risks / Trade-offs

- 悬浮提示遮住终端最上面几行。多数提示可关闭或随状态消失；观察端的“观察中”条带（单行、紧凑）在观察期间常驻，会持续遮住约一行半，接管控制后消失。
- 弹窗与菜单的层级高于 `z-30`，不受影响。
