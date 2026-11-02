# Design

## Context

动机见 proposal.md。现状：

- `install.sh` 是 POSIX `sh` 脚本，通常以 `curl … | sh` 运行，此时标准输入是脚本本身；现有配置项全部是 `WEBMUX_*` 环境变量。
- `webmux service install` 负责生成配置、写入服务定义并启动服务；已有定义时不带 `--force` 会报错，带 `--force` 会重启服务。
- `service install --dry-run` 输出 `# Service definition: <路径>`，不写文件、不创建配置。

## Goals / Non-Goals

**Goals:** 一个环境变量加一次可选询问，复用 `service install` 的全部逻辑，不在脚本中复制服务定义或配置生成。

**Non-Goals:** 不卸载或重启已有服务；不处理 root/系统级服务；不改 Go 代码。

## Decisions

### D1：`WEBMUX_SERVICE` 环境变量，而不是命令行参数

与现有 `WEBMUX_VERSION` 等保持一致，`curl … | VAR=1 sh` 的写法也比 `sh -s -- --service` 更常见。取值只接受 `1`/`0`，其他值在任何下载前失败，避免拼写错误被当成“否”。

### D2：从 `/dev/tty` 询问

管道运行时 stdin 不可用，脚本先用 `( : </dev/tty )` 判断能否打开终端，再从 `/dev/tty` 读取回答；默认“是”，与 Windows 安装程序默认勾选一致。打不开终端（CI、cron、无控制终端的 SSH 命令）时不询问、不安装，保持旧行为。测试通过 `WEBMUX_TTY` 指向一个文件来模拟终端（与 `WEBMUX_BASE_URL` 一样仅供测试）。

### D3：已有服务的判断交给二进制

脚本运行新二进制的 `service install --dry-run`，从 `# Service definition:` 行取得定义路径；文件存在即视为已有服务，不再调用 `service install`。路径规则（`XDG_CONFIG_HOME`、`~/Library/LaunchAgents`）因此只保存在 Go 代码中。定义文件中不包含本次安装路径时，提示 `--force` 切换。

备选：直接调用 `service install` 并识别“already installed”报错——依赖错误文案，且无法区分其他失败。

### D4：服务失败不回滚二进制

二进制已校验并原子替换，回滚没有意义；脚本输出 `service install` 的错误并以退出码 1 结束，提示修复后重试的命令。

## Risks / Trade-offs

- [`--dry-run` 的输出格式变化会使检测失效] → 无法解析时按“没有已有服务”处理，随后的 `service install` 仍会因已有定义而失败并给出报错，不会覆盖服务；测试覆盖解析。
- [交互默认“是”改变了在终端中运行脚本的结果] → 只在能打开终端时询问，问题中说明会写入用户服务；`WEBMUX_SERVICE=0` 保留旧行为。
