# 安装与升级

部署 WebMux 的运行要求、安装方式、升级与持久化升级诊断。

## 运行要求

- Linux 或 macOS（amd64 / arm64）。
- 一个可执行的 shell，以及打算使用的 CLI agent（Claude Code、Codex CLI 等）。
- 可选：Git，用于仓库与 worktree 管理（需支持 `git worktree list --porcelain -z`）。

运行端不需要 Node.js、Go、tmux 或数据库。例外：Codex CLI 与 Claude Code 的混合模式（聊天 ↔ 终端）分别需要第三方适配器 codex-acp、claude-agent-acp；它们运行在 Node 上，Claude 连接器要求 Node.js 22 或更高版本，见 [配置项](configuration.md#配置项)。

## 安装

Linux/macOS 推荐使用仓库根目录的 [`install.sh`](../../install.sh)。

- 依赖：POSIX `sh`、`curl`、`uname`、`mktemp`、`awk`，以及 `sha256sum` 或 `shasum`（任一即可）。
- 流程：自动识别 Linux/macOS 与 amd64/arm64（Apple Silicon 的 Rosetta 终端优先选择 arm64），下载对应二进制与 `SHA256SUMS`，校验通过后才原子替换目标文件。
- 默认安装到 `~/.local/bin/webmux`，不需要 sudo，也不会执行 `config init`、`service install` 或改写 shell 配置。

安装脚本本身不依赖 `raw.githubusercontent.com`。任选一个可访问的入口：

```sh
# GitHub 直连；install.sh 随 Release 一起发布
curl -fsSL https://github.com/mzdluo123/webmux/releases/latest/download/install.sh | sh

# 国内网络：ghfast.top 代理同一 Release 资产
curl -fsSL https://ghfast.top/https://github.com/mzdluo123/webmux/releases/latest/download/install.sh | sh

# jsDelivr 读取仓库中的脚本；脚本随后仍从 GitHub 或镜像下载二进制
curl -fsSL https://cdn.jsdelivr.net/gh/mzdluo123/webmux@main/install.sh | sh
```

| 变量                 | 默认值                                    | 说明                                                            |
| -------------------- | ----------------------------------------- | --------------------------------------------------------------- |
| `WEBMUX_VERSION`     | 最新 Release                              | 指定 `x.y.z`（可带 `v` 前缀），从 `releases/download/v...` 下载 |
| `WEBMUX_INSTALL_DIR` | `~/.local/bin`                            | 目标目录；不可写时明确报错，不自动 sudo                         |
| `WEBMUX_MIRRORS`     | `https://ghfast.top https://gh-proxy.com` | 空格分隔的 gh-proxy 前缀；空值表示只用 GitHub                   |
| `WEBMUX_SHA256`      | 空                                        | 固定二进制 SHA-256，跳过同源 `SHA256SUMS`                       |

```sh
curl -fsSL https://github.com/mzdluo123/webmux/releases/latest/download/install.sh \
  | WEBMUX_VERSION=0.2.0 WEBMUX_INSTALL_DIR="$HOME/bin" sh
```

下载顺序为 GitHub 直连，然后依次尝试镜像（地址为 `<前缀>/https://github.com/...`），每个来源都有连接、低速和总时限。

- 二进制与 `SHA256SUMS` 必须来自同一来源并通过校验；失败时保留原有二进制、清理临时文件，并汇总各来源的错误。
- 镜像是第三方服务，能看到请求者 IP。
- 同源校验只能发现截断和损坏，无法防范同时替换两者的恶意镜像；高保证或离线场景请用 `WEBMUX_SHA256` 固定摘要。

脚本只安装二进制。完成后按提示运行 `webmux --listen 127.0.0.1:7681`。

- 需要后台服务时再执行 `webmux service install`，首次启动会自动生成配置与 Token。
- 重复执行脚本会升级二进制；正在运行的服务需手动重启，重启会结束现有会话。

### 手动下载

也可以不用脚本，手动下载并校验 Release 资产：

- 服务端：`webmux-{linux,darwin}-{amd64,arm64}`。
- Windows 托盘：`webmux-tray-windows-{amd64,arm64}.exe`；控制台客户端：`webmux-windows-{amd64,arm64}.exe`，见 [Windows（WSL）部署](windows-wsl.md)。

```sh
curl -LO https://github.com/mzdluo123/webmux/releases/latest/download/webmux-linux-amd64
curl -LO https://github.com/mzdluo123/webmux/releases/latest/download/SHA256SUMS
sha256sum --check --ignore-missing SHA256SUMS   # macOS: shasum -a 256 -c --ignore-missing SHA256SUMS
install -m 755 webmux-linux-amd64 ~/.local/bin/webmux
```

### Linux 安装包（deb / rpm）

Release 中提供 amd64、arm64 的 deb 与 rpm 包，二进制与同一 Release 的 `webmux-linux-<架构>` 相同，安装需要 sudo，之后由 apt / dnf 升级与卸载。

```sh
# Debian / Ubuntu（arm64 换成 _arm64.deb）
sudo apt install ./webmux_0.2.0-1_amd64.deb
# Fedora / RHEL / openSUSE（arm64 换成 .aarch64.rpm）
sudo dnf install ./webmux-0.2.0-1.x86_64.rpm
```

- 包内只有 `/usr/bin/webmux` 与 systemd 用户单元 `/usr/lib/systemd/user/webmux.service`，没有依赖，也没有安装脚本。
- 安装、升级、卸载都不会为任何用户启用、启动或重启服务。需要开机自启的用户执行一次 `systemctl --user enable --now webmux`；首次启动会在 `~/.config/webmux/config.toml` 生成带随机 Token 的配置。
- 升级后正在运行的服务继续使用旧进程，执行 `systemctl --user restart webmux`（或 `webmux service restart`）后生效，重启会结束现有会话。
- `webmux service status|start|stop|restart|logs` 同样作用于包内单元；`webmux service uninstall` 只禁用并停止它，删除请卸载软件包。也可以改用 `webmux service install`，它写入的 `~/.config/systemd/user/webmux.service` 优先于包内单元。

Windows 不使用 `install.sh` 或这些安装包：请使用 `webmux-setup-windows-*.exe` 安装托盘，或下载 `webmux-windows-*.exe` / 独立托盘程序，或按 [Windows（WSL）](windows-wsl.md) 在 WSL 中运行 `install.sh` 或安装 deb / rpm。

## 升级

替换二进制后重启服务。**重启会结束所有终端会话**，升级前请保存工作；仓库清单和磁盘上的 worktree 会保留。混合会话在重启后标记为已中断，可从首页“可恢复”分组恢复到聊天或终端（对话由 agent 保存，排队消息不保留）。只替换磁盘上的文件不会让正在运行的进程加载新版本。

## 持久化升级与诊断

完整路径、锁与故障策略见 [持久化清单](../persistence.md)。新版统一使用数据文件名加 `.lock`，包含新增 `settings.json` 与 `settings.json.lock`。升级前停止旧进程并备份，不支持新旧版本共用状态目录。旧 Tailscale 所有权文件须先核实其外部入口；不能仅删除记录后继续暴露服务。

`config show --json` 查看磁盘配置，`config show --live --json` 查看运行进程实际采用值。输出包含字段来源与脱敏的存储健康；离线检查不创建或修复文件。推送文件损坏保留原位并禁用推送，修复后重启；不会自动丢弃订阅并生成新密钥。
