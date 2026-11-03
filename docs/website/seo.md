# SEO 规范

官网的搜索目标是两类查询：品牌词（“webmux”）和问题词（“Claude Code 手机上用”“浏览器终端 断线 会话不丢”“ttyd 替代”）。前者靠权威信号（GitHub 互链、稳定域名），后者靠专门页面与长尾文档。技术 SEO 必须在第一阶段做完，内容 SEO 随页面扩展。

## 技术基础

- 全站静态 HTML，每页一个 H1，标题层级不跳级。
- `<title>` 不超过 60 个字符（中文约 30 字），格式“页面名 · WebMux”，首页为“WebMux：在浏览器里持续运行的终端，远程遥控 Claude Code 与 Codex”这类含关键词的完整句。
- `<meta name="description">` 120–155 个字符，写收益而不是功能堆砌；每页不同。
- URL 全小写 ASCII，使用连字符，末尾斜杠风格全站一致（推荐带斜杠），不使用中文或查询参数作为页面标识。
- 每页 `rel="canonical"` 指向自身的正式 URL；`www` 与裸域二选一并 301。
- `@astrojs/sitemap` 生成 `sitemap-index.xml`，包含各语言版本的 `xhtml:link` 备用链接；`robots.txt` 指向站点地图。
- 多语言：`<link rel="alternate" hreflang="zh-CN">`、`hreflang="en"` 与 `hreflang="x-default"`（指向中文版）两两互指。
- 404 返回真实 404 状态；删除或改名的页面做 301，不留“软 404”。
- 不使用 `noindex` 的页面不得被 `robots.txt` 屏蔽资源（CSS、图片）。

## 元信息与社交卡片

每页输出以下标签，OG 图按页面标题生成，模板见 [视觉规范](design.md#logo-使用)：

- `og:type`：首页为 `website`，其余页面为 `article`。
- `og:title`、`og:description`、`og:url`、`og:site_name`。
- `og:image`（1200 × 630，绝对 URL）与 `og:image:alt`。
- `og:locale`（`zh_CN` / `en_US`）与 `og:locale:alternate`。
- `twitter:card` 为 `summary_large_image`。

## 结构化数据

用 JSON-LD 输出，内容来自构建期数据，不手写版本号：

- 首页与下载页：`SoftwareApplication`，字段如下。
  - `name`、`applicationCategory: DeveloperApplication`、`operatingSystem: "Linux, macOS, Windows (WSL 2)"`。
  - `softwareVersion`（Release tag）、`datePublished`、`downloadUrl`。
  - `license`（MIT 的 SPDX 链接）、`offers`（价格 0）。
  - `screenshot`、`author`、`sameAs`（GitHub 仓库）。
- FAQ 页与首页 FAQ 区块：`FAQPage`，问题与答案与页面可见文本一字不差。
- 文档与 agent 子页：`BreadcrumbList`。
- 全站：`WebSite` 与 `Organization`（或 `Person`），`sameAs` 指向 GitHub。
- 更新日志每条：`SoftwareApplication` 的 `releaseNotes` 或简单 `Article`。

发布前用 Google Rich Results Test 与 Schema.org 校验器检查；结构化数据描述的内容必须在页面上可见。

## 关键词地图

每页只承接一个主题，关键词写进标题、首段、H2 与图片替代文本，不堆砌。核心词按用户语言分列：

| 页面          | 中文查询意图                                      | 英文查询意图                                    |
| ------------- | ------------------------------------------------- | ----------------------------------------------- |
| 首页          | 浏览器终端、网页终端、持久会话、远程遥控 agent    | web terminal, persistent terminal sessions      |
| 首页 / agents | Claude Code 手机、Codex 远程、手机上跑 agent      | run Claude Code from phone, remote coding agent |
| 对比          | ttyd 替代、tmux 手机、mosh 对比                   | ttyd alternative, tmux alternative for mobile   |
| 下载          | webmux 安装、webmux 下载、Linux 单二进制终端      | webmux install, download                        |
| Windows       | WSL 终端 手机、WSL 托盘                           | WSL web terminal, WSL tray                      |
| 安全          | web terminal 安全、公网暴露 shell、Tailscale 终端 | secure web terminal, Tailscale terminal         |
| 文档镜像      | 各功能名（混合会话、worktree、Web Push、分屏）    | hybrid session, git worktree agent, web push    |

每季度对照搜索平台的真实查询调整这张表；新增关键词先确认页面确实回答了该意图，再改文案。

## 内容规则

- 不复制 README 全文：官网文案重写并补充上下文，避免与 GitHub 页面形成重复内容；README 保留摘要并链接到官网。
- 每款 agent 一个落地页（`/agents/claude-code/` 等），回答“如何用 WebMux 远程跑 X”；这是长尾流量的主要来源。
- 更新日志每个版本独立 URL（`/changelog/v0.10.1/`），标题含版本号与日期。
- 文档镜像是第一阶段的内容主体：几十篇各自回答一个问题的文档页，让“混合会话”“worktree”“Web Push”等功能词有可收录的落地页。
  - 更多内容带来更多权重的前提：每页独立 URL、`canonical` 指向官网、被侧栏与营销页链接、标题与描述各不相同。
  - GitHub 上的 Markdown 不能设置 canonical，权威靠官网互链与 README 指向官网建立。
- 低价值页面不收录：面向 agent 的规则、核对日志等在同步时排除（见 [文档镜像](stack.md#文档镜像)）；它们会稀释站点主题，还会在 Search Console 里制造“已抓取但未编入索引”的噪音。
- 图片文件名用描述性英文（`session-overview-desktop.png`），替代文本描述内容。
- 避免的做法：门页、隐藏文字、机器翻译的低质量英文页、仅为关键词存在的空页面、自动生成的标签页。

## 性能与可抓取性

- Core Web Vitals 目标见 [性能预算](ux.md#性能预算)；字体自托管、图片带尺寸、关键 CSS 内联。
- 首屏图片 `fetchpriority="high"`，其余 `loading="lazy"`；图片提供 AVIF / WebP 与回退。
- 不加载第三方脚本，统计脚本 `defer` 且放在允许的 CSP 列表中。
- 内部链接：首页链接到所有一级页面；每个功能描述链接到对应文档或 agent 页；页脚是完整站点地图；没有孤立页面。

## 权威与外链

- GitHub 仓库 About 的 Website 字段填官网；README 顶部与“文档”节链接官网；Release 说明模板附官网更新日志链接。
- `deploy/linux/nfpm.yaml` 的 `homepage` 改为官网；deb / rpm 元数据会把它带到包管理器。
- 发布到开发者社区（Hacker News、V2EX、少数派、掘金）时链接官网而不是仓库；在 awesome 列表（web terminals、Claude Code 工具）提交条目。
- 不购买外链，不做互惠链接页。

## 搜索平台提交

- Google Search Console：验证域名，提交站点地图，关注“体验”与“富媒体结果”报告。
- Bing Webmaster Tools：提交站点地图（同时覆盖 DuckDuckGo 等使用 Bing 索引的引擎）；开启 Cloudflare 的 IndexNow 自动推送。
- 百度站长平台：可选。海外托管抓取慢，提交站点地图并保持 HTTPS；不为百度单独做页面。
- 上线后两周检查索引覆盖，处理“已发现但未编入索引”的页面（通常是内部链接不足）。

## 面向 AI 搜索与 agent

- 提供 `/llms.txt`：一页说明产品定位、安装命令、文档入口与安全边界，供大模型检索与 agent 阅读；文档镜像后再提供 `/llms-full.txt`。
- `robots.txt` 明确对主流 AI 爬虫的态度（允许或拒绝），不默认留空。
- 安装命令与关键事实在 HTML 中以纯文本出现，不藏在脚本渲染或图片里。

## 度量与维护

- 每月：Search Console 查询词、点击率与平均排名；按页面比较“展示多、点击少”的标题并改写。
- 每次 Release：确认更新日志页被收录、首页版本号正确、结构化数据的 `softwareVersion` 更新。
- 每季度：重跑 Lighthouse 与富媒体结果校验；检查失效外链；复核关键词地图。
