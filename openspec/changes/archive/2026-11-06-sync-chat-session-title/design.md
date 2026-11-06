# Design

## Context

动机见 `proposal.md`。与实现相关的现状：

- agent 上报的标题（ACP `session_info_update`）由 `actor.emit` 写入 `actor.title`，经 `session.Overlay.Title` 成为未命名聊天会话的标题，`saveShellTitle` 把显示出来的标题写入可恢复记录。只有当标题属于当前显示的 agent 会话（`titleSession == agentSessionID`）时才发布。
- 终端端的标题仍由 TUI 的 OSC 0/2 决定，覆盖层不发布标题。
- 用户创建时指定的名称（`userTitle`）始终优先于覆盖层标题。

## Decisions

- 首条消息临时标题复用 `actor.title` 与 `titleSession`，不新增覆盖层字段、记录字段或发布通道：`send` 在第一次发送时把清洗后的消息文本写入 `actor.title` 并把 `titleSession` 设为当前 agent 会话，下一次 `flush` 经覆盖层发布，`saveShellTitle` 随即写入记录。
- `promptTitle` 标志只在新建 agent 会话（`started` 的 `session/new` 路径）时置位，因此恢复、加载和从终端切换而来的会话不会使用临时标题。收到 agent 标题后清除标志，之后清除标题只回到目录名。
- 用户命名由 `session` 的 `userTitle` 优先级保证，`actor` 不需要另存命名状态。
- 清洗只做展示层处理：去除控制字符，折叠空白，最多 80 个 rune。agent 标题的清洗沿用 `translate.go` 的 `sessionTitle`。

## Risks

- 临时标题来自用户消息文本，会写入 `hybrid-sessions.json` 的 `title`。该字段此前已保存 agent 标题；记录仍不含环境变量值、消息正文或令牌之外的内容，临时标题只有首条消息的前 80 个字符。
