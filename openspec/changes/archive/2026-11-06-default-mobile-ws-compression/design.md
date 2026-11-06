# Design

## Context

`web/src/ws.ts` 的 `wsCompression()` 读取本机存储 `webmux.wsCompression`，等于 `on` 才在 WebSocket URL 追加 `compress=1`；浏览器总会提议 `permessage-deflate`，服务端只在带参数时协商。

## Goals / Non-Goals

- 目标：手机与平板在未选择时默认压缩；用户选择优先；桌面行为不变。
- 非目标：服务端默认值、按网络类型（Network Information API）自适应、跨设备同步。

## Decisions

### 1. 默认值由主指针类型决定

未存储选择时返回 `matchMedia("(pointer: coarse)").matches`。UA 嗅探不可靠，屏幕宽度会误伤窄窗口桌面；`any-pointer` 会误伤带触摸屏的笔记本（项目中 `hasTouchInput` 用于键盘行为，语义不同）。无 `matchMedia` 的环境（测试、非浏览器）视为关闭。

### 2. 只在存储缺失时使用默认值

存储值 `on`/`off` 直接生效，其余值回落到默认值。不迁移旧数据：旧版本只会写入 `on`/`off`，已明确选择“关”的手机用户保持关闭。

### 3. 不新增存储键与服务端改动

仍使用 `wsCompression` 键，开关组件无需新状态，`setWsCompression` 语义不变。

## Risks / Trade-offs

- 手机默认多占每连接约 1.3 MB 服务端内存与少量 CPU；单用户场景可接受，且可在设置页关闭。
- 手机经本机或局域网访问时压缩收益小；小于 128 字节的消息不压缩，不增加输入延迟。
