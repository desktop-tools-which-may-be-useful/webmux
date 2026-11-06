# Tasks

## 1. 版本号与签名

- [x] 1.1 `internal/update` 的版本解析与比较支持 `X.Y.Z-dev.N`（无前导零），拒绝其他后缀；`Newer` 按 semver 预发布优先级比较。
  - 验证：单元测试覆盖 `0.10.2 < 0.10.3-dev.1 < 0.10.3-dev.2 < 0.10.3-dev.10 < 0.10.3 < 0.11.0-dev.1`、`-dev.0`、`-dev.01`、`-rc.1` 被拒绝。
- [x] 1.2 内置公钥分为 stable 与 dev 两组；验证清单后检查版本与密钥组匹配，`stable` 通道只信任 stable 组。
  - 验证：单元测试覆盖 dev 密钥签 `webmux 9.9.9` 被拒、stable 通道拒绝 dev 清单、dev 通道接受两组各自的版本。
- [x] 1.3 `cmd/internal/releasesign` 的 `sign` 按版本读取 `WEBMUX_RELEASE_SIGNING_KEY` 或 `WEBMUX_DEV_SIGNING_KEY`，`verify` 按版本选择密钥组；生成 dev 密钥并把公钥加入 dev 组。
  - 验证：Go 测试覆盖用错组签名时 `verify` 失败、开发版版本读取 dev 环境变量；`TestSignerLinksOnlyStandardLibrary` 仍通过。

## 2. 通道与清单发现

- [x] 2.1 新增通道读取：读 `/etc/webmux/update-channel`；缺失按 `stable` 且不警告；无法识别、不归 root 所有或可被组与其他用户写入时按 `stable` 并记录警告。
  - 验证：单元测试覆盖缺失、`stable`、`dev`、无法识别、权限不安全五种情况。
- [x] 2.2 `Client.Latest` 按通道读取清单：`dev` 同时读 `dev` 与 `latest` 并取较高版本，一个失败不影响另一个；`Fetch` 按版本选择产物来源与带版本号的文件名。
  - 验证：httptest 覆盖 dev 较高、正式版较高、dev 清单全部失败三种情况，以及开发版产物 URL。
- [x] 2.3 `PackageAsset` 为开发版生成 `X.Y.Z-dev.N` 的 deb/rpm 文件名（文件名不含 `~`）。
  - 验证：单元测试覆盖 deb 与 rpm、amd64 与 arm64。

## 3. CLI

- [x] 3.1 新增 `webmux update channel [stable|dev]`（Linux 与 macOS）：无参数显示，带参数要求 root，必要时创建 `/etc/webmux`，以临时文件加改名写入 0644 的文件。
  - 验证：`updatecmd_test.go` 覆盖显示、非 root 拒绝、写入与权限、无效参数退出码 2。
- [x] 3.2 `update check` 输出通道；`contract.UpdateCheckV1` 增加 `channel`，并更新 `docs/protocol/control-json.md`（缺少该字段按 `stable` 理解）。
  - 验证：JSON 输出测试包含 `channel`；切回 stable 且当前开发版更高时提示“正式版超过它后自动更新”。

## 4. 托盘

- [x] 4.1 `tray.json` 增加 `updateChannel`（缺省 `stable`），菜单增加“更新通道”单选；切换后立即检查，非发布构建禁用。
  - 验证：`internal/tray` 测试覆盖旧设置默认值、菜单状态与切换后发起检查。
- [x] 4.2 自更新按通道检查并下载 `webmux-<版本>-setup-windows-<架构>.exe`；便携版“下载托盘”按版本打开 `v<版本>` 或 `dev` 页面。
  - 验证：核心测试覆盖 dev 通道安装开发版、切回 stable 不降级、便携版链接。
- [x] 4.3 WSL 服务端安装按托盘版本从 `v<版本>` 或 `dev` 下载对应文件名，开发版文件已删除时给出明确原因；`install.go` 的可用性分类与 `compat.go` 的对应判定支持 `-dev.N`。
  - 验证：`install_test.go` 与 `compat_test.go` 覆盖开发版下载路径、文件已删除的报错、开发版之间与开发版对正式版的判定。
- [x] 4.4 更新入口：WSL 较新且为开发版、托盘通道为 stable 时不提供“更新托盘”、不自动检查，改为“切换到开发版通道并更新托盘”（设为 `dev` 后立即检查并按手动更新安装）；便携版按版本打开 `v<版本>` 或 `dev` 页面。
  - 验证：核心测试覆盖 stable 通道遇到开发版服务端时的菜单、点击后通道变为 `dev` 并安装开发版、便携版链接。

## 5. 发布流水线

- [x] 5.1 `Makefile` 支持开发版版本与带版本号的产物名（二进制、托盘、安装程序、deb/rpm 文件名用 `-dev.N`，包内版本由 nfpm 生成 `~dev.N`），`install.sh` 保持固定文件名并列入 `SHA256SUMS`；`deploy/windows/webmux.nsi` 的 `VIProductVersion`/`VIFileVersion` 只取 `-` 之前的数字部分。
  - 验证：本地以 `VERSION=0.10.3-dev.2` 运行发布构建，makensis 成功，产物名、deb/rpm 包内版本与 `SHA256SUMS` 符合 `release-distribution`。
- [x] 5.2 新增 `.github/workflows/dev-release.yml`（#266 新增，#308 改为发布后删除不在本次清单中的文件）：
  - 只处理 `test.yml` 对本仓库 main 推送的成功运行；以完整历史检出 `workflow_run.head_sha`；`N` 为 0 时跳过。
  - build 之前：若自标签 `dev` 所指提交以来只改了文档（以该提交中的 `.github/scripts/classify-changes.sh` 判定），跳过；标签不存在或不是祖先时不跳过。
  - plan 与发布前若该提交已不是 main 最新提交，或版本不高于 Release `dev` 当前清单的版本，跳过。
  - build 先运行 `pnpm --dir web build` 与 `go test -race`，再构建产物；build 按 `dev-release` 分组可取消；publish 按 `dev-release-publish` 分组不取消，sign 不入组；隔离与正式版相同。
  - publish 按“上传带版本号的产物、替换 `install.sh` 与清单、移动标签、删除不在本次清单中的文件”的顺序操作 Release `dev`，标记为预发布且不设为 latest。
  - 验证：`actionlint` 通过；在测试仓库或 fork 中连续推送两次，Release `dev` 只剩最后一次构建的产物与 `install.sh`，`latest` 不变；手动重跑较早提交的运行时跳过发布；只改 Markdown 的推送不构建也不发布。
  - 实际运行记录：本仓库 main 上该工作流持续运行，Release `dev` 为预发布且不是 latest，只含当前一组 `0.12.2-dev.N` 带版本号产物、`install.sh`、`SHA256SUMS` 与 `SHA256SUMS.minisig`；只改 OpenSpec 文档的提交 `0ca063e0` 对应运行（42153787054）只执行 plan，build、sign、publish 均为 skipped。
- [x] 5.3 `release.yml` 生成发布说明时以 `--notes-start-tag` 指定上一个 `v*` 标签（排除带 `-` 的预发布标签）。
  - 验证：`actionlint` 通过；在 fork 中存在 `dev` 预发布时打正式标签，发布说明从上一个 `v*` 标签开始。
- [x] 5.4 `install.sh` 支持 `WEBMUX_CHANNEL=dev|stable`：开发版从 Release `dev` 的 `SHA256SUMS` 找出唯一匹配的带版本号二进制；`WEBMUX_VERSION` 接受 `X.Y.Z` 或当前开发版版本，通道与版本矛盾时报错。
  - 验证：在本地测试 Release 上安装开发版与指定当前开发版，`webmux version` 报告开发版版本；无效取值、已过期的开发版版本、通道与版本矛盾都报错且不改动文件。

## 6. 文档与用户故事

- [x] 6.1 更新 `docs/deployment/updates.md`（通道、切换、通道文件权限、dev 公钥与轮换、正式标签必须在 main 上、移动 `dev` 标签的影响、只改文档不发布）、`install-and-upgrade.md`（开发版安装地址 `releases/download/dev/install.sh`）、`windows-wsl.md`（切换到开发版通道并更新托盘）。
  - 验证：`make docs-check` 通过。
- [x] 6.2 按 `docs/agent-rules/user-stories-maintenance.md` 新增或更新用户故事（命令行通道、托盘通道、开发版发布），同步两个索引与核对记录。
  - 验证：故事 ID 唯一，`make docs-check` 通过。

## 7. 集成验证

- [x] 7.1 `make check` 与 `make test-fast` 通过。
- [ ] 7.2 端到端：以本地签名的测试 Release 在 systemd 容器中把 deb 从正式版切到开发版再切回，验证升级、不降级与正式版追上；在 Windows 上验证托盘切换通道后自更新到开发版。
  - 2026-11-06 已完成部分（使用真实 GitHub Release，而非本地签名的测试 Release；Debian bookworm systemd 容器）：
    - 安装 `webmux_0.12.1-1_amd64.deb`，`webmux update channel dev` 写入 `0644 root` 的 `/etc/webmux/update-channel`。
    - `systemctl start webmux-update.service` 下载并校验 `webmux_0.12.2-dev.8-1_amd64.deb`，升级到 `0.12.2-dev.8`（包版本 `0.12.2~dev.8-1`）。
    - 切回 `stable` 后检查与定时任务均不降级，提示“当前开发版 0.12.2-dev.8 高于最新正式版 0.12.1，正式版超过它后自动更新”。
  - **待验收**：“正式版追上”需要高于当前开发版的正式版（或可替换下载源的测试 Release，`internal/update` 目前没有此入口）；Windows 托盘切换到 dev 通道后自更新到开发版。

## Workflow follow-up

- 维护者生成 dev 签名密钥并配置 Actions 密钥 `WEBMUX_DEV_SIGNING_KEY`（已完成：Release `dev` 带有 `SHA256SUMS.minisig`）。
- 审查通过后归档本变更。
