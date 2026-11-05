# Spec Delta

## Purpose

在 WebMux 现有监听地址上提供受认证保护的端口转发网关，把服务器本机回环端口上的开发服务器安全地转发到用户浏览器预览，并提供转发的管理接口与首页管理界面。

## ADDED Requirements

### Requirement: 转发网关路由

系统 SHALL 在 WebMux 现有的 HTTP 监听地址上提供路由 `/forward/{port}/`，不得为转发新开任何监听端口。

- `{port}` MUST 是不带前导零的十进制整数，范围为 1–65535。
- 网关只代理已添加的转发。请求未添加的端口，或 `{port}` 不合法时，SHALL 返回 404。
- 上游地址 MUST 固定为服务器本机回环地址的该端口，不接受任何其他主机。
- 访问 `/forward/{port}`（末尾不带 `/`）时，SHALL 以 308 重定向到 `/forward/{port}/`，查询参数保持不变。

#### Scenario: 代理已添加的端口

- **WHEN** 已添加端口 5173 的转发，且已认证的浏览器请求 `/forward/5173/src/main.ts`
- **THEN** 网关把请求代理到本机 5173 端口，并原样返回上游的状态码、响应头和响应体（兼容改写除外）

#### Scenario: 未添加的端口

- **WHEN** 已认证的浏览器请求 `/forward/8080/`，但 8080 没有添加转发
- **THEN** 网关返回 404，且不连接本机 8080 端口

#### Scenario: 非法端口

- **WHEN** 请求 `/forward/0/`、`/forward/65536/`、`/forward/05173/` 或 `/forward/abc/`
- **THEN** 网关返回 404

#### Scenario: 补全末尾斜杠

- **WHEN** 请求 `/forward/5173?x=1`
- **THEN** 网关返回 308，`Location` 为 `/forward/5173/?x=1`

### Requirement: 转发登记

系统 SHALL 只在服务端内存中保存转发，MUST NOT 写入磁盘。服务重启后转发列表为空。转发与会话无关：创建它的会话退出或被删除后，转发仍然保留，直到用户手动移除。

每个转发包含：

- 端口；
- 可选的显示名称（最多 64 个字符）；
- 可选的来源会话 ID；
- 兼容选项；
- 上游状态；
- 创建时间。

#### Scenario: 重启后清空

- **WHEN** 添加了端口 5173 的转发后重启 WebMux 服务
- **THEN** `GET /api/forwards` 返回空列表，请求 `/forward/5173/` 返回 404

#### Scenario: 会话退出不影响转发

- **WHEN** 端口 5173 的转发来自会话 A，之后会话 A 被删除
- **THEN** 转发仍在列表中，开发服务器还在运行时仍然可以访问

### Requirement: 转发管理接口

系统 SHALL 提供以下接口，全部需要主 Token 认证，并执行现有的 Origin 检查：

- `GET /api/forwards`：列出转发；
- `POST /api/forwards`：添加转发；
- `PATCH /api/forwards/{port}`：修改名称和兼容选项；
- `DELETE /api/forwards/{port}`：移除转发。

添加时，端口不合法或等于 WebMux 自身的监听端口 SHALL 返回 400；该端口已有转发时 SHALL 返回 409。修改或移除不存在的转发 SHALL 返回 404。

#### Scenario: 添加转发

- **WHEN** 已认证客户端以 `{"port":5173}` 调用 `POST /api/forwards`
- **THEN** 返回 201 和该转发，兼容选项为默认值，访问路径为 `/forward/5173/`

#### Scenario: 拒绝转发 WebMux 自身端口

- **WHEN** WebMux 监听 7681 端口，客户端以 `{"port":7681}` 添加转发
- **THEN** 返回 400，转发列表不变

#### Scenario: 重复添加

- **WHEN** 端口 5173 已有转发，客户端再次添加 5173
- **THEN** 返回 409，原转发的选项保持不变

#### Scenario: 移除后立即失效

- **WHEN** 客户端调用 `DELETE /api/forwards/5173`
- **THEN** 之后对 `/forward/5173/` 的新请求返回 404，正在进行的该端口 WebSocket 连接被关闭

#### Scenario: 未认证管理请求

- **WHEN** 请求 `POST /api/forwards` 时不带主 Token，只带有效的 `webmux_forward` Cookie
- **THEN** 返回 401

### Requirement: 转发列表实时同步

`/ws/sessions` SHALL 在初始快照中包含当前的转发列表。转发被添加、修改、移除，或上游状态变化时，SHALL 推送 `forwards` 事件，内容为完整的转发列表。前端 MUST 先用 Zod 校验事件，再更新状态。

#### Scenario: 另一台设备添加转发

- **WHEN** 手机和桌面同时打开 WebMux，用户在桌面添加端口 5173 的转发
- **THEN** 手机上的转发按钮计数和对话框列表在 1 秒内更新

### Requirement: 上游状态

每个转发 SHALL 带有上游状态：`listening` 表示本机端口可以连接，`unreachable` 表示无法连接。端口开始或停止监听后，状态 SHALL 在 5 秒内更新。上游无法连接时，网关请求 SHALL 返回 502 和一个简短的纯文本说明，说明中包含端口号。

#### Scenario: 开发服务器停止

- **WHEN** 端口 5173 已转发，用户在终端按 Ctrl+C 停止 Vite
- **THEN** 5 秒内转发状态变为 `unreachable`，访问 `/forward/5173/` 返回 502

### Requirement: 转发凭据 Cookie

网关 SHALL 通过名为 `webmux_forward` 的 Cookie 认证，Cookie 的值是服务端内存中随机生成的转发凭据，至少 256 位，MUST NOT 是主 Token 或由主 Token 推导得到。

- Cookie 属性 MUST 为 `HttpOnly`、`SameSite=Strict`、`Path=/forward/`、`Max-Age=43200`；通过 HTTPS 访问时（包括公网来源）MUST 加 `Secure`。
- 凭据 12 小时后过期，服务重启后全部失效。
- 该 Cookie MUST NOT 认证 `/api/` 和 `/ws/` 下的任何请求。
- 主 Token 为空（认证关闭）时，网关与 `/api/` 一样不要求认证。

#### Scenario: 凭据访问网关

- **WHEN** 浏览器带有效的 `webmux_forward` Cookie 请求 `/forward/5173/`
- **THEN** 网关正常代理，不要求 Bearer 头

#### Scenario: 凭据不能访问 API

- **WHEN** 请求 `GET /api/sessions` 时只带有效的 `webmux_forward` Cookie
- **THEN** 返回 401

#### Scenario: 重启后凭据失效

- **WHEN** 服务重启后，浏览器带旧的 `webmux_forward` Cookie 发起子资源请求
- **THEN** 网关返回 401

### Requirement: 兑换转发凭据

前端 SHALL 用主 Token 调用 `POST /api/forwards/tickets` 获取一次性兑换码，兑换码有效期 60 秒，只能使用一次。随后前端 SHALL 以顶层表单 `POST /forward/_auth` 提交兑换码和目标路径 `next`。兑换码 MUST NOT 出现在任何 URL 中。

兑换端点 MUST 校验 Origin 与 WebMux 同源，并且 `next` 必须以 `/forward/` 开头。校验通过后，SHALL 设置 Cookie 并以 303 跳转到 `next`。

#### Scenario: 兑换成功

- **WHEN** 前端在 60 秒内提交有效兑换码，`next=/forward/5173/`
- **THEN** 响应设置 `webmux_forward` Cookie 并以 303 跳转到 `/forward/5173/`，浏览器地址栏和历史记录中都不含兑换码

#### Scenario: 兑换码只能用一次

- **WHEN** 同一兑换码被提交第二次
- **THEN** 返回 401，不设置 Cookie

#### Scenario: 拒绝站外跳转

- **WHEN** 提交有效兑换码，但 `next=https://evil.example/` 或 `next=//evil.example/`
- **THEN** 返回 400，不设置 Cookie

#### Scenario: 拒绝跨站提交

- **WHEN** 来自其他站点的页面向 `/forward/_auth` 提交表单
- **THEN** 返回 403，不设置 Cookie

### Requirement: 未认证访问自动认证

网关收到没有有效凭据的请求时，SHALL 先认证再查询转发，因此未认证请求无法区分端口是否已转发：

- 如果是浏览器顶层导航（`GET` 且 `Sec-Fetch-Mode: navigate`；浏览器不发送 `Sec-Fetch-Mode` 时，以 `Accept` 含 `text/html` 判断），SHALL 以 303 跳转到 `/forward-auth?next=<URL 编码的原路径和查询参数>`；
- 其他请求 SHALL 返回 401。

`/forward-auth` 是前端页面：已有主 Token 时自动完成兑换并回到原地址；没有主 Token 时先显示现有的 Token 登录表单，登录后继续兑换。

加载 `/forward-auth` 的请求的 `Sec-Fetch-Site` 不是 `same-origin` 或 `none`（包括缺失）时，服务端 SHALL 以 303 加上 `confirm=1` 参数，页面 MUST 等用户点击“打开”后才兑换，使其他网站不能驱动浏览器向转发应用发出带凭据的请求。该页面 MUST NOT 允许被嵌入框架。

#### Scenario: 直接打开转发链接

- **WHEN** 已登录 WebMux 的浏览器在新标签页打开从对话框复制的 `https://host/forward/5173/?page=2`，此时还没有 Cookie
- **THEN** 浏览器经过 `/forward-auth` 后回到 `/forward/5173/?page=2`，页面正常显示，用户无需任何操作

#### Scenario: 其他网站发起的打开需要确认

- **WHEN** 其他网站的页面让浏览器导航到 `/forward/5173/`，此时还没有 Cookie
- **THEN** `/forward-auth` 显示该转发地址和“打开”按钮，用户点击前不兑换凭据、不向开发服务器发出请求

#### Scenario: 子资源不跳转

- **WHEN** 页面脚本在没有有效 Cookie 时请求 `/forward/5173/api/data`
- **THEN** 网关返回 401，不跳转

#### Scenario: 不暴露已转发的端口

- **WHEN** 未认证的请求分别访问已转发的 `/forward/5173/x` 和未转发的 `/forward/1/x`
- **THEN** 两者都返回 401

### Requirement: 网关跨站防护

网关处理 WebSocket 升级请求时 SHALL 执行与 `/ws/` 相同的 Origin 检查，不通过时返回 403。网关 MUST 执行现有的 Host 检查。

#### Scenario: 跨站 WebSocket 被拒绝

- **WHEN** 来自 `https://evil.example` 的页面向 `/forward/5173/` 发起 WebSocket 升级
- **THEN** 网关返回 403，不连接上游

#### Scenario: 同源 HMR 连接

- **WHEN** 转发页面中的 Vite 客户端以同源 Origin 向 `/forward/5173/` 发起 WebSocket 升级
- **THEN** 网关建立双向转发，HMR 更新能到达页面

### Requirement: 请求转发

网关 SHALL 流式转发请求体和响应体，不缓冲整个响应。`text/event-stream` 和分块响应 MUST 立即刷新。网关 SHALL 透明转发 WebSocket 升级。

网关 MUST 遵守以下限制：

- 发往上游的请求中 MUST 删除 `webmux_forward` Cookie，其他 Cookie 和 `Authorization` 头原样转发；
- MUST 删除上游响应中的 `Service-Worker-Allowed` 头，使转发页面注册的 Service Worker 作用域不超出 `/forward/{port}/`；
- MUST NOT 记录请求体、响应体或 Cookie。

#### Scenario: 不泄露转发凭据

- **WHEN** 浏览器带 `webmux_forward` 和 `session=abc` 两个 Cookie 请求网关
- **THEN** 上游收到的 `Cookie` 头只有 `session=abc`

#### Scenario: 流式响应

- **WHEN** 上游以 `text/event-stream` 每秒推送一个事件
- **THEN** 浏览器每秒都能收到事件，不需要等响应结束

### Requirement: 开发服务器兼容改写

每个转发 SHALL 提供以下兼容选项，新建转发时全部默认开启：

- `rewriteHost`：上游 `Host` 改写为 `localhost:{port}`；
- `rewriteOrigin`：上游 `Origin` 和 `Referer` 中的网关来源改写为 `http://localhost:{port}`；
- `forwardedHeaders`：添加 `X-Forwarded-Host`、`X-Forwarded-Proto` 和 `X-Forwarded-Prefix`；
- `rewriteResponse`：把响应中指向 `localhost:{port}` 或 `127.0.0.1:{port}` 的 `Location` 和 `Access-Control-Allow-Origin` 改写回网关地址；把 `Set-Cookie` 的 `Path` 加上转发前缀，并删除 `Domain`。

选项关闭时，相应的头 MUST 原样转发。

#### Scenario: Vite 默认可用

- **WHEN** Vite 6 以默认 `server.allowedHosts` 和 `--base /forward/5173/` 运行，用户用默认选项转发 5173 端口，并通过 Tailscale 主机名访问
- **THEN** 页面和 HMR WebSocket 都能正常工作，Vite 不返回 “Blocked request”

#### Scenario: 改写重定向

- **WHEN** 上游返回 `Location: http://localhost:5173/login`，且路径前缀模式为“去除”
- **THEN** 浏览器收到的 `Location` 为 `/forward/5173/login`

#### Scenario: 关闭 Host 改写

- **WHEN** 转发关闭了 `rewriteHost`，浏览器通过 `host.example:7681` 访问
- **THEN** 上游收到的 `Host` 为 `host.example:7681`

### Requirement: 路径前缀模式

每个转发 SHALL 提供路径前缀模式，新建时默认为 `keep`：

- `keep`：原样转发 `/forward/{port}/...`，适用于已把 base 设为 `/forward/{port}/` 的开发服务器（如 Vite `--base`、Next.js `basePath`）；
- `strip`：去掉 `/forward/{port}` 前缀后再转发，适用于只使用相对路径的服务。

网关 MUST NOT 改写 HTML、CSS 或 JavaScript 响应体中的路径。

#### Scenario: 保留前缀

- **WHEN** 端口 5173 的模式为 `keep`，浏览器请求 `/forward/5173/@vite/client`
- **THEN** 上游收到的路径为 `/forward/5173/@vite/client`

#### Scenario: 去除前缀

- **WHEN** 端口 8000 的模式为 `strip`，浏览器请求 `/forward/8000/docs/index.html`
- **THEN** 上游收到的路径为 `/docs/index.html`

### Requirement: 首页端口转发入口

首页顶栏 SHALL 在“新建会话”按钮和设置按钮之间显示一个“端口转发”纯图标按钮，带 `aria-label` 和可见提示。有转发时，按钮上显示转发数量。点击后打开“端口转发”对话框，窄屏（`max-sm`）显示为底部抽屉。

#### Scenario: 显示转发数量

- **WHEN** 当前有 2 个转发
- **THEN** 顶栏按钮显示数字 2，可访问名称为“端口转发（2 个）”

#### Scenario: 手机打开

- **WHEN** 在 390×844 视口点击“端口转发”按钮
- **THEN** 对话框以底部抽屉打开，顶栏保持单行

### Requirement: 转发对话框

对话框 SHALL 包含三部分：添加转发表单、转发列表、检测到但未转发的端口。

- 每个转发行显示端口、名称（或来源会话名）、上游状态（圆点加文字）和访问路径，并提供四个操作：打开、复制链接、设置、移除。
- “打开”在新标签页打开 `/forward/{port}/`。“复制链接”复制带当前来源的完整地址。
- 移除转发不需要二次确认。
- 没有转发时显示空状态说明。

#### Scenario: 添加并打开

- **WHEN** 用户在添加表单输入 5173 并提交
- **THEN** 列表出现 5173 的转发，状态为“可访问”或“无法连接”；点击“打开”后新标签页显示开发服务器页面

#### Scenario: 转发检测到的端口

- **WHEN** 会话 “web” 正在监听 5173 但未转发
- **THEN** “检测到的端口”中显示 `5173 · node · web`，点击“转发”后它移到转发列表，来源会话为 “web”

#### Scenario: 修改兼容选项

- **WHEN** 用户在某个转发的设置中把路径前缀模式改为“去除”并保存
- **THEN** 之后发往该端口的新请求按 `strip` 模式转发，正在进行的连接不受影响

#### Scenario: 输入无效端口

- **WHEN** 用户在添加表单输入 7681（WebMux 自身端口）或 70000
- **THEN** 表单内联显示错误说明，不关闭对话框
