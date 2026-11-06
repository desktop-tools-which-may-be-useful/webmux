# 会话

会话是服务端持有的 PTY 与进程。浏览器断开不会结束会话，服务重启会结束所有会话。

## 列出与查看

```sh
webmux ls                                   # 表格：ID、名称、状态、命令、目录
webmux ls --json                            # SessionInfo 数组
webmux ls --parent self --status exited     # 我创建且已结束的会话
webmux get                                  # 在会话内查看当前会话的详情
webmux get <ID> --json                      # SessionDetails：仓库、分支、启动配置、混合状态、链接、父子会话、标签
webmux get --field worktree.branch          # 只输出一个字段，不需要 jq
```

状态取值：`running`（有输出）、`waiting`（像在等待输入）、`idle`、`exited`。`attention` 字段表示会话需要用户介入（问题、授权、完成、出错、通知）。

## 父会话与标签

- 从会话内经本机 API 创建的会话自动记录 `parentId`（调用者所在的会话），请求无法伪造。
- 标签用于标记和筛选：`webmux label <ID> task=lint run=42`，`KEY-` 删除；`webmux ls --label run=42`。
- 标签键匹配 `[a-z0-9][a-z0-9._-]{0,62}`，值最多 256 个字符，每个会话最多 16 个；父会话与标签只在内存中。

## 输入与等待

```sh
webmux send <ID> 'make test' --enter --wait idle --timeout 10m   # 输入并回车，等会话回到空闲
webmux send <ID> --key esc                                       # 发送按键：enter esc tab ctrl-c ctrl-d up down left right backspace
git diff | webmux send <ID> - --enter                            # 从标准输入读取文字
webmux wait <ID> --until attention,idle --changed --timeout 30m  # 等 agent 需要介入或本轮结束
webmux wait <ID> --until exited --json | jq .exitCode
```

- `send` 只在服务端确认输入已写入 PTY 后才以 0 退出；它会像在浏览器里打字一样取得终端控制权。
- `send --wait` 先订阅再输入，只接受输入之后的状态变化；单独的 `wait` 默认当前状态也算，`--changed` 才只看之后的变化，`--stable 5s` 过滤短暂的空闲。
- 混合会话在聊天端时不能 `send`，请用 `webmux chat send`。

## 输出与画面

```sh
webmux logs <ID> --strip --tail 50       # 最近 50 行纯文本
webmux logs <ID> --follow --strip        # 以观察者身份持续输出，不抢控制权
webmux screen <ID>                       # 全屏 TUI 当前显示的画面文本（在 CLI 本地渲染）
webmux screen <ID> --json                # {cols, rows, cursor, altScreen, lines}
webmux events --session <ID>             # 会话事件，每行一个 JSON
```

`logs --strip` 只适合按行输出的程序；看全屏 TUI（例如 agent 停在授权选择框）用 `screen`。

## 结束与改名

```sh
webmux rename self "lint fixes"
webmux kill <ID>                         # 结束进程，会话保留为已结束
webmux rm <ID>                           # 移除会话（运行中的先结束）
webmux api /api/launch-configs           # 没有专用命令的接口：原样输出响应
```

## 附着与链接

- `webmux attach <ID>` 在当前终端接管会话，`Ctrl+]` 分离；`--observe` 只观察。需要真实终端。
- `webmux open <ID>` 输出不含 Token 的本机链接，`--browser` 用浏览器打开。

完整选项见 [命令参考](/llms/commands.md) 或 `webmux <命令> --help`。
