# HTTP API：端口转发

端口转发把服务器本机回环端口上的开发服务器（例如 Vite）经 WebMux 现有监听地址转发到浏览器，不新开端口。转发只保存在内存中，服务重启后清空；与会话无关，会话退出后转发仍保留，直到手动移除。

服务端实现：登记、探测与凭据在 `internal/forward`，网关与接口在 `internal/api/forward.go`。网关只注册在 TCP 监听器上，本机 Unix 套接字控制面没有网关。

## 管理接口

全部需要主 Token（`Authorization: Bearer`），并执行常规 Origin 检查。

| 方法   | 路径                    | 说明                                                                           |
| ------ | ----------------------- | ------------------------------------------------------------------------------ |
| GET    | `/api/forwards`         | `Forward[]`，按端口升序                                                        |
| POST   | `/api/forwards`         | `ForwardRequest`，201 返回 `Forward`；非法端口或 WebMux 自身端口 400，重复 409 |
| PATCH  | `/api/forwards/{port}`  | `{label?, options?}`，200 返回 `Forward`；不存在 404                           |
| DELETE | `/api/forwards/{port}`  | 204；不存在 404。之后新请求返回 404，该端口进行中的连接（含 WebSocket）被关闭  |
| POST   | `/api/forwards/tickets` | 201 `{code, expiresAt}`：60 秒内有效、只能用一次的兑换码                       |

```ts
interface ForwardOptions {
  rewriteHost: boolean; // 上游 Host 改为 localhost:{port}
  rewriteOrigin: boolean; // 上游 Origin/Referer 中的网关来源改为 http://localhost:{port}
  forwardedHeaders: boolean; // 添加 X-Forwarded-For/Host/Proto/Prefix
  rewriteResponse: boolean; // 改写 Location、Access-Control-Allow-Origin 与 Set-Cookie
  prefix: "keep" | "strip"; // keep 原样转发 /forward/{port}/...；strip 去掉 /forward/{port}
}
interface Forward {
  port: number;
  path: string; // "/forward/{port}/"
  label: string; // 显示名称，最多 64 个字符，可为空
  session: string; // 检测到该端口的会话 ID，可为空；只作引用
  process: string; // 检测到的进程名，可为空
  options: ForwardOptions;
  status: "listening" | "unreachable"; // 每 3 秒拨号一次本机端口
  createdAt: string; // RFC 3339
}
interface ForwardRequest {
  port: number; // 1–65535
  label?: string;
  session?: string;
  process?: string;
  options?: ForwardOptions; // 省略时全部开启、prefix 为 strip
}
```

转发列表的变化通过 `/ws/sessions` 的 `forwards` 事件推送，见 [WebSocket 接口](websocket.md)。

## 网关 `/forward/{port}/`

- `{port}` 必须是不带前导零的十进制 1–65535，否则 404；已认证请求访问未添加的端口也返回 404，不连接上游。认证在查询转发之前，未认证请求不论端口是否已转发都得到同样的结果。
- `/forward/{port}`（无末尾斜杠）308 到 `/forward/{port}/`，保留查询参数。
- 上游固定为 `127.0.0.1:{port}`，IPv4 连不上时再试 `[::1]:{port}`；不使用环境代理，不接受其他主机。
- 上游无法连接时返回 502 纯文本，说明中包含端口号。
- 请求体与响应体流式转发，`FlushInterval` 为立即刷新，SSE 和分块响应逐条到达；WebSocket 升级透明转发。
- 发往上游的 `Cookie` 删除 `webmux_forward`，其余 Cookie 与 `Authorization` 原样转发。
- 响应删除 `Service-Worker-Allowed`；不使用 WebMux 的 `Referrer-Policy: no-referrer`，保留上游自己的策略；`X-Content-Type-Options: nosniff` 保留。
- 网关执行与其他路径相同的 Host 检查；WebSocket 升级还执行与 `/ws/` 相同的 Origin 检查，不通过返回 403。
- 每次请求都读取最新的转发选项，修改选项只影响之后的新请求。
- 网关不改写 HTML、CSS 或 JS 响应体：默认的 `strip` 模式只适合使用相对路径的应用；使用绝对路径的应用必须改为 `keep` 前缀模式，并把 base 设为 `/forward/{port}/`（Vite `--base /forward/5173/`，Next.js `basePath`）。

## 认证

网关只认 `webmux_forward` Cookie。主 Token 为空（认证关闭）时，网关与 `/api/` 一样不要求认证。

| 情况                            | 结果                                               |
| ------------------------------- | -------------------------------------------------- |
| 有效 Cookie                     | 代理                                               |
| 无有效 Cookie，页面导航（见下） | 303 到 `/forward-auth?next=<URL 编码的路径和查询>` |
| 无有效 Cookie，其他请求         | 401 纯文本                                         |

页面导航指 `GET` 且 `Sec-Fetch-Mode: navigate`。浏览器只向可信来源（HTTPS、localhost）发送 `Sec-Fetch-*`；没有该头时，以 `Accept` 含 `text/html` 判断。

加载 `/forward-auth` 时，服务端检查 `Sec-Fetch-Site`：不是 `same-origin` 或 `none`（包括缺失）时 303 到同一地址并加上 `confirm=1`，页面显示目标地址并等用户点击“打开”再兑换，使其他网站不能驱动浏览器向转发应用发出带凭据的请求。该页面带 `X-Frame-Options: DENY` 与 `frame-ancestors 'none'`。

`webmux_forward` 的值是 32 字节随机数（base64url），服务端只保存 SHA-256 并以常数时间比较；它独立于主 Token，12 小时后过期，服务重启后全部失效。它 MUST NOT 认证 `/api/` 与 `/ws/`：只带该 Cookie 的请求在那里返回 401。

兑换流程：

1. `/forward-auth` 是前端页面。已有主 Token 时，它调用 `POST /api/forwards/tickets` 取得兑换码；没有主 Token 时先显示 Token 登录表单。
2. 页面以顶层表单 `POST /forward/_auth`（`application/x-www-form-urlencoded`）提交 `code` 与 `next`，兑换码不出现在任何 URL 中。
3. 服务端要求请求带 `Origin` 且与 WebMux 同源（同 `/api/` 规则），否则 403；`next` 必须以 `/forward/` 开头，不能含 `//`、反斜杠、换行或协议，否则 400；兑换码未知、已用或过期返回 401。
4. 成功时设置 Cookie 并 303 到 `next`。Cookie 属性：`HttpOnly`、`SameSite=Strict`、`Path=/forward/`、`Max-Age=43200`；经 TLS 或运行时 HTTPS 入口（隧道、Tailscale）访问时加 `Secure`。

## 兼容改写

| 选项               | 开启时                                                                                                                                                                                                                                                                           |
| ------------------ | -------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| `rewriteHost`      | 上游 `Host` 为 `localhost:{port}`，满足 Vite 默认的 `server.allowedHosts`；关闭时为浏览器访问 WebMux 的 Host                                                                                                                                                                     |
| `rewriteOrigin`    | 与网关同主机的 `Origin`、`Referer` 改为 `http://localhost:{port}`，满足 Vite HMR 的 Origin 检查                                                                                                                                                                                  |
| `forwardedHeaders` | 设置 `X-Forwarded-For`、`X-Forwarded-Host`（浏览器访问的 Host）、`X-Forwarded-Proto`、`X-Forwarded-Prefix: /forward/{port}`；浏览器传入的同名头总是丢弃                                                                                                                          |
| `rewriteResponse`  | 指向 `localhost:{port}` 或 `127.0.0.1:{port}` 的 `Location` 改为网关路径（`strip` 模式还给以 `/` 开头的相对 `Location` 加前缀）；同样指向上游的 `Access-Control-Allow-Origin` 改为网关来源；`Set-Cookie` 删除 `Domain`，`Path` 加上 `/forward/{port}` 前缀（已带前缀的保持不变） |

选项关闭时对应的头原样转发。

## 安全边界

- 转发页面与 WebMux 同源，页面脚本能读取 `localStorage` 中的主 Token；只转发自己信任的开发服务器。
- 浏览器按主机而不是端口隔离 Cookie，同一主机其他端口上的服务也能收到 `webmux_forward`；它只能访问网关，不能访问 shell。
- 网关不记录请求体、响应体或 Cookie。
