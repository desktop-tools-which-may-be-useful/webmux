# 连接与身份

`webmux` 命令经本机 Unix 套接字访问服务，不需要 Token。

## 可执行文件

下文的 `webmux` 指 WebMux 可执行文件，按以下顺序确定：

1. 环境变量 `WEBMUX_BIN`，写作 `"$WEBMUX_BIN"`（带引号以容忍路径中的空格）；它就是正在运行的服务端那个文件，版本一定一致；
2. `{{webmux}}` 简述或本实例信息中给出的绝对路径；
3. `PATH` 中的 `webmux`：可能与服务版本不同，用 `webmux version` 核对。

## 会话中的变量

| 变量                | 含义                                     |
| ------------------- | ---------------------------------------- |
| `WEBMUX_SESSION_ID` | 当前会话 ID；命令中的 `self` 指的就是它  |
| `WEBMUX_SOCKET`     | 本实例本机 API 套接字的绝对路径          |
| `WEBMUX_BIN`        | 服务端正在运行的可执行文件的绝对路径     |

在聊天视图中运行的 agent 进程同样有这三个变量。

## 定位服务

CLI 依次使用 `--socket PATH`、`--config PATH`、`WEBMUX_SOCKET`、默认配置。指定的套接字连不上时以退出码 3 结束，不会改连其他实例。

## 会话内被禁止的操作

会话内的进程不能停止服务、读取 Token、控制 Cloudflare Tunnel 或修改 Tailscale 设置，这些请求返回 403。

## 排错

1. 运行 `"$WEBMUX_BIN" version` 与 `"$WEBMUX_BIN" status`，确认可执行文件与服务都可用；
2. 工具 shell 丢失了 `WEBMUX_*` 变量时，使用简述中的绝对路径，加 `--socket <套接字>`，并把 `self` 换成简述中的会话 ID；
3. 没有可用的可执行文件时，直接调用本机 API：`curl -s --unix-socket <套接字> http://webmux/api/sessions`，手册本身也可以这样读取：`curl -s --unix-socket <套接字> http://webmux/llms.txt`；
4. 处在禁止访问套接字或外部程序的沙箱中时，告诉用户无法控制 WebMux，不要反复重试；
5. 命令提示“服务端版本过旧”时，改用 `"$WEBMUX_BIN"`。
