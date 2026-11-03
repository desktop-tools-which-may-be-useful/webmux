# Spec Delta

## Purpose

定义 WebMux 自动更新的正式版与开发版两个通道：各安装方式如何选择并保存通道，以及切换通道时只升级不降级的规则，使尝鲜用户能跟随 main 的构建，而正式版用户不受影响。

## ADDED Requirements

### Requirement: 通道定义

更新 SHALL 支持两个通道：`stable`（正式版，只包含标签发布的版本）与 `dev`（开发版，另外包含 main 上通过 CI 的构建）。

- 未选择通道或保存通道的位置不存在时 SHALL 为 `stable`，SHALL NOT 记录警告。
- 保存的通道值无法识别时 SHALL 按 `stable` 处理并记录警告。
- `stable` 通道 SHALL 只考虑正式版。
- `dev` 通道 SHALL 同时考虑开发版与正式版，取其中最高的已验证版本，因此正式版发布后 `dev` 通道会升到该正式版。

#### Scenario: 默认通道

- **WHEN** 新安装的 deb 没有任何通道配置，最新正式版为 0.10.2，最新开发版为 0.10.3-dev.4
- **THEN** 更新程序只考虑 0.10.2，日志中没有通道相关的警告

#### Scenario: 开发版用户收到正式版

- **WHEN** 通道为 `dev`，当前为 0.10.3-dev.4，正式版 0.10.3 发布
- **THEN** 下一次检查安装 0.10.3

### Requirement: 命令行的通道选择

Linux 与 macOS 上的通道 SHALL 保存在 `/etc/webmux/update-channel`，内容为 `stable` 或 `dev`。该文件 SHALL NOT 属于任何软件包，安装、升级与卸载 SHALL NOT 改动它。

- `webmux update channel` SHALL 显示当前通道，不需要 root。
- `webmux update channel stable|dev` SHALL 要求 root；必要时创建 `/etc/webmux`（0755），以临时文件加改名的方式写入 0644 的文件，下一次检查即生效；其他参数 SHALL 视为用法错误。
- 该文件不归 root 所有，或可被组、其他用户写入时，SHALL 按 `stable` 处理并记录警告。
- `webmux update check` 与 `webmux update apply` SHALL 使用该文件中的通道。

#### Scenario: 切换到开发版

- **WHEN** 管理员执行 `sudo webmux update channel dev`，最新开发版为 0.10.3-dev.4
- **THEN** `/etc/webmux/update-channel` 内容为 `dev`，下一次更新服务运行安装 `webmux_0.10.3-dev.4-1_amd64.deb`

#### Scenario: 普通用户修改通道

- **WHEN** 普通用户执行 `webmux update channel dev`
- **THEN** 命令报告需要 root 权限并失败，通道不变

#### Scenario: 文件可被普通用户写入

- **WHEN** `/etc/webmux/update-channel` 内容为 `dev`，权限为 0666
- **THEN** 更新程序按 `stable` 检查并记录警告

### Requirement: 托盘的通道选择

托盘设置 SHALL 包含 `updateChannel`，取值 `stable` 或 `dev`，缺省为 `stable`。托盘菜单 SHALL 提供“更新通道”，下设“正式版”“开发版”两个单选项；切换后 SHALL 立即写入设置并发起一次更新检查。非发布构建的托盘 SHALL 禁用该菜单。

#### Scenario: 托盘切到开发版

- **WHEN** 安装版托盘 0.10.2 开启自动更新，用户在“更新通道”中选择“开发版”，最新开发版为 0.10.3-dev.4
- **THEN** 托盘立即检查，下载并静默安装 0.10.3-dev.4 的安装程序

### Requirement: 安装脚本的通道选择

`install.sh` SHALL 支持环境变量 `WEBMUX_CHANNEL`，取值 `stable`（默认）或 `dev`；其他取值 SHALL 报错退出且不改动已安装文件。该变量只影响本次安装，SHALL NOT 写入任何通道配置。Release `dev` 中的 `install.sh` 与正式版 Release 中的是同一份脚本，SHALL 同样默认 `stable`。

- 为 `dev` 时 SHALL 从 Release `dev` 读取 `SHA256SUMS`，从中找出唯一一个与本机系统、架构匹配的 `webmux-<X.Y.Z-dev.N>-<系统>-<架构>` 行，以文件名中的版本安装该二进制；找不到或多于一行时 SHALL 报错退出。
- 下载的二进制 SHALL 与同一来源 `SHA256SUMS` 中的摘要一致后才安装，与正式版安装相同。

#### Scenario: 安装开发版

- **WHEN** 用户执行 `curl -fsSL <Release dev 的 install.sh> | WEBMUX_CHANNEL=dev sh`，最新开发版为 0.10.3-dev.4
- **THEN** `~/.local/bin/webmux version` 输出 `webmux 0.10.3-dev.4`

### Requirement: 安装脚本指定版本

`install.sh` 的 `WEBMUX_VERSION` SHALL 接受 `X.Y.Z` 或 `X.Y.Z-dev.N`。

- 开发版版本隐含 `dev` 通道，且 SHALL 等于 Release `dev` 当前清单的版本；不等时 SHALL 报错退出并给出当前开发版版本。
- `WEBMUX_CHANNEL=dev` 搭配 `X.Y.Z`，或 `WEBMUX_CHANNEL=stable` 搭配开发版版本时，SHALL 报错退出。
- 报错时 SHALL NOT 改动已安装文件。

#### Scenario: 指定当前开发版

- **WHEN** Release `dev` 当前为 0.10.3-dev.4，用户执行 `curl -fsSL <install.sh> | WEBMUX_VERSION=0.10.3-dev.4 sh`
- **THEN** 脚本安装 0.10.3-dev.4

#### Scenario: 指定已过期的开发版

- **WHEN** Release `dev` 当前为 0.10.3-dev.4，用户执行 `curl -fsSL <install.sh> | WEBMUX_VERSION=0.10.3-dev.3 sh`
- **THEN** 脚本报错退出，说明 0.10.3-dev.3 已不可用、当前开发版为 0.10.3-dev.4，已安装文件不变

#### Scenario: 通道与版本矛盾

- **WHEN** 用户执行 `curl -fsSL <install.sh> | WEBMUX_CHANNEL=dev WEBMUX_VERSION=0.10.2 sh`
- **THEN** 脚本报错退出，已安装文件不变

### Requirement: 切换通道只升级不降级

切换通道 SHALL NOT 导致降级。从 `dev` 切回 `stable` 时，若当前版本高于最新正式版，更新程序 SHALL 不安装任何版本，并说明当前开发版高于最新正式版、正式版超过它后会自动更新。

#### Scenario: 切回正式版

- **WHEN** 当前为 0.10.3-dev.4，通道切换为 `stable`，最新正式版为 0.10.2
- **THEN** 不下载也不安装，`webmux update check` 报告当前开发版高于最新正式版

#### Scenario: 正式版追上

- **WHEN** 同上，之后正式版 0.10.3 发布
- **THEN** 下一次检查安装 0.10.3
