#!/bin/sh
# Adapted from e2e/devtest-ios.sh (itself from jplayer-mobile/e2e/devtest-ios.sh
# at 49de3c2) for a physical iPhone, P8: builds the package's tests for the
# iphoneos SDK, signs them (e2e/ios-device-sign.sh), installs with devicectl,
# runs them with the console streamed back, and uninstalls at the end:
#   e2e/devtest-iphone.sh [pkg] [-test.run X]
# DFV_IPHONE_UDID is the phone's UDID (required), DFV_E2E_BG the
# seconds of the background phase (default 15). The phone must be awake and
# unlocked. Nothing on it but this app is changed (the app is uninstalled
# afterwards; DFV_KEEP=1 keeps it).
# DFV_FACEID=1 puts NSFaceIDUsageDescription into the test app's plist.
# The driver lines "devtest: <n> <cmd>" of the tests: home (Calculator in
# front, so the app goes to the background; an inert app, no setting is
# touched), front, shot-NAME (devicectl screenshot into
# local/p4/shots/iphone-NAME.png, copied to the app's tmp as shot-NAME.png).
# Everything else answers "none". The log lands in local/p8/iphone/logs.
set -eu
here=$(cd "$(dirname "$0")" && pwd)
root=$(cd "$here/.." && pwd)
case "${1:-}" in
-* | "") pkg=./e2e/engine ;;
*) pkg=$1; shift ;;
esac
dev=${DFV_IPHONE_UDID:?set DFV_IPHONE_UDID to the UDID of the iPhone, see xcrun devicectl list devices}
out=$(mktemp -d)
logs="$root/local/p8/iphone/logs"
shots="$root/local/p8/iphone/e2e-shots"
mkdir -p "$logs" "$shots"
appid=com.jivecode.dancefitvn.e2e
cleanup() {
	touch "$out/ended"
	[ "${DFV_KEEP:-}" = 1 ] || xcrun devicectl device uninstall app --device "$dev" "$appid" >/dev/null 2>&1 || true
	rm -rf "$out"
}
trap cleanup EXIT
sdk=$(xcrun --sdk iphoneos --show-sdk-path)
target="-isysroot $sdk -target arm64-apple-ios17.0"
app="$out/E2E.app"
mkdir -p "$app/media"
(cd "$root" && GOOS=ios GOARCH=arm64 CGO_ENABLED=1 \
	CC="$(xcrun --sdk iphoneos -f clang) $target" CGO_CFLAGS="$target" CGO_LDFLAGS="$target" \
	go test -c -o "$app/e2e.test" "$pkg")
"$here/prepare-media.sh" "$app/media"
cat >"$app/Info.plist" <<PLIST
<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
	<key>CFBundleExecutable</key><string>e2e.test</string>
	<key>CFBundleIdentifier</key><string>$appid</string>
	<key>CFBundleName</key><string>DanceFITVN e2e</string>
	<key>CFBundlePackageType</key><string>APPL</string>
	<key>CFBundleVersion</key><string>1</string>
	<key>CFBundleShortVersionString</key><string>1.0</string>
	<key>MinimumOSVersion</key><string>17.0</string>
	<key>CFBundleSupportedPlatforms</key><array><string>iPhoneOS</string></array>
	<key>UIDeviceFamily</key><array><integer>1</integer><integer>2</integer></array>
	<key>UILaunchScreen</key><dict/>
	<key>UIApplicationSceneManifest</key><dict>
		<key>UIApplicationSupportsMultipleScenes</key><false/>
	</dict>
	<key>NSAppTransportSecurity</key><dict><key>NSAllowsLocalNetworking</key><true/></dict>
	<key>NSPhotoLibraryUsageDescription</key><string>e2e</string>
	${DFV_FACEID:+<key>NSFaceIDUsageDescription</key><string>e2e</string>}
	<key>UIBackgroundModes</key><array><string>audio</string></array>
</dict>
</plist>
PLIST
"$here/ios-device-sign.sh" "$app"
xcrun devicectl device install app --device "$dev" "$app" >/dev/null
fdev() { xcrun devicectl device "$@" --device "$dev"; }
# drive runs the commands the output asks for and has not run yet.
drive() {
	tr -d '\r' <"$out/log" 2>/dev/null | grep -E '^devtest: [0-9]+ [a-z0-9-]+$' | while read -r _ n cmd; do
		[ -e "$out/driven-$n" ] && continue
		reply=
		case "$cmd" in
		home) xcrun devicectl device process launch --device "$dev" com.apple.calculator >/dev/null 2>&1 && reply=ok ;;
		front) xcrun devicectl device process launch --device "$dev" "$appid" >/dev/null 2>&1 && reply=ok ;;
		shot-*)
			name=${cmd#shot-}
			if xcrun devicectl device capture screenshot --device "$dev" --destination "$shots/iphone-$name.png" >/dev/null 2>&1 &&
				xcrun devicectl device copy to --device "$dev" --domain-type appDataContainer --domain-identifier "$appid" \
					--source "$shots/iphone-$name.png" --destination "tmp/shot-$name.png" >/dev/null 2>&1; then reply=ok; fi
			;;
		*) reply="unknown command: $cmd" ;;
		esac
		printf '%s\n' "${reply:-none}" >"$out/answer-$n"
		xcrun devicectl device copy to --device "$dev" --domain-type appDataContainer --domain-identifier "$appid" \
			--source "$out/answer-$n" --destination "tmp/devtest-$n.txt" >/dev/null 2>&1 || true
		touch "$out/driven-$n"
	done
}
: >"$out/log"
(while [ ! -e "$out/ended" ]; do drive; sleep 1; done) &
driving=$!
xcrun devicectl device process launch --device "$dev" --console --terminate-existing \
	--environment-variables "{\"DFV_E2E_BG\":\"${DFV_E2E_BG:-}\"}" -- "$appid" -test.v "$@" 2>&1 | tee "$out/log" || true
touch "$out/ended"
wait "$driving" 2>/dev/null || true
tr -d '\r' <"$out/log" >"$out/log.clean"
name=$(basename "$pkg")
cp "$out/log.clean" "$logs/$name-iphone-$(date +%Y%m%d-%H%M%S).log"
if ! grep -qE '^(PASS|FAIL)$' "$out/log.clean"; then
	echo "FAIL: devtest-iphone: the app ended without a verdict (crash?)" >&2
	exit 1
fi
grep -q '^PASS$' "$out/log.clean"
