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
- 校验和：`SHA256SUMS`；
- 校验和签名：`SHA256SUMS.minisig`。

每个二进制 SHALL 报告该版本号，并 SHALL 标记为发布构建。构建、测试或签名失败时 SHALL NOT 创建或更新 Release。

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

`SHA256SUMS` SHALL 为 Release 中除 `SHA256SUMS.minisig` 外的每个其他产物各列一行，格式与 `sha256sum` 输出一致，可用 `sha256sum -c` 直接校验。

#### Scenario: 用户校验下载

- **WHEN** 用户下载 `webmux-linux-amd64` 与 `SHA256SUMS` 后执行 `sha256sum -c --ignore-missing SHA256SUMS`
- **THEN** 该文件校验通过

## ADDED Requirements

### Requirement: 发布签名

发布流水线 SHALL 用发布私钥对 `SHA256SUMS` 生成 minisign 格式的 `SHA256SUMS.minisig`，可信注释为 `webmux <版本>`，并 SHALL 在发布前以内置公钥验证该签名。

- 私钥 SHALL 只来自流水线密钥配置，SHALL NOT 出现在仓库、构建日志或产物中。
- 构建、测试与打包 SHALL 在看不到私钥的环境中完成；签名 SHALL 在独立环境中进行，该环境只接收 `SHA256SUMS`，SHALL NOT 运行前端依赖、第三方 Go 模块或打包工具。
- 密钥缺失或签名失败时 SHALL NOT 创建 Release。
- 公钥 SHALL 内置于发布构建并在仓库文档中公布，使用户可以用 `minisign -Vm SHA256SUMS -P <公钥>` 手动验证。

#### Scenario: 用户手动验证

- **WHEN** 用户下载 `SHA256SUMS` 与 `SHA256SUMS.minisig`，以文档中的公钥执行 `minisign -Vm SHA256SUMS -P <公钥>`
- **THEN** 验证通过，输出的可信注释为 `webmux 0.5.0`

#### Scenario: 缺少签名密钥

- **WHEN** 推送标签时流水线没有配置发布私钥
- **THEN** 发布失败，不出现该标签的 Release

#### Scenario: 构建环境拿不到私钥

- **WHEN** 发布构建中的某个依赖在构建时读取环境变量
- **THEN** 它读不到发布私钥，签名环境只运行仓库自带的签名工具
