# 启动会话

## run

```sh
webmux run --detach -- make test                          # 后台运行，输出 "<ID>\t<链接>"
webmux run --detach --json --wait idle --timeout 60s --launch-config omp   # 等 agent 就绪，输出 {session, link}
webmux run --follow --strip -- go test ./...              # 在持久会话里跑命令，输出纯文本，以命令的退出码退出
webmux run --new-worktree fix/lint --base main --launch-config "Claude Code" --start-side chat --init --prompt "修复 lint" --detach
webmux run --dry-run --label run=7 -- make build          # 只打印将要发送的请求，不创建任何东西
```

- `--` 之后的参数原样作为 argv，不经 shell 展开；需要 shell 特性时显式写 `bash -lc '...'`。
- 自定义命令继承调用方的环境（终端身份变量与 `WEBMUX_*` 除外），`--env` 覆盖同名变量；`--launch-config` 的环境由服务端展开，与 `--` 后的命令、`--env` 互斥。
- 没有终端时（agent 的工具调用、管道）`run` 自动后台运行；`--attach` 强制附着。
- 位置：当前目录所在的已登记 worktree 会自动关联；`--worktree REPO:WORKTREE|PATH` 选已有 worktree；`--new-worktree BRANCH` 新建 worktree（默认放在 `<仓库>.worktrees/<分支末段>`，并像 Web 端一样加入主页显示，`--no-pin` 不加入）。
- 混合会话：`--start-side terminal|chat`，`--prompt TEXT|-` 作为首条消息，`--init` 在前面加上仓库初始化指令。
- `--follow`：Ctrl+C 只停止跟随（退出码 130），会话继续运行；`--kill-on-interrupt` 先结束程序。连接中断时以 1 退出，用 `webmux attach` 重新查看。
- `--label KEY=VALUE` 给会话打标签，便于之后 `webmux ls --label` 找回。

## chat new

`webmux chat new` 等同于 Web 端“新聊天”：一次完成新建 worktree、启动混合 agent、应用配置项与权限模式、发送首条消息。

```sh
webmux chat new --agent "Claude Code" "Add a POST /books endpoint"
webmux chat new --agent Codex --label task=lint --wait --timeout 20m "修复所有 lint 并汇报改动"
webmux launch-config options "Claude Code" --json        # 列出 --set 可用的配置项与值
webmux chat new --agent omp --set model=smart --permission ask --attach design.png -
```

- 仓库默认是当前目录所属的已登记仓库；分支名默认自动生成（`wt/<日期>-<时间>`）。
- `--wait` 等第一轮结束，输出 agent 的最终回复；回合中 agent 请求授权或提问时以 1 退出并在标准错误给出请求 ID，`--auto-wait-requests` 则继续等待用户在 Web 端回答。
- 失败时按步骤（precheck、worktree、session、config、prompt）报告，已创建的 worktree 与会话保留。

启动配置名称可以在 Web 端设置页的“启动配置”中查看。
