# Proposal

## Why

手机在慢速链接上从终端页点“返回会话列表”回到首页时，首页需要重新读取会话快照（WebSocket）、服务端设置（`/api/config`）、Git 仓库发现和启动配置。

现有实现只缓存了上次的仓库分组；会话列表和服务端设置仍然必须等待，启动骨架屏因此显示很久，用户看不到刚刚还在看的会话。

## What Changes

- 首页把最近一次成功得到的会话列表、服务端设置（默认目录、主机名、待安装更新、Git 与仓库登记能力）连同已有的仓库分组一起缓存在当前标签页的内存里。
- 再次进入首页（例如从终端返回）时，如果有缓存，立即渲染缓存内容，不显示启动骨架屏；会话快照、设置和 Git 发现在后台刷新，到达后覆盖缓存内容。
- 没有缓存（首次进入、整页刷新、登录失效后）时行为不变：显示骨架屏直到会话快照与设置返回。
- 会话列表在每个实时事件之后更新缓存，所以离开前看到的就是缓存内容。
- 令牌失效（`webmux:auth`）时清空全部缓存，不跨凭据复用。
- 不持久化到 `localStorage`，不改变后端、REST/WebSocket 协议和刷新时机。

## Capabilities

### New Capabilities

- `workspace-return-cache`：首页再次进入时先渲染上次内容并在后台刷新的规则，包括缓存范围、刷新覆盖、无缓存回退和清除条件。

### Modified Capabilities

无。

## Impact

- 前端：`web/src/repository-cache.ts` 改名为 `web/src/home-cache.ts` 并扩展为会话、设置与仓库三项缓存；`web/src/pages/SessionList.svelte` 读取与写入缓存；`web/src/App.svelte` 在令牌失效时清空。
- 测试：`web/test/e2e/frontend-quality.spec.ts` 新增“返回首页在设置读取挂起时仍显示上次列表”。
- 文档：`docs/user-stories/access-and-sessions.md` 的 US-C01、US-C02，`docs/usage.md` 首页说明，`docs/user-stories/verification-log-2026-11.md`。
- 无后端、协议或依赖变更。
