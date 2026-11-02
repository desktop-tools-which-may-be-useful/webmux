#!/bin/sh
# Install, inspect and remove the deb and rpm packages for this machine's
# architecture in clean Debian and Fedora containers.
#
# Usage: test-linux-packages.sh <directory with the packages> <version>
# DEBIAN_IMAGE / FEDORA_IMAGE override the images (for example a registry
# mirror such as public.ecr.aws/docker/library/debian:stable-slim).
set -eu

dir=$(CDPATH= cd -- "$1" && pwd)
version=$2
debian_image=${DEBIAN_IMAGE:-debian:stable-slim}
fedora_image=${FEDORA_IMAGE:-fedora:latest}

case $(uname -m) in
x86_64 | amd64) deb_arch=amd64 rpm_arch=x86_64 ;;
aarch64 | arm64) deb_arch=arm64 rpm_arch=aarch64 ;;
*)
	printf 'test-linux-packages: unsupported architecture %s\n' "$(uname -m)" >&2
	exit 1
	;;
esac
deb=webmux_${version}-1_${deb_arch}.deb
rpm=webmux-${version}-1.${rpm_arch}.rpm
for file in "$deb" "$rpm"; do
	[ -f "$dir/$file" ] || {
		printf 'test-linux-packages: missing %s/%s\n' "$dir" "$file" >&2
		exit 1
	}
done

# Shared checks after installation: the two files, their modes, the unit's
# command and the version the binary reports.
checks='
	test "$(stat -c %a /usr/bin/webmux)" = 755
	test "$(stat -c %a /usr/lib/systemd/user/webmux.service)" = 644
	grep -qx "ExecStart=/usr/bin/webmux serve" /usr/lib/systemd/user/webmux.service
	test "$(/usr/bin/webmux version)" = "webmux $VERSION"
'
removed='
	test ! -e /usr/bin/webmux
	test ! -e /usr/lib/systemd/user/webmux.service
'

# No dependencies and no maintainer scripts: installing never touches a
# user's service.
docker run --rm -e VERSION="$version" -v "$dir:/pkgs:ro" "$debian_image" sh -euc "
	test -z \"\$(dpkg-deb -f /pkgs/$deb Depends)\"
	test -z \"\$(dpkg-deb --ctrl-tarfile /pkgs/$deb | tar -t | grep -E 'inst|rm\$' || true)\"
	dpkg -i /pkgs/$deb >/dev/null
	$checks
	dpkg -r webmux >/dev/null
	$removed
" || {
	printf 'test-linux-packages: %s failed its checks on Debian\n' "$deb" >&2
	exit 1
}
printf 'ok - %s installs, runs and removes on Debian\n' "$deb"

docker run --rm -e VERSION="$version" -v "$dir:/pkgs:ro" "$fedora_image" sh -euc "
	test -z \"\$(rpm -qp --scripts /pkgs/$rpm)\"
	test \"\$(rpm -qpl /pkgs/$rpm | tr '\n' ' ')\" = '/usr/bin/webmux /usr/lib/systemd/user/webmux.service '
	rpm -i /pkgs/$rpm
	$checks
	rpm -e webmux
	$removed
" || {
	printf 'test-linux-packages: %s failed its checks on Fedora\n' "$rpm" >&2
	exit 1
}
printf 'ok - %s installs, runs and removes on Fedora\n' "$rpm"
