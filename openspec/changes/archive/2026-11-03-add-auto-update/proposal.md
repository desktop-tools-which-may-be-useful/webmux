# Proposal

## Why

通过 deb/rpm 或 Windows 安装程序“安装到系统”的 WebMux 只能手动更新：Linux 要重新下载安装包，Windows 要重新下载安装程序，托盘规格还明确禁止自动更新自身。

现有下载只以同一来源的 `SHA256SUMS` 校验，镜像或发布账号被攻破即可同时替换文件与摘要。手动安装时尚可接受，但后台自动替换可执行文件需要独立的信任根。

## What Changes

- 发布流水线用内置公钥对应的 ed25519 私钥对 `SHA256SUMS` 签名，产出 minisign 兼容的 `SHA256SUMS.minisig`，签名的可信注释绑定版本号；缺少签名密钥时不创建 Release。
- 新增统一的更新发现与校验规则：从 GitHub 与内置镜像读取最新版本的 `SHA256SUMS` 与签名，只信任内置公钥，只接受高于当前版本的发布版本，非发布构建不更新。新增 `webmux update check`。
- deb/rpm 包新增系统级 `webmux-update.timer` 与 `webmux-update.service`：以 root 每日检查，下载并校验同格式、同架构的新安装包后用 `dpkg`/`rpm` 安装。首次安装时启用定时器，升级时保留用户的禁用选择。**BREAKING**：安装包不再只有两个文件，安装时会启用一个系统定时器。
- Windows 托盘（仅限由安装程序安装的副本）自动检查新版本，下载并校验安装程序后静默升级自身；托盘重启不结束会话。升级后的托盘自动把 WSL 中较旧的 webmux 升级到同版本。托盘设置新增 `autoUpdate`（默认开启）与菜单项“检查更新”。
- 静默升级保留已有安装的登录启动选择；关闭托盘后升级失败时重新启动原托盘。
- 正在运行的服务端检测到磁盘上的可执行文件已被替换为其他版本时，在 `webmux status`、本机状态 API 与 Web UI 中提示“已安装 <版本>，重启后生效，重启会结束所有会话”。任何更新路径都 SHALL NOT 自动重启 WebMux。
- 不在范围内：`install.sh`/手动安装与 macOS 的自动替换（只支持 `webmux update check`）；Windows 原生控制台客户端；APT/YUM 仓库；自动重启服务。

## Capabilities

### New Capabilities

- `signed-release-updates`：发现最新发布版本、验证发布签名与版本绑定、下载来源顺序、版本比较与不降级策略，以及 `webmux update check` 命令。
- `linux-package-auto-update`：deb/rpm 安装的系统级定时自动更新，包括启用/禁用、安装方式识别、通过包管理器安装与失败处理。
- `windows-tray-self-update`：安装版托盘的自动检查、下载、校验与静默升级，便携版托盘的提示，以及 `autoUpdate` 设置与“检查更新”菜单项。
- `server-update-notice`：运行中的服务端识别已安装的新版本，并在 CLI、本机状态 API 与 Web UI 中提示重启后生效。

### Modified Capabilities

- `release-distribution`：Release 产物新增 `SHA256SUMS.minisig`；校验和文件规则排除签名文件；新增发布签名要求。
- `linux-packages`：安装包内容增加系统级更新服务与定时器单元。
- `windows-installer`：静默模式在升级已有安装时保留登录启动选择，失败时恢复原托盘。
- `windows-tray-wsl-install`：开启自动更新时，WSL 中较旧或过旧的 webmux 无需点击即自动升级。
- `windows-tray-server-compat`：取消“托盘 SHALL NOT 自动更新自身”的限制，WSL 较新时由安装版托盘自动更新自身。

## Impact

- 发布：`.github/workflows/release.yml`、`Makefile` 的 `release` 目标、新增签名工具与 GitHub Actions 密钥 `WEBMUX_RELEASE_SIGNING_KEY`。
- Go：新增共享包（签名验证、清单解析、版本策略、下载），复用 `internal/fetch` 的镜像与超时；`cmd/webmux` 新增 `update` 子命令；`internal/contract` 的 `StatusV1` 与配置响应新增可选字段；`internal/tray` 新增自更新逻辑与设置项；`internal/archtest` 登记新包。
- 打包：`deploy/linux/nfpm.yaml` 新增单元文件与安装脚本；`deploy/windows` 安装脚本的静默升级行为。
- 前端：`web/src/schemas.ts` 配置结构新增可选字段，主页与设置页显示待生效版本提示；需重新录制 `internal/api/testdata/wire`。
- 依赖：仅使用 Go 标准库 `crypto/ed25519`，不新增第三方依赖。
- 用户故事：`docs/user-stories/operations-and-tooling.md`、`docs/user-stories/clients-network-tray.md`。
