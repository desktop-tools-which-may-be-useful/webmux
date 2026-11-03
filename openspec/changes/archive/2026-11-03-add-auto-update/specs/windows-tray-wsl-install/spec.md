# Spec Delta

## MODIFIED Requirements

### Requirement: 一键安装与升级

处于缺失、不兼容或较旧状态时，托盘菜单 SHALL 提供“安装 WSL 中的 webmux”或“升级 WSL 中的 webmux”。安装 SHALL 写入托盘设置中的 webmux 路径（默认 `~/.local/bin/webmux`），必要时创建父目录，文件权限为 0755。是否需要用户点击按状态区分：

- 缺失：托盘 SHALL NOT 在用户未选择该动作时下载或改写 WSL 中的文件。
- 较旧或“不兼容：WSL 过旧”：托盘为发布构建且 `autoUpdate` 开启时，SHALL 在检查完成后自动执行升级；失败时 SHALL 在状态行显示原因，并 SHALL NOT 在下次检查时机之前重试。
- “不兼容：WSL 过新”或无法判断：SHALL NOT 自动改写。

#### Scenario: 缺失时安装

- **WHEN** WSL 中没有 `~/.local/bin/webmux`，用户点击“安装 WSL 中的 webmux”
- **THEN** 该路径出现与托盘同版本的可执行文件，托盘状态变为“已停止”，“启动”可用

#### Scenario: 不自动安装

- **WHEN** 托盘检测到 webmux 缺失，用户没有点击安装
- **THEN** WSL 中的文件系统没有任何变化

#### Scenario: 自动升级较旧版本

- **WHEN** 托盘 0.5.0 开启自动更新，WSL 中的 webmux 为 0.4.0，WebMux 正在运行并有 2 个会话
- **THEN** 托盘自动把配置路径上的文件升级到 0.5.0，2 个会话保持可用，状态行提示重启后生效

#### Scenario: 关闭自动更新

- **WHEN** `autoUpdate` 为 `false`，WSL 中的 webmux 较旧
- **THEN** WSL 中的文件不变，菜单提供“升级 WSL 中的 webmux”

#### Scenario: 路径不可写

- **WHEN** 托盘设置中的路径为 `/usr/bin/webmux`，WSL 默认用户没有写权限，自动升级触发
- **THEN** 升级失败，状态行显示无法写入该路径，原有文件不变
