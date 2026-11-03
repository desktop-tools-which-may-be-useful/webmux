#!/bin/sh
# deb prerm / rpm %preun for the WebMux package: removing the package stops
# and disables the automatic update timer and forgets that it was enabled, so
# a later reinstall enables it again; the old package's removal during an
# upgrade leaves both alone. Never touches webmux.service.
#
# Arguments: deb "remove" or "upgrade <new version>"; rpm "0" (erase) or "1" (upgrade).
set -e

case "${1:-}" in
remove | 0)
	if [ -d /run/systemd/system ]; then
		systemctl disable --now webmux-update.timer >/dev/null 2>&1 || true
	fi
	rm -f /var/lib/webmux/auto-update-enabled
	rmdir /var/lib/webmux 2>/dev/null || true
	;;
esac
exit 0
