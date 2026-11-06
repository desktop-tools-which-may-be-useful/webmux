# 混合会话聊天

混合会话在终端 TUI 与聊天视图（ACP）之间切换同一个 agent 会话。`webmux chat` 子命令使用与 Web 端聊天视图相同的接口，
每次写操作都带新的请求 ID，所有设备同步看到结果。目标不是混合会话时以 1 结束；需要聊天端的命令在终端端时以 1 结束，并提示先切换。

## 发送与等待

```sh
webmux chat send <ID> "修复 lint 并汇报改动" --wait --timeout 20m   # 输出 agent 的最终回复，end_turn 时退出码 0
webmux chat send <ID> "下一步"                                        # 不等待：输出 {queued, itemId, turnId, promptId}
git diff | webmux chat send <ID> - --attach design.png --wait --json  # 从标准输入读取，附带文件，输出该回合的事件
webmux chat send <ID> "改用较小的修复" --mode steer                   # 插入正在进行的回合
```

- agent 空闲时立即发送，否则进入服务端队列；`--wait` 跟踪这条提示自己的回合（排队的提示等到它被发送）。
- 回合中出现授权或提问时，`--wait` 在标准错误给出请求 ID 与选项并以 1 退出，回合保持等待；`--auto-wait-requests` 继续等待别人回答。
- 超时以 124 退出，不取消回合；排队的提示在发送前被删除时以 1 退出。

## 查看

```sh
webmux chat status <ID>            # 所在端、回合、队列、权限模式、待回答请求（含选项）与 agent 设置
webmux chat status <ID> --json     # ChatState
webmux chat watch <ID>             # 持续输出消息、工具调用、请求与回合边界；--json 每行一个帧，--history 包含历史
```

## 回答、取消与设置

```sh
webmux chat answer <ID> <请求ID> --option allow_once        # 授权请求选一个选项
webmux chat answer <ID> <请求ID> --content '{"plan":"B"}'   # 回答问题表单；--decline 拒绝，--cancel 取消
webmux chat cancel <ID>                                     # 结束当前回合；空闲时什么也不做
webmux chat set <ID> model=sonnet                           # 设置 agent 配置项（ID 与可选值见 chat status）
webmux chat permission <ID> ask                             # ask：每次授权都等回答；auto：服务端自动批准
webmux chat queue ls <ID>                                   # 排队的提示；rm、edit（--up 上移）、send-now、clear、resume
```

## 切换与历史

```sh
webmux chat switch <ID> --to chat          # 切到聊天端并等待完成；终端客户端（attach、浏览器）被分离
webmux chat switch <ID> --to terminal --force   # 终端 TUI 忙时强制中断；不加 --force 则取消切换并以 1 退出
webmux chat ls --archived                  # 已关闭或已中断、可以恢复的记录
webmux chat resume <记录ID> --side chat --detach
webmux chat forget <记录ID> --yes          # 删除记录（agent 自己的会话历史保留）；非交互时必须 --yes
```

服务端找不到 agent 会话时会列出候选，用 `--agent-session ID` 重试；启动配置已被删除时，`chat resume` 需要 `--without-env`。
