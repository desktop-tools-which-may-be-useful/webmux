# Spec Delta

## MODIFIED Requirements

### Requirement: 更新入口

托盘 SHALL 按方向提供更新入口：

- WSL 过旧或较旧：提供“升级 WSL 中的 webmux”（安装与托盘同版本的 `webmux`，行为见 `windows-tray-wsl-install`，包括开启自动更新时的自动升级）。
- WSL 过新或较新且其版本为 `MAJOR.MINOR.PATCH`：安装版托盘 SHALL 提供“更新托盘”，点击后立即按 `windows-tray-self-update` 检查并安装最新版本；`autoUpdate` 开启时托盘 SHALL 在判定后自动发起一次该检查。便携版托盘 SHALL 提供“下载托盘 <版本>”，用默认浏览器打开该版本的 Release 页面。
- 无法判断：SHALL NOT 提供更新入口。

托盘 SHALL NOT 自动重启 WebMux。自动安装与自动更新只在 `windows-tray-wsl-install` 与 `windows-tray-self-update` 规定的条件下进行。更新完成后的下一次检查 SHALL 更新状态行。

#### Scenario: WSL 较新

- **WHEN** 便携版托盘为 0.3.0，WSL 中的 `webmux` 为 0.4.0 且兼容
- **THEN** 菜单出现“下载托盘 0.4.0”，点击后浏览器打开 `v0.4.0` 的 Release 页面

#### Scenario: WSL 较新（安装版）

- **WHEN** 安装版托盘 0.3.0 开启自动更新，WSL 中的 `webmux` 为 0.4.0，最新发布版本为 0.4.0
- **THEN** 托盘自动下载、校验并静默安装 0.4.0 安装程序，新托盘判定为对应

#### Scenario: 升级后恢复

- **WHEN** WSL 较旧，用户点击“升级 WSL 中的 webmux”并安装成功
- **THEN** 托盘重新检查，判定为对应，状态行不再显示版本提示
