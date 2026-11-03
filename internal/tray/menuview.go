package tray

import (
	"fmt"
	"strings"

	"github.com/rainchan/webmux/internal/contract"
	"github.com/rainchan/webmux/internal/update"
)

// InstallItem describes the install / upgrade menu entry.
type InstallItem struct {
	Show, Enabled bool
	Title         string
}

// InstallEntry derives the install entry. supported is false for a tray
// without server digests.
func InstallEntry(snap Snapshot, supported bool) InstallItem {
	if !snap.Installable() {
		return InstallItem{}
	}
	title := "安装 WSL 中的 webmux"
	switch {
	case snap.Blocker == BlockerIncompatible, snap.Compat == CompatIncompatibleOlder:
		title = "升级 WSL 中的 webmux"
	case snap.Compat == CompatServerOlder:
		title = "升级 WSL 中的 webmux（" + snap.Installed.Version + " → " + snap.Tray.Version + "）"
	}
	switch {
	case !supported:
		return InstallItem{Show: true, Title: title + "（" + ErrInstallUnsupported.Error() + "）"}
	case snap.Installing:
		return InstallItem{Show: true, Title: "正在安装 WSL 中的 webmux…"}
	}
	busy := snap.State == StateStarting || snap.State == StateStopping || snap.State == StateRestarting
	return InstallItem{Show: true, Enabled: !busy, Title: title}
}

// UpdateAction is what the tray update entry does when clicked.
type UpdateAction int

const (
	// UpdateOpenRelease opens URL in the default browser.
	UpdateOpenRelease UpdateAction = iota + 1
	// UpdateInstall checks for the latest release and installs it.
	UpdateInstall
	// UpdateSwitchToDev moves the tray to the dev channel, then checks
	// and installs as UpdateInstall does.
	UpdateSwitchToDev
)

// UpdateItem describes the single entry updating the tray.
type UpdateItem struct {
	Show, Enabled bool
	Title, URL    string
	Action        UpdateAction
}

// UpdateEntry offers a newer tray: the release found by the update check
// first, else the version of a newer WSL webmux. An installed tray
// installs it; a portable one only opens the release page. Only release
// and development versions have a release page. A newer development build
// in WSL is never offered on the stable channel, so an installed tray
// offers to switch to the dev channel instead.
func UpdateEntry(snap Snapshot, u SelfUpdateView) UpdateItem {
	if u.Latest != "" {
		if !u.Installed {
			return UpdateItem{Show: true, Enabled: true, Title: "下载托盘 " + u.Latest, URL: update.ReleasePage(u.Latest), Action: UpdateOpenRelease}
		}
		return UpdateItem{Show: true, Enabled: !u.Busy, Title: "更新到 " + u.Latest, Action: UpdateInstall}
	}
	version := snap.Installed.Version
	if _, ok := update.ParseVersion(version); !ok || !snap.Compat.Newer() {
		return UpdateItem{}
	}
	if u.Installed && u.Release {
		if devAhead(snap, u) {
			return UpdateItem{Show: true, Enabled: !u.Busy, Title: "切换到开发版通道并更新托盘", Action: UpdateSwitchToDev}
		}
		return UpdateItem{Show: true, Enabled: !u.Busy, Title: "更新托盘", Action: UpdateInstall}
	}
	return UpdateItem{Show: true, Enabled: true, Title: "下载托盘 " + version, URL: update.ReleasePage(version), Action: UpdateOpenRelease}
}

// ChannelHint is the status line explaining why a newer development build
// in WSL is not offered: the tray follows the stable channel. It is ""
// otherwise.
func ChannelHint(snap Snapshot, u SelfUpdateView) string {
	if !devAhead(snap, u) {
		return ""
	}
	return "WSL 中为开发版 " + snap.Installed.Version + "，托盘使用正式版通道"
}

// devAhead reports a WSL webmux that is a newer development build while
// the tray follows the stable channel.
func devAhead(snap Snapshot, u SelfUpdateView) bool {
	return snap.Compat.Newer() && update.IsDevVersion(snap.Installed.Version) && u.Channel != update.ChannelDev
}

// CheckItem describes the "检查更新" entry.
type CheckItem struct {
	Enabled bool
	Title   string
}

// CheckEntry is disabled with the reason in a development build and while
// an update runs.
func CheckEntry(u SelfUpdateView) CheckItem {
	switch {
	case !u.Release:
		return CheckItem{Title: "检查更新（" + update.ErrNotRelease.Error() + "）"}
	case u.Busy:
		return CheckItem{Title: "检查更新（正在进行）"}
	}
	return CheckItem{Enabled: true, Title: "检查更新"}
}

// ChannelItem describes the "更新通道" submenu and its two radio items.
type ChannelItem struct {
	Enabled     bool
	Title       string
	Stable, Dev bool // which radio item is checked
}

// ChannelEntry is disabled with the reason in a development build and while
// an update runs, so a switch never races a download or install.
func ChannelEntry(u SelfUpdateView) ChannelItem {
	item := ChannelItem{Enabled: true, Title: "更新通道", Stable: u.Channel != update.ChannelDev, Dev: u.Channel == update.ChannelDev}
	switch {
	case !u.Release:
		item.Enabled, item.Title = false, "更新通道（"+update.ErrNotRelease.Error()+"）"
	case u.Busy:
		item.Enabled, item.Title = false, "更新通道（正在进行）"
	}
	return item
}

// VersionHint is the status line of the version check: the older side when
// both are compatible but differ, the direction when incompatible, and a
// running instance that differs from the installed file. It is "" when
// they match or cannot be compared.
func VersionHint(snap Snapshot) string {
	var hints []string
	installed := snap.Installed.Version
	wsl := "WSL 中的 webmux"
	if installed != "" {
		wsl += " " + installed
	}
	switch snap.Compat {
	case CompatServerOlder:
		hints = append(hints, "托盘 "+snap.Tray.Version+" · WSL "+installed+"（WSL 较旧）")
	case CompatServerNewer:
		hints = append(hints, "托盘 "+snap.Tray.Version+" · WSL "+installed+"（托盘较旧）")
	case CompatIncompatibleOlder:
		hints = append(hints, "WSL 中的 webmux 版本过旧，请升级")
	case CompatIncompatibleNewer:
		hints = append(hints, wsl+" 比托盘新，请升级托盘")
	}
	if r := snap.Running; r != nil && snap.Compat != "" && installed != "" && r.Version != installed {
		hints = append(hints, "WSL 中已安装 "+installed+"，重启 WebMux 后生效，重启会结束所有会话")
	}
	if snap.UpgradeError != "" {
		hints = append(hints, "自动升级 WSL 中的 webmux 失败："+snap.UpgradeError)
	}
	return strings.Join(hints, "；")
}

// RunningState reports the states with a running instance.
func RunningState(s State) bool {
	return s == StateRunning || s == StateUnreachable || s == StateExternal
}

// StateLabel names the state; a degraded instance is marked limited.
func StateLabel(snap Snapshot) string {
	switch snap.State {
	case StateStarting:
		return "启动中"
	case StateRunning:
		if snap.Degraded {
			return "运行中（不兼容，功能受限）"
		}
		return "运行中"
	case StateUnreachable:
		return "运行中（Windows 侧不可达）"
	case StateExternal:
		return "运行中（外部实例）"
	case StateStopping:
		return "停止中"
	case StateRestarting:
		return "重启中"
	case StateFailed:
		return "异常"
	}
	return "已停止"
}

// StatusLine is the first menu line. A degraded instance reports no
// session count.
func StatusLine(snap Snapshot) string {
	line := "状态：" + StateLabel(snap)
	if RunningState(snap.State) {
		if snap.Degraded {
			line += " · 会话数未知"
		} else {
			line += fmt.Sprintf(" · %d 个会话", snap.Sessions)
		}
	}
	if snap.HasStatus && snap.Status.Version != "" {
		line += " · WebMux " + snap.Status.Version
	}
	return line
}

// TunnelItem describes the public tunnel entries.
type TunnelItem struct {
	// Show is false when no instance runs or it cannot control its tunnel.
	Show, Checked bool
	SettingsURL   string
	// Info is the state line; PublicURL is set only when ready.
	Info, PublicURL string
}

// TunnelEntry derives the tunnel entries from the running instance.
func TunnelEntry(snap Snapshot) TunnelItem {
	t := snap.Status.Tunnel
	if !snap.HasStatus || snap.Status.State != contract.StateRunning || t == nil || !snap.Actions().Access {
		return TunnelItem{}
	}
	item := TunnelItem{Show: true, Checked: snap.TunnelEnabled()}
	if t.SettingsAvailable && snap.Status.URLs.Local != "" {
		item.SettingsURL = strings.TrimRight(snap.Status.URLs.Local, "/") + "/settings/tunnel"
	}
	switch t.State {
	case contract.TunnelStarting:
		item.Info = "公网隧道：正在连接"
	case contract.TunnelReady:
		item.Info = "公网隧道：" + t.PublicURL
		item.PublicURL = t.PublicURL
	case contract.TunnelFailed:
		item.Info = "公网隧道失败：" + t.Error
	}
	return item
}

// PushItem describes the phone push hint under the tunnel entries.
type PushItem struct {
	// Show is false when the tunnel entries are hidden or the server
	// predates push.
	Show bool
	// Info is the hint line; Guide is the setup dialog text. Neither
	// carries a token, tunnel credential or subscription endpoint.
	Info, Guide string
}

// PushEntry derives the push hint. Phones can only subscribe from a Named
// Tunnel origin, so the hint names that prerequisite first.
func PushEntry(snap Snapshot) PushItem {
	p := snap.Status.Push
	if p == nil || !TunnelEntry(snap).Show {
		return PushItem{}
	}
	item := PushItem{Show: true}
	switch {
	case !p.Enabled:
		item.Info = "手机推送：已在配置中停用"
	case !p.Available:
		item.Info = "手机推送不可用：" + p.Error
	case p.PublicOrigin == "":
		item.Info = "手机推送：需要 Named Tunnel"
	case p.Subscriptions == 0:
		item.Info = "手机推送：尚未有设备开启"
	default:
		item.Info = fmt.Sprintf("手机推送：%d 台设备已开启", p.Subscriptions)
	}
	item.Guide = pushGuide(p, snap.Status.URLs.Local, snap.Status.Tunnel.SettingsAvailable)
	return item
}

const pushPhoneSteps = "在手机上开启推送：\n" +
	"1. 点击托盘“复制链接 → 公网链接”，在手机浏览器中打开；\n" +
	"2. iPhone / iPad（iOS 16.4 以上）：在 Safari 中点“分享 → 添加到主屏幕”，从主屏幕图标打开 WebMux 并输入 Token；\n" +
	"3. 进入 设置 → 通知推送，打开开关。"

func pushGuide(p *contract.PushStatus, local string, webSettings bool) string {
	var b strings.Builder
	if !p.Enabled {
		b.WriteString("推送已在配置中停用：在 WebMux 配置文件的 [push] 中设置 enabled = true，再通过托盘重启 WebMux。\n\n")
	}
	if p.PublicOrigin == "" {
		target := "WebMux 本地地址"
		if local != "" {
			target += "（" + local + "）"
		}
		b.WriteString("Quick Tunnel 的地址每次启动都会变化，旧订阅点开即失效，无法用于手机推送。请先配置 Cloudflare Named Tunnel：\n" +
			"1. 在 Cloudflare 控制台创建 Tunnel，把公共主机名指向" + target + "；\n")
		if webSettings {
			b.WriteString("2. 在 WebMux 设置 → 公网访问中选择 Named Tunnel，填写隧道凭据与 hostname；\n" +
				"3. 保存并启用公网访问，无需重启 WebMux。\n\n")
		} else {
			b.WriteString("2. 当前服务端尚不支持网页公网访问设置，请升级服务端，或在配置文件的 [tunnel] 中写入 token（tunnel.token）与 hostname（tunnel.hostname）；\n" +
				"3. 修改旧版服务端配置后重启 WebMux，再开启“公网隧道（Cloudflare）”。\n\n")
		}
	} else {
		b.WriteString("公网地址：" + p.PublicOrigin + "\n\n")
	}
	b.WriteString(pushPhoneSteps)
	return b.String()
}

// TunnelWarning is the confirmation shown before every enable.
const TunnelWarning = "开启公网隧道后，WebMux 会通过 Cloudflare 暴露到公网。\n\n任何获得链接和 Token 的人都能以你的身份执行命令。Quick Tunnel 没有额外的身份验证，Token 是唯一的屏障。\n\n确定开启？"
