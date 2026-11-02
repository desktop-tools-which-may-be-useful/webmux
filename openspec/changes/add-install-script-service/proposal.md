# Proposal

## Why

Linux 与 macOS 用户用 `install.sh` 安装后，还要另外执行 `webmux service install` 才能让 WebMux 后台运行并在登录后自动启动；Windows 安装程序已在安装时提供“开机后自动启动”选项。让 `install.sh` 在安装二进制后可选地完成用户服务安装，使三个平台的安装体验一致。

## What Changes

- `install.sh` 新增环境变量 `WEBMUX_SERVICE`：`1` 在安装二进制后执行 `webmux service install`（systemd 用户服务 / launchd 用户代理，登录后自动启动），`0` 只安装二进制。
- 未设置 `WEBMUX_SERVICE` 且可以访问终端时，脚本询问是否安装用户服务，默认是；无法交互（如 CI）时保持现有行为，只安装二进制并提示命令。
- 已存在用户服务时不改写、不重启（重启会结束会话），只提示升级后用 `webmux service restart` 生效；服务指向其他可执行文件时提示用 `service install --force` 切换。
- 服务安装失败时保留已安装的二进制，说明原因并以非零退出码结束。
- 不改变下载、校验、镜像与安装目录的现有行为。

## Capabilities

### New Capabilities

- `install-script-service`: `install.sh` 在安装二进制后是否、以及如何安装用户服务（选择方式、已有服务、失败处理与提示）。

### Modified Capabilities

## Impact

- `install.sh`、`.github/scripts/test-install.sh`（`make test-install`，CI lint 作业运行）。
- 文档：`docs/deployment.md` 安装章节；用户故事 US-K11（一键安装脚本）。
- 依赖已发布二进制的 `webmux service install`（含 `--dry-run`）行为，不修改 Go 代码。
