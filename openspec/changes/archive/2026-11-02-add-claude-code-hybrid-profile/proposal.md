## Why

混合模式已经支持 OpenCode、omp 与 Codex CLI，但预置的 Claude Code 启动配置仍只能作为普通终端会话使用。

#181 当时因第三方适配器认证失败而暂缓 Claude Code。旧适配器现已迁移为仍在维护的 `@agentclientprotocol/claude-agent-acp`，具备 ACP v1 的会话列出、加载、恢复、权限、配置和附件能力，需要重新验证并接入。

本变更让 Claude Code 在同一 WebMux 会话中双向切换原生 TUI 与 ACP 聊天视图，并复用现有连接器检查、单写者切换、恢复和聊天能力，而不是另建一套 Claude 专用工作流。

## What Changes

- 新增 Claude Code 档案：识别 `claude` 与 `app=claude-code`，支持直接执行及既有 bashrc 包装，以 `--resume <id>` 恢复 TUI，并声明参数分类、一次 Esc 中断和启动静默期。
- ACP 端由用户另行安装的 `claude-agent-acp` 提供，使用 ACP v1；安装命令跟随 `@agentclientprotocol/claude-agent-acp` 的 `latest`，本次以 0.99.1 为实测基线。WebMux 不自动安装连接器或 Node。
- ACP 进程通过 `CLAUDE_CODE_EXECUTABLE` 指向 TUI 实际使用的 Claude Code，并复用同一服务用户、`HOME` 与 `CLAUDE_CONFIG_DIR` 下已有的订阅登录、API key 或第三方 provider；不保存或记录凭据。
- 通过 Claude CLI 的 `<CLAUDE_CONFIG_DIR>/sessions/<pid>.json` 进程注册记录跟踪当前会话 ID，校验 PID、进程启动标识、cwd 与配置目录；记录不可信或不可读时退回 `session/list` 候选选择。利用同一注册记录尽力发现外部 Claude 客户端占用。
- 从 argv 推导首次聊天端的 `model`、`effort` 与 `mode`，包括 `--model`、`--effort`、`--permission-mode` 和 `--dangerously-skip-permissions`；冷 load/resume 后重应用 model/effort，保留适配器恢复的实际 mode，并隐藏聊天菜单中的 `login`/`logout`。
- 启用适配器已声明的 `session/load`、`session/resume`、配置项、图片/嵌入资源与 steering；空闲时 steering 使用 `promptRequired`，避免产生 WebMux 未跟踪的回合。
- 新安装的 Claude Code 预置配置改为 `hybrid: true`；已有 `launch-configs.json` 不改写，用户可手动开启。
- 已在本机 Linux 原生 Claude Code 2.1.316、Node 26.13.0、连接器 0.99.1 与当前第三方 provider 上完成真实请求探测，固定认证、双向恢复、列表可见性、配置重应用、权限、附件、OSC、中断和进程清理行为；实现后按同一基线做 round-trip 验收，fakeagent 与 E2E 覆盖回归。

## Capabilities

### New Capabilities

无。本次复用现有第三方 ACP 连接器、混合会话、聊天队列与恢复能力。

### Modified Capabilities

- `hybrid-agent-profiles`：内置档案增加 Claude Code；声明其连接器、参数、恢复、ACP 环境、配置推导、版本与聊天能力。
- `agent-session-identity`：新增 Claude 进程注册记录这一会话 ID 来源、候选降级与外部占用识别，并明确相应隐私和资格规则。
- `launch-configs`：新安装的 Claude Code 预置改为默认混合，已有配置保持不变。

## Impact

- Go：新增 `internal/agentprofile/claudecode`，更新注册表与契约测试；`internal/session` 提供必要的 agent-neutral 进程事实；`internal/hybrid` 接入注册记录跟踪和占用判定；`internal/launch` 更新新安装预置。
- ACP/契约：沿用现有 ACP v1、connector、candidate、chat config 与冲突负载；若调研后的实现确需新增通用字段，须同步 `contract`、`wire`、Zod 和 wire 快照，不增加 Claude 专用前端分支。
- 测试：扩展 agentprofile 契约、进程注册记录、hybrid 切换/恢复、fakeagent、浏览器与 E2E；真实验收覆盖当前稳定连接器和支持的部署形态。
- 部署：聊天端需要 Node 22+ 与 PATH 中的 `claude-agent-acp`；TUI 端不依赖连接器。文档需说明浮动版本风险、未文档化注册记录格式、外部占用检测边界及订阅使用条款。
- 用户故事与文档：更新混合会话相关 US-Q 故事、部署和协议说明；不改变 WebMux 的持久 PTY 或 durable session store 边界。
