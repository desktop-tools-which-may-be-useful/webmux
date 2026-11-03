# Tasks

## 1. OSC 7501 子记录段长、agy 档案与协议适配

- [x] 1.1 将 `internal/session/osc.go` 中的 `maxProgramIDSegment` 从 `32` 调整为 `64`，
  并在 `internal/session/osc_test.go` 与 `internal/session/attention_test.go` 中补充
  36 字符 UUID 子记录 `id` 的提取与不泄露测试，运行 `go test -race ./internal/session` 验证通过。
- [x] 1.2 在 `internal/agentprofile` 中支持按参数配置 `PassToACP`、多 token `TUIInject`，
  对照 `design.md` 决策 1–3 与 `baseline.md` 新增 `internal/agentprofile/agy`
  （Antigravity CLI）适配器（可执行文件 `agy`、实测版本 `1.4.12`、单次 `Esc` 中断键、
  `ResumesSessions: true`、`ReapplyOnLoad: []string{"model", "thought_level"}`、
  `--conversation` 恢复参数、`--settings` `title` 钩子注入、白名单/丢弃/拒绝参数表）与
  `Identify` 对 `app=antigravity` + 36 字符小写 UUID 子记录 `id` 的识别。
- [x] 1.3 在 `internal/agentprofile/agents` 注册 `agy.Agent`，并在
  `internal/agentprofile/agy/agy_test.go` 与 `internal/agentprofile/agents/agents_test.go`
  中覆盖 `agy` 的命令分类、`TUIArgv`、`ResumeArgv`、`ACPArgv`、身份识别与版本比较，
  运行 `go test -race ./internal/agentprofile/...` 验证通过。
- [x] 1.4 在 `internal/hybrid/actor.go` 的 `onUpdate` 中忽略活动回合（`a.turn != nil`）
  期间收到的 `*acp.UserMessageChunk` 通知（保留 `session/load` 期间 `a.turn == nil`
  的用户消息回放），并在 `internal/hybrid` 测试中验证实时回合回显不产生重复用户消息且加载回放正常。
- [x] 1.5 对照 `baseline.md` 扩展 `internal/acp/testdata/fakeagent` 支持 `--profile agy`
  （及以可执行文件名 `agy` 启动时自动选定该档案、识别 `--conversation` 与 `--settings`、
  发出 `app=antigravity` 与 UUID 子记录、`session/prompt` 回显 `user_message_chunk`、
  跨进程加载重置 `model`/`thought_level`、单次 `Esc` 中断），并在
  `internal/acp/fakeagent_test.go` 与 `internal/hybrid/agy_test.go` 中增加 `agy`
  双向切换与配置补齐用例，运行 `go test -race ./internal/acp ./internal/hybrid` 验证通过。

## 2. 启动配置预置、图标与前端适配

- [x] 2.1 在 `internal/launch/presets.go` 与 `internal/launch/types.go` 中将新安装预置的
  `Gemini CLI` 替换为 `Antigravity CLI`（`agy --dangerously-skip-permissions`，
  图标 `antigravity`，`Hybrid: true`），同时允许 `antigravity` 与旧 `geminicli` 图标值。
- [x] 2.2 更新 `internal/launch/store_test.go` 与 `internal/api/launch_test.go`
  （并运行 `go test ./internal/api -run TestWireSnapshots -update` 更新线上快照），
  运行 `go test -race ./internal/launch ./internal/api ./internal/wire` 验证通过。
- [x] 2.3 添加 `web/src/icons/lobehub/antigravity.svg`（来自固定提交
  `eb3e353a5cf3720ef83d52058966ed51b10f10fa` 的 `lobehub/lobe-icons`），更新
  `web/src/components/LaunchConfigIcon.svelte` 与 `web/src/launchConfigs.ts`
  （图标选项 `Antigravity CLI`、`agy` 自动匹配 `antigravity`、保留 `gemini`/`geminicli` 兼容渲染）。
- [x] 2.4 在 `web/test/launchConfigs.test.ts` 与 `web/test/browser/LaunchConfigDialog.test.ts`
  中更新图标与预设断言，运行 `pnpm --dir web check`、`pnpm --dir web check:test`、
  `pnpm --dir web lint`、`pnpm --dir web check:arch` 与定向 Vitest 用例验证通过。

## 3. 用户故事、文档与全量门禁

- [x] 3.1 更新 `docs/user-stories.md`、`docs/user-stories/access-and-sessions.md`
  （US-B01、US-B08）、`docs/user-stories/hybrid-sessions.md`（US-Q01、US-Q08、US-Q11）、
  `docs/user-stories/coverage.md`、`docs/protocol/http-sessions.md`、`docs/cli-agents.md`、
  `docs/hybrid-sessions.md`、`docs/design-system.md` 与 `docs/usage.md`，运行
  `make docs-check` 与 `make format-check` 验证通过。
- [x] 3.2 运行 `openspec validate "add-antigravity-acp-hybrid" --strict` 与 `make check`
  验证所有规格、类型、架构守卫与代码检查通过。
