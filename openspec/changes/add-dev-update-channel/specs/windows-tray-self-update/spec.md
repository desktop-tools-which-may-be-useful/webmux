# Spec Delta

## MODIFIED Requirements

### Requirement: 安装版识别

只有从安装程序登记的安装目录运行的托盘 SHALL 视为安装版，可以自动更新自身。其他位置的托盘为便携版：发现新版本时 SHALL 只在状态行提示，并在菜单提供“下载托盘 <版本>”，用默认浏览器打开该版本所在的 Release 页面（正式版为标签 `v<版本>`，开发版为 Release `dev`），SHALL NOT 下载或运行安装程序。

#### Scenario: 便携版

- **WHEN** 用户运行 `Downloads\webmux-tray-windows-amd64.exe` 0.4.0，最新版本为 0.5.0
- **THEN** 状态行提示有新版本，菜单出现“下载托盘 0.5.0”，没有下载安装程序

#### Scenario: 便携版开发版通道

- **WHEN** 便携版托盘通道为 `dev`，最新版本为 0.10.3-dev.4
- **THEN** 菜单出现“下载托盘 0.10.3-dev.4”，点击后打开 Release `dev` 的页面

### Requirement: 自动安装新版本

安装版托盘发现更高版本时，SHALL 下载同架构的安装程序到 `%LOCALAPPDATA%\WebMux\updates`，按 `signed-release-updates` 校验后以静默模式运行它。安装程序的文件名：正式版为 `webmux-setup-windows-<架构>.exe`，开发版为 `webmux-<版本>-setup-windows-<架构>.exe`。

- 触发：`autoUpdate` 开启时 SHALL 无需用户操作；关闭时 SHALL 只在用户点击“更新到 <版本>”后执行。
- 中途关闭：未经用户点击“更新到 <版本>”的安装在下载或等待空闲期间 `autoUpdate` 被关闭时，SHALL NOT 运行安装程序，状态行恢复为“有新版本 <版本>”。
- 记录：运行安装程序前 SHALL 在设置中记录本次尝试 `updateAttempt`（版本与时间）；无法记录时 SHALL NOT 运行安装程序，并显示更新失败。
- 清除：该版本或更高版本的托盘启动后 SHALL 清除记录；安装程序无法启动，或在本托盘仍运行时退出，SHALL 清除记录；等待超时 SHALL 保留记录。
- 退出：托盘 SHALL 以“仅退出托盘”的方式响应安装程序的退出请求。

#### Scenario: 后台更新

- **WHEN** 安装版托盘 0.4.0 开启自动更新，最新版本为 0.5.0，WebMux 有 2 个会话
- **THEN** 托盘下载并校验安装程序后静默运行它，旧托盘退出，0.5.0 托盘启动并显示运行中，2 个会话保持可用

#### Scenario: 签名无效

- **WHEN** 所有来源提供的安装程序都无法通过签名清单校验
- **THEN** 托盘不运行任何安装程序，状态行显示更新失败及原因，下次计划检查时重试

#### Scenario: 等待期间关闭自动更新

- **WHEN** 安装程序已下载，托盘正在等待 WebMux 重启完成，用户取消勾选“自动更新”
- **THEN** 托盘不运行安装程序，状态行显示“有新版本 <版本>”

#### Scenario: 开发版安装程序

- **WHEN** 安装版托盘通道为 `dev`，开启自动更新，最新版本为 0.10.3-dev.4，架构为 amd64
- **THEN** 托盘从 Release `dev` 下载并校验 `webmux-0.10.3-dev.4-setup-windows-amd64.exe` 后静默运行它
