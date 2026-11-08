# Spec Delta

## ADDED Requirements

### Requirement: 向导入口与路由

系统 SHALL 提供需要认证的页面 `/settings/remote-access`，作为“远程访问向导”。

- 设置页 SHALL 在公网访问与 Tailscale 两张卡片之前显示向导入口卡片；两张现有卡片 SHALL 提供进入向导的链接。
- 向导页 SHALL 在每一步提供“退出向导”，回到设置页，也 SHALL 提供直达 `/settings/tunnel` 与 `/settings/tailscale` 详细页的链接。
- 向导 MUST NOT 新增服务端接口或持久化字段，所有读写 SHALL 复用 `GET /api/config`、`GET /api/tunnel`、`GET /api/tailscale`、`PUT /api/tunnel/settings`、`PUT /api/tunnel/enabled`、`PUT /api/tailscale/settings` 及其现有的校验与错误。
- 向导的当前步骤 SHALL 只保存在页面内存里，刷新页面后回到第 1 步，但 SHALL 按当前服务端状态给出推荐。

#### Scenario: 从设置页进入

- **WHEN** 用户打开 `/settings`
- **THEN** 页面显示向导入口卡片，点击后进入 `/settings/remote-access` 的第 1 步

#### Scenario: 随时退出到详细页

- **WHEN** 用户在第 2 步点击“打开 Tunnel 详细设置”
- **THEN** 页面跳转到 `/settings/tunnel`，已保存的设置与状态不受影响

#### Scenario: 刷新后回到第一步

- **WHEN** 用户在第 3 步刷新页面
- **THEN** 向导回到第 1 步，且当前隧道或 Tailscale 的真实状态仍被读取并显示

### Requirement: 方式推荐

向导第 1 步 SHALL 并列展示三种方式：Tailscale Serve、Cloudflare Named Tunnel、Cloudflare Quick Tunnel，并 SHALL 对每种方式显示：地址是否固定、谁能访问、是否支持 Web Push、前提条件。

向导 SHALL 根据已读取的状态标记一个推荐项，规则按优先级：

1. 已有处于 `ready` 的入口：推荐“继续使用当前入口”，直接进入第 4 步。
2. Tailscale 后端为 `running` 且 `httpsReady`：推荐 Tailscale Serve。
3. 已保存 Named 凭据（`tokenConfigured` 且 `hostname` 非空）：推荐 Named Tunnel。
4. 其他情况：推荐 Quick Tunnel，并标注“仅适合临时使用”。

推荐只是排序与徽标，用户 SHALL 能选择任何方式；不满足前提的方式 SHALL 保持可选，并说明缺少什么，不得隐藏。

读取某个状态失败时，对应方式 SHALL 显示失败原因，其他方式与向导继续可用，推荐按已读到的状态计算。

向导 MUST NOT 把 Funnel 作为推荐项，也 MUST NOT 在第 1 步直接开启 Funnel。需要 Funnel 的用户 SHALL 通过 Tailscale 详细页完成。

#### Scenario: 检测到 Tailscale

- **WHEN** `GET /api/tailscale` 返回 `backendState = "running"`、`httpsReady = true`，且没有入口处于 `ready`
- **THEN** Tailscale Serve 带“推荐”徽标并排在第一位，Named Tunnel 与 Quick Tunnel 仍可选

#### Scenario: 没有任何前提

- **WHEN** Tailscale 为 `unavailable`，且没有保存 Named 凭据
- **THEN** Quick Tunnel 被推荐并带“临时使用”提示，Tailscale 项说明“未检测到 Tailscale”，仍可选择

#### Scenario: 已经可用

- **WHEN** 隧道状态为 `ready` 且有 `publicUrl`
- **THEN** 向导提示“已有可用入口”，提供“直接获取手机链接”与“重新配置”两个动作

#### Scenario: 某个状态读取失败

- **WHEN** `GET /api/tailscale` 失败，`GET /api/tunnel` 成功
- **THEN** Tailscale 项显示读取失败的原因，向导不整体报错，推荐按隧道状态计算

#### Scenario: 不推荐 Funnel

- **WHEN** 任何状态下展示第 1 步
- **THEN** 推荐徽标不会出现在 Funnel 上，页面不提供一键开启 Funnel 的动作

### Requirement: 在向导内完成配置

用户选择方式后，向导第 2 步 SHALL 复用现有配置表单完成设置，行为与对应详细页一致：

- **Tailscale Serve：** 模式固定为 `serve`，可编辑端口与 provider；保存走 `PUT /api/tailscale/settings`。
- **Named Tunnel：** 输入隧道 Token 与主机名；保存走 `PUT /api/tunnel/settings` 并以 `mode = "named"`，随后 `PUT /api/tunnel/enabled`。向导 SHALL 就地给出获取 Token 与主机名的步骤，不要求用户先阅读外部文档。
- **Quick Tunnel：** 无输入项；保存走 `PUT /api/tunnel/settings` 并以 `mode = "quick"`，随后开启。

当后端的前提不满足时，向导 SHALL 直接说明原因并 MUST NOT 发出会被拒绝的写请求：

- `canEnable = false` 时显示 `enableBlockedReason`，保存按钮不可用。
- 登录 Token 少于 43 字符时（HTTP 409 的前提），说明需要通过 `webmux config init` 或设置强 Token 并重启，向导 MUST NOT 提供在网页内生成或更换登录 Token 的功能。
- 存储不可用（`storageAvailable = false`）时显示 `storageError`，并禁用写入。
- WSL NAT 等 Tailscale 阻塞原因沿用 `tailscale-web-settings` 的指引文案。

配置过程中，向导 SHALL NOT 展示或回显已保存的隧道 Token 与 Tailscale 以外的任何密钥，沿用 `tokenConfigured` 布尔值。

#### Scenario: 选择 Quick Tunnel

- **WHEN** 用户选择 Quick Tunnel，登录 Token 满足 43 字符，点击“开启”
- **THEN** 向导依次发送 `PUT /api/tunnel/settings`（`mode = "quick"`）与 `PUT /api/tunnel/enabled`，并进入第 3 步

#### Scenario: 登录 Token 太短

- **WHEN** `canEnable = false` 且 `enableBlockedReason` 说明登录 Token 不足 43 字符
- **THEN** 第 2 步显示该原因与 `webmux config init` 的提示，开启按钮不可用，且没有发出任何写请求

#### Scenario: Named 凭据校验失败

- **WHEN** 用户提交的主机名是通配符或含协议前缀，服务端返回 400
- **THEN** 向导停留在第 2 步，在对应字段旁显示服务端错误，已填写的内容保留

#### Scenario: 存储不可用

- **WHEN** `storageAvailable = false`
- **THEN** 第 2 步显示存储错误并禁用保存，其他设置与终端功能不受影响

### Requirement: 等待就绪

保存并开启后，向导第 3 步 SHALL 反映入口的真实状态，直到 `ready` 或 `failed`：

- `starting`：显示进度说明，并按固定间隔重新读取状态，页面离开后 SHALL 停止轮询。
- `ready`：自动进入第 4 步。
- `failed`：显示服务端返回的失败原因，提供“返回修改”和“重试”；重试 SHALL 再次请求开启，MUST NOT 自动重试。

向导 MUST NOT 把 `starting` 当成功，也 MUST NOT 在 `failed` 时显示手机链接。

#### Scenario: 隧道就绪

- **WHEN** Quick Tunnel 状态从 `starting` 变为 `ready` 并带 `publicUrl`
- **THEN** 向导自动进入第 4 步并显示该地址

#### Scenario: cloudflared 下载失败

- **WHEN** 状态变为 `failed`，错误为无法下载 cloudflared
- **THEN** 第 3 步显示该错误，提供“返回修改”与“重试”，不显示链接，不再继续轮询

#### Scenario: 离开页面停止轮询

- **WHEN** 用户在 `starting` 期间离开向导页面
- **THEN** 页面不再发出状态读取请求，服务端的启动过程不受影响

### Requirement: 手机访问链接与二维码

向导第 4 步 SHALL 显示当前入口的手机访问方式：

- 入口公开地址（`publicUrl`），可一键复制。
- “显示带登录的链接”动作：点击后才生成 `<publicUrl>#token=<URL 编码的 Token>`，并同时提供复制按钮与二维码。
- 在以触屏为主要输入的设备上（本页面正在手机上打开）SHALL 默认不显示二维码，改为突出复制按钮；设备支持 Web Share 时 SHALL 提供系统分享。

Token 展示边界：

- 带 Token 的链接与二维码 MUST 仅在用户主动点击后生成，并在 60 秒后或离开该步骤时自动收起、释放。
- Token MUST NOT 出现在页面的可见文本、`title`、`aria-label`、`value` 之外的任何位置；链接复制失败时的手动复制输入框是唯一允许显示的位置，沿用现有“复制失败”降级。
- Token MUST NOT 写入新的 `localStorage`/`sessionStorage` 项、Cookie、URL 查询参数、日志或遥测；读取 Token 沿用 `access-link` 的现有存储位置。
- 二维码 MUST 在浏览器内生成，不调用任何外部服务。
- 没有已保存的 Token 时（空 Token 实例），SHALL 只提供不带片段的地址，并说明无需登录 Token。

针对不同入口 SHALL 显示相应说明：

- Quick Tunnel：地址在每次启动时变化，不支持 Web Push，适合临时使用；提供“改用固定入口”的返回动作。
- Tailscale Serve：说明手机需要登录同一个 tailnet。
- Named Tunnel：说明任何拿到地址和 Token 的人都能执行命令，沿用现有风险提示。
- 稳定的 HTTPS 入口（Named、Tailscale）SHALL 提供前往 Web Push 设置的链接。

#### Scenario: 桌面上显示二维码

- **WHEN** 桌面浏览器上的用户在第 4 步点击“显示带登录的链接”
- **THEN** 页面显示该链接的二维码与复制按钮，扫描二维码得到的 URL 以 `#token=` 片段携带 Token，且该片段不会被发送到服务器

#### Scenario: 60 秒后自动收起

- **WHEN** 带 Token 的二维码已显示 60 秒，用户没有操作
- **THEN** 二维码与链接被移除，页面恢复为“显示带登录的链接”按钮

#### Scenario: 在手机上打开向导

- **WHEN** 向导在触屏手机上打开并进入第 4 步
- **THEN** 页面不默认显示二维码，而是突出“复制链接”，支持 Web Share 的浏览器同时显示“分享”

#### Scenario: Quick Tunnel 的限制说明

- **WHEN** 当前入口是 Quick Tunnel
- **THEN** 第 4 步显示“地址会随重启变化、不支持 Web Push”，不显示前往 Web Push 设置的链接

#### Scenario: 复制失败

- **WHEN** 剪贴板写入失败
- **THEN** 页面显示只读输入框承载链接供手动复制，输入框随自动收起一并移除

#### Scenario: 空 Token 实例

- **WHEN** 当前实例没有登录 Token
- **THEN** 第 4 步只显示公开地址，不提供“带登录的链接”，并说明该实例无需 Token

### Requirement: 风险提示不被向导弱化

向导 SHALL 保留现有的公网风险提示，且 MUST NOT 为了简化流程删除或默认折叠它们：

- 选择 Named Tunnel 或 Quick Tunnel 时，在开启前显示“公网入口等同于把 shell 暴露给能拿到地址和 Token 的人”。
- 选择 Tailscale 时，显示 Serve 仅 tailnet 内设备可访问的说明。
- 开启公网入口前，用户 SHALL 明确确认风险；未确认时开启按钮不可用。

#### Scenario: 开启 Quick Tunnel 前确认

- **WHEN** 用户在第 2 步选择 Quick Tunnel
- **THEN** 页面显示公网风险说明与确认项，未勾选确认时“开启”按钮不可用，且不发出写请求

#### Scenario: Tailscale Serve 不要求公网确认

- **WHEN** 用户选择 Tailscale Serve
- **THEN** 页面显示“仅 tailnet 内设备可访问”，不要求公网风险确认

### Requirement: 可访问性与响应式

向导 SHALL 在桌面与手机宽度下可用：

- 步骤指示 SHALL 暴露当前步骤给辅助技术，步骤切换后焦点 SHALL 移到新步骤的标题。
- 状态变化（`starting`、`ready`、`failed`）SHALL 通过 `role="status"` 或 `role="alert"` 播报。
- 二维码 SHALL 有文字替代（“登录二维码，扫描后用手机打开”），且 MUST NOT 把 Token 写入替代文字。
- 手机宽度下不出现横向滚动，主要按钮高度满足触摸目标要求。

#### Scenario: 键盘与读屏

- **WHEN** 用户用键盘完成从第 1 步到第 4 步的全流程
- **THEN** 每次切换后焦点位于新步骤标题，状态变化被播报，全程无需鼠标

#### Scenario: 手机宽度

- **WHEN** 视口宽度为 360 px
- **THEN** 方式卡片纵向排列，页面无横向滚动，二维码在可视范围内完整显示
