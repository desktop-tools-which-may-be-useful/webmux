# HTTP API：会话、启动配置与其他

## 会话

| 方法   | 路径                           | 说明                                                                               |
| ------ | ------------------------------ | ---------------------------------------------------------------------------------- |
| GET    | `/api/sessions`                | 会话列表 `SessionInfo[]`，按 `lastActiveAt` 倒序                                   |
| POST   | `/api/sessions`                | 创建会话，201 返回 `SessionInfo`                                                   |
| GET    | `/api/sessions/{id}`           | 单个会话；不存在 404                                                               |
| PATCH  | `/api/sessions/{id}`           | 设置用户标题 `{"name"}`，返回会话；空串恢复自动标题；含控制字符或超过 512 字节 400 |
| DELETE | `/api/sessions/{id}`           | 关闭会话（SIGHUP → SIGKILL），204                                                  |
| DELETE | `/api/sessions/{id}?remove=1`  | 移除已退出会话，204；仍在运行 409                                                  |
| GET    | `/api/sessions/{id}/buffer`    | 下载完整原始缓冲（`application/octet-stream`）                                     |
| GET    | `/api/sessions/{id}/upload`    | 读取本会话已上传的单个文件（`?path=`），见下文                                     |
| POST   | `/api/sessions/{id}/upload`    | 上传文件，见下文                                                                   |
| POST   | `/api/sessions/{id}/artifacts` | 确保会话临时文件夹存在，200 `{path}`，见下文                                       |
| GET    | `/api/sessions/{id}/details`   | 会话详情 `SessionDetails`，见下文；不存在 404                                      |
| PATCH  | `/api/sessions/{id}/labels`    | 修改标签 `{set, remove}`，返回 `SessionInfo`；非法标签 400，不修改任何标签         |

创建请求有两种互斥形式：

```jsonc
// 显式自定义命令
{ "cwd": "/home/u/proj", "cmd": ["bash", "-lc", "make dev"], "name": "可选" }
// 已保存的启动配置
{ "cwd": "/home/u/proj", "configId": "k3m9q2x7", "name": "可选" }
```

自定义命令必须包含非空可执行程序，`cmd` 是完整 argv，不经 shell 展开；不再接收 `preset` 或单独的预设 `args`。可附加 `workspace: {repositoryId, worktreeId}`：省略 `cwd` 时使用工作树根目录，提供时必须位于该工作树内。

显式命令可附加可选的 `env: {"NAME": "value"}`：只为该会话在服务环境之上追加或覆盖列出的变量。

- 名称须匹配 `[A-Za-z_][A-Za-z0-9_]*`，值不含 NUL；`WEBMUX_SESSION_ID`、`WEBMUX_SOCKET` 与 `WEBMUX_BIN` 保留（服务端为每个会话设置它们），出现时 400。
- 服务端不自行复制调用方的环境，`env` 也不出现在 `SessionInfo` 中。
- Linux/macOS 的 `webmux run` 会把调用方环境（去掉终端身份等服务端自有变量）放进该字段发送。

`configId` 形式按已保存配置的完整 argv 与环境变量启动：

- `cmd` 须为空数组或省略，`env` 须省略或为 `{}`，否则 400。
- 空字符串 `configId` 400，未知 ID 404，启动配置存储不可用 503。这些检查都在目录与 Git 操作之前完成。
- 之后修改或删除该配置不影响已启动会话的命令与环境。会话信息保留 `configId`，首页按该配置当前保存的预览设置渲染，配置删除后回退浏览器全局设置。

启用混合模式（`hybrid: true`）的配置创建混合会话，可附加 `startSide: "tui" | "acp"`（缺省 `tui`）：

- `tui`：在 PTY 中运行档案生成的终端命令（OpenCode 注入 `--standalone`）。
- `acp`：不启动终端，而是启动 ACP 进程并依次调用 `initialize` 与 `session/new`。

可选 `prompt`（用户的第一条消息）与 `useInitInstruction`（把仓库的初始化指令放在它前面，需要 `workspace`，否则 400）：会话启动后自动发送。

- 聊天端（`acp`）作为第一条聊天消息；发送失败时不保留新会话。
- 终端（普通命令或混合的 `tui`）在程序有输出并安静 0.7 秒后（最长等 15 秒）输入并回车，多行文本在程序开启括号粘贴时按粘贴发送。
- 指令引用了不存在或保留的名称、`prompt` 与指令合计超过上限时 400。

混合会话的失败状态码（普通会话不受这些检查影响）：

- 对普通配置或自定义命令提供 `startSide`、或取值无效：400。
- 记录文件 `hybrid-sessions.json` 不可用：503，错误信息含文件路径。
- 记录已达 200 条：409 `{error, reason: "limit"}`。
- agent 启动或建会话失败：502，错误信息附 agent 标准错误的末尾，仅返回给请求方，不写日志。

**父会话与标签**：

- 创建请求可附加 `labels: {"KEY": "value"}`。键须匹配 `[a-z0-9][a-z0-9._-]{0,62}`，值最多 256 个字符且不含控制字符，最多 16 个；非法时 400，且不创建会话。
- 经本机套接字从某个会话内的进程创建时，服务端按对端进程记录 `parentId`（调用者所在会话）；请求体不能指定它，经 TCP 创建的会话没有。
- `SessionInfo` 中二者为空时省略，变化经 `/ws/sessions` 的 `updated` 事件同步；只保存在内存中。
- `PATCH /api/sessions/{id}/labels` 的 `set` 先合并、`remove` 再删除，整体生效；结果超过 16 个时 400。

**会话详情**：`GET /api/sessions/{id}/details` 返回：

```jsonc
{
  "session": { "id": "ab2cd3ef", "...": "SessionInfo" },
  "link": "http://127.0.0.1:7681/s/ab2cd3ef", // 服务不知道本机链接时为 null
  "repository": { "id": "…", "name": "app", "path": "/home/u/app" }, // 无工作区关联时为 null
  "worktree": {
    "id": "…",
    "branch": "fix/lint",
    "path": "/home/u/app.worktrees/fix-lint",
    "main": false,
  },
  "launchConfig": {
    "id": "k3m9q2x7",
    "name": "Claude Code",
    "icon": "claude",
    "hybrid": true,
  }, // 否则 null
  "launchConfigMissing": true, // 仅当配置已被删除
  "hybrid": { "...": "HybridRecord" }, // 普通会话为 null
  "artifactsDir": null, // 临时文件夹已存在时为路径；接口不创建它
  "forwards": [], // 来源为该会话的端口转发
  "parentId": "p4rent22",
  "labels": { "run": "42" },
  "children": ["ch2ld345"], // parentId 为该会话的会话，按创建时间排序
}
```

详情不含 Token、启动环境变量的值或其他会话的输出。

**上传**：`multipart/form-data`，恰好一个文件字段 `file`。成功返回 201 `{"path": "<服务器绝对路径>"}`。

- 文件位于会话临时文件夹的 `uploads/` 中，同名时追加 ` (2)` 等序号而不覆盖。
- 单文件上限 100 MiB（超限 413），读期限 5 分钟（超时 408）。
- 无效请求 400，会话不存在 404，会话已退出 409，临时文件夹不可用 503。
- `GET /api/sessions/{id}/upload?path=<服务器绝对路径>` 读取该会话 `uploads/` 内的普通非符号链接文件（上限 10 MiB，超限 413），按文件内容嗅探 `Content-Type` 并返回原始字节；其他路径或不存在的文件返回 400，会话不存在 404。

**会话临时文件夹**：`POST /api/sessions/{id}/artifacts` 幂等地创建并返回 `$TMPDIR/webmux-artifacts-<uid>-<实例摘要>/<会话ID>`（0700）。

- 会话存在（含已退出）时 200，不存在 404。
- 路径已被符号链接、他人目录或可被其他用户访问的目录占用时 503，且不修改该条目。
- 普通会话移除后删除文件夹；混合会话在 `DELETE /api/hybrid-sessions/{id}` 成功后删除；服务启动清扫不属于可恢复混合会话的文件夹。
- 代码视图以 `GET /api/sessions/{id}/code/context?root=artifacts` 浏览它（`rootKind: "artifacts"`，无 Diff）。`root` 缺省或为 `workspace` 时是工作区，其他值 400，文件夹尚未创建时 404。

```ts
interface SessionInfo {
  id: string;
  generation: string; // 本次运行的代际，16 位十六进制；输出 offset 与提交都绑定它。混合会话在聊天端（没有终端运行）时为空字符串
  title: string;
  cwd: string;
  cmd: string[];
  status: "running" | "waiting" | "idle" | "exited"; // waiting 表示提示符、终端读取或 attention；计时休眠不是 waiting
  exitCode: number | null;
  preview: string; // 屏幕底部最多 100 条非空行，以 \n 分隔
  createdAt: string; // RFC 3339
  lastActiveAt: string;
  cols: number;
  rows: number;
  clients: number; // 当前附着的连接数
  workspace: {
    repositoryId: string;
    worktreeId: string;
    worktreePath: string;
  } | null;
  // 程序主动报告的“需要介入”，存在时 status 为 "waiting"；不含任何终端正文。
  attention: {
    kind: "question" | "permission" | "done" | "error" | "notify" | "bell";
    source: "agent" | "terminal"; // 结构化事件 / 普通通知或响铃
    since: string; // RFC 3339，种类变化时更新
  } | null;
  // 程序通过 OSC 7 报告的目录；null 表示未报告或进程已退出。
  // 只作导航线索，不改变 cwd、workspace、读取边界或命令执行环境。
  reportedCwd: {
    path: string; // 已解码的绝对路径
    host: string; // 报告中的主机名，可能为空
    remote: boolean; // 非本机（例如 SSH 远端）；不得用于本机文件定位
    since: string; // RFC 3339，路径或主机变化时更新
  } | null;
  // 创建时使用的已保存启动配置；临时命令省略该字段。
  configId?: string;
  // 混合会话（终端 TUI ↔ 聊天视图）的所在端；普通会话省略该字段。
  hybrid?: {
    side: "tui" | "acp" | "switching";
    phase?: string; // 切换中的步骤，切换之外省略
    state: "active" | "closed" | "interrupted";
    agent: string;
    agentTitle: string; // agent 显示名
    agentIcon: string; // 品牌图标键，属于启动配置图标目录
    agentSessionId: string | null; // agent 自己的会话 ID，未知时为 null
  };
  // 会话进程树中监听在回环或通配地址上的 TCP 端口，按端口升序，不含 WebMux 自身端口；
  // 端口开始或停止监听后 3 秒内更新。不支持检测的平台（非 Linux/macOS）恒为 []。
  listeningPorts: { port: number; process: string }[];
}
```

## 其他

| 方法 | 路径                   | 说明                                                                                                                                                                                                           |
| ---- | ---------------------- | -------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| GET  | `/api/health`          | `{ok: true, sessions: n}`                                                                                                                                                                                      |
| GET  | `/api/config`          | `{defaultCwd, defaultShell, hostname, workspace: {gitAvailable, registryAvailable, gitError, registryError}, tunnel, pendingUpdate?, portDetection}`，不含 token；`portDetection` 表示本平台会话会报告监听端口 |
| GET  | `/api/fs/dirs?prefix=` | 匹配前缀的全部子目录（含符号链接目录），普通目录优先，按名称排序                                                                                                                                               |
| POST | `/api/fs/mkdir`        | `{path}`，204                                                                                                                                                                                                  |

`tunnel` 是只读的运行时隧道状态：`{enabled, state, named, hostname, publicUrl?}`。

- `state`：`off`、`starting`、`ready` 或 `failed`。
- `enabled`：仅在 `starting`/`ready` 时为 `true`，与[公网隧道设置](http-tunnel.md)的启用期望不同。
- `named` 与 `hostname`：动态读取当前有效设置（Web 覆盖优先，否则使用配置文件），关闭时也保留模式信息；Named Tunnel 缺少 `hostname` 时为空字符串。
- `publicUrl`：仅在 `ready` 时出现且不含 token。
- 响应不包含 Cloudflare 凭据。

`defaultShell` 为只读的启动文件 shell：`"bash"`、`"zsh"`，或空字符串表示不自动包装；由服务进程 `$SHELL` 的 basename 决定，未设置时为 `"bash"`。

`pendingUpdate` 仅在可执行文件路径上已安装的版本已知且不同于运行版本时出现：`{installedVersion, launcher}`，`launcher` 为 `systemd`、`launchd`、`windows-tray` 或 `manual`，决定 Web UI 给出的重启方式。旧服务端不返回该字段。

## 启动配置

启动配置保存在服务端 `<state_dir>/launch-configs.json`，在所有已认证设备间共享。只提供用户自定义的启动配置，不再提供内置预设或 `/api/presets` 路由；新安装首次启动时服务端在该文件写入常见 agent 的普通配置，经同一接口列出、修改和删除。

| 方法   | 路径                       | 说明                                                                                                   |
| ------ | -------------------------- | ------------------------------------------------------------------------------------------------------ |
| GET    | `/api/launch-configs`      | 摘要数组（按创建顺序）：`{id, name, icon?, cmd, envNames, chatOptions, preview, createdAt, updatedAt}` |
| GET    | `/api/launch-configs/{id}` | 完整配置：`{id, name, icon?, cmd, env, preview, createdAt, updatedAt}`                                 |
| POST   | `/api/launch-configs`      | `{name, icon?, cmd, env?, preview?}`，201 返回完整配置                                                 |
| PUT    | `/api/launch-configs/{id}` | `{name, icon?, cmd, env?, preview?}` 整体替换，省略的 `env` 与 `preview` 被清空，200 返回完整配置      |
| DELETE | `/api/launch-configs/{id}` | 204                                                                                                    |

`icon` 为可选的本地图标 ID：`terminal`、`antigravity`、`claudecode`、`codex`、`geminicli`、`cursor`、`githubcopilot`、
`opencode`、`qwen`、`kimi`、`codebuddy` 或 `pi`。

- 省略或空字符串表示自动匹配；更新时同样恢复自动匹配。
- 不接受 URL 或 SVG 内容。
- 旧文件省略此字段仍可读取：浏览器按可识别的直接命令或标准 Bash/Zsh 启动文件包装匹配图标，未匹配时显示通用终端。
- `terminal` 可显式覆盖自动匹配。

`preview` 为 `null` 或省略时沿用服务端同步的全局预览设置；否则为 `{lines, skip}`，`lines` 为 1–50 的整数、`skip` 为 0–50 的整数。会话卡片按创建时记录的 `configId` 读取该配置的当前预览值，配置被删除或不可用时回退全局设置。

列表的 `chatOptions` 是该配置最近报告的配置项清单与最后的值（结构同聊天状态的 `config`），还没有时为空数组：

- 来源是该配置最近一次聊天端会话，或快速开始的配置读取 `POST /api/quick-starts/options`；
- 服务端保存在 `<state_dir>/agent-options.json`，删除启动配置时一并清除；
- 它只供快速开始在 agent 启动前展示配置，不参与切换或恢复。

列表只含环境变量名（字典序），值只由单条读取与创建/更新响应返回。限制：

- 名称去首尾空白后 1–64 个字符且唯一（重名 409）。
- `cmd` 1–128 项、首项非空、单项不超过 8 KiB。
- `env` 最多 64 项，名称规则与会话 `env` 相同，值不超过 8 KiB。
- 最多 100 条。

错误状态码：

- 校验失败 400，未知 ID 404，落盘失败 500（内存回滚）。
- 文件损坏、目录不可读写或状态目录已被另一实例占用：503，错误信息包含原因与文件路径。

`launch-configs.json` 当前版本为 `2`，新增 `preview` 字段。本次升级仅接受版本 `2`；版本 `1` 文件保留原样，启动配置 API 返回 503。升级前备份旧配置，通过支持 v1 的旧版本保存为 v2，或备份后移走旧文件并在新版重新创建；详见 [部署升级说明](../deployment.md)。
