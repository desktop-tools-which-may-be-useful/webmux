# 官网原型

本目录是官网候选方向与选定方向的静态原型，可直接在浏览器中打开（文件相对引用 `docs/images/` 的截图与 `web/public/fonts/` 的字体，不依赖网络）。它们是 [视觉与品牌规范](../design.md) 的可视化依据：规范里的每个尺寸、颜色与版式都能在这里找到对应实例。

原型由维护者的设计画布导出，画布是可编辑的源；修改原型请先改画布再重新导出，不要只改这里的 HTML。原型中的数字（版本、资产大小、SHA-256）是导出时的 v0.10.1 快照，正式站点由构建期数据生成。

## 候选方向（2026-11-03 评审）

| 文件                                 | 方向                                 | 结果 |
| ------------------------------------ | ------------------------------------ | ---- |
| [a-workbench.html](a-workbench.html) | 工作台：浅色，延续产品设计系统的卡片 | 未选 |
| [b-terminal.html](b-terminal.html)   | 深色终端：终端优先，薄荷绿强调       | 未选 |
| [c-story.html](c-story.html)         | 叙事长卷：大留白，交替图文讲故事     | 选定 |
| [d-split.html](d-split.html)         | 双屏对偶：明暗分区，电脑 ↔ 手机      | 未选 |

## 选定方向的配套页面

| 文件                               | 内容                                                  |
| ---------------------------------- | ----------------------------------------------------- |
| [c-mobile.html](c-mobile.html)     | 首页手机版（390px）：堆叠叙事行、整行命令框、折叠导航 |
| [c-dark.html](c-dark.html)         | 首页深色主题：按规范的令牌映射逐项替换                |
| [c-download.html](c-download.html) | 下载与安装页：标签页、更多选项、资产表、校验命令      |
| [c-docs.html](c-docs.html)         | 文档镜像页：覆写 Starlight 布局的侧栏、正文与本页目录 |

## 截图

截图由 Playwright 从上述 HTML 渲染（桌面 1280px、手机 390px，整页 JPEG），位于 `docs/images/website-prototypes/`。

| 原型                 | 截图                                                    |
| -------------------- | ------------------------------------------------------- |
| A · 工作台           | [查看](../../images/website-prototypes/a-workbench.jpg) |
| B · 深色终端         | [查看](../../images/website-prototypes/b-terminal.jpg)  |
| C · 叙事长卷（选定） | [查看](../../images/website-prototypes/c-story.jpg)     |
| D · 双屏对偶         | [查看](../../images/website-prototypes/d-split.jpg)     |
| C · 首页手机版       | [查看](../../images/website-prototypes/c-mobile.jpg)    |
| C · 首页深色主题     | [查看](../../images/website-prototypes/c-dark.jpg)      |
| C · 下载与安装页     | [查看](../../images/website-prototypes/c-download.jpg)  |
| C · 文档镜像页       | [查看](../../images/website-prototypes/c-docs.jpg)      |

## 原型的边界

- 原型是静态版式：标签页、复制按钮、菜单与搜索不响应操作；交互行为以 [交互与可访问性](../ux.md) 为准。
- 文案与截图来自 README 与 `docs/images/`；正式站点的文案以 [页面清单](../pages.md) 的来源规则为准。
- 原型使用内联样式便于逐元素对照；正式实现应把这些值收敛到令牌文件与组件中。
