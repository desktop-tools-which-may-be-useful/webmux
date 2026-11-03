# Spec Delta

## MODIFIED Requirements

### Requirement: 发布产物

每个标签发布的 Release SHALL 包括以下产物（Linux 安装包见“Linux 安装包”；Release `dev` 的产物见“开发版产物命名”）：

- Linux：`webmux-linux-amd64`、`webmux-linux-arm64`；
- macOS：`webmux-darwin-amd64`、`webmux-darwin-arm64`；
- Windows：`webmux-windows-amd64.exe`、`webmux-windows-arm64.exe`；
- Windows 托盘：`webmux-tray-windows-amd64.exe`、`webmux-tray-windows-arm64.exe`；
- Windows 安装程序：`webmux-setup-windows-amd64.exe`、`webmux-setup-windows-arm64.exe`；
- 校验和：`SHA256SUMS`；
- 校验和签名：`SHA256SUMS.minisig`。

#### Scenario: 产物齐全

- **WHEN** 标签 `v0.2.0` 的发布流水线成功
- **THEN** Release 中包含上述每个文件，`SHA256SUMS.minisig` 能以内置公钥验证 `SHA256SUMS`

### Requirement: 校验和文件

`SHA256SUMS` 的格式 SHALL 与 `sha256sum` 输出一致，可用 `sha256sum -c` 直接校验。

- 标签发布：SHALL 为 Release 中除 `SHA256SUMS.minisig` 外的每个其他产物各列一行。
- Release `dev`：SHALL 只为本次构建的产物（包括 `install.sh`）各列一行；为下载中的客户端保留的上一次构建产物 SHALL NOT 列在其中。

#### Scenario: 用户校验下载

- **WHEN** 用户下载 `webmux-linux-amd64` 与 `SHA256SUMS` 后执行 `sha256sum -c --ignore-missing SHA256SUMS`
- **THEN** 该文件校验通过

#### Scenario: 开发版清单不列旧产物

- **WHEN** Release `dev` 中有 0.10.3-dev.2 与 0.10.3-dev.3 的产物，当前清单为 dev.3
- **THEN** `SHA256SUMS` 只列出 dev.3 的产物

## ADDED Requirements

### Requirement: 开发版发布

main 上的提交通过 CI 后，开发版流水线 SHALL 以开发版版本构建与正式版相同的二进制、托盘、安装程序与 Linux 安装包，并发布到唯一的 Release `dev`。

- 版本 SHALL 为最近一个 `vX.Y.Z` 正式标签的补丁号加 1，加 `-dev.N`，`N` 为该标签之后的提交数；`N` 为 0（提交本身即正式标签）时 SHALL NOT 发布。
- 流水线 SHALL 构建通过 CI 的那个提交。发布前该提交已不是 main 的最新提交，或其版本不高于 Release `dev` 当前清单的版本时，SHALL NOT 发布。
- Release `dev` SHALL 标记为预发布，SHALL NOT 成为 `latest`；替换清单后标签 `dev` SHALL 指向本次提交。
- 连续推送时 SHALL 只发布最后一次；拉取请求、非 main 分支与其他仓库 SHALL NOT 触发开发版发布。
- 替换清单的步骤一旦开始 SHALL NOT 被新的运行取消。

#### Scenario: 合入后发布开发版

- **WHEN** v0.10.2 之后的第 2 个提交合入 main 且 CI 通过
- **THEN** Release `dev` 的签名清单可信注释为 `webmux 0.10.3-dev.2`，`latest` 仍指向 v0.10.2

#### Scenario: 较早的提交后完成测试

- **WHEN** 提交 A、B 依次合入 main，B 的 CI 先通过并发布为 0.10.3-dev.5，随后 A 的 CI 通过
- **THEN** A 不发布，Release `dev` 的清单仍为 0.10.3-dev.5，标签 `dev` 仍指向 B

#### Scenario: 正式标签提交

- **WHEN** 打上 `v0.10.3` 的提交推送到 main 且 CI 通过
- **THEN** 开发版流水线不发布，Release `dev` 不变

### Requirement: 开发版跳过只改文档的变更

自 Release `dev` 当前发布的提交以来只改了文档时，开发版流水线 SHALL NOT 构建与发布。

- 判定 SHALL 使用当前发布提交中的变更分类规则，SHALL NOT 采用待发布提交中的规则。
- 分类器要求完整运行的变更（工具链、未知路径等）SHALL 照常发布。
- 当前发布的提交不存在或不是本次提交的祖先时 SHALL 照常发布。

#### Scenario: 只改文档

- **WHEN** Release `dev` 当前为 0.10.3-dev.5，之后合入 main 的提交只修改了 `docs/` 与 `openspec/` 下的 Markdown，CI 通过
- **THEN** 开发版流水线不构建也不发布，Release `dev` 仍为 0.10.3-dev.5

#### Scenario: 文档之后的代码提交

- **WHEN** 接上例，再合入一个修改 `internal/` 的提交，它是 v0.10.2 之后的第 7 个提交
- **THEN** Release `dev` 发布 0.10.3-dev.7

### Requirement: 开发版产物命名

开发版产物文件名 SHALL 带版本号，且 SHALL NOT 含有 GitHub 会改写的字符：

- 二进制、托盘与安装程序：`webmux-<版本>-<正式版文件名去掉 webmux- 前缀>`；
- deb：`webmux_<版本>-1_<架构>.deb`；rpm：`webmux-<版本>-1.<架构>.rpm`；包内版本 SHALL 为 `X.Y.Z~dev.N`。

Release `dev` SHALL 以固定文件名包含 `install.sh`，内容与同一提交中的脚本相同；它 SHALL 列在本次清单中，SHALL 随清单一起替换，不为上一次构建保留。

#### Scenario: 产物名

- **WHEN** 开发版 0.10.3-dev.2 发布完成
- **THEN** Release `dev` 包含 `webmux-0.10.3-dev.2-linux-amd64`、`webmux-0.10.3-dev.2-setup-windows-amd64.exe` 与 `webmux_0.10.3-dev.2-1_amd64.deb`，均列在 `SHA256SUMS` 中
- **AND** 该 deb 的包内版本为 `0.10.3~dev.2-1`

### Requirement: 开发版发布顺序与保留

开发版发布 SHALL 依次：读取 Release `dev` 当前的 `SHA256SUMS`，其中列出的文件为“上一次构建”；上传本次带版本号的产物；替换 `install.sh`、`SHA256SUMS` 与签名；移动标签 `dev`；删除 Release 中既不在本次清单、也不在上一次清单中的文件。

- 替换清单之前任何一步失败时 SHALL NOT 替换清单，已列在当前清单中的产物 SHALL 保持不变；上传了一部分的本次产物 SHALL 由下一次成功的发布删除。
- 替换清单之后删除失败时，SHALL 只留下多余的文件，由下一次成功的发布删除。
- 判断“版本不高于当前清单”时 SHALL 只采用能以内置公钥验证的现有清单与签名；缺失、不配对或无法验证时 SHALL 视为未发布并照常发布。

#### Scenario: 只保留两次构建

- **WHEN** 开发版 0.10.3-dev.3 发布完成，此前 Release `dev` 中有 dev.1 与 dev.2 的产物，当前清单为 dev.2
- **THEN** Release `dev` 中只剩 dev.2 与 dev.3 的产物以及 `install.sh`，签名清单列出 dev.3

#### Scenario: 上次替换清单中途失败

- **WHEN** 发布 0.10.3-dev.3 时签名已替换为 dev.3 而 `SHA256SUMS` 仍是 dev.2 的，随后重跑 0.10.3-dev.3 的发布
- **THEN** 重跑判定现有清单无法验证，重新上传 dev.3 的清单与签名，Release `dev` 恢复为可验证的 dev.3

#### Scenario: 发布中途失败

- **WHEN** 上传 0.10.3-dev.3 的产物时失败，之后 0.10.3-dev.4 发布成功
- **THEN** 失败后签名清单仍为 dev.2，dev.2 的产物全部保留；dev.4 发布后 Release `dev` 中只剩 dev.2 与 dev.4 的产物

### Requirement: 开发版签名

开发版 SHALL 使用与正式版不同的私钥签名，私钥 SHALL 只来自流水线密钥配置。开发版的构建、签名与发布 SHALL 采用与“发布签名”相同的隔离：构建环境看不到私钥，签名环境只接收 `SHA256SUMS` 并只运行仓库自带的签名工具。

- 发布前 SHALL 以内置的开发版公钥组验证签名。
- 开发版公钥 SHALL 内置于发布构建，并在仓库文档中与正式版公钥分开公布。

#### Scenario: 缺少开发版密钥

- **WHEN** main 推送后流水线没有配置开发版私钥
- **THEN** 开发版发布失败，Release `dev` 保持上一次的内容
