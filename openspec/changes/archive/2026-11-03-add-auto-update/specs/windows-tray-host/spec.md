# Spec Delta

## MODIFIED Requirements

### Requirement: 外部退出请求

托盘程序 SHALL 提供 `quit` 子命令，请求当前 Windows 用户正在运行的托盘退出。收到请求的托盘 SHALL 按“仅退出托盘”处理且 SHALL NOT 弹出询问：WebMux 与保持发行版存活的进程继续运行，托盘转发停止。`quit` SHALL NOT 创建托盘图标，也 SHALL NOT 影响其他 Windows 用户的托盘。

`quit` SHALL 最多等待 10 秒，并以退出码区分结果：0 表示运行中的托盘已退出；1 表示仍有托盘在运行；2 表示参数无效；3 表示没有运行中的托盘。安装程序据此判断是否关闭过托盘。指定 `--kill-path <路径>` 时，等待超时后 `quit` SHALL 只结束当前用户从该路径运行的托盘进程，再按结果返回退出码。

#### Scenario: 请求退出

- **WHEN** 托盘管理的 WebMux 正在运行且有 2 个会话，用户执行 `webmux-tray.exe quit`
- **THEN** 托盘图标消失且不弹出询问，命令以退出码 0 结束；2 个会话继续可用，再次启动托盘后显示为运行中

#### Scenario: 没有运行中的托盘

- **WHEN** 当前用户没有运行托盘，用户执行 `webmux-tray.exe quit`
- **THEN** 命令立即以退出码 3 结束，不出现托盘图标

#### Scenario: 旧版托盘不响应

- **WHEN** 当前用户运行着不支持退出请求的旧版托盘，用户执行 `webmux-tray.exe quit`
- **THEN** 命令约 10 秒后以退出码 1 结束，旧托盘保持运行

#### Scenario: 超时后结束指定路径

- **WHEN** 从 `C:\Users\u\AppData\Local\Programs\WebMux\webmux-tray.exe` 运行的托盘 10 秒内未响应，执行的是 `quit --kill-path` 加该路径
- **THEN** 该进程被结束，命令以退出码 0 结束；从其他路径运行的托盘不受影响
