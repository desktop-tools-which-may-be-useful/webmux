# Tasks

## 1. 托盘 `quit` 子命令

- [x] 1.1 在 `internal/tray` 中加入跨平台的 `quit` 参数解析（`quit`、`quit --kill-path <路径>`，拒绝未知参数）与等待/结束流程（`Quitter`），单元测试覆盖无托盘、正常退出、旧托盘、指定路径结束与其他路径保留及非法参数；进程路径比较复用 `winui` 现有的 `samePath`；`go test -race ./internal/tray` 通过
- [x] 1.2 在 `internal/tray/winui/singleinstance.go` 创建 `Local\WebMuxTrayQuit-<SID>` 事件并提供等待入口；托盘收到信号后执行 `shutdown(false)` 与 `systray.Quit()`，日志记录“quit requested by another process”；`GOOS=windows go vet ./internal/tray/...` 通过
- [x] 1.3 在 `winui` 中实现请求方：检测互斥量、置位事件、每 100ms 轮询至多 10 秒，`--kill-path` 超时后只结束当前用户、当前会话中映像路径匹配的进程并再等 2 秒，按 D3 返回 0/1；`cmd/webmux-tray/main_windows.go` 在 `AcquireInstance` 之前分派 `quit`，且该路径不创建图标；`make tray` 成功
- [x] 1.4 在 `internal/tray/install.go` 的缓存文件名处注释安装程序依赖该命名（指向 `deploy/windows/webmux.nsi`）；`go vet ./internal/tray` 通过
- [x] 1.5 更新 `docs/deployment/windows-wsl.md`，说明 `webmux-tray.exe quit [--kill-path]` 的用途与退出码；`make docs-check` 通过

## 2. NSIS 安装脚本

- [x] 2.1 新增 `deploy/windows/webmux.nsi` 骨架，参数全部经 `-D` 传入；验证：本机用 `makensis` 以 amd64/arm64 两组参数编译成功
  - 简体中文 MUI 界面、`RequestExecutionLevel user`、版本资源；
  - 默认目录 `$LOCALAPPDATA\Programs\WebMux`，`InstallDirRegKey` 沿用上次目录；
  - `.onInit` 架构检查：不符时提示应下载的安装包，静默模式退出码 2。
- [x] 2.2 实现安装 Section；验证：`makensis` 编译成功
  - 先解出新托盘到 `$PLUGINSDIR`，执行 `quit --kill-path "$INSTDIR\webmux-tray.exe"`；失败时交互模式提供重试/取消，静默模式退出码 3，且未写任何文件；
  - 写入托盘、`uninstall.exe`、开始菜单快捷方式、HKCU 卸载项（D2 所列字段）；
  - 预置服务端到 `$LOCALAPPDATA\WebMux\downloads\webmux-<VERSION>-linux-<ARCH>`；
  - 可选（默认勾选）写入 `Run\WebMuxTray`；完成后启动托盘。
- [x] 2.3 实现卸载 Section：同样先执行 `quit`；只删除 D2 列出的文件、快捷方式、卸载项与本版本预置服务端，`Run\WebMuxTray` 仅在指向本次安装路径时删除；完成页说明 WebMux 仍在 WSL 中运行及保留的内容；`makensis` 编译成功
- [x] 2.4 在 `docs/deployment/windows-wsl.md` 中把“放置托盘与登录启动”改为以安装程序为首选，补充升级、卸载（含保留项）、静默参数 `/S`、SmartScreen 提示与“独立 exe 仍可用”；`make docs-check` 通过

## 3. 构建与发布

- [x] 3.1 `Makefile` 新增 `installer` 目标（依赖 `bin/` 中同架构的托盘与 Linux 服务端，输出 `bin/webmux-setup-windows-<架构>.exe`），`release` 在托盘摘要验证后调用并把两个安装包加入 `RELEASE_FILES`；本机执行 `make release VERSION=0.0.1` 后 `cd bin && sha256sum -c SHA256SUMS` 通过且包含两个安装包
- [x] 3.2 `.github/workflows/release.yml` 在构建前安装 `nsis`，并把两个安装包加入 `gh release create` 的文件列表；`actionlint`（若可用）或人工核对文件列表与 `RELEASE_FILES` 一致
- [x] 3.3 在 `docs/deployment.md` 的安装章节说明 Windows 安装程序入口与 Release 资产名；`make docs-check` 通过

## 4. Windows 冒烟验证

- [ ] 4.1 在 `.github/workflows/test.yml` 新增 `windows-latest` 任务（检测或 `choco install nsis`，编译两个架构的托盘，以占位文件代替 Linux 服务端后打包，运行 `.github/scripts/windows-installer-smoke.ps1`），并纳入 `Test result` 汇总；任务在 PR 上运行通过
- [x] 4.2 冒烟脚本验证静默安装：文件、开始菜单快捷方式、卸载项字段、`Run\WebMuxTray` 值、预置服务端路径与 SHA-256 等于打包时的服务端文件；托盘进程在运行
- [x] 4.3 冒烟脚本验证 `quit`：无托盘时退出码 0；运行中托盘被正常关闭且退出码 0；不响应的进程（以 `--kill-path` 指向的同名测试进程模拟）被结束，其他路径的进程不受影响
- [x] 4.4 冒烟脚本验证覆盖安装：旧托盘被关闭、文件被替换、新托盘运行；再验证静默卸载删除项与保留项（`tray.json`、`tray.log` 保留，指向其他路径的 `Run` 值保留）

## 5. 用户故事与集成检查

- [x] 5.1 更新用户故事；验证：ID 唯一、链接路径存在，`make docs-check` 通过
  - `docs/user-stories/clients-network-tray.md`：修订 US-O01（本地缓存与预置服务端的离线安装）、US-O04（安装程序登记登录启动、`quit` 退出）；
  - 新增安装程序故事，覆盖安装、升级、卸载、架构检查与静默模式，并更新依据链接；
  - 同步 `docs/user-stories/coverage.md` 与 `docs/user-stories.md` 的索引与计数。
- [ ] 5.2 在一台带 WSL 的真实 Windows 上手工验证：交互式安装 → 托盘菜单离线安装 WSL 服务端 → 运行中覆盖升级（会话保持）→ 卸载（WebMux 继续运行）；在 PR 描述中附安装界面截图
- [x] 5.3 运行 `make check` 与 `go test -race ./internal/tray/...`，全部通过；`openspec validate add-windows-installer --strict` 通过

## Workflow follow-up

- 合并后按项目流程归档该变更，并确认 `windows-installer` 主规格的 Purpose 已写入。
