## MODIFIED Requirements

### Requirement: 新建会话页选择启动配置

新建会话对话框 SHALL 通过单一命令选择器选择命令来源：列表包含已保存的启动配置与末尾的“自定义命令…”，二者都只使用自定义命令；系统 SHALL NOT 提供内置预设，SHALL NOT 再以“预先输入 / 手动填写”Tab 区分。提交内容取决于所选来源：

- 保存的启动配置：提交 SHALL 发送 `configId` 与本次选择的 `cwd` 或 `workspace`，可选发送 `name` 与 `startSide`，SHALL NOT 同时发送 `cmd` 或 `env`。
- 自定义命令：提交 SHALL 发送完整 `cmd` 及本次选择的 `cwd` 或 `workspace`，该命令不写入服务端。

页面 SHALL 只请求启动配置摘要，SHALL NOT 请求环境变量值。在配置与自定义命令之间切换 SHALL 保留已选配置与已输入命令，但只提交当前来源。

页面 SHALL 在当前浏览器以键 `lastLaunch`（`config:<ID>` 或 `custom`）记住最近一次成功创建所用的启动方案、以 `customCommand` 记住自定义命令，SHALL NOT 缓存配置内容；这些键的格式 SHALL 保持兼容。记住的配置已不存在时 SHALL NOT 选中它。默认填入与失效处理的对话框行为见 `new-session-dialog` 规格。

没有启动配置时，命令选择器 SHALL 显示空状态与前往 `/settings#launch-configs` 的链接，并默认选中“自定义命令…”；启动配置不可用时 SHALL 说明原因。启动配置的加载或失败 SHALL NOT 阻塞自定义命令；加载期间保存的配置不可选，避免发送空的 `configId`。

#### Scenario: 从配置启动

- **WHEN** 用户选择名为“Claude 恢复”的启动配置与目录后提交
- **THEN** 请求只包含 `configId`、目标目录与会话名，会话按该配置保存的 argv 与 env 启动

#### Scenario: 手动命令照常启动

- **WHEN** 用户在命令选择器选择“自定义命令…”并输入完整命令后提交
- **THEN** 请求包含 `cmd` 与目标目录，不包含 `configId`，命令不写入服务端

#### Scenario: 存储不可用时仍可启动

- **WHEN** 启动配置接口返回 503
- **THEN** 命令选择器显示不可用原因，“自定义命令…”仍可选择并启动会话
