# Spec Delta

## MODIFIED Requirements

### Requirement: 版本对应判定

兼容时，托盘 SHALL 只比较版本号：两侧相同为“对应”；否则按 `signed-release-updates` 的版本比较规则（`MAJOR.MINOR.PATCH`，开发版 `-dev.N` 低于同号正式版）归为“WSL 较旧”或“WSL 较新”。

任一侧版本号不是正式版或开发版格式（如 `dev`）时 SHALL 归为“无法判断”，按兼容处理且不提示更新。托盘 SHALL NOT 以构建提交区分版本号相同的构建。

#### Scenario: 发布版本对应

- **WHEN** 托盘与 WSL 中的 `webmux` 都报告 0.3.0
- **THEN** 托盘判定“对应”，状态行不显示版本提示

#### Scenario: 本地构建

- **WHEN** 两侧都是本地 `make` 构建、都报告 0.1.0，即使来自不同提交
- **THEN** 托盘判定“对应”

#### Scenario: 开发构建

- **WHEN** WSL 中的 `webmux` 报告版本 `dev`
- **THEN** 托盘判定“无法判断”，照常工作，不提示更新

#### Scenario: 开发版托盘与正式版服务端

- **WHEN** 托盘为 0.10.3-dev.4，WSL 中的 `webmux` 为 0.10.2
- **THEN** 托盘判定“WSL 较旧”

#### Scenario: 开发版之间

- **WHEN** 托盘为 0.10.3-dev.4，WSL 中的 `webmux` 为 0.10.3-dev.7
- **THEN** 托盘判定“WSL 较新”

### Requirement: 更新入口

托盘 SHALL 按方向提供更新入口：

- WSL 过旧或较旧：提供“升级 WSL 中的 webmux”（安装与托盘同版本的 `webmux`，行为见 `windows-tray-wsl-install`，包括开启自动更新时的自动升级）。
- WSL 过新或较新且其版本为正式版或开发版格式（下一条情形除外）：
  - 安装版托盘 SHALL 提供“更新托盘”，点击后立即按 `windows-tray-self-update` 检查并安装最新版本；`autoUpdate` 开启时托盘 SHALL 在判定后自动发起一次该检查。
  - 便携版托盘 SHALL 提供“下载托盘 <版本>”，用默认浏览器打开该版本所在的 Release 页面（正式版为标签 `v<版本>`，开发版为 Release `dev`）。
- WSL 较新且为开发版、托盘通道为 `stable`：stable 通道不会提供该版本。
  - 状态行 SHALL 提示“WSL 中为开发版 <版本>，托盘使用正式版通道”。
  - 安装版托盘 SHALL NOT 提供“更新托盘”，也 SHALL NOT 自动发起自更新检查；菜单 SHALL 提供“切换到开发版通道并更新托盘”。
  - 点击该项后托盘 SHALL 把 `updateChannel` 设为 `dev` 并立即检查，发现更高版本时视同用户点击“更新到 <版本>”立即安装，不受 `autoUpdate` 与退避限制。
  - 便携版托盘的“下载托盘 <版本>”照常提供，打开 Release `dev` 页面。
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

#### Scenario: 正式版通道遇到开发版服务端

- **WHEN** 安装版托盘 0.10.2 通道为 `stable` 并开启自动更新，WSL 中的 `webmux` 为 0.10.3-dev.7 且兼容
- **THEN** 托盘不自动发起自更新检查，菜单没有“更新托盘”，提供“切换到开发版通道并更新托盘”

#### Scenario: 切换通道并更新托盘

- **WHEN** 接上例，用户点击“切换到开发版通道并更新托盘”，最新开发版为 0.10.3-dev.7
- **THEN** 托盘设置中 `updateChannel` 变为 `dev`，托盘下载、校验并静默安装 0.10.3-dev.7 的安装程序，新托盘判定为对应

#### Scenario: 便携版遇到开发版服务端

- **WHEN** 便携版托盘为 0.10.2，WSL 中的 `webmux` 为 0.10.3-dev.7 且兼容
- **THEN** 菜单出现“下载托盘 0.10.3-dev.7”，点击后浏览器打开 Release `dev` 的页面
