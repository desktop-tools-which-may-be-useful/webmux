# Design

## Context

- 会话列表快照、列表事件与单个会话信息都经同一个转换函数 `wire.Session` 输出 `hybrid` 对象，其中已有 `agent`（档案名，如 `opencode`、`omp`），所以首页与会话页无需额外请求即可拿到 agent 身份。
- 档案 `agentprofile.Profile` 已有显示名 `Title`（`omp`、`OpenCode`、`Codex CLI`、`Claude Code`），但它没有进入任何负载。
- `internal/archtest/agentnames_test.go` 禁止 `web/src` 出现已注册 agent 的名称（只豁免启动配置图标目录 `LaunchConfigIcon.svelte`），因此前端不能自建“agent 名 → 显示名/图标”的对照表。
- 本地已打包的 LobeHub 单色品牌图标由 `LaunchConfigIcon.svelte` 按图标键渲染，图标键目录与 `internal/launch/types.go` 的允许值一致；该组件目前固定为 `size-5`，没有尺寸选项。
- 首页卡片 `SessionRow.svelte` 的元信息行已有“所在端”片段（通用聊天/终端图标 + `hybridSideLabel`）；会话页头部 `TerminalHeader.svelte` 的标题区只有标题、状态点与连接状态，聊天端与终端端共用该头部，`hybrid.view` 表示当前显示的视图。

## Goals / Non-Goals

**Goals:**

- 由服务端给出 agent 的显示名与图标键，新增 agent 时只改适配器子包。
- 复用现有品牌图标资源与 `LaunchConfigIcon`，不新增图标集或第二套图标组件。

**Non-Goals:**

- “可恢复”分组的记录卡片：它目前直接显示档案名，统一改造涉及记录列表负载，不在本次范围。
- 终端端的卡片与头部：用户在终端中能直接看到 agent 自身的界面。
- 浏览器标签页标题、推送通知内容。
- 任何交互（点击标识查看 agent 版本、切换 agent 等）。

## Decisions

### D1：显示名与图标由服务端随 `hybrid` 对象下发

在 `session.HybridInfo` 增加 `AgentTitle`、`AgentIcon`，`internal/hybrid/actor.go` 在构造 `HybridInfo` 时从 `a.cmd.Profile` 读取 `Title` 与新增的 `Icon`；`contract.SessionHybrid` 增加 `agentTitle`、`agentIcon`，`wire.Hybrid` 直接拷贝。

- 备选：前端维护对照表。被 archtest 的 agent 名称约束否决，而且每加一个 agent 都要改前端。
- 备选：新增 `GET /api/agents` 目录接口，前端按 `agent` 查询。多一次请求与一份缓存，只为两个短字符串，不值得。
- `hybrid` 包只读取档案的通用字段，不出现任何 agent 名称，保持 agent 中立。

### D2：图标键放在档案上，取值属于启动配置图标目录

`agentprofile.Profile` 新增 `Icon string`，各适配器声明：omp → `pi`，OpenCode → `opencode`，Codex CLI → `codex`，Claude Code → `claudecode`（与 `internal/launch/presets.go` 中对应预设一致）。

- 不能用会话的启动配置 `icon`：它由用户选择，可能是 `terminal` 或其他品牌，与实际 agent 不符。
- 一致性由 `internal/launch` 的测试守护：遍历 `agents.Default().Agents()`，断言每个 `Profile().Icon` 都是启动配置允许的图标键且非空。放在 `launch` 是因为 `agentprofile` 属于共享层，不能导入服务包。

### D3：前端用 `LaunchConfigIcon` 渲染，新增尺寸选项

`LaunchConfigIcon` 增加 `size` 选项（`xs` = `size-3.5`、`sm` = `size-4`、默认 `md` = `size-5`），符合“给设计系统组件加尺寸而不是用 class 改样式”的约定。卡片用 `xs`，与同行 tabler 图标一致；头部用 `sm`。

未知图标键当前回退为终端图标，这对 agent 标识会造成误导。组件对外增加一个判断函数（例如导出 `hasLaunchIcon(icon)`），调用方据此决定只显示名称；启动配置的现有回退行为不变。

`SessionHybridSchema` 增加 `agentTitle: z.string()` 与 `agentIcon: z.string()`。两者是服务端必有字段，按必填校验，与 `agent` 一致。

### D4：卡片——品牌图标替换通用聊天图标

`SessionRow` 中 `hybrid.side` 为 `acp` 或 `switching` 时，“所在端”片段渲染为 `[品牌图标] {agentTitle} · {hybridSideLabel}`；其余情况保持现有 `IconMessageCircle`/`IconTerminal2` 分支。只替换而不是并排追加，是为了让元信息行仍只有一个前导图标，信息密度不变。

判断条件用 `side` 而不是现有的 `chat` 派生值（它在 `phase` 存在时为假）。切换期间 `side` 为 `switching`，因此把它一并纳入，休眠和切换过程中仍能看出 agent。

### D5：头部——标题后的低强调标识

`TerminalHeader` 在 `hybrid?.view === "chat"` 且 `t.info?.hybrid` 存在时，在标题 `<span>` 与状态点之间渲染一个 `shrink-0` 的 `<span role="img">`：

- 内含 `LaunchConfigIcon size="sm"` 与名称 `<span class="max-sm:hidden text-xs text-muted-foreground">`；
- `aria-label` 与 `title` 都为 `Agent：{agentTitle}`，满足“仅图标时有可访问名称与可见提示”；
- 标题保持 `min-w-0 truncate`，标识 `shrink-0`，窄屏时只截断标题。

放在标题旁而不是“聊天 | 终端”分段控件旁：标识描述的是“这个会话是谁”，与标题同属身份信息；放在控件旁会被误读为可切换的选项。不使用 `Badge`：带边框的徽章在头部显得过重，与“不太明显”的要求不符。

## Risks / Trade-offs

- [新增必填字段使旧前端/新后端或新前端/旧后端不匹配] → 前端与后端由同一二进制嵌入发布，不存在混合版本；wire 快照会捕获字段变化。
- [后续新增 agent 时漏填 `Icon`] → D2 的 `internal/launch` 测试在 `Icon` 为空或不在目录中时失败。
- [头部在 320px 屏幕上空间更紧] → 窄屏只占一个 16px 图标，标题先截断；浏览器组件测试覆盖窄屏布局。
- [品牌图标为单色] → 跟随 `currentColor`，深浅主题自动适配；这是现有启动配置图标的既定做法。

## Migration Plan

纯新增字段，无持久化数据变化；混合会话记录文件格式不变（显示名与图标在运行时从档案读取）。回滚即回退代码。
