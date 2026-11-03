## Why

托盘转发第一次在非回环地址上监听时，Windows Defender 防火墙才弹出“允许访问”提示。这个时间点不可预期：可能出现在用户刚勾选转发时，也可能出现在托盘自更新、重启之后。直接关掉对话框时 Windows 会建立阻止规则，之后转发被静默拦截，托盘也看不到原因。

用户希望在第一次安装时就完成这次授权，之后不再被打断。

## What Changes

- 安装程序“安装选项”页增加“允许其他设备通过托盘转发访问 WebMux（添加 Windows 防火墙规则）”，交互安装默认勾选。
- 勾选时，安装完成后弹出一次管理员确认，为安装目录中的 `webmux-tray.exe` 添加入站 TCP 允许规则，只放行专用网络与域网络，不放行公用网络。
- 规则名称含托盘路径的摘要，静默自更新在原路径覆盖时规则继续有效，不再弹窗；不同安装目录互不影响。
- 卸载时删除安装程序添加的规则（再弹一次管理员确认）。
- 托盘新增子命令 `webmux-tray firewall allow|remove`，供安装程序调用，也可手工重试；规则已是目标状态时不请求管理员权限。
- 静默安装（`/S`）从不弹出管理员确认，也不改动已有规则。
- 托盘运行时本身仍不修改防火墙。

## Capabilities

### New Capabilities

（无）

### Modified Capabilities

- `windows-installer`：安装除可选防火墙规则外仍无需管理员；新增防火墙规则选项；卸载删除该规则。
- `windows-tray-host`：新增 `firewall` 子命令。
- `windows-tray-forwarding`：说明防火墙放行由安装程序可选完成，托盘运行时不修改防火墙。

## Impact

- `deploy/windows/webmux.nsi`：安装选项页、安装后调用 `firewall allow` 并在卸载项记录 `FirewallRule`、卸载时调用 `firewall remove`。
- `cmd/webmux-tray`、`internal/tray/firewall.go`、`internal/tray/winui/firewall.go`：子命令、规则名称与 netsh 参数、以 `runas` 提权运行 netsh。
- `.github/scripts/windows-installer-smoke.ps1`：静默安装无规则、参数校验；以管理员运行时验证添加与删除。
- 文档：`docs/deployment/windows-wsl.md`、用户故事 US-O08、US-O12。
