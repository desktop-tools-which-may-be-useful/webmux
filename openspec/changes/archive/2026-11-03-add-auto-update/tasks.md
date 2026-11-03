# Tasks

## 1. 签名与共享更新包

- [x] 1.1 新增 `cmd/internal/releasesign`（`keygen`、`sign`、`verify`），按 D1 生成 minisign `Ed` 签名，可信注释为 `webmux <版本>`。
  - 验证：Go 测试中签名后用同一工具验证通过。
  - 验证：本机有 `minisign` 时，`minisign -Vm` 能验证该签名；结论写进 design.md 的 Risks。
- [x] 1.2 新增共享包 `internal/update`：minisig 解析与验证、可信注释版本、`SHA256SUMS` 解析、版本比较、`release` 构建标记。
  - 在 `internal/archtest/layers_test.go` 中登记为 Shared。
  - 验证：单元测试覆盖篡改内容、篡改注释、未知密钥 ID、注释格式错误与版本比较；`go test ./internal/archtest` 通过。
- [x] 1.3 为 `internal/fetch` 增加落盘权限选项；在 `internal/update` 中实现 `Check`（latest 清单，带来源回退）与 `Fetch`（按标签下载并校验）。
  - 验证：`httptest` 模拟 GitHub 与镜像，覆盖首个来源签名无效时回退、旧清单不被当作新版本。
  - 验证：同一测试覆盖版本不高于当前时不下载、摘要不符时删除临时文件。

## 2. 发布流水线

- [x] 2.1 `Makefile` 的 `release` 目标为所有二进制注入 `internal/update.release=1`，并在生成 `SHA256SUMS` 后签名、用内置公钥验证。
  - 缺少 `WEBMUX_RELEASE_SIGNING_KEY` 时失败。
  - 验证：用真实私钥单独运行 `make release-sign`，产出签名并通过内置公钥验证；不设该变量时 `make release` 以非零码退出。完整的 `make release` 因本机没有 makensis 未运行。
- [x] 2.2 `.github/workflows/release.yml` 把密钥以环境变量传给签名步骤，校验步骤增加签名验证，并上传 `SHA256SUMS.minisig`。
  - 验证：`actionlint`（若可用）通过；检查上传列表包含该文件，日志不会打印密钥。

## 3. CLI 更新检查与 Linux 自动更新

- [x] 3.1 在 `cmd/webmux` 中新增 `update check [--json]`，以 `dpkg-query -S`、`rpm -qf` 识别安装方式，否则为 `other`。
  - 非发布构建按规格报错，不发起网络请求。
  - 验证：测试覆盖 JSON 输出、非发布构建报错，以及 `other` 时提示重新运行 `install.sh`。
- [x] 3.2 新增 `update apply`，按 D4 的流程实现：root 检查、文件锁、安装方式、`Check`/`Fetch`、argv 调用包管理器、清理缓存。
  - 验证：用假的 `dpkg`、`rpm`、`dpkg-query` 脚本测试 argv、非 root 拒绝、非包安装拒绝。
  - 验证：同一测试覆盖锁被占用时立即退出，包管理器失败时返回非零并删除安装包。
- [x] 3.3 在 `deploy/linux/` 新增 `webmux-update.service`、`webmux-update.timer`、`postinstall.sh`、`preremove.sh`、`postremove.sh`，并写入 `nfpm.yaml`。
  - 验证：`dpkg-deb -c` 与 `rpm -qlp` 只列出四个文件；`systemd-analyze verify` 检查两个单元通过。
- [x] 3.4 扩展 `.github/scripts/test-linux-packages.sh`，覆盖定时器的启用、保留与停用，以及无 systemd 时安装。
  - 断言：首次安装启用；`disable` 后升级仍为禁用；卸载后停用；无 systemd 时不报错；不触碰 `webmux.service`。
  - 验证：在 Debian 与 Fedora 容器中运行该脚本通过。
- [x] 3.5 冒烟：在一次性 systemd 容器或虚拟机中，安装旧版 deb，并用本地签名的测试 Release 触发更新服务。
  - 测试 Release 由本地 HTTP 服务提供，二进制以测试公钥构建。
  - 验证：`/usr/bin/webmux version` 变为新版本，`journalctl` 有版本、来源与结果。
  - 验证：另一个用户正在运行的 `webmux.service` PID 不变。

## 4. 服务端待生效版本提示

- [x] 4.1 在 `internal/update` 中按 D8 实现 `InstalledVersion`，由 `cmd/webmux` 构造并注入。
  - 验证：用临时可执行脚本测试标识未变时不执行、替换后报告新版本、删除或超时时报告未知。
- [x] 4.2 `contract.StatusV1` 增加 `installedVersion`，`contract.ServerConfig` 增加 `pendingUpdate`，由 `internal/wire` 转换。
  - `webmux status` 文本输出增加提示行。
  - 执行 `go test ./internal/api -run TestWireSnapshots -update` 重新录制快照。
  - 验证：`go test -race ./internal/api ./cmd/webmux` 通过，快照只新增预期字段。
- [x] 4.3 前端：`ConfigSchema` 增加可选的 `pendingUpdate`；主页顶部与设置页显示提示与重启命令；主页关闭记录按版本保存。
  - 关闭记录存放在设备本地设置中，经 Zod 校验。
  - 验证：Vitest 覆盖缺少该字段、按启动方显示命令、关闭后出现新版本时重新显示。
  - 验证：`pnpm --dir web check:arch` 与 `make web` 通过。
- [x] 4.4 冒烟：在临时 `state_dir` 中启动带发布标记的二进制，把其可执行文件替换为另一版本。
  - 验证：60 秒后 `webmux status --json` 含 `installedVersion`，主页显示提示（附截图），会话与进程不变。

## 5. Windows 托盘自更新与安装程序

- [x] 5.1 托盘设置增加 `autoUpdate`（缺省为 `true`）与 `lastVersion`；菜单增加“自动更新”勾选项和“检查更新”。
  - 非发布构建时禁用“检查更新”。
  - 验证：`internal/tray` 测试覆盖旧设置文件的默认值，`menuview` 测试覆盖菜单项状态。
- [x] 5.2 实现安装版识别、检查调度、空闲判定，以及下载安装程序并以分离进程运行 `/S`；便携版只显示“下载托盘 <版本>”。
  - 安装版识别比较 HKCU 卸载项的 `InstallLocation` 与 exe 目录。
  - 调度：启动后 2 分钟、每 24 小时、手动检查；下载目录为 `%LOCALAPPDATA%\WebMux\updates`。
  - 验证：非 Windows 也能运行的核心测试覆盖调度、空闲推迟、便携版不下载、签名无效时不运行。
- [x] 5.3 新托盘根据 `lastVersion` 显示一次“已更新到 <版本>”并清理 `updates` 目录；更新入口区分安装版与便携版。
  - WSL 较新时，安装版托盘触发自更新检查。
  - 验证：`compat_test.go`、`menuview_test.go` 覆盖这些状态行和菜单项。
- [x] 5.4 WSL 服务端自动升级：结果为较旧或“不兼容：WSL 过旧”且 `autoUpdate` 开启时，自动执行现有安装流程。
  - 失败只更新状态行，不立即重试。
  - 验证：`install_test.go` 覆盖自动触发、关闭时不触发、缺失与过新时不触发、写入失败时显示原因。
- [x] 5.5 NSIS 脚本：静默覆盖安装沿用 `InstallLocation` 并保持登录启动项现状；关闭托盘后的失败路径重启原托盘。
  - 验证：取消登录启动后以 `/S` 升级，启动项仍不存在。
  - 验证：模拟文件被占用导致失败，原托盘被重新启动，退出码非零。
  - 结果（2026-11-03，Windows 11 build 26300，NSIS 3.11）：删除 `Run\WebMuxTray` 后 `/S` 升级到 0.0.2 退出码 0、启动项仍不存在、安装目录不变。
  - 结果：以独占句柄锁住 `uninstall.exe` 使安装中止：托盘在运行时退出码 4，`.old` 恢复为 0.0.1 并重新启动；没有运行中的托盘时退出码 2，恢复 `.old` 且不启动托盘。
- [x] 5.6 Windows 端到端冒烟：安装旧版本安装程序，用本地签名的测试 Release 让托盘自更新。
  - 验证：WebMux PID 与会话不变，转发恢复，WSL 中的 webmux 被自动升级并提示重启后生效。
  - 验证：是否出现 SmartScreen 提示，结论写进 design.md 的 Risks。
  - 结果：0.0.1 托盘启动 2 分钟后下载、校验并静默安装 0.0.2，约 1.5 秒后 0.0.2 托盘启动，`lastVersion` 为 0.0.2，`updates` 目录为空。
  - 结果：新托盘把 WSL 中的 webmux 升级到 0.0.2，运行中的服务端 PID 不变，`webmux status` 提示已安装 0.0.2、重启后生效；没有出现 SmartScreen 或 UAC 提示。
  - 结果：被锁定导致失败后，重新启动的 0.0.1 托盘记录 `updateAttempt` 并提示失败，之后约 3 分钟内的 WSL 触发检查与计划检查都没有再次运行安装程序。
  - 结果（转发）：在 WebMuxFwdE2E 测试发行版中模拟 NAT 网络（本机 WSL 为 mirrored 模式，托盘只在 NAT 下转发），托盘转发 `192.168.31.17:7681`；自更新时转发中断约 1.75 秒，已建立的连接被断开，客户端约 2.3 秒后重新连通，新托盘在同一地址与端口恢复转发，WSL 中的服务端 PID 不变。
  - 限制：未在真实 NAT 网络模式的主机上验证转发；NAT 网络路径由测试替身代替，托盘自身的安装、自更新与转发均为真实运行。

## 6. 文档与用户故事

- [x] 6.1 在 `docs/deployment/updates.md` 中公布发布公钥与手动验证命令，并说明停用 Linux 定时器、关闭托盘自动更新、配置代理。
  - 验证：`make docs-check` 通过，文档中的 `minisign -Vm` 命令能验证测试 Release。
- [x] 6.2 按 `docs/agent-rules/user-stories-maintenance.md` 新增和更新用户故事，并同步两个索引的计数。
  - `operations-and-tooling.md`：Linux 包自动更新、`update check`、服务端待生效提示。
  - `clients-network-tray.md`：托盘自更新、WSL 自动升级，以及受影响的现有托盘故事。
  - 验证：`make docs-check` 通过，索引计数一致。

## 7. 集成检查

- [x] 7.1 运行 `make check` 与 `make test-fast`。
  - 验证：`make check` 通过，没有新增 lint 警告，也没有扩大架构基线；`make test-fast` 的 Go race 全部通过。
  - 浏览器测试三次运行中 `TerminalGraphics`、`CodeReader`、`TerminalTransport` 各失败过一次，失败用例每次不同、均不在本次改动范围内，单独重跑通过。

## 8. 审查修复（PR #260）

- [x] 8.1 安装脚本改用标记 `/var/lib/webmux/auto-update-enabled` 决定是否启用定时器，从不含定时器的版本升级也会启用；卸载删除标记。
  - 验证：`test-linux-packages.sh` 在 Debian 与 Fedora 容器中覆盖首次安装、带标记升级、无标记升级与卸载。
- [x] 8.2 发布 workflow 拆为 build、sign、publish；Makefile 拆出 `release-build`，只有 sign job 读取私钥。
  - 验证：`actionlint` 通过；在空模块缓存且 `GOPROXY=off` 的环境中用真实私钥运行 `make release-sign` 成功。
- [x] 8.3 `update apply` 遇到 `apt-mark hold` 或 `dnf versionlock` 锁定时跳过（9.2 改为直接读取锁文件并在无法确认时按锁定处理）。
  - 验证：单元测试覆盖锁定与未锁定输出；容器中确认 dpkg 与 dnf5 的真实输出格式。
- [x] 8.4 托盘等待安装程序结束，安装程序未关闭托盘就退出或超时 15 分钟时清除进行中状态并提示失败。
  - 验证：`internal/tray` 核心测试覆盖退出码 2、超时与运行中保持忙碌。
- [x] 8.5 NSIS 覆盖前把旧托盘改名为 `.old`，关闭托盘后失败时恢复并重新启动旧托盘。
  - 验证：makensis 3.11 以 `-WX` 构建两种架构；未在 Windows 上运行失败路径。
- [x] 8.6 为“首次渲染前的跳转行”增加确定性的浏览器测试。
  - 验证：撤销修复时 3 次均失败，恢复后通过。

## 9. 审查修复（PR #260 第二轮）

- [x] 9.1 `update apply` 的 30 分钟超时只覆盖检查与下载；`dpkg` / `rpm` 以不可取消的上下文运行，子进程使用独立进程组。
  - `webmux-update.service` 改为 `KillMode=process`、`TimeoutStartSec=1h`、`TimeoutStopSec=15min`，去掉 `NoNewPrivileges=`、`Nice=`、`IOSchedulingClass=`。
  - 验证：单元测试覆盖安装期间取消父上下文时安装完成、下载后已取消时不安装。
  - 结果（2026-11-03，以 systemd 为 PID 1 的 Debian 13 容器）：定时器服务把 0.0.1 升到 0.0.2，另一用户的 `webmux.service` PID 不变；`postinst` 中 `sleep 40` 的测试包安装期间执行 `systemctl stop`，约 41 秒后停止返回，dpkg 完成且 `dpkg --audit` 为空。
  - 结果：Fedora 44 容器中 rpm 0.0.1 → 0.0.2 同样通过；`apt-mark hold` 时跳过且没有任何 HTTP 请求。
  - 结果：Fedora 44 Cloud（selinux-policy-targeted 43.3，enforcing，QEMU 虚拟机）中更新服务连续升级 rpm 0.0.1 → 0.0.4，服务运行在 `unconfined_service_t`，scriptlet 正常进入 `rpm_script_t`，期间没有 AVC 拒绝，`rpm -Va` 无差异。
  - 结果：对照实验加回 `NoNewPrivileges=yes` 后，`nnp_transition` 被拒绝，scriptlet 无法执行，rpm 数据库同时留下新旧两个版本，证实去掉该项是必要的。
- [x] 9.2 版本锁定改为读取 dnf4 锁文件（含 `locklist` 覆盖）与 dnf5 的 `versionlock.toml`，`dpkg-query` 出错或锁文件不可读、无法解析时按锁定跳过。
  - dnf5 文件按行查找 `name`，不引入 TOML 模块：签名工具链接 `internal/update`，必须只依赖标准库。
  - 验证：单元测试覆盖默认路径、路径覆盖、纯名字、架构后缀、排除项、不相关的包名、dnf5、不可读文件与缺失文件。
  - 验证：`releasesign` 新增测试用 `go list -deps` 断言只链接标准库与本模块，引入 TOML 模块时失败。
  - 结果：Fedora 44（dnf5 5.4.3）仓库不可达时 `dnf versionlock add` 卡在加载仓库；更新服务直接读取 `versionlock.toml` 并跳过，没有 HTTP 请求；锁文件换成目录或未加引号的 `name` 时按已锁定跳过。
- [x] 9.3 `InstalledVersion` 读取失败时保留旧标识，下一次检查重新读取。
  - 验证：单元测试覆盖超时、失败、无效输出、空版本后的重试与成功后的缓存。
- [x] 9.4 GitHub 失败时读取所有镜像，采用签名有效的最高版本。
  - 验证：单元测试覆盖旧镜像在前、在后、签名无效、404，以及 GitHub 成功时不访问镜像。
- [x] 9.5 托盘在运行安装程序前记录 `updateAttempt`；被安装程序重启的原托盘显示失败并在 24 小时内不自动重装同一版本。
  - 下载或等待空闲期间关闭“自动更新”时不再运行安装程序。
  - 安装程序无法启动或在本托盘仍运行时退出则清除记录，等待超时保留。
  - 验证：`internal/tray` 核心测试覆盖失败提示、计划检查与 WSL 触发的退避、手动安装与更高版本不受限、24 小时后重试、记录失败时不运行、等待期间关闭自动更新，以及退出、启动失败与超时时的记录清除。
- [x] 9.6 `webmux-tray quit` 在没有运行中的托盘时以 3 退出；NSIS 只在确实关闭了托盘时于失败后重启它并以 4 退出，否则恢复 `.old` 后以 2 退出。
  - 验证：`internal/tray` 测试与 Windows amd64/arm64 交叉 vet 通过；CI 的 Windows installer smoke 用 makensis 构建两种架构并在 Windows 上确认无托盘时 `quit` 退出码为 3。
- [x] 9.7 首次放置前提交的跳转行只替换恢复状态中的位置，保留选区与搜索；相对跳转从恢复的选区起算。
  - 验证：`CodeReader.test.ts` 新增用例通过。

## Workflow follow-up

- 维护者执行 `go run ./cmd/internal/releasesign keygen`，把公钥提交到 `internal/update`，把种子配置为 Actions 密钥 `WEBMUX_RELEASE_SIGNING_KEY`，并离线备份。
- 在发布说明中告知：低于本版本的用户需要手动升级一次。
- 审查通过后归档本变更。
