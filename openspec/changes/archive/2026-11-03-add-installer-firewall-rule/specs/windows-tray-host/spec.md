# Spec Delta

## ADDED Requirements

### Requirement: 防火墙规则命令

托盘程序 SHALL 提供 `firewall allow` 与 `firewall remove [路径]` 子命令，为自身所在路径的 `webmux-tray.exe` 添加或删除转发所用的入站防火墙规则；`remove` 可以指定另一个托盘的绝对路径，删除该路径的规则，相对路径 SHALL 视为参数无效。该命令 SHALL NOT 创建托盘图标，也 SHALL NOT 影响正在运行的托盘。

- 规则名称 SHALL 以 `WebMux tray forwarding` 开头并标识该托盘路径，不同路径的规则互不影响。
- 规则 SHALL 为入站、允许、TCP，只适用于专用网络与域网络。
- 规则已是目标状态时 SHALL 直接以 0 结束，不请求管理员权限；否则 SHALL 弹出一次管理员确认并以 argv 运行系统的 `netsh`，完成后再次确认规则状态。
- 退出码：0 已是目标状态；1 失败；2 参数无效；3 用户取消管理员确认。

#### Scenario: 已有规则

- **WHEN** 规则已存在，用户运行 `webmux-tray.exe firewall allow`
- **THEN** 不出现管理员确认，命令以 0 结束

#### Scenario: 取消确认

- **WHEN** 没有规则，用户运行 `webmux-tray.exe firewall allow` 并在管理员确认中选择“否”
- **THEN** 命令以 3 结束，防火墙规则不变

#### Scenario: 删除不存在的规则

- **WHEN** 没有规则，运行 `webmux-tray.exe firewall remove`
- **THEN** 不出现管理员确认，命令以 0 结束
