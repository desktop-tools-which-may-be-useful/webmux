# Spec Delta

## ADDED Requirements

### Requirement: Linux 安装包

标签发布 SHALL 另外产出 Linux 安装包并上传到同一 Release：`webmux_<版本>-1_amd64.deb`、`webmux_<版本>-1_arm64.deb`、`webmux-<版本>-1.x86_64.rpm`、`webmux-<版本>-1.aarch64.rpm`。

安装包 SHALL 由同一次构建的 `webmux-linux-<架构>` 打包，并各自在 `SHA256SUMS` 中占一行。安装包构建失败时 SHALL NOT 创建 Release。

#### Scenario: 发布包含安装包

- **WHEN** 维护者推送标签 `v0.2.0`
- **THEN** Release 中包含上述四个安装包，`sha256sum -c SHA256SUMS` 对它们全部通过
