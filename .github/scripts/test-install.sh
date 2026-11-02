#!/bin/sh
# Exercise install.sh against a local release fixture.
set -eu

repo=$(CDPATH= cd -- "$(dirname -- "$0")/../.." && pwd)
work=$(mktemp -d)
server_pid=
cleanup() {
	if [ -n "$server_pid" ]; then
		kill "$server_pid" 2>/dev/null || true
	fi
	rm -rf "$work"
}
trap cleanup EXIT HUP INT TERM

fail() {
	printf 'test-install: %s\n' "$*" >&2
	exit 1
}

# macOS ships shasum rather than GNU sha256sum.
digest() {
	if command -v sha256sum >/dev/null 2>&1; then
		sha256sum "$1" | awk '{print $1}'
	else
		shasum -a 256 "$1" | awk '{print $1}'
	fi
}

write_sums() {
	(
		cd "$1"
		for file in webmux-*; do
			printf '%s  %s\n' "$(digest "$file")" "$file"
		done > SHA256SUMS
	)
}

# fixture writes a release binary stand-in. It reports a version, logs its
# arguments to $WEBMUX_TEST_LOG and plays `service install`: --dry-run names
# $WEBMUX_TEST_DEFINITION, a real install writes that file (or fails when
# $WEBMUX_TEST_SERVICE_FAIL is 1).
fixture() {
	cat > "$1" <<EOF
#!/bin/sh
# $2
[ -z "\${WEBMUX_TEST_LOG:-}" ] || printf '%s\n' "\$*" >> "\$WEBMUX_TEST_LOG"
case "\$*" in
"service install --dry-run")
	printf '# Service definition: %s\n# Executable must remain at: %s\n' "\$WEBMUX_TEST_DEFINITION" "\$0"
	;;
"service install")
	if [ "\${WEBMUX_TEST_SERVICE_FAIL:-}" = 1 ]; then
		printf 'systemctl is required for user services\n' >&2
		exit 1
	fi
	mkdir -p "\$(dirname "\$WEBMUX_TEST_DEFINITION")"
	printf 'ExecStart=%s serve\n' "\$0" > "\$WEBMUX_TEST_DEFINITION"
	printf 'Installed %s\n' "\$WEBMUX_TEST_DEFINITION"
	;;
*) printf 'webmux 0.2.0\n' ;;
esac
EOF
	chmod 755 "$1"
}

sh -n "$repo/install.sh" || fail "install.sh has a syntax error"
sh -n "$0" || fail "test-install.sh has a syntax error"

www=$work/www
mkdir -p "$www/releases/latest/download"
for os in linux darwin; do
	for arch in amd64 arm64; do
		fixture "$www/releases/latest/download/webmux-$os-$arch" "webmux-$os-$arch latest"
	done
done
write_sums "$www/releases/latest/download"

mkdir -p "$www/releases/download/v0.2.0"
for os in linux darwin; do
	for arch in amd64 arm64; do
		fixture "$www/releases/download/v0.2.0/webmux-$os-$arch" "webmux-$os-$arch pinned"
	done
done
write_sums "$www/releases/download/v0.2.0"

mkdir -p "$www/bad/releases/latest/download"
printf 'bad binary\n' > "$www/bad/releases/latest/download/webmux-linux-amd64"
printf '%064d  webmux-linux-amd64\n' 0 > "$www/bad/releases/latest/download/SHA256SUMS"

port=$(python3 -c 'import socket; s=socket.socket(); s.bind(("127.0.0.1", 0)); print(s.getsockname()[1]); s.close()')
python3 -m http.server "$port" --bind 127.0.0.1 --directory "$www" >/dev/null 2>&1 &
server_pid=$!
server=http://127.0.0.1:$port
tries=0
while ! curl -fsS "$server/" >/dev/null 2>&1; do
	tries=$((tries + 1))
	[ "$tries" -lt 100 ] || fail "local fixture server did not start"
	sleep 0.1
done

mkdir -p "$work/home" "$work/out"

fake_uname() {
	dir=$1
	system=$2
	machine=$3
	mkdir -p "$dir"
	cat > "$dir/uname" <<EOF
#!/bin/sh
case "\$1" in
-s) printf '%s\\n' "$system" ;;
-m) printf '%s\\n' "$machine" ;;
*) printf '%s\\n' "$system" ;;
esac
EOF
	chmod 755 "$dir/uname"
}

# Without WEBMUX_TTY the script would ask on the developer's terminal; point
# it at a missing file so the default runs are non-interactive like CI.
run_install() {
	uname_dir=$1
	shift
	env -u WEBMUX_SERVICE HOME="$work/home" PATH="$uname_dir:$PATH" WEBMUX_INSTALL_DIR="$work/out" \
		WEBMUX_BASE_URL="$base" WEBMUX_MIRRORS="$mirrors" WEBMUX_TTY="$work/no-tty" \
		WEBMUX_TEST_LOG="$work/calls.log" WEBMUX_TEST_DEFINITION="$work/def/webmux.service" "$@" \
		sh "$repo/install.sh"
}

reset_out() {
	rm -rf "$work/out" "$work/def" "$work/calls.log"
	mkdir -p "$work/out"
	: > "$work/calls.log"
}

service_installs() {
	grep -cx 'service install' "$work/calls.log" || true
}

base=$server
mirrors=

fake_uname "$work/uname-linux-amd64" Linux x86_64
reset_out
run_install "$work/uname-linux-amd64" >"$work/linux.log" 2>&1 || {
	cat "$work/linux.log"
	fail "Linux amd64 installation failed"
}
[ -x "$work/out/webmux" ] || fail "Linux amd64 binary was not installed"
grep -q 'webmux-linux-amd64 latest' "$work/out/webmux" || fail "Linux amd64 selected the wrong asset"
[ "$(service_installs)" = 0 ] || fail "non-interactive default installed the user service"
grep -q 'service install$' "$work/linux.log" || fail "non-interactive default did not print the service install hint"

# WEBMUX_SERVICE=1 installs the user service through the new binary.
reset_out
run_install "$work/uname-linux-amd64" WEBMUX_SERVICE=1 >"$work/service.log" 2>&1 || {
	cat "$work/service.log"
	fail "WEBMUX_SERVICE=1 installation failed"
}
[ "$(service_installs)" = 1 ] || fail "WEBMUX_SERVICE=1 did not run service install once"
[ -f "$work/def/webmux.service" ] || fail "WEBMUX_SERVICE=1 wrote no service definition"
grep -q 'loginctl enable-linger' "$work/service.log" || fail "Linux service install did not mention linger"
grep -q 'service status' "$work/service.log" || fail "service install did not print the status command"

# An existing service is neither replaced nor restarted.
run_install "$work/uname-linux-amd64" WEBMUX_SERVICE=1 >"$work/service-again.log" 2>&1 || {
	cat "$work/service-again.log"
	fail "upgrade with an existing service failed"
}
[ "$(service_installs)" = 1 ] || fail "existing service was installed again"
grep -q 'service restart' "$work/service-again.log" || fail "existing service did not get the restart hint"
if grep -q -- '--force' "$work/service-again.log"; then
	fail "existing service for the same executable got the --force hint"
fi

printf 'ExecStart=/usr/local/bin/webmux serve\n' > "$work/def/webmux.service"
run_install "$work/uname-linux-amd64" WEBMUX_SERVICE=1 >"$work/service-other.log" 2>&1 || {
	cat "$work/service-other.log"
	fail "upgrade with a service for another executable failed"
}
[ "$(service_installs)" = 1 ] || fail "service for another executable was replaced"
grep -q "$work/out/webmux service install --force" "$work/service-other.log" ||
	fail "service for another executable did not get the --force hint"
[ "$(cat "$work/def/webmux.service")" = 'ExecStart=/usr/local/bin/webmux serve' ] ||
	fail "service for another executable was modified"
if grep -q 'service restart' "$work/service-other.log"; then
	fail "service for another executable got a restart hint that would not load this version"
fi

# A failing service install keeps the binary and exits non-zero.
reset_out
if run_install "$work/uname-linux-amd64" WEBMUX_SERVICE=1 WEBMUX_TEST_SERVICE_FAIL=1 >"$work/service-fail.log" 2>&1; then
	fail "failed service install exited 0"
fi
[ -x "$work/out/webmux" ] || fail "failed service install removed the binary"
grep -q 'service install failed' "$work/service-fail.log" || grep -q '"service install" failed' "$work/service-fail.log" ||
	fail "failed service install was not reported"
grep -q 'systemctl is required' "$work/service-fail.log" || fail "service install error was hidden"

# On a terminal the script asks; Enter means yes, n means no.
printf '\n' > "$work/tty-yes"
reset_out
run_install "$work/uname-linux-amd64" WEBMUX_TTY="$work/tty-yes" >"$work/ask-yes.log" 2>&1 || {
	cat "$work/ask-yes.log"
	fail "interactive default answer failed"
}
grep -q '\[Y/n\]' "$work/ask-yes.log" || fail "terminal run did not ask"
[ "$(service_installs)" = 1 ] || fail "Enter at the prompt did not install the service"

printf 'n\n' > "$work/tty-no"
reset_out
run_install "$work/uname-linux-amd64" WEBMUX_TTY="$work/tty-no" >"$work/ask-no.log" 2>&1 || {
	cat "$work/ask-no.log"
	fail "interactive no answer failed"
}
[ "$(service_installs)" = 0 ] || fail "answering n installed the service"

# WEBMUX_SERVICE=0 never asks, even on a terminal.
reset_out
run_install "$work/uname-linux-amd64" WEBMUX_SERVICE=0 WEBMUX_TTY="$work/tty-yes" >"$work/service-off.log" 2>&1 || {
	cat "$work/service-off.log"
	fail "WEBMUX_SERVICE=0 installation failed"
}
if grep -q '\[Y/n\]' "$work/service-off.log"; then
	fail "WEBMUX_SERVICE=0 still asked"
fi
[ "$(service_installs)" = 0 ] || fail "WEBMUX_SERVICE=0 installed the service"

# Invalid values fail before anything is downloaded.
reset_out
if run_install "$work/uname-linux-amd64" WEBMUX_SERVICE=yes >"$work/service-bad.log" 2>&1; then
	fail "invalid WEBMUX_SERVICE was accepted"
fi
grep -q 'invalid WEBMUX_SERVICE' "$work/service-bad.log" || fail "invalid WEBMUX_SERVICE error was unclear"
[ -z "$(ls -A "$work/out")" ] || fail "invalid WEBMUX_SERVICE changed the install directory"

# Hide sha256sum so install.sh has to fall back to shasum.
if command -v shasum >/dev/null 2>&1; then
	shasum_only=$work/shasum-only
	mkdir -p "$shasum_only"
	for tool in sh curl mktemp awk tr mkdir mv chmod rm; do
		ln -s "$(command -v "$tool")" "$shasum_only/$tool"
	done
	ln -s "$(command -v shasum)" "$shasum_only/shasum"
	fake_uname "$shasum_only" Linux x86_64
	reset_out
	env HOME="$work/home" PATH="$shasum_only" WEBMUX_INSTALL_DIR="$work/out" \
		WEBMUX_BASE_URL="$base" WEBMUX_MIRRORS= sh "$repo/install.sh" \
		>"$work/shasum.log" 2>&1 || {
		cat "$work/shasum.log"
		fail "shasum fallback installation failed"
	}
	grep -q 'webmux-linux-amd64 latest' "$work/out/webmux" || fail "shasum fallback installed the wrong asset"
fi

fake_uname "$work/uname-darwin-arm64" Darwin arm64
reset_out
run_install "$work/uname-darwin-arm64" >"$work/darwin.log" 2>&1 || {
	cat "$work/darwin.log"
	fail "Darwin arm64 installation failed"
}
grep -q 'webmux-darwin-arm64 latest' "$work/out/webmux" || fail "Darwin arm64 selected the wrong asset"

# Rosetta reports x86_64; sysctl.proc_translated=1 must select arm64.
fake_uname "$work/uname-darwin-rosetta" Darwin x86_64
cat > "$work/uname-darwin-rosetta/sysctl" <<'EOF'
#!/bin/sh
printf '1\n'
EOF
chmod 755 "$work/uname-darwin-rosetta/sysctl"
reset_out
run_install "$work/uname-darwin-rosetta" >"$work/rosetta.log" 2>&1 || {
	cat "$work/rosetta.log"
	fail "Rosetta installation failed"
}
grep -q 'webmux-darwin-arm64 latest' "$work/out/webmux" || fail "Rosetta did not select arm64"

base=$server
mirrors=
fake_uname "$work/uname-linux-pinned" Linux x86_64
reset_out
run_install "$work/uname-linux-pinned" WEBMUX_VERSION=v0.2.0 >"$work/pinned.log" 2>&1 || {
	cat "$work/pinned.log"
	fail "pinned version installation failed"
}
grep -q 'webmux-linux-amd64 pinned' "$work/out/webmux" || fail "pinned version selected the wrong release"

# A pinned digest must work even when the source has no SHA256SUMS.
mkdir -p "$www/nosums/releases/latest/download"
cp "$www/releases/latest/download/webmux-linux-amd64" "$www/nosums/releases/latest/download/"
pinned_sum=$(digest "$www/nosums/releases/latest/download/webmux-linux-amd64")
base=$server/nosums
reset_out
run_install "$work/uname-linux-pinned" WEBMUX_SHA256="$pinned_sum" >"$work/pinned-sum.log" 2>&1 || {
	cat "$work/pinned-sum.log"
	fail "pinned digest installation failed"
}
grep -q 'webmux-linux-amd64 latest' "$work/out/webmux" || fail "pinned digest installed the wrong asset"

# A checksum mismatch must fail and leave the previous binary untouched.
base=$server/bad
mirrors=
reset_out
printf 'old binary\n' > "$work/out/webmux"
chmod 755 "$work/out/webmux"
if run_install "$work/uname-linux-amd64" >"$work/bad.log" 2>&1; then
	cat "$work/bad.log"
	fail "checksum mismatch unexpectedly succeeded"
fi
[ "$(cat "$work/out/webmux")" = "old binary" ] || fail "checksum mismatch replaced the existing binary"
if find "$work/out" -maxdepth 1 -name '.webmux-install.*' | grep -q .; then
	fail "checksum mismatch left a temporary directory"
fi

# The first source 404s; the gh-proxy style mirror prefix must supply the asset.
base=$server/missing
mirrors=$server/mirror
mirror_dir="$www/mirror/http:/127.0.0.1:$port/missing/releases/latest/download"
mkdir -p "$mirror_dir"
for entry in "$www/releases/latest/download"/*; do
	cp "$entry" "$mirror_dir/"
done
reset_out
run_install "$work/uname-linux-amd64" >"$work/mirror.log" 2>&1 || {
	cat "$work/mirror.log"
	fail "mirror fallback installation failed"
}
grep -q 'webmux-linux-amd64 latest' "$work/out/webmux" || fail "mirror fallback installed the wrong asset"

fake_uname "$work/uname-windows" MINGW64_NT-10.0 x86_64
if run_install "$work/uname-windows" >"$work/windows.log" 2>&1; then
	fail "Windows installation unexpectedly succeeded"
fi
grep -q 'Windows' "$work/windows.log" || fail "Windows failure did not mention the supported download"

fake_uname "$work/uname-freebsd" FreeBSD amd64
if run_install "$work/uname-freebsd" >"$work/freebsd.log" 2>&1; then
	fail "unsupported OS unexpectedly succeeded"
fi
grep -q 'unsupported operating system' "$work/freebsd.log" || fail "unsupported OS error was unclear"

printf 'install.sh: platform, checksum, mirror and error-path checks passed\n'
