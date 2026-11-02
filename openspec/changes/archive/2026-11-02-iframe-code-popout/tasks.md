## 1. 契约与宿主接口

- [x] 1.1 在 `web/src/schemas.ts` 定义带版本号的 `CodeWorkspaceSnapshotSchema`（面板、导航、文件树、修改列表、剪贴板、变化检测基线、标签及各阅读组件状态）与 `CodePopoutMessageSchema` 判别联合，类型用 `z.infer` 导出
- [x] 1.2 定义 `CodeHost` 接口；`CodePanel` 改为接收宿主，`SessionTerminal` 提供直接实现
- [x] 1.3 引用插入改为异步：`paste` 返回 `Promise<TextInsertion>`，更新 `CodeDocumentView`、`CodeLineAction` 等调用方的提示时机

## 2. 阅读状态采集与恢复

- [x] 2.1 `CodeReader`：采集顶部行与行内偏移、选区、查找条件与当前匹配、跳转面板与选中行，并在文档就绪后恢复
- [x] 2.2 `CodeDiffReader`：记录已展开段；采集选中行、比较侧、查找与匹配游标、顶部可见行并恢复（先核实 `@pierre/diffs` 可用 API）
- [x] 2.3 `CodeDocumentView`：采集与恢复文件/Diff 模式、比较侧、预览/源码与 SVG 视图、行范围，并转发子组件状态
- [x] 2.4 `CodeMarkdownPreview`、`CodeImageView`、`CodePdfView`、`CodeJsonPreview`：按设计表采集，内容就绪后以内容锚点恢复
- [x] 2.5 `CodeFileTree`、`CodeChangesList`、`CodeFileActions`、`CodeNavigator`：采集与恢复展开文件夹及已加载子目录、修改列表位置、待粘贴条目、导航页签与搜索
- [x] 2.6 `CodePanel`：汇总 `capture()` 为快照，支持以快照作为初始状态（标签内容内联、不重新请求、保留更新标记与变化检测基线）
- [x] 2.7 浏览器测试：同一环境内“采集 → 新实例恢复”往返，覆盖滚动行、选区、Diff 展开与选中、预览滚动、图片/PDF 位置、更新标记

## 3. 嵌入路由与通道

- [x] 3.1 `App.svelte` 新增按需加载的 `/code/<id>` 分支：复用登录检查、主题与设置同步，只渲染代码工作区；无通道时显示“需从会话页面打开”及 `/s/<id>` 链接
- [x] 3.2 实现窗口侧通道：接受首条同源握手并取得端口，校验每条消息，实现 `CodeHost`（异步 `paste` 带超时），防抖推送快照并在 `pagehide` 发送最终快照，响应 `collect`
- [x] 3.3 实现主页面侧通道：转发宿主字段变化与 `reveal`，应答 `paste`，保存最新快照，握手超时与 Schema 校验失败按加载失败处理

## 4. 用 iframe 替换 DOM 搬移

- [x] 4.1 重写 `codePopout.ts`：打开画中画或普通弹窗，写入最小壳文档与全屏 iframe，建立通道，普通弹窗随主页面 `pagehide` 关闭，窗口关闭时回调最新快照
- [x] 4.2 `CodePanel` 弹出流程：采集快照 → 打开窗口 → 收到 `ready` 后卸载阅读视图与轮询并显示占位；“收回”先 `collect` 再关闭；关闭或失败时以最新快照原位恢复并提示失败原因
- [x] 4.3 删除 `PopoutEventRoot.svelte`、样式复制与属性镜像、`adoptedStyleSheets` 补丁、CodeMirror `setRoot` 重绑、`CodeDiffReader` 跨文档 `relayPointer` 及 `ownerDocument` 判断；`matchMedia` 判断改用组件自身窗口
- [x] 4.4 删除 `CodePanel.test.ts` 中只验证 DOM 搬移细节的模拟窗口用例，改写为通道协议用例（非同源或非法消息被丢弃、握手超时恢复、`paste` 应答与超时）

## 5. 拖放上传

- [x] 5.1 从 `CodeFileActions` 抽出 `upload(files, into)` 供菜单与拖放共用；暴露上传进行中状态
- [x] 5.2 文件树行与根目录行加 `data-code-drop` 目标标记；在代码工作区 `<section>` 以捕获阶段处理含 `Files` 的拖动，解析目标目录并高亮、提示“松开以上传到 <目录>”
- [x] 5.3 拒绝规则：根目录不可写、包含文件夹（`webkitGetAsEntry`）、已有上传进行中时显示不可放置并在松开后提示，不上传任何文件
- [x] 5.4 浏览器测试：拖到文件夹行、文件行、空白处的目标解析，三类拒绝，重名按 “ (n)” 保存

## 6. 端到端验证与文档

- [x] 6.1 扩充 `web/test/e2e/code-popout.spec.ts`（画中画与普通弹窗）：右键菜单在独立窗口内且位于指针处、主页面无菜单；重命名对话框在窗口内；窄窗口底部面板在窗口内；行号单击与 Shift 范围选择
- [x] 6.2 E2E：弹出与收回保留标签、滚动行、选区、Diff 展开与选中；用窗口关闭按钮关闭后保留选中行
- [x] 6.2a 浏览器测试：Markdown、图片、PDF、JSON 位置往返（组件级）；直接关闭窗口按最后上报状态恢复；“有更新”标记与已发送正文引用跨弹出保留
- [x] 6.3 E2E：独立窗口插入引用到主页面终端；刷新主页面关闭普通弹窗；直接访问 `/code/<id>` 的提示
- [x] 6.3a 浏览器测试：插入结果经通道回传；主页面“显示此文件”请求转发到窗口且只转发一次；未就绪窗口超时后原位保留
- [x] 6.4 E2E：主页面与独立窗口中拖放上传到文件夹，及只读根目录拒绝
- [x] 6.5 更新用户故事 US-I13、US-I19 及索引计数，并同步 `docs/usage.md` 独立窗口说明
- [x] 6.6 运行 `make check`、`make test-fast`、`pnpm --dir web check:arch` 与上述 E2E，全部通过
