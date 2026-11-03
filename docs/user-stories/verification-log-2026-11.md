# 核对记录：2026-11-01 至 2026-11-03

本文收录 2026-11-01 至 2026-11-03 各次局部核对的记录：对应的故事、已运行的验证、范围限制与未核对的部分。

条目由[用户故事索引](../user-stories.md)原文迁入，按日期升序排列，措辞保持当时的口径；故事的当前内容以各分组文档为准。

其他时间段见[verification-log-2026-10-23-to-26.md](verification-log-2026-10-23-to-26.md)、[verification-log-2026-10-27-to-31.md](verification-log-2026-10-27-to-31.md)。

本次局部核对（2026-11-01）：基于上游 `76d82fd` 整合九款 CLI 入口，恢复 Gemini 默认启动配置，并新增 Cursor、Copilot、Qwen、Kimi、CodeBuddy，对应 US-B01、US-B08。

- 通过：53 项启动配置单元、21 项浏览器组件、6 项桌面/Pixel 7 E2E，相关 Go race、固定版本 Go lint、前端类型、架构、文档治理和生产构建检查。
- 隔离生产实例以替身 CLI 验证九款配置的命令、参数边界与退出码，并重拍桌面/393px 手机的浅色和深色对比图；未使用真实账号或调用模型。
- Gemini 认证方式见 [接入说明](../cli-agents.md)，保留入口不承诺账号套餐或额度；既有启动配置仍不补写或覆盖。
- 核对了 174 个故事 ID、两份索引及 B 组来源路径；未重新验收其余功能组或真实手机键盘。

本次局部核对（2026-11-01）：#220 代码工作区资源管理器的右键文件操作（上传、下载、目录 ZIP、新建文件夹、重命名、复制、剪切、粘贴、删除）对应新增 US-I19，并更新 US-I18（临时文件夹改为可写）与 US-A05（代码接口含文件操作）；覆盖索引同步。未重新核验其余故事能力。

本次局部核对（2026-11-01）：#220 补充移动端文件操作（长按、底部操作面板、触控尺寸、剪贴板提示条、重命名预选文件名），对应更新 US-I19；Playwright 桌面与 Pixel 7 用例覆盖长按与操作面板。未重新核验其余故事能力。

本次局部核对（2026-11-01）：独立代码窗口改为在窗口内用同源 iframe 运行独立的代码工作区，并新增代码工作区拖放上传，对应更新 US-I13、US-I19。未重新核验其余故事能力。

- 行为：弹出、收回与直接关闭窗口保留完整阅读状态；窗口内右键菜单、对话框与底部面板不再打开在主窗口。
- 依据：`CodePanel`、各阅读组件与导航的浏览器测试，以及 `code-popout.spec.ts` 的桌面 E2E（画中画、普通弹窗、拖放上传）。

本次局部核对（2026-11-01）：#208 的 Codex CLI 混合模式与第三方 ACP 连接器安装检查对应新增 US-Q16，并更新 US-Q01、US-Q03、US-Q08，覆盖索引同步。

- 依据：`internal/agentprofile`、`internal/hybrid`（含 `codex_test.go` 的往返、连接器缺失与安装后重检、启动文件检查、外部占用、插话退回队首、启动静默期）与 `internal/session` 的 Go race 测试，线上快照与 wire 契约用例，浏览器组件测试，以及桌面/Pixel 7 的 Codex E2E 用例。
- 真实工具实测：Linux 上 Codex CLI 0.166.0 与 codex-acp 2.5.0 的写锁、`session/load` 配置重置、`thread_active_writer`、插话 `idleBehavior` 与权限选项（结论记录在 OpenSpec 变更的 design.md）；未在 macOS 上用真实 Codex 验收。
- 一致性：175 个故事 ID 唯一；未重新核验其余故事能力。

本次局部核对（2026-11-01）：#222 把 agent 差异收拢到 `internal/agentprofile` 的适配器接口与每个 agent 一个子包，对应更新 US-Q01（不支持原因与勾选框说明列出服务端返回的受支持 agent）、US-Q03（强制切换损失说明由服务端下发）与 US-L05（agent 名称守卫）。未重新核验其余故事能力。

- 依据：`internal/agentprofile/...` 的适配器契约测试与命令解析测试、`internal/archtest`、`internal/hybrid` 与 `internal/api` 的 Go 测试、重录的线上快照，以及前端单元与浏览器组件测试。

本次局部核对（2026-11-01）：#227 重连期间恢复“正在恢复…”提示，对应更新 US-D03 与 `terminal-stream-resume` 规格。可续接终端在重连时显示不遮挡内容的提示，连接恢复后消失，已有内容保持可见。

- 通过：定向 `TerminalReplay` 与 `TerminalViewport` 浏览器用例、类型检查、测试类型检查、Oxlint、Oxfmt、前端架构守卫与文档治理。
- 未重新核验其余故事能力。

本次局部核对（2026-11-01）：`AGENTS.md` 体积预算对应新增 US-L11；`AGENTS.md` 由约 22 KB 压缩到 10 KiB 以内，频率较低的规则原文迁入 `docs/agent-rules/`。

- 依据：`go test -count=1 ./internal/archtest -run TestAgents` 与 `go test -count=1 ./internal/docguard`。
- 一致性：177 个故事 ID 唯一，两份索引已同步 US-L11；未重新核验其余故事能力。

本次局部核对（2026-11-01）：调整尺寸重绘风暴对应 US-D01；新增文字与 Kitty 图片混合的整屏重绘模拟程序与 4x CPU 性能用例。

- 依据：`go test -race ./internal/hub ./internal/api`（新增突发输出合并回归：200 块由 200 帧降为约 16 帧）、`kitty-graphics` 单元与 `TerminalGraphics` 浏览器用例。
- 本机性能用例（DOM 渲染器）：每次风暴 WebSocket 消息 113→13，最大帧间隔 1.7 s→0.18 s，时长 4.6 s→约 2.5 s；未测 WebGL；未重新核验其余故事能力。

本次局部核对（2026-11-01）：#239 恢复时闪屏对应 US-D01、US-D03（同步输出与回放期间文字与 Kitty 图片同帧切换）。

- 依据：新增 `TerminalSyncOutput` 浏览器用例（同步重绘、滚动重绘、快照回放、同步期间 resize 与焦点变化、快照停在图片上传中），修复前均失败；全部浏览器与单元用例、终端相关 E2E 通过。
- 本机重绘风暴性能用例（DOM 渲染器、4x CPU、同步输出）：可见重绘帧 37→2–3，长任务 2.3 s→0.33 s；未测 WebGL；未重新核验其余故事能力。

本次局部核对（2026-11-02）：聊天回合由服务端记录开始与完成时间，进行中和完成后的耗时跨刷新、重连与设备保持一致，对应更新 US-Q05；不新增或重编号故事；文档守卫不再豁免表格行，对应更新 US-L08，故事表格改为列表，ID 与验收内容不变；超限整理新增“重构而不是压缩”原则。

本次局部核对（2026-11-02）：单文件阅读迁移到 `@pierre/diffs` 只读 `File`/`CodeView`，与 Diff 共用渲染器、主题和高亮 worker；复核 US-I04、US-I05、US-I09、US-I13，并更新 US-I04、US-I05 的验收与来源。

- 已运行：svelte-check、前端单元与浏览器测试、`check:arch`、`codeview`/`code-popout`/`code-documents`/`compact-chrome` E2E 及大型代码文件性能用例。
- 范围限制：未重新核对其余功能组。

本次局部核对（2026-11-02）：统一会话创建入口、当前仓库默认目标、工作树独立创建及分屏完整表单，对应 US-B01、US-B03、US-B06、US-D13、US-J03、US-J06；验证范围见本次 PR，不代表重新核对其余故事。

评审补充（2026-11-02）：同步使用指南与工作区创建规范；分屏中新建工作树后启动失败，切换仓库仍可重选并重试，对应 US-D13。定向验证见 PR，未重新核对其余故事。

测试补充（2026-11-02）：修复代码根识别被取消后缓存普通目录的问题，对应 US-I13；同时完善弹出窗口与终端输入用例的同步条件。验证范围见 PR，未重新核对其余故事。

本次局部核对（2026-11-02）：`add-claude-code-hybrid-profile` 的 Claude Code 混合档案对应更新 US-Q01、US-Q03、US-Q04、US-Q06–US-Q08、US-Q10、US-Q12、US-Q14–US-Q16；不新增或重编号故事，覆盖索引同步。

- 内容：新安装默认混合、Node 22+ 与 `claude-agent-acp`、现有认证复用、进程注册身份与候选、外部占用、配置恢复、插话/附件及后台进程边界。
- 依据：变更 design/delta specs 中固定的 Linux 原生 Claude Code 2.1.316、连接器 0.99.1 实测基线，以及当前适配器与身份实现；本次未重新运行真实双向验收。
- 限制：npm 启动器、macOS 与订阅登录不属于该实测矩阵；未重新核对其余故事能力。

本次局部核对（2026-11-02）：选择 Bash / Zsh 启动文件与只读 `defaultShell` 对应 US-B01、US-B08；后端定向 Go race 测试覆盖 Bash/Zsh 包装、恢复、ACP fd 3 与连接器检查，前端由 `launchConfigs` 单元测试与启动配置对话框浏览器测试覆盖；未重新核对其余故事。

本次局部核对（2026-11-02）：可恢复混合会话持久化并展示当前会话标题，旧记录回退到创建名称，对应 US-Q12，不新增故事或改变覆盖索引。

- 通过：标题持久化与恢复、API/wire Go race、线上快照、Zod 契约及可恢复分组浏览器测试。
- 其余混合会话 Go race 在排除 `TestClaudeRoundTrip` 后通过；该用例在未修改的 HEAD 同样失败。未重新核对其余故事。

本次局部核对（2026-11-02）：Windows 安装程序对应新增 US-O12，并更新 US-O01（本地缓存与预置服务端离线安装）与 US-O04（安装程序登记登录启动、`webmux-tray.exe quit`）；覆盖索引同步。
核对依据为 `internal/tray` 的 Go race 测试与在 Windows 11 上实际运行的 `windows-installer-smoke.ps1`（静默安装、`quit`、运行中升级、拒绝其他位置的托盘、架构不符、卸载）；178 个故事 ID 唯一，O 组来源路径有效；未用真实 WSL 发行版验收托盘菜单离线安装，未重新核验其余故事能力。

本次局部核对（2026-11-02）：Linux deb / rpm 安装包对应新增 US-K13，覆盖索引同步。

- 依据：`internal/service` 的 Go 测试（包内单元的状态、卸载与无单元错误），以及 `test-linux-packages.sh` 在 Debian 与 Fedora 容器中的安装、检查与卸载。
- 实测：在本机 systemd 用户管理器上把同内容单元放在非 `~/.config` 的用户单元目录，`webmux service status|restart|stop|start|uninstall` 均作用于该单元，卸载后文件保留。
- 实测：在以 systemd 为 PID 1 的 Debian 与 Fedora 容器中为普通用户启用 linger 后安装 0.0.1 包。
  - 安装后服务保持未启用；`systemctl --user enable --now webmux` 启动并生成配置。
  - 升级到 0.0.2 时服务进程不变，`webmux service restart` 后运行新版本。
  - `webmux service uninstall` 禁用并停止且保留单元文件；卸载包后两项文件删除、用户配置保留。
- 限制：未验证 arm64 包的安装；未重新核验其余故事能力。

本次局部核对（2026-11-02）：文档整理，不改变任何故事的内容，故事 ID 与验收内容不变，未重新核对其余功能组。

- 迁移：`docs/user-stories.md` 中的核对记录原文迁入本目录的三个核对记录文件，索引只保留产品边界、功能索引与分组表；`docs/deployment.md` 按主题拆到 `docs/deployment/`，章节标题与锚点不变，外部链接已更新。
- 依据：对应 US-L08 新增的“超限时重构而不是压缩”整理原则。
- 验证：已运行 `make docs-check`、Oxfmt 检查、迁移前后的逐行对账与相对链接/锚点检查。

本次局部核对（2026-11-02）：文档导航治理与 README 重写，对应新增 US-L12，覆盖索引与功能索引同步。

- 迁移：README 中的设计理念、完整特性、混合会话说明、截图生成说明与技术栈原文迁入 `docs/overview.md`，常见问题迁入 `docs/faq.md`。
  - 手动下载步骤迁入 `docs/deployment/install-and-upgrade.md#手动下载`，从源码构建迁入 `CONTRIBUTING.md#从源码构建`。
  - 配置项表、服务命令、Windows 托盘步骤与安装脚本变量已由 `docs/deployment/` 完整覆盖，README 中只保留链接。
- 新增：`docs/README.md`（文档中心）与 `docs/specs.md`（64 个 capability 的规格索引），原先不可达的 `docs/performance.md`、`docs/page-performance.md`、`docs/binary-size.md` 与全部主规格由此可达。
- 修正：`docs/persistence.md` 指向已归档变更的失效链接；`docs/cli-agents.md` 与 `docs/usage.md` 中“只有 OpenCode / omp 默认混合”的过时描述，按 `internal/launch/presets.go` 改为 Claude Code、Codex CLI、OpenCode 与 omp。
- 验证：已运行 `make docs-check`（含新增的 `TestDocsNavigation`、`TestNavigationRules`、`TestSlug`）与 Oxfmt 检查；未重新核对其余故事。

本次局部核对（2026-11-02）：README 首屏改版，对应 US-L03（新增 `screenshots:hero` 合成主图）与 US-L12（README 预算不变）。

- 首屏改为合成主图、一句话主张、使用场景叙述、“三步看懂”截图流程与六项亮点；README 仍为 170 行以内。
- 验证：以预装 Chromium 运行 `pnpm --dir web screenshots:hero` 生成 `docs/images/hero.jpg`，并用本地 Markdown 渲染截图目视检查首屏；已运行 `make docs-check` 与 Oxfmt 检查。未在 GitHub 页面上实际渲染。

本次局部核对（2026-11-02）：重新生成 README 截图与主图，截图脚本新增回放模式，对应 US-L03。

- 录制：以真实 omp 18.15.0（Opus 5.5）运行 `WEBMUX_SCREENSHOTS_RECORD=1` 流程，导出两轮对话到 `web/scripts/screenshots/omp-session.jsonl`；文件只含演示仓库内容。
- 回放：在未设置任何 API key 的环境中运行 `pnpm --dir web screenshots`，十张截图全部生成；再运行 `screenshots:hero` 更新 `docs/images/hero.jpg`。
- 发现：经 npm/Bun 安装的 omp 进程名为 `bun`，混合会话判定为 `not_root`，聊天切换不可用；本次用以 `omp` 命名的 Bun 运行时启动同一安装包绕过。产品侧的识别规则未改动。
- 限制：截图在 Linux 容器中以预装 Chromium 生成，未在 macOS 上运行脚本。

本次局部核对（2026-11-03）：自动更新新增 US-K14–US-K16、US-L13、US-O13，并更新 US-K13、US-O01、US-O11、US-O12；功能索引与覆盖索引已同步。发布签名故事最初编号为 US-L12，合并 main 时与先合入的“文档导航与首页”冲突，改为 US-L13；合并后 187 个故事 ID 唯一。

- 依据：`internal/update`、`cmd/webmux`、`cmd/internal/releasesign`、`internal/api`、`internal/tray` 的 Go race 测试与前端 `pendingUpdate` 用例。
- 安装包：`test-linux-packages.sh` 在 Debian/Fedora 容器中覆盖定时器的首次启用、带标记与无标记升级、卸载；systemd 容器中定时器服务把 0.0.1 升到 0.0.2，用户服务 PID 与会话不变。
- 签名：minisign 0.12 验签通过；签名步骤在空模块缓存且 `GOPROXY=off` 时可完成。
- 限制：托盘自更新与 NSIS 静默升级只在 Linux 上测试与编译（makensis 3.11），未在 Windows 上运行；未重新核验其余故事能力。

本次局部核对（2026-11-03）：PR #260 第二轮审查修复，更新 US-K14、US-K15、US-K16、US-O04、US-O12、US-O13，不新增故事，覆盖索引不变。

- 依据：`internal/update`、`internal/fetch`、`internal/tray`、`cmd/webmux` 的 Go race 测试，`internal/tray` 与 `cmd/webmux-tray` 的 Windows amd64/arm64 交叉 vet，`CodeReader` 浏览器测试。
- 覆盖：安装期间取消不终止包管理器、锁文件判定与无法确认时跳过、已安装版本读取失败后重试、镜像取最高版本、托盘失败退避与等待期间关闭自动更新、`quit` 无托盘时退出码 3。
- 限制：未在真实 systemd/SELinux 主机上运行更新服务，未编译或在 Windows 上运行 NSIS 脚本，`windows-installer-smoke.ps1` 未运行；未重新核验其余故事能力。

本次局部核对（2026-11-03）：PR #260 端到端验证，对应 US-K13、US-K14、US-K16、US-O01、US-O12、US-O13，故事内容与覆盖索引不变。均用临时测试密钥签名、本地 HTTP 提供的测试 Release。

- Linux：以 systemd 为 PID 1 的 Debian 13 与 Fedora 44 容器中，更新服务把 deb/rpm 从 0.0.1 升到 0.0.2，另一用户的 `webmux.service` PID 不变。
  - 安装中 `systemctl stop` 等 dpkg 完成后才返回，`dpkg --audit` 为空。
  - `apt-mark hold` 与 dnf5 `versionlock.toml`（仓库不可达）时跳过且不发起请求，锁文件无法解析时按锁定跳过。
- Windows 11（build 26300，NSIS 3.11）：
  - 删除登录启动项后 `/S` 升级仍不登记；安装中止时托盘运行则退出码 4 并恢复、重启 0.0.1 托盘，未运行则退出码 2 且不启动托盘。
  - 安装版 0.0.1 托盘 2 分钟后自更新到 0.0.2，无 SmartScreen/UAC 提示；新托盘把 WSL 中的 webmux 升到 0.0.2，运行中的服务端 PID 不变并提示重启后生效。
  - 托盘触发的安装失败后，重新启动的托盘保留 `updateAttempt` 并提示失败，此后约 3 分钟内的自动检查不再运行安装程序。
- 限制：SELinux enforcing 无法在容器中启用，未验证；未配置托盘转发，转发恢复未单独验证；未重新核验其余故事能力。

本次局部核对（2026-11-03）：补充上一条记录中未验证的两项，对应 US-K14、US-O13，故事内容与覆盖索引不变。

- SELinux：Fedora 44 Cloud 虚拟机（QEMU，enforcing，selinux-policy-targeted 43.3）中更新服务连续把 rpm 从 0.0.1 升到 0.0.4，scriptlet 进入 `rpm_script_t`，没有 AVC 拒绝；加回 `NoNewPrivileges=yes` 时 `nnp_transition` 被拒绝、scriptlet 失败。
- 转发：本机 WSL 为 mirrored 模式，在测试发行版中模拟 NAT 后，托盘自更新期间转发中断约 1.75 秒，新托盘在同一地址与端口恢复转发，服务端 PID 不变。
- 限制：NAT 网络路径由测试替身代替，未在真实 NAT 模式主机上验证；未重新核验其余故事能力。

本次局部核对（2026-11-03）：安装程序可选添加托盘转发的防火墙规则，更新 US-O08、US-O12，不新增故事，覆盖索引不变。

- 依据：`internal/tray` 的规则名称、netsh 参数与 `Firewall.Apply` 单元测试，Windows amd64/arm64 交叉 vet，makensis 3.11 `-WX` 构建。
- 真机（Windows 11 build 26300，非管理员会话）：安装程序冒烟脚本全部通过，静默安装不加规则；交互安装经 UAC 确认后添加入站 TCP 允许规则（域、专用），再次运行不弹窗，静默覆盖后规则保留；交互卸载经 UAC 确认后删除规则。
- 对账（记录托盘路径之后的版本）：首次交互安装经 UAC 添加规则并记录路径；再次安装选项默认勾选、不弹窗；取消勾选后经 UAC 删除规则并清除记录；之后静默卸载无 UAC。
- CI：Windows installer smoke 以管理员运行，`firewall allow` 添加的规则为入站、允许、域与专用网络，重复运行幂等，`remove` 删除规则。
- 限制：未重新核验其余故事能力。

本次局部核对（2026-11-03）：合并 main 后发现 Fedora 45（rpm 6.1，`%_pkgverify_level` 为 `all`）拒绝未签名的 rpm，自动更新改用 `rpm -U --nosignature`，对应 US-K14，故事内容不变。

- 复现：Fedora 45 容器中直接 `rpm -U` 测试包报 “does not verify: no signature”，版本不变。
- 验证：同一容器中 `webmux update apply` 从本地签名的测试 Release 把 0.0.1 升到 0.0.2，`rpm -Va webmux` 无差异；`dnf install` 本地包照常可用。
- 限制：未在 Fedora 45 的 systemd 环境中经定时器运行；未重新核验其余故事能力。

本次局部核对（2026-11-03）：聊天端展示当前 agent 新增 US-Q17，覆盖索引同步，不改变其他故事。

- 依据：`internal/launch` 的档案图标一致性测试、`internal/hybrid` 与 `internal/wire` 的 Go 测试、线上快照与 Zod 契约，以及 `SessionRow` 与 `TerminalHeader` 的浏览器测试。
- 变更作者运行了 `make check`、`make test-fast`（Go race、Vitest 单元与浏览器）、Storybook 与 `hybrid.spec.ts`；本次修复后重跑 `SessionRow` 浏览器用例（14 项通过）与文档治理检查。
- 覆盖：聊天端、休眠与切换中的卡片保留品牌图标和名称，终端端不变，未知图标键只显示名称；聊天页头部的可访问名称、窄屏仅图标与标题截断。
- 限制：截图与手动冒烟使用 fake agent，未连接真实 agent 账号；未重新核验其余故事能力。

本次局部核对（2026-11-03）：新增开发版更新通道（OpenSpec `add-dev-update-channel`），新增 US-K17、US-L14、US-O14，更新 US-K11、US-K15、US-O01、US-O11 与覆盖索引。

- 依据：变更的规格与设计、`internal/update`、`cmd/webmux`、`internal/tray`、发布脚本的实现，以及部署文档。
- 已运行：`make check`、`make test-fast`、`make test-install` 通过；本地 `make release-build VERSION=0.10.3-dev.2` 产出带版本号的全部产物，deb/rpm 包内版本为 `0.10.3~dev.2-1`。
- 已运行（US-K15、US-K17）：Debian 容器中以 root 和本地签名的测试 Release（正式版 0.10.2、开发版 0.10.3-dev.4）执行 `webmux update channel` 与 `update check --json`：切到 dev 后发现 0.10.3-dev.4，未知通道退出码 2，文件 0666 时按 stable 并警告，非 root 写入被拒。
- 已运行（US-K17）：同一测试 Release 上 deb 0.10.2 切到 dev 后 `update apply` 安装 `webmux_0.10.3-dev.4-1_amd64.deb`（`dpkg-query` 报告 `0.10.3~dev.4-1`）；切回 stable 不降级并提示；发布 0.10.3 后升级到 `0.10.3-1`。
- 待验证：未经 systemd 定时器运行；`dev-release.yml` 只经 actionlint 与模拟验证，未在 GitHub 上运行（US-L14）；Windows 托盘未在真实 Windows 上验证（US-O01、US-O11、US-O14）。
