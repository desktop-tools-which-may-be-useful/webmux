## 1. 档案数据模型扩展

- [ ] 1.1 `internal/agentprofile/profile.go`：
  `Profile` 新增 `ACPArg string` 字段，
  文档说明默认空值等同于 `"acp"` 子命令，
  设置后替代该默认值
- [ ] 1.2 `internal/agentprofile/command.go`：
  `ACPArgv()` 使用 `c.Profile.ACPArg`（非空时）
  替代硬编码的 `"acp"`
- [ ] 1.3 `internal/agentprofile/command_test.go`：
  新增测试覆盖 `ACPArg` 为 `"--acp"` 时的 argv 生成

## 2. Qoder 档案

- [ ] 2.1 创建 `internal/agentprofile/qoder/qoder.go`：
  定义 `Name = "qoder"`、`Title = "Qoder"`、
  `Executables = ["qoder"]`、`VerifiedVersion`、
  `ACPArg = "--acp"`、`ResumeFlag = "--resume"`、
  `HiddenCommands = ["login", "logout"]`
- [ ] 2.2 定义参数分类表 `qoderOptions()`：
  对齐 Claude Code 的参数分类
  （Keep: `--model`, `--reasoning-effort`, `--thinking`,
  `--permission-mode`, `--system-prompt`,
  `--append-system-prompt`, `--agents`, `--agent`,
  `--plugin-dir`, `--settings`, `--setting-sources`,
  `--mcp-config`, `--allowed-tools`, `--disallowed-tools`,
  `--tools`, `--add-dir`, `--strict-mcp-config`, `--bare`,
  `--fallback-model`;
  Drop: `-c`/`--continue`, `-r`/`--resume`;
  Session: `--fork-session`, `--session-id`, `--from-pr`;
  Directory: `-w`/`--worktree`, `--cwd`;
  NonInteractive: `-p`/`--print`, `-h`/`--help`,
  `-v`/`--version`, `-o`/`--output-format`,
  `--input-format`, `--no-session-persistence`;
  Store: `--config-dir`）
- [ ] 2.3 定义子命令表：`mcp`, `hooks`, `hook`, `rollback`,
  `update`, `config`, `plugins`, `plugin`, `commit`,
  `agents`, `agent`, `skills`, `skill`, `feedback`,
  `security`, `login`, `status`, `remote-control`,
  `sites`, `wiki`
- [ ] 2.4 实现 `Identify()`：
  匹配 `s.ProgramApp == "qoder"` 或空
  （无 OSC 7501 时依赖 session/list 候选）
- [ ] 2.5 实现 `InterruptKeys()`：
  返回 `[][]byte{[]byte("\x1b")}`（一次 Esc）
- [ ] 2.6 实现可选接口：
  `ResumesSessions() bool` 返回 `true`；
  `ReapplyOnLoad() []string` 返回 `["model", "effort"]`；
  `SessionSteering` 嵌入（`IdlePrompt: true`）
- [ ] 2.7 实现 `ChatSide` 函数：
  从 argv 推导 `model`、`effort`（`--reasoning-effort`）、
  `mode`（`--permission-mode`、`--dangerously-skip-permissions`），
  列出聊天端不生效的参数
- [ ] 2.8 `internal/agentprofile/qoder/qoder_test.go`：
  参数分类、恢复命令、ACP argv、ChatSide 推导

## 3. QoderCN 档案

- [ ] 3.1 创建 `internal/agentprofile/qodercn/qodercn.go`：
  定义 `Name = "qodercn"`、`Title = "QoderCN"`、
  `Executables = ["qodercn", "qoderclicn", "qoder-cn"]`，
  其余与 Qoder 档案相同但 ACP agent 名为 `qoder-cli-cn`
- [ ] 3.2 `Identify()` 匹配
  `s.ProgramApp == "qoder-cli-cn"` 或 `"qodercn"`
- [ ] 3.3 `internal/agentprofile/qodercn/qodercn_test.go`：
  参数分类、恢复命令、ACP argv

## 4. 注册与预置

- [ ] 4.1 `internal/agentprofile/agents/agents.go`：
  在 `defaultRegistry` 中添加 `qoder.Agent` 和 `qodercn.Agent`
- [ ] 4.2 `internal/launch/presets.go`：
  新增两个预置配置：
  - `{Name: "Qoder", Icon: "qoder",
     Cmd: command("qoder", "--dangerously-skip-permissions"),
     Hybrid: true}`
  - `{Name: "QoderCN", Icon: "qoder",
     Cmd: command("qodercn", "--dangerously-skip-permissions"),
     Hybrid: true}`
- [ ] 4.3 `internal/launch/types.go`：
  `normalize()` 图标白名单增加 `"qoder"`

## 5. 前端图标

- [ ] 5.1 创建 `web/src/icons/lobehub/qoder.svg`：
  Qoder 品牌图标（或使用合适的通用图标）
- [ ] 5.2 `web/src/components/LaunchConfigIcon.svelte`：
  导入并注册 `qoder` 图标
- [ ] 5.3 `web/src/launchConfigs.ts`：
  `LAUNCH_ICON_OPTIONS` 增加
  `{value: "qoder", label: "Qoder"}`；
  `launchConfigIcon()` 自动检测
  `qoder`/`qodercn`/`qoderclicn` → `"qoder"`

## 6. 集成测试

- [ ] 6.1 `internal/agentprofile/agents/agents_test.go`：
  更新档案数量与识别测试
- [ ] 6.2 `internal/api/testdata/wire/`：更新 wire 快照
- [ ] 6.3 `web/test/e2e/launch-configs.spec.ts`：
  验证 Qoder/QoderCN 预置配置的混合模式开关
- [ ] 6.4 fakeagent 扩展：
  增加 Qoder/QoderCN 档案的测试替身

## 7. 文档

- [ ] 7.1 `docs/cli-agents.md`：新增 Qoder 与 QoderCN 段落
- [ ] 7.2 `docs/hybrid-sessions.md`：更新支持的 agent 列表
- [ ] 7.3 相关用户故事更新
