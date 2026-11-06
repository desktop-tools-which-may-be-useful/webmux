# 核对记录：脚本与 agent 控制（2026-11-05 起）

本文件记录 [脚本与 agent 控制](shell-control.md)（S 组）及其关联故事的局部核对，条目按日期升序。前一份记录见 [verification-log-2026-11](verification-log-2026-11.md)。

本次局部核对（2026-11-05）：实现 OpenSpec 变更 `add-agent-shell-control` 的第一部分（会话发现、会话元数据与帮助框架），新增 S 组 US-S01–US-S04，更新 US-M01、US-M05 的命令帮助与 `ls` 筛选说明及覆盖索引。

- 范围：
  - 会话环境注入 `WEBMUX_SOCKET` 与 `WEBMUX_BIN`（聊天端 ACP 进程共用同一入口），二者与 `WEBMUX_SESSION_ID` 一样保留；
  - CLI 的 `--socket`、`self`、退出码 124 常量与版本过旧提示，本机状态新增 `bin`；
  - `GET /api/sessions/{id}/details`、`PATCH /api/sessions/{id}/labels`，创建请求的 `labels`，本机套接字创建时自动记录 `parentId`；
  - `webmux get`、`webmux label`、`ls --label/--parent/--status`；
  - 命令表、`--help`、`help --json`，运维命令迁入命令表，选项可写在位置参数之后。
- 依据：
  - `go test -race` 覆盖以下包：
    - `internal/session`：标签校验与原子修改、父会话、环境注入、进程所属会话；
    - `internal/api`：本机创建记录父会话与标签、TCP 不记录、非法标签不创建、详情列出子会话，并重录 wire 快照；
    - `internal/sessioncmd`：get、label、ls 筛选、交错解析、旧服务端提示、帮助输出；
    - `cmd/webmux`：帮助写到标准输出、分组与用法错误、运维命令选项与命令表一致、套接字定位优先级；
  - `internal/archtest`、`internal/docguard`、`golangci-lint`（v2.15.0）通过；Windows 客户端交叉编译通过。
- 限制：
  - 本地 git 2.43 输出的提交时间格式使 `TestWireSnapshots` 的 `repository-commits` 快照在本机不一致，未修改该快照，以 CI 结果为准；
  - 未在 macOS 上运行；未在原生 Windows 终端中执行帮助命令；Web 端不显示父会话与标签。

本次局部核对（2026-11-05）：实现 `add-agent-shell-control` 的第二部分（llms.txt 手册与 `{{webmux}}`），新增 US-S05、US-S06，更新 US-G06、US-J12 的初始化指令说明与覆盖索引。

- 范围：
  - 新增共享包 `internal/agentguide`：`/llms.txt`、`/llms/<主题>.md`、`/llms-full.txt` 只在内存中渲染，命令参考由命令表生成；TCP 上不含实例信息，本机套接字附加版本、可执行文件与套接字；
  - `POST /api/sessions/{id}/agent-guide` 返回 `{{webmux}}` 说明；初始化指令先展开 `{{dir}}` 再插入说明，说明不可用时预检 503；
  - CLI 新增 `webmux guide`（服务未运行时输出内嵌副本）与 `webmux dir`；
  - 前端：快捷指令支持 `{{webmux}}`，预设“临时文件夹”换为“WebMux 工具”，初始化指令“可插入”新增按钮，快速开始预览说明 `{{webmux}}`，移除保留变量提示。
- 依据：
  - `go test -race` 覆盖 `internal/agentguide`（索引结构、链接、公开内容无实例信息、简述）、`internal/api`（手册路由与 Host 检查、说明接口；wire 快照新增 `session-agent-guide`）、`internal/workspace`（说明插入顺序与预检）、`internal/sessioncmd`（guide 与 dir）、`cmd/webmux`（命令参考与命令表同步）；
  - 前端单元测试 `quickCommands.test.ts`，以及受影响的浏览器与 E2E 用例（预设菜单、快捷指令、临时文件夹）。
- 限制：未在 macOS 与真机浏览器上核对；以前保存的“临时文件夹”预设不迁移。
