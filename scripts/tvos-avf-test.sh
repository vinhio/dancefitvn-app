#!/usr/bin/env bash
# Adapted from jplayer-tv/scripts/tvos-avf-test.sh at 781bee5.
#
# tvos-avf-test.sh runs the Apple TV engine (internal/media/avftv) on the
# tvOS Simulator, against the mock's media served on 127.0.0.1 by the test
# app itself (e2e/e2emedia over internal/api/mock).
#
#   scripts/tvos-avf-test.sh unit [device] [args]   build internal/media/avftv's tests into an app, run them
#   scripts/tvos-avf-test.sh e2e [device] [args]    build e2e/tvengine (the engine on the native plane) into an app, run it
#   scripts/tvos-avf-test.sh vet                    go vet with GOOS=ios and the tvos tag
#   scripts/tvos-avf-test.sh device-unit|device-e2e [args]
#                                                   the same on the paired Apple TV
#                                                   (DFV_TV_UDID, required; DFV_TEAM for the remote):
#                                                   built for appletvos, signed by
#                                                   scripts/tvos-device-sign.sh, run by devicectl
#
# device  a simulator name or UDID (default "DanceFITVN Apple TV"). A
#         simulator this script boots, it shuts down after; one already
#         booted is left as it was.
# args    extra arguments for the test binary (-test.run X; -test.v is on).
#
# Media: e2e/prepare-media.sh copies the mock's lesson l_3503 (HLS) and
# sample MP3 into the bundle's media folder; no URL is written in code.
#
# The tests talk to the script with lines "avf-test: <request>":
#   screenshot NAME   simctl screenshot into local/p4/shots/tvos-NAME.png,
#                     copied to the app's tmp as avf-shot-NAME.png
#   background        the TV Settings app comes to the front
#   foreground        this app again
# and the script answers each by creating avf-ack-<request> (spaces as
# dashes) in the app's temporary directory.
#
# Outputs: local/p4/build/<App>.app, local/p4/logs/<mode>-tvos-<time>.log
# (the console), local/p4/shots/. PASS iff the console has a line ^PASS.
#
# On the Apple TV (device-*): a screenshot is devicectl's capture (3840×2160
# at 4K), copied and acknowledged into the app's tmp by devicectl copy to;
# background is the Siri Remote's Home pressed by e2e/remote on the device
# (scripts/tvos-remote-device.sh with DFV_APP, DFV_NOLAUNCH), foreground its
# "front" (no other app is opened). Outputs under DFV_AVF_OUT (default
# local/p8/appletv/avf): <App>.app, <mode>-<time>.log, shots/.
set -euo pipefail

root=$(cd "$(dirname "$0")/.." && pwd)
cd "$root"

cmd=${1:-}
[[ -n $cmd ]] || { sed -n '2,30p' "$0"; exit 2; }
shift

min_os=17.0
# The SDK and clang target suffix: the simulator's, or the Apple TV's for
# the device-* commands.
tv_sdk=appletvsimulator tv_suffix=-simulator tv_platform=AppleTVSimulator
p4=local/p4
build_dir=$p4/build
log_dir=$p4/logs
shot_dir=$p4/shots
mkdir -p "$build_dir" "$log_dir" "$shot_dir"

# ---------------------------------------------------------------------
# Helpers
# ---------------------------------------------------------------------

# go_tvos args...: go with the tvOS Simulator's toolchain (GOOS=ios, cgo).
go_tvos() {
	local sdk cc target
	sdk=$(xcrun --sdk "$tv_sdk" --show-sdk-path)
	cc=$(xcrun --sdk "$tv_sdk" -f clang)
	target="-isysroot $sdk -target arm64-apple-tvos$min_os$tv_suffix"
	GOOS=ios GOARCH=arm64 CGO_ENABLED=1 \
		CC="$cc $target" CGO_CFLAGS="$target" CGO_LDFLAGS="$target" \
		go "$@"
}

# udid_of prints the UDID of a simulator name, or the argument itself.
udid_of() {
	local u
	u=$(xcrun simctl list devices | grep -F "$1 (" | head -1 | grep -oE '[0-9A-F]{8}-[0-9A-F-]{27}' || true)
	echo "${u:-$1}"
}

# boot boots udid unless it runs; it sets booted_here=1 when it booted it.
booted_here=0
boot() {
	local state
	state=$(xcrun simctl list devices | grep "$1" || true)
	[[ -n $state ]] || { echo "no simulator $1" >&2; exit 1; }
	if [[ $state != *"(Booted)"* ]]; then
		echo "+ xcrun simctl boot $1"
		xcrun simctl boot "$1"
		booted_here=1
	fi
	xcrun simctl bootstatus "$1" -b >/dev/null
}

# build_app builds pkg's tests into an app for the Apple TV simulator:
# tvOS family, launch screen, background audio, local networking (the
# media server is on 127.0.0.1), the mock's media in its media folder.
build_app() {
	rm -rf "$app"
	mkdir -p "$app/media"
	echo "+ go test -c -tags tvos -o $app/$exe $pkg (GOOS=ios, $tv_sdk)"
	go_tvos test -c -tags tvos -o "$app/$exe" "$pkg"
	e2e/prepare-media.sh "$app/media"
	write_plist
}

# write_plist writes the app's Info.plist and signs it.
write_plist() {
	local sdk_version sdk_build
	sdk_version=$(xcrun --sdk "$tv_sdk" --show-sdk-version)
	sdk_build=$(xcrun --sdk "$tv_sdk" --show-sdk-build-version)
	cat >"$app/Info.plist" <<PLIST
<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
	<key>CFBundleDevelopmentRegion</key><string>en</string>
	<key>CFBundleExecutable</key><string>$exe</string>
	<key>CFBundleIdentifier</key><string>$bundle_id</string>
	<key>CFBundleInfoDictionaryVersion</key><string>6.0</string>
	<key>CFBundleName</key><string>$name</string>
	<key>CFBundlePackageType</key><string>APPL</string>
	<key>CFBundleShortVersionString</key><string>1.0</string>
	<key>CFBundleVersion</key><string>1</string>
	<key>CFBundleSupportedPlatforms</key><array><string>$tv_platform</string></array>
	<key>DTPlatformName</key><string>$tv_sdk</string>
	<key>DTPlatformVersion</key><string>$sdk_version</string>
	<key>DTSDKBuild</key><string>$sdk_build</string>
	<key>DTSDKName</key><string>$tv_sdk$sdk_version</string>
	<key>MinimumOSVersion</key><string>$min_os</string>
	<key>NSAppTransportSecurity</key><dict><key>NSAllowsLocalNetworking</key><true/></dict>
	<key>UIApplicationSceneManifest</key><dict>
		<key>UIApplicationSupportsMultipleScenes</key><false/>
	</dict>
	<key>UIBackgroundModes</key><array><string>audio</string></array>
	<key>UIDeviceFamily</key><array><integer>3</integer></array>
	<key>UILaunchScreen</key><dict/>
</dict>
</plist>
PLIST
	plutil -lint "$app/Info.plist" >/dev/null
	if [[ $tv_sdk == appletvos ]]; then
		plutil -insert UIRequiredDeviceCapabilities -json '["arm64"]' "$app/Info.plist"
		scripts/tvos-device-sign.sh "$app"
	else
		codesign --force --sign - --timestamp=none "$app" >/dev/null
	fi
}

# ack answers a request of the tests: a file in the app's tmp directory.
ack() {
	local udid=$1 request=$2 dir
	dir=$(xcrun simctl get_app_container "$udid" "$bundle_id" data)/tmp
	mkdir -p "$dir"
	touch "$dir/avf-ack-${request// /-}"
}

# run builds the app, installs it on the device and runs it, answering the
# tests' requests; the verdict is the console's PASS.
run() {
	local device=$1 udid log
	shift
	udid=$(udid_of "$device")
	build_app
	boot "$udid"
	if ((booted_here)); then
		trap 'echo "+ xcrun simctl shutdown $udid"; xcrun simctl shutdown "$udid" >/dev/null 2>&1 || true' EXIT
	fi
	xcrun simctl install "$udid" "$app"
	log=$log_dir/$mode-tvos-$(date +%Y%m%d-%H%M%S).log
	echo "+ simctl launch $device $bundle_id -test.v $* (console in $log)"
	set +e
	xcrun simctl launch --console-pty --terminate-running-process "$udid" "$bundle_id" \
		-test.v -test.timeout=20m "$@" 2>&1 | while IFS= read -r line; do
		line=${line%$'\r'} # the pty's CRLF; a filter here would buffer the requests
		printf '%s\n' "$line" | tee -a "$log"
		case "$line" in
		*"avf-test: screenshot "*)
			name=$(printf '%s' "${line##*avf-test: screenshot }" | tr -cd 'A-Za-z0-9._-')
			xcrun simctl io "$udid" screenshot "$shot_dir/tvos-$name.png" >/dev/null 2>&1 || true
			cp "$shot_dir/tvos-$name.png" "$(xcrun simctl get_app_container "$udid" "$bundle_id" data)/tmp/avf-shot-$name.png" 2>/dev/null || true
			ack "$udid" "screenshot $name"
			;;
		*"avf-test: background"*)
			xcrun simctl launch "$udid" com.apple.TVSettings >/dev/null 2>&1 || true
			ack "$udid" "background"
			;;
		*"avf-test: foreground"*)
			xcrun simctl launch "$udid" "$bundle_id" >/dev/null 2>&1 || true
			ack "$udid" "foreground"
			;;
		esac
	done
	set -e
	echo "--- measurements"
	grep 'avf-measure:' "$log" || true
	grep -q '^PASS' "$log"
}

# dev_put copies the local file $1 into the test app's data container as
# $2 (a path in it, tmp/...).
dev_put() {
	xcrun devicectl device copy to --device "$tv_udid" --domain-type appDataContainer \
		--domain-identifier "$bundle_id" --source "$1" --destination "$2" >"$build_dir/put.log" 2>&1 && return 0
	# The test deletes an answer as soon as it sees it: devicectl's look at
	# the file it copied then fails (CoreDeviceError 7000), the copy made.
	grep -q "Failed to retrieve the file node" "$build_dir/put.log" && return 0
	{ echo "avf-device: devicectl copy to $2:"; grep -iE "error|fail" "$build_dir/put.log" | head -3; } >>"$log_dir/$mode-put-errors.txt"
	return 1
}

# dev_ack answers a request of the tests on the Apple TV.
dev_ack() {
	: >"$build_dir/ack"
	dev_put "$build_dir/ack" "tmp/avf-ack-${1// /-}" || echo "avf-device: ack $1 failed" >&2
}

# run_device builds the app for the Apple TV, installs it and runs it with
# devicectl, answering the tests' requests; the verdict is the console's
# PASS.
run_device() {
	local log line shot
	build_app
	xcrun devicectl device install app --device "$tv_udid" "$app" >"$build_dir/install.log" 2>&1 ||
		{ cat "$build_dir/install.log" >&2; exit 1; }
	log=$log_dir/$mode-$(date +%Y%m%d-%H%M%S).log
	echo "+ devicectl launch $tv_udid -- $bundle_id -test.v $* (console in $log)"
	set +e
	# "--" before the bundle: devicectl leaves what follows to the app, and
	# Go's flag package sees no bare "--" (it would stop at it).
	xcrun devicectl device process launch --device "$tv_udid" --terminate-existing --console \
		-- "$bundle_id" -test.v -test.timeout=20m "$@" 2>&1 | while IFS= read -r line; do
		line=${line%$'\r'}
		printf '%s\n' "$line" | tee -a "$log"
		case "$line" in
		*"avf-test: screenshot "*)
			shot=$(printf '%s' "${line##*avf-test: screenshot }" | tr -cd 'A-Za-z0-9._-')
			xcrun devicectl device capture screenshot --device "$tv_udid" --destination "$shot_dir/tvos-$shot.png" >/dev/null 2>&1 || true
			dev_put "$shot_dir/tvos-$shot.png" "tmp/avf-shot-$shot.png" || true
			dev_ack "screenshot $shot"
			;;
		*"avf-test: background"*)
			DFV_APP=$bundle_id DFV_NOLAUNCH=1 DFV_REMOTE_OUT=$log_dir DFV_TV_UDID=$tv_udid \
				scripts/tvos-remote-device.sh "home wait:1000 state:after-home" "$mode-background" >>"$log_dir/$mode-remote.txt" 2>&1 || true
			dev_ack "background"
			# The watchdog: an app sent to the background with nothing
			# playing is suspended there and cannot ask to come back; with
			# no new request 14 s on, the script brings it to the front.
			(
				n=$(grep -c 'avf-test:' "$log")
				sleep 14
				if [[ $(grep -c 'avf-test:' "$log") == "$n" ]]; then
					echo "avf-device: no request for 14 s after background: front (the app was suspended)" | tee -a "$log"
					DFV_APP=$bundle_id DFV_NOLAUNCH=1 DFV_REMOTE_OUT=$log_dir DFV_TV_UDID=$tv_udid \
						scripts/tvos-remote-device.sh "front state:after-front" "$mode-watchdog" >>"$log_dir/$mode-remote.txt" 2>&1 || true
				fi
			) &
			;;
		*"avf-test: foreground"*)
			DFV_APP=$bundle_id DFV_NOLAUNCH=1 DFV_REMOTE_OUT=$log_dir DFV_TV_UDID=$tv_udid \
				scripts/tvos-remote-device.sh "front state:after-front" "$mode-foreground" >>"$log_dir/$mode-remote.txt" 2>&1 || true
			dev_ack "foreground"
			;;
		esac
	done
	set -e
	echo "--- measurements"
	grep 'avf-measure:' "$log" || true
	grep -q '^PASS' "$log"
}

# ---------------------------------------------------------------------
# Commands
# ---------------------------------------------------------------------

mode=$cmd
case $cmd in
unit)
	device=${1:-DanceFITVN Apple TV}
	[[ $# -gt 0 ]] && shift
	pkg=./internal/media/avftv exe=avftv.test name=AVFTVTests
	app=$build_dir/AVFTVTests.app bundle_id=com.jivecode.dancefitvn.avftvtests
	run "$device" "$@"
	;;
e2e)
	device=${1:-DanceFITVN Apple TV}
	[[ $# -gt 0 ]] && shift
	pkg=./e2e/tvengine exe=tvengine.test name=TVEngine
	app=$build_dir/TVEngine.app bundle_id=com.jivecode.dancefitvn.tvengine
	run "$device" "$@"
	;;
vet)
	go_tvos vet -tags tvos ./internal/media/... ./e2e/...
	;;
device-unit | device-e2e)
	tv_sdk=appletvos tv_suffix="" tv_platform=AppleTVOS
	tv_udid=${DFV_TV_UDID:?set DFV_TV_UDID to the UDID of the Apple TV, see xcrun devicectl list devices}
	out=${DFV_AVF_OUT:-local/p8/appletv/avf}
	build_dir=$out/build log_dir=$out shot_dir=$out/shots
	mkdir -p "$build_dir" "$shot_dir"
	if [[ $cmd == device-unit ]]; then
		pkg=./internal/media/avftv exe=avftv.test name=AVFTVTests
		app=$build_dir/AVFTVTests.app bundle_id=com.jivecode.dancefitvn.avftvtests
	else
		pkg=./e2e/tvengine exe=tvengine.test name=TVEngine
		app=$build_dir/TVEngine.app bundle_id=com.jivecode.dancefitvn.tvengine
	fi
	run_device "$@"
	;;
*)
	echo "unknown command $cmd" >&2
	exit 2
	;;
esac
