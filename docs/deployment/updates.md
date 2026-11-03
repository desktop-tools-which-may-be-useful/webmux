# 自动更新

WebMux 的更新只替换磁盘上的文件，从不自动重启 WebMux：**重启会结束所有终端会话**，由你决定何时重启。

| 安装方式                      | 更新方式                                                                                |
| ----------------------------- | --------------------------------------------------------------------------------------- |
| deb / rpm 安装包              | 系统定时器 `webmux-update.timer` 每天以 root 检查并通过 `dpkg` / `rpm` 安装新版本       |
| Windows 安装程序安装的托盘    | 托盘每 24 小时检查，下载新安装程序后静默升级自身，再把 WSL 中较旧的 webmux 升级到同版本 |
| 便携版托盘                    | 只提示新版本，菜单“下载托盘 <版本>”打开 Release 页面                                    |
| `install.sh`、手动下载、macOS | 不自动替换；`webmux update check` 查看是否有新版本，重新运行 `install.sh` 升级          |

本地 `make build` / `make tray` 构建的程序不是发布构建，不参与更新：`webmux update check` 会报告“此构建不是发布构建，不支持更新”，托盘的“检查更新”不可用。

自动更新从第一个包含该功能的版本开始生效。更早的版本没有更新程序，需要手动升级一次：deb / rpm 用 `apt install ./webmux_<版本>-1_<架构>.deb` 或 `dnf install ./webmux-<版本>-1.<架构>.rpm` 安装新包，Windows 运行新的安装程序。之后的版本按上表自动更新。

## 发布签名

每个 Release 附带 `SHA256SUMS.minisig`：用发布私钥对 `SHA256SUMS` 做的 [minisign](https://jedisct1.github.io/minisign/) 签名，可信注释为 `webmux <版本>`。更新程序只信任内置的公钥：

```text
untrusted comment: minisign public key 4AF7A1E8058C841C
RWQchIwF6KH3SuZyvOfqZZAoobboXxcEAyyLB3YptnKdLs7vUvxLHm4e
```

- 来源顺序为 GitHub，然后是 `ghfast.top`、`gh-proxy.com` 镜像。每个来源的 `SHA256SUMS` 都必须通过签名验证，所以镜像只影响能否下载，不能改变内容。
- GitHub 的清单验证通过就直接采用；GitHub 不可用时会读取所有镜像，采用签名有效且版本最高的那份，所以一个缓存了旧清单的镜像不会挡住其他镜像上的新版本。
- 签名里的版本号决定清单的版本，安装产物从 `v<该版本>` 标签下载；只安装严格高于当前版本的发布，不会降级。

手动验证下载（任意平台安装 minisign 0.11 及以上）：

```sh
minisign -Vm SHA256SUMS -P RWQchIwF6KH3SuZyvOfqZZAoobboXxcEAyyLB3YptnKdLs7vUvxLHm4e
sha256sum -c --ignore-missing SHA256SUMS
```

第一条输出 `Trusted comment: webmux <版本>`，应与你下载的 Release 一致。

## 检查更新

```sh
webmux update check          # 当前版本、最新版本、是否有可用更新
webmux update check --json   # {"v":1,"current":…,"latest":…,"available":…,"installMethod":"deb|rpm|other"}
```

该命令不下载安装产物、不改写文件；检查失败时退出码非零。

## Linux 安装包的自动更新

deb / rpm 第一次装上带定时器的版本时启用 `webmux-update.timer`：新安装，或从更早、不含定时器的版本升级。没有运行 systemd 的容器中只安装文件。

启用后会留下 `/var/lib/webmux/auto-update-enabled`，之后的升级不再改动定时器；卸载软件包时一并删除。定时器每天触发一次，带最长 6 小时随机延迟，关机错过的会在下次开机后补上。

- 服务 `webmux-update.service` 以 root 运行 `webmux update apply`：确认 `/usr/bin/webmux` 属于 `webmux` 软件包，下载并校验同格式、同架构的安装包，再用 `dpkg -i` 或 `rpm -U --nosignature` 安装。它不能访问 `/home` 与 `/root`。
  - 发布的 rpm 不带 rpm 签名，完整性由签名的 `SHA256SUMS` 在安装前保证；Fedora 45 的 rpm 6.1 默认拒绝未签名的包，所以更新程序跳过 rpm 的签名检查，摘要检查照常进行。
- 检查与下载最长 30 分钟；`dpkg` / `rpm` 一旦开始就一定运行到结束。停止服务、关机或 Ctrl-C 只会让 `webmux` 等待包管理器完成后退出（服务设置 `KillMode=process`），不会把软件包留在半安装状态。
- 同一时刻只有一个更新在运行。`dpkg` 被占用（例如你正在 `apt upgrade`）时本次失败，原安装不变，次日重试；`rpm -U` 则会等待 rpm 数据库锁，等 `dnf` 结束后再安装。
- 用 `apt-mark hold webmux` 或 `dnf versionlock add webmux` 锁定版本后，更新服务记录“已被锁定版本”并跳过，且不会发起任何网络请求。
  - dnf 的锁定直接读取锁文件，不依赖软件仓库是否可达：`/etc/dnf/plugins/versionlock.list`（或 `versionlock.conf` 中 `locklist` 指定的文件），以及 dnf5 的 `/etc/dnf/versionlock.toml`。
  - 无法确认锁定状态（`dpkg-query` 出错、锁文件不可读或格式错误）时按已锁定处理，并记录原因。
- 查看记录：`journalctl -u webmux-update.service`，其中有当前版本、最新版本、来源与结果。
- 立即更新：`sudo systemctl start webmux-update.service`（或 `sudo webmux update apply`）。

关闭自动更新，之后的升级会保留这一选择：

```sh
sudo systemctl disable --now webmux-update.timer
```

需要代理时为服务添加环境变量：

```sh
sudo systemctl edit webmux-update.service
# [Service]
# Environment=HTTPS_PROXY=http://proxy.example:3128
```

安装、升级与卸载都不会为任何用户启用、启动、停止或重启 `webmux.service`。

## 重启后生效的提示

正在运行的 WebMux 发现自己的可执行文件被换成了其他版本（例如定时器刚升级了软件包），会在以下位置提示“已安装 <版本>，重启后生效，重启会结束所有会话”：

- `webmux status` 增加一行，`webmux status --json` 与本机状态 API 增加 `installedVersion`；
- Web UI 主页顶部与设置页，并按启动方式给出重启方法：systemd 为 `systemctl --user restart webmux`，launchd 为 `webmux service restart`，Windows 托盘为“在托盘菜单中重启”，手动运行为“停止后重新运行 webmux”。

WebMux 只在有人查询状态或配置时检查，两次检查至少间隔 60 秒；文件变化后运行新文件的 `version --json` 读取版本。这次读取失败（例如 5 秒超时）时暂不提示，下一次检查会重新读取。

主页上的提示可以关闭，关闭只记在当前设备上，出现更新的版本时会再次显示；设置页中的提示始终保留。

## Windows 托盘

托盘菜单中的“自动更新”默认勾选，对应 `%APPDATA%\WebMux\tray.json` 中的 `autoUpdate`。

- 开启时，托盘启动 2 分钟后检查一次，之后每 24 小时检查一次；“检查更新”随时手动检查。
- 安装版托盘发现新版本后，把 `webmux-setup-windows-<架构>.exe` 下载到 `%LOCALAPPDATA%\WebMux\updates`，校验后以 `/S` 静默运行。安装程序沿用原安装目录与登录启动设置。旧托盘按“仅退出托盘”退出，WebMux 与会话不受影响；经托盘转发的连接会短暂断开（浏览器自动重连），转发在新托盘启动后恢复。
- 托盘正在安装 WSL 中的 webmux、启动、停止、重启 WebMux 或显示对话框时，等这些操作结束后再升级；等待期间关闭“自动更新”则不再安装。
- 运行安装程序前，托盘在 `tray.json` 的 `updateAttempt` 中记下版本与时间；设置文件无效或无法写入时不运行安装程序。
- 安装程序结束后托盘仍在运行，或 15 分钟内没有结束，状态行显示“更新失败：<原因>”。前一种情况没有替换托盘，托盘会清除 `updateAttempt`；超时后安装程序仍可能关闭托盘，记录保留。安装程序无法启动时同样清除记录。
- 安装程序关闭托盘后才失败时，它恢复并重新启动原托盘。原托盘根据 `updateAttempt` 显示一次“更新到 <版本> 失败，已恢复原托盘”，24 小时内自动检查不再安装这个版本；菜单中的“更新到 <版本>”仍可立即重试，更高的版本不受影响。
- 新托盘在状态行显示一次“已更新到 <版本>”并清除 `updateAttempt`。之后它发现 WSL 中的 webmux 较旧时会自动升级该文件，并提示重启 WebMux 后生效。
- 关闭“自动更新”后不再自动检查；手动检查发现新版本时，菜单提供“更新到 <版本>”。

## 密钥轮换（维护者）

发布私钥是 GitHub Actions 密钥 `WEBMUX_RELEASE_SIGNING_KEY`，内容为 `go run ./cmd/internal/releasesign keygen -secret <文件>` 生成的一行 Base64，需要离线备份；缺失时发布失败。发布 workflow 的构建 job 看不到它，只有单独的签名 job 在全新 runner 上读取它，并且只签 `SHA256SUMS`。

更换密钥分两步：先发布一个同时内置新旧公钥（`internal/update` 的 `releaseKeys`）、仍用旧私钥签名的版本，下一个版本再改用新私钥。私钥丢失后，已安装的版本无法再自动更新，用户需要手动升级一次。
