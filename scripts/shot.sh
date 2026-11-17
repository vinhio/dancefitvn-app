#!/usr/bin/env bash
# Adapted from jplayer-mobile/scripts/mobile-shots.sh at 49de3c2.
#
# Draws states of the app and writes, per state, its picture and its
# layout.json (docs/plan/07-testing.md §3.3); with -compare, runs
# cmd/shotcmp against the prototype's captures.
#
#   scripts/shot.sh <ID[,ID...]> [ID ...] ios|android|tester|tvos|tvtester [-compare]
#
# The IDs come before the mode: comma-separated ("TV01,TV02"), as separate
# words (TV01 TV02) or one quoted list ("TV01 TV02"), mixed as one likes.
#
#   ios      the developer's build (scripts/dev-build.sh) on the simulator
#            "DanceFITVN iPhone 16": SIMCTL_CHILD_DFV_STATE=<ID> simctl
#            launch, the app writes Library/Caches/com.jivecode.dancefitvn/
#            dfv-shots/<ID>.layout.json once it rested, then simctl io
#            screenshot. Writes local/shots/app/ios/<ID>.{png,layout.json}.
#   android  the same on the emulator (ANDROID_SERIAL, default
#            emulator-5554): am start --es DFV_STATE <ID>, the layout read
#            with adb root (else from logcat, DFV_SHOT_LOG), adb exec-out
#            screencap. Writes local/shots/app/android/. On a real phone
#            (ANDROID_SERIAL of a device) no setting is changed and the log
#            is not cleared: the status bar shows what it shows.
#   tester   in process, both phones (Core Text for ios, gotext for
#            android): go test ./internal/app -run TestShots. Writes
#            local/shots/app-tester/{ios,android}/.
#   tvos     the developer's tvOS build on the Apple TV simulator (DFV_TVSIM,
#            a name or a UDID, default "DanceFITVN Apple TV"; with -compare
#            DFV_TVSIM1080, default "DanceFITVN Apple TV 1080p"): as ios, the
#            layout from Library/Caches/com.jivecode.dancefitvn/dfv-shots.
#            Writes local/shots/app/tvos/<ID>.{png,layout.json}; a 4K
#            simulator screenshots at 2x (shotcmp -app-scale auto). Before
#            each launch the app is terminated and its process waited out
#            (simctl launch --terminate-running-process alone left tvOS
#            Settings in front about every other launch, the layout all
#            zero); a layout whose window is empty launches again (twice at
#            most).
#   tvtester in process at 1x, Core Text: go test -tags tvos ./internal/app
#            -run TestTVShots. Writes local/shots/app-tester/tv/.
#
# The TV's states are those of internal/app/states_tv.go (TV01 ... TV12,
# TV-toast, and the developer's TV02-expired, TV04-offline, TV-PAIR).
#
# The states are those of internal/app/states_mobile.go (M01, M02, M02B,
# M03B, M03, M04, M05, M05B, and the developer's M02-locked ...): each is
# reached through the real flow over the mock. DFV_NOBUILD=1 skips the
# build and install; DFV_SETTLE=<ms> is the rest before the layout (1500).
# -compare writes local/shots/app-report-<mode>.md and the diff pictures in
# local/shots/diff-app/<mode>/; its exit status is shotcmp's (1: differences).
set -euo pipefail
cd "$(dirname "$0")/.."

usage="usage: scripts/shot.sh <ID[,ID...]> [ID ...] ios|android|tester|tvos|tvtester [-compare]"
ids=()
mode=""
compare=""
for arg in "$@"; do
	case "$arg" in
	ios | android | tester | tvos | tvtester)
		[[ -z "$mode" ]] || { echo "$usage" >&2; exit 2; }
		mode=$arg ;;
	-compare) compare=$arg ;;
	-*) echo "$usage" >&2; exit 2 ;;
	*)
		[[ -z "$mode" ]] || { echo "$usage" >&2; exit 2; }
		IFS=', ' read -r -a more <<<"$arg"
		for id in ${more[@]+"${more[@]}"}; do
			[[ -z "$id" ]] || ids+=("$id")
		done ;;
	esac
done
if [[ -z "$mode" || ${#ids[@]} -eq 0 ]]; then
	echo "$usage" >&2
	exit 2
fi
states=$(IFS=,; echo "${ids[*]}")

bundle=com.jivecode.dancefitvn
sim="DanceFITVN iPhone 16"
settle="${DFV_SETTLE:-1500}"
wait_s=40

shoot_ios() {
	local out=local/shots/app/ios
	mkdir -p "$out"
	if [[ "${DFV_NOBUILD:-}" != 1 ]]; then
		scripts/dev-build.sh ios
		xcrun simctl install "$sim" dist/ios-simulator-arm64/DanceFITVN.app
	fi
	xcrun simctl status_bar "$sim" override --time 9:41 --batteryState charged --batteryLevel 100 \
		--cellularBars 4 --wifiBars 3 --dataNetwork wifi >/dev/null 2>&1 || true
	local data
	data=$(xcrun simctl get_app_container "$sim" "$bundle" data)
	local dir="$data/Library/Caches/$bundle/dfv-shots"
	for id in "${ids[@]}"; do
		rm -f "$dir/$id.layout.json"
		SIMCTL_CHILD_DFV_STATE="$id" SIMCTL_CHILD_DFV_SETTLE="$settle" \
			xcrun simctl launch --terminate-running-process "$sim" "$bundle" >/dev/null
		local n=0
		until [[ -f "$dir/$id.layout.json" ]]; do
			((n++ < wait_s * 4)) || { echo "shot: $id: no layout after ${wait_s}s ($dir)" >&2; exit 1; }
			sleep 0.25
		done
		xcrun simctl io "$sim" screenshot --type=png "$out/$id.png" >/dev/null 2>&1
		cp "$dir/$id.layout.json" "$out/$id.layout.json"
		echo "shot: ios $id -> $out/$id.png"
	done
}

shoot_android() {
	export ANDROID_SERIAL="${ANDROID_SERIAL:-emulator-5554}"
	local out=local/shots/app/android
	mkdir -p "$out"
	if [[ "${DFV_NOBUILD:-}" != 1 ]]; then
		scripts/dev-build.sh android
		adb install -r dist/android-arm64/DanceFITVN.apk >/dev/null
	fi
	local root=1
	if ! adb root 2>/dev/null | grep -qE "restarting|already running as root"; then
		root=0
		echo "shot: no adb root: the layouts come from logcat" >&2
	fi
	adb wait-for-device
	# A real phone (no emulator: ro.kernel.qemu is not 1) is its owner's: no
	# setting changed (stay-on, the System UI demo mode), no keyguard
	# dismissed, its log never cleared (read from the launch's time on); it
	# must be awake and unlocked.
	local phone=1
	[[ "$(adb shell getprop ro.kernel.qemu | tr -d '\r')" == 1 || "$(adb shell getprop ro.boot.qemu | tr -d '\r')" == 1 ]] && phone=0
	if ((phone)); then
		echo "shot: real phone $ANDROID_SERIAL: no system setting changed; keep it awake and unlocked" >&2
	else
		# a sleeping or locked emulator screenshots black
		adb shell svc power stayon true >/dev/null 2>&1 || true
		adb shell input keyevent KEYCODE_WAKEUP >/dev/null
		adb shell wm dismiss-keyguard >/dev/null 2>&1 || true
		adb shell settings put global sysui_demo_allowed 1 >/dev/null
		adb shell am broadcast -a com.android.systemui.demo -e command enter >/dev/null
		adb shell am broadcast -a com.android.systemui.demo -e command clock -e hhmm 0941 >/dev/null
		adb shell am broadcast -a com.android.systemui.demo -e command battery -e level 100 -e plugged false >/dev/null
		adb shell am broadcast -a com.android.systemui.demo -e command notifications -e visible false >/dev/null
	fi
	for id in "${ids[@]}"; do
		adb shell am force-stop "$bundle"
		local file=""
		if ((root)); then
			adb shell "find /data/user/0/$bundle -name '$id.layout.json' -delete" 2>/dev/null || true
		fi
		local since
		since=$(adb shell date +%s | tr -d '\r').000
		((phone)) || adb logcat -c
		adb shell am start -n "$bundle/dev.gdesk.android.GDeskActivity" --es DFV_STATE "$id" \
			--es DFV_SETTLE "$settle" --es DFV_SHOT_LOG "$((1 - root))" >/dev/null
		local n=0
		while :; do
			local line
			line=$(adb logcat -d -T "$since" -s gdesk:I | grep -m1 "dfv: shot ready $id " || true)
			if [[ -n "$line" ]]; then
				file=${line##* }
				break
			fi
			((n++ < wait_s * 4)) || { echo "shot: $id: no layout after ${wait_s}s" >&2; exit 1; }
			sleep 0.25
		done
		adb exec-out screencap -p >"$out/$id.png"
		if ((root)); then
			adb shell cat "$file" | tr -d '\r' >"$out/$id.layout.json"
		else
			adb logcat -d -T "$since" -s gdesk:I | grep "dfv:L $id " | sed 's/.*dfv:L [^ ]* [0-9]*\/[0-9]* //' |
				tr -d '\r\n' | base64 -d >"$out/$id.layout.json"
		fi
		echo "shot: android $id -> $out/$id.png"
	done
}

# sim_udid prints the UDID of the simulator named (or numbered) $1.
sim_udid() {
	xcrun simctl list devices -j | python3 -I -c 'import json, sys
want = sys.argv[1]
for ds in json.load(sys.stdin)["devices"].values():
    for d in ds:
        if want in (d["udid"], d["name"]) and d.get("isAvailable", True):
            print(d["udid"]); sys.exit(0)
sys.exit(1)' "$1"
}

# Adapted from jplayer-tv/scripts/tvos-soak.sh at 781bee5 (app_pid_of):
# app_pids prints the pids of the app's process on the simulator $1.
app_pids() { pgrep -f "Devices/$1/data/Containers/Bundle/Application/.*/DanceFITVN\.app/DanceFITVN" || true; }

# stop_app terminates the app on the simulator $1 and waits until its
# process is gone (10 s at most).
stop_app() {
	local udid=$1 n=0
	xcrun simctl terminate "$udid" "$bundle" >/dev/null 2>&1 || true
	while [[ -n "$(app_pids "$udid")" ]]; do
		((n++ < 40)) || { echo "shot: the app on $udid did not stop in 10s" >&2; return 1; }
		sleep 0.25
	done
}

# layout_drawn tells whether the layout $1 has a window and an element with
# a size (the app was in front when it rested).
layout_drawn() {
	python3 -I -c 'import json, sys
d = json.load(open(sys.argv[1]))
ok = d["screen"]["w"] > 0 and d["screen"]["h"] > 0 and any(e["w"] > 0 and e["h"] > 0 for e in d["elements"])
sys.exit(0 if ok else 1)' "$1"
}

# tv1080 is the 1080p Apple TV simulator of the comparisons (local/plans/P7.md
# P7.7, Q-P7-7): screenshots at 1x, as the prototype's captures. A name or
# a UDID; sim_udid resolves it.
tv1080="${DFV_TVSIM1080:-DanceFITVN Apple TV 1080p}"

shoot_tvos() {
	local tvsim="${DFV_TVSIM:-DanceFITVN Apple TV}"
	[[ -n "${DFV_TVSIM:-}" || -z "$compare" ]] || tvsim=$tv1080
	local udid
	udid=$(sim_udid "$tvsim") || { echo "shot: no simulator $tvsim" >&2; exit 1; }
	local out=local/shots/app/tvos
	mkdir -p "$out"
	if [[ "${DFV_NOBUILD:-}" != 1 ]]; then
		scripts/dev-build.sh tvos
		xcrun simctl install "$udid" dist/tvos-simulator-arm64/DanceFITVN.app
	fi
	local data
	data=$(xcrun simctl get_app_container "$udid" "$bundle" data)
	local dir="$data/Library/Caches/$bundle/dfv-shots"
	for id in "${ids[@]}"; do
		local try
		for try in 1 2 3; do
			rm -f "$dir/$id.layout.json"
			stop_app "$udid"
			SIMCTL_CHILD_DFV_STATE="$id" SIMCTL_CHILD_DFV_SETTLE="$settle" ${DFV_API:+SIMCTL_CHILD_DFV_API="$DFV_API"} \
				xcrun simctl launch "$udid" "$bundle" >/dev/null
			local n=0
			until [[ -f "$dir/$id.layout.json" ]]; do
				((n++ < wait_s * 4)) || { echo "shot: $id: no layout after ${wait_s}s ($dir)" >&2; exit 1; }
				sleep 0.25
			done
			xcrun simctl io "$udid" screenshot --type=png "$out/$id.png" >/dev/null 2>&1
			layout_drawn "$dir/$id.layout.json" && break
			echo "shot: tvos $id: the layout is empty (try $try): the app was not in front" >&2
			((try < 3)) || exit 1
		done
		cp "$dir/$id.layout.json" "$out/$id.layout.json"
		echo "shot: tvos $id -> $out/$id.png"
	done
}

shoot_tvtester() {
	DFV_SHOTS="$PWD/local/shots/app-tester" DFV_SHOT_STATES="$states" \
		go test -count=1 -tags tvos ./internal/app -run '^TestTVShots$'
	echo "shot: tvtester -> local/shots/app-tester/tv"
}

# compare_tv runs shotcmp on the TV's shots against the prototype's 1x
# captures (local/plans/P7.md P7.7: the plane, the QR, the expiry digits and
# the mock pictures masked), with the waivers of DFV_TV_EXCEPT (default
# local/shots/app-exceptions-tv.txt) when the file is there.
compare_tv() {
	local which=$1 app=$2
	local masks=(-mask "TV02:1290,250,340,340" -mask "TV02:388,834,78,38"
		-mask "TV02:1270,780,380,90" -mask "TV02:1370,870,180,90"                  # c1 (K2, removed) and c2 under it
		-mask "TV12:280,200,220,210" -mask "TV12:140,700,540,190"                  # photo and medals (mock pictures)
		-mask "TV12:780,885,700,30"                                                # version line (K2)
		-mask "TV07:656,0,608,1080" -mask "TV08:656,0,608,1080"
		-mask "TV04:1770,42,60,60" -mask "TV05:1770,42,60,60" -mask "TV09:1770,42,60,60"
		-mask "TV12:1770,42,60,60" -mask "TV-toast:1770,42,60,60")
	local args=(-platform tv -proto local/shots/prototype-kit/tv -app "$app" -states "$states"
		-diff "local/shots/diff-app/$which" -report "local/shots/app-report-$which.md" -mode "${DFV_MODE:-layout,full}")
	local except=${DFV_TV_EXCEPT:-local/shots/app-exceptions-tv.txt}
	[[ -f "$except" ]] && args+=(-except-file "$except")
	go run ./cmd/shotcmp "${args[@]}" "${masks[@]}"
}

shoot_tester() {
	DFV_SHOTS="$PWD/local/shots/app-tester" DFV_SHOT_STATES="$states" \
		go test -count=1 ./internal/app -run '^TestShots$'
	echo "shot: tester -> local/shots/app-tester/{ios,android}"
}

# compare runs shotcmp on the shots of a folder: iOS (and the tester's ios)
# at 3x against the prototype's 3x pictures (pixel modes with the masks of
# local/plans/P3.md §4), Android on the layout alone (stretch rule).
compare() {
	local which=$1 app=$2
	local masks=(-mask "0,836,393,16"                          # home indicator
		-mask "M02:64,428,240,26" -mask "M02B:64,506,240,26"     # password placeholder text (K2): input#2 / input.err text row
		-mask "M02B:72,334,300,18"                               # banner sub line (K2): div.muted#1 335.6,15
		-mask "M03:20,68,230,14"                                 # M03 eyebrow div.eb "THỨ BẢY, 14/11 · CHÀO BUỔI SÁNG" (clock)
		-mask "M03:329,69,44,48"                                 # M03 header avatar go.account#1 (not the tab)
		-mask "M06:0,0,393,852" -mask "M06-playing:0,0,393,852"  # 06: the whole screen is the real video frame (K5, P4.8): layout only
		-mask "M06-hidden:0,0,393,852"
		-mask "M07:0,0,393,375"                                
		-mask "M09:20,725,353,17"                                # M09 p.muted "Bộ nhớ trống: ..." (K11: free space)
		-mask "M11:279,156,80,23" -mask "M11:34,191,325,8"       # M11 span.muted "Trống 18,2 GB" and the storage bar (K11)
		-mask "M14:279,156,80,23" -mask "M14:34,191,325,8"
		-mask "M16:20,448,353,18" -mask "M16-code:20,448,353,18")   # P6.7: K24 expiry/hint line p.muted#2 (clock icon svg.i#2 inside)
	# P6.7: avatar circle (image) and the 3 medals (72x72 at badge x+19.5, y 286) on screen 15 and what shows through its sheets' dim
	local s
	for s in M15 M15B-password M15D M15-logout; do
		masks+=(-mask "$s:33,131,82,82" -mask "$s:39.5,286,72,72" -mask "$s:160.5,286,72,72" -mask "$s:281.5,286,72,72")
	done
	masks+=(-mask "M15D:20,393,353,230")                        # P6.7: crop interior (mock picture)
	masks+=(-mask "M-toast:0,0,393,704" -mask "M-toast:0,756,393,96")  # P6.7: pixel part is the toast only
	local args=(-proto local/shots/prototype-kit/mobile -app "$app" -states "$states"
		-diff "local/shots/diff-app/$which" -report "local/shots/app-report-$which.md")
	[[ -f "local/shots/app-exceptions-$which.txt" ]] && args+=(-except-file "local/shots/app-exceptions-$which.txt")
	case "$which" in
	android | tester-android)
		if [[ "$which" == android ]]; then
			# the tab bar grows with the system's bottom inset: 48 with 3-button navigation, 24 with gestures (the prototype draws 34)
			local nav inset=${DFV_BOTTOM_INSET:-}
			if [[ -z "$inset" ]]; then
				nav=$(adb shell settings get secure navigation_mode 2>/dev/null | tr -d '\r' || true)
				case "$nav" in 2) inset=24 ;; *) inset=48 ;; esac
			fi
			args+=(-bottom-inset "$inset")
		fi
		# splash (M01): a flex column centred on the screen, its bar at bottom:120
		# 06 and 07 (P4.8): the controls block (`.vctrl`, chips to prev/next row) sits on the bottom edge: 40 more pt on 412 x 892
		local k
		for k in M06 M06-playing M06-hidden M07; do
			for e in span.chip.y#1 span.chip.k#1 div.d#1 prevLes#1 seek:-10#1 svg.i#7 togglePlay#1 seek:10#1 svg.i#8 nextLes#1; do
				args+=(-vmove "$k:$e=1")
			done
		done
		args+=(-mode layout -vmove "M01:div.d.acc#1=0.5" -vmove "M01:div.eb#1=0.5" -vmove "M01:div.prog#1=1" -vmove "M01:i#1=1") ;;
	*) args+=(-proto-3x local/shots/prototype-kit/mobile@3x -mode "${DFV_MODE:-layout,full}" -text-max 0.08 "${masks[@]}") ;;
	esac
	go run ./cmd/shotcmp "${args[@]}"
}

case "$mode" in
ios) shoot_ios ;;
android) shoot_android ;;
tester) shoot_tester ;;
tvos) shoot_tvos ;;
tvtester) shoot_tvtester ;;
*) echo "$usage" >&2; exit 2 ;;
esac

if [[ "$compare" == -compare ]]; then
	case "$mode" in
	tester)
		status=0
		compare tester-ios local/shots/app-tester/ios || status=$?
		compare tester-android local/shots/app-tester/android || status=$?
		exit "$status"
		;;
	tvos) compare_tv tvos local/shots/app/tvos ;;
	tvtester) compare_tv tvtester local/shots/app-tester/tv ;;
	*) compare "$mode" "local/shots/app/$mode" ;;
	esac
fi
