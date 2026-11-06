# Proposal

## Why

项目中原有的 Gemini CLI 已更名为 Antigravity CLI（可执行程序 `agy`），并且 `agy`
原生提供 `agy acp`（Agent Client Protocol stdio 服务）与 `--conversation <id>`
会话恢复能力。当前 WebMux 的默认启动配置仍预置旧的 `Gemini CLI`（`gemini --yolo`），
且内置混合会话适配器尚未覆盖 `agy`，导致用户无法在 WebMux 中直接将
`agy --dangerously-skip-permissions` 作为混合会话（聊天视图 ↔ 终端 TUI）使用。

## What Changes

- 新增 `agy`（Antigravity CLI）内置混合会话适配器（`internal/agentprofile/agy`，
  实测基线见 `baseline.md`）：
  - 识别 `agy` 直接执行与 WebMux Bash/Zsh 启动文件包装写法；
  - 白名单保留参数：`--dangerously-skip-permissions`、`--sandbox`、`--agent`、
    `--add-dir`、`--log-file`、`--model`、`--effort`、`--mode`；其中前五项透传给
    `agy acp`，`--model` 与 `--effort` 分别作为 `model` 与 `thought_level` 配置种子延续到聊天端，
    `--mode` 在聊天视图列为不支持；
  - 恢复参数：`--conversation <id>`；
  - 丢弃参数：位置提示词、`-c`/`--continue`、`--conversation`、
    `-i`/`--prompt-interactive`、`--settings`、`--remote-control` 及未知参数；
  - 拒绝参数：`--project`、`--new-project`（云端会话库，`session_store`）；
    `-p`/`--print`/`--prompt`、`--output-format`、`--input-format`、
    `--json-schema`、`--print-timeout`、`--disable-slash-commands`、
    `-h`/`--help`、`-v`/`--version` 及子命令（`non_interactive`）；
  - 在混合会话 TUI 启动与恢复时注入会话级 `--settings`，通过 `title` 命令将当前
    `conversation_id` 以 `OSC 7501` 子记录上报到控制终端，结合 `agy` 原生发出的
    `OSC 7501` 根记录（`app=antigravity`）与根进程 `cwd` 完成身份识别、变化跟踪与资格核对；
  - 配置中断键为单次 `Esc`（`\x1b`），启用 `session/resume` 异常重连，声明跨进程加载后
    需自动补齐的配置项 `ReapplyOnLoad: ["model", "thought_level"]`，记录实测版本 `1.4.12`。
- 在混合会话 ACP 转换层（`internal/hybrid`）中忽略活动 `session/prompt` 回合期间
  agent 回显的 `user_message_chunk` 通知，仅在 `session/load` 历史回放阶段转换
  `user_message_chunk`，避免 `agy acp` 在实时回合中回显用户消息导致聊天时间线出现重复的用户消息气泡。
- 放宽 `OSC 7501` 子记录 `id` 单段长度上限（从 32 字节调整为 64 字节，总长仍不超过 128 字节），
  使 `agy` 使用的标准 36 字符 UUID 会话 ID 能作为合法子记录第一段被采集。
- 更新新安装默认预置的启动配置与图标（`internal/launch`、`web/src/`）：
  - 将首次创建 `launch-configs.json` 时预置的 `Gemini CLI`（`gemini --yolo`，
    `hybrid: false`）替换为 `Antigravity CLI`（`agy --dangerously-skip-permissions`，
    图标 `antigravity`，`hybrid: true`）；
  - 新增本地打包的 LobeHub `antigravity.svg` 图标选项并按 `agy` 自动匹配，同时保留对已有
    `launch-configs.json` 中 `geminicli` 图标值的兼容读取与渲染，避免升级后已有配置文件报错 503。
- 扩展测试用 `fakeagent` 的 `--profile agy` 模式，覆盖 `agy` 的参数形态、UUID 会话 ID、
  `OSC 7501` 信号、实时回合回显 `user_message_chunk`、跨进程配置项重置与单次 `Esc` 中断。

## Capabilities

### New Capabilities
<!-- 无新增顶层 capability，全部归入现有 capability 的增量规格 -->

### Modified Capabilities
- `hybrid-agent-profiles`: 新增 `agy` 内置档案，定义其可识别命令、参数白名单/丢弃/拒绝表、
  TUI 会话标识注入、ACP 启动参数透传、恢复命令生成、跨进程加载配置补齐项与实测版本 `1.4.12`。
- `hybrid-sessions`: 补充 `agy` 跨进程加载时的配置项补齐（`model` 与 `thought_level`）
  以及在活动 `session/prompt` 回合期间忽略 agent 回显的 `user_message_chunk` 通知。
- `agent-session-identity`: 增加 `agy` 的终端身份映射规则（`app=antigravity` +
  UUID 格式的 `OSC 7501` 子记录 `id` + 根进程 `cwd`）。
- `program-status`: 将 `OSC 7501` 子记录 `id` 单段长度上限从 32 字节放宽到 64 字节以容纳
  36 字符 UUID，并登记 `agy` 对子记录 `id` 的身份跟踪用途。
- `launch-configs`: 将新安装预置的 `Gemini CLI`（`gemini --yolo`）替换为默认开启混合模式的
  `Antigravity CLI`（`agy --dangerously-skip-permissions`），新增 `antigravity`
  图标并保持已有 `geminicli` 配置兼容。

## Impact

- 后端包：`internal/agentprofile`、`internal/agentprofile/agy`、
  `internal/agentprofile/agents`、`internal/hybrid`（`actor.go` 活动回合过滤
  `user_message_chunk`）、`internal/session`（`osc.go` 子记录段长常量）、
  `internal/launch`（`presets.go`、`types.go`）、`internal/acp/testdata/fakeagent`。
- 前端与资源：`web/src/icons/lobehub/antigravity.svg`、
  `web/src/components/LaunchConfigIcon.svelte`、`web/src/launchConfigs.ts`
  及相关单元/浏览器测试。
- 文档与用户故事：`docs/user-stories.md`、`docs/user-stories/access-and-sessions.md`、
  `docs/user-stories/hybrid-sessions.md`、`docs/user-stories/coverage.md`、
  `docs/protocol/http-sessions.md`、`docs/cli-agents.md`、`docs/hybrid-sessions.md`、
  `docs/design-system.md`、`docs/usage.md`。
