## MODIFIED Requirements

### Requirement: 命令定义的单一来源

`webmux` 的每个命令与子命令 SHALL 在一张命令表中声明，表项包括：命令路径、分组、用法、一句话摘要、详细说明、位置参数、选项、
用到的环境变量、输出、退出码、示例，以及相关的 llms 主题与命令。覆盖范围包括现有命令（`serve`、`start`、`version`、`status`、`stop`、`config`、
`service`、`update`、`bridge`、`run`、`attach`、`ls`、`open`）和本变更新增的全部命令。

- 每个选项 SHALL 声明：长名、可选短名、值类型、默认值、是否可重复、说明，以及互斥或依赖关系。
- 会话类命令的选项只能经命令表声明（`Bind`），解析与帮助共用同一份声明。
- 运维命令（`serve`、`config`、`service`、`update` 等）保留各自的解析器；它们的 `--help` 由分发层统一按命令表输出，测试 SHALL 比对每个运维命令实现接受的选项与命令表一致。
- 除 `run` 等以其他程序 argv 结尾的命令外，选项 SHALL 可以写在位置参数之后；`--` 之后的参数 SHALL 原样作为位置参数。
- `--help` 输出、`help --json`、shell 补全，以及 llms 的命令目录（`/llms/commands.md`），SHALL 全部由这张表生成或经测试比对，不维护第二份手写副本。
  命令目录 SHALL 在运行时渲染，仓库中 SHALL NOT 保存生成的副本（见 `agent-shell-guide` 的“手册与命令帮助的分工”）。
- 共享命令放在 `internal/sessioncmd`，Linux/macOS CLI 与 Windows 控制台客户端 SHALL 得到相同的帮助。
  只属于某个平台的命令（例如 `service`）SHALL 只出现在该平台的帮助中。

#### Scenario: 新增选项忘记写说明

- **WHEN** 开发者给 `send` 新增一个选项，但没有写说明或默认值
- **THEN** 命令表一致性测试失败，并指出缺少的字段

### Requirement: 完整的 --help

每个命令与子命令 SHALL 提供 `--help`，内容依次包括以下各节，某节没有内容时省略：

1. **Usage**：完整用法，标出可选与可重复的部分；
2. **摘要与说明**：做什么，以及与相近命令的区别；
3. **Arguments**：每个位置参数的含义，包括 `self` 与仓库参数的写法；
4. **Options**：每个选项的类型、默认值与说明，互斥关系，以及全局选项 `--config` 和 `--socket`；
5. **Environment**：读取的环境变量（`WEBMUX_SOCKET`、`WEBMUX_SESSION_ID`、`WEBMUX_BIN` 等）；
6. **Output**：文本与 `--json` 的输出形式，以及对应的契约类型名；流式命令说明 NDJSON；
7. **Exit status**：该命令可能返回的退出码及含义；
8. **Examples**：至少两个示例，一个基本用法，一个脚本或 agent 用法；
9. **See also**：相关命令，以及 `webmux guide <主题>`。

帮助正文 SHALL 使用英文，与现有 CLI 一致；示例中的提示文字可以是中文。帮助 SHALL 按 80 列换行。
说明中用到手册已完整说明的概念（会话状态的含义、`self`、临时文件夹）时，SHALL 只给一句话定义，并在 See also 中给出对应的 `webmux guide <主题>`，SHALL NOT 复述手册的完整说明。

#### Scenario: 查看子命令帮助

- **WHEN** 用户执行 `webmux send --help`
- **THEN** 输出包含用法、全部选项及默认值、`--reply` 的退出码含义、至少两个示例，以及 `webmux guide sessions`

### Requirement: shell 补全

`webmux completion bash|zsh|fish` SHALL 把对应 shell 的补全脚本写到标准输出，帮助中 SHALL 给出安装方法。

- 静态补全：命令、子命令、选项名，以及枚举值（状态、视图、`--kind`、llms 主题、shell 名）。
- 动态补全由脚本调用隐藏命令 `webmux __complete <words...>` 完成：
  - 补全会话 ID（zsh 与 fish 附带标题作为说明；`resume` 与 `rm --forget` 补全已归档会话的 ID）、启动配置名称、仓库、worktree 与转发端口；
  - 经本机 API 查询，超时 1 秒；服务未运行或超时时，只返回静态候选，SHALL NOT 输出错误；
  - SHALL NOT 输出 Token 或环境变量的值。
- `__complete` SHALL NOT 出现在帮助与 `help --json` 中。
- Windows 控制台客户端 SHALL 提供 `--help` 与 `help --json`，本变更不为 PowerShell 提供补全。

#### Scenario: 补全会话 ID

- **WHEN** 用户在 zsh 中输入 `webmux send ` 后按 Tab
- **THEN** 列出当前会话 ID，每项附带会话标题

#### Scenario: 服务未运行

- **WHEN** 服务未运行时，用户在 bash 中输入 `webmux chat s` 后按 Tab
- **THEN** 补全出 `set`、`status`，没有报错输出
