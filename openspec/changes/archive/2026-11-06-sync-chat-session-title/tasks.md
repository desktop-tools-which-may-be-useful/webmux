# Tasks

## 1. 首条消息临时标题（internal/hybrid）

- [x] 1.1 新增 `title.go` 的 `promptTitle`：去除控制字符，折叠空白，超过 80 个 rune 时截断为 79 个加“…”。验证：`TestPromptTitle`。
- [x] 1.2 `actor` 增加 `promptTitle` 标志：仅在新建 agent 会话（`started` 走 `session/new`）时置位；`send` 首次发送时，在没有 agent 标题且文本非空的条件下设置聊天端标题；收到 agent 标题后清除标志。验证：`TestFirstPromptTitle`、`TestFirstPromptTitleSkipped`（已命名、只有附件、从终端切到聊天）。

## 2. 端到端

- [x] 2.1 在 `web/test/e2e/hybrid.spec.ts` 增加用例：未命名的聊天会话发送第一条消息后标题为该文本，随后变为 agent 标题，关闭后“可恢复”卡片显示 agent 标题。

## 3. 用户故事

- [x] 3.1 更新 US-Q02 的验收要点，同步 `docs/user-stories.md` 与 `docs/user-stories/coverage.md`。验证：`make docs-check`。
