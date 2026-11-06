# Design

## Context

终端与首页是同一单页应用的两个路由，返回首页会重新挂载 `SessionList.svelte`。组件原先只在模块级缓存了仓库分组；会话列表 `sessions`、`loading` 和 `initializing` 每次都从空开始，骨架屏要等 `/ws/sessions` 的首个快照和 `/api/config` 都返回才消失。慢速链接上这是若干个往返。

## Goals / Non-Goals

**Goals:**

- 返回首页时，有缓存就立即展示，刷新在后台完成。
- 缓存的会话列表在刷新到达后被服务端数据完整替换。

**Non-Goals:**

- 不持久化到浏览器存储：整页刷新或新开标签页仍是冷启动。
- 不缓存启动配置（`/api/launch-configs`）：它只在创建会话和卡片预览中使用，现有逻辑已在加载期间保留旧预览。
- 不改变刷新时机、协议或后端。

## Decisions

### 1. 一个内存缓存模块 `home-cache.ts`

模块级变量保存 `repositories`、`sessions` 与 `setup`（默认目录、主机名、待安装更新、Git 与登记能力、能力错误文本），导出 `homeCache` 读写函数和 `clearHomeCache`。取代原先只保存仓库的 `repository-cache.ts`，调用方一次迁移，不留旧别名。

备选：`sessionStorage`。拒绝原因：会话列表含标题、目录等数据，写入存储扩大暴露面并需要版本与清理；而需求只是路由切换，内存足够。

### 2. 有缓存即视为已加载

组件初始化时：

- `sessions` 取缓存；`loading = !cache.sessions`。
- 设置字段取缓存；`initializing = !cache.setup`，`setupSettled = !!cache.setup`。

骨架屏条件 `loading || initializing` 不变，因此无缓存时仍显示骨架屏。

WebSocket 首个快照仍按原逻辑整体替换 `sessions` 并触发草稿对账与可恢复记录刷新；`/api/config` 返回后覆盖设置字段。刷新期间连接状态徽标仍按真实连接状态显示，所以用户能分辨缓存内容正在刷新。

### 3. 每次变更写缓存

`sessions` 在快照与每个增量事件后写入缓存；设置在 `/api/config` 成功后写入；仓库发现沿用原有写入。读取失败不写缓存，也不清缓存。

### 4. 令牌失效清空

`App.svelte` 在 `webmux:auth` 中原本调用 `clearRepositories()`，改为 `clearHomeCache()`。

## Risks / Trade-offs

- 缓存的会话可能已在离开期间被关闭或结束：直到首个快照到达前，列表会短暂显示过时的卡片。快照到达后整体替换，可接受；打开已移除的会话走现有的“会话不存在”提示。
- 缓存设置里的“待安装更新”可能过时：`/api/config` 返回后立即更新。
