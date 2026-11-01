## MODIFIED Requirements

### Requirement: 混合会话的创建与起始端

系统 SHALL 只在使用勾选了混合模式的启动配置创建会话时创建混合会话。不带配置的自定义命令和未勾选的配置 SHALL 创建普通会话，行为不变。

`POST /api/sessions` 在 `configId` 指向混合配置时 SHALL 接受可选字段 `startSide`，取值为 `tui` 或 `acp`，缺省为 `tui`。对普通配置或自定义命令提供 `startSide` SHALL 返回 400。

- 从终端开始：系统 SHALL 在 PTY 中执行配置保存的 argv，会话处于 `tui` 端。
- 从聊天开始：档案声明了第三方连接器时，系统 SHALL 先检查连接器已安装，未安装时 SHALL 返回 409 `connector_missing`
  并附连接器信息，SHALL NOT 创建会话或写入混合会话记录。检查通过后，系统 SHALL 按档案启动 ACP 进程，
  依次调用 `initialize` 和 `session/new {cwd}`，记录返回的 agent 会话 ID，会话处于 `acp` 端。

两种起始方式 SHALL 都写入混合会话记录。

Web 端新建会话弹层 SHALL 在选中混合配置时显示“从终端开始 / 从聊天开始”选择，默认从终端开始，并按浏览器记住上一次的选择。

#### Scenario: 从聊天开始

- **WHEN** 用户选择勾选了混合模式的 omp 配置，选择“从聊天开始”并提交
- **THEN** 会话出现在列表中，`hybrid.side = "acp"`，页面显示聊天视图；PTY 中没有运行任何进程

#### Scenario: 从聊天开始但连接器未安装

- **WHEN** 用户选择 Codex 混合配置、从聊天开始并提交，服务上没有 `codex-acp`
- **THEN** 服务端返回 409 `connector_missing`，会话列表不变，没有启动任何进程

#### Scenario: 对普通配置指定起始端

- **WHEN** 客户端以未勾选混合模式的配置 ID 和 `startSide: "acp"` 创建会话
- **THEN** 服务端返回 400，不创建会话

### Requirement: 从终端切到聊天

收到切到 `acp` 的请求且会话在 `tui` 端时，系统 SHALL 先按 `agent-session-identity` 的资格判定确认可以切换（含连接器检查），不满足时返回 409 并附原因，不改变会话。确认可以切换后，系统 SHALL 进入 `switching`，按以下顺序执行：

1. 根据终端信号判断 TUI 是否在执行回合；在执行时，向 PTY 写入档案定义的中断键，并等待信号变为空闲；
2. 结束 TUI 进程，并等待其进程组清理完成；
3. 按档案启动 ACP 进程，调用 `initialize`，再用资格判定得到的 agent 会话 ID 与 cwd 调用 `session/load`；
4. 用记录中的配置项取值补齐 agent 未返回或回到默认值的配置（按档案判断）；记录中尚无某项取值时，使用按档案从 argv 推导的值；
5. 进入 `acp` 端。

切换期间，所有设备的输入 SHALL 被禁用，已连接的终端客户端 SHALL 以关闭码 4409 断开并收到“已切换到聊天视图”的原因。

#### Scenario: 空闲时切换

- **WHEN** TUI 空闲，用户点击“聊天”
- **THEN** 会话在一次切换内进入 `acp` 端，聊天视图回放完整历史，模型与推理强度与切换前一致

#### Scenario: 回合进行中切换

- **WHEN** TUI 正在执行回合，用户确认切到聊天
- **THEN** 系统先发送中断键并等到空闲，再退出 TUI；被中断的回合以中断结束，会话之后可以继续对话

#### Scenario: Codex 首次切到聊天

- **WHEN** argv 为 `["codex","-m","gpt-5.6-luna","-s","read-only"]` 的 Codex 混合会话在 TUI 中发出首条消息后切到聊天
- **THEN** 聊天端的 `model` 为 `gpt-5.6-luna`、`mode` 为 `read-only`，并写入混合会话记录；之后每次加载都重新应用记录中的取值

## ADDED Requirements

### Requirement: 首次在聊天端打开时播种配置

档案提供了 argv → 配置项的推导时，系统 SHALL 在混合会话第一次在聊天端打开 agent 会话（`session/new` 或第一次 `session/load`）之后，
把推导出的、记录中尚无取值的配置项用 `session/set_config_option` 设置，并写入混合会话记录。

- agent 返回的配置项中没有该 ID 或没有该取值时，系统 SHALL 跳过该项，并在聊天视图提示“以下参数在聊天端不生效”；
- 设置失败 SHALL NOT 使打开失败，系统 SHALL 在聊天视图显示失败的配置项；
- 用户之后在聊天视图修改的取值 SHALL 覆盖推导值。TUI 中通过 `/model` 等命令做出的修改 SHALL NOT 被延续。

#### Scenario: 从聊天开始的 Codex 会话

- **WHEN** 预置的 Codex CLI 混合配置从聊天开始
- **THEN** `session/new` 之后系统设置 `mode = agent-full-access`，聊天视图的模式显示为完全访问

#### Scenario: agent 不提供推导值

- **WHEN** argv 带 `-m no-such-model`，agent 的 `model` 配置项不含该取值
- **THEN** 会话照常打开，聊天视图提示 `-m no-such-model` 在聊天端不生效
