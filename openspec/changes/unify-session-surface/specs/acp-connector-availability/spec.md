## MODIFIED Requirements

### Requirement: 连接器信息与原因码的负载

`connector_missing` 出现在以下负载中时，SHALL 同时携带连接器信息 `connector: {executable, title, installCommand, docsUrl}`：

- 会话信息中的聊天资格 `{available: false, reason: "connector_missing", connector}`；
- 新建会话、切换、恢复请求失败时的 409 冲突体 `{error, reason: "connector_missing", connector}`；
- 自动重连失败后会话进入 `interrupted` 时发布的通知。

连接器信息 SHALL 只来自档案，SHALL NOT 包含查找到的路径、PATH 内容或环境变量。

#### Scenario: 冲突体

- **WHEN** 客户端以 Codex 混合配置和 `view: "chat"` 创建会话，连接器未安装
- **THEN** 服务端返回 409，`reason` 为 `connector_missing`，`connector.executable` 为 `codex-acp`，不创建会话，不写入混合会话记录
