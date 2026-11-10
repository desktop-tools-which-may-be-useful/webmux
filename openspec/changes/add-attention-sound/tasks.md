# Tasks

## 1. 判定与设置（纯函数）

- [ ] 1.1 新增 `web/src/attention-sound.ts`：
  - 关注种类到紧急组、普通组的映射；
  - `AttentionSoundSettingsSchema` 与默认值（紧急组 `chime`、普通组静音、音量 70），整体解析失败回退默认值，单项无效时只回退该项；
  - 批次内按“紧急组优先、静音组跳过”选出一次播放；
  - 跨标签页去重表的合并、按 32 条裁剪与“过滤已播放”；
  - 上传文件的大小与时长校验结论。

  验证：新增 `web/test/attention-sound.test.ts`，覆盖分组映射、默认值与损坏回退、单项回退、批次选组（含普通组静音、两组都静音）、去重表过滤与裁剪、1 MiB 与 10 秒边界，并用 `pnpm --dir web exec vitest run --config vitest.config.ts test/attention-sound.test.ts` 通过。

## 2. 音色合成与自定义音频存储

- [ ] 2.1 新增 `web/src/attention-sound-tones.ts`：`chime`、`ding`、`pulse` 三种音色，每种不超过 2 秒，用振荡器与包络合成，经传入的 `GainNode` 输出。验证：浏览器测试中用真实 `AudioContext` 播放每种音色，不出现网络请求、不抛错，且合成时长不超过 2 秒。
- [ ] 2.2 新增 `web/src/attention-sound-store.ts`：在数据库 `webmux` 的仓库 `attention-sound` 中以键 `custom` 读、写、删 `{ blob, name, type, size, savedAt }`；IndexedDB 不可用时读取返回空，写入抛出带原因的错误。验证：浏览器测试覆盖写入后读回同一 Blob 与文件名、删除后读取为空。

## 3. 运行时播放

- [ ] 3.1 新增 `web/src/attention-sound.svelte.ts`：
  - 懒创建 `AudioContext`，在一次性的 `pointerdown` / `keydown` 捕获监听中 `resume()`；
  - `notifyAttention` 进入 1 秒批次，批次结算时先过滤 `webmux.attentionSound.played` 去重表再播放；
  - 未解锁时跳过，不补播；
  - 缓存自定义音频的解码结果，读取或解码失败时改播 `chime` 并把 `customAvailable` 置为 false；
  - 提供试听入口（不经批次与去重）。

  验证：浏览器测试覆盖以下情况：
  - 未交互时不发声，交互后发声；
  - 同批次多条只播一次，且选紧急组；
  - 两个页面实例共用 `localStorage` 时，同一关注只播一次；
  - 自定义音频缺失时回退并更新 `customAvailable`。
- [ ] 3.2 修改 `web/src/attention-toast.svelte.ts`，接入提示音：
  - 在 `observeSession` 判定出新关注、检查弹窗开关之前调用 `notifyAttention`；
  - 基线快照、重复键、退出与清除不调用；
  - `resetAttentionToasts` 同时重置提示音的批次与内存状态。

  验证：扩展 `web/test/browser/AttentionToasts.test.ts`，覆盖基线快照不触发、重复键不触发、重连快照新键触发一次、弹窗开关关闭时仍触发。

## 4. 设置界面

- [ ] 4.1 新增 `web/src/components/AttentionSoundSettings.svelte` 并嵌入 `PushSettings.svelte` 中弹窗开关之后：
  - 两组声音选择（静音、三种内置音色、自定义音频；未上传时“自定义音频”不可选）；
  - 0–100 音量滑块；
  - 每组的试听按钮；
  - 上传、替换与删除自定义音频，显示文件名；
  - 校验失败与自定义音频不可用时用 `InlineNotice` 提示。

  只使用 `web/src/lib/components/ui/` 与 `web/src/components/` 中的组件，图标按钮带可访问名与提示。

  验证：
  - 新增 `web/test/browser/AttentionSoundSettings.test.ts`，覆盖默认值显示、修改后写入 `localStorage`、删除后组回退为 `chime`、`customAvailable` 为 false 时的提示；
  - 同一测试覆盖超过 1 MiB、无法解码与超过 10 秒三种拒绝文案，且原选择不变；
  - 补充对应 Storybook story，`pnpm --dir web check:arch` 通过。
- [ ] 4.2 更新 `PushSettings.svelte` 卡片说明，提及提示音；非安全上下文提示加入“页内弹窗与提示音在当前地址仍可用”。验证：`web/test/browser/PushSettings.test.ts` 中非安全上下文用例断言该说明可见。
- [ ] 4.3 按 `webmux-ui` skill 截取设置卡片在桌面、平板、手机尺寸下的改动前后截图，以及上传与试听交互的 GIF，供 PR 使用。

## 5. 文档与用户故事

- [ ] 5.1 更新文档：
  - `docs/usage/attention.md`：提示音的分组、默认值、自定义音频与限制（需页面打开并交互过、锁屏不保证、不改变系统通知声音）；
  - `docs/deployment/pwa-and-push.md`：HTTP 远程访问只有页内弹窗与提示音；同一浏览器从不同地址开启推送会形成多个订阅并收到重复通知；
  - `docs/usage/sessions.md` 跨设备偏好表的“只留在当前设备”一栏加入提示音设置与自定义音频。

  验证：`make docs-check` 通过。
- [ ] 5.2 更新用户故事：
  - 修改 `docs/user-stories/access-and-sessions/workspace.md` 的 US-C06、US-C12，新增 US-C19“页面提示音与自定义音频”；
  - 同步 `docs/user-stories.md` 与 `docs/user-stories/coverage.md` 中的编号范围与计数；
  - 在当月核对记录中只登记实际运行过的验证。

  验证：`make docs-check` 通过，两处索引的 C 组范围一致。

## 6. 集成检查

- [ ] 6.1 运行 `make check` 与 `make test-fast`，并在 Chromium 中手动验收：经非回环 HTTP 地址打开两个标签页，交互后触发一次 `permission` 关注，只听到一次提示音；上传自定义音频后再次触发，播放该音频。
