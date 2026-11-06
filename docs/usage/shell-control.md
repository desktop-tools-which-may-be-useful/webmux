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

## 帮助与退出码

- `webmux <命令> --help` 写到标准输出并以 0 退出，包括用法、参数、选项与默认值、环境变量、输出、退出码和示例。
- 用法错误以 2 退出，标准错误给出原因和 `Run 'webmux <命令> --help' for details.`。
- 选项可以写在位置参数之后（`webmux get self --json`）；`run` 在 `--` 之后的参数原样作为命令。
- 退出码：0 成功；1 操作失败；2 用法错误；3 服务未运行；124 等待超时。
