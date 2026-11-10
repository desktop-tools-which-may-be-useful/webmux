# Spec Delta

## MODIFIED Requirements

### Requirement: 非安全上下文提示

页面不在安全上下文时，卡片 SHALL 说明推送需要 HTTPS，并给出操作步骤：用 Cloudflare Named Tunnel、Tailscale Serve/Funnel 或其他已授权 HTTPS 入口打开 WebMux，再在该地址开启推送。卡片 SHALL 同时说明页内弹窗提醒与提示音在当前地址仍然可用。

#### Scenario: 通过 VPN 地址访问

- **WHEN** 手机通过 `http://100.64.0.2:7681` 打开设置页
- **THEN** 卡片说明需要 HTTPS，并列出改用 Tailscale Serve/Funnel 或 Named Tunnel 等 HTTPS 入口打开的步骤，同时说明页内弹窗与提示音在当前地址可用
