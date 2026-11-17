#!/usr/bin/env bash
# Sets the network a developer's DanceFITVN sees (P5.1c, local/plans/P5.md
# F-6), without touching the Mac's own network.
#
#   scripts/net.sh ios|android none|wifi|cellular|auto
#
# ios: the simulator has no airplane mode and no network control, so a
# developer's build (scripts/dev-build.sh, the dancefitvndev tag) reads the
# kind to force from a file of its data container every second
# (internal/app/network_mobile.go): Library/Application Support/
# com.jivecode.dancefitvn/dev/network. "auto" gives the system's kind back
# (Wi-Fi on the simulator). DFV_SIM names the simulator (default: booted).
#
# android: the emulator's real switches (ANDROID_SERIAL, default
# emulator-5554): none = airplane mode; cellular = airplane off, Wi-Fi off,
# mobile data on; wifi and auto = airplane off, Wi-Fi and mobile data on.
# Put Wi-Fi and data back (auto) after a run.
set -euo pipefail

sys="${1:?usage: scripts/net.sh ios|android none|wifi|cellular|auto}"
kind="${2:?usage: scripts/net.sh ios|android none|wifi|cellular|auto}"
case "$kind" in
none | wifi | cellular | auto) ;;
*)
	echo "net.sh: unknown kind $kind (none, wifi, cellular, auto)" >&2
	exit 2
	;;
esac

case "$sys" in
ios)
	app_id=com.jivecode.dancefitvn
	data="$(xcrun simctl get_app_container "${DFV_SIM:-booted}" "$app_id" data)"
	dir="$data/Library/Application Support/$app_id/dev"
	mkdir -p "$dir"
	printf '%s\n' "$kind" >"$dir/network.tmp"
	mv "$dir/network.tmp" "$dir/network"
	echo "net.sh: ios network $kind ($dir/network)"
	;;
android)
	adb=("${ANDROID_HOME:-$HOME/Library/Android/sdk}/platform-tools/adb" -s "${ANDROID_SERIAL:-emulator-5554}")
	case "$kind" in
	none)
		"${adb[@]}" shell cmd connectivity airplane-mode enable
		;;
	cellular)
		"${adb[@]}" shell cmd connectivity airplane-mode disable
		"${adb[@]}" shell svc wifi disable
		"${adb[@]}" shell svc data enable
		;;
	wifi | auto)
		"${adb[@]}" shell cmd connectivity airplane-mode disable
		"${adb[@]}" shell svc wifi enable
		"${adb[@]}" shell svc data enable
		;;
	esac
	echo "net.sh: android network $kind"
	;;
*)
	echo "net.sh: unknown system $sys (ios, android)" >&2
	exit 2
	;;
esac
