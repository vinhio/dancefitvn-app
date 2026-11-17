#!/bin/sh
# Adapted from jplayer-mobile/e2e/devtest-ios.sh at 49de3c2.
#
# Builds the tests of one package for the iOS simulator and runs them in a
# gDesk app on it, the output on the terminal and in local/p4/logs:
#   e2e/devtest-ios.sh [pkg] [-test.run X]
# pkg is a path from the repository's top (default ./e2e/engine); when the
# first argument starts with -, it is a test flag and pkg is the default.
# After the package, the arguments go to the test binary (-test.v is on).
# GDESK_SIMULATOR names the device (name or UDID; default "DanceFITVN
# iPhone 16"). A simulator this script boots, it shuts down after.
# DFV_E2E_BG gives the seconds of the background phase (default 15).
# The app plays on in the background (UIBackgroundModes audio, as the
# app's gdesk.json asks). The tests drive what only the Mac can through
# lines "devtest: <n> <cmd>" in their output, which the script runs as it
# reads them and answers in devtest-<n>.txt of the app's tmp folder:
# home (Settings launched, so the app goes to the background), front (the
# app launched again), shot-NAME (a screenshot into
# local/p4/shots/ios-NAME.png, copied to the app's tmp as shot-NAME.png
# for the test to read). See e2e/engine/driver_test.go.
# The media comes from e2e/prepare-media.sh (the mock's) and lands in the
# bundle's media folder; the tests serve it on 127.0.0.1 themselves.
set -eu
here=$(cd "$(dirname "$0")" && pwd)
root=$(cd "$here/.." && pwd)
case "${1:-}" in
-* | "") pkg=./e2e/engine ;;
*) pkg=$1; shift ;;
esac
out=$(mktemp -d)
trap 'rm -rf "$out"' EXIT
logs="$root/local/p4/logs"
shots="$root/local/p4/shots"
mkdir -p "$logs" "$shots"
sdk=$(xcrun --sdk iphonesimulator --show-sdk-path)
target="-isysroot $sdk -target arm64-apple-ios17.0-simulator"
app="$out/E2E.app"
mkdir -p "$app/media"
(cd "$root" && GOOS=ios GOARCH=arm64 CGO_ENABLED=1 \
	CC="$(xcrun --sdk iphonesimulator -f clang) $target" CGO_CFLAGS="$target" CGO_LDFLAGS="$target" \
	go test -c -o "$app/e2e.test" "$pkg")
"$here/prepare-media.sh" "$app/media"
cat >"$app/Info.plist" <<PLIST
<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
	<key>CFBundleExecutable</key><string>e2e.test</string>
	<key>CFBundleIdentifier</key><string>com.jivecode.dancefitvn.e2e</string>
	<key>CFBundleName</key><string>DanceFITVN e2e</string>
	<key>CFBundlePackageType</key><string>APPL</string>
	<key>CFBundleVersion</key><string>1</string>
	<key>CFBundleShortVersionString</key><string>1.0</string>
	<key>MinimumOSVersion</key><string>17.0</string>
	<key>CFBundleSupportedPlatforms</key><array><string>iPhoneSimulator</string></array>
	<key>UIDeviceFamily</key><array><integer>1</integer><integer>2</integer></array>
	<key>UILaunchScreen</key><dict/>
	<key>UIApplicationSceneManifest</key><dict>
		<key>UIApplicationSupportsMultipleScenes</key><false/>
	</dict>
	<key>NSAppTransportSecurity</key><dict><key>NSAllowsLocalNetworking</key><true/></dict>
	<key>UIBackgroundModes</key><array><string>audio</string></array>
</dict>
</plist>
PLIST
codesign --force --sign - "$app" >/dev/null 2>&1
device=${GDESK_SIMULATOR:-DanceFITVN iPhone 16}
booted=$(xcrun simctl list devices booted | grep -c "$device" || true)
if [ "$booted" = 0 ]; then
	xcrun simctl boot "$device"
	trap 'rm -rf "$out"; xcrun simctl shutdown "$device" >/dev/null 2>&1' EXIT
fi
xcrun simctl bootstatus "$device" -b >/dev/null
xcrun simctl install "$device" "$app"
appid=com.jivecode.dancefitvn.e2e
# The app's data outlives a run: drop the driver's answers and shots of the
# run before.
rm -f "$(xcrun simctl get_app_container "$device" "$appid" data)"/tmp/devtest-*.txt \
	"$(xcrun simctl get_app_container "$device" "$appid" data)"/tmp/shot-*.png 2>/dev/null || true
# drive runs the commands the output asks for and has not run yet.
drive() {
	tr -d '\r' <"$out/log" 2>/dev/null | grep -E '^devtest: [0-9]+ [a-z0-9-]+$' | while read -r _ n cmd; do
		[ -e "$out/driven-$n" ] && continue
		reply=
		data=$(xcrun simctl get_app_container "$device" "$appid" data)
		case "$cmd" in
		home) xcrun simctl launch "$device" com.apple.Preferences >/dev/null 2>&1 && reply=ok ;;
		front) xcrun simctl launch "$device" "$appid" >/dev/null 2>&1 && reply=ok ;;
		shot-*)
			name=${cmd#shot-}
			xcrun simctl io "$device" screenshot "$shots/ios-$name.png" >/dev/null 2>&1 &&
				cp "$shots/ios-$name.png" "$data/tmp/shot-$name.png" && reply=ok
			;;
		*) reply="unknown command: $cmd" ;;
		esac
		printf '%s\n' "${reply:-none}" >"$data/tmp/devtest-$n.txt"
		touch "$out/driven-$n"
	done
}
: >"$out/log"
(while [ ! -e "$out/ended" ]; do drive; sleep 1; done) &
driving=$!
SIMCTL_CHILD_DFV_E2E_BG=${DFV_E2E_BG:-} \
	xcrun simctl launch --console-pty --terminate-running-process "$device" "$appid" -test.v "$@" 2>&1 | tee "$out/log"
touch "$out/ended"
wait "$driving" 2>/dev/null || true
# simctl launch --console-pty returns when the app's process ends, so a crash
# or an exit without a verdict fails at once, not after a timeout.
# The pty gives CRLF lines: drop the CRs before the verdict is looked for.
tr -d '\r' <"$out/log" >"$out/log.clean"
cp "$out/log.clean" "$logs/engine-ios-$(date +%Y%m%d-%H%M%S).log"
if ! grep -qE '^(PASS|FAIL)$' "$out/log.clean"; then
	echo "FAIL: devtest-ios: the app ended without a verdict (crash?)" >&2
	exit 1
fi
grep -q '^PASS$' "$out/log.clean"
