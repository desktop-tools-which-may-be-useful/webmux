## Why

当前“独立代码窗口”把主页面里活着的代码工作区 DOM 搬进画中画或普通弹窗，Svelte 状态、事件委托、CodeMirror、bits-ui/vaul 浮层却仍运行在主页面的 JS 环境里。凡默认使用全局 `document`/`window` 的库都会出错：

- 已修补：样式丢失、事件委托失效、Diff 行号选择失效（#215）；
- 本次发现：右键菜单打开在主窗口且坐标错位；
- 仍失效：Drawer 浮层、`matchMedia` 视口判断、拖放上传。

逐个打补丁无法收敛，需要让独立窗口运行自己的应用实例。

## What Changes

- 独立窗口（画中画与普通弹窗两种窗口都保留）改为只承载一个同源 `<iframe>`，iframe 加载新的 `/code/<会话 ID>` 路由，运行独立的 Svelte 应用实例；不再搬移 DOM。
- 删除 DOM 搬移方案的全部配套：样式复制与 `<html>` 属性镜像、`ShadowRoot.adoptedStyleSheets` 补丁、`PopoutEventRoot`、CodeMirror `setRoot` 重绑、跨文档指针转发与 `ownerDocument` 判断。不保留旧实现作回退，也不合入临时 `PortalHost` 修复。
- 代码工作区的阅读状态可序列化为快照：标签及其已加载内容、当前标签、导航与文件树状态、修改列表筛选、文件操作剪贴板、变化检测基线，以及每个标签的滚动位置与选择（代码选区、行范围、Diff 选中行与展开段、查找条件与当前匹配、预览模式、图片/PDF 缩放与位置、JSON 展开）。弹出与收回时完整保留这些状态。
- 主页面与独立窗口之间建立经 Zod 校验的消息通道：插入引用到终端/草稿（改为异步应答）、草稿开关、连接状态、会话目录、外部“显示此文件”请求，以及快照回传。
- 修复独立窗口中右键菜单、对话框、底部面板出现在主窗口或位置错误的问题（随新架构消除）。
- 新增：代码工作区文件树支持从系统拖入文件上传——拖到文件夹行上传到该文件夹，拖到文件行上传到其所在文件夹，拖到工作区其他位置上传到当前根目录；与“上传到此目录”使用同一接口与冲突规则。主页面代码区与独立窗口同样生效。

## Capabilities

### New Capabilities

- `code-popout-window`: 代码工作区独立窗口——iframe 承载的独立应用实例、打开/收回/关闭生命周期、阅读状态快照的完整保留、与主页面的消息通道、窗口内浮层与交互正确性。
- `code-tree-drop-upload`: 代码工作区拖放上传——拖放目标目录的确定、可写性与并发限制、文件夹拖入的拒绝、进度与结果提示。

### Modified Capabilities

（无。`terminal-diff-review` 中独立窗口入口的显示规则保持不变。）

## Impact

- 前端：
  - `web/src/components/code-popout.ts` 重写为窗口与 iframe 壳，删除 `PopoutEventRoot.svelte`；
  - `CodePanel.svelte` 及阅读组件（`CodeDocumentView`、`CodeReader`、`CodeDiffReader`、`CodeMarkdownPreview`、`CodeImageView`、`CodePdfView`、`CodeJsonPreview`）增加状态采集与恢复；
  - 导航组件（`CodeFileTree`、`CodeNavigator`、`CodeChangesList`、`CodeFileActions`）增加状态采集与恢复；
  - `App.svelte` 新增 `/code/<id>` 路由，`SessionTerminal.svelte` 改为通过宿主接口与代码区交互；
  - `CodeDocumentView`/`CodeLineAction` 的引用插入改为异步，`schemas.ts` 新增快照与消息 Schema。
- 后端：无接口变更；`internal/api/static.go` 已对无扩展名路径回退到 `index.html`，`/code/<id>` 无需改动。
- 测试：`web/test/browser/CodePanel.test.ts` 中模拟窗口的 DOM 搬移用例删除并改写为协议与快照用例；`web/test/e2e/code-popout.spec.ts` 扩充状态保留、浮层位置、引用插入、拖放上传用例。
- 文档：用户故事 US-I13（独立代码窗口）、US-I19（资源管理器文件操作）。
