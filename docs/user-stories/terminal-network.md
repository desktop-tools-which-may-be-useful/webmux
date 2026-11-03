# 网络压缩用户故事

本页延续手机、平板与触摸操作组（H 组）的 US-H10–US-H11，描述移动网络等慢速链路上的实时连接与页面加载压缩。
完整索引见[终端、输入与移动端](terminal.md)。

- **US-H10 压缩实时连接**：作为在移动网络等慢速链路上使用终端的使用者，我希望实时连接压缩在手机上默认开启，并可按设备调整，以便减少回放与输出流量而不拖慢输入。
  验收要点：
  - 设置页“网络”分区提供开关，只保存在当前浏览器（`webmux.wsCompression`）；未选择过时，主指针为粗指针（`(pointer: coarse)`，手机与平板）的设备默认开启，其余设备默认关闭，显式选择优先于默认值；
  - 开启后列表、终端、聊天与设置 WebSocket 均以 `compress=1` 重连，服务端协商 `permessage-deflate`（上下文复用），未带参数时不协商，即使浏览器总是提议该扩展；
  - 切换立即重连已打开的连接并按 offset 续传；
  - 小于 128 字节的消息不压缩；
  - 认证与 Origin 校验不变。
- **US-H11 压缩加载应用**：作为在移动网络上打开 WebMux 的使用者，我希望页面资源以最小体积传输且服务端不增加延迟，以便更快加载应用。
  验收要点：
  - 构建时为 1 KiB 及以上的 HTML、JS、CSS、SVG、manifest、文本与 WebAssembly 生成 brotli（质量 11）和 gzip（级别 9）版本，内嵌二进制只保存这两个版本；
  - 按 `Accept-Encoding` 质量值选择 br、gzip 或原文，同等质量优先 br，响应带 `Vary: Accept-Encoding`；
  - 服务时不实时压缩，仅在客户端拒绝两种编码时一次性解出原文；
  - 压缩版本不能按 `.br`/`.gz` 路径直接访问；
  - WOFF2 与 PNG 本身已压缩，按原样提供。

## 取舍

- 实时连接压缩在手机与平板上默认开启：这类设备常走移动网络，带宽是瓶颈；桌面默认关闭，因为本机与局域网访问时压缩只增加 CPU 与每连接约 1.3 MB 服务端内存。默认值只在用户未选择时生效，选择一次后固定。
- 以主指针是否为粗指针判断设备，而不是 UA 或屏幕宽度：窄窗口的桌面浏览器与带触摸屏的笔记本（主指针为鼠标）不受影响。
- 开关按设备保存而非跨设备同步：手机走移动网络、桌面走局域网时可分别选择。
- 静态资源压缩没有开关：压缩在构建期完成，服务时只返回已有字节，不增加首个请求的延迟。
- 同时保留 brotli 与 gzip：Chrome 在非本机的纯 HTTP 地址上不提议 `br`，经 VPN 的 HTTP 访问仍需 gzip。

## 代码依据

- [实时连接压缩开关](../../web/src/components/NetworkSettings.svelte)
- [连接与重连](../../web/src/ws.ts)
- [握手协商](../../internal/api/router.go)
- [协商用例](../../internal/api/compression_test.go)
- [构建期预压缩](../../web/scripts/embed-assets.ts)
- [静态资源协商](../../internal/api/static.go)
- [静态资源用例](../../internal/api/static_test.go)
- [协议说明](../protocol/websocket.md#压缩)

## 验证范围

Go 用例覆盖握手协商与编码选择；隔离实例中用 Chromium 核对了开关、重连 URL 与 `Sec-WebSocket-Extensions` 响应头。
未做真机移动网络测速。
