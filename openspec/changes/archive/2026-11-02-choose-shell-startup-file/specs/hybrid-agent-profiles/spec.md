## MODIFIED Requirements

### Requirement: 内置档案与可识别的命令写法

系统 SHALL 内置 OpenCode、omp 与 Codex 三个档案，第一期 SHALL NOT 开放用户自定义档案。启动配置的 argv 只有在以下两种写法之一中，且可执行文件的基本名与某个档案匹配时，才 SHALL 视为可用于混合模式：

1. 直接执行：`[<可执行文件>, 参数…]`；
2. webmux 的启动文件包装：`["bash", "-ic", "exec -- \"$@\"", "webmux", <可执行文件>, 参数…]` 或 `["zsh", "-ic", "exec -- \"$@\"", "webmux", <可执行文件>, 参数…]`。

其他任何写法（例如 fish、nu 或其他 `sh -c` 包装、`env` 前缀）SHALL 视为不可用，并给出原因“命令写法不受支持”。

#### Scenario: 预置的 Bash 包装

- **WHEN** 配置的 argv 为 `["bash","-ic","exec -- \"$@\"","webmux","omp"]`
- **THEN** 判定为可用，档案为 omp

#### Scenario: 预置的 Zsh 包装

- **WHEN** 配置的 argv 为 `["zsh","-ic","exec -- \"$@\"","webmux","omp","--auto-approve"]`
- **THEN** 判定为可用，档案为 omp，后续恢复与 ACP 启动继续使用 Zsh

#### Scenario: 自定义包装

- **WHEN** 配置的 argv 为 `["sh","-c","omp"]` 或 `["fish","-c","omp"]`
- **THEN** 判定为不可用，原因为命令写法不受支持

### Requirement: 恢复命令的生成

切换或恢复到 TUI 时，系统 SHALL 按以下方式生成恢复命令：

- 由可执行文件、原 argv 中位于档案白名单内的参数、档案的恢复参数（OpenCode 为 `-s <id>`，omp 为 `--resume <id>`）组成，并套用与原命令相同的写法（直接执行，或使用原 shell 的启动文件包装）。
- 白名单参数 SHALL 同时识别 `--flag value` 与 `--flag=value` 两种形式，并按档案声明的参数个数解析。
- 系统 SHALL 丢弃以下参数：
  - omp 的位置参数（提示词）；
  - 提示词参数（例如 OpenCode 的 `--prompt`）；
  - 会话类参数（`-c`、`--continue`、`-r`、`--resume`、`--restore`、`-s`、`--session`、`--fork` 等）；
  - 白名单之外的参数。
- 被丢弃的参数 SHALL 在切换结果中列出，界面 SHALL 提示“以下参数在恢复时未保留”。

#### Scenario: 丢弃提示词与会话参数

- **WHEN** 原 argv 为 `["omp","--model","haiku","--continue","修 bug"]`，agent 会话 ID 为 `01abc`
- **THEN** 恢复命令为 `["omp","--model","haiku","--resume","01abc"]`，界面列出未保留的 `--continue` 与 `修 bug`

#### Scenario: Zsh 包装保持同一 shell

- **WHEN** 原 argv 为 `["zsh","-ic","exec -- \"$@\"","webmux","omp","--auto-approve"]`，agent 会话 ID 为 `01abc`
- **THEN** 恢复命令为 `["zsh","-ic","exec -- \"$@\"","webmux","omp","--auto-approve","--resume","01abc"]`

### Requirement: ACP 启动命令

系统 SHALL 按档案启动 ACP 进程：OpenCode 为 `opencode acp`，omp 为 `omp acp`，并套用与原命令相同的写法。

ACP 进程的参数透传按 agent 区分：

- omp：`omp acp` 解析完整 argv，并静默忽略未知参数。因此原 argv 中位于 omp 白名单内的参数 SHALL 全部透传给 ACP 进程（例如 `--model`、`--thinking`、`--profile`、`--config`、`--approval-mode`、`--auto-approve`），使两端使用相同的模型、配置档与批准策略。
- OpenCode：`opencode acp` 只接受 `--login`，因此 OpenCode 的白名单参数 SHALL NOT 透传。`--auto` 也 SHALL NOT 在聊天视图提示不支持，因为聊天端的工具批准由聊天视图的权限模式决定（默认自动批准）。

使用启动文件包装时，所选 shell 的启动文件（Bash 的 `~/.bashrc` 或 Zsh 的 `~/.zshrc`）执行期间写到 stdout 的内容 SHALL NOT 进入 ACP 协议通道；ACP 协议 SHALL 继续写到 fd 3。

#### Scenario: Bash 启动文件有输出

- **WHEN** 用户的 `.bashrc` 会 `echo` 一行欢迎语，混合会话从聊天开始
- **THEN** ACP 握手成功，欢迎语不导致协议错误

#### Scenario: Zsh 启动文件有输出

- **WHEN** 用户的 `.zshrc` 会 `echo` 一行欢迎语，且配置使用 `zsh -ic` 包装，混合会话从聊天开始
- **THEN** ACP 进程以同一 Zsh 包装启动，欢迎语留在 stdout/stderr，握手与协议通道不受影响

## ADDED Requirements

### Requirement: 连接器检查跟随启动文件 shell

混合会话在启动 ACP 进程前检查档案声明的第三方连接器时，若配置使用启动文件包装，SHALL 使用同一 shell（Bash 或 Zsh）执行 `command -v`。启动文件的输出 SHALL 先于结果标记；超时、输出上限、结果解析与“启动前以 127 退出表示连接器缺失”的判定 SHALL 与既有规则一致。

#### Scenario: Zsh 下连接器已安装

- **WHEN** 配置使用 `zsh -ic` 包装且 `~/.zshrc` 设置了包含 `codex-acp` 的 PATH
- **THEN** 连接器检查经同一 Zsh 包装找到 `codex-acp`，聊天端启动

#### Scenario: Zsh 启动文件有输出

- **WHEN** `~/.zshrc` 输出多行提示后以 127 退出且未找到连接器
- **THEN** 检查结果仍不含启动文件输出，并判定为连接器缺失
