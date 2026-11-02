# release-distribution Specification

## Purpose

定义 WebMux 正式版本的发布方式：一次标签发布产出所有平台二进制与校验和，并让 Windows 托盘内置同版本 Linux 二进制的摘要，使托盘可以安全地安装服务端。

## Requirements

### Requirement: 标签触发发布

推送形如 `v<版本>` 的 Git 标签时，发布流水线 SHALL 以该版本号构建全部发布产物，并创建同名 GitHub Release 上传它们。

产物 SHALL 包括：

- Linux：`webmux-linux-amd64`、`webmux-linux-arm64`；
- macOS：`webmux-darwin-amd64`、`webmux-darwin-arm64`；
- Windows：`webmux-windows-amd64.exe`、`webmux-windows-arm64.exe`；
- Windows 托盘：`webmux-tray-windows-amd64.exe`、`webmux-tray-windows-arm64.exe`；
- Windows 安装程序：`webmux-setup-windows-amd64.exe`、`webmux-setup-windows-arm64.exe`；
- 校验和：`SHA256SUMS`。

每个二进制 SHALL 报告该版本号。构建或测试失败时 SHALL NOT 创建或更新 Release。

#### Scenario: 发布新版本

- **WHEN** 维护者推送标签 `v0.2.0`
- **THEN** Release `v0.2.0` 包含上述全部文件，`webmux version` 输出 `webmux 0.2.0`

#### Scenario: 构建失败

- **WHEN** 推送标签后某个平台构建失败
- **THEN** 不出现该标签的 Release，也不上传任何部分产物

#### Scenario: 安装程序构建失败

- **WHEN** 推送标签后 Windows 安装程序打包失败
- **THEN** 不出现该标签的 Release，也不上传任何部分产物

### Requirement: 校验和文件

`SHA256SUMS` SHALL 为 Release 中每个其他产物各列一行，格式与 `sha256sum` 输出一致，可用 `sha256sum -c` 直接校验。

#### Scenario: 用户校验下载

- **WHEN** 用户下载 `webmux-linux-amd64` 与 `SHA256SUMS` 后执行 `sha256sum -c --ignore-missing SHA256SUMS`
- **THEN** 该文件校验通过

### Requirement: 托盘内置服务端摘要

发布构建的托盘 SHALL 内置同一次构建产出的 `webmux-linux-amd64` 与 `webmux-linux-arm64` 的 SHA-256。托盘 SHALL 只信任内置摘要，SHALL NOT 以下载得到的 `SHA256SUMS` 作为校验依据。未内置摘要的托盘（如开发构建）SHALL 报告“此版本托盘不支持安装 webmux”，且不提供下载。

#### Scenario: 发布版托盘

- **WHEN** 用户运行 Release 中的托盘
- **THEN** 托盘内置的两个摘要与同一 Release 的 `SHA256SUMS` 中对应行一致

#### Scenario: 开发构建

- **WHEN** 用户运行本地 `make tray` 构建、未注入摘要的托盘，且 WSL 中缺少 webmux
- **THEN** 托盘显示缺少 webmux 及“此版本托盘不支持安装 webmux”，菜单没有可用的安装动作

### Requirement: 安装程序内容来自同一次构建

每个 Windows 安装程序 SHALL 只包含同一次发布构建产出、同架构的托盘（与 Release 中 `webmux-tray-windows-<架构>.exe` 字节相同）以及同架构的 Linux 服务端（与 `webmux-linux-<架构>` 字节相同）。因此安装后的托盘 SHALL 内置与预置服务端相符的摘要。安装程序登记的版本号 SHALL 与该发布版本相同。

#### Scenario: 安装后文件一致

- **WHEN** 用户安装 `v0.2.0` 的 `webmux-setup-windows-amd64.exe`
- **THEN** 安装目录中 `webmux-tray.exe` 的 SHA-256 与同一 Release `SHA256SUMS` 中 `webmux-tray-windows-amd64.exe` 一行一致，预置的服务端与 `webmux-linux-amd64` 一行一致

### Requirement: Linux 安装包

标签发布 SHALL 另外产出 Linux 安装包并上传到同一 Release：`webmux_<版本>-1_amd64.deb`、`webmux_<版本>-1_arm64.deb`、`webmux-<版本>-1.x86_64.rpm`、`webmux-<版本>-1.aarch64.rpm`。

安装包 SHALL 由同一次构建的 `webmux-linux-<架构>` 打包，并各自在 `SHA256SUMS` 中占一行。安装包构建失败时 SHALL NOT 创建 Release。

#### Scenario: 发布包含安装包

- **WHEN** 维护者推送标签 `v0.2.0`
- **THEN** Release 中包含上述四个安装包，`sha256sum -c SHA256SUMS` 对它们全部通过
