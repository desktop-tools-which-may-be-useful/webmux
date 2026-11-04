# Proposal

## Why

自动更新目前只有正式版一个来源：只在推送 `v*` 标签时发布，客户端只读 `releases/latest`，版本号只接受 `MAJOR.MINOR.PATCH`。维护者和少数尝鲜用户想在每次合入 main 后就能自动拿到新构建，在正式发布前发现问题，又不能让正式版用户收到未经发布流程的构建。

## What Changes

- 新增开发版通道：main 上的提交通过 CI 后自动构建并发布开发版，版本号为“最近正式标签的补丁号加 1”加 `-dev.<距该标签的提交数>`，例如 v0.10.2 之后的第 2 个提交为 `0.10.3-dev.2`。
- 只发布通过测试的那个提交；该提交已不是 main 最新提交、版本不高于已发布开发版、恰好是正式标签，或自上次发布以来只改了文档时跳过，保证开发版版本只增不减且不为文档变更打扰用户。
- 开发版只发布到一个固定的预发布 Release `dev`：产物名带版本号，先上传新产物，再替换签名清单，最后删除其他产物，只保留当前构建。不会累积 Release、标签或旧版本产物，也不会成为 `latest`。
- deb/rpm 包内版本为 `X.Y.Z~dev.N`，文件名为 `X.Y.Z-dev.N`（GitHub 会改写文件名中的 `~`）。
- 开发版使用独立的签名密钥。正式版客户端只信任正式版密钥、只接受不带后缀的版本；开发版密钥只能签开发版版本号。
- 安装可以选择通道，默认正式版：
  - Linux 与 macOS 命令行：`webmux update channel stable|dev`，写入 root 拥有的 `/etc/webmux/update-channel`；
  - Windows 托盘：设置项 `updateChannel` 与菜单“更新通道”；
  - `install.sh`：`WEBMUX_CHANNEL=dev` 安装开发版二进制，`WEBMUX_VERSION` 可指定当前开发版；Release `dev` 自带同一份 `install.sh`，开发版安装地址从第一次开发版发布起可用。
- 开发版通道同时读取开发版与正式版清单，取其中最高的已验证版本；正式版发布后开发版用户自动升到正式版。切换通道只升级不降级。
- 版本比较、deb/rpm 版本号、托盘与 WSL 服务端的版本对应判定、服务端可用性分类与更新入口都支持 `-dev.N`。
- `webmux update check --json` 增加字段 `channel`；新版本总是输出它，读取方遇到缺少该字段的旧版本输出时按 `stable` 理解。

## Capabilities

### New Capabilities

- `update-channels`：正式版与开发版通道的定义、各安装方式的通道选择与持久化、切换通道的升级规则。

### Modified Capabilities

- `signed-release-updates`：开发版流水线的构建也是发布构建；版本号格式支持 `-dev.N`；按通道分组的签名密钥；按通道读取清单与下载产物；预发布版本的比较规则；检查命令输出通道。
- `release-distribution`：固定文件名产物与“清单列出全部产物”只适用于标签发布；新增开发版发布流程（含跳过只改文档的变更）、产物命名（含固定文件名的 `install.sh`）、保留策略与独立签名。
- `linux-package-auto-update`：开发版安装包的文件名与 deb/rpm 包内版本。
- `windows-tray-wsl-install`：服务端可用性分类支持 `-dev.N`；开发版托盘从 `dev` Release 下载同一次构建的服务端，产物已删除时明确报错。
- `windows-tray-server-compat`：版本对应判定与更新入口支持 `-dev.N`；托盘通道为 stable 而 WSL 为开发版时，以“切换到开发版通道并更新托盘”替代无效的“更新托盘”。
- `windows-tray-self-update`：开发版安装程序使用带版本号的文件名；便携版托盘的“下载托盘”按版本打开正式版或开发版的 Release 页面。

## Impact

- 代码：`internal/update`（版本解析与比较、通道、密钥分组、清单与产物路径）、`cmd/webmux/updatecmd.go`（`channel` 子命令、`--json` 字段）、`internal/tray`（设置、菜单、自更新、WSL 安装、可用性分类、兼容判定与更新入口）、`install.sh`、`cmd/internal/releasesign`（按通道选择私钥与密钥组）。
- 契约：`contract.UpdateCheckV1` 增加 `channel`；`docs/protocol/control-json.md` 同步。
- 发布：新增 `.github/workflows/dev-release.yml`；`release.yml` 生成发布说明时指定上一个 `v*` 标签；`Makefile` 支持带版本号的产物名；`deploy/windows/webmux.nsi` 的文件版本资源只取数字部分；新增 Actions 密钥 `WEBMUX_DEV_SIGNING_KEY`。
- 文档：`docs/deployment/updates.md`、`docs/deployment/install-and-upgrade.md`、`docs/deployment/windows-wsl.md`，以及相关用户故事。
