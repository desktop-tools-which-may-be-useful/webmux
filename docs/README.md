# WebMux 文档中心

这里是所有文档的入口，按“你想做什么”分组。每份文档都能从本页或 [AGENTS.md](../AGENTS.md) 沿链接找到，`make docs-check` 会拒绝无法到达的文档与失效链接（规则见 [文档治理](doc-governance.md)）。

## 了解 WebMux

- [项目介绍](overview.md)：定位、设计理念、全部特性、混合会话概览与技术栈。
- [User Stories](user-stories.md)：每项功能的用户目的与验收要点，适合评审和查找“某个功能能做什么”。
- [常见问题与获取帮助](faq.md)：常见疑问、排障入口与反馈渠道。

## 安装与部署

- [部署指南](deployment.md)：总索引。
  - [安装与升级](deployment/install-and-upgrade.md)：一键脚本、deb / rpm、手动下载与升级。
  - [配置](deployment/configuration.md)：命令行参数、配置项与状态目录。
  - [服务管理与平台差异](deployment/services.md)：systemd / launchd、agent 的 PATH。
  - [远程访问与 HTTPS](deployment/remote-access.md)：手机访问、访问链接、HTTPS。
  - [安装为应用与手机推送](deployment/pwa-and-push.md)：PWA 与 Web Push。
  - [Windows（WSL）部署](deployment/windows-wsl.md)：托盘、控制台客户端与故障诊断。
- [Cloudflare Named Tunnel 配置指南](cloudflare-tunnel.md)：固定的 HTTPS 公网入口。
- [Tailscale Serve / Funnel](tailscale.md)：自动证书的 tailnet / 公网入口。
- [安全策略](../SECURITY.md)：安全模型、支持的版本与漏洞报告。

## 日常使用

- [使用指南](usage.md)：会话、新建会话、后台启动、终端附着、草稿、代码工作区、仓库与 worktree、移动端。
  - [终端使用](usage/terminal.md)：查找、分屏、复制粘贴、终端图形与文件上传。
- [混合会话](hybrid-sessions.md)：OpenCode / omp / Codex CLI / Claude Code / Qoder / QoderCN 在聊天视图与终端间切换。
- [CLI agent 接入](cli-agents.md)：预置的十一款 CLI agent 启动配置与提醒。

## 开发与架构

- [参与贡献](../CONTRIBUTING.md)：开发环境、常用命令、测试、PR 与 OpenSpec 工作流。
- [架构设计](architecture.md)：会话模型、缓冲与回放、状态探测、分层规则 R0–R10。
- [API 与协议](protocol.md)：REST、WebSocket 帧与错误码（按主题分页）。
- [配置与持久化清单](persistence.md)：状态目录中每类文件的格式与治理。
- [OpenSpec 规格索引](specs.md)：按主题列出 `openspec/specs/` 下的全部 capability。
- 代码规范：[Go](go-style.md)、[前端](frontend-style.md)、[前端架构守卫](frontend-architecture.md)。
- 界面：[设计系统](design-system.md)、[Storybook 与防溢出测试](storybook.md)。

## 性能与质量

- [布局稳定性与 4× CPU 回归](performance.md)：性能测试的测量方式与阈值。
- [页面性能与连接稳定性验证](page-performance.md)：页面加载与重连的实测记录。
- [内嵌二进制体积](binary-size.md)：发布二进制的体积对比。
- [CI 耗时优化与验收](ci-performance.md)：CI 墙钟时间的优化记录。
- [产品能力 baseline](../baseline/README.md)：可复现评测、统计口径与量化结果。

## 官网与推广

- [官网建设指南](website.md)：定位、静态站点生成器选型、页面清单、视觉规范、交互与 SEO 规则、站点功能与验收清单。

## 文档与 Agent 规则

- [文档治理](doc-governance.md)：文档规模、链接可达性与 README 预算。
- [Agent 规则](agent-rules.md)：从 `AGENTS.md` 迁出的仓库地图、约定、测试与 User Stories 维护规则。
