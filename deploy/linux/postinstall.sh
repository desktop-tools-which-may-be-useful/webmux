#!/bin/sh
# deb postinst / rpm %post for the WebMux package. The automatic update
# timer is enabled once per installation: on a first install, and on an
# upgrade from a release that never shipped it. The marker records that this
# happened, so an administrator's later `systemctl disable` survives every
# upgrade; removing the package deletes it. Never touches webmux.service:
# restarting it would end every terminal session.
set -e

marker=/var/lib/webmux/auto-update-enabled

# Containers and chroots without a running systemd install the files only;
# a later upgrade on a booted system still enables the timer.
if [ -d /run/systemd/system ]; then
	systemctl daemon-reload >/dev/null 2>&1 || true
	if [ ! -e "$marker" ]; then
		if systemctl enable --now webmux-update.timer >/dev/null 2>&1; then
			mkdir -p /var/lib/webmux
			: >"$marker"
		fi
	fi
fi
exit 0
