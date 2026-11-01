## Context

独立代码窗口由 `web/src/components/codePopout.ts` 实现：`popOut()` 打开 Document Picture-in-Picture 窗口或 `window.open("")` 弹窗，把 `CodePanel.svelte` 的 `<section>` 节点移入新文档，原位置留注释标记，`pagehide` 时再移回。

组件状态因“同一批对象”而天然保留，但所有代码仍运行在主页面的 JS 环境，导致一系列只在独立窗口出现的缺陷与补丁：

| 现象                                                                          | 现有处理                                                                                                         |
| ----------------------------------------------------------------------------- | ---------------------------------------------------------------------------------------------------------------- |
| 样式丢失、Shadow DOM 的 `adoptedStyleSheets` 被丢弃                           | 复制 `<link>/<style>`、镜像 `<html>` 属性、给 `ShadowRoot.prototype.adoptedStyleSheets` 打补丁并跨环境克隆样式表 |
| Svelte 委托事件不触发                                                         | 在弹窗 body 挂载空组件 `PopoutEventRoot`                                                                         |
| CodeMirror 绑定主窗口                                                         | `moved()` 中逐个 `EditorView.setRoot(document)`                                                                  |
| Diff 行号范围选择失效（#215）                                                 | `CodeDiffReader` 中的 `relayPointer` 跨文档转发                                                                  |
| 右键菜单/对话框出现在主窗口、坐标错位（本次报告）                             | 未修复；`BitsConfig` 方案在开发模式下因 bits-ui 的 `instanceof Element` 跨环境检查失败，且对 vaul 底部面板无效   |
| `matchMedia("(max-width:1023px)")` 以主窗口视口判断（`CodePanel.svelte:379`） | 潜在缺陷                                                                                                         |
| 独立窗口不能拖放上传                                                          | 无（主页面代码区也不支持拖放）                                                                                   |

`CodePanel` 由 `SessionTerminal.svelte` 渲染，外部输入为 `id`、`visible`、`connection`、`paste(text) => TextInsertion`（同步）、`draftEnabled`、`sessionCwd`、`reportedCwd`、`reveal`。其余依赖都可在同源新实例中复用：

- API 调用与登录令牌（`localStorage["webmux.token"]`）；
- 服务器设置（`/ws/settings`）与主题；
- 静态路由：`internal/api/static.go` 对无扩展名路径回退到 `index.html`，`/code/<id>` 无需后端改动。

代码区阅读状态目前不做任何序列化。标签切换靠“全部挂载、隐藏非当前标签”保留状态；显式恢复只有两处：`CodeReader.line()`→`restoreLine`，以及 `CodeDiffReader` 的 `getSelectedLines()`→`setSelectedLines()`。

## Goals / Non-Goals

**Goals:**

- 独立窗口中的代码工作区运行在自己的文档与 JS 环境中，消除跨文档类缺陷及其补丁。
- 弹出、收回、直接关闭窗口三条路径都完整保留阅读状态，包括滚动位置与选择（范围见 `code-popout-window` 规格）。
- 引用插入、草稿开关、连接状态、“显示此文件”在窗口间正常工作。
- 代码工作区支持拖放上传到文件树目标目录，主页面与独立窗口一致。
- 修复独立窗口中浮层位置错误的问题，并以真实浏览器 E2E 回归。

**Non-Goals:**

- 不保留 DOM 搬移实现，也不合入临时 `PortalHost` 修复。
- 不提供脱离主页面、可单独使用的 `/code/<id>` 页面。
- 不保留富文本预览（Markdown/HTML/表格）中浏览器原生的文字选中。
- 不改变独立窗口入口的显示规则、窗口类型选择与服务端接口。
- 不支持拖入文件夹（递归上传）。

## Decisions

### D1 两种窗口都只承载 iframe

画中画窗口不能导航（地址固定为 `about:blank`），只能通过子 iframe 加载路由；规范规定主页面获得用户激活时，同源子 iframe 一并激活。普通弹窗也采用同样的“空壳 + iframe”，使两种窗口共用一条代码路径与一组测试。壳文档只写入最小样式（全屏 iframe、`color-scheme` 与背景色防闪白）和标题。

备选：普通弹窗直接 `window.open("/code/<id>")`。少一层壳，但与画中画分叉成两条初始化路径，否决。

### D2 `/code/<id>` 嵌入路由

`App.svelte` 新增按需加载的 `/code/<id>` 分支，复用现有登录检查、`initializeTheme()` 与服务器设置同步，只渲染代码工作区，不注册终端页的全局快捷键、上传拖放区与会话列表订阅。该路由只在收到主页面的通道后渲染工作区；直接访问时显示“需从会话页面打开”并链接 `/s/<id>`。

### D3 `CodeHost` 宿主接口

把 `CodePanel` 对外部的依赖收拢为一个接口：

```ts
interface CodeHost {
  readonly draftEnabled: boolean;
  readonly connection: string;
  readonly sessionCwd?: string;
  readonly reportedCwd?: string;
  readonly reveal?: { path: string; key: number };
  paste(text: string): Promise<TextInsertion>;
}
```

主页面由 `SessionTerminal` 直接实现；iframe 内由通道实现。`paste` 改为异步，`CodeDocumentView`、`CodeLineAction` 等调用方等待结果后再显示“输入目标未就绪”提示。这是唯一的行为接口变化，其余 props 退化为宿主上的响应式字段。

### D4 消息通道

- 握手由 iframe 发起，避免应用尚未挂载监听时丢失：
  - iframe 的应用在登录检查与路由加载完成后，向父窗口（壳）发送握手请求；
  - 主页面在壳上监听，只接受来源为本次创建的 iframe、同源且格式合法的请求，随后新建 `MessageChannel`，以 `targetOrigin = location.origin` 回复握手并转移 `port2`；
  - 回复由主页面发出而非壳，因此 iframe 只校验同源与格式，只接受第一条，此后只用端口通信。
- 消息为 `schemas.ts` 中的 Zod 判别联合（`CodePopoutInboundSchema` / `CodePopoutOutboundSchema`，握手带协议版本号）：
  - 主页面 → 窗口：`init{snapshot, host}`、`host{…}`（草稿开关、连接、目录变化）、`reveal{path,key}`、`pasteResult{requestId,result}`、`collect{requestId}`；
  - 窗口 → 主页面：`ready`、`paste{requestId,text}`、`snapshot{requestId?,snapshot}`、`close`。
- 未通过校验的消息丢弃并在开发模式记录；握手 10 秒未完成视为加载失败（关闭窗口、原位恢复、提示）。`paste` 请求 5 秒无应答按失败处理。

备选：同源下直接调用 `window.opener` 上的函数。会把跨环境对象引用重新带回来，否决。

### D5 快照内联已加载内容

快照（`CodeWorkspaceSnapshotSchema`，带版本号）包含标签已加载的 `CodeFile`/`CodeDiff`、已加载的目录列表与修改列表页，经结构化克隆传递，不重新请求。理由：

- 弹出/收回不会把标签静默换成更新版本，符合“自动检测更新但由用户刷新内容”；
- 恢复时不产生 N 次请求。

不可克隆或可再生的数据在新环境重算或重新加载：Worker 计算的 `FileDiffMetadata`、pdf.js 文档、图片对象 URL；标签上的 `menu` 片段由新环境的组件重新生成。

数据量上限为 20 个标签与服务端单次读取上限，同进程结构化克隆可接受。

备选：快照只带标识、恢复时按 `snapshotId` 重新请求。服务端不保证旧快照可再读，且会改变版本语义，否决。

### D6 分组件采集与恢复

每个持有阅读状态的组件导出 `capture()` 并接受一次性的 `restore` 输入，在内容就绪后应用；`CodePanel` 汇总为快照。位置一律以内容为锚点：

| 组件                  | 采集内容                                                                                                                                                      | 恢复方式                                                                    |
| --------------------- | ------------------------------------------------------------------------------------------------------------------------------------------------------------- | --------------------------------------------------------------------------- |
| `CodeReader`          | 顶部行号与行内偏移比例、`EditorSelection.toJSON()`、查找条件与当前匹配、跳转面板开合、选中行                                                                  | `scrollIntoView` 到行并补偏移；`EditorSelection.fromJSON`；`setSearchQuery` |
| `CodeDiffReader`      | 选中行、比较侧、已展开的未修改段、查找条件与匹配游标、顶部可见行                                                                                              | `setSelectedLines`、`expandHunk`、`scrollTo({type:"line"})`                 |
| `CodeDocumentView`    | 文件/Diff 模式、比较侧、预览/源码模式、SVG 视图、行范围                                                                                                       | 直接赋值                                                                    |
| `CodeMarkdownPreview` | 最近的上方标题锚点与相对偏移；无标题时用滚动比例                                                                                                              | 渲染完成后定位                                                              |
| `CodeImageView`       | `scale`、`x`、`y`、`fitted`                                                                                                                                   | 尺寸已知后 `place()`                                                        |
| `CodePdfView`         | `zoom`、`fitted`、当前页与页内偏移比例                                                                                                                        | 页面尺寸已知后滚动                                                          |
| `CodeJsonPreview`     | `opened`、`shown`                                                                                                                                             | 直接赋值                                                                    |
| `CodeFileTree`        | 根目录开合、已展开文件夹、已加载的子目录列表                                                                                                                  | 直接赋值                                                                    |
| `CodeChangesList`     | 首个可见条目序号                                                                                                                                              | 列表就绪后滚动                                                              |
| `CodeFileActions`     | 待粘贴的复制/剪切条目                                                                                                                                         | 直接赋值                                                                    |
| `CodePanel`           | 标签与顺序、当前标签、导航开合/宽度/页签、根目录、忽略项、路径搜索、修改列表筛选与已加载页、变化检测基线（`sequence`、`contentVersion`、`directoryVersions`） | 作为初始状态                                                                |

`CodeDiffReader` 的展开状态目前只由应用调用 `expandHunk` 写入、库内不可读，因此由应用记录已展开段的序号与方向。

### D7 同一时刻只有一个活动实例

弹出成功（收到 `ready`）后，主页面 `CodePanel` 卸载阅读视图与轮询，仅保留占位；独立窗口是唯一权威。窗口内实例在状态变化后防抖（约 1 秒）推送快照，并在 `pagehide` 时同步推送最终快照；主页面始终持有最新快照。“收回”按钮先发 `collect` 取得最新快照再关闭窗口；窗口自行关闭时以最后收到的快照恢复。

备选：只在关闭时索取快照。用户直接关闭窗口时无法等待异步应答，否决。

### D8 删除的补丁

删除以下只为跨文档存在的代码：

- `PopoutEventRoot.svelte`、样式复制与属性镜像、`adoptedStyleSheets` 补丁与克隆；
- `moved()` 中的 CodeMirror 重绑、`CodeDiffReader` 的跨文档 `relayPointer`；
- `CodePanel` 中以 `section.ownerDocument` 代替 `document` 的判断。

`code-highlight.ts`、`download.ts` 中按所属文档取环境的写法保留，单环境下依然正确。`matchMedia` 视口判断在 iframe 中自然以独立窗口为准。

### D9 拖放上传

在代码工作区 `<section>` 上以捕获阶段监听 `dragenter/dragover/dragleave/drop`，与终端 `useUpload` 一样只处理 `dataTransfer.types` 含 `Files` 的拖动。

- 目标目录：由 `event.target.closest("[data-code-drop]")` 解析。文件夹行标记自身节点，文件行标记其父目录，根目录行标记根；未命中则为当前根目录。
- 上传：复用 `CodeFileActions` 的目录上传流程（抽出 `upload(files, into)`），继承冲突命名、进度、取消与成功后的列表刷新。
- 拒绝：`drop` 时用 `DataTransferItem.webkitGetAsEntry()` 识别文件夹并整体拒绝；不可写或已有上传进行时，`dragover` 设 `dropEffect = "none"`，`drop` 时提示且不上传。

## Risks / Trade-offs

- [`pagehide` 期间发出的最终快照可能丢失] → 防抖推送保证主页面至少持有 1 秒内的状态；E2E 覆盖“用窗口关闭按钮关闭后恢复滚动与选择”。
- [弹出比原方案慢：需启动第二个应用实例并恢复视图] → 路由按需只加载代码区相关代码；恢复期间主页面保持原工作区可见，收到 `ready` 后再切换占位，避免空白。
- [恢复依赖异步渲染（Markdown 图片、PDF 页面尺寸、Diff Worker）] → 每个组件在自身“内容就绪”后才应用 `restore`，并以内容锚点而非像素定位。
- [大标签内容的结构化克隆开销] → 受 20 个标签与服务端读取上限约束；在 E2E 中以较大文件验证弹出时延。
- [快照 Schema 与组件状态漂移] → 快照带版本号；Zod 校验失败时按“加载失败”路径恢复原工作区，不部分应用。
- [每个独立窗口多一条设置 WebSocket] → 单用户场景可接受，窗口关闭即断开。

## Migration Plan

纯前端变更，无持久化数据与接口迁移。按任务顺序在同一分支完成后一次合入；回滚即回退该提交。

## Open Questions

- `@pierre/diffs` 是否提供读取已展开段或可见行的公开 API；若无，按 D6 由应用记录，实现时以类型定义核实。
- 防抖推送间隔与握手、`paste` 超时的具体取值，实现时以 E2E 稳定性校准。
