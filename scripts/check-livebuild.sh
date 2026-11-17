#!/usr/bin/env bash
# Adapted from jplayer-mobile/scripts/check-storebuild.sh at 49de3c2.
#
# Checks that the live build of DanceFITVN (the live tag, the build that
# ships: local/plans/P8.md P8.1b) has none of the mock and none of a
# developer's parts: neither the mock's package nor its JSON nor its media
# nor its mock-media:// scheme nor its overlay, and no shot state, shot
# writer or developer's variable, in the program.
#
#   scripts/check-livebuild.sh [ios] [tvos] [android]    (all three when none is named)
#   scripts/check-livebuild.sh --check <app|ipa|apk|aab|binary>...
#   scripts/check-livebuild.sh -h | --help
#
# For each system:
#  1. go list -deps of the main package (GOOS, CGO_ENABLED=1, -tags live,
#     tvos,live for tvOS) must list no dancefitvn-app/internal/api/mock.
#  2. gdesk build -os <system> -tags live, of a snapshot of the working
#     tree in a temporary folder: dist/ is left alone (another build, a
#     developer's, may be in it).
#  3. The program it built (the .app's executable, the .apk's libmain.so)
#     must hold none of the MARKERS and every one of the REQUIRED texts
#     (api.DefaultBase: the live API is what it talks to).
# A developer's build for the Mac (go build -tags dancefitvndev, the phone
# shell) is checked to hold the markers, so that a marker gone stale shows
# (WARN); the TV's own markers (TV_ONLY) are not in the phone shell and are
# skipped there.
#
# DFV_LIVEBUILD_KEEP=<dir> keeps a copy of each build in <dir>/<system>/.
#
# --check scans builds made elsewhere (scripts/release.sh) for the markers
# only. Exit status: 0 when every check passes, 1 on a failure, 2 on a
# usage error.
set -euo pipefail

usage() { awk 'NR > 1 && /^#/ { sub(/^# ?/, ""); print; next } NR > 1 { exit }' "$0"; }

# MARKERS are texts of the mock and of a developer's build.
MARKERS=(
	# The mock's package and folder (internal/api/mock, the root's
	# mockfs.go), its JSON (routes.json keys, the demo user, the pairing's
	# confirmed answer), its media (the sample HLS playlist and MP3) and the
	# scheme of its media URLs (overlaytag.go).
	"dancefitvn-app/internal/api/mock"
	"dancefitvn-mock-api"
	"default_delay_ms"
	"template_tokens"
	"thaovy.hlv@gmail.com"
	"pair-poll.confirmed"
	"#EXTM3U"
	"_sample.mp3"
	"mock-media://"
	# The overlay of the mock's data (store.Overlay's key).
	"mock-overlay"
	# The shot states (states_mobile.go, states_account_mobile.go,
	# states_tv.go), their seed account, the shot writer (devshot.go) and
	# the smoke states of a developer's build (devsmoke_mobile.go).
	"seed@example.invalid"
	"M02-locked"
	"OVERLAY-RESET"
	"TV01-confirm"
	"TV02-expired"
	"dfv-shots"
	"dfv: shot ready"
	"SMOKE-PLAY"
	# The performance hooks and states and the accessibility dump of a
	# developer's build (devperf*.go, devaccess.go; P8.3d, P8.4a).
	"dancefitvn: mark home"
	"PERF-SCROLL"
	"PERF-VIDEO10"
	"dfv: perf scroll"
	"dfv: a11y ready"
	# A developer's variables (devenv.go, devtransport.go, apimode.go).
	"DFV_STATE"
	"DFV_A11Y"
	"DFV_PERF_MIN"
	"DFV_FRAMELOG"
	"DFV_API"
	"DFV_LIVE_ROUTES"
	"DFV_OFFLINE"
)

# TV_ONLY are markers of the TV shell, which the phone shell's control
# build cannot hold.
TV_ONLY=("TV01-confirm" "TV02-expired")

# FORBIDDEN is the package no live build links.
FORBIDDEN='^dancefitvn-app/internal/api/mock$'

root="$(cd "$(dirname "$0")/.." && pwd)"

# REQUIRED are texts the live build must hold: the base of the live API,
# read from api.DefaultBase (internal/api/live.go), its only place.
base=$(sed -n 's/^const DefaultBase = "\(.*\)"$/\1/p' "$root/internal/api/live.go")
[ -n "$base" ] || { echo "FAIL  no api.DefaultBase in internal/api/live.go"; exit 1; }
REQUIRED=("$base")
export JAVA_HOME="${JAVA_HOME:-/opt/homebrew/opt/openjdk@21/libexec/openjdk.jdk/Contents/Home}"
export ANDROID_HOME="${ANDROID_HOME:-/opt/homebrew/share/android-commandlinetools}"

fail=0
ok() { echo "ok    $*"; }
warn() { echo "WARN  $*"; }
bad() { echo "FAIL  $*"; fail=1; }

tmp=$(mktemp -d)
trap 'rm -rf "$tmp"' EXIT

# binary prints the program of the build $1 (a folder of $tmp to unpack in
# is $2).
binary() {
	local b=$1 x=$2
	case $b in
	*.app)
		echo "$b/$(plutil -extract CFBundleExecutable raw -o - "$b/Info.plist")"
		;;
	*.ipa)
		mkdir -p "$x" && unzip -qo "$b" 'Payload/*' -d "$x"
		local app
		app=$(echo "$x"/Payload/*.app)
		echo "$app/$(plutil -extract CFBundleExecutable raw -o - "$app/Info.plist")"
		;;
	*.apk | *.aab)
		mkdir -p "$x" && unzip -qo "$b" '*lib/arm64-v8a/libmain.so' -d "$x"
		find "$x" -name libmain.so | head -1
		;;
	*)
		echo "$b"
		;;
	esac
}

# scan checks the program $1 (named $2) for the markers and the required
# texts.
scan() {
	local bin=$1 name=$2 m n found=0
	[ -f "$bin" ] || { bad "$name: no program found"; return; }
	for m in "${MARKERS[@]}"; do
		n=$(LC_ALL=C grep -acF -- "$m" "$bin" || true)
		if [ "$n" != 0 ]; then
			bad "$name: holds \"$m\""
			found=1
		fi
	done
	if [ $found = 0 ]; then
		ok "$name: none of the ${#MARKERS[@]} markers ($(du -h "$bin" | cut -f1 | tr -d ' '))"
	fi
	for m in "${REQUIRED[@]}"; do
		n=$(LC_ALL=C grep -acF -- "$m" "$bin" || true)
		if [ "$n" = 0 ]; then
			bad "$name: lacks \"$m\""
		else
			ok "$name: holds \"$m\""
		fi
	done
}

# deps checks what the main package links for system $1 with the live tag.
deps() {
	local sys=$1 goos=$1 tags=live out
	case $sys in
	tvos) goos=ios tags=tvos,live ;;
	esac
	out=$(cd "$root" && GOOS=$goos GOARCH=arm64 CGO_ENABLED=1 GOFLAGS= go list -deps -tags "$tags" .)
	if echo "$out" | grep -E "$FORBIDDEN"; then
		bad "$sys: go list -deps -tags $tags links the package above"
	else
		ok "$sys: go list -deps -tags $tags links no internal/api/mock ($(echo "$out" | grep -c '^dancefitvn-app') packages of the app)"
	fi
}

# control checks that a developer's build for the Mac holds the markers:
# a marker it lacks proves nothing.
control() {
	local bin="$tmp/dev" m n skip t
	(cd "$root" && GOFLAGS= go build -tags dancefitvndev -o "$bin" .)
	for m in "${MARKERS[@]}"; do
		skip=0
		for t in "${TV_ONLY[@]}"; do [ "$m" = "$t" ] && skip=1; done
		[ $skip = 1 ] && continue
		n=$(LC_ALL=C grep -acF -- "$m" "$bin" || true)
		[ "$n" != 0 ] || warn "control: a developer's build lacks \"$m\": the marker is stale"
	done
	ok "control: a developer's build for the Mac holds the markers (TV's aside)"
}

# snapshot copies the working tree that the app builds from into $tmp/src.
snapshot() {
	[ -d "$tmp/src" ] && return
	mkdir -p "$tmp/src"
	rsync -a --exclude '/.git' --exclude '/dist' --exclude '/local' --exclude '/reference' \
		--exclude '/e2e' --exclude '/dancefitvn-app' --exclude '*.test' --exclude '.DS_Store' "$root/" "$tmp/src/"
}

# last prints the last line of gdesk's output that names an existing path.
last() { local p=""; while IFS= read -r l; do [ -e "$l" ] && p=$l; done; echo "$p"; }

# build makes the live build for $1 and scans it.
build() {
	local sys=$1 out
	snapshot
	out=$(cd "$tmp/src" && GOFLAGS= gdesk build -os "$sys" -tags live 2>"$tmp/build-$sys.log" | last) || true
	if [ -z "$out" ]; then
		out=$(cd "$tmp/src" && find dist -maxdepth 2 \( -name '*.app' -o -name '*.apk' \) -newer go.mod 2>/dev/null | grep -E "^dist/$sys" | head -1 || true)
		[ -n "$out" ] && out="$tmp/src/$out"
	fi
	if [ -z "$out" ] || [ ! -e "$out" ]; then
		tail -20 "$tmp/build-$sys.log" >&2
		bad "$sys: gdesk build -os $sys -tags live failed"
		return
	fi
	ok "$sys: gdesk build -os $sys -tags live ($(basename "$out"))"
	if [ -n "${DFV_LIVEBUILD_KEEP:-}" ]; then
		mkdir -p "$DFV_LIVEBUILD_KEEP/$sys"
		rm -rf "${DFV_LIVEBUILD_KEEP:?}/$sys/$(basename "$out")"
		cp -R "$out" "$DFV_LIVEBUILD_KEEP/$sys/"
	fi
	scan "$(binary "$out" "$tmp/x-$sys")" "$sys"
}

case "${1:-}" in
-h | --help) usage; exit 0 ;;
--check)
	shift
	[ $# -gt 0 ] || { usage >&2; exit 2; }
	i=0
	for b in "$@"; do
		i=$((i + 1))
		[ -e "$b" ] || { bad "$b: no such build"; continue; }
		scan "$(binary "$b" "$tmp/c$i")" "$b"
	done
	exit $fail
	;;
esac

systems=("$@")
[ ${#systems[@]} -gt 0 ] || systems=(ios tvos android)
for sys in "${systems[@]}"; do
	case $sys in
	ios | tvos | android) ;;
	*) usage >&2; exit 2 ;;
	esac
done
control
for sys in "${systems[@]}"; do
	deps "$sys"
	build "$sys"
done
if [ $fail = 0 ]; then echo "check-livebuild: ok"; else echo "check-livebuild: FAILED"; fi
exit $fail
