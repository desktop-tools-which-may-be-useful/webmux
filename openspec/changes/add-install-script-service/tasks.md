# Tasks

## 1. install.sh

- [x] 1.1 解析 `WEBMUX_SERVICE`（`1`/`0`/未设置），无效值在下载前报错；验证：`make test-install` 覆盖无效值且安装目录不变
- [x] 1.2 未设置时按 D2 从终端询问（默认是），无终端时不安装并保留原提示；验证：`make test-install` 覆盖回车、输入 n 与无终端三种情况
- [x] 1.3 选择安装时按 D3 检测已有服务，未安装则运行 `service install` 并输出状态、日志与 Linux linger 提示；已有服务时只提示 restart，指向其他路径时提示 `--force`；验证：`make test-install` 用记录参数的替身二进制断言调用与输出
- [x] 1.4 `service install` 失败时保留二进制、输出重试命令并以退出码 1 结束；验证：`make test-install` 覆盖失败路径

## 2. 文档与用户故事

- [x] 2.1 更新 `docs/deployment.md` 安装章节的变量表与说明；验证：`make docs-check` 与格式检查通过
- [x] 2.2 更新 US-K11 验收要点、覆盖索引与 `docs/user-stories.md` 核对记录；验证：故事 ID 唯一、链接有效

## 3. 集成检查

- [x] 3.1 在真实 Linux（systemd 用户管理器）上用本地发布产物运行 `WEBMUX_SERVICE=1` 安装、再次运行验证已有服务不被重启，然后卸载；验证：`systemctl --user is-active webmux` 与服务 PID 不变
- [x] 3.2 `make test-install`、`make check`、`openspec validate add-install-script-service --strict` 通过
