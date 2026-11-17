#!/usr/bin/env bash
# Adapted from jplayer-tv/scripts/tvos-store-check.sh at 781bee5.
#
# tvos-store-check.sh checks the TV's state on the Apple TV simulator
# (P7.5d, local/plans/P7.md Q-P7-6): the app's Mirror of NSUserDefaults
# (primary) and the state folder in Library/Caches (copy), for the keys
# prefs, progress-queue and device, across relaunches, a purge of
# Library/Caches, spoilt slots, a reboot, an install over and a reinstall,
# through e2e/tvstore.
#
#   scripts/tvos-store-check.sh <command> [device]
#
# command  build  build local/p7/p7.5d/build/TVStore.app only
#          run    build, then nine launches, each living 6 s before
#                 simctl terminate; exits 0 only with 9 + 1 PASS: the nine
#                 launches, and the purge-restore row (step 3 read every
#                 key from NSUserDefaults with the Caches copy gone, its
#                 save wrote the files again, step 4 read both slots fine)
#          app    the app itself (P7.5c, R-P7-7): the developer's build
#                 (scripts/dev-build.sh tvos) paired over the mock
#                 (DFV_STATE=TV04) with DFV_PREFS=quality=720p,autonext=off,
#                 then a relaunch, an install over, a purge of Library/Caches
#                 and an uninstall + install; exits 0 only with 5 PASS: the
#                 preferences survive all but the uninstall, the session
#                 survives the relaunch and the install over, and the purge
#                 brings TV01 back (the tokens live in Caches until G1).
#                 DFV_NOBUILD=1 installs dist/ as it is. Logs:
#                 local/p7/p75c/store/app-<n>-<event>.log, app-summary.txt
#          test   run internal/store/tvstore's tvOS tests in the simulator
#                 (simctl spawn of the test binary)
#          vet    go vet with GOOS=ios and the tvos tag
# device   a simulator name or UDID (default "DanceFITVN Apple TV")
#
# Step 7 shuts the device down and boots it again. A device booted at the
# start is left booted at the end; one this script booted is shut down
# unless STORECHECK_KEEP=1.
#
# Outputs: local/p7/p7.5d/build/{TVStore.app,tvstore.test};
# local/p7/p7.5d/logs/tvstore-<n>-<event>.log (console, NSUserDefaults
# plist, the copy on disk) and tvstore-summary.txt. No URL, token or
# real address is involved (ids v1..v8, *@example.invalid).
set -euo pipefail

root=$(cd "$(dirname "$0")/.." && pwd)
cd "$root"

cmd=${1:-}
[[ -n $cmd ]] || { sed -n '2,41p' "$0"; exit 2; }
device=${2:-DanceFITVN Apple TV}

min_os=17.0
out_dir=local/p7/p7.5d
build_dir=$out_dir/build
log_dir=$out_dir/logs
mkdir -p "$build_dir" "$log_dir"

app_name=TVStore
exe=tvstore
bundle_id=com.jivecode.dancefitvn.tvstore
app=$build_dir/$app_name.app
app_id=com.jivecode.dancefitvn # sys.AppID: the folder under Library/Caches
prefix=dancefitvn.v1.          # tvstore.Prefix

# ---------------------------------------------------------------------
# Build.
# ---------------------------------------------------------------------

# go_tvos args...: go with the tvOS Simulator's toolchain (GOOS=ios, cgo).
go_tvos() {
	local sdk cc target
	sdk=$(xcrun --sdk appletvsimulator --show-sdk-path)
	cc=$(xcrun --sdk appletvsimulator -f clang)
	target="-isysroot $sdk -target arm64-apple-tvos$min_os-simulator"
	GOOS=ios GOARCH=arm64 CGO_ENABLED=1 \
		CC="$cc $target" CGO_CFLAGS="$target" CGO_LDFLAGS="$target" \
		go "$@"
}

build_app() {
	local sdk_version sdk_build
	sdk_version=$(xcrun --sdk appletvsimulator --show-sdk-version)
	sdk_build=$(xcrun --sdk appletvsimulator --show-sdk-build-version)
	rm -rf "$app"
	mkdir -p "$app"
	echo "+ GOOS=ios GOARCH=arm64 CGO_ENABLED=1 go build -tags tvos -o $app/$exe ./e2e/tvstore"
	go_tvos build -tags tvos -o "$app/$exe" ./e2e/tvstore
	xcrun vtool -show-build "$app/$exe" | grep -E "platform|minos|sdk"
	cat >"$app/Info.plist" <<EOF
<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
	<key>CFBundleDevelopmentRegion</key><string>en</string>
	<key>CFBundleDisplayName</key><string>$app_name</string>
	<key>CFBundleExecutable</key><string>$exe</string>
	<key>CFBundleIdentifier</key><string>$bundle_id</string>
	<key>CFBundleInfoDictionaryVersion</key><string>6.0</string>
	<key>CFBundleName</key><string>$app_name</string>
	<key>CFBundlePackageType</key><string>APPL</string>
	<key>CFBundleShortVersionString</key><string>0.0.1</string>
	<key>CFBundleSupportedPlatforms</key><array><string>AppleTVSimulator</string></array>
	<key>CFBundleVersion</key><string>1</string>
	<key>DTPlatformName</key><string>appletvsimulator</string>
	<key>DTPlatformVersion</key><string>$sdk_version</string>
	<key>DTSDKBuild</key><string>$sdk_build</string>
	<key>DTSDKName</key><string>appletvsimulator$sdk_version</string>
	<key>MinimumOSVersion</key><string>$min_os</string>
	<key>UIApplicationSceneManifest</key><dict>
		<key>UIApplicationSupportsMultipleScenes</key><false/>
	</dict>
	<key>UIDeviceFamily</key><array><integer>3</integer></array>
	<key>UILaunchScreen</key><dict/>
</dict>
</plist>
EOF
	plutil -lint "$app/Info.plist" >/dev/null
	codesign --force --sign - --timestamp=none "$app" 2>/dev/null
	codesign --verify "$app"
	echo "built $app"
}

# ---------------------------------------------------------------------
# Simulator helpers. Always by UDID: other simulators may be booted too.
# ---------------------------------------------------------------------

# udid_of prints the UDID of a simulator name, or the argument itself.
udid_of() {
	local u
	u=$(xcrun simctl list devices | grep -F "$1 (" | head -1 | grep -oE '[0-9A-F]{8}-[0-9A-F-]{27}' || true)
	echo "${u:-$1}"
}

udid=$(udid_of "$device")
name=$(printf '%s' "$device" | tr -c 'A-Za-z0-9._-' '-')

# state prints the device's line of simctl list, failing when there is
# none.
state() {
	local line
	line=$(xcrun simctl list devices | grep "$udid" || true)
	[[ -n $line ]] || { echo "no simulator $udid" >&2; exit 1; }
	echo "$line"
}

boot() {
	if [[ $(state) != *"(Booted)"* ]]; then
		echo "+ xcrun simctl boot $udid"
		xcrun simctl boot "$udid"
	fi
	xcrun simctl bootstatus "$udid" -b >/dev/null
}

# container prints the app's data container.
container() { xcrun simctl get_app_container "$udid" "$bundle_id" data 2>/dev/null || true; }

# dump prints the two slots as they are on disk.
dump() {
	local data=$1
	echo "--- data container: $data"
	echo "--- NSUserDefaults plist on disk ($(date +%H:%M:%S))"
	python3 -I - "$data/Library/Preferences/$bundle_id.plist" "$prefix" <<'PY' 2>&1 || true
import os, plistlib, sys
if not os.path.exists(sys.argv[1]):
    sys.exit("(no plist)")
with open(sys.argv[1], "rb") as f:
    for k, v in sorted(plistlib.load(f).items()):
        if k.startswith(sys.argv[2]):
            print(f"{k} = {v.decode('utf-8', 'replace') if isinstance(v, bytes) else v!r}")
PY
	echo "--- copy on disk: Library/Caches/$app_id/state"
	for k in prefs progress-queue device; do
		printf '%s.json = ' "$k"
		cat "$data/Library/Caches/$app_id/state/$k.json" 2>/dev/null || printf '(missing)'
		echo
	done
}

# launch n event args...: one launch with the console in its log, 6 s,
# terminate; then the store's two slots as they are on disk.
launch() {
	local n=$1 event=$2 launcher
	shift 2
	local log=$log_dir/tvstore-$n-$event.log
	echo "+ xcrun simctl launch --console-pty --terminate-running-process $udid $bundle_id $* > $log"
	xcrun simctl launch --console-pty --terminate-running-process "$udid" "$bundle_id" "$@" >"$log" 2>&1 &
	launcher=$!
	sleep 6
	xcrun simctl terminate "$udid" "$bundle_id" 2>/dev/null || true
	wait "$launcher" 2>/dev/null || true
	dump "$(container)" >>"$log"
	grep -E "^tvstore: " "$log" | tr -d '\r' || { echo "no tvstore lines:"; cat "$log"; }
	sed -n '/^--- NSUserDefaults/,$p' "$log"
}

install_fresh() {
	xcrun simctl uninstall "$udid" "$bundle_id" 2>/dev/null || true
	xcrun simctl install "$udid" "$app"
}

# purge empties the app's Library/Caches, as tvOS does when storage runs
# low: the state folder, and the secret folder (install id, F1) with it.
purge() {
	local data
	data=$(container)
	[[ -n $data && -d $data/Library/Caches ]] || { echo "no Caches in '$data'" >&2; exit 1; }
	echo "+ rm -rf <container>/Library/Caches/*"
	find "$data/Library/Caches" -mindepth 1 -maxdepth 1 -exec rm -rf {} +
	{
		echo "--- Library/Caches after the purge:"
		find "$data/Library/Caches" | sed "s#^$data/##"
		echo "--- end"
	} | tee "$log_dir/tvstore-3-purge-before.txt"
}

# ---------------------------------------------------------------------
# run: the nine launches.
# ---------------------------------------------------------------------
run_check() {
	local booted_at_start=""
	build_app
	[[ $(state) == *"(Booted)"* ]] && booted_at_start=1
	boot
	rm -f "$log_dir"/tvstore-*.log "$log_dir"/tvstore-*.txt
	echo "== 1 uninstall + install"
	install_fresh
	launch 1 fresh 1-fresh -expect=defaults -write=v1
	echo "== 2 terminate + launch"
	launch 2 relaunch 2-relaunch -expect=v1 -from=primary -write=v2
	echo "== 3 purge Library/Caches + launch"
	purge
	launch 3 purge 3-purge -expect=v2 -from=primary -write=v3
	echo "== 4 terminate + launch after the purge"
	launch 4 after-purge 4-after-purge -expect=v3 -from=primary -write=v4
	echo "== 5 corrupt primary"
	launch 5 corrupt-primary 5-corrupt-primary -corrupt=primary -expect=v4 -from=copy -write=v5
	echo "== 6 corrupt copy"
	launch 6 corrupt-copy 6-corrupt-copy -corrupt=copy -expect=v5 -from=primary -write=v6
	echo "== 7 simulator shutdown + boot"
	xcrun simctl shutdown "$udid"
	boot
	launch 7 reboot 7-reboot -expect=v6 -write=v7
	echo "== 8 install over, at once"
	xcrun simctl install "$udid" "$app"
	launch 8 install-over 8-install-over -expect=v7 -write=v8
	echo "== 9 uninstall + install"
	install_fresh
	launch 9 reinstall 9-reinstall -expect=defaults
	xcrun simctl uninstall "$udid" "$bundle_id" 2>/dev/null || true
	if [[ -z $booted_at_start && -z ${STORECHECK_KEEP:-} ]]; then
		xcrun simctl shutdown "$udid" 2>/dev/null || true
	fi
	summarize
}

# field log key name: the value of name= on the "slots" line of key.
field() {
	grep -E "^tvstore: slots .* key=$2 " "$1" | tr -d '\r' | head -1 | grep -oE "$3=[^ (]*" | head -1 | cut -d= -f2
}

# summarize prints one row per launch (result, each key's slot, the
# install id) into the summary file and on standard output, then the
# purge-restore row, and exits 0 only with 9 + 1 PASS.
summarize() {
	local summary=$log_dir/tvstore-summary.txt passes=0 n log result from id restore=PASS why=""
	{
		echo "device $device ($udid): $(state | sed 's/^ *//')"
		echo "tvOS simulator SDK $(xcrun --sdk appletvsimulator --show-sdk-version); $(date '+%Y-%m-%d %H:%M:%S')"
		printf '%-2s | %-17s | %-6s | %-34s | %s\n' n event result "from prefs/queue/device" "install id"
	} >"$summary"
	for n in 1 2 3 4 5 6 7 8 9; do
		log=$(ls "$log_dir"/tvstore-"$n"-*.log 2>/dev/null | head -1 || true)
		result="" from="" id=""
		if [[ -n $log ]]; then
			result=$(grep -oE "^tvstore: (PASS|FAIL) " "$log" | head -1 | awk '{print $2}' || true)
			from="$(field "$log" prefs from)/$(field "$log" progress-queue from)/$(field "$log" device from)"
			id=$(grep -E "^tvstore: install " "$log" | tr -d '\r' | head -1 | grep -oE "id=[^ ]*" | cut -d= -f2 || true)
		fi
		[[ $result == PASS ]] && passes=$((passes + 1))
		printf '%-2s | %-17s | %-6s | %-34s | %s\n' "$n" "$(basename "${log:-none}" .log | sed "s/^tvstore-$n-//")" \
			"${result:-NONE}" "$from" "$id" >>"$summary"
	done
	# The purge-restore row: step 3 found no copy of any key and read
	# NSUserDefaults; its save put the three files back (on disk after it);
	# step 4 read both slots without an error.
	local l3 l4
	l3=$(ls "$log_dir"/tvstore-3-*.log 2>/dev/null | head -1 || true)
	l4=$(ls "$log_dir"/tvstore-4-*.log 2>/dev/null | head -1 || true)
	for k in prefs progress-queue device; do
		[[ -n $l3 && $(field "$l3" "$k" copyErr) == notfound ]] || { restore=FAIL; why+=" 3:$k copy not purged"; }
		[[ -n $l3 && $(field "$l3" "$k" from) == primary ]] || { restore=FAIL; why+=" 3:$k not from primary"; }
		grep -qE "^$k.json = \{\"seq\"" "$l3" || { restore=FAIL; why+=" 3:$k.json not rewritten"; }
		[[ -n $l4 && $(field "$l4" "$k" copyErr) == nil && $(field "$l4" "$k" primaryErr) == nil ]] || { restore=FAIL; why+=" 4:$k slot error"; }
	done
	[[ $restore == PASS ]] && passes=$((passes + 1))
	printf '%-2s | %-17s | %-6s | %s\n' 10 purge-restore "$restore" "${why:-3: copy gone, read from NSUserDefaults, files rewritten; 4: both slots fine}" >>"$summary"
	local id2 id3
	id2=$(awk -F' [|] ' '$1 ~ /^2 *$/ {print $5}' "$summary" | tr -d ' ')
	id3=$(awk -F' [|] ' '$1 ~ /^3 *$/ {print $5}' "$summary" | tr -d ' ')
	if [[ -n $id2 && $id2 != "$id3" ]]; then
		echo "note: the install id changed with the purge ($id2 -> $id3): it lives in Library/Caches until G1 (R-P7-7)" >>"$summary"
	fi
	echo "PASS $passes/10 (9 launches + purge-restore)" >>"$summary"
	cat "$summary"
	[[ $passes == 10 ]]
}

# ---------------------------------------------------------------------
# test: tvstore's tests in the simulator.
# ---------------------------------------------------------------------
run_test() {
	echo "+ go test -c -tags tvos -o $build_dir/tvstore.test ./internal/store/tvstore"
	go_tvos test -c -tags tvos -o "$build_dir/tvstore.test" ./internal/store/tvstore
	local booted_here=""
	if [[ $(state) != *"(Booted)"* ]]; then
		booted_here=1
	fi
	boot
	local status=0
	xcrun simctl spawn "$udid" "$root/$build_dir/tvstore.test" -test.v -test.count=1 | tee "$log_dir/tvstore-unit.log" || status=$?
	if [[ -n $booted_here && -z ${STORECHECK_KEEP:-} ]]; then
		xcrun simctl shutdown "$udid" 2>/dev/null || true
	fi
	grep -q '^PASS$' "$log_dir/tvstore-unit.log" && [[ $status == 0 ]]
}

# ---------------------------------------------------------------------
# app: the app itself across relaunch, install over, purge, reinstall.
# ---------------------------------------------------------------------

# app_launch n event wait_re env...: one launch of the app with its console
# in the log until wait_re shows (60 s at most) and 2 s more, terminate;
# then its slots on disk.
app_launch() {
	local n=$1 event=$2 wait_re=$3 launcher i=0
	shift 3
	local log=$app_logs/app-$n-$event.log
	echo "+ $* xcrun simctl launch --console-pty --terminate-running-process $udid $app_bundle > $log"
	env "$@" xcrun simctl launch --console-pty --terminate-running-process "$udid" "$app_bundle" >"$log" 2>&1 &
	launcher=$!
	until grep -qE "$wait_re" "$log" 2>/dev/null; do
		((i++ < 240)) || { echo "no \"$wait_re\" after 60 s" >>"$log"; break; }
		sleep 0.25
	done
	sleep 2
	xcrun simctl terminate "$udid" "$app_bundle" 2>/dev/null || true
	wait "$launcher" 2>/dev/null || true
	bundle_id=$app_bundle dump "$(xcrun simctl get_app_container "$udid" "$app_bundle" data 2>/dev/null || true)" |
		sed -E 's/("(access|refresh)_token": *")[^"]*/\1<redacted>/g' >>"$log"
	grep -E "dancefitvn: first screen|dfv: prefs" "$log" | tr -d '\r' | sed 's/^/  /'  || true
}

# app_row n event want_screen want_prefs: PASS when the launch's log shows
# both.
app_row() {
	local n=$1 event=$2 screen=$3 prefs=$4 log result=FAIL got_s got_p
	log=$app_logs/app-$n-$event.log
	got_s=$(grep -oE "dancefitvn: first screen [a-z]+" "$log" | head -1 | awk '{print $4}' || true)
	got_p=$(grep -oE "dfv: prefs quality=[^ ]+ autonext=[a-z]+" "$log" | head -1 | cut -d' ' -f3- | tr -d '\r' || true)
	[[ $got_s == "$screen" && $got_p == "$prefs" ]] && result=PASS
	printf '%-2s | %-12s | %-6s | screen %-8s (want %-8s) | %s (want %s)\n' "$n" "$event" "$result" \
		"${got_s:-none}" "$screen" "${got_p:-none}" "$prefs" >>"$app_logs/app-summary.txt"
}

run_app() {
	app_bundle=com.jivecode.dancefitvn
	app_logs=local/p7/p75c/store
	local dist=dist/tvos-simulator-arm64/DanceFITVN.app set="quality=720p autonext=false" def="quality=auto autonext=true"
	mkdir -p "$app_logs"
	rm -f "$app_logs"/app-*
	if [[ -z ${DFV_NOBUILD:-} ]]; then
		scripts/dev-build.sh tvos >"$app_logs/app-build.log" 2>&1
	fi
	boot
	echo "== 1 uninstall + install, pair (TV04) and set the preferences"
	xcrun simctl uninstall "$udid" "$app_bundle" 2>/dev/null || true
	xcrun simctl install "$udid" "$dist"
	app_launch 1 paired "dfv: shot ready" SIMCTL_CHILD_DFV_STATE=TV04 SIMCTL_CHILD_DFV_PREFS=quality=720p,autonext=off
	echo "== 2 terminate + launch"
	app_launch 2 relaunch "dfv: prefs"
	echo "== 3 install over"
	xcrun simctl install "$udid" "$dist"
	app_launch 3 install-over "dfv: prefs"
	echo "== 4 purge Library/Caches + launch"
	local data
	data=$(xcrun simctl get_app_container "$udid" "$app_bundle" data)
	find "$data/Library/Caches" -mindepth 1 -maxdepth 1 -exec rm -rf {} +
	app_launch 4 purge "dfv: prefs"
	echo "== 5 uninstall + install"
	xcrun simctl uninstall "$udid" "$app_bundle"
	xcrun simctl install "$udid" "$dist"
	app_launch 5 reinstall "dfv: prefs"
	{
		echo "device $device ($udid); $(date '+%Y-%m-%d %H:%M:%S')"
		echo "the app: dev build of DanceFITVN over the mock in its process"
	} >"$app_logs/app-summary.txt"
	app_row 1 paired welcome "$set" # signed out at first, then paired: TV04
	app_row 2 relaunch home "$set"
	app_row 3 install-over home "$set"
	app_row 4 purge welcome "$set"
	app_row 5 reinstall welcome "$def"
	local passes
	passes=$(grep -c " PASS " "$app_logs/app-summary.txt" || true)
	echo "PASS $passes/5 (4: the purge takes the session: pair again, R-P7-7)" >>"$app_logs/app-summary.txt"
	cat "$app_logs/app-summary.txt"
	[[ $passes == 5 ]]
}

case $cmd in
build) build_app ;;
app) run_app ;;
run) run_check ;;
test) run_test ;;
vet) go_tvos vet -tags tvos ./internal/store/... ./internal/app ./e2e/tvstore ;;
*) echo "unknown command $cmd" >&2; exit 2 ;;
esac
