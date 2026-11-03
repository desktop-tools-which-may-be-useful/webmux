# Spec Delta

## MODIFIED Requirements

### Requirement: 服务端可用性分类

托盘 SHALL 把所选发行版中配置路径上的 webmux 归为以下之一：缺失（无法执行）、不兼容（缺少托盘依赖的功能或状态契约）、较旧（可用，但版本号低于托盘）、可用。

- 版本号 SHALL 按 `signed-release-updates` 的版本比较规则比较；正式版 `X.Y.Z` 与开发版 `X.Y.Z-dev.N` 都是发布版本。
- 任一方不是这两种格式（如 `dev`）时 SHALL 视为可用，不提示升级。
- WSL 中的 webmux 版本高于托盘时 SHALL 视为可用。

#### Scenario: 版本较旧

- **WHEN** 托盘为 0.2.0，WSL 中的 webmux 报告 0.1.0 且兼容
- **THEN** 托盘照常管理 WebMux，状态行附加“WSL 中的 webmux 0.1.0 可升级到 0.2.0”

#### Scenario: 开发版本

- **WHEN** WSL 中的 webmux 报告版本 `dev`
- **THEN** 托盘不提示升级

#### Scenario: 开发版托盘

- **WHEN** 托盘为 0.10.3-dev.4，WSL 中的 webmux 报告 0.10.3-dev.2 且兼容
- **THEN** 状态行附加“WSL 中的 webmux 0.10.3-dev.2 可升级到 0.10.3-dev.4”

### Requirement: 下载来源与完整性

托盘 SHALL 从 GitHub Release 中与托盘同版本的位置下载所需文件：正式版托盘从标签 `v<版本>` 下载 `webmux-linux-<架构>`；开发版托盘从 Release `dev` 下载 `webmux-<版本>-linux-<架构>`。

- 托盘本地下载缓存中已有该版本、该架构的文件，且其 SHA-256 与托盘内置摘要一致时，托盘 SHALL 直接使用它，SHALL NOT 发起网络下载；摘要不一致时 SHALL 视为不存在并重新下载。安装程序预置的服务端即放在该缓存中。
- GitHub 在 10 秒内无响应或 20 秒没有数据时 SHALL 依次尝试内置镜像。
- 任何来源的文件 SHALL 在 SHA-256 与托盘内置摘要一致后才可安装；不一致 SHALL 丢弃并尝试下一个来源。
- 全部来源失败时 SHALL 报告每个来源的失败原因，WSL 中原有的文件 SHALL 保持不变。
- 开发版托盘的文件已不在 Release `dev`（GitHub 返回不存在）且缓存中没有时，失败原因 SHALL 说明“开发版 <版本> 的服务端已不在 dev Release，请先更新托盘”。

#### Scenario: GitHub 不可达

- **WHEN** GitHub 连接超时，第一个镜像返回正确文件
- **THEN** 安装成功，日志记录使用的来源

#### Scenario: 摘要不符

- **WHEN** 所有来源返回的文件摘要都与内置摘要不同
- **THEN** 安装失败并列出各来源原因，原有 webmux 不变，没有执行下载的文件

#### Scenario: 使用本地缓存

- **WHEN** 下载缓存中已有与托盘同版本、摘要匹配的 `linux-amd64` 文件，发行版为 `x86_64`，网络不可用
- **THEN** 安装成功，没有发起网络请求

#### Scenario: 本地缓存损坏

- **WHEN** 下载缓存中的文件被截断，摘要与内置摘要不同
- **THEN** 托盘丢弃该文件并按来源顺序重新下载，校验通过后安装

#### Scenario: 开发版托盘

- **WHEN** 托盘为 0.10.3-dev.4，缓存中没有对应文件，发行版为 `x86_64`
- **THEN** 托盘从 Release `dev` 下载 `webmux-0.10.3-dev.4-linux-amd64`，摘要与内置摘要一致后安装

#### Scenario: 开发版服务端已删除

- **WHEN** 便携版托盘为 0.10.3-dev.4，Release `dev` 中只剩 dev.6 与 dev.7 的产物，缓存中没有对应文件
- **THEN** 安装失败，状态行提示该开发版的服务端已不在 dev Release、请先更新托盘，WSL 中原有文件不变
