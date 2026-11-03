# 部署指南

部署内容按主题分在以下文件中；每个文件保留了原章节标题与锚点。Windows（WSL）见 [Windows（WSL）部署](deployment/windows-wsl.md)。

| 文件                                                        | 主题                 | 包含的章节                                                                                                  |
| ----------------------------------------------------------- | -------------------- | ----------------------------------------------------------------------------------------------------------- |
| [install-and-upgrade.md](deployment/install-and-upgrade.md) | 安装与升级           | 运行要求、安装、升级、持久化升级与诊断                                                                      |
| [configuration.md](deployment/configuration.md)             | 配置                 | 配置                                                                                                        |
| [services.md](deployment/services.md)                       | 服务管理与平台差异   | 服务管理命令、systemd 用户服务（Linux）、launchd 用户代理（macOS）、Windows（WSL）、agent 的 PATH、平台差异 |
| [remote-access.md](deployment/remote-access.md)             | 远程访问与 HTTPS     | 手机访问（Tailscale）、访问链接、公网访问（Cloudflare Tunnel）、HTTPS                                       |
| [pwa-and-push.md](deployment/pwa-and-push.md)               | 安装为应用与手机推送 | 安装为应用（PWA）、手机推送（Web Push）                                                                     |
| [windows-wsl.md](deployment/windows-wsl.md)                 | Windows（WSL）部署   | 前置条件、安装与托盘设置、启动方式、远程转发、Token 与剪贴板、故障诊断                                      |
| [updates.md](deployment/updates.md)                         | 自动更新             | deb / rpm 定时更新、托盘自更新、检查更新、发布签名与密钥轮换                                                |

## 常见入口

- 首次安装：[安装](deployment/install-and-upgrade.md#安装)；已装好要做成后台服务：[服务管理命令](deployment/services.md#服务管理命令)。
- 手机访问：[Tailscale](deployment/remote-access.md#手机访问tailscale)；公网：[Cloudflare Tunnel](deployment/remote-access.md#公网访问cloudflare-tunnel)；通知：[Web Push](deployment/pwa-and-push.md#手机推送web-push)。
- agent 找不到命令：[agent 的 PATH](deployment/services.md#agent-的-path)；升级会结束会话：[升级](deployment/install-and-upgrade.md#升级)。
- 配置项与状态目录：[配置](deployment/configuration.md)。
