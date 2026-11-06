# Proposal

## Why

手机多走移动网络，实时连接（终端输出、回放、会话列表）流量占大头。服务端已支持 `permessage-deflate`，但前端设置页开关默认关闭，手机用户需要手动找到“网络”分区开启，实际几乎无人使用。

## What Changes

- 前端在用户未选择过时，按设备类型决定“压缩实时连接”的默认值：主指针为粗指针（`(pointer: coarse)`，手机与平板）默认开启，其余设备默认关闭。
- 用户一旦在设置页切换开关，选择写入本机存储并优先于默认值，之后不再随设备特征变化。
- 设置页开关显示当前生效值，说明文字注明手机与平板默认开启。
- 不改变服务端协商（仍由 `compress=1` 表达意图）、协议、认证与 Origin 校验。

## Capabilities

### New Capabilities

- `websocket-compression`：实时连接压缩的客户端默认值、用户覆盖与服务端按需协商。

### Modified Capabilities

无。现有规格没有覆盖实时连接压缩。

## Impact

- 前端：`web/src/ws.ts`（`wsCompression` 默认值）、`web/src/components/NetworkSettings.svelte`（说明文字）、`web/test/ws.test.ts`。
- 文档：`docs/user-stories/terminal-network.md`（US-H10）、`docs/protocol/websocket.md`、`docs/user-stories/coverage.md`、核对记录。
- 无后端、REST/WebSocket 协议或依赖变更。
