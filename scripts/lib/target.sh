# shellcheck shell=bash
# Adapted from scripts/shot.sh (sim_udid, app_pids, stop_app, the Android
# launch), scripts/tvos-remote-device.sh (the devicectl launch with its
# console, pid_of), scripts/tvos-memgate.sh (footprint_bytes) and
# jplayer-mobile/scripts/m11-bg.sh at 49de3c2 (the console of a launch read
# while it runs, dumpsys meminfo).
#
# target.sh is sourced by the developer's measuring scripts (perf-*.sh,
# a11y-dump.sh): it picks the system the app runs on and launches,
# watches, stops and samples a developer's build of DanceFITVN there.
#
#   source scripts/lib/target.sh
#   target_init ios|android|tvos      # sets T_KIND, T_ID, T_LABEL
#   target_build                      # dev build + install (DFV_NOBUILD=1: install only)
#   target_launch <log> [K=V ...]     # a cold launch, the console into <log>
#   target_wait <log> <regex> <s>     # 0 once a line matches, 1 after s seconds
#   target_stop                       # the app stopped, the console closed
#
# The target, by the environment (no default device: scripts/README.md
# "Devices"):
#   ios      DFV_IPHONE_UDID set: that iPhone (built -device, signed with
#            e2e/ios-device-sign.sh, installed and launched with devicectl);
#            else the simulator DFV_SIM ("DanceFITVN iPhone 16"), booted.
#   tvos     DFV_TV_UDID set: that Apple TV (scripts/tvos-device-sign.sh);
#            else the simulator DFV_TVSIM ("DanceFITVN Apple TV", the 4K one).
#   android  ANDROID_SERIAL (required): the emulator or the phone adb names.
# On a device nothing of the system is changed (no setting, no keyguard,
# its log never cleared): the scripts install, launch, stop and read only
# com.jivecode.dancefitvn. It must be awake and unlocked.

T_BUNDLE=com.jivecode.dancefitvn
T_ACTIVITY=dev.gdesk.android.GDeskActivity
T_CONSOLE="" # the pid of the process streaming the app's console

# t_now_ms prints the Mac's Unix time in milliseconds.
t_now_ms() { python3 -I -c 'import time; print(int(time.time() * 1000))'; }

# t_sim_udid prints the UDID of the simulator named (or numbered) $1.
t_sim_udid() {
	xcrun simctl list devices -j | python3 -I -c 'import json, sys
want = sys.argv[1]
for ds in json.load(sys.stdin)["devices"].values():
    for d in ds:
        if want in (d["udid"], d["name"]) and d.get("isAvailable", True):
            print(d["udid"]); sys.exit(0)
sys.exit(1)' "$1"
}

# target_init picks the target of system $1 (see above).
target_init() {
	T_SYS=$1
	case $T_SYS in
	ios)
		if [[ -n ${DFV_IPHONE_UDID:-} ]]; then
			T_KIND=device T_ID=$DFV_IPHONE_UDID T_LABEL=iphone
			T_APP=dist/ios-arm64/DanceFITVN.app
		else
			T_KIND=sim T_LABEL=ios-sim
			T_ID=$(t_sim_udid "${DFV_SIM:-DanceFITVN iPhone 16}") || { echo "no simulator ${DFV_SIM:-DanceFITVN iPhone 16}" >&2; exit 2; }
			T_APP=dist/ios-simulator-arm64/DanceFITVN.app
		fi
		;;
	tvos)
		if [[ -n ${DFV_TV_UDID:-} ]]; then
			T_KIND=device T_ID=$DFV_TV_UDID T_LABEL=appletv
			T_APP=dist/tvos-arm64/DanceFITVN.app
		else
			T_KIND=sim T_LABEL=tvos-sim
			T_ID=$(t_sim_udid "${DFV_TVSIM:-DanceFITVN Apple TV}") || { echo "no simulator ${DFV_TVSIM:-DanceFITVN Apple TV}" >&2; exit 2; }
			T_APP=dist/tvos-simulator-arm64/DanceFITVN.app
		fi
		;;
	android)
		T_KIND=android T_ID=${ANDROID_SERIAL:?set ANDROID_SERIAL to the emulator or phone adb lists (adb devices -l)}
		T_APP=dist/android-arm64/DanceFITVN.apk
		export ANDROID_SERIAL
		export JAVA_HOME="${JAVA_HOME:-/opt/homebrew/opt/openjdk@21/libexec/openjdk.jdk/Contents/Home}"
		export ANDROID_HOME="${ANDROID_HOME:-/opt/homebrew/share/android-commandlinetools}"
		adb get-state >/dev/null 2>&1 || { echo "adb does not see $ANDROID_SERIAL" >&2; exit 2; }
		T_PHONE=1
		[[ "$(adb shell getprop ro.kernel.qemu | tr -d '\r')" == 1 || "$(adb shell getprop ro.boot.qemu | tr -d '\r')" == 1 ]] && T_PHONE=0
		if ((T_PHONE)); then T_LABEL=android-phone; else T_LABEL=android-emulator; fi
		;;
	*)
		echo "unknown system $T_SYS: ios, android or tvos" >&2
		exit 2
		;;
	esac
	if [[ $T_KIND == sim ]] && ! xcrun simctl list devices | grep -q "$T_ID) (Booted)"; then
		echo "the simulator $T_ID is not booted (xcrun simctl boot …)" >&2
		exit 2
	fi
}

# target_build makes the developer's build for the target and installs it
# (DFV_NOBUILD=1: installs the one in dist/ as it is).
target_build() {
	local log=${1:-/dev/null}
	if [[ -z ${DFV_NOBUILD:-} ]]; then
		case $T_SYS-$T_KIND in
		ios-sim) scripts/dev-build.sh ios >"$log" 2>&1 ;;
		ios-device) scripts/dev-build.sh ios -device >"$log" 2>&1 && DFV_IPHONE_UDID=$T_ID e2e/ios-device-sign.sh "$T_APP" >>"$log" 2>&1 ;;
		tvos-sim) scripts/dev-build.sh tvos >"$log" 2>&1 ;;
		tvos-device) scripts/dev-build.sh tvos -device >"$log" 2>&1 && DFV_TV_UDID=$T_ID scripts/tvos-device-sign.sh "$T_APP" >>"$log" 2>&1 ;;
		android-*) scripts/dev-build.sh android >"$log" 2>&1 ;;
		esac || { echo "the build failed: see $log" >&2; exit 1; }
	fi
	case $T_KIND in
	sim) xcrun simctl install "$T_ID" "$T_APP" ;;
	device) xcrun devicectl device install app --device "$T_ID" "$T_APP" >>"$log" 2>&1 || { echo "install failed: see $log" >&2; exit 1; } ;;
	android) adb install -r "$T_APP" >/dev/null ;;
	esac
}

# target_pid prints the app's process id, or nothing when it does not run.
target_pid() {
	case $T_KIND in
	sim) pgrep -f "Devices/$T_ID/data/Containers/Bundle/Application/.*/DanceFITVN\.app/DanceFITVN" | head -1 || true ;;
	device)
		xcrun devicectl device info processes --device "$T_ID" --json-output - 2>/dev/null |
			python3 -I -c 'import json,sys
d = json.load(sys.stdin)
for p in d.get("result", {}).get("runningProcesses", []):
    if str(p.get("executable", "")).endswith("/DanceFITVN.app/DanceFITVN"):
        print(p.get("processIdentifier")); break' || true
		;;
	android) adb shell pidof "$T_BUNDLE" 2>/dev/null | tr -d '\r' || true ;;
	esac
}

# target_stop stops the app and the console's stream, and waits until the
# process is gone (15 s at most).
target_stop() {
	local n=0 pid
	case $T_KIND in
	sim) xcrun simctl terminate "$T_ID" "$T_BUNDLE" >/dev/null 2>&1 || true ;;
	device)
		pid=$(target_pid)
		[[ -n $pid ]] && xcrun devicectl device process terminate --device "$T_ID" --pid "$pid" >/dev/null 2>&1 || true
		;;
	android) adb shell am force-stop "$T_BUNDLE" ;;
	esac
	if [[ -n $T_CONSOLE ]]; then
		kill "$T_CONSOLE" 2>/dev/null || true
		wait "$T_CONSOLE" 2>/dev/null || true
		T_CONSOLE=""
	fi
	while [[ -n "$(target_pid)" ]]; do
		((n++ < 60)) || { echo "the app did not stop in 15 s" >&2; return 1; }
		sleep 0.25
	done
}

# target_launch launches the app cold (stopped first) with the variables
# K=V given, its console streamed into the file $1 (the Android log of the
# app's tag from the launch on: stdout and stderr are logcat's tag gdesk).
# T_LAUNCH_MS is the Mac's time just before the launch command.
target_launch() {
	local log=$1 kv json
	shift
	target_stop || true
	: >"$log"
	case $T_KIND in
	sim)
		local envs=()
		for kv in "$@"; do envs+=("SIMCTL_CHILD_$kv"); done
		T_LAUNCH_MS=$(t_now_ms)
		env ${envs[@]+"${envs[@]}"} xcrun simctl launch --console-pty "$T_ID" "$T_BUNDLE" >"$log" 2>&1 &
		T_CONSOLE=$!
		;;
	device)
		json=$(python3 -I -c 'import json, sys
print(json.dumps(dict(a.split("=", 1) for a in sys.argv[1:])))' "$@")
		local envarg=()
		[[ $json != "{}" ]] && envarg=(--environment-variables "$json") # devicectl refuses an empty object
		T_LAUNCH_MS=$(t_now_ms)
		xcrun devicectl device process launch --device "$T_ID" --terminate-existing --console \
			${envarg[@]+"${envarg[@]}"} "$T_BUNDLE" >"$log" 2>&1 &
		T_CONSOLE=$!
		;;
	android)
		local extras=() since
		for kv in "$@"; do extras+=(--es "${kv%%=*}" "${kv#*=}"); done
		since=$(adb shell date +%s | tr -d '\r').000
		adb logcat -v raw -T "$since" -s 'gdesk:*' >"$log" 2>/dev/null &
		T_CONSOLE=$!
		T_LAUNCH_MS=$(t_now_ms)
		adb shell am start -W -n "$T_BUNDLE/$T_ACTIVITY" ${extras[@]+"${extras[@]}"} >"$log.am" 2>&1
		;;
	esac
}

# target_wait waits for a line of the log $1 matching the extended regexp
# $2, $3 seconds at most: 0 when one does, 1 when none came (or the
# console's stream ended, the app gone).
target_wait() {
	local log=$1 re=$2 n=0 limit=$((${3:-60} * 4))
	until tr -d '\r' <"$log" 2>/dev/null | grep -qE "$re"; do
		((n++ < limit)) || return 1
		if [[ -n $T_CONSOLE ]] && ! kill -0 "$T_CONSOLE" 2>/dev/null; then
			tr -d '\r' <"$log" 2>/dev/null | grep -qE "$re" && return 0
			return 1
		fi
		sleep 0.25
	done
}

# target_mem_bytes prints the memory of the app's process $1 in bytes: on
# a simulator its phys_footprint (footprint, the Mac's process), on
# Android its total PSS (dumpsys meminfo); nothing where it cannot be read
# from the Mac (a device: see perf-video.sh, xctrace).
target_mem_bytes() {
	case $T_KIND in
	sim)
		footprint -p "$1" -f bytes --noCategories 2>/dev/null |
			awk '/[Ff]ootprint:/ { for (i = 1; i <= NF; i++) if ($i ~ /^[0-9]+$/) { print $i; exit } }' || true
		;;
	android)
		adb shell dumpsys meminfo "$T_BUNDLE" 2>/dev/null | tr -d '\r' |
			awk '/TOTAL PSS:/ { for (i = 1; i <= NF; i++) if ($i == "PSS:") { print $(i + 1) * 1024; exit } }
			     /^ *TOTAL +[0-9]/ { print $2 * 1024; exit }' || true
		;;
	esac
}

# target_shot writes a screenshot of the target into the file $1.
target_shot() {
	case $T_KIND in
	sim) xcrun simctl io "$T_ID" screenshot --type=png "$1" >/dev/null 2>&1 || true ;;
	device) xcrun devicectl device capture screenshot --device "$T_ID" --destination "$1" >/dev/null 2>&1 || true ;;
	android) adb exec-out screencap -p >"$1" 2>/dev/null || true ;;
	esac
}

# target_wake keeps an emulator's screen on and unlocked (a phone is its
# owner's: nothing is done to it).
target_wake() {
	[[ $T_KIND == android ]] && ((T_PHONE == 0)) || return 0
	adb shell svc power stayon true >/dev/null 2>&1 || true
	adb shell input keyevent KEYCODE_WAKEUP >/dev/null 2>&1 || true
	adb shell wm dismiss-keyguard >/dev/null 2>&1 || true
}

# target_ax_off turns the simulator's accessibility runtime off for a
# measure (com.apple.Accessibility ApplicationAccessibilityEnabled, the
# simulator's own defaults; scripts/a11y-dump.sh turns it on): loaded, it
# adds about 2 s to a launch of the iPhone simulator (P8.4a: 3.4 s against
# 1.3 s to Home). target_ax_restore puts the value back. T_AX tells the
# value the measure ran with ("off", or "device" where nothing is changed).
T_AX_WAS=""
T_AX=device
target_ax_off() {
	[[ $T_KIND == sim ]] || return 0
	T_AX_WAS=$(xcrun simctl spawn "$T_ID" defaults read com.apple.Accessibility ApplicationAccessibilityEnabled 2>/dev/null || echo unset)
	xcrun simctl spawn "$T_ID" defaults write com.apple.Accessibility ApplicationAccessibilityEnabled -bool false
	T_AX=off
}
target_ax_restore() {
	[[ $T_KIND == sim && -n $T_AX_WAS ]] || return 0
	case $T_AX_WAS in
	unset) xcrun simctl spawn "$T_ID" defaults delete com.apple.Accessibility ApplicationAccessibilityEnabled 2>/dev/null || true ;;
	1 | true | YES) xcrun simctl spawn "$T_ID" defaults write com.apple.Accessibility ApplicationAccessibilityEnabled -bool true ;;
	*) xcrun simctl spawn "$T_ID" defaults write com.apple.Accessibility ApplicationAccessibilityEnabled -bool false ;;
	esac
	T_AX_WAS=""
}
