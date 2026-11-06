# 用脚本和 agent 控制 WebMux

`webmux` 命令行可以完成 Web 端的大部分操作，脚本和运行在 WebMux 会话里的 CLI agent 都能直接调用。本页先讲怎样连上正确的实例，再按用途介绍命令。每个命令都有完整的 `--help`，`webmux help --json` 输出机器可读的命令树。

## 连接到正确的实例

命令经本机 Unix 套接字访问服务，不需要 Token。CLI 按以下顺序找到服务：

1. `--socket PATH`：直接连接这个套接字；
2. `--config PATH`：按配置文件定位，规则与服务相同；
3. 环境变量 `WEBMUX_SOCKET`：WebMux 为每个会话设置，指向启动它的实例；
4. 默认配置 `~/.config/webmux/config.toml`。

`--socket` 或 `WEBMUX_SOCKET` 指定的套接字连不上时，命令以退出码 3 结束，不会改连其他实例。

每个会话（包括混合会话聊天端的 agent 进程）还会得到：

| 变量                | 含义                                                      |
| ------------------- | --------------------------------------------------------- |
| `WEBMUX_SESSION_ID` | 当前会话 ID；命令中的 `self` 指的就是它                   |
| `WEBMUX_SOCKET`     | 本实例本机 API 套接字的绝对路径                           |
| `WEBMUX_BIN`        | 服务端正在运行的可执行文件；`PATH` 里没有 `webmux` 时用它 |

`PATH` 中的 `webmux` 可能比服务旧或新。用 `"$WEBMUX_BIN"` 运行的命令总是与服务同一版本；旧版 CLI 调用新接口时会提示“服务端版本过旧”并给出两边的版本。

## 查看会话元数据

```sh
webmux get                               # 在会话内查看当前会话：仓库、分支、启动配置、混合状态、链接、父子会话与标签
webmux get abcd2345 --json               # 另一个会话的完整详情（SessionDetails）
branch=$(webmux get --field worktree.branch)   # 不用 jq 取单个字段
```

详情不包含 Token 或启动环境变量的值；临时文件夹只在已经存在时报告，`get` 不会创建它。

## 父会话与标签

从会话内经本机 API 创建的会话会自动记录 `parentId`，即调用者所在的会话。服务端按连接进程判断，请求体无法指定。

```sh
webmux label self task=lint run=42       # 设置标签；KEY- 删除
webmux ls --parent self                  # 我创建的会话
webmux ls --label run=42 --status exited --json   # 按标签和状态筛选
```

标签键匹配 `[a-z0-9][a-z0-9._-]{0,62}`，值最多 256 个字符，每个会话最多 16 个。父会话和标签只保存在内存中，随会话结束而消失。

## 启动会话

```sh
webmux run --detach --json --wait idle --timeout 60s --launch-config omp   # 等 agent 就绪
webmux run --follow --strip -- go test ./...       # 在持久会话里跑命令，输出纯文本，以命令的退出码退出
webmux run --new-worktree fix/lint --launch-config "Claude Code" --start-side chat --init --prompt "修复 lint"
webmux chat new --agent Codex --wait "修复所有 lint 并汇报"   # 一条命令完成快速开始，等第一轮结束并输出回复
webmux launch-config options "Claude Code"         # 列出 chat new --set 可用的配置项
```

- 没有终端时 `run` 自动后台运行；`--attach` 强制附着。
- 当前目录所在的已登记 worktree 会自动关联；`--new-worktree` 新建的 worktree 像 Web 端一样加入主页显示（`--no-pin` 不加入）。
- `--follow` 时 Ctrl+C 只停止跟随（130），会话继续运行；`--kill-on-interrupt` 先结束程序。
- `--dry-run` 打印将要发送的请求，环境变量值显示为 `<inherited>` 或 `<explicit>`。
- `chat new` 失败时按步骤报告，已创建的 worktree 与会话保留；`--wait` 遇到授权请求时以 1 退出，`--auto-wait-requests` 继续等待。

## 输入、等待与读取输出

```sh
webmux send <ID> 'make test' --enter --wait idle --timeout 10m   # 输入并回车，等会话回到空闲
webmux send <ID> --key esc                       # 发送按键
webmux wait <ID> --until attention,idle --changed --timeout 30m
webmux logs <ID> --strip --tail 50               # 最近 50 行纯文本；--follow 持续输出
webmux screen <ID>                               # 全屏 TUI 当前画面的文本
webmux events --session <ID>                     # 会话事件，NDJSON
webmux kill <ID>; webmux rm <ID>; webmux rename <ID> 新名称
webmux api /api/launch-configs                   # 没有专用命令的接口，原样输出响应
```

- `send` 在服务端确认输入写入 PTY 后才以 0 退出，并像浏览器输入一样取得控制权；控制指示器显示 `script`（在会话内还附上来源会话 ID）。
- `send --wait` 先订阅再输入，只接受之后的状态变化；`wait --changed` 同理，`--stable` 要求状态保持一段时间。
- `logs --strip` 适合按行输出的程序；全屏 TUI 用 `screen`，画面在 CLI 本地渲染，服务端不做终端模拟。
- `api` 只接受 `/api/` 与 `/local/v1/` 路径，请求体须为 JSON；WebSocket 路径请用 `events`、`logs`、`screen`。

## 提醒用户

```sh
make release; webmux notify --kind done "release 构建完成"
webmux notify --clear --session <ID>
```

- 会话显示提醒徽章，打开的浏览器弹出提示（消息作为说明行），订阅推送的设备收到以消息为正文的通知（`include_title = false` 时用固定正文）。
- `--kind` 取 `notify`（默认）、`done`、`error`、`question`；消息最多 200 个字符，只保存在内存中，服务重启后消失。
- 提醒来源记为 `cli`，覆盖当前提醒；之后按原有规则清除。`--clear` 与 `send` 的输入都和用户输入一样清除提醒。

## 驱动混合会话聊天

```sh
webmux chat send <ID> "修复 lint 并汇报改动" --wait --timeout 20m   # 输出 agent 的最终回复
webmux chat status <ID>                          # 回合、队列、待回答请求与选项、agent 设置
webmux chat answer <ID> <请求ID> --option allow_once
webmux chat watch <ID>                           # 持续输出聊天；--json 每行一个帧
webmux chat switch <ID> --to terminal --force
webmux chat set <ID> model=sonnet; webmux chat permission <ID> ask
webmux chat queue ls <ID>                        # rm、edit、send-now、clear、resume
webmux chat ls --archived; webmux chat resume <记录ID> --side chat --detach; webmux chat forget <记录ID> --yes
```

- 这些命令使用与 Web 端聊天视图相同的接口，结果同步到所有设备；目标不是混合会话、或需要聊天端却在终端端时以 1 结束。
- `send --wait` 先订阅再发送，跟踪这条提示自己的回合：排队的提示等到被发送；`end_turn` 以 0 退出，其他结束原因以 1 退出。
  遇到授权或提问时输出请求 ID 与选项并以 1 退出，`--auto-wait-requests` 继续等待；超时 124，不取消回合。
- `switch` 等到切换结束；终端 TUI 忙时需要 `--force`，否则取消切换并以 1 退出。找不到 agent 会话时列出候选，用 `--agent-session` 重试。
- `forget` 在终端上先询问，非交互时需要 `--yes`；进行中的记录不能删除。

## 仓库、worktree、启动配置与转发

```sh
webmux worktree create . --branch fix/lint --base main   # 输出路径，并像 Web 端一样加入主页显示（--no-pin 不加入）
webmux worktree ls . --hidden                    # HOME 列：pinned、sessions、hidden
webmux worktree pin . exp/a; webmux worktree home . --reset
webmux worktree rm . fix/lint --yes              # 先预览；有阻止原因时需要 --force
webmux repo add; webmux repo branches .; webmux repo commits . --from main; webmux repo base . main
printf '{{webmux}}\n\n{{user}}' | webmux repo settings . --init -
webmux launch-config add "Dev" --icon terminal --env MODE=dev -- bash -lc 'npm run dev'
webmux launch-config edit "Dev" --unset-env MODE; webmux launch-config show "Dev" --json
webmux forward add 5173 --session self --label web
```

- `REPO` 是仓库 ID、仓库内路径或 `.`；worktree 用 ID、分支名或路径。找不到时以 2 结束并列出候选。
- 主页显示的规则与 Web 端相同：从未设置的仓库只显示主工作区，有会话或归档记录的 worktree 总会显示。
  写入只改该仓库的 `homeWorktrees` 条目，与其他设备冲突（409）时重新读取并重放，最多 3 次。
- `worktree rm`、`repo rm`、`launch-config rm` 与 `chat forget` 一样：终端上先确认，非交互时需要 `--yes`。`repo rm` 不删除文件，并清理该仓库的主页显示设置。
- 初始化指令最多 4000 个字符，超出时 CLI 以 2 拒绝；引用不存在的快捷指令只警告，仍会保存。`--edit` 用 `$VISUAL` 或 `$EDITOR` 编辑。
- 启动配置的变量值默认显示为 `<hidden>`，`--reveal-env` 才输出；`edit` 读取后只改给出的字段再整体保存；`--hybrid` 先检查是否支持。

## shell 补全

```sh
echo 'source <(webmux completion bash)' >> ~/.bashrc
webmux completion zsh > ~/.zfunc/_webmux
webmux completion fish > ~/.config/fish/completions/webmux.fish
```

补全命令、选项与枚举值，并向运行中的服务查询会话 ID、启动配置、仓库与 worktree；服务未运行或 1 秒内没有响应时只给出静态候选。

## WebMux 工具手册

手册按 llms.txt 约定放在站点根路径，内容与运行中的服务版本一致：

```sh
webmux guide                     # 索引（/llms.txt），列出各主题
webmux guide artifacts           # 单个主题（/llms/artifacts.md）
webmux guide --full              # 全部主题（/llms-full.txt）
webmux guide --path              # 服务端可执行文件与套接字路径
curl -s http://127.0.0.1:7681/llms.txt   # 也可以直接读 HTTP 地址，不需要 Token
```

手册不写死安装路径：示例中的 `webmux` 指 `"$WEBMUX_BIN"`、指令中给出的绝对路径或 `PATH` 中的 `webmux`。经 TCP 读取的版本不含任何实例信息，经本机套接字读取时附带本实例的版本、可执行文件与套接字。

在快捷指令或仓库初始化指令里写 `{{webmux}}`，会插入一段说明：会话 ID、命令与套接字路径、手册地址，以及本会话临时文件夹的路径和“交付文件放这里而不是项目仓库”的规则。系统预设“WebMux 工具”就是一条正文为 `{{webmux}}` 的快捷指令。`webmux dir self` 输出临时文件夹路径。

## 帮助与退出码

- `webmux <命令> --help` 写到标准输出并以 0 退出，包括用法、参数、选项与默认值、环境变量、输出、退出码和示例。
- 用法错误以 2 退出，标准错误给出原因和 `Run 'webmux <命令> --help' for details.`。
- 选项可以写在位置参数之后（`webmux get self --json`）；`run` 在 `--` 之后的参数原样作为命令。
- 退出码：0 成功；1 操作失败；2 用法错误；3 服务未运行；124 等待超时。
