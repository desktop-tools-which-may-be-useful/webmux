# Spec Delta

## MODIFIED Requirements

### Requirement: 标签触发发布

推送形如 `v<版本>` 的 Git 标签时，发布流水线 SHALL 以该版本号构建全部发布产物，并创建同名 GitHub Release 上传它们。

产物 SHALL 包括：

- Linux：`webmux-linux-amd64`、`webmux-linux-arm64`；
- macOS：`webmux-darwin-amd64`、`webmux-darwin-arm64`；
- Windows：`webmux-windows-amd64.exe`、`webmux-windows-arm64.exe`；
- Windows 托盘：`webmux-tray-windows-amd64.exe`、`webmux-tray-windows-arm64.exe`；
- Windows 安装程序：`webmux-setup-windows-amd64.exe`、`webmux-setup-windows-arm64.exe`；
- 校验和：`SHA256SUMS`。

每个二进制 SHALL 报告该版本号；安装程序登记的版本号 SHALL 与之相同。构建或测试失败时 SHALL NOT 创建或更新 Release。

#### Scenario: 发布新版本

- **WHEN** 维护者推送标签 `v0.2.0`
- **THEN** Release `v0.2.0` 包含上述全部文件，`webmux version` 输出 `webmux 0.2.0`

#### Scenario: 构建失败

- **WHEN** 推送标签后某个平台构建失败
- **THEN** 不出现该标签的 Release，也不上传任何部分产物

#### Scenario: 安装程序构建失败

- **WHEN** 推送标签后 Windows 安装程序打包失败
- **THEN** 不出现该标签的 Release，也不上传任何部分产物

## ADDED Requirements

### Requirement: 安装程序内容来自同一次构建

每个 Windows 安装程序 SHALL 只包含同一次发布构建产出、同架构的托盘（与 Release 中 `webmux-tray-windows-<架构>.exe` 字节相同）以及同架构的 Linux 服务端（与 `webmux-linux-<架构>` 字节相同）。因此安装后的托盘 SHALL 内置与预置服务端相符的摘要。

#### Scenario: 安装后文件一致

- **WHEN** 用户安装 `v0.2.0` 的 `webmux-setup-windows-amd64.exe`
- **THEN** 安装目录中 `webmux-tray.exe` 的 SHA-256 与同一 Release `SHA256SUMS` 中 `webmux-tray-windows-amd64.exe` 一行一致，预置的服务端与 `webmux-linux-amd64` 一行一致
