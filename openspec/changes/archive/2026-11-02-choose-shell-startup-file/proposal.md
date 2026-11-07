## Why

现状有三个缺口：

- 新安装的 agent 启动配置一律写成 `bash -ic 'exec -- "$@"' webmux …`，只读取 `~/.bashrc`。
- macOS 的默认交互 shell 是 zsh，PATH 与 Homebrew、pnpm 等路径通常写在 `~/.zshrc`，而新机器上常常没有 `~/.bashrc`；launchd 示例虽然把 `SHELL` 设为 `/bin/zsh`，agent 预置仍可能找不到命令。
- 混合会话还有三处只认 Bash：恢复命令与 ACP fd 3 脚本、连接器 `command -v` 探测，以及前端的前缀识别与图标匹配。

## What Changes

- 按方案 1（显式选择）把启动配置编辑器里的“加载 ~/.bashrc”改为“加载 shell 启动文件”，旁边可选 Bash（`~/.bashrc`）或 Zsh（`~/.zshrc`）；默认按服务端 `$SHELL` 的 basename 在两者之间预选。
- 新安装的 agent 预置配置按同一规则选择包装 shell；`$SHELL` 为 fish、nu 等非 bash/zsh 时不包装。
- 服务端在 `GET /api/config` 增加只读的 `defaultShell`，供编辑器预选；不新增 `launch-configs.json` 字段，不改变文件版本。
- 泛化 agent 档案的包装形式：识别 Bash 与 Zsh 两种前缀，恢复命令、TUI 启动、ACP fd 3 脚本和连接器探测都跟随同一 shell。
- 更新相关文档与 US-B01、US-B08。
- 不迁移或改写既有 `launch-configs.json`：升级后旧配置保持原样，用户可手动编辑；本次不新增迁移。

## Capabilities

### New Capabilities

无。

### Modified Capabilities

- `launch-configs`：启动文件选择控件、默认 shell 协议字段、新安装预置的 shell 选择与旧配置不改写。
- `hybrid-agent-profiles`：可识别的包装形式、恢复/ACP 命令与连接器探测同时支持 Bash 与 Zsh。

## Impact

- 服务端：`internal/launch/presets.go`、`internal/agentprofile/command.go`、`internal/hybrid/connector.go`、`internal/contract/api.go`、`internal/api/router.go` 与相关测试；`cmd/webmux/main.go` 继续把 `SHELL` 传给预置。
- Web：`web/src/launch-configs.ts`、`web/src/components/LaunchConfigDialog.svelte`、`web/src/schemas.ts` 及启动配置的单元/浏览器测试。
- 文档与故事：`docs/cli-agents.md`、`docs/usage.md`、`docs/usage/terminal.md`、`docs/hybrid-sessions.md`、`docs/deployment.md`、`docs/protocol/http-sessions.md` 与 `docs/protocol/http-hybrid.md`（如有接口字段变化）；US-B01、US-B08。
- 无新增依赖；不改变认证、Origin、会话生命周期、回放或环境变量契约。
- 本次实现不迁移既有数据；评审通过后合入。
