# Spec Delta

## MODIFIED Requirements

### Requirement: WebSocket 认证不经过 URL

前端 MUST 在 WebSocket 握手中提供两个子协议：`webmux`，以及 `webmux.token.<base64url(UTF-8 Token)>`（无填充）；Token 为空时只提供 `webmux`。服务端 SHALL 只选择 `webmux`，MUST NOT 在响应中回显携带 Token 的子协议。

- WebSocket 请求 URL MUST NOT 携带 Token，服务端 MUST NOT 接受 `?token=` 查询参数。
- 前端 MUST NOT 把 Token 写入 Cookie。原因：浏览器按主机而非端口隔离 Cookie，同一主机任意端口上的其他页面都能读取。
- 系统唯一允许设置的 Cookie 是端口转发凭据 `webmux_forward`。它的值独立于主 Token，只能认证 `/forward/` 网关，MUST NOT 认证 `/api/` 和 `/ws/` 下的任何请求。

#### Scenario: 片段登录后建立 WebSocket

- **WHEN** 用户通过带 `#token=` 的链接登录后，前端连接 `/ws/sessions`
- **THEN** 握手通过 Token 子协议认证，请求 URL 不含 `token`，`document.cookie` 不含 Token，连接成功

#### Scenario: 手动输入 Token 后建立 WebSocket

- **WHEN** 用户在 Token 表单输入有效 Token 并进入会话列表
- **THEN** 后续 WebSocket 握手同样使用 Token 子协议，不使用 query 参数或 Cookie

#### Scenario: 查询参数 Token 被拒绝

- **WHEN** 客户端以 `/ws/sessions?token=<有效 Token>` 握手，且不提供 Token 子协议、Bearer 头或 Cookie
- **THEN** 服务端返回 401

#### Scenario: 转发凭据不能建立 WebSocket

- **WHEN** 客户端只带有效的 `webmux_forward` Cookie 握手 `/ws/sessions`
- **THEN** 服务端返回 401
