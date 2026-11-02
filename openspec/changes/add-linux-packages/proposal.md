# Proposal

## Why

Linux 用户目前只能用 `install.sh` 或手动下载二进制安装，无法通过 apt/dnf 安装、升级和卸载，也没有随系统分发的 systemd 用户服务。提供 deb 与 rpm 安装包，让习惯包管理器的用户用一条 `apt install` / `dnf install` 完成安装，并用 `systemctl --user enable --now webmux` 设置登录后自启。

## What Changes

- 发布产物新增 Linux 安装包：amd64、arm64 各一个 `.deb` 与 `.rpm`，由 nfpm 在现有 ubuntu 发布任务中生成，计入 `SHA256SUMS`。
- 包内容：`/usr/bin/webmux` 与 systemd 用户单元 `/usr/lib/systemd/user/webmux.service`（以默认配置运行，首次启动自动生成配置与 Token）。安装、升级、卸载都不为任何用户启用或重启服务。
- `webmux service start|stop|restart|status|uninstall` 在没有 `~/.config/systemd/user/webmux.service` 时识别安装包（或管理员）提供的 `webmux.service`：前四个直接作用于该单元，`uninstall` 只禁用并停止它，不删除包内文件。
- 新增 CI 作业：构建安装包并在 Debian、Fedora 容器中实际安装、检查、卸载。
- macOS 不在范围内，仍使用 `install.sh`。

## Capabilities

### New Capabilities

- `linux-packages`: deb/rpm 安装包的内容、安装/升级/卸载行为，以及 `webmux service` 命令与包内用户单元的配合。

### Modified Capabilities

- `release-distribution`: 新增“Linux 安装包”要求（以 ADDED 形式，不改动现有产物清单要求）。

## Impact

- 新文件：`deploy/linux/nfpm.yaml`、`deploy/linux/webmux.service`、`.github/scripts/test-linux-packages.sh`。
- 代码：`internal/service/service.go`（`Control` 识别包内单元）及测试。
- 构建：`Makefile` 新增 `linux-packages` 目标并扩展 `RELEASE_FILES`；`release.yml` 上传安装包；`test.yml` 新增安装包作业。nfpm 以 `go run …@v2.46.3` 调用，不进入 `go.mod`。
- 文档与用户故事：`docs/deployment.md`、README；新增 US-K13。
