# Windows（WSL）部署

WebMux 不原生运行在 Windows 上，而是运行在 WSL 2 发行版中，由托盘程序 `webmux-tray.exe` 在 Windows 通知区域启动、监控和停止。托盘只是 WebMux 的客户端：配置解析、链接推导和停止都由 WSL 中的 `webmux` 完成。

## 前置条件

- Windows 10 22H2 及以上或 Windows 11（amd64 / arm64），无需管理员权限。
- 从 Microsoft Store 安装的 WSL 2：托盘依赖 `wsl.exe --version`、`--exec`、`--cd` 与 `WSL_UTF8`，系统自带的旧版 WSL 不支持。在 PowerShell 中执行 `wsl --update` 可升级。
- 至少一个已安装的发行版。`wsl.exe --version` 失败或没有发行版时，托盘图标显示异常，状态行提示“未检测到 WSL 发行版”。

## 在发行版中安装 webmux

最简单的方式是让托盘安装：WSL 中的 `webmux` 缺失、缺少本机 API、控制契约过旧或版本低于托盘时，菜单出现“安装 / 升级 WSL 中的 webmux”。

安装过程：

1. 托盘在 Windows 侧取得与托盘同版本、与发行版架构（`uname -m`）匹配的 Linux 二进制：
   - `%LOCALAPPDATA%\WebMux\downloads\` 中已有摘要匹配的文件时直接使用，不联网；安装程序会把同架构的服务端预置在这里。
   - 否则从 GitHub 下载；GitHub 无响应时依次改用 `ghfast.top`、`gh-proxy.com` 镜像。
2. 只有 SHA-256 与发布构建时写入托盘的摘要一致才会使用。
3. 再以 `install -D -m 0755` 放到托盘设置中的 `webmux` 路径。

限制：

- 缺失时托盘从不在后台安装；WSL 中的 webmux 较旧或契约过旧、托盘为发布构建且开启“自动更新”时，检查后自动升级，失败只在状态行提示，见[自动更新](updates.md#windows-托盘)。
- 要求发行版能通过 `/mnt/<盘符>` 访问 Windows 文件（`/etc/wsl.conf` 未关闭 automount）。
- 本地 `make tray` 构建的托盘不带摘要，不提供安装。
- WebMux 运行中升级时旧进程继续运行，重启后生效。

托盘以 `webmux version --json` 检查 WSL 中的 `webmux`，时机为托盘启动、切换发行版或路径、安装后，以及处于缺失或不兼容状态时打开菜单。兼容只看控制契约版本，版本对应只比较版本号，`dev` 等非发布版本不比较。

结果只显示在菜单状态行，不发通知：

- 两侧版本不同时标出较旧的一方；WSL 中的版本较新时，安装版托盘提供“更新托盘”（开启自动更新时自动检查一次），便携版提供“下载托盘 <版本>”，打开该版本的 Release 页面。
- 契约不兼容时仍可启动、停止和重启：托盘以 Windows 侧能否访问本机链接判断运行状态，显示“运行中（不兼容，功能受限）”，会话数、隧道等依赖状态的功能不可用。
- 正在运行的实例版本与已安装文件不同时，状态行提示重启后生效。

也可以在发行版的终端中手动安装到 `~/.local/bin/webmux`（托盘默认路径），并生成带随机 token 的配置：

```sh
install -m 755 webmux-linux-amd64 ~/.local/bin/webmux # arm64 设备用 webmux-linux-arm64
webmux config init
```

也可以跳过 `config init`：WSL 中的 `serve` 首次启动同样会在默认路径生成配置。

- 托盘菜单的“初始化配置（生成 Token）”调用同一个 `config init`，只在配置文件不存在时提供该入口，从不改写已有配置；修改配置请用“打开 WebMux 配置文件”。
- 缺少 webmux 时托盘停在错误状态且不定时重试；手动安装后重新打开托盘菜单即恢复。

## 安装托盘

推荐使用安装程序：从 Release 下载与 Windows 架构一致的 `webmux-setup-windows-amd64.exe` 或 `webmux-setup-windows-arm64.exe` 并运行。

- 只为当前用户安装，不需要管理员；唯一的管理员确认来自下面可选的防火墙规则。默认目录为 `%LOCALAPPDATA%\Programs\WebMux\`，可在安装界面更改，之后的升级沿用该目录。
- 安装 `webmux-tray.exe`，创建开始菜单“WebMux”，并在“应用和功能”中登记卸载项。
- “安装选项”页的“开机后自动启动 WebMux（登录 Windows 时启动托盘）”默认勾选，写入与托盘菜单“登录时自动启动”相同的启动项；它在登录 Windows 后生效，不会在登录前启动。
- “安装选项”页的“允许其他设备通过托盘转发访问 WebMux（添加 Windows 防火墙规则）”默认勾选：安装完成后弹出一次管理员确认，为安装目录中的托盘添加入站 TCP 允许规则，只放行专用网络与域网络，不放行公用网络。之后开启转发或托盘自更新都不会再出现防火墙提示。
  - 取消确认或添加失败时安装照常完成；之后可以重新运行安装程序，或在命令行运行 `webmux-tray.exe firewall allow`。
  - 不使用托盘转发可以取消勾选。之后再次运行安装程序时，该选项默认沿用上次的选择；取消勾选会弹出管理员确认并删除之前添加的规则，改换安装目录时旧目录的规则也会被删除。
- 安装包内置同架构的 Linux 服务端，放在托盘的下载缓存中；安装程序本身不访问 WSL，首次使用仍在托盘菜单中选择“安装 WSL 中的 webmux”。
- 架构不符时安装程序提示应下载的安装包并退出，不写任何文件。
- 安装程序未签名，Windows SmartScreen 可能拦截，选择“更多信息 → 仍要运行”。

**升级**：直接运行新版本安装程序。它先让正在运行的托盘按“仅退出托盘”退出（WebMux 与会话不受影响，托盘转发会中断，新托盘启动后自动恢复），替换文件后再启动新托盘。

- 开启“自动更新”的安装版托盘会自己下载、校验并以 `/S` 运行新版本安装程序，见[自动更新](updates.md#windows-托盘)。
- 安装目录中的托盘在 `quit` 等待 10 秒仍未退出时，会再被强制结束（`--kill-path` 路径匹配时）。
- 其他位置运行的旧托盘（例如手工存放的 exe）不会被结束：安装程序提示先从菜单退出，然后点击“重试”。

**卸载**：在“应用和功能”中卸载 WebMux。卸载会关闭托盘，删除程序、快捷方式、卸载项和预置的服务端；登录启动项仅在指向本次安装的托盘时删除。安装时添加过防火墙规则的，卸载会再弹出一次管理员确认并删除该规则。

- 保留托盘设置（`%APPDATA%\WebMux`）、托盘日志（`%LOCALAPPDATA%\WebMux`）和 WSL 中的 webmux。
- 不停止 WebMux：正在运行的实例继续在 WSL 中运行，需要时在 WSL 中执行 `webmux stop`。

静默安装与卸载使用 `/S`：首次安装取默认值（含登录启动，但不添加防火墙规则）；覆盖安装沿用原安装目录，并保持登录启动项与已有防火墙规则现状。安装失败时总会恢复升级前的 `webmux-tray.exe`。退出码：

- 0：成功；
- 2：架构不符（不写任何文件），或没有关闭任何托盘时安装失败（不启动托盘）；
- 3：有无法关闭的托盘在运行（不写任何文件）；
- 4：关闭运行中的托盘后安装失败，安装程序重新启动原托盘。

```bat
webmux-setup-windows-amd64.exe /S
"%LOCALAPPDATA%\Programs\WebMux\uninstall.exe" /S
```

也可以不用安装程序：把 `webmux-tray-windows-amd64.exe`（或 `-arm64.exe`）放到固定位置后双击运行。

- 同一 Windows 用户只运行一个实例，再次启动只会让已有实例弹出状态通知。
- 勾选菜单中的“登录时自动启动”会在 `HKCU\Software\Microsoft\Windows\CurrentVersion\Run` 中登记当前 exe 路径；移动 exe 后复选项变为未勾选，重新勾选即可。

`webmux-tray.exe quit` 请求当前用户正在运行的托盘按“仅退出托盘”退出，不弹出询问，最多等待 10 秒，供安装程序和脚本使用。

- 退出码：0 表示运行中的托盘已退出，1 表示仍有托盘在运行（例如不支持该命令的旧版本），2 表示参数错误，3 表示没有运行中的托盘。
- `--kill-path <路径>`：等待超时后结束当前用户从该路径运行的托盘，其他位置的托盘不受影响。

`webmux-tray.exe firewall allow|remove` 为该托盘路径添加或删除转发用的防火墙规则（名称以 `WebMux tray forwarding` 开头），供安装程序调用，也可手工运行。`remove` 可以跟一个托盘的绝对路径，删除该路径的规则，安装程序用它删除旧安装目录的规则。

- 规则已是目标状态时直接返回，不弹出管理员确认。
- 退出码：0 已是目标状态，1 失败，2 参数错误，3 取消了管理员确认。

托盘设置保存在 `%APPDATA%\WebMux\tray.json`（“编辑托盘设置”用记事本打开，保存后 2 秒内自动生效；文件无效时托盘以默认值运行并在状态行提示，不会覆盖该文件）：

| 字段          | 默认值                             | 说明                                                   |
| ------------- | ---------------------------------- | ------------------------------------------------------ |
| `distro`      | 空（系统默认发行版）               | 目标发行版，也可在“发行版”子菜单中选择                 |
| `mode`        | `auto`                             | 启动方式：`auto` / `tray`（托盘托管）/ `systemd`       |
| `webmux`      | `~/.local/bin/webmux`              | WSL 中的可执行文件                                     |
| `config`      | 空（WebMux 默认配置路径）          | WSL 中的配置文件，绝对路径或 `~/` 开头                 |
| `logFile`     | `~/.local/state/webmux/webmux.log` | 托盘托管方式下的服务日志                               |
| `startWebMux` | `true`                             | 托盘启动时自动启动 WebMux                              |
| `autoRestart` | `true`                             | 托盘托管方式下意外退出后自动重启（10 分钟内最多 5 次） |
| `forward`     | `[]`                               | 已勾选的转发地址，由“远程访问”子菜单维护               |

路径类设置在下次启动 WebMux 时生效；切换发行版或启动方式会先停止由托盘管理的实例（有会话时先确认）。托盘日志位于 `%LOCALAPPDATA%\WebMux\tray.log`（1 MiB 轮转），不记录 token。

## 两种启动方式

- **托盘托管**：托盘在发行版中以参数数组（不经 shell）运行 `<webmux> serve --log-file <logFile> [--config <config>]`，WebMux 把启动方记为 `windows-tray`；该进程独立于托盘，并保持发行版存活。意外退出时托盘通知并按退避重启。
- **systemd 用户服务**：发行版启用了 systemd 且已按[部署指南的 systemd 用户服务](services.md#systemd-用户服务linux)安装 `webmux.service` 时使用。
  - 托盘通过 `systemctl --user start|stop|restart webmux` 管理该单元，重启策略交给单元自身。
  - 托盘另外维持一个 `wsl.exe --exec sleep infinity` 保活进程，避免发行版因没有 WSL 客户端而被关闭，停止 WebMux 时一并结束。

- **自动**（默认）：每次启动前检测，满足 systemd 条件时用 systemd，否则托盘托管。

在 WSL 终端中手动运行的实例、或启动方与当前方式不符的实例显示为“外部实例”：托盘只提供访问类操作，不会停止或重启它。“仅退出托盘”后 WebMux 继续运行，重新打开托盘即恢复控制；Windows 注销或关机时托盘直接退出。

## localhost 转发诊断

托盘从 Windows 侧直接请求 `http://127.0.0.1:<端口>/api/health`（不带 token、不经系统代理）。WebMux 在 WSL 中健康但 Windows 无法访问时，图标显示异常，状态行提示检查 WSL 的 `localhostForwarding` / `networkingMode`。

常见原因：

- `%USERPROFILE%\.wslconfig` 中设置了 `localhostForwarding=false`。
- VPN 或安全软件劫持了回环地址。
- WebMux 只监听了 WSL 内部地址。

修改 `.wslconfig` 后执行 `wsl --shutdown` 使其生效（会结束所有会话）。

## 远程访问

NAT 网络模式（WSL 默认）下，局域网与 Windows 上 Tailscale 的流量不会进入 WSL，手机无法直接访问 WebMux。三种途径，按推荐顺序：

1. **mirrored 网络模式**：在 `.wslconfig` 的 `[wsl2]` 下设置 `networkingMode=mirrored`，WSL 与 Windows 共享网卡，按[部署指南的手机访问](remote-access.md#手机访问tailscale)配置即可。托盘的“远程访问”子菜单会显示“无需转发（镜像网络模式）”。
2. **在 WSL 内安装 Tailscale**：发行版获得自己的 Tailscale 地址，WebMux 直接监听该地址；状态报告了远程链接时托盘同样提示无需转发。
3. **托盘转发**：在“远程访问”子菜单中勾选 Windows 主机的地址（Tailscale 地址置顶），托盘在该地址与 WebMux 相同的端口上监听，并把 HTTP 与 WebSocket 原样转给本机链接。
   - 转发默认关闭，只在 WebMux 运行且健康、配置了 token、Windows 侧可达时启用；条件不满足时暂停，恢复后自动继续。
   - 只接受 `Host` 与 `Origin` 恰为该地址的请求，认证仍由 WebMux 完成。
   - 勾选的地址保存在 `tray.json` 中，下次启动自动恢复；端口被占用时该条目显示失败原因。
   - 转发随托盘退出而停止，需要常驻远程访问时请改用前两种途径。

首次在非回环地址上监听时，若没有适用的防火墙规则，Windows 防火墙会弹出授权提示（允许需要管理员确认）。安装版可以在安装时添加规则（见[安装托盘](#安装托盘)），托盘运行时本身不修改防火墙。只为可信网络（如 Tailscale）放行。

- 规则只适用于专用网络与域网络；当前网络被识别为公用网络时仍按 Windows 原生提示处理，可在 Windows 网络设置中把可信网络改为专用网络。
- 以前关掉过 Windows 的提示时，系统会留下名为 `webmux-tray.exe` 的阻止规则，它优先于允许规则；需要在“高级安全 Windows Defender 防火墙”的入站规则中删除它。

## 公网隧道

WebMux 运行时，菜单中的“公网隧道（Cloudflare）”在不重启、不影响会话的情况下开启或关闭 Cloudflare Tunnel。

- 新服务按持久化的期望开关勾选（失败但期望开启时仍勾选），开关跨重启保留；命令行覆盖只影响本次进程。
- 每次开启前托盘确认风险，要求已有至少 43 字符的 WebMux Token。
- 新增“打开公网访问设置…”进入网页配置；旧服务保留原临时开关，不显示不支持的网页入口。
- 就绪后可复制带 Token 的公网链接；隧道断开不自动重开。

隧道项下方的“手机推送”显示前置条件与设备数；“手机推送设置说明…”引导到网页“设置 → 公网访问”配置 Named Tunnel，再从固定 HTTPS 地址开启手机推送。托盘不修改配置文件。

## Token 与剪贴板

托盘不在 Windows 侧保存 token，只在复制时当场读取：

- “复制 Token”：点击时才经本机 API 从 WSL 读取，写入剪贴板后立即丢弃。剪贴板内容标记为不进入 Windows 剪贴板历史（Win+V）与跨设备云剪贴板；60 秒后若剪贴板仍是该 token 则自动清空，期间复制了其他内容则保持不变。
- 所有“复制…链接”：配置了 token 时同样当场读取 token，复制 `<链接>/#token=<token>`（片段不会发送到服务器，打开即登录），并按同样规则排除剪贴板历史、60 秒后自动清除；未配置 token 时复制普通链接。
- 通知中只显示不含 token 的地址。
