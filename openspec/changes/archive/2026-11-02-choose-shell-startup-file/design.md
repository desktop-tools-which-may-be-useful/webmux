## Context

动机见 proposal.md。

已核对的实现边界：

- `internal/launch/presets.go:32` 的 `bashrcCommand` 硬编码 `["bash","-ic","exec -- \"$@\"","webmux",…]`；`Presets` 只用 `$SHELL` 生成 Shell 配置，agent 预置不参考它。
- `cmd/webmux/main.go:380` 把 `os.Getenv("SHELL")` 传给 `launch.Presets`；`deploy/com.webmux.plist` 设置 `SHELL=/bin/zsh`，`deploy/webmux.service` 没有设置 `SHELL`。
- `internal/agentprofile/command.go` 用 `FormBashrc` 表示包装形式，`splitForm` 只匹配 Bash 前缀；`wrapExec`、`Lookup`、`ProtocolOnFD3`、`ACPArgv` 都按该形式分支。
- `internal/hybrid/connector.go:78` 与 `:168` 只在 `FormBashrc` 时经 `bash -ic` 执行 `command -v` 并识别退出码 127。
- `web/src/launch-configs.ts:18` 的 `BASHRC_PREFIX` 只识别 Bash；`LaunchConfigDialog.svelte:323` 的复选框文案为“加载 ~/.bashrc”。
- 既有 `launch-configs.json` 按设计不重写；`fileVersion` 保持 `2`。

## Goals / Non-Goals

**Goals:**

- 让 macOS 新安装的 agent 预置默认读取 `~/.zshrc`，同时保留 Bash 用户与旧配置的现有行为。
- 把“加载哪个 shell 的启动文件”做成显式选择，避免依赖服务环境猜测用户意图。
- 让混合会话的恢复、ACP 启动和连接器检查在 Bash 与 Zsh 下语义一致。
- 保持 `launch-configs.json` 版本与旧配置不变。

**Non-Goals:**

- 不支持 fish、nu、sh、tcsh 等其他 shell 的启动文件；它们不包装。
- 不读取 `.zprofile`、`.zlogin`、`.bash_profile` 等登录文件，也不改变 Shell 配置的 `-l -i` 语义。
- 不自动迁移或改写旧配置，不新增迁移命令。
- 不改变环境变量、PTY、回放、认证或会话生命周期。
- 不允许用户在启动配置里自由填写任意 shell 包装脚本。

## Decisions

### 1. 选择只存在于 argv，不新增持久字段

Bash 与 Zsh 包装都使用现有 `webmux` 标记，只是第一项与 `-ic` 的可执行文件不同：

```text
bash -ic 'exec -- "$@"' webmux <命令> <参数>…
zsh  -ic 'exec -- "$@"' webmux <命令> <参数>…
```

这样 `launch-configs.json` 继续是版本 2，无需迁移；旧配置的 `bash -ic` argv 仍被识别。编辑器通过精确前缀还原选择，任意用户手写的其他包装保持原样。

备选的新增 `startupShell` 字段需要升级文件版本并迁移旧文件，超出本次范围，不采用。

### 2. 默认 shell 由服务端统一解析

新增内部解析规则，输入 `$SHELL` 路径，输出 `"bash"`、`"zsh"` 或空：

- basename 为 `bash` → `"bash"`；
- basename 为 `zsh` → `"zsh"`；
- 未设置或为空 → `"bash"`，保持现有 Linux/systemd 行为，避免无 `SHELL` 的旧部署突然失去 `.bashrc`；
- 其他 shell → 空字符串，表示不自动包装。

`GET /api/config` 增加只读字段 `defaultShell`（`"bash"`、`"zsh"` 或 `""`）。

- 前端 `ConfigSchema` 对旧服务端缺省为 `""`。
- 编辑器在用户勾选“加载 shell 启动文件”时用 `defaultShell || "bash"` 作为初始值。
- 编辑已有包装配置时以 argv 中的 shell 为准。

这样 fish 用户不会在预置里被强行套 Bash，但手动勾选时仍有可用的显式选择。

### 3. 包装 shell 用 `-ic`，与现有 Bash 语义对齐

`bash -ic` 读取 `~/.bashrc`，`zsh -ic` 读取 `~/.zshrc`。两者都是交互、非登录 shell，避免登录 shell 的额外副作用。限制是 `.zprofile` 与 `.zlogin` 不会被读取；需要登录文件路径的用户仍可使用 Shell 配置或显式命令。若后续实测表明 macOS 用户主要把 PATH 放在 `.zprofile`，再单独评估 `-lic`。

### 4. 包装形式从单一 Bash 泛化为带 shell 的形式

服务端把 `FormBashrc` 泛化为“启动文件包装”，并在命令解析结果中携带所用 shell（`bash` 或 `zsh`）。

- `splitForm` 同时识别两种精确前缀。
- `wrapExec`、`Lookup`、`TUIArgv`、`ResumeArgv`、`ACPArgv`、`ProtocolOnFD3` 都按该 shell 生成或分支。
- 原来的 `bashrcScript` 保留为共享脚本，ACP 的 `1>&3 3>&-` 脚本对两种 shell 都适用。

不在 `Form` 里继续增加互斥的 `FormZshrc` 分支，否则每处分支都要维护两份；用 shell 字段避免组合爆炸。

### 5. 连接器检查复用同一 shell

`prepareACP` 与 `exitedNotFound` 从“是否 Bash 包装”改为“是否启动文件包装”，`Lookup` 自动使用配置的 shell。结果标记、超时、输出上限与退出码 127 的判定不变；`~/.zshrc` 的输出同样先于标记行，由现有解析丢弃。

### 6. 新安装预置沿用默认 shell，旧文件不动

`launch.Presets` 接收 `$SHELL`，按第 2 条解析后生成 agent argv：bash → Bash 包装，zsh → Zsh 包装，空 → 直接命令。Shell 配置仍为 `[$SHELL, "-l", "-i"]`，为空时回退 `/bin/sh`。`Seed` 只在文件不存在时写入，既有文件保持原样。

## Risks / Trade-offs

- [`.zprofile` 中的 PATH 不会被读取] → 在文档中说明包装只读取 `~/.zshrc`，并保留 Shell 配置；若实际反馈强烈再评估登录 shell。
- [zsh 启动文件输出可能干扰协议] → 继续使用 fd 3 包装脚本，并补 `.zshrc` 有输出的 ACP 握手测试。
- [服务环境没有 `SHELL`] → 明确回退 Bash，保持升级前后的行为稳定。
- [两个并行的 agent 档案变更都改同一要求] → 本变更只泛化包装写法，不改动档案列表；合入时按 OpenSpec 顺序解决文本冲突。
- [用户把任意命令手写成 zsh 包装] → 只识别 WebMux 的精确前缀，不把任意 `zsh -c` 当作混合模式写法。

## Migration Plan

1. 不迁移 `launch-configs.json`；升级后旧配置继续按原 argv 运行。
2. 新安装按 `defaultShell` 生成预置；已有安装需要用户手动编辑或新建配置。
3. 回滚时旧的 Bash 包装继续可用；Zsh 包装在旧服务端会作为不受支持的混合写法处理，但终端仍可执行。无数据格式变化。
4. 文档与 US-B01、US-B08 同步说明默认选择、其他 shell 回退与不迁移。
