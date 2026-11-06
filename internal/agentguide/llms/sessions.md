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

## 附着与链接

- `webmux attach <ID>` 在当前终端接管会话，`Ctrl+]` 分离；`--observe` 只观察。需要真实终端。
- `webmux open <ID>` 输出不含 Token 的本机链接，`--browser` 用浏览器打开。

完整选项见 [命令参考](/llms/commands.md) 或 `webmux <命令> --help`。
