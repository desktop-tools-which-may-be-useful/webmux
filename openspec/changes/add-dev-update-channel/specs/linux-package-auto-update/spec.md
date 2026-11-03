# Spec Delta

## MODIFIED Requirements

### Requirement: 安装方式识别

`webmux update apply` SHALL 只在 `/usr/bin/webmux` 属于已安装的 `webmux` 软件包时执行更新，并选择与当前二进制同架构的安装包：

- 由 dpkg 管理：`webmux_<版本>-1_<amd64|arm64>.deb`；
- 由 rpm 管理：`webmux-<版本>-1.<x86_64|aarch64>.rpm`；
- 不属于任何软件包：SHALL 报告“当前安装方式不支持自动更新”并以非零退出码结束，SHALL NOT 下载。

文件名中的 `<版本>` 为 `X.Y.Z` 或 `X.Y.Z-dev.N`。开发版安装包的包内版本为 `X.Y.Z~dev.N`，使 dpkg 与 rpm 都把开发版排在同号正式版之前。

#### Scenario: rpm 系统

- **WHEN** 在 aarch64 的 Fedora 上通过 rpm 安装 0.4.0，最新版本为 0.5.0
- **THEN** 更新服务下载并校验 `webmux-0.5.0-1.aarch64.rpm`

#### Scenario: 非包安装

- **WHEN** 用户以 root 运行 `install.sh` 安装到 `/usr/local/bin` 后执行 `webmux update apply`
- **THEN** 命令报告当前安装方式不支持自动更新并失败，没有网络下载

#### Scenario: 开发版安装包

- **WHEN** deb 安装、通道为 `dev`，最新版本为 0.10.3-dev.4
- **THEN** 更新服务下载并校验 `webmux_0.10.3-dev.4-1_amd64.deb`，安装后 `dpkg-query -W webmux` 报告 `0.10.3~dev.4-1`

#### Scenario: 开发版升到正式版

- **WHEN** 已安装包内版本为 `0.10.3~dev.4-1` 的 deb，最新版本为 0.10.3
- **THEN** 更新服务安装 `webmux_0.10.3-1_amd64.deb`，dpkg 把它视为升级
