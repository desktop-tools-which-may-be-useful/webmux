# Tasks

## 1. 防火墙规则命令

- [x] 1.1 在 `internal/tray/firewall.go` 实现规则名称（托盘路径摘要）、netsh 参数、`ParseFirewall` 与 `Firewall.Apply`，规则已是目标状态时不提权。
  - 验证：单元测试覆盖参数解析、名称随路径变化且不区分大小写、只放行专用与域网络、已存在不提权、取消确认返回 3、netsh 失败或规则未变化返回 1。
- [x] 1.2 在 `internal/tray/winui/firewall.go` 用普通权限的 `netsh ... show rule` 查询，用 `ShellExecuteExW` 的 `runas` 运行 `netsh` 并等待退出码；`cmd/webmux-tray` 增加 `firewall` 子命令。
  - 验证：Windows amd64/arm64 交叉 `go vet` 与 golangci-lint 通过。

## 2. 安装程序

- [x] 2.1 `webmux.nsi` 的“安装选项”页增加防火墙选项：首次安装默认勾选，覆盖安装沿用上次的选择，静默安装不勾选。
- [x] 2.2 安装完成后运行 `firewall allow`，成功时在卸载项 `FirewallRule` 记录托盘路径；取消或失败只提示手工命令，不影响安装结果。
- [x] 2.3 覆盖安装取消勾选、或安装目录与记录不同时，运行 `firewall remove <记录的路径>`；卸载在有记录时同样删除，失败时说明手工删除方法。
  - 验证：makensis 3.11 以 `-WX` 构建两种架构。
- [x] 2.4 `windows-installer-smoke.ps1` 检查静默安装不加规则、参数错误返回 2（含 `allow` 带路径、`remove` 带相对路径）、无规则时 `remove` 返回 0；以管理员运行时检查添加、幂等与删除。

## 3. 文档

- [x] 3.1 更新 `docs/deployment/windows-wsl.md` 的安装、卸载、静默安装、子命令与远程访问说明。
- [x] 3.2 更新 US-O08、US-O12，并追加核对记录。

## 4. 验证

- [x] 4.1 Windows 11（build 26300）真机，非管理员会话运行冒烟脚本：全部通过，管理员相关检查按设计跳过。
- [x] 4.2 真机交互安装（记录为路径之前的版本）：保持勾选，UAC 确认后出现 `WebMux tray forwarding 069f8525554f`，入站、允许、TCP、域与专用网络，程序为安装目录中的托盘。
  - 再次运行 `firewall allow` 不弹窗、0.13 秒返回 0；`/S` 静默覆盖安装后规则保留。
- [x] 4.3 真机交互卸载：UAC 确认后规则被删除，安装目录、卸载项与登录启动项一并删除。
- [x] 4.4 真机验证按记录对账：首次交互安装经 UAC 添加规则并记录托盘路径；再次安装选项默认勾选且不弹窗；取消勾选后经 UAC 删除规则并清除记录；之后静默卸载约 1 秒完成，无 UAC。
- [x] 4.5 CI 的 Windows installer smoke 以管理员运行 `firewall allow/remove` 检查通过（运行 41595128147）：添加、规则内容（入站、允许、域与专用网络）、幂等与删除均通过。
