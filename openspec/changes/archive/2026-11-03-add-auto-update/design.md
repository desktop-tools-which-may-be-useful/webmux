# Design

## Context

动机见 proposal.md。与实现相关的现状：

- `make release` 以 `-X main.version=$(VERSION)` 注入版本号；本地 `make build` 同样报告 `0.1.0`，所以版本号无法区分发布构建。托盘只在发布构建中由 `-X internal/tray.linuxAMD64SHA256=...` 注入服务端摘要。
- `internal/fetch`（共享层，仅依赖标准库）已实现“GitHub → `ghfast.top` → `gh-proxy.com`”的来源顺序、10 秒连接与 20 秒无数据超时，以及“摘要一致才原子落盘”。它目前要求调用方预先给出摘要。
- `install.sh` 与托盘以同一来源的 `SHA256SUMS` 或内置摘要校验。没有任何发布签名。
- deb/rpm 由 nfpm 打包（`deploy/linux/nfpm.yaml`），只包含 `/usr/bin/webmux` 与用户单元，没有安装脚本。
- Windows 安装程序（NSIS）支持 `/S`，覆盖安装时会请求托盘退出、替换文件并启动新托盘。托盘把 WSL 服务端安装到可配置路径，并按版本比较给出“较旧/较新”判定。
- 服务端的 `StatusV1`（`internal/contract`）已有 `version` 与 `launcher`，托盘已经对“运行版本 ≠ 已安装文件版本”给出重启提示。服务端自身还没有这种判断。

## Goals / Non-Goals

**Goals:**

- 一个信任根（内置 ed25519 公钥）覆盖 Linux 包更新与 Windows 托盘更新；镜像只影响可用性。
- 更新只替换文件，进程重启永远由用户决定。
- 不新增第三方 Go 依赖，不依赖 GitHub API（会限流，也无法走镜像）。

**Non-Goals:**

- APT/YUM 仓库；`install.sh` 安装和 macOS 的自动替换；Windows 原生控制台客户端的更新。
- 预发布通道、分阶段推送、回滚到旧版本。
- Authenticode 代码签名。

## Decisions

### D1 签名格式：minisign 兼容的纯 Ed25519 签名

`SHA256SUMS.minisig` 使用 minisign 的旧版算法标识 `Ed`，对文件原文做 Ed25519 签名，全局签名覆盖“签名 ‖ 可信注释”。验证只需要标准库的 `crypto/ed25519`。

- 不选 minisign 默认的预哈希 `ED`：它需要 BLAKE2b-512，会引入 `golang.org/x/crypto`。`SHA256SUMS` 只有几 KB，不需要预哈希。
- 不选 OpenPGP 或 cosign：依赖重，用户手动验证的门槛也更高。
- 可信注释固定为 `webmux <版本>`。验证后的版本来自注释，而不是 URL，这样可以防止把旧清单放到新标签路径下冒充新版本。

### D2 密钥管理与签名工具

新增 `cmd/internal/releasesign`，提供 `keygen`、`sign` 与 `verify` 三个子命令。

- 私钥是 Base64 编码的 32 字节 Ed25519 种子，只存放在 GitHub Actions 密钥 `WEBMUX_RELEASE_SIGNING_KEY` 中，由环境变量传给 `sign`，不写入磁盘。
- 发布 workflow 分为三个 job，构建依赖因此读不到私钥。本地的 `make release` 依次执行 `release-build` 与 `release-sign`。
  - `build`：不带私钥运行 `make release-build`，pnpm、Go 模块、nfpm、makensis 都在这里运行。
  - `sign`：在全新 runner 上只下载 `SHA256SUMS`，不恢复任何缓存，用 `make release-sign` 签名并验证；它在 `GOPROXY` 不可用时也能完成。
  - `publish`：合并产物与签名，再次验证后发布。
- 公钥（8 字节密钥 ID 加 32 字节公钥）以 minisign 公钥文本的形式写在 `internal/update` 源码中，作为可审查的常量集合；同时公布在 `docs/deployment/updates.md`。
- 更换密钥的流程：先发布一个同时内置新旧公钥、仍用旧私钥签名的版本，下一个版本再改用新私钥签名。

### D3 共享包 `internal/update`

新增共享层包 `internal/update`，只依赖标准库、`internal/fetch` 与 `internal/contract`，需要在 `internal/archtest` 中登记为 Shared。它包含：

- 签名与清单验证：解析 minisig、按密钥 ID 选公钥、验证、解析可信注释与 `SHA256SUMS`。
- 版本比较：严格的 `MAJOR.MINOR.PATCH`。
- `Check(ctx)`：按 `fetch` 的来源顺序读取 `releases/latest/download/SHA256SUMS{,.minisig}`，返回已验证的清单。
- `Fetch(ctx, manifest, asset, dir)`：从 `releases/download/v<版本>/` 下载产物，用清单中的摘要校验。
- 发布构建标记：变量 `release`，只由 `make release` 通过 `-X .../internal/update.release=1` 注入，供服务端与托盘共用。

`internal/fetch` 增加一个选项，让落盘文件的权限可以是 0644，因为安装包不需要执行权限。其他下载逻辑沿用现有实现，不另写一份。

### D4 Linux：root 一次性服务 + 定时器，而不是软件仓库

`/usr/bin` 由 root 拥有，用户服务无法替换它。比较过的方案：

- APT/YUM 仓库：需要第二把 OpenPGP 密钥和托管；`unattended-upgrades` 默认只自动安装安全源，`dnf-automatic` 默认只下载，并不能保证自动更新。
- 由用户服务调用 `pkexec`：需要交互授权，不适合无人值守。
- 选定方案：一个 root 一次性服务加一个定时器，复用 D1 的信任根与镜像：
  - `webmux-update.service`：`Type=oneshot`、`ProtectHome=yes`、`PrivateTmp=yes`、`CacheDirectory=webmux-update`、`KillMode=process`、`TimeoutStartSec=1h`、`TimeoutStopSec=15min`；
  - 不设 `NoNewPrivileges=`、`Nice=`、`IOSchedulingClass=`：包管理器继承这些设置，前者会阻止 rpm scriptlet 的 SELinux 域转换，后两者会在持有 dpkg 锁时拖慢安装；
  - `webmux-update.timer`：`OnCalendar=daily`、`RandomizedDelaySec=6h`、`Persistent=true`；
  - 代理通过单元的 drop-in 设置 `HTTPS_PROXY`。

`webmux update apply` 的流程：

1. 检查发布构建标记与 `euid == 0`。
2. 用 `flock` 锁住 `/run/webmux-update.lock`，非阻塞；拿不到锁立即退出。
3. 用 `dpkg-query -S /usr/bin/webmux` 或 `rpm -qf /usr/bin/webmux` 判断安装方式。
4. 在任何网络请求之前检查管理员是否用 `apt-mark hold` 或 `dnf versionlock` 锁定了 `webmux`，锁定时记录后跳过；`dpkg -i` 与 `rpm -U` 本身不理会这两种锁。
5. 调用 `Check`；有新版本时按架构用 `Fetch` 下载到缓存目录。
6. 以 argv 执行 `dpkg -i -- <file>` 或 `rpm -U -- <file>`；包没有依赖，所以不需要 apt/dnf 解析依赖。
7. 删除缓存文件。

日志写到标准输出和标准错误，由 journald 收集。

### D5 nfpm 安装脚本

`deploy/linux/` 下新增 `postinstall.sh`、`preremove.sh` 与 `postremove.sh`。是否启用定时器不按“首次安装还是升级”判断，而按标记 `/var/lib/webmux/auto-update-enabled`：

- 运行 systemd 且没有标记（首次安装，或从不含定时器的版本升级）：`systemctl enable --now webmux-update.timer`，成功后写入标记。只按 deb/rpm 的“是否升级”参数判断会让已有用户永远拿不到定时器。
- 有标记：只执行 `daemon-reload`，管理员的 `disable` 因此在升级后保留。
- 卸载（不含升级时的旧包移除）：`systemctl disable --now webmux-update.timer` 并删除标记，重新安装会再次启用。
- 没有运行 systemd：只安装文件，不写标记。

脚本任何时候都不触碰 `webmux.service`。`.github/scripts/test-linux-packages.sh` 覆盖首次安装、带标记升级、无标记升级、卸载与无 systemd 安装。

### D6 Windows 托盘：调用静默安装程序完成自更新

托盘不自己替换 exe，因为正在运行的 exe 无法覆盖。它下载同架构的安装程序并运行 `/S`，安装程序已经负责请求托盘退出、替换文件、预置服务端和启动新托盘。安装版的判定方式：比较 `HKCU\...\Uninstall\WebMux` 的 `InstallLocation` 与托盘自身 exe 所在目录。

NSIS 脚本相应改动：

- 检测到已安装时，静默模式沿用 `InstallLocation`，并保持登录启动项现状。
- 在关闭托盘之后的任何失败路径上，重新启动安装目录中的 `webmux-tray.exe`，然后以非零码退出。

托盘只在空闲时运行安装程序（无对话框，也无 WSL 安装、启动、停止、重启在进行），并以分离进程方式启动它，然后等待它结束。安装程序没有关闭托盘就退出，或 15 分钟内没有结束，都视为失败并清除进行中状态。

新托盘启动时，比较设置中记录的 `lastVersion` 与自身版本，据此显示“已更新到 <版本>”并清理 `updates` 目录。

安装程序关闭托盘之后的失败由它重新启动的原托盘承接，原进程已退出，看不到退出码。所以托盘在运行安装程序前把 `updateAttempt`（版本与时间）写入 `tray.json`：

- 启动时记录的版本高于自身版本，说明上次安装失败。托盘显示一次失败提示，24 小时内自动路径（计划检查、WSL 版本触发）不再安装同一版本，避免持续失败时托盘每隔几分钟被关闭又重启。
- 用户点击“更新到 <版本>”不受限制，更高的版本也不受限制。
- 该版本或更高版本的托盘启动后清除记录。安装程序无法启动，或在本托盘仍运行时退出（退出码 2、3），没有替换托盘，也清除记录；等待 15 分钟超时则保留，因为安装程序仍可能关闭托盘。

NSIS 只在 `quit` 报告确实关闭了运行中的托盘（退出码 0，没有托盘时为 3）后才在失败时重启托盘并以 4 退出，否则只恢复 `.old` 并以 2 退出。

### D7 托盘内的更新顺序

托盘先更新自身，WSL 服务端则由新托盘按 `windows-tray-wsl-install` 自动升级。原因：

- 服务端摘要内置在托盘里，旧托盘只能安装旧版本。
- 新安装程序已经把同版本的服务端预置到缓存中，WSL 架构相同时可以离线完成升级。

自动升级沿用现有的安装流程（argv、摘要校验、原子替换），只是触发条件从“用户点击”扩展为“检查结果为较旧或过旧，且 `autoUpdate` 开启”。

### D8 服务端识别已安装版本

在 `internal/update` 中新增 `InstalledVersion`，由 `cmd/webmux` 构造，注入 `api.Deps` 与本机 API。

- 启动时记录 `os.Executable()` 解析符号链接后的路径，以及该文件的标识（设备号、inode、大小、修改时间）。
- 状态或配置被查询时，若距上次检查已超过 60 秒且标识发生变化，以 argv 运行 `<路径> version --json`（5 秒超时，环境只保留 `PATH`），只把成功读取的结果按标识缓存；读取失败时保留旧标识，下一次检查重新运行。
- 采用按需检查而不用后台定时器：没有人查询时不产生任何开销，也不需要新的生命周期管理。

### D9 契约与前端

- `contract.StatusV1` 增加 `InstalledVersion string \`json:"installedVersion,omitempty"\``。
- `GET /api/config` 的 `contract.ServerConfig` 增加 `PendingUpdate *PendingUpdate \`json:"pendingUpdate,omitempty"\``，内容为 `{installedVersion, launcher}`；`internal/wire` 负责转换。
- `web/src/schemas.ts` 的 `ConfigSchema` 加上对应的可选字段，旧服务端不返回该字段时行为不变。
- 主页顶部与设置页用 `card`/`item` 设计系统组件显示提示。
- 主页上的关闭记录以版本号为键，存放在经 Zod 校验的设备本地设置中。
- 重新录制 `internal/api/testdata/wire`。

## Risks / Trade-offs

- [发布私钥泄露] → 攻击者可以向所有自动更新的安装推送代码。缓解：私钥只放在 Actions 密钥中，签名步骤只在标签触发的发布任务里运行；泄露时按 D2 的流程换密钥，并发布公告。
- [私钥丢失] → 已安装版本无法再接受新版本，只能手动更新一次。缓解：维护者离线备份种子。
- [来源扣留更新（冻结攻击）] → 恶意或过期的镜像可以一直返回旧清单。签名与不降级策略只能保证不安装错误内容，无法强制更新。缓解：GitHub 优先；GitHub 不可用时读取所有镜像并采用最高的已验证版本，日志记录来源。所有可达来源一起扣留时仍无法更新，可以接受。
- [`Ed` 是 minisign 的旧版签名格式] → 已实测：Debian 的 minisign 0.12 用 `-Vm` 直接验证本实现的签名（含可信注释），无需 `-l`。若将来的 minisign 不再支持该格式，只影响用户手动验证，不影响更新程序。
- [功能引入前的版本] → 不会自动更新。首个包含本功能的版本需要用户手动安装一次。
- [root 定时任务的攻击面] → 未经签名验证的数据不会被解析，也不会交给 dpkg/rpm。服务有沙箱限制，并且只执行固定的 argv。
- [包管理器锁冲突] → dpkg 被占用时本次失败，次日重试；`rpm -U` 等待 rpm 数据库锁。`Persistent=true` 只补执行错过的触发，不会频繁重试。
- [安装中途被终止] → 下载阶段 30 分钟超时；安装命令不随取消而终止，单元以 `KillMode=process` 只向 `webmux` 发信号，避免半安装的软件包。
- [NSIS 中途失败留下不完整的安装目录] → 覆盖前把旧托盘改名为 `.old`，关闭托盘后的失败路径恢复它并重新启动。
- [下载的安装程序未经 Authenticode 签名] → 托盘自己下载的文件不带 Mark-of-the-Web；2026-11-03 在 Windows 11 build 26300 上实测自更新静默完成，没有 SmartScreen 或 UAC 提示。完整性由 D1 保证。
- [托盘重启期间转发中断数秒] → 规格已说明，会话不受影响。

## Migration Plan

1. 维护者执行 `go run ./cmd/internal/releasesign keygen`，把公钥提交到 `internal/update`，把种子配置为 Actions 密钥，并离线备份。
2. 发布包含本变更的版本 N。从 N 开始的 Release 附带 `SHA256SUMS.minisig`，N 的 deb/rpm 安装后启用定时器，N 的安装版托盘开始自动更新。
3. 版本低于 N 的用户需要手动升级到 N 一次。
4. 停用方式：Linux 执行 `systemctl disable --now webmux-update.timer`；托盘取消勾选“自动更新”。如果某个版本有问题，发布更高的修复版本，而不是降级。
