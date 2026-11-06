## 1. 创建与输入

- [x] 1.1 `run`：`--new-worktree` 改为开关并配合 `--branch`（省略时由服务端生成），`--start-side` 改为 `--view`，新增 `--set`、`--permission`、`--attach-file`、`--reply`；创建只发一次 `POST /api/sessions` 并按步骤报告失败
- [x] 1.2 `run` 在没有 `--` 时拒绝位置参数并提示 `--branch`；`--reply` 要求 `--view chat` 与 `--prompt`；删除 `chat new`
- [x] 1.3 `send` 改为调用 `/input`，增加 `--attach-file`（经会话上传接口）、`--mode`、`--reply`、`--auto-wait-requests`，`--reply` 复用原 `chat send --wait` 的回合跟踪；删除 `chat send`

## 2. 会话操作

- [x] 2.1 新增 `interrupt`、`view`、`resume`，沿用原 `chat switch` 的等待选择与候选重试、原 `chat resume` 的附着与 `--without-env` 规则；删除 `chat cancel`、`chat switch`、`chat resume`
- [x] 2.2 `ls` 增加 `--state`、`--archived`、`--repo`，表格按 `view`/`transition` 显示视图；`rm` 增加 `--forget`、`--yes`，对已归档会话不带 `--forget` 时以 1 提示；删除 `chat ls`、`chat forget`
- [x] 2.3 `chat status|watch|answer|set|permission|queue` 改读新字段，统一 `no_chat_view`/`wrong_view` 的错误与 `webmux view SESSION chat` 提示
- [x] 2.4 `launch-config add|edit` 的 `--hybrid`/`--no-hybrid` 改为 `--chat`/`--no-chat`，`launch-config options` 改用 `POST /api/launch-configs/{id}/options`
- [x] 2.5 `attach` 的移动提示与 4409 处理改用 `terminal`/`chat` 取值

## 3. 生成物与测试

- [x] 3.1 补全的已归档会话来源与 `worktree` 命令的已归档计数改读 `GET /api/sessions?state=archived`
- [x] 3.2 更新命令表一致性测试与 `internal/sessioncmd` 单元测试
- [x] 3.3 更新 `cmd/webmux` 端到端测试（真实服务进程与假 ACP agent），覆盖 `run --new-worktree --view chat --prompt --reply`、`send --reply`、`send` 的 `wrong_view`、`interrupt`、`view`、`resume`、`ls --archived`、`rm --forget`
- [x] 3.4 改写 `launch`、`chat`、`sessions`、`recipes` 主题与 `agentguide.go` 的主题摘要

## 4. 手册去重

- [x] 4.1 `internal/agentguide`：引入持有命令目录文本的 `Guide` 值，`Index`、`Page`、`Full` 改为它的方法；`commands` 主题从注入的目录渲染
- [x] 4.2 `internal/sessioncmd`：以 `Directory`（按分组每个命令一行的用法与摘要，页首指向 `--help` 与 `help --json`）取代 `Reference`；`webmux guide` 的离线副本用它渲染
- [x] 4.3 `cmd/webmux`：启动时渲染目录、构造 `agentguide.Guide` 并注入 `api.Deps`；`internal/api/guide.go` 改用注入的值
- [x] 4.4 删除 `internal/agentguide/llms/commands.md`、`TestGuideCommandReference` 与 `WEBMUX_UPDATE_GUIDE` 的说明
- [x] 4.5 手写主题删除选项清单、按键与枚举清单、默认值和单命令示例，改为写命令名并指向 `--help`；把 `wait`、`send` 等帮助中复述会话状态与 `self` 的段落缩为一句定义加 See also
- [x] 4.6 扩展 `TestHelpReferencesResolve`：校验页面与 `docs/usage/shell-control.md` 中的选项属于对应命令，除 `recipes` 外不得出现与命令表示例相同的命令行，命令目录覆盖全部可见命令且不含帮助小节
- [x] 4.7 更新 `internal/agentguide` 与 `internal/api` 中手册的测试（索引、链接、公开内容无实例信息），确认 `llms-full.txt` 不含任何 `Options:` 小节

## 5. 文档与用户故事

- [x] 5.1 更新 `docs/usage/shell-control.md` 的命令示例与说明，删除与 `--help` 重复的选项说明，手册部分说明命令目录与 `--help` 的分工
- [x] 5.2 更新 `docs/user-stories/shell-control.md`（US-S04、US-S05、US-S07、US-S08、US-S09、US-S11、US-S12、US-S14、US-S15、US-S16、US-S17、US-S18）与 `docs/user-stories/hybrid-sessions.md` 中引用旧命令的条目，同步覆盖索引
- [x] 5.3 运行 `make docs-check`

## 6. 验证

- [x] 6.1 与 `unify-session-surface` 一起运行 `make check` 与 `make test`
- [ ] 6.2 用真实服务进程冒烟：普通 shell 的 `send --enter --wait`、`interrupt`；聊天视图会话的 `run --new-worktree --view chat --prompt --reply`、`send --reply`、`view`、`kill`、`ls --archived`、`resume`、`rm --forget --yes`；旧命令 `chat send` 以 2 结束
- [ ] 6.3 冒烟手册：`webmux guide commands` 每个命令一行；服务运行与未运行时 `webmux guide --full` 都不含 `Options:`；对比改动前后 `llms-full.txt` 的字节数
