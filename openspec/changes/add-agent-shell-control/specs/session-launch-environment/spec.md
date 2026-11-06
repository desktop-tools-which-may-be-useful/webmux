# Spec Delta

## ADDED Requirements

### Requirement: WebMux 自有的会话变量

服务端 SHALL 在每个会话的环境中设置以下变量，使会话内的程序不依赖 `PATH` 或默认配置就能找到并连接本实例：

- `WEBMUX_SOCKET`：本实例本机 API 套接字的绝对路径。本机 API 不可用时 SHALL NOT 设置。
- `WEBMUX_BIN`：可以执行 `webmux` 子命令的可执行文件的绝对路径。按下文规则解析不到时 SHALL NOT 设置。

注入要求：

- 注入 SHALL 发生在构造会话进程环境的唯一入口：PTY 主进程，以及混合会话聊天端的 ACP agent 等辅助进程，都 SHALL 得到相同的值。
- 两个变量 SHALL 属于服务端自有变量：
  - 创建请求的 `env` 包含它们时，SHALL 以 400 拒绝；
  - 保存包含它们的启动配置时，SHALL 以 400 拒绝；
  - `webmux run` 继承调用方环境时，SHALL 跳过它们。
- 它们 SHALL NOT 出现在会话信息中。
- 服务端 SHALL NOT 为此修改会话的 `PATH`。

`WEBMUX_BIN` SHALL 是服务进程自身的可执行文件路径，即服务启动时 `os.Executable()` 的结果，原样保存在内存中，不另做配置或改写。
`os.Executable()` 返回错误时 SHALL NOT 设置该变量，并记录一条警告日志。

- 会话里执行的就是正在运行的服务端那一个文件，CLI 与服务端因此版本一致。
- 软件包原地升级后，同一路径指向新版本的文件，变量 SHALL 继续有效。
- 以 `go run` 启动时，该路径是 Go 构建出的临时可执行文件；它在服务运行期间一直存在，变量同样有效。

#### Scenario: PATH 中没有 webmux

- **WHEN** 服务以 `/opt/webmux/bin/webmux serve` 启动，会话的 `PATH` 不含 `/opt/webmux/bin`
- **THEN** 会话中 `"$WEBMUX_BIN" ls` 能列出本实例的会话，`PATH` 保持原样

#### Scenario: 聊天端 agent 同样可见

- **WHEN** 混合会话处于聊天端，ACP agent 执行 `echo "$WEBMUX_BIN" "$WEBMUX_SOCKET"`
- **THEN** 输出与同一会话终端端相同的两个绝对路径

#### Scenario: 开发构建

- **WHEN** 服务由 `go run ./cmd/webmux` 启动
- **THEN** 会话中的 `WEBMUX_BIN` 是 Go 构建出的临时可执行文件，`"$WEBMUX_BIN" ls` 在服务运行期间可用

#### Scenario: 请求覆盖被拒绝

- **WHEN** 创建请求的 `env` 包含 `WEBMUX_SOCKET=/tmp/other.sock` 或 `WEBMUX_BIN=/tmp/fake`
- **THEN** 服务端返回 400，不创建会话

#### Scenario: 跨实例继承被跳过

- **WHEN** 在实例 A 的会话里，用户用 `webmux run --config b.toml -- bash` 在实例 B 创建会话
- **THEN** 新会话的 `WEBMUX_SOCKET` 与 `WEBMUX_BIN` 都是实例 B 的值
