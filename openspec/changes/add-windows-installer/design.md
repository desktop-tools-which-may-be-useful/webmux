# Design

## Context

动机见 proposal.md。与实现相关的现状：

- 发布只有一个 ubuntu 任务（`.github/workflows/release.yml`）。`make release` 先编译 Linux 服务端，再把它们的 SHA-256 以 `-ldflags -X` 写入托盘（`internal/tray/install.go` 的 `linuxAMD64SHA256` / `linuxARM64SHA256`），最后生成 `SHA256SUMS`。
- 托盘单实例使用 `Local\WebMuxTray-<SID>` 互斥量（创建时不持有）和 `Local\WebMuxTrayWake-<SID>` 自动复位事件（`internal/tray/winui/singleinstance.go`）。
- “仅退出托盘”即 `App.shutdown(false)` 后 `systray.Quit()`（`internal/tray/winui/lifecycle.go`）。systemd 方式的保活进程是分离启动的，PID 记录在 `%LOCALAPPDATA%\WebMux\keepalive.json`，托盘退出后仍然存活（`internal/tray/service.go`）。托盘托管的 `serve` 同样独立于托盘进程。
- 登录启动写入 `HKCU\Software\Microsoft\Windows\CurrentVersion\Run` 的 `WebMuxTray` 值，内容为带引号的 exe 路径；复选状态按路径比较（`internal/tray/winui/autostart.go`）。
- `Installer.Install` 先检查 `<DownloadDir>\webmux-<版本>-linux-<架构>` 的摘要，匹配就跳过下载；`DownloadDir` 为 `%LOCALAPPDATA%\WebMux\downloads`（`cmd/webmux-tray/main_windows.go`）。
- 托盘已有 `version` 子命令，以 `os.Args[1]` 识别。

## Goals / Non-Goals

**Goals:**

- 安装包在现有 ubuntu 发布任务中构建，不新增发布平台。
- 安装程序与托盘之间只通过已有约定（Run 值、下载缓存文件名）和新增的 `quit` 子命令交互，不引入其他进程间协议。
- 在 Windows CI 上以静默模式验证安装、升级、卸载。

**Non-Goals:**

- 代码签名、自动更新、winget/Scoop 清单。
- 安装程序向 WSL 安装服务端、修改 `.wslconfig` 或防火墙。
- 按机器（所有用户）安装。

## Decisions

### D1：NSIS 打包

使用 NSIS 3（`makensis`），在 ubuntu 上用 apt 安装。脚本放在 `deploy/windows/webmux.nsi`，由 `make installer` 以 `-D` 传入 `VERSION`、`ARCH`、托盘与服务端的路径和输出文件名，两个架构各调用一次。

- 备选 Inno Setup：自带 Restart Manager 与 `AppMutex`，但需要 Windows runner 或 wine，会把发布拆成多平台任务，失败时更难保证“不留下部分 Release”。
- 备选 WiX/MSI：只能在 Windows 上构建；按用户安装的 MSI 有不少限制。
- 备选 MSIX：必须签名；写入 HKCU `Run` 会被虚拟化，需要改用 `StartupTask`，和现有 `autostart.go` 冲突。

NSIS 安装程序本身是 x86 程序，可在 amd64 和 arm64 Windows 上运行。架构检查使用 `x64.nsh` 的 `${IsNativeAMD64}` / `${IsNativeARM64}`。

### D2：安装布局与注册表

- `RequestExecutionLevel user`，使用当前用户的 shell 变量上下文；默认 `$LOCALAPPDATA\Programs\WebMux`，`InstallDirRegKey` 读取上次的 `InstallLocation`，使升级沿用原目录。
- 安装目录内只有 `webmux-tray.exe` 与 `uninstall.exe`。卸载按文件名删除这两个文件后执行非递归 `RMDir`，不会误删用户放进该目录的其他文件。
- 卸载项写在 `HKCU\Software\Microsoft\Windows\CurrentVersion\Uninstall\WebMux`：`DisplayName`、`DisplayVersion`、`Publisher`、`DisplayIcon`、`InstallLocation`、`UninstallString`、`QuietUninstallString`、`NoModify`、`NoRepair`、`EstimatedSize`。
- 版本资源 `VIProductVersion` 使用 `<VERSION>.0`。
- 登录启动写入 `Run\WebMuxTray = "<INSTDIR>\webmux-tray.exe"`，格式与 `SetAutostart` 相同，托盘菜单复选项因此能正确识别。卸载时只有该值等于本次安装的路径（NSIS `StrCmp` 不区分大小写）才删除。
- 服务端预置到 `$LOCALAPPDATA\WebMux\downloads\webmux-<VERSION>-linux-<ARCH>`。卸载时只删除该版本的这个文件，不删除下载目录中的其他文件。

### D3：`quit` 子命令

- 托盘在 `AcquireInstance` 成功后额外创建自动复位事件 `Local\WebMuxTrayQuit-<SID>`，并在独立的 goroutine 中等待它。收到信号后在 UI 线程执行 `shutdown(false)` 与 `systray.Quit()`：不弹出询问，正在显示的询问框随进程退出而关闭。
- `webmux-tray.exe quit` 不调用 `AcquireInstance`：
  1. 用 `OpenMutex` 检测 `Local\WebMuxTray-<SID>`，不存在则返回 0。
  2. 用 `OpenEvent` 打开退出事件并置位；事件不存在说明对方是不支持该命令的旧版托盘，跳过置位。
  3. 每 100ms 检测一次互斥量，直到它消失（持有句柄的进程已全部退出）或 10 秒超时。
  4. 超时且指定了 `--kill-path` 时，枚举当前会话中属于当前用户 SID、映像路径（`QueryFullProcessImageName`，规范化后不区分大小写比较）等于该路径的进程，执行 `TerminateProcess`，然后再等待最多 2 秒。
  5. 互斥量已消失返回 0，否则返回 1。
- 互斥量创建时不持有，无法通过等待它来判断对方退出，因此采用“检测对象是否还存在”的方式。
- 参数解析与等待/结束流程放在跨平台、可单元测试的 `tray.Quitter` 中；Win32 调用只放在 `winui`，路径比较复用 `autostart.go` 的 `samePath`。
- 备选方案：NSIS 用 `taskkill /IM` 按映像名结束进程，会误伤其他位置或其他用户的托盘；改用插件（如 nsProcess）则 apt 的 NSIS 不自带，增加构建依赖。

### D4：安装与卸载流程

```
.onInit:  架构不符 -> 提示/静默退出码 2
Section:  解出新托盘到 $PLUGINSDIR
          ExecWait "$PLUGINSDIR\webmux-tray.exe" quit --kill-path "$INSTDIR\webmux-tray.exe"
            非 0 -> 交互：重试/取消；静默：退出码 3，未写任何文件
          写入文件、快捷方式、卸载项、预置服务端、(可选) Run 值
          Exec "$INSTDIR\webmux-tray.exe"
un.Section: ExecWait "$INSTDIR\webmux-tray.exe" quit --kill-path "$INSTDIR\webmux-tray.exe"
          非 0 -> 交互：重试/取消；静默：退出码 3
          删除 D2 中列出的内容；完成页说明保留项
```

“交互/静默”通过 `IfSilent` 区分。之所以用新解出的托盘执行 `quit`，是因为旧版本的已安装托盘可能还没有这个子命令。

### D5：构建与发布接线

- `Makefile`：新增 `installer` 目标，依赖 `bin/webmux-tray-windows-<架构>.exe` 与 `bin/webmux-linux-<架构>`，输出 `bin/webmux-setup-windows-<架构>.exe`。`release` 在验证托盘摘要之后调用它，并把两个安装包加入 `RELEASE_FILES`，从而计入 `SHA256SUMS`。
- `release.yml`：在构建前执行 `sudo apt-get install -y nsis`，并在 `gh release create` 的文件列表中加入两个安装包。
- 安装包内的文件直接取自 `bin/`，与 Release 中的同名产物字节一致，满足 `release-distribution` 的同一构建要求，无需额外比对。

### D6：Windows 冒烟验证

在 `test.yml` 新增 `windows-latest` 任务：编译两个架构的托盘，以占位文件代替 Linux 服务端（冒烟只核对预置文件的位置与摘要，不需要真实服务端，也就不需要前端构建），用 runner 上的 NSIS 打包，然后由 `.github/scripts/windows-installer-smoke.ps1` 依次执行：

1. 静默安装，检查文件、快捷方式、卸载项与 Run 值。
2. 启动托盘（runner 上没有 WSL，托盘处于“未检测到 WSL 发行版”的异常状态，可以接受）。
3. 再次静默安装，验证托盘被 `quit` 关闭、文件被替换，新托盘再次运行。
4. 静默卸载，验证删除项与保留项。

`quit` 在“没有托盘 / 正常退出 / 不响应且指定路径”三种情况下的退出码，也在该任务中用实际进程验证。

## Risks / Trade-offs

- [下载缓存文件名 `webmux-<版本>-linux-<架构>` 同时写在 Go（`install.go`）与 NSIS 中，可能不同步] → 两处互相注释引用；D6 冒烟在静默安装后断言该路径存在且 SHA-256 等于打包时的服务端文件，任一侧改名而另一侧未改时需要同时修改冒烟脚本，评审时可见。
- [`quit` 到达时托盘正在执行 WSL 安装，退出后 `wsl.exe` 子进程是否继续不确定] → 托盘退出不会回滚已写入的文件，`install -D` 失败时原文件保持不变；在托盘日志中记录退出时正在进行的操作。实现时需要实测确认。
- [GitHub `windows-latest` 镜像未必预装 NSIS] → 冒烟任务先检测 `makensis`，缺失时用 `choco install nsis` 安装。
- [未签名，SmartScreen 拦截比独立 exe 更显眼] → 在部署文档中说明“更多信息 → 仍要运行”，签名另行立项。
- [arm64 安装包无法在 x86 runner 上实际运行] → CI 只冒烟 amd64 包；arm64 包与 amd64 包共用同一脚本，仅 `-D` 参数不同。

## Migration Plan

- 独立托盘 exe 的用户：运行安装程序后，安装版托盘与 Run 值接管；原 exe 如果仍在运行，安装程序会提示先退出它。原 exe 文件和 `tray.json` 都不受影响，可以手动删除。
- 回滚：卸载安装版，重新使用独立 exe 并在菜单中勾选“登录时自动启动”。
