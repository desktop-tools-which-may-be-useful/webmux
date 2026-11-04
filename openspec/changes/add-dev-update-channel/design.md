# Design

## Context

现有更新链路（`signed-release-updates`）：

- 客户端读 `releases/latest/download/SHA256SUMS` 与签名；可信注释 `webmux <版本>` 决定版本；产物从 `releases/download/v<版本>/` 以固定文件名下载。
- 只有一组内置公钥，`ParseVersion` 只接受 `MAJOR.MINOR.PATCH`；托盘的版本对应判定、服务端可用性分类与自更新也只认这种格式。
- 发布 workflow 由 `v*` 标签触发，分为 build、sign、publish 三个 job，签名 job 只拿到 `SHA256SUMS`；`SHA256SUMS` 同时列出 `install.sh`。
- `install.sh` 只核对同一来源 `SHA256SUMS` 中的摘要，不验证 minisign 签名。
- NSIS 安装程序以 `VIProductVersion "${VERSION}.0"` 写入文件版本资源，该字段只接受四段数字。
- `test.yml` 在 main 推送时总会跑完；只改文档的提交跳过其中的 job，结论仍为成功。
- 现有正式标签：`v0.10.2`。

## Goals / Non-Goals

**Goals:**

- main 上每次通过 CI 的提交都能以开发版发布，开发版用户按现有更新节奏（Linux 每日、托盘每 24 小时）自动收到。
- 正式版用户在任何情况下都不会收到开发版，即使开发版签名密钥泄露。
- GitHub 上不累积 Release 或标签；`dev` Release 的版本只增不减。

**Non-Goals:**

- beta、rc 等第三个通道；开发版的发布说明；回滚到更早的开发版；安装已不在 `dev` Release 当前清单中的开发版。
- 为 macOS 或 `install.sh` 安装提供自动更新（它们仍只检查不替换）。
- 让 `webmux.service` 自动重启以应用开发版。
- 为只改文档的提交发布开发版（D8）。

## Decisions

### D1 版本号

开发版版本为 `X.Y.Z-dev.N`：

- `X.Y.Z` 为最近一个正式标签 `vA.B.C` 的补丁号加 1，即 `A.B.(C+1)`；
- `N` 为该标签之后的提交数，正整数、无前导零；
- 由 `git describe --tags --match 'v[0-9]*.[0-9]*.[0-9]*' --exclude '*-*'` 推导，需要完整历史与标签，不需要手工维护；
- 本次提交恰好是正式标签（`N` 为 0）时不发布开发版，该提交由正式发布覆盖。

比较规则遵循 semver 预发布优先级，并只接受 `dev` 这一种后缀：

```text
0.10.2 < 0.10.3-dev.1 < 0.10.3-dev.2 < 0.10.3 < 0.11.0-dev.1 < 0.11.0
```

下一个正式版无论是补丁、次版本还是主版本，都高于此前所有开发版；新标签后 `N` 从 1 重新计数，但基数升高，整体仍单调递增。前提是正式标签打在 main 可达的提交上。

deb/rpm 的包内版本写成 `X.Y.Z~dev.N`，`~` 使它排在 `X.Y.Z` 之前；nfpm 以 semver 解析 `X.Y.Z-dev.N` 时会自动生成该形式。
安装包的文件名不使用 `~`：GitHub 会把上传文件名中的 `~` 改为 `.`，使文件名与 `SHA256SUMS` 不符。文件名使用 `X.Y.Z-dev.N`，dpkg 与 rpm 只依据包内版本排序。

### D2 单个滚动 Release，产物名带版本号

开发版只使用一个 Release 与标签 `dev`，标记为预发布且不设为 latest。产物文件名：

- 二进制、托盘与安装程序：`webmux-<版本>-<正式版文件名去掉 webmux- 前缀>`，例如 `webmux-0.10.3-dev.2-linux-amd64`、`webmux-0.10.3-dev.2-setup-windows-amd64.exe`；
- 安装包：`webmux_0.10.3-dev.2-1_amd64.deb`、`webmux-0.10.3-dev.2-1.x86_64.rpm`（包内版本 `0.10.3~dev.2`）；
- `install.sh` 使用固定文件名，与正式版 Release 中的是同一份脚本，列在本次清单中，每次发布随清单一起替换。

`install.sh` 不在保留范围内：只有执行 `curl | sh` 的用户直接取用它，没有任何客户端用旧清单校验它，替换它不会让读到旧清单的客户端失败。放进 Release `dev` 后，开发版安装地址从第一次开发版发布起就可用，不必等带通道支持的正式版。

每次发布：

1. Release 不存在时创建它；
2. 上传本次的产物；
3. 替换 `install.sh`、`SHA256SUMS` 与 `SHA256SUMS.minisig`，清单只列本次的产物（包括 `install.sh`）；
4. 把标签 `dev` 移到本次提交；
5. 删除 Release 中不在本次清单中的文件，包括上一次构建与此前失败运行留下的残留。

Release `dev` 只保留当前构建，避免旧版本堆积。客户端先读清单再下载产物，替换清单前读到旧清单的客户端会下载失败，下一次检查时重试。

第 3 步不是原子的：`--clobber` 先删除再上传，三个文件分别替换。期间读取的客户端会得到缺失或不配对的清单与签名，该来源按签名无效处理，改用镜像或在下次检查时重试。
发布 job 不可被取消（D8）；若它中途失败，重跑或下一次发布会先校验 Release 上现有的清单与签名，不配对或无法验证时视为未发布并重新上传，而不是按签名里的版本跳过。

比较过的方案：

- 每次构建一个预发布并定期清理：Release 与标签会累积，清理与正在下载的客户端存在竞态。
- 覆盖同名产物：清单与产物可能在下载途中错位。
- 只发 nightly：不满足“每次提交可用”。

### D3 签名密钥按通道分组

- 新增 Actions 密钥 `WEBMUX_DEV_SIGNING_KEY`，与正式版私钥不同。
- 发布构建内置两组公钥：stable 组与 dev 组。
- 验证清单后检查版本与密钥组是否匹配：stable 组只能签不带后缀的版本，dev 组只能签 `-dev.N` 版本，不匹配视为签名无效。
- 正式版通道只信任 stable 组。开发版密钥泄露时，攻击者签出的任何清单都会被正式版客户端拒绝；开发版用户受影响，此时轮换 dev 密钥即可。
- `releasesign sign` 按版本选择私钥环境变量：正式版读 `WEBMUX_RELEASE_SIGNING_KEY`，开发版读 `WEBMUX_DEV_SIGNING_KEY`；`verify` 按版本选择密钥组。

签名环境沿用正式版的隔离方式：独立 job 只接收 `SHA256SUMS`，只运行仓库自带的签名工具。

### D4 清单发现

| 通道 | 读取的清单 | 产物来源 |
|---|---|---|
| stable | `releases/latest/download/` | `releases/download/v<版本>/` 固定文件名 |
| dev | `releases/download/dev/` 与 `releases/latest/download/` | 版本带 `-dev.N` 时为 `releases/download/dev/` 带版本文件名；否则同 stable |

dev 通道对两个清单分别按现有来源顺序与镜像规则取得已验证结果，再取版本较高的一个；一个清单失败不影响另一个。正式版发布后，开发版用户因此自动升到正式版。

### D5 通道的选择与保存

- 默认 stable。未配置或文件缺失时按 stable 处理，不记录警告；内容无法识别时按 stable 处理并记录警告。
- 命令行（Linux 与 macOS）：`/etc/webmux/update-channel`，内容为 `stable` 或 `dev`，不属于任何软件包，升级与卸载都不改动它。
  - `webmux update channel` 显示当前通道；`webmux update channel stable|dev` 需要 root，必要时创建 `/etc/webmux`（0755），以临时文件加改名写入 0644 的文件，下一次检查即生效。
  - 文件不归 root 所有或可被组、其他用户写入时按 stable 处理并记录警告，避免非特权用户改变 root 更新服务的信任范围。
  - `webmux update check` 与 `update apply` 都读取该文件，因此 `install.sh` 安装的开发版也可以用它让检查跟随开发版。
- 托盘：`tray.json` 的 `updateChannel`，菜单“更新通道”下为“正式版”“开发版”两个单选项；切换后立即写入设置并发起一次检查。
- `install.sh`：开发版的安装地址为 `releases/download/dev/install.sh`，也可以用正式版 Release 中的同一份脚本。
  - `WEBMUX_CHANNEL=dev` 时从 `dev` Release 读取 `SHA256SUMS`，找出唯一一个与本机系统、架构匹配的 `webmux-<X.Y.Z-dev.N>-<系统>-<架构>` 行，从文件名得到版本，按现有方式下载并核对摘要。
  - 与现有正式版安装一样只核对同一来源的摘要，不验证 minisign 签名。
  - `WEBMUX_VERSION=X.Y.Z-dev.N` 表示安装该开发版，隐含 `dev` 通道。它必须是 `dev` Release 当前清单的版本，否则报错并给出当前开发版版本。
  - 通道与版本矛盾时报错退出：`WEBMUX_CHANNEL=dev` 搭配 `X.Y.Z`，或 `WEBMUX_CHANNEL=stable` 搭配开发版版本。
  - 只影响本次安装，不写配置。

### D6 切换通道只升级不降级

从 dev 切回 stable 时，若当前开发版高于最新正式版则不做任何安装，状态中说明“当前开发版 X 高于最新正式版 Y，正式版超过它后自动更新”。这样不需要降级路径，也不会出现 dpkg/rpm 降级的特殊处理。

### D7 托盘与 WSL 服务端

- 开发版托盘内置同一次构建的服务端摘要，安装 WSL 服务端时从 `dev` Release 下载带版本号的文件；本地缓存与安装程序预置文件按同一文件名查找。
- `dev` Release 只保留当前构建，较旧开发版托盘需要的服务端已被删除。此时托盘报告“开发版 <版本> 的服务端已不在 dev Release，请先更新托盘”，不改动 WSL 中的文件。
- 版本对应判定与服务端可用性分类按 D1 的规则比较，两侧都是开发版或一侧是开发版时也能判定较旧或较新；字面 `dev` 等无法解析的版本仍归为“无法判断”或可用。
- 托盘切到开发版后，自动升级 WSL 服务端的规则不变，因此服务端会跟随到同一次开发版构建。
- WSL 中是开发版、比托盘新，而托盘通道为 stable 时，stable 通道永远不会提供它，“更新托盘”没有意义：
  - 安装版托盘不自动发起自更新检查，以“切换到开发版通道并更新托盘”替代“更新托盘”；点击后把 `updateChannel` 设为 `dev` 并立即检查，发现更高版本时视同用户点击“更新到 <版本>”立即安装。
  - 便携版的“下载托盘 <版本>”打开 Release `dev` 页面。
- 安装程序的文件版本资源取版本中 `-` 之前的数字部分，写成 `X.Y.Z.0`；登记的显示版本与产品版本字符串保留完整版本。

### D8 开发版发布 workflow

新增 `dev-release.yml`：

- 在 `test.yml` 完成后触发（`workflow_run`），只处理结论为成功、事件为 push、分支为 main、来源为本仓库的运行，不处理 PR。
- 构建 `workflow_run.head_sha`，即通过测试的那个提交，而不是触发时 main 的最新提交；以完整历史检出以便 D1 推导版本，`N` 为 0 时跳过。
- 测试完成顺序可能与推送顺序不同。plan 阶段与发布前各检查一次：该提交已不是 main 的最新提交，或其版本不高于 Release `dev` 当前清单的版本时跳过，保证 `dev` 的版本只增不减，也避免过时的运行取消最新提交的构建。
- push 上的 `test.yml` 只覆盖本次推送自身的差异：只改文档的提交测试很快通过，却不能说明它之前的代码提交已通过测试。build job 因此像 `release.yml` 一样先构建前端并运行 `go test -race`，再构建产物。
- build job 按 `dev-release` 分组并取消进行中的运行；publish 按 `dev-release-publish` 分组且不取消，避免在替换清单途中被打断。sign 不入组：它不改动 Release，入组会让等待中的 sign 被较早运行的 publish 顶替。
- 复用 `make release-build`，以 `VERSION=<开发版版本>` 与带版本号的产物命名构建；sign 与 publish 结构与正式版相同，publish 按 D2 的顺序操作。
- 跳过只改文档的变更（在 build 之前判断，跳过时不构建）：以 Release `dev` 当前发布的提交（标签 `dev`）为基准、本次提交为终点，运行该基准提交中的 `.github/scripts/classify-changes.sh`（与 `test.yml` 一样不采用被测提交中的规则）。
  - 结果为只改文档时不发布；工具链、未知路径等让分类器要求完整运行的变更照常发布。
  - 标签 `dev` 不存在或不是本次提交的祖先时照常发布。
  - 版本号仍按提交数计算，所以跳过后下一个开发版的 `N` 会跳号，不影响排序。

正式版 workflow 发布时以 `--notes-start-tag` 指定上一个 `v*` 标签，避免自动生成的发布说明以滚动的 `dev` 标签为起点。

## Risks / Trade-offs

- [每次合入都跑完整的发布构建] → 约 10 分钟；build job 的 `concurrency` 合并连续推送；只改文档的变更不发布（D8）。
- [开发版质量] → 只发布 build job 内重新通过 Go 测试的提交；开发版用户主动选择，切回正式版不降级。
- [镜像缓存 `dev` 清单] → 沿用“多个来源取最高已验证版本”；缓存的清单指向已删除的产物时下载失败，最坏情况是开发版用户晚一天收到。
- [dev 密钥泄露] → 正式版用户不受影响（D3）；文档写明轮换 dev 密钥的流程。
- [只保留当前构建] → 替换清单前读到旧清单的客户端下载失败，下一次检查重试；较旧开发版托盘无法再安装 WSL 服务端（D7）。
- [替换清单的窗口] → 清单与签名分两次替换，期间的检查失败并重试（D2）。
- [正式标签不在 main 上] → `git describe` 找不到它，main 上的开发版会低于该正式版；正式标签必须打在 main 可达的提交上。
- [移动 `dev` 标签] → 贡献者 `git fetch --tags` 会提示标签被改写，需要 `--force`；文档说明。

## Migration Plan

1. 维护者生成 dev 密钥，配置 `WEBMUX_DEV_SIGNING_KEY`，把公钥加入 dev 组。
2. 合入后，带有通道支持的第一个正式版发布前，开发版就会出现在 `dev` Release；更早的客户端只认 stable，不受影响。
3. 在此之前，`releases/latest` 上的 `install.sh` 不认识 `WEBMUX_CHANNEL`，会静默安装正式版；文档给出的开发版安装地址为 `releases/download/dev/install.sh`。
4. 用户按 D5 选择 dev 通道即可加入；退出只需切回 stable。
