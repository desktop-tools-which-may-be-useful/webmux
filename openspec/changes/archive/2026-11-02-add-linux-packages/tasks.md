# Tasks

## 1. 服务命令识别包内单元

- [x] 1.1 `internal/service.Control` 按 D3 识别包内单元，`uninstall` 只禁用并停止；验证：新增 Go 测试覆盖 status、uninstall 与无单元错误，`go test -race ./internal/service` 通过
- [x] 1.2 在本机 systemd 用户管理器上把单元放到非 `~/.config` 的用户单元目录，实际执行 `webmux service status|stop|start|uninstall`；验证：命令作用于该单元，uninstall 后文件仍在

## 2. 安装包

- [x] 2.1 新增 `deploy/linux/webmux.service` 与 `deploy/linux/nfpm.yaml`；验证：`make linux-packages` 产出四个安装包
- [x] 2.2 `Makefile` 新增 `linux-packages` 目标并纳入 `release`/`RELEASE_FILES`；`release.yml` 上传安装包；验证：`make release VERSION=0.0.1` 后 `sha256sum -c SHA256SUMS` 通过
- [x] 2.3 新增 `.github/scripts/test-linux-packages.sh` 与 `test.yml` 作业；验证：本机用 Docker（经 `DEBIAN_IMAGE`/`FEDORA_IMAGE` 指向镜像源）运行脚本，Debian 与 Fedora 安装、检查、卸载均通过

## 3. 文档与用户故事

- [x] 3.1 更新 `docs/deployment.md`、README 的安装说明；验证：`make docs-check` 与格式检查通过
- [x] 3.2 新增 US-K13 并同步覆盖索引与核对记录；验证：故事 ID 唯一、链接有效

## 4. 集成检查

- [x] 4.1 `make check`、`go test -race ./internal/service ./cmd/webmux`、`openspec validate add-linux-packages --strict` 通过
