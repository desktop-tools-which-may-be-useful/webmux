## 1. 默认 Shell 解析与协议字段

- [x] 1.1 在 `internal/agentprofile` 或合适的共享位置增加 `$SHELL` basename 解析：`bash` → bash、`zsh` → zsh、空 → bash、其他 → 空；补表驱动测试
- [x] 1.2 `contract.ServerConfig` 增加 `DefaultShell string`，`GET /api/config` 返回该字段；`web/src/schemas.ts` 的 `ConfigSchema` 增加带空字符串默认值的可选字段
- [x] 1.3 更新 `internal/api` 的 wire 快照与契约测试；确认字段不含路径或环境变量值

## 2. Agent 档案的包装形式

- [x] 2.1 把 `FormBashrc` 泛化为启动文件包装并携带 shell；`splitForm` 同时识别 `bash -ic` 与 `zsh -ic` 的精确前缀，其他 shell 包装仍不可用
- [x] 2.2 `wrapExec`、`TUIArgv`、`ResumeArgv`、`ACPArgv`、`Lookup`、`ProtocolOnFD3` 跟随命令的 shell；保留共享脚本与 `webmux` 标记
- [x] 2.3 更新不可用原因文案为“加载 shell 启动文件”，补 Bash/Zsh 参数表、恢复 argv、ACP argv、fd 3 与 `command -v` 的表驱动测试
- [x] 2.4 在 `internal/hybrid/connector.go` 把 Bash 专用分支改为启动文件包装分支，连接器检查经配置的 shell 执行；补 `.zshrc` 有输出、连接器缺失与退出码 127 的测试

## 3. 新安装预置

- [x] 3.1 `internal/launch/presets.go` 按解析出的默认 shell 生成 agent argv：bash/zsh 包装，空字符串直接执行；Shell 配置保持 `$SHELL -l -i` 与 `/bin/sh` 回退
- [x] 3.2 确认 `cmd/webmux/main.go` 仍把 `SHELL` 传入预置；补 Bash、Zsh、未设置、fish 四种预置断言
- [x] 3.3 补既有 `launch-configs.json` 不被改写、版本仍为 2 的回归测试

## 4. Web 编辑器

- [x] 4.1 `web/src/launch-configs.ts` 用带 shell 的包装前缀替换 `BASHRC_PREFIX`；`launchCommandDraft` 返回普通命令与所选 shell，新增统一包装函数，图标自动匹配复用同一识别
- [x] 4.2 `LaunchConfigDialog.svelte` 把复选框改为“加载 shell 启动文件”，勾选时显示 Bash（`~/.bashrc`）/ Zsh（`~/.zshrc`）选择；创建时用 `defaultShell || "bash"`，编辑时按 argv 还原
- [x] 4.3 更新校验文案、参数上限提示与无障碍名称；取消勾选恢复直接命令，保存不重复包装
- [x] 4.4 补 `web/test/launch-configs.test.ts` 与 `web/test/browser/LaunchConfigDialog.test.ts` 的 Bash/Zsh 往返、默认值、切换与旧配置用例
- [x] 4.5 运行 `pnpm --dir web check:arch`

## 5. 文档与用户故事

- [x] 5.1 更新 `docs/cli-agents.md`、`docs/usage.md`、`docs/usage/terminal.md`、`docs/hybrid-sessions.md` 与 `docs/deployment.md` 的启动文件说明、macOS zsh 默认、其他 shell 回退与不迁移
- [x] 5.2 按实际接口变化更新 `docs/protocol/http-sessions.md` 与 `docs/protocol/http-hybrid.md` 的 `defaultShell` 与包装写法
- [x] 5.3 更新 US-B01、US-B08 的验收要点，同步 `docs/user-stories.md` 与 `docs/user-stories/coverage.md` 的索引与计数

## 6. 验证

- [x] 6.1 运行 `go test -race ./internal/agentprofile ./internal/launch ./internal/hybrid ./internal/api ./internal/wire`
- [x] 6.2 运行相关前端单元与浏览器测试、类型检查、lint 与生产构建
- [x] 6.3 按 `docs/agent-rules/testing.md` 在隔离实例核对 Bash/Zsh 预置、连接器探测与旧配置不变，并准备 UI 截图证据
