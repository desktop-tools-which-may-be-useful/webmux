## MODIFIED Requirements

### Requirement: 仓库设置

`webmux repo settings REPO` SHALL 对应 Web 端的“仓库设置”对话框，读取或修改仓库设置。目前设置项只有初始化指令，以后新增的仓库设置 SHALL 加到这个命令下。

- 不带修改选项时，SHALL 输出全部设置：初始化指令全文、长度与上限，以及其中引用的快捷指令和内置变量；`--json` 时输出 `{initInstruction}`。
- `--init TEXT`、`--init -`（标准输入）或 `--init @FILE` 设置初始化指令，`--clear-init` 清空，对应 `PATCH /api/repositories/{id}`。
  去除首尾空白后为空，等同于清空。超过上限时 SHALL 在本地以 2 拒绝。
- 指令中可以写 `{{webmux}}`、`{{dir}}`、`{{user}}` 与快捷指令引用。引用了不存在的快捷指令时，SHALL 在标准错误给出与设置页相同的提示，但仍然保存。
- `--edit` SHALL 用 `$VISUAL` 或 `$EDITOR` 打开当前指令，保存后写回；没有终端时 SHALL 以 2 结束。

#### Scenario: 为新仓库准备快速开始

- **WHEN** 脚本在新克隆的仓库目录中执行 `webmux repo add && printf '{{webmux}}\n\n{{user}}' | webmux repo settings . --init -`
- **THEN** 仓库出现在所有浏览器的工作区中，之后的 `webmux run --new-worktree --view chat --init` 带上包含 WebMux 工具说明的初始化指令

#### Scenario: 查看设置

- **WHEN** 用户执行 `webmux repo settings .`
- **THEN** 输出初始化指令全文，并列出其中引用的 `{{webmux}}` 与快捷指令名称

### Requirement: 启动配置命令

- `webmux launch-config ls [--json]` SHALL 列出启动配置摘要：名称、图标、命令、是否开启聊天视图，以及环境变量名（不含值）。
- `webmux launch-config show NAME [--json] [--reveal-env]` SHALL 按区分大小写的精确名称输出一条配置。
  默认 SHALL NOT 输出环境变量的值，`--reveal-env` 时才输出；`--json` 的结构与 `GET /api/launch-configs/{id}` 相同，值按 `--reveal-env` 决定是否替换为 `"<hidden>"`。
- `webmux launch-config add NAME [--icon ICON] [--env NAME=VALUE]... [--chat] [--preview-lines N] [--preview-skip N] -- <argv...>`
  SHALL 调用 `POST /api/launch-configs`。`argv` 原样作为命令，SHALL NOT 经过 shell 展开。
  `--chat` 时 SHALL 先调用 `POST /api/launch-configs/chat-support` 检查能否开启聊天视图，不支持时输出原因并以 1 结束。
- `webmux launch-config edit NAME [--rename NEW] [--icon ICON] [--env NAME=VALUE]... [--unset-env NAME]... [--chat|--no-chat] [--preview-lines N] [--preview-skip N] [-- <argv...>]`
  SHALL 读取现有配置，只修改给出的字段，再用 `PUT /api/launch-configs/{id}` 整体保存。未给出的字段 SHALL 保持原值，包括环境变量的值。
- `--hybrid` 与 `--no-hybrid` SHALL NOT 再被接受，按未知选项以 2 结束。
- `webmux launch-config rm NAME [--yes]` SHALL 删除配置，确认规则与 `worktree rm` 相同。
- `webmux launch-config presets [--json]` SHALL 列出内置预设；`webmux launch-config restore [PRESET]` SHALL 恢复全部或指定的预设。
- 名称不存在时 SHALL 以 2 结束并列出已保存的名称；名称冲突或校验失败时 SHALL 输出服务端原因并以 1 结束。
- 环境变量值 SHALL 只在请求体中传输，SHALL NOT 出现在命令的标准错误或诊断输出中。

#### Scenario: 用脚本添加 agent 配置

- **WHEN** 用户执行 `webmux launch-config add "Claude (sandbox)" --icon claude --chat --env CLAUDE_CONFIG_DIR=/w/.claude -- claude --dangerously-skip-permissions`
- **THEN** 配置出现在 Web 端新建会话的命令选择器中，且已开启混合模式（聊天视图）

#### Scenario: 只改一个字段

- **WHEN** 用户执行 `webmux launch-config edit "Claude (sandbox)" --unset-env CLAUDE_CONFIG_DIR`
- **THEN** 只删除这个变量，命令、图标、混合模式与其他环境变量的值保持不变

#### Scenario: 默认隐藏变量值

- **WHEN** 用户执行 `webmux launch-config show "Claude (sandbox)" --json`
- **THEN** `env` 中每个值都是 `"<hidden>"`
