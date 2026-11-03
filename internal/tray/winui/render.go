//go:build windows

package winui

import (
	"slices"
	"strings"

	"github.com/rainchan/webmux/internal/tray"
)

// render applies a snapshot to the menu and returns the new bindings.
func (m *menu) render(snap tray.Snapshot, v view) bindings {
	var b bindings
	acts := snap.Actions()
	if v.noWSL {
		acts = tray.Actions{}
	}
	status := snap.Status
	local := ""
	if snap.HasStatus {
		local = status.URLs.Local
	}
	busy := snap.State == tray.StateStarting || snap.State == tray.StateStopping

	m.status1.set(tray.StatusLine(snap), false, false)
	m.status2.set("发行版："+distroLabel(v)+" · 启动方式："+modeLabel(snap, v), false, false)
	hint := snap.Reason
	for _, version := range []string{tray.VersionHint(snap), tray.ChannelHint(snap, v.selfUpdate)} {
		if version == "" {
			continue
		}
		if hint != "" {
			hint += "；"
		}
		hint += version
	}
	if hint != "" {
		m.hint1.set(hint, false, false)
	} else {
		m.hint1.hide()
	}
	if v.settingsWarn != "" {
		m.hint2.set(v.settingsWarn, false, false)
	} else {
		m.hint2.hide()
	}
	if status := v.selfUpdate.Status; status != "" {
		m.updateInfo.set(status, false, false)
	} else {
		m.updateInfo.hide()
	}

	access := acts.Access && local != ""
	m.open.set("打开 WebMux", access, false)
	b.remote, b.reported = remoteLinks(snap, v)
	if !v.noWSL {
		b.access = tray.AccessEntries(snap)
	}
	if len(b.access) > len(m.accessLinks) {
		b.access = b.access[:len(m.accessLinks)]
	}
	tunnel := tray.TunnelEntry(snap)
	if access || len(b.remote) > 0 || len(b.access) > 0 {
		m.copyLinks.set("复制链接", true, false)
	} else {
		m.copyLinks.hide()
	}
	if access {
		m.copyLocal.set("本机链接", true, false)
	} else {
		m.copyLocal.hide()
	}
	if len(b.remote) > 0 && snap.HasStatus && exposed(status) {
		m.remoteWarn.set(exposedWarning, false, false)
	} else {
		m.remoteWarn.hide()
	}
	for i, it := range m.remoteLinks {
		if i < b.reported {
			it.set(b.remote[i].Label, acts.Access, false)
		} else {
			it.hide()
		}
	}
	forwarded := b.remote[b.reported:]
	for i, it := range m.forwardLinks {
		if i < len(forwarded) {
			it.set(forwarded[i].Label, acts.Access, false)
		} else {
			it.hide()
		}
	}
	for i, it := range m.accessLinks {
		if i < len(b.access) {
			it.set(b.access[i].Label, true, false)
		} else {
			it.hide()
		}
	}
	if snap.HasStatus && status.TokenConfigured {
		m.copyToken.set("复制 Token", acts.Access, false)
	} else {
		m.copyToken.hide()
	}
	ts := tray.TailscaleEntry(snap)
	if (tunnel.Show || ts.Show) && !v.noWSL {
		m.public.set("公网访问", true, false)
	} else {
		m.public.hide()
	}
	if tunnel.Show && !v.noWSL {
		m.tunnel.set("Cloudflare Tunnel", !busy, tunnel.Checked)
		if tunnel.Info != "" {
			m.tunnelInfo.set(tunnel.Info, false, false)
		} else {
			m.tunnelInfo.hide()
		}
	} else {
		m.tunnel.hide()
		m.tunnelInfo.hide()
	}
	if tunnel.SettingsURL != "" && !v.noWSL {
		m.tunnelSettings.set("Cloudflare 设置…", true, false)
	} else {
		m.tunnelSettings.hide()
	}
	if ts.Show && !v.noWSL {
		m.tailscale.set(ts.Title, !busy, ts.Checked)
		if ts.Info != "" {
			m.tailscaleInfo.set(ts.Info, false, false)
		} else {
			m.tailscaleInfo.hide()
		}
		if ts.SettingsURL != "" {
			m.tsSettings.set("Tailscale 设置…", true, false)
		} else {
			m.tsSettings.hide()
		}
	} else {
		m.tailscale.hide()
		m.tailscaleInfo.hide()
		m.tsSettings.hide()
	}
	if push := tray.PushEntry(snap); push.Show && !v.noWSL {
		m.pushInfo.set(push.Info, false, false)
		m.pushGuide.set("手机推送设置说明…", true, false)
	} else {
		m.pushInfo.hide()
		m.pushGuide.hide()
	}

	m.lanAccess.set("允许局域网访问", !v.noWSL && !busy && snap.HasStatus && !snap.Degraded && snap.Blocker == "", snap.LANEnabled())

	m.start.set("启动", acts.Start, false)
	m.stop.set("停止", acts.Stop, false)
	m.restart.set("重启", acts.Restart, false)
	if install := tray.InstallEntry(snap, v.installSupported); install.Show && !v.noWSL {
		m.install.set(install.Title, install.Enabled, false)
	} else {
		m.install.hide()
	}
	// The tray updates itself even without WSL.
	b.update = tray.UpdateEntry(snap, v.selfUpdate)
	if b.update.Show {
		m.update.set(b.update.Title, b.update.Enabled, false)
	} else {
		m.update.hide()
	}

	serviceLog := !v.noWSL && (systemdMode(snap, v) || (snap.HasStatus && strings.HasPrefix(status.LogFile, "/")))
	m.logService.set("打开 WebMux 日志", serviceLog, false)
	switch {
	case snap.HasStatus && !snap.Degraded && !status.Config.Exists:
		m.openConfig.hide()
		m.initConfig.set("初始化配置（生成 Token）", !v.noWSL, false)
	default:
		m.initConfig.hide()
		m.openConfig.set("打开 WebMux 配置文件", !v.noWSL && snap.HasStatus && strings.HasPrefix(status.Config.Path, "/"), false)
	}

	selectable := !v.noWSL && !busy
	m.distro.set("发行版", selectable, false)
	name := defaultDistro(v.distros)
	defaultTitle := "系统默认"
	if name != "" {
		defaultTitle += "（" + name + "）"
	}
	m.distroDefault.set(defaultTitle, true, v.settings.Distro == "")
	for _, d := range v.distros {
		b.distros = append(b.distros, d.Name)
	}
	if v.settings.Distro != "" && !slices.Contains(b.distros, v.settings.Distro) {
		b.distros = append(b.distros, v.settings.Distro)
	}
	if len(b.distros) > len(m.distros) {
		b.distros = b.distros[:len(m.distros)]
	}
	for i, it := range m.distros {
		if i < len(b.distros) {
			it.set(b.distros[i], true, b.distros[i] == v.settings.Distro)
		} else {
			it.hide()
		}
	}

	m.mode.set("启动方式", selectable, false)
	autoTitle := "自动"
	if current := autoCurrent(snap, v); current != "" {
		autoTitle = "自动（当前：" + current + "）"
	}
	m.modeAuto.set(autoTitle, true, v.settings.Mode == tray.ModeAuto)
	m.modeTray.set("托盘托管", true, v.settings.Mode == tray.ModeTray)
	systemdTitle, systemdOK := "systemd 用户服务", true
	if v.detected {
		if reason := v.detection.Reason(); reason != "" {
			systemdTitle += "（" + reason + "）"
			systemdOK = false
		}
	}
	m.modeSystemd.set(systemdTitle, systemdOK, v.settings.Mode == tray.ModeSystemd)

	m.editSettings.set("编辑托盘设置", !v.noWSL, false)
	m.autostart.set("登录时自动启动", !v.noWSL, v.autostart)
	m.autoUpdate.set("自动更新", true, v.settings.AutoUpdate)
	channel := tray.ChannelEntry(v.selfUpdate)
	m.channel.set(channel.Title, channel.Enabled, false)
	m.channelStable.set("正式版", channel.Enabled, channel.Stable)
	m.channelDev.set("开发版", channel.Enabled, channel.Dev)
	check := tray.CheckEntry(v.selfUpdate)
	m.checkUpdate.set(check.Title, check.Enabled, false)
	m.quit.set("退出", true, false)
	return b
}
