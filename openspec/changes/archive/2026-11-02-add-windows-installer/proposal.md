# Proposal

## Why

Windows 用户目前需要手工下载 `webmux-tray-windows-*.exe` 并自行选择存放位置：没有开始菜单入口和“应用和功能”中的卸载项，移动 exe 后登录启动失效，首次在 WSL 中安装服务端还必须联网下载。

按用户安装的 Windows 安装程序可以把托盘放到固定位置、完成登录启动登记，并随包提供 WSL 服务端，降低首次使用和升级的门槛。

## What Changes

- 新增 Windows 安装程序 `webmux-setup-windows-amd64.exe`、`webmux-setup-windows-arm64.exe`：按当前用户安装、不需要管理员权限，界面为简体中文。
  - 把托盘安装到 `%LOCALAPPDATA%\Programs\WebMux\webmux-tray.exe`（可更改目录），创建开始菜单快捷方式，在当前用户下登记卸载项。
  - 可选（默认勾选）登录时启动，登记方式与托盘菜单“登录时自动启动”一致。
  - 内置与安装包架构对应的 Linux 服务端，预置到托盘的下载缓存目录，使托盘菜单中的“安装 / 升级 WSL 中的 webmux”无需联网；安装程序本身不改动 WSL。
  - 覆盖安装即升级：先请求正在运行的托盘退出，超时再强制结束，完成后重新启动托盘。
  - 卸载只移除安装程序写入的内容，保留托盘设置、日志和 WSL 中的 webmux；不停止 WebMux。
  - 安装包架构与 Windows 本机架构不符时拒绝安装并指明应下载的安装包。
- 托盘新增 `webmux-tray.exe quit [--kill-path <路径>]` 子命令：请求本用户正在运行的托盘按“仅退出托盘”退出，最多等待 10 秒；指定路径时超时后只结束从该路径运行的托盘进程。供安装程序和用户脚本使用。
- 发布流水线在现有 ubuntu 任务中用 NSIS 构建两个安装包，加入 Release 与 `SHA256SUMS`；原有独立托盘 exe 继续发布。
- 不包含代码签名；安装程序与独立 exe 一样会出现 SmartScreen 提示。

## Capabilities

### New Capabilities

- `windows-installer`: Windows 安装程序的安装、升级、卸载、登录启动登记、服务端预置与架构检查行为。

### Modified Capabilities

- `release-distribution`: Release 产物增加两个 Windows 安装程序，并纳入 `SHA256SUMS`；安装包内的托盘与服务端必须来自同一次构建。
- `windows-tray-host`: 新增“外部退出请求”要求（`quit` 子命令），效果等同“仅退出托盘”。
- `windows-tray-wsl-install`: 下载来源要求增加“本地已有且摘要匹配的文件直接使用、不发起下载”，覆盖安装程序预置的服务端。

## Impact

- 代码：`cmd/webmux-tray/main_windows.go`（解析 `quit`）、`internal/tray/winui/singleinstance.go`（退出事件与等待）、托盘主循环对退出请求的处理。
- 新文件：`deploy/windows/webmux.nsi` 及其资源。
- 构建与发布：`Makefile` 新增 `installer` 目标并扩展 `RELEASE_FILES`；`.github/workflows/release.yml` 安装 `nsis` 并上传安装包。构建依赖新增 NSIS 3（仅发布与本地打包需要，运行时无依赖）。
- 文档与用户故事：`docs/deployment/windows-wsl.md`；`docs/user-stories/clients-network-tray.md` 中的 US-O01、US-O04 及新增安装程序故事，同步两个索引。
- 不影响服务端、REST/WebSocket 契约或 Linux/macOS 安装方式。
