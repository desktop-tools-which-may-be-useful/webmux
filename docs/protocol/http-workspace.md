# HTTP API：仓库与代码工作区

## 仓库与 worktree

| 方法       | 路径                                                            | 说明                                                                                       |
| ---------- | --------------------------------------------------------------- | ------------------------------------------------------------------------------------------ |
| GET / POST | `/api/repositories`                                             | 列出仓库快照；POST `{path}` 登记仓库（新增 201，已存在 200）                               |
| DELETE     | `/api/repositories/{id}`                                        | 仅移除登记，204                                                                            |
| GET        | `/api/repositories/{id}/worktrees`                              | 刷新仓库快照（含 `checkedAt` 与 `worktrees`）                                              |
| GET        | `/api/repositories/{id}/branches`                               | `{branches, bases, defaultBase, defaultBaseSubject, defaultParent}`，不 fetch              |
| POST       | `/api/repositories/{id}/worktrees`                              | `{path, mode: "new", branch, base}` 或 `{path, mode: "existing", branch}`，201             |
| POST       | `/api/repositories/{id}/worktrees/{worktreeId}/removal-preview` | 删除预览：`{confirmationId, expiresAt, worktree, sessions, blockers, forceAllowed}`        |
| POST       | `/api/repositories/{id}/worktrees/{worktreeId}/remove`          | `{confirmationId, mode: "safe"}`；强制删除另需 `confirmPath` 与 `closeSessions: true`，204 |

`bases` 每项为 `{name, head, subject}`，`subject` 是引用指向的提交标题。`defaultBase` 保持为主工作区的精确提交 OID，`defaultBaseSubject` 是对应标题（无默认提交时为空字符串）。浏览器用标题展示候选项，创建请求的 `base` 仍传引用或 OID。

错误码：400 输入错误，404 不存在，409 冲突或确认过期，503 Git / 清单不可用，504 超时，500 存储或非预期错误。JSON 结构见 `internal/contract/workspace.go`。

## 代码工作区

位于 `/api/sessions/{id}/code`，响应带 `Cache-Control: no-store`。

| 路由                                                       | 说明                                                                                              |
| ---------------------------------------------------------- | ------------------------------------------------------------------------------------------------- |
| `GET /context`                                             | 根目录与能力：`contextId`、`rootPath`、`browseAvailable`、`diffAvailable`、`writableAvailable` 等 |
| `GET /tree?contextId&nodeId&includeIgnored&cursor`         | 分页目录列表                                                                                      |
| `GET /files?contextId&q&includeIgnored`                    | 有界路径搜索                                                                                      |
| `GET /locate?contextId&path`                               | 解析用户输入的路径，返回根内 `nodeId`、`path`、`kind`（file / directory）                         |
| `GET /file?contextId&nodeId&source`                        | 文件快照，`source` 为 worktree / index / head                                                     |
| `GET /changes?contextId&cursor&q&nodeId&group`             | 分页 Git 修改列表                                                                                 |
| `GET /diff?contextId&changeId`                             | 冻结的新旧双侧原文                                                                                |
| `GET /snapshots/{snapshotId}/file?side=old\|new`           | 读取已捕获的比较侧全文                                                                            |
| `GET /snapshots/{snapshotId}/raw?side=old\|new&download=1` | 按快照返回原始字节（≤100 MiB）；Diff 快照必须指定 `side`；也接受 HEAD                             |
| `POST /check`                                              | 批量变化检测，返回 unchanged / updated / missing / expired / unknown                              |

`/locate` 的相对路径以本机 OSC 7 报告目录为基点，没有报告时用启动目录；按实际文件系统解析符号链接后，必须位于代码根内（403），远端报告返回 409。结果只是节点标识，读取仍走根目录的逐段 `O_NOFOLLOW` 检查。

文档（`document` / `old` / `new`）带 `mediaType`（未识别为空串）、`size`（字节数）与 `downloadable`；`kind` 为 text / binary / unsupported / too-large / symlink / image / pdf。

扩展名属于 png、jpg、jpeg、gif、webp、avif、bmp、ico 且文件头签名匹配时识别为图片，`mediaType` 取自签名；不超过 20 MiB 为 `image`，更大为 `too-large` 但仍可下载。`.svg` 为合法 UTF-8 且首个 XML 元素是 `svg` 时保持 `text` 并带 `image/svg+xml`。

扩展名为 `.pdf` 且前 1024 字节内出现 `%PDF-` 时为 `pdf`，`mediaType` 为 `application/pdf`；超过 20 MiB 为 `too-large` 并保留媒体类型。空文件或签名缺失时为 `binary` 并说明原因；其他扩展名的 PDF 内容仍为 `binary`。只读取文件头，不读入正文；Diff 中仍为 `metadata`。

`/tree` 与 `/files` 返回的节点带仅按扩展名推导的 `mediaType`。Diff 的 `kind` 增加 `image`（两侧均为图片，或一侧不存在；两侧都是存在的文本时仍为 `text`）。

`/raw` 不受 JSON 接口的 10 秒时限约束，返回与快照完全一致的字节：

- 工作树来源在发送前后校验文件身份、大小与修改时间；索引 / HEAD 按对象 ID 读取。
- 响应带正确的 `Content-Type`（未识别媒体类型时为 `application/octet-stream`）、`Content-Length`、`nosniff` 与 `no-store`。
- `Content-Disposition`：图片与 PDF 未带 `download=1` 时为 `inline`；其余为 `attachment` 并给出原文件名。
- SVG 与 PDF 另加 `Content-Security-Policy: default-src 'none'; style-src 'unsafe-inline'; sandbox`。
- 同时最多 4 个传输；发送中文件变化会中断连接。

错误码：400 非法路径、比较侧或跨会话快照，404 不存在，409 版本冲突或内容已变化，410 快照失效，413 超出限制，429 传输繁忙，504 取消或超时。

### 文件操作

资源管理器的文件操作作用于 `contextId` 所在的根（工作区或临时文件夹），不能跨根；文件内容、Git 索引与配置不受影响。

| 路由                                                  | 说明                                                                                      |
| ----------------------------------------------------- | ----------------------------------------------------------------------------------------- |
| `GET /node?contextId&nodeId`                          | 按树节点下载常规文件（≤100 MiB），`attachment` + 原文件名；也接受 HEAD                    |
| `GET /archive?contextId&nodeId&includeIgnored`        | 目录流式 ZIP，`application/zip`，无预知长度；未带 `includeIgnored=true` 时排除 Git 忽略项 |
| `GET /usage?contextId&nodeId`                         | `{entries, bytes, limited}`：节点及其下条目数与常规文件字节，用于删除确认                 |
| `POST /rename {contextId, nodeId, name}`              | 同目录改名，返回 `{nodes}`                                                                |
| `POST /mkdir {contextId, parentNodeId, name}`         | 新建文件夹，201 返回 `{nodes}`                                                            |
| `POST /copy {contextId, nodeIds, targetNodeId, mode}` | `mode` 为 `copy` / `move`，最多 100 项，返回按请求顺序的 `{nodes}`                        |
| `POST /upload?contextId&nodeId`                       | 单文件 multipart（字段 `file`，≤100 MiB）上传到目录，201 返回 `{nodes}`                   |
| `DELETE /node?contextId&nodeId&recursive=1`           | 删除；非空目录需 `recursive=1`，成功返回 204                                              |

- 名称只能是单个条目：拒绝空名、`.`、`..`、任意大小写的 `.git`、`/`、`\`、控制字符与超过 255 字节（400）。上传只取文件名的最后一段。
- 重名时自动取最小可用的 ` (n)` 后缀（放在扩展名前，`.env` 之类的前导点不算扩展名），从不覆盖已有条目；`{nodes}` 给出最终名称。
- 根目录不可重命名、移动或删除（403）；嵌套仓库边界与特殊文件不可修改，符号链接只能删除且不影响其目标（400）。
- 复制递归重建文件、目录与符号链接，跳过 `.git`、嵌套仓库与特殊文件；复制与 ZIP 先扫描，合计超过 100 MiB 或 10000 项返回 413。
- 不能粘贴到自身或其子目录（400）；移动是同根 `rename`，跨文件系统返回 409；移动到原目录时保持原位。
- 非空目录未带 `recursive=1`，或目录中含 `.git` / 嵌套仓库时返回 409，不会删除一部分后停止。
- 根不可写时 `writableAvailable` 为 false，写操作返回 403。上传与下载、ZIP 共用 4 个传输槽（429）；其余操作 60 秒超时。
