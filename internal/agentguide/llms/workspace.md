# 仓库、worktree、启动配置与端口转发

这些命令操作与 Web 端相同的数据，修改立即同步到所有浏览器。仓库参数 `REPO` 可以是仓库 ID、仓库内的路径或 `.`（当前目录所属的仓库）；
worktree 参数可以是 ID、分支名或路径。参数指向不存在的仓库或 worktree 时以 2 结束并列出候选。

## worktree

```sh
webmux worktree ls .                               # REPO ID KIND BRANCH PATH STATE SESSIONS HOME
webmux worktree create . --branch fix/lint --base main   # 输出新 worktree 路径（默认 <仓库>.worktrees/<分支末段>）
webmux worktree create . --branch exp/a --refresh-base   # 先取回上游，以取回的提交为基点
webmux worktree show . fix/lint --json             # 字段、主页状态、其中的会话与归档记录
webmux worktree rm . fix/lint --yes                # 先预览；有阻止原因时需要 --force（会关闭其中的会话）
```

- `HOME` 与 Web 端主页一致：`pinned`（加入了主页显示）、`sessions`（未加入，但因为有会话或归档记录而显示）、`hidden`。
  从未设置过的仓库只显示主工作区。
- `create` 与 `run --new-worktree` 新建的 worktree 默认加入主页显示，`--no-pin` 不加入。
- `worktree pin|unpin . <W>...` 加入或取消；`worktree home . [--set <W>... | --reset]` 查看或整体替换；只改该仓库的条目，冲突时自动重放。
- `rm` 在非交互环境下必须 `--yes`；终端上需要输入 worktree 路径确认。分支保留。

## 仓库

```sh
webmux repo add                       # 登记当前目录所在的仓库；已登记时原样输出
webmux repo ls                        # ID NAME PATH AVAILABILITY WORKTREES HOME INIT
webmux repo show . --json
webmux repo refresh .                 # 在终端外改动 worktree 后让服务端重新检查
webmux repo branches .                # 本地分支、提交、检出它们的 worktree
webmux repo commits . --from main     # 最近的提交，用来选 --base
webmux repo base . main               # 取回上游并报告新 worktree 的基点；取回失败只警告
webmux repo rm . --yes                # 取消登记，不删除任何文件
```

## 仓库设置（初始化指令）

```sh
webmux repo settings .                                  # 输出初始化指令全文、长度与引用
printf '{{webmux}}\n\n{{user}}' | webmux repo settings . --init -
webmux repo settings . --init @.webmux/init.md          # 从文件读取；--clear-init 清空；--edit 用 $EDITOR 编辑
```

初始化指令最多 4000 个字符，在快速开始（`webmux chat new`）时放在首条消息前面。可以写 `{{webmux}}`（WebMux 工具说明）、
`{{dir}}`（会话临时文件夹）、`{{user}}`（用户消息的位置）与 `{{名称}}`（已保存的快捷指令）；引用不存在的快捷指令时只警告。

## 启动配置

```sh
webmux launch-config ls                                  # 名称、图标、混合模式、变量名（不含值）、命令
webmux launch-config show "Claude Code" --json           # 变量值显示为 <hidden>；--reveal-env 才输出值
webmux launch-config add "Claude (sandbox)" --icon claudecode --hybrid --env CLAUDE_CONFIG_DIR=/w/.claude -- claude
webmux launch-config edit "Claude (sandbox)" --unset-env CLAUDE_CONFIG_DIR   # 只改给出的字段，其他（含变量值）保持不变
webmux launch-config rm "Dev server" --yes
webmux launch-config presets; webmux launch-config restore "Claude Code"
```

`--` 之后的命令原样保存，不经 shell 展开；`--hybrid` 先检查该命令是否是支持混合模式的 agent。

## 端口转发

```sh
webmux forward add 5173 --session self --label web       # 输出 /forward/5173/ 的完整地址
webmux forward ls
webmux forward rm 5173
```

转发地址需要 WebMux 登录；`--no-compat` 关闭开发服务器兼容改写。
