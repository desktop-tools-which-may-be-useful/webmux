# Spec Delta

## ADDED Requirements

### Requirement: 防火墙放行

托盘运行时 SHALL NOT 修改 Windows 防火墙。其他设备能否连到转发地址取决于防火墙：安装版可以在安装时添加专用网络与域网络的放行规则（见 `windows-installer` 的“防火墙规则”）；便携版或未添加规则时，由 Windows 在首次监听时弹出原生提示。

#### Scenario: 安装时已放行

- **WHEN** 安装时添加了防火墙规则，当前网络为专用网络，用户勾选转发地址
- **THEN** 转发开始监听时不出现 Windows 防火墙提示，同一网络中的其他设备可以连接

#### Scenario: 公用网络

- **WHEN** 当前网络被 Windows 识别为公用网络，用户勾选转发地址
- **THEN** 安装时添加的规则不适用，是否放行由 Windows 原生提示决定
