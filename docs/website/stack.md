# 技术方案：静态站点生成器与部署

官网是纯静态站点：没有登录、没有服务端状态，所有动态数据（版本、资产、校验值）在构建期取得，运行期最多只有少量渐进增强的脚本。本页给出生成器选型、仓库位置、项目骨架、数据来源、托管与质量门禁。

## 选型结论：Astro

推荐使用 Astro（取当前稳定大版本，锁定在 `package.json` 中）。理由对应官网的硬性要求：

- **默认零 JavaScript**：页面是纯 HTML，只有标记为岛屿的组件才下发脚本，天然满足性能预算与 SEO。
- **内容优先**：Markdown / MDX 内容集合带 Zod 校验，FAQ、对比表、agent 矩阵都可以写成结构化数据。
- **内置能力**：`astro:assets` 图片优化（AVIF / WebP、尺寸属性防布局位移）、`@astrojs/sitemap`、i18n 路由、View Transitions。
- **可复用团队技能**：TypeScript、pnpm、Zod 与主仓库一致；需要交互时用 `@astrojs/svelte` 写 Svelte 5 岛屿，与产品前端同栈。
- **文档与营销页同一框架**：Starlight 提供文档侧栏、全文搜索（Pagefind）、多语言与深浅色；营销页与文档镜像在同一个 Astro 项目里，共用令牌与布局。

| 备选       | 适合场景             | 不选的原因                                         |
| ---------- | -------------------- | -------------------------------------------------- |
| VitePress  | 以文档为主的站点     | 营销首页需大量自定义布局，Vue 栈与团队不一致       |
| Docusaurus | 大型文档站           | React 运行时偏重，首页定制成本高，默认下发较多脚本 |
| Hugo       | 纯内容、极快构建     | 模板语言与交互组件生态与团队栈脱节，交互功能需手写 |
| Next.js    | 需要服务端能力的站点 | 静态导出能力有限，运行时过重，官网不需要服务端     |

## 仓库位置

建议把官网放在独立仓库（例如 `mzdluo123/webmux-site`），不放进主仓库：

- 主仓库 CI 的 `Classify changes` 会把未知路径视为全量变更，官网的每次文案修改都会触发 Go race 与 E2E。
- `docs/` 受 [文档治理](../doc-governance.md) 约束，官网源码与内容集合不适合这些规则；`make format-check` 的覆盖范围也要跟着扩。
- 官网与产品的发布节奏不同：文案可以随时改，产品按 tag 发布。

两个仓库的联动方式：官网构建期从 GitHub Releases API 读取数据，并配置每日定时重建、手动触发，以及由主仓库 Release 工作流通过 `repository_dispatch` 触发（需在主仓库保存一个只有 `repo` 范围的细粒度 PAT）。前两者已经足够，`repository_dispatch` 只是让新版本更快出现在官网。

如果坚持放在主仓库（例如 `website/`），需要同时修改 CI 分类脚本、oxfmt 范围与 knip 配置，并把 `website/` 从 docguard 的范围外明确排除；这不是本指南的推荐路径。

## 项目骨架

```
webmux-site/
├── .nvmrc                 # 与主仓库相同的 Node 版本
├── package.json           # pnpm，packageManager 固定版本
├── astro.config.mjs       # site、i18n、sitemap、集成
├── public/                # favicon、OG 图、robots.txt、_redirects、_headers、llms.txt
├── scripts/
│   ├── fetch-releases.mjs # 构建期读取 Releases API，写入 src/data/releases.json
│   ├── sync-assets.mjs    # 从主仓库指定 tag 复制 Logo、截图、字体与许可文件
│   └── sync-docs.mjs      # 从主仓库同一 tag 复制并改写 docs/，生成 Starlight 内容
├── src/
│   ├── content/           # zh-cn/ 与 en/ 的页面文案、faq、compare、agents（内容集合 + Zod）
│   ├── data/releases.json # 构建期数据与提交的回退副本
│   ├── components/        # Hero、InstallCommand、DownloadTable、FeatureGrid、Gallery…
│   ├── layouts/           # Base（head、导航、页脚、JSON-LD）、Doc
│   ├── pages/             # index、download、faq、security、compare、changelog、agents/、windows、404
│   ├── content/docs/      # 由 sync-docs.mjs 生成的文档镜像，不手工编辑
│   ├── styles/tokens.css  # 从 web/src/design-tokens.css 派生的令牌
│   └── i18n/              # 语言字典与 hreflang 辅助
└── tests/                 # 链接检查、Lighthouse 预算、axe
```

依赖规则：锁定版本、使用 pnpm 冻结锁文件；不引用任何运行期 CDN（字体、图标、脚本都随站点打包，与产品“图标随应用打包”的原则一致）；图标用 Iconify 的 `tabler` 集合在构建期内联。

## 内容与资产同步

- **Logo 与图标**：`docs/images/brand/webmux-logo-{light,dark}.svg` 作横向标志，`web/public/icon.svg` 作 favicon 与 OG 图里的单独图标。
- **字体**：复制 `web/public/fonts/` 中 Inter 与 Maple Mono 的拉丁子集及 `licenses/` 下的 OFL 文件，站点页脚或 `/licenses/` 保留版权声明。
- **截图**：从主仓库某个 tag 的 `docs/images/` 复制首页用到的图片，文件名保持不变，并维护一份“文件名 → 替代文本”的映射（替代文本可直接取自 README 的 `alt`）。
- **同步脚本**：`sync-assets.mjs` 用 sparse checkout 或 GitHub 内容 API 拉取指定 tag，复制后生成清单（来源 tag、路径、SHA-256），提交到官网仓库以便审查差异。

文档镜像的具体做法见下一节；截图目录 `docs/images/` 与文档一起同步，文档里的相对图片路径保持可用。

## 文档镜像

主仓库 `docs/` 已有约六十篇中文文档，覆盖安装、部署、使用、混合会话、协议与架构。把它们镜像到官网 `/docs/` 是第一阶段的内容主体。

- 原因：搜索引擎按页面与主题评估站点，几十篇互相链接、各自回答一个具体问题的文档页，比一个首页带来的长尾覆盖多得多。
- 前提：每页都是正式、可索引、`canonical` 指向官网的独立 URL，否则只是重复内容。

**范围**：

- 同步 `docs/**/*.md`，以及根目录的 `README.md`（作 `/docs/` 首页的来源之一）、`SECURITY.md`、`CONTRIBUTING.md`。
- 默认排除面向 agent 的 `docs/agent-rules/`、`docs/agent-rules.md`、`docs/doc-governance.md`，以及 `docs/user-stories/verification-log-*.md`。它们对访客价值低，收录会稀释站点主题。
- `openspec/` 不镜像。
- 排除列表写在脚本里，而不是靠手工删。

**来源与版本**：`sync-docs.mjs` 从主仓库某个 Release tag 读取（与 `sync-assets.mjs` 同一 tag），不是 `main`，保证文档描述的行为与下载页的版本一致。每次同步生成清单（tag、文件、SHA-256）并提交，PR 中能看到文档差异。

**改写规则**（用 remark 插件在构建期完成，不改主仓库原文）：

- 路径：`docs/usage/terminal.md` → `/docs/usage/terminal/`；`docs/usage.md` 与 `docs/usage/` 并存时，前者成为该目录的索引页 `/docs/usage/`。根目录文件映射到 `/docs/security/`、`/docs/contributing/`。
- 相对链接：按上面的路径规则改写；指向 `../README.md` 的链接改为官网首页；指向 `openspec/` 或被排除文件的链接改为对应的 GitHub 地址。
- 锚点：中文标题的 slug 必须按 GitHub 规则生成（去掉行内标记与标点、小写、空格变连字符、同名追加 `-1`），与 docguard 的实现一致；Starlight 默认的 slug 算法不同，需要在配置中替换，否则 `#需要你介入的提醒` 这类站内锚点全部失效。
- GitHub 特性：`> [!WARNING]` / `[!NOTE]` 转为 Starlight 的 aside；`<details>`、`<picture>`、HTML 表格原样保留（Astro Markdown 允许 HTML）；`<img>` 的相对路径改写到同步后的图片目录。
- 首行 H1 作为页面标题进入 frontmatter；`description` 取第一段正文的前 150 个字符。
- 每页顶部或底部加“在 GitHub 上查看 · 编辑”链接与“基于 vX.Y.Z”标注；`canonical` 指向官网自身。

**侧栏与导航**：

- 侧栏分组沿用 [文档中心](../README.md) 的分组（了解、安装与部署、日常使用、开发与架构、性能与质量、官网与推广），顺序与标题一致；“文档与 Agent 规则”分组按排除列表跳过。
- `/docs/` 首页就是文档中心的镜像加一个搜索框。
- 营销页的每个功能描述都链接到对应文档页，文档页页脚链接回下载页，形成双向内链。

**质量门禁**：同步后运行链接与锚点检查，要求 `make docs-check` 认可的相对链接在官网上全部可解析；任何未映射的链接或锚点使构建失败，而不是静默变成 404。

**与主仓库的约定**：主仓库文档继续以 GitHub 为编辑入口，官网不反向修改；文档治理规则（20–400 行、300 列）保证每页体量适中，正好适合独立 URL。若某篇文档不适合公开站点，在主仓库文档首部加 `<!-- site: exclude -->` 注释，同步脚本识别后跳过。

## 构建期数据

`fetch-releases.mjs` 在构建前运行，请求 `GET https://api.github.com/repos/mzdluo123/webmux/releases/latest`（更新日志页再取 `releases?per_page=30`），提取：

- `tag_name`、`name`、`published_at`、`body`（发布说明 Markdown）。
- 每个资产的 `name`、`size`、`digest`（形如 `sha256:…`）、`browser_download_url`。
- 可选：仓库的 `stargazers_count`，用于页头的 GitHub 徽标。

规则：

- 未认证请求每小时 60 次，CI 中传入 `GITHUB_TOKEN` 提高配额。
- 请求失败时使用提交在仓库中的 `releases.json` 回退副本并在日志中告警；构建不能因为 API 失败而中断。
- 资产按名称模式分组：`webmux-linux-*`、`webmux-darwin-*`、`webmux-windows-*.exe`、`webmux-tray-*`、`webmux-setup-*`、`*.deb`、`*.rpm`、`install.sh`、`SHA256SUMS`。只生成有匹配资产的分组；`v0.10.1` 不含安装程序、deb 或 rpm，不得为其显示空入口。
- 遇到未知名称时放进“其他”而不是丢弃。

## 托管与部署

| 方案             | 优点                                                          | 注意                                       |
| ---------------- | ------------------------------------------------------------- | ------------------------------------------ |
| Cloudflare Pages | 全球 CDN、预览部署、`_redirects` / `_headers`、无 Cookie 统计 | 推荐作为主站                               |
| GitHub Pages     | 与仓库同生态、零配置 HTTPS                                    | 无自定义响应头与重定向规则，大陆访问不稳定 |

推荐 Cloudflare Pages。中国大陆可达性对两者都不稳定，缓解办法是让站点足够轻（首屏传输小于 200 KB、字体自托管、不引第三方脚本），必要时再在国内对象存储做镜像（域名需备案，属于可选项）。

部署工作流（GitHub Actions）：`pnpm install --frozen-lockfile` → `node scripts/fetch-releases.mjs` → `astro build` → 质量门禁 → Wrangler 部署。触发条件：`push` 到主分支、`pull_request`（只构建与预览）、`schedule` 每日一次、`workflow_dispatch`、`repository_dispatch` 的 `release` 事件。

`_headers` 至少设置：`Content-Security-Policy`（`default-src 'self'`，统计脚本按需放行）、`X-Content-Type-Options: nosniff`、`Referrer-Policy: strict-origin-when-cross-origin`、带哈希文件名的静态资源 `Cache-Control: public, max-age=31536000, immutable`。

`_redirects` 建议：

```
/install.sh   https://github.com/mzdluo123/webmux/releases/latest/download/install.sh  302
/releases     https://github.com/mzdluo123/webmux/releases  302
/github       https://github.com/mzdluo123/webmux  302
/docs/images/* https://raw.githubusercontent.com/mzdluo123/webmux/main/docs/images/:splat 302   # 仅未同步的图片目录
```

`/install.sh` 让 `curl -fsSL https://<域名>/install.sh | sh` 可用（`-L` 会跟随重定向）。它必须始终指向 GitHub Release 资产，官网自身不托管脚本副本，并把“域名失守即安装入口失守”的风险写进 [安全策略](../../SECURITY.md) 的维护者注意事项；README 的安装命令继续使用 GitHub 直链。

## 域名

选择一个短域名并全站 HTTPS；`.dev` 顶级域自带 HSTS 预载，是合适的选择。`www` 重定向到裸域（或反之，二选一并保持一致）。域名确定后同步更新：GitHub 仓库的 Website 字段、README 的首行链接、`deploy/linux/nfpm.yaml` 的 `homepage`、Release 说明模板。

## 本地开发与质量门禁

- `pnpm dev` 本地预览；`pnpm build && pnpm preview` 检查生产输出。
- 每个 PR 运行：`astro check`、格式化检查（oxfmt 或 Prettier，二选一）、文档同步清单校验、链接检查（lychee，含锚点）、Lighthouse CI（预算见 [交互与可访问性](ux.md#性能预算)）、axe / pa11y 可访问性扫描、HTML 校验。
- 主分支合并后自动部署；预览部署链接贴在 PR 里，UI 变更附桌面与手机、浅色与深色截图（与主仓库 PR 规则一致）。
