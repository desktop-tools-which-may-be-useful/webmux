## Context

动机见 proposal.md 的 Why，行为契约见 `specs/attention-sound/spec.md`。与实现相关的现状：

- `AppShell.svelte` 挂载一次 `createSessionList`，它把 `/ws/sessions` 的快照与增量交给 `web/src/attention-toast.svelte.ts` 的 `observeAttentionSnapshot`、`observeAttentionSession` 与 `observeAttentionRemoved`。
- 这三个函数已经以 `attentionKey(attention)`（`kind` 与 `since`）维护每个会话的已见关注、首个快照建立基线、重连快照补报；判定出新关注后，若弹窗开关关闭就直接返回。
- 页面本地设置统一通过 `web/src/api.ts` 的 `stored()` / `store()` 读写 `localStorage`（键前缀 `webmux.`），私密模式下写入失败被忽略。前端目前没有使用 IndexedDB。
- “通知推送”卡片是 `web/src/components/PushSettings.svelte`，顶部为“页面内右上角弹窗提醒”开关，其后是推送状态与各类不可用提示。
- 远程设备当前经 `http://<Tailscale IP>:7681` 访问，不是安全上下文：`navigator.locks`、Service Worker、Push 与 Notification 不可用，Web Audio、IndexedDB 与 `localStorage` 可用。

## Goals / Non-Goals

**Goals:**

- 提示音与弹窗共用同一个“新关注”判定，不新增第二套去重状态。
- 所有存储与播放都在浏览器内完成，不改服务端、契约与 wire 快照。
- 非安全上下文中的行为与 HTTPS 下一致。

**Non-Goals:**

- 不做页面后台时的系统通知，也不改 `sw.js`。
- 不按会话或按关注种类逐一设置声音，只分两组。
- 不做音频格式转换或裁剪，只校验后原样保存。
- 不尝试绕过浏览器的自动播放策略（例如静音视频或无声循环保活）。

## Decisions

### 1. 触发点放在弹窗判定之后、开关判断之前

`observeSession` 在判定出新关注（`seen` 更新）之后、`if (!enabled) return;` 之前调用提示音模块的 `notifyAttention(sessionId, attention)`；快照分支同理，基线快照不调用。这样提示音与弹窗的“新关注”定义天然一致，弹窗开关不影响提示音。

- 备选：提示音模块自己订阅 `/ws/sessions`。会多一条连接并复制基线与重连逻辑，放弃。
- 备选：在 `AttentionToasts.svelte` 渲染新弹窗时播放。弹窗关闭时就不响，且把播放绑在组件生命周期上，放弃。

### 2. 模块划分

| 文件 | 职责 |
| --- | --- |
| `web/src/attention-sound.ts` | 纯函数：关注种类到分组的映射、设置的 Zod 模式与默认值、批次内选最紧急的组、跨标签页去重键的合并与裁剪、上传文件的校验结论 |
| `web/src/attention-sound-tones.ts` | 内置音色的定义（振荡器类型、频率、包络）与用 `AudioContext` 合成播放 |
| `web/src/attention-sound-store.ts` | 自定义音频的 IndexedDB 读、写、删，只暴露这三个异步函数 |
| `web/src/attention-sound.svelte.ts` | 运行时：读取设置、解锁 `AudioContext`、缓存解码结果、1 秒批次合并、跨标签页去重、播放与兜底；向设置组件暴露状态（例如自定义音频是否可用） |
| `web/src/components/AttentionSoundSettings.svelte` | 设置界面，嵌入 `PushSettings.svelte` 中弹窗开关之后 |

纯函数与 Web Audio、IndexedDB 分开，便于在 Vitest 中直接测试判定逻辑。

### 3. 设置保存在 `localStorage`，音频保存在 IndexedDB

- 分组选择与音量序列化为一个 JSON，经 `store("attentionSound", …)` 保存，读取时用 `AttentionSoundSettingsSchema` 校验；整体解析失败用默认值，单项引用了不存在的音色时只把该项回退为默认值。
- 自定义音频以 `Blob` 存进数据库 `webmux` 的对象仓库 `attention-sound`，固定键 `custom`，值为 `{ blob, name, type, size, savedAt }`。`localStorage` 只能存字符串，base64 后体积增加约三分之一，且与其他设置共享约 5 MB 的配额，因此音频不放 `localStorage`。
- 设置中不记录音频内容，只记录某组选择了 `custom`；音频是否存在以 IndexedDB 的读取结果为准。

### 4. 上传校验顺序

1. `file.size > 1 MiB` 直接拒绝，不读取内容；
2. 读成 `ArrayBuffer` 后用 `decodeAudioData` 解码，失败即“当前浏览器无法播放该文件”；
3. `audioBuffer.duration > 10` 拒绝；
4. 全部通过后写入 IndexedDB，并把解码结果放进内存缓存。

文件选择框使用 `accept="audio/*"` 只作提示，以解码结果为准，因此能否使用某种格式取决于当前浏览器。

### 5. 播放与自动播放限制

- `AudioContext` 在首次需要播放或首次用户交互时懒创建。模块在 `window` 上注册一次性的 `pointerdown` / `keydown` 捕获监听，在其中调用 `resume()`。
- 播放时若 `audioContext.state !== "running"`，直接跳过，不排队补播。
- 音量经 `GainNode` 统一作用于内置音色与自定义音频，0 时不创建声源。
- 自定义音频解码一次后缓存 `AudioBuffer`；IndexedDB 读取失败、记录缺失或解码失败时改用默认内置音色，并把 `customAvailable` 状态置为 false，供设置卡片显示提示。

### 6. 批次合并与跨标签页去重

- 同一页面在 1 秒窗口内收到的新关注先缓存；窗口结束时选出其中最紧急的组，组内静音则看下一组，最终只播放一次。
- 跨标签页：`navigator.locks` 在 HTTP 下不可用，`BroadcastChannel` 需要额外的选主协议。这里改用 `localStorage` 的去重表 `webmux.attentionSound.played`：保存最近 32 个 `sessionId + attentionKey` 及时间。
  - 窗口结束时，先重新读取去重表，丢掉已经记录的关注，再把剩余的写入；只有写入了新关注的页面播放。
  - 同一浏览器的各标签页共享 `localStorage`；两个页面在同一毫秒内读写的竞态可能导致偶尔响两次，代价可以接受。
- 备选：只让可见的标签页播放。所有标签页都在后台时就不响，正好违背需求，放弃。

### 7. 内置音色

提供三种：`chime`（两个上行音，默认）、`ding`（单个短音）、`pulse`（三个短促音），均不超过 2 秒，用 `OscillatorNode` 加指数衰减包络合成，不打包音频文件，也不发起网络请求。界面名称分别为“叮咚”“单响”“急促”。

### 8. 设置界面

- 使用现有设计系统组件：`Select` 选择每组的声音，`Slider` 调音量，`Button` 负责试听、上传、删除；上传使用隐藏的文件输入框加按钮。
- 试听按钮是图标按钮，需要可访问名与可见提示。
- 校验失败与自定义音频不可用时用 `InlineNotice` 提示。
- 遵循 `webmux-ui` skill，PR 附桌面、平板、手机截图。

## Risks / Trade-offs

- [iOS Safari 在锁屏或页面进入后台后暂停 `AudioContext`] → 提示音只承诺页面处于可运行状态时播放；锁屏场景由 HTTPS 下的 Web Push 覆盖，在文档中说明。
- [桌面浏览器会节流后台标签页的定时器，1 秒批次可能延后] → 只影响延迟，不影响去重；批次在定时器回调中结算，延后后仍只播放一次。
- [浏览器在存储压力下清除 IndexedDB] → 按兜底要求改用内置音色并提示重新上传；不调用 `navigator.storage.persist()`，因为它在部分浏览器会弹出权限请求。
- [多台设备同时打开页面会各响一次] → 每台设备的设置独立，用户可在不需要的设备上设为静音；跨设备协调需要服务端参与，不在本次范围。
- [`localStorage` 去重存在极小的竞态] → 只会偶尔多响一次，不影响正确性。

## Migration Plan

纯前端新增功能，无数据迁移。已有设备首次加载新版本时没有提示音设置，按默认值处理：紧急组响默认音色，普通组静音。回滚即还原前端，遗留的 `webmux.attentionSound*` 键与 IndexedDB 数据库不影响旧版本。
