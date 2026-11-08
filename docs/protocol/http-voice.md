# HTTP API：语音识别设置

所有端点需要认证并通过 Host/Origin 检查。

- 语音识别命令与环境变量的网页设置保存在 `voice-settings.json`（见 [持久化清单](../persistence.md)），覆盖 `config.toml` 的 `[voice]`。
- 保存后立即用于下一次录音，不需要重启；正在进行的录音不受影响。
- `/ws/voice` 见 [WebSocket 接口](websocket.md#websocket-wsvoice)；`GET /api/config` 的 `voice` 反映当前生效的设置。

| 方法与路径                   | 说明                                                   |
| ---------------------------- | ------------------------------------------------------ |
| `GET /api/voice/settings`    | 当前生效的识别设置与来源                               |
| `PUT /api/voice/settings`    | 整体替换并立即应用；`cmd` 为空表示关闭语音识别，200    |
| `DELETE /api/voice/settings` | 删除网页设置并立即回到 `config.toml` 的 `[voice]`，200 |

```ts
interface VoiceSettings {
  cmd: string[]; // 完整 argv，不经 shell
  env: Record<string, string>; // 附加到识别进程的环境变量
  enabled: boolean; // 已配置识别程序，界面才显示麦克风按钮
  source: "config" | "web"; // 当前设置来自 TOML 或网页
  storageAvailable: boolean; // 网页设置能否保存
  storageError?: string;
}

// PUT 请求体，只接受这两个字段
interface VoiceSettingsUpdate {
  cmd: string[];
  env: Record<string, string>;
}
```

- `env` 的值是识别服务的凭据：与启动配置一样，只有持有登录 Token 的请求才能读到，`GET /api/config` 从不返回它们。
- 校验与 `[voice]` 相同：`cmd[0]` 不能为空，环境变量名合法且不是保留名。违反返回 400。WebMux 不提供识别服务，`cmd` 是使用者自己的程序，不再有 `provider`。
- 设置文件损坏或无法写入时语音识别保持关闭（`fail_closed`），读取返回 `storageAvailable: false`，写入返回 503，原文件不被改写。
