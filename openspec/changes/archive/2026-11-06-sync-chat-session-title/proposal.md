# Proposal

## Why

从聊天开始的混合会话在 agent 上报标题之前只显示目录名（例如 `fair-harbor`）。omp 等 agent 只在回合结束时才上报标题，新建的聊天会话在第一轮完成前没有任何可辨识的名字，多个聊天会话无法区分。agent 上报标题后的显示与记录已由既有的标题持久化能力覆盖，本变更只补上第一轮之前的空白。

## What Changes

- 从聊天开始、由 webmux 新建 agent 会话的混合会话，在用户未命名且 agent 尚未上报标题时，以第一条消息的文本作为临时标题（去除控制字符、折叠空白、最多 80 个字符）。agent 上报标题后被替换，清除标题后回到目录名而不是再用临时标题。
- 临时标题经既有的会话标题写入可恢复记录，“可恢复”卡片因此同样显示它。

## Capabilities

### New Capabilities

（无）

### Modified Capabilities

- `hybrid-sessions`：新增“首条消息临时标题”要求。

## Impact

- 后端：`internal/hybrid`（`actor` 的首条消息临时标题，新增 `title.go`）。
- 前端、线上契约与状态文件：不变。
- 用户故事：更新 US-Q02 并同步覆盖索引。
