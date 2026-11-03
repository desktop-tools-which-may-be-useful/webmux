# Spec Delta

## MODIFIED Requirements

### Requirement: 仅发布构建参与更新

只有由发布流水线（标签发布或开发版发布）构建的二进制 SHALL 参与更新检查与自动更新。本地 `make build`、`make tray` 或 `go build` 的产物即使报告 `MAJOR.MINOR.PATCH` 或 `MAJOR.MINOR.PATCH-dev.N` 版本号，也 SHALL 视为非发布构建：不发起检查，并在被要求检查时报告“此构建不是发布构建，不支持更新”。

#### Scenario: 本地构建

- **WHEN** 用户运行本地 `make build` 产出的 `bin/webmux update check`，该二进制报告 `webmux 0.1.0`
- **THEN** 命令以非零退出码结束并报告“此构建不是发布构建，不支持更新”，没有发起网络请求

#### Scenario: 发布构建

- **WHEN** 用户运行 Release 中的 `webmux-linux-amd64 update check`
- **THEN** 命令发起更新检查

#### Scenario: 开发版发布构建

- **WHEN** 用户运行 Release `dev` 中的 `webmux-0.10.3-dev.4-linux-amd64 update check`
- **THEN** 命令发起更新检查

### Requirement: 内置公钥验证

发布构建 SHALL 内置两组发布公钥：正式版组与开发版组。只接受由其中某个公钥签名的 `SHA256SUMS`。签名 SHALL 为 minisign 格式；验证 SHALL 覆盖文件内容与可信注释。

验证失败、签名文件缺失或公钥 ID 不在内置集合中时，该来源 SHALL 视为失败，SHALL NOT 使用其中任何摘要。下载得到的 `SHA256SUMS` 不经签名验证 SHALL NOT 用作校验依据。

- 正式版组的公钥 SHALL 只用于不带后缀的版本，开发版组的公钥 SHALL 只用于 `-dev.N` 版本；版本与签名公钥所属的组不匹配时 SHALL 视为签名无效。
- `stable` 通道 SHALL 只信任正式版组。

#### Scenario: 镜像篡改

- **WHEN** 某镜像返回被修改的二进制和与之匹配、但未经有效签名的 `SHA256SUMS`
- **THEN** 该来源被判定失败并记录“签名无效”，更新程序尝试下一个来源，不安装该二进制

#### Scenario: 未知公钥

- **WHEN** `SHA256SUMS.minisig` 由不在内置集合中的密钥签名
- **THEN** 该来源被判定失败并记录“未知签名密钥”

#### Scenario: 开发版密钥签出正式版号

- **WHEN** 某清单由开发版组的公钥签名，可信注释为 `webmux 9.9.9`
- **THEN** 该清单被判定签名无效，任何通道都不使用它

#### Scenario: 正式版通道不信任开发版密钥

- **WHEN** 通道为 `stable`，某来源返回开发版组签名的 `webmux 0.10.3-dev.4` 清单
- **THEN** 该来源被判定失败，更新程序不考虑 0.10.3-dev.4

### Requirement: 签名绑定版本

签名的可信注释 SHALL 为 `webmux <版本>`。`<版本>` 为不带 `v` 的 `MAJOR.MINOR.PATCH`，或开发版的 `MAJOR.MINOR.PATCH-dev.N`（`N` 为无前导零的正整数）；其他后缀 SHALL 视为格式不符。更新程序 SHALL 以可信注释中的版本作为该清单的版本；可信注释格式不符时 SHALL 视为签名无效。

随后下载的安装产物 SHALL 由版本决定来源，SHALL NOT 来自 `latest` 路径：

- 正式版来自标签 `v<该版本>`，使用固定文件名；
- 开发版来自标签 `dev`，使用带版本号的文件名（见 `release-distribution` 的“开发版产物命名”）。

#### Scenario: 旧清单冒充新版本

- **WHEN** 来源在 `v0.5.0` 路径下返回 `v0.4.0` 的已签名 `SHA256SUMS`
- **THEN** 更新程序把该清单视为 0.4.0，不把它当作 0.5.0 使用

#### Scenario: 开发版产物来源

- **WHEN** 已验证的清单版本为 0.10.3-dev.4，需要 `linux-amd64` 二进制
- **THEN** 更新程序从 `releases/download/dev/webmux-0.10.3-dev.4-linux-amd64` 下载并按清单校验

#### Scenario: 不支持的后缀

- **WHEN** 已签名清单的可信注释为 `webmux 0.11.0-rc.1`
- **THEN** 该清单被判定签名无效

### Requirement: 最新版本发现

更新程序 SHALL 按通道读取 `SHA256SUMS` 与 `SHA256SUMS.minisig`，以验证通过的可信注释版本作为该清单的版本：

- `stable` 通道读取 `releases/latest/download/`；
- `dev` 通道读取 `releases/download/dev/` 与 `releases/latest/download/` 两个清单，各自按下列来源规则取得结果后取版本较高的一个；一个清单失败 SHALL NOT 影响另一个。

每个清单的来源规则：

- 来源顺序 SHALL 为 GitHub，然后依次为内置镜像。
- GitHub 返回验证通过的清单时 SHALL 直接采用，SHALL NOT 再访问镜像。
- GitHub 失败时 SHALL 读取所有镜像，采用验证通过的清单中版本最高的一份（版本相同取靠前的镜像）；某个镜像失败或签名无效 SHALL NOT 使其他镜像验证通过的结果作废。
- 连接 10 秒无响应或 20 秒没有数据时 SHALL 改用下一个来源。
- 所有来源失败时 SHALL 报告每个来源的失败原因，且不改变任何已安装文件。

#### Scenario: GitHub 不可达

- **WHEN** GitHub 连接超时，只有第一个镜像返回有效签名的清单
- **THEN** 更新程序以该清单的版本作为最新版本，日志记录使用的来源

#### Scenario: 镜像缓存了旧清单

- **WHEN** GitHub 连接超时，第一个镜像返回有效签名的 0.5.0 清单，第二个镜像返回有效签名的 0.6.0 清单
- **THEN** 更新程序以 0.6.0 作为最新版本，日志记录第二个镜像为来源

#### Scenario: 全部失败

- **WHEN** 所有来源都超时或签名无效
- **THEN** 检查失败并列出每个来源的原因，已安装文件不变

#### Scenario: 开发版通道取较高版本

- **WHEN** 通道为 `dev`，开发版清单为 0.10.3-dev.4，正式版清单为 0.10.2
- **THEN** 最新版本为 0.10.3-dev.4

#### Scenario: 开发版清单不可用

- **WHEN** 通道为 `dev`，开发版清单在所有来源都失败，正式版清单为 0.10.2
- **THEN** 最新版本为 0.10.2，日志记录开发版清单的失败原因

### Requirement: 只升级不降级

更新程序 SHALL 只在最新版本严格高于当前版本时安装。版本按 `MAJOR.MINOR.PATCH` 比较；三段相同时，带 `-dev.N` 的版本低于不带后缀的版本，两个开发版按 `N` 比较。最新版本等于或低于当前版本时 SHALL 报告“已是最新版本”，SHALL NOT 下载安装产物。

#### Scenario: 来源返回旧版本

- **WHEN** 当前版本为 0.5.0，镜像返回有效签名的 0.4.0 清单
- **THEN** 更新程序报告已是最新版本，不下载也不安装

#### Scenario: 开发版之间

- **WHEN** 当前版本为 0.10.3-dev.4，最新版本为 0.10.3-dev.7
- **THEN** 更新程序安装 0.10.3-dev.7

#### Scenario: 正式版高于同号开发版

- **WHEN** 当前版本为 0.10.3-dev.7，最新版本为 0.10.3
- **THEN** 更新程序安装 0.10.3

### Requirement: 检查更新命令

`webmux update check` SHALL 在任意安装方式下按当前通道执行一次检查，输出当前通道、当前版本、最新版本与是否有可用更新，SHALL NOT 下载安装产物或改写任何文件。

- `--json` 时 SHALL 只向标准输出写入一个 JSON 对象，包含 `v`、`current`、`latest`、`available`、`installMethod`（`deb`、`rpm` 或 `other`）与 `channel`（`stable` 或 `dev`）。读取方遇到缺少 `channel` 的旧版本输出时 SHALL 按 `stable` 理解。
- 检查成功时无论是否有更新，退出码 SHALL 为 0；检查失败时 SHALL 为非零。

#### Scenario: 手动安装检查

- **WHEN** 用户以 `install.sh` 安装 0.4.0，最新版本为 0.5.0，执行 `webmux update check`
- **THEN** 输出显示 0.4.0 与 0.5.0、有可用更新，并提示重新运行 `install.sh` 升级；`~/.local/bin/webmux` 不变

#### Scenario: JSON 输出

- **WHEN** deb 安装的 0.5.0 执行 `webmux update check --json`，通道为 `stable`，最新版本为 0.5.0
- **THEN** 标准输出为 `{"v":1,"current":"0.5.0","latest":"0.5.0","available":false,"installMethod":"deb","channel":"stable"}`
