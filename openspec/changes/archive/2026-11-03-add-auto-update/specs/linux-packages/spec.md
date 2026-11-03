# Spec Delta

## MODIFIED Requirements

### Requirement: 安装包内容

每个安装包 SHALL 只包含以下文件，并 SHALL NOT 声明运行时依赖：

- 可执行文件 `/usr/bin/webmux`（权限 0755），与同一 Release 中对应架构的 `webmux-linux-<架构>` 字节相同；
- systemd 用户单元 `/usr/lib/systemd/user/webmux.service`（权限 0644）；
- 系统单元 `/usr/lib/systemd/system/webmux-update.service` 与 `/usr/lib/systemd/system/webmux-update.timer`（权限 0644）。

安装包 SHALL 附带启用、保留与停用更新定时器的安装脚本，行为见 `linux-package-auto-update`。

#### Scenario: 查看 deb 内容

- **WHEN** 用户对 `webmux_0.2.0-1_amd64.deb` 执行 `dpkg-deb -c`
- **THEN** 列表中只有 `/usr/bin/webmux`、`/usr/lib/systemd/user/webmux.service`、`/usr/lib/systemd/system/webmux-update.service` 与 `/usr/lib/systemd/system/webmux-update.timer` 四个文件，安装后 `webmux version` 输出 `webmux 0.2.0`
