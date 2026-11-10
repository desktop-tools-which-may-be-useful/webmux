# 仓库、worktree、启动配置与端口转发

这些命令操作与 Web 端相同的数据，修改立即同步到所有浏览器。选项与示例用 `webmux <命令> --help` 查看。

## 参数

仓库参数 `REPO` 可以是仓库 ID、仓库内的路径或 `.`（当前目录所属的仓库）；worktree 参数可以是 ID、分支名或路径。
参数指向不存在的仓库或 worktree 时以 2 结束并列出候选。

## worktree 与主页显示

- 主页显示与 Web 端一致：`pinned`（加入了主页显示）、`sessions`（未加入，但因为有会话或已归档会话而显示）、`hidden`。从未设置过的仓库只显示主工作区。
- `webmux worktree create` 与 `run` 的新建 worktree 默认加入主页显示，可以选择不加入；`webmux worktree pin`、`unpin`、`home` 调整显示，只改该仓库的条目，冲突时自动重放。
- `webmux worktree rm` 先预览其中的会话、已归档会话与阻止原因；强制删除会关闭其中的会话，分支保留。非交互环境下需要明确确认。
- `webmux worktree switch` 原地切换 worktree 的分支（已有本地分支、`--track` 远程分支、`--create` 新分支或 `--detach` 提交），先在标准错误输出预览。有未提交修改、进行中的 merge/rebase 或目标分支已在别处检出时不切换并以 1 结束；修改不会被贮藏或丢弃。其中的会话不会被关闭，会看到切换后的文件；有运行会话时非交互环境需要 `--yes`，否则以 2 结束。需要并行处理时改用 `worktree create`。
- 在 WebMux 之外改动 worktree（`git worktree add/remove`）后，用 `webmux repo refresh` 让服务端重新检查。

## 仓库

`webmux repo add` 登记当前目录所在的仓库，`webmux repo rm` 取消登记（不删除任何文件）。
`webmux repo branches`、`commits` 与 `base` 帮助选择新 worktree 的基点；`base` 会先取回上游，取回失败只警告。
`webmux repo show`、`worktree ls/show` 与 `repo branches` 显示与上游的领先/落后（`+A/-B`，上游已删除为 `gone`，未计算为 `+?/-?`）、修改统计和最近提交；`repo show` 的上次获取时间是近似值。
`webmux repo remotes` 列出远程及其 fetch/push 地址，凭据由服务端脱敏。

## 仓库设置（初始化指令）

`webmux repo settings` 对应 Web 端的仓库设置对话框。初始化指令在创建会话时（Web 端快速开始，或 `run` 的 `--init`）放在首条消息前面，最多 4000 个字符。
可以写 `{{webmux}}`（WebMux 工具说明）、`{{dir}}`（会话临时文件夹）、`{{user}}`（用户消息的位置）与 `{{名称}}`（已保存的快捷指令）；引用不存在的快捷指令时只警告。

## 启动配置

- 启动配置是 Web 端新建会话时的命令选择项；`run` 用名称引用它，名称区分大小写。
- 命令原样保存，不经 shell 展开。环境变量值只在请求体中传输，列出与显示时默认隐藏。
- 开启聊天视图前，服务端先检查该命令是否是支持聊天视图的 agent，不支持时给出原因。
- `webmux launch-config edit` 只改给出的字段，其他字段（含变量值）保持不变；内置预设删掉后可以恢复。
- `webmux launch-config options` 列出聊天视图的 agent 提供的设置项与取值，供创建时设置。

## 端口转发

`webmux forward add` 把本机端口转发到 `/forward/<端口>/`，访问需要 WebMux 登录；默认对开发服务器做兼容改写，可以关闭。
