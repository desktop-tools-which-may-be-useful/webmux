## ADDED Requirements

### Requirement: 会话列表中的 agent 标识

首页会话卡片 SHALL 为任意所在端的混合会话（`hybrid.side` 为 `tui`、`acp` 或 `switching`，含休眠，含“恢复到终端”后的会话）在“所在端”信息处显示 agent 标识：

- 标识由 `agentIcon` 对应的单色品牌小图标与 `agentTitle` 组成，紧接其后是现有的所在端文字，形如“OpenCode · 聊天”；
- 品牌图标 SHALL 替换原来的通用聊天或终端图标，尺寸与同一行的其他元信息图标相同，颜色跟随该行的弱化文字色；
- 图标 SHALL 是装饰性的（对辅助技术隐藏），agent 名称以可见文字提供；
- `agentIcon` 不在本地图标目录中时 SHALL 只显示名称，SHALL NOT 用终端等其他图标代替。

#### Scenario: 聊天端卡片

- **WHEN** 首页列出一个处于聊天端的 OpenCode 混合会话，有 2 个设备连接
- **THEN** 卡片元信息行显示 OpenCode 品牌小图标与“OpenCode · 聊天 · 2 个设备已连接”，不再显示通用聊天图标

#### Scenario: 休眠的聊天端卡片

- **WHEN** 处于聊天端的 omp 混合会话进入休眠
- **THEN** 卡片显示 omp 的品牌小图标与“omp · 聊天·休眠”

#### Scenario: 切换中的卡片

- **WHEN** 首页列出一个正在切换的 OpenCode 混合会话
- **THEN** 卡片显示 OpenCode 品牌小图标与“OpenCode · 切换中”，切换过程中标识不消失

#### Scenario: 终端端卡片

- **WHEN** 用户把一个已关闭的 Claude Code 混合会话“恢复到终端”，随后回到首页
- **THEN** 卡片显示 Claude Code 品牌小图标与“Claude Code · 终端”，不再只显示通用终端图标

#### Scenario: 未知图标键

- **WHEN** 会话的 `agentIcon` 不在本地图标目录中
- **THEN** 卡片只显示 agent 名称与所在端文字，不显示任何替代图标

### Requirement: 聊天页头部的 agent 标识

会话页显示聊天视图时，头部 SHALL 在会话标题之后、状态点之前显示当前 agent 的标识，强调程度低于标题：

- 宽度不小于 640px 时，标识 SHALL 显示小图标与 `agentTitle`，文字使用弱化色与小号字；
- 宽度小于 640px 时，标识 SHALL 只显示图标；
- 任何宽度下，标识 SHALL 有可访问名称“Agent：<agentTitle>”，并以悬停提示显示同样的文字；
- 空间不足时 SHALL 先截断会话标题，agent 标识 SHALL 保持完整；
- 标识 SHALL 只用于展示，SHALL NOT 可聚焦或可点击；
- `agentIcon` 不在本地图标目录中时，所有宽度 SHALL 显示名称且不显示图标。

会话页显示终端视图时，头部 SHALL NOT 显示该标识。

#### Scenario: 桌面聊天页

- **WHEN** 用户在 1280px 宽的窗口中打开处于聊天端的 Claude Code 混合会话
- **THEN** 头部依次显示会话标题、Claude Code 小图标与“Claude Code”、状态点；悬停标识时提示“Agent：Claude Code”

#### Scenario: 手机聊天页

- **WHEN** 用户在 390px 宽的手机上打开同一会话的聊天视图
- **THEN** 头部在标题后只显示 Claude Code 图标，屏幕阅读器读出“Agent：Claude Code”，长标题被截断而图标完整可见

#### Scenario: 切到终端后隐藏

- **WHEN** 用户在聊天页点击“终端”并完成切换
- **THEN** 头部不再显示 agent 标识；切回聊天后标识重新出现
