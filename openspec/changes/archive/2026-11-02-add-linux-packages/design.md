# Design

## Context

动机见 proposal.md。现状：

- `make release` 在 ubuntu 任务中交叉编译全部二进制，再生成 `SHA256SUMS`；`release.yml` 列出上传文件。
- `webmux service install` 写入 `~/.config/systemd/user/webmux.service`；`Control` 以该文件存在为前提。
- 托盘以 `systemctl --user cat webmux.service` 判断单元是否存在，包内单元对它天然可见。

## Goals / Non-Goals

**Goals:** 一份配置同时产出 deb 与 rpm；包内单元可直接 `systemctl --user enable --now webmux`；`webmux service` 命令与包内单元一致。

**Non-Goals:** apt/yum 软件源与签名、macOS 包、包安装脚本（postinst 等）、系统级（root）服务。

## Decisions

### D1：nfpm 打包

`deploy/linux/nfpm.yaml` 用环境变量 `VERSION`、`ARCH` 展开；nfpm 把 `amd64`/`arm64` 映射成 deb 与 rpm 各自的架构名。

- 以 `go run github.com/goreleaser/nfpm/v2/cmd/nfpm@v2.46.3` 固定版本调用，不写入 `go.mod`，也不新增 CI 安装步骤。
- 版本选 v2.46.3（要求 Go 1.25）：v2.47 起要求 Go 1.26.4，而 CI 按 `go.mod` 使用 Go 1.26.0 且 `GOTOOLCHAIN=local`。

备选：`dpkg-deb` 与 `rpmbuild` 各写一份元数据——两套格式各自维护，且 ubuntu runner 需额外安装 rpm 工具。

### D2：不带维护脚本

包内不放 postinst/prerm：启用或重启用户服务需要进入每个用户的 systemd 管理器，而且重启会结束会话。用户自行 `systemctl --user daemon-reload` / `enable --now`，文档说明。

### D3：`Control` 识别包内单元

`Control` 先看用户定义文件；不存在时在 Linux 上运行 `systemctl --user cat webmux.service`（输出丢弃），成功即视为包内（或管理员放在 `/etc/systemd/user` 等目录的）单元，判定方式与托盘一致。

- `start|stop|restart|status` 照常调用 `systemctl --user`。
- `uninstall` 只执行 `disable --now`，不删除文件、不 `daemon-reload`，输出说明需卸载软件包。

### D4：验证

- Go 单元测试覆盖包内单元的 status/uninstall 与“没有任何单元”的错误。
- `.github/scripts/test-linux-packages.sh` 在 `debian:stable-slim` 与 `fedora:latest` 容器中分别安装 deb / rpm（runner 上没有同时可用的 dpkg 与 rpm），检查：无依赖、无维护脚本、只有两项文件、权限、单元的 `ExecStart` 与 `webmux version`，再卸载并确认文件被移除。
- `test.yml` 新增作业用占位二进制构建安装包后运行该脚本；`release.yml` 的产物校验对真实安装包运行同一脚本。

## Risks / Trade-offs

- [`go run …@version` 首次需下载 nfpm 及依赖] → 构建时受 Go 模块代理可用性影响；发布任务已依赖 Go 模块下载。
- [用户同时有 `service install` 的定义与包内单元] → systemd 以 `~/.config` 中的定义为准；`webmux service` 也优先使用它，两者一致。
