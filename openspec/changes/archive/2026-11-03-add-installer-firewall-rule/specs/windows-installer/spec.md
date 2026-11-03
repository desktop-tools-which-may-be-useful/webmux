# Spec Delta

## MODIFIED Requirements

### Requirement: 按用户安装且无需管理员

安装程序 SHALL 只为当前 Windows 用户安装，SHALL NOT 写入 `HKEY_LOCAL_MACHINE`、`Program Files` 或其他用户的目录。默认安装目录 SHALL 为 `%LOCALAPPDATA%\Programs\WebMux`，用户可以在安装界面改为其他可写目录。安装程序界面 SHALL 使用简体中文。

安装本身 SHALL NOT 请求管理员权限。唯一的例外是“防火墙规则”中用户勾选的可选规则：添加时 SHALL 只弹出一次管理员确认，用户取消时安装照常完成。

#### Scenario: 普通用户安装

- **WHEN** 非管理员用户双击 `webmux-setup-windows-amd64.exe`，取消勾选防火墙选项并接受其他默认选项
- **THEN** 安装过程中不出现 UAC 提示，`%LOCALAPPDATA%\Programs\WebMux\webmux-tray.exe` 存在

#### Scenario: 自定义目录

- **WHEN** 用户把安装目录改为 `D:\Tools\WebMux`
- **THEN** 托盘被安装到 `D:\Tools\WebMux\webmux-tray.exe`，开始菜单快捷方式与卸载项指向该路径

### Requirement: 卸载

卸载程序 SHALL 先按“覆盖安装即升级”的方式让托盘退出，再删除安装程序写入的内容：安装目录中的文件、开始菜单快捷方式、卸载项与预置的服务端文件。登录启动项只有指向本次安装的 `webmux-tray.exe` 时才 SHALL 删除。

安装程序为本次安装添加过防火墙规则时，卸载 SHALL 删除该规则（弹出一次管理员确认）；用户取消时 SHALL 说明如何手工删除，卸载照常完成。

卸载 SHALL NOT 停止 WebMux，SHALL NOT 删除托盘设置、托盘日志、保活记录或 WSL 中的任何文件。完成页 SHALL 说明保留了哪些内容；WebMux 正在运行时还 SHALL 说明它仍在 WSL 中运行。

#### Scenario: 卸载保留数据

- **WHEN** WebMux 正在运行，用户从“应用和功能”卸载 WebMux
- **THEN** 托盘消失，安装目录、快捷方式与卸载项被删除；`%APPDATA%\WebMux\tray.json`、`%LOCALAPPDATA%\WebMux\tray.log` 与 WSL 中的 webmux 保留，浏览器中的会话继续可用

#### Scenario: 启动项指向其他路径

- **WHEN** 登录启动项指向另一个位置的托盘 exe，用户卸载安装版
- **THEN** 该启动项保持不变

#### Scenario: 删除防火墙规则

- **WHEN** 安装时添加过防火墙规则，用户卸载并在管理员确认中选择“是”
- **THEN** 以“WebMux tray forwarding”开头、指向该托盘的规则被删除

## ADDED Requirements

### Requirement: 防火墙规则

交互安装的“安装选项”页 SHALL 提供“允许其他设备通过托盘转发访问 WebMux（添加 Windows 防火墙规则）”，并说明只放行专用网络与域网络、会弹出一次管理员确认。首次安装时 SHALL 默认勾选；覆盖已有安装时 SHALL 默认沿用上次是否添加了规则。

- 勾选时，安装完成后 SHALL 为安装目录中的 `webmux-tray.exe` 添加入站 TCP 允许规则，只适用于专用网络与域网络，SHALL NOT 适用于公用网络。
- 规则已存在时 SHALL NOT 再次请求管理员权限；成功后 SHALL 在卸载项中记录该托盘路径，供以后删除。
- 覆盖安装时取消勾选，或安装目录与记录的路径不同，SHALL 删除记录的规则（弹出管理员确认）。
- 用户取消管理员确认或添加失败时，安装 SHALL 照常完成，并说明之后如何手工添加或删除。
- 静默安装（`/S`）SHALL NOT 请求管理员权限，SHALL NOT 添加或删除规则；静默覆盖原路径时已有规则继续有效。

#### Scenario: 首次安装时授权

- **WHEN** 用户交互安装并保持勾选防火墙选项，在管理员确认中选择“是”
- **THEN** 防火墙中出现指向 `%LOCALAPPDATA%\Programs\WebMux\webmux-tray.exe` 的入站允许规则，配置文件为“专用、域”；之后托盘开始转发时不再出现 Windows 防火墙提示

#### Scenario: 取消管理员确认

- **WHEN** 用户在管理员确认中选择“否”
- **THEN** 安装成功完成，安装程序提示已跳过防火墙规则及手工添加的命令

#### Scenario: 重新安装时取消勾选

- **WHEN** 之前安装时添加过规则，用户再次交互安装并取消勾选防火墙选项，在管理员确认中选择“是”
- **THEN** 之前的规则被删除，卸载项不再记录规则

#### Scenario: 托盘自更新

- **WHEN** 已添加规则的安装版托盘以 `/S` 静默更新到新版本
- **THEN** 没有管理员确认，原规则继续适用于新托盘
