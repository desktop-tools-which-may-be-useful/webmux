#!/bin/sh
# deb postrm / rpm %postun for the WebMux package: forget the removed unit
# files. Never touches webmux.service.
set -e

if [ -d /run/systemd/system ]; then
	systemctl daemon-reload >/dev/null 2>&1 || true
fi
exit 0
