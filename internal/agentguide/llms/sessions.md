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

状态取值：

- `running`：最近 2 秒内有输出，或 agent 报告正在工作。
- `waiting`：需要用户介入。包括 agent 的授权请求或提问、未读的完成或出错；没有 agent 信号时，末行像提示符或进程在等终端输入也算，因此停在提示符的 shell 是 `waiting`。
- `idle`：长期无活动（超过 `idle_after_seconds` 无输出）且没有上述迹象。它不是“命令或回合已结束”的信号。
- `exited`：进程已结束。

`attention` 字段表示会话需要用户介入（问题、授权、完成、出错、通知）；agent 完成一回合后会锁存 `done` 关注，用户输入后才清除。

## 父会话与标签

- 从会话内经本机 API 创建的会话自动记录 `parentId`（调用者所在的会话），请求无法伪造。
- 标签用于标记和筛选：`webmux label <ID> task=lint run=42`，`KEY-` 删除；`webmux ls --label run=42`。
- 标签键匹配 `[a-z0-9][a-z0-9._-]{0,62}`，值最多 256 个字符，每个会话最多 16 个；父会话与标签只在内存中。

## 输入与等待

```sh
webmux send <ID> 'make test' --enter --wait waiting,idle --timeout 10m   # 输入并回车，等 shell 回到提示符（提示符读作 waiting）
webmux send <ID> --key esc                                       # 发送按键：enter esc tab ctrl-c ctrl-d up down left right backspace
git diff | webmux send <ID> - --enter                            # 从标准输入读取文字
webmux wait <ID> --until attention --changed --timeout 30m       # 等 agent 需要介入或本轮结束
webmux wait <ID> --until exited --json | jq .exitCode
```

- `send` 只在服务端确认输入已写入 PTY 后才以 0 退出；它会像在浏览器里打字一样取得终端控制权。
- `send --wait` 先订阅再输入，只接受输入之后的状态变化；单独的 `wait` 默认当前状态也算，`--changed` 才只看之后的变化，`--stable 5s` 要求条件持续一段时间，过滤短暂的状态切换。
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

## 提醒用户

```sh
make release; webmux notify --kind done "release 构建完成"   # 会话显示提醒，浏览器弹出提示，订阅推送的设备收到通知
webmux notify --kind question "请在终端里选择迁移方案"          # kind：notify（默认）、done、error、question
webmux notify --clear --session <ID>                          # 清除提醒，效果与用户输入相同
```

- 默认作用于当前会话（`self`）；会话外需要 `--session`。消息最多 200 个字符，只保存在内存中。
- 提醒覆盖当前锁存的提醒，之后按原有规则清除：用户输入或打开会话、程序的下一次状态报告、`--clear`。`send` 的输入同样会清除它。

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
