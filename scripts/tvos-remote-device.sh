#!/usr/bin/env bash
# Adapted from scripts/tvos-remote.sh and jplayer-tv/scripts/tvos-device.sh
# at 781bee5 (build, sign, install, launch with "--console", the UI-test
# runner on the device).
#
# tvos-remote-device.sh drives DanceFITVN on a paired Apple TV with the
# Siri Remote, as scripts/tvos-remote.sh does on the simulator: it builds
# the developer's app for the device (scripts/dev-build.sh tvos -device),
# signs it (scripts/tvos-device-sign.sh), installs and launches it with
# devicectl (its console in a log, the DFV_ and GDESK_ variables as its
# environment), waits for it to be ready, then runs
# e2e/remote/RemoteUITests.swift on the Apple TV (XCUIRemote). The shots
# are the result bundle's attachments, exported to PNGs.
#
#   scripts/tvos-remote-device.sh "<steps>" [tag]
#
# steps  the grammar of RemoteUITests.swift (up down left right select
#        menu playpause home, <button>@<ms>, hold:<ms>, wait:<ms>,
#        since:<ms>, shot:<name>, front, state:<name>, mark:<text>)
# tag    names the outputs (default "remote")
#
# Environment (required):
#   DFV_TV_UDID    the Apple TV's UDID (xcrun devicectl list devices)
#   DFV_TEAM       the team ID of the UI-test bundle's automatic signing
#                  (xcodebuild -allowProvisioningUpdates: it may create or
#                  renew that team's development profile in the developer
#                  account, and register the Apple TV there)
# Environment (optional):
#   DFV_NOBUILD    1: install dist/tvos-arm64/DanceFITVN.app as it is
#                  (signed already)
#   DFV_NOLAUNCH   1: neither install nor launch; drive the app running now
#   DFV_KEEPAPP    1: leave the app running at the end
#   DFV_APP        the bundle the steps drive (default DanceFITVN's); with
#                  DFV_NOLAUNCH=1, e.g. the engine test app of
#                  scripts/tvos-avf-test.sh device-e2e ("home", "front")
#   DFV_REMOTE_OUT the outputs' folder (default local/p8/appletv/remote)
#   DFV_READY      an extended regexp of the console to wait for (default
#                  with DFV_STATE: "dfv: shot ready"; without: none, 4 s);
#                  DFV_READY_S its time limit (default 90)
#   DFV_*, GDESK_* every other one is the app's environment
#
# Outputs in DFV_REMOTE_OUT: <tag>.log (the app's console),
# <tag>.xcodebuild.log (the markers "dfvremote: <device time> ..."),
# <tag>.xcresult, <tag>-<shot>.png, <tag>.crashes/ (crash logs of the
# app since the start, if any). The app plays the mock's media only; no URL,
# token or account is passed.
set -euo pipefail

root=$(cd "$(dirname "$0")/.." && pwd)
cd "$root"

steps=${1:?usage: scripts/tvos-remote-device.sh \"<steps>\" [tag]}
tag=${2:-remote}
udid=${DFV_TV_UDID:?set DFV_TV_UDID to the UDID of the Apple TV, see xcrun devicectl list devices}
team=${DFV_TEAM:?set DFV_TEAM to the Apple developer team ID that signs the UI-test bundle}
out=${DFV_REMOTE_OUT:-local/p8/appletv/remote}
build_dir=local/p8/appletv/remote-build
bundle_id=com.jivecode.dancefitvn
exe=DanceFITVN
app=dist/tvos-arm64/DanceFITVN.app
mkdir -p "$out" "$build_dir"
log=$out/$tag.log
xlog=$out/$tag.xcodebuild.log
rm -rf "$out/$tag.xcresult"

own=" DFV_APP DFV_TV_UDID DFV_TEAM DFV_IPHONE_UDID DFV_NOBUILD DFV_NOLAUNCH DFV_KEEPAPP DFV_REMOTE_OUT DFV_READY DFV_READY_S DFV_STEPS DFV_SHOT DFV_PROFILE DFV_SIGN_IDENTITY "

# ---------------------------------------------------------------------
# The Apple TV: paired, connected, Developer Mode on.
# ---------------------------------------------------------------------
info=$(xcrun devicectl device info details --device "$udid" 2>&1) || { echo "the Apple TV $udid is not reachable:" >&2; echo "$info" >&2; exit 1; }
grep -q "Pairing State: paired" <<<"$info" || { echo "the Apple TV is not paired (Xcode › Devices and Simulators)" >&2; exit 1; }
grep -q "Developer Mode Status: Enabled" <<<"$info" || { echo "Developer Mode is off on the Apple TV: Settings › Privacy & Security › Developer Mode" >&2; exit 1; }

# ---------------------------------------------------------------------
# The UI-test bundle, signed automatically for the device (Xcode makes
# the team's tvOS profile on the first build), built once.
# ---------------------------------------------------------------------
dir=$build_dir/xcui
proj=$dir/Remote.xcodeproj
mkdir -p "$proj/xcshareddata/xcschemes"
if ! cmp -s e2e/remote/RemoteUITests.swift "$dir/RemoteUITests.swift"; then
	cp e2e/remote/RemoteUITests.swift "$dir/"
	rm -rf "$dir/dd"
fi
cat >"$proj/project.pbxproj" <<'EOF'
// !$*UTF8*$!
{
	archiveVersion = 1;
	classes = {
	};
	objectVersion = 56;
	objects = {
		BB0000000000000000000001 = {isa = PBXBuildFile; fileRef = BB0000000000000000000002; };
		BB0000000000000000000002 = {isa = PBXFileReference; lastKnownFileType = sourcecode.swift; path = RemoteUITests.swift; sourceTree = "<group>"; };
		BB0000000000000000000003 = {isa = PBXFileReference; explicitFileType = wrapper.cfbundle; includeInIndex = 0; path = RemoteUITests.xctest; sourceTree = BUILT_PRODUCTS_DIR; };
		BB0000000000000000000004 = {isa = PBXGroup; children = (BB0000000000000000000002, BB0000000000000000000005, ); sourceTree = "<group>"; };
		BB0000000000000000000005 = {isa = PBXGroup; children = (BB0000000000000000000003, ); name = Products; sourceTree = "<group>"; };
		BB0000000000000000000006 = {isa = PBXNativeTarget; buildConfigurationList = BB0000000000000000000009; buildPhases = (BB0000000000000000000007, ); buildRules = ( ); dependencies = ( ); name = RemoteUITests; productName = RemoteUITests; productReference = BB0000000000000000000003; productType = "com.apple.product-type.bundle.ui-testing"; };
		BB0000000000000000000007 = {isa = PBXSourcesBuildPhase; buildActionMask = 2147483647; files = (BB0000000000000000000001, ); runOnlyForDeploymentPostprocessing = 0; };
		BB0000000000000000000008 = {isa = PBXProject; buildConfigurationList = BB000000000000000000000A; compatibilityVersion = "Xcode 14.0"; developmentRegion = en; hasScannedForEncodings = 0; knownRegions = (en, Base, ); mainGroup = BB0000000000000000000004; productRefGroup = BB0000000000000000000005; projectDirPath = ""; projectRoot = ""; targets = (BB0000000000000000000006, ); };
		BB0000000000000000000009 = {isa = XCConfigurationList; buildConfigurations = (BB000000000000000000000B, ); defaultConfigurationIsVisible = 0; defaultConfigurationName = Debug; };
		BB000000000000000000000A = {isa = XCConfigurationList; buildConfigurations = (BB000000000000000000000C, ); defaultConfigurationIsVisible = 0; defaultConfigurationName = Debug; };
		BB000000000000000000000B = {isa = XCBuildConfiguration; buildSettings = {
			CODE_SIGN_STYLE = Automatic;
			GENERATE_INFOPLIST_FILE = YES;
			PRODUCT_BUNDLE_IDENTIFIER = com.jivecode.dancefitvn.remote.uitests;
			PRODUCT_NAME = "$(TARGET_NAME)";
			SWIFT_VERSION = 5.0;
			TARGETED_DEVICE_FAMILY = 3;
		}; name = Debug; };
		BB000000000000000000000C = {isa = XCBuildConfiguration; buildSettings = {
			SDKROOT = appletvos;
			SUPPORTED_PLATFORMS = "appletvos appletvsimulator";
			TVOS_DEPLOYMENT_TARGET = 17.0;
			ONLY_ACTIVE_ARCH = YES;
			ENABLE_TESTABILITY = YES;
			SWIFT_OPTIMIZATION_LEVEL = "-Onone";
		}; name = Debug; };
	};
	rootObject = BB0000000000000000000008;
}
EOF
cat >"$proj/xcshareddata/xcschemes/Remote.xcscheme" <<'EOF'
<?xml version="1.0" encoding="UTF-8"?>
<Scheme LastUpgradeVersion = "2600" version = "1.7">
   <BuildAction parallelizeBuildables = "YES" buildImplicitDependencies = "YES">
      <BuildActionEntries>
         <BuildActionEntry buildForTesting = "YES" buildForRunning = "NO" buildForProfiling = "NO" buildForArchiving = "NO" buildForAnalyzing = "NO">
            <BuildableReference BuildableIdentifier = "primary" BlueprintIdentifier = "BB0000000000000000000006" BuildableName = "RemoteUITests.xctest" BlueprintName = "RemoteUITests" ReferencedContainer = "container:Remote.xcodeproj">
            </BuildableReference>
         </BuildActionEntry>
      </BuildActionEntries>
   </BuildAction>
   <TestAction buildConfiguration = "Debug" selectedDebuggerIdentifier = "" selectedLauncherIdentifier = "Xcode.IDEFoundation.Launcher.PosixSpawn" shouldUseLaunchSchemeArgsEnv = "YES">
      <Testables>
         <TestableReference skipped = "NO" parallelizable = "NO">
            <BuildableReference BuildableIdentifier = "primary" BlueprintIdentifier = "BB0000000000000000000006" BuildableName = "RemoteUITests.xctest" BlueprintName = "RemoteUITests" ReferencedContainer = "container:Remote.xcodeproj">
            </BuildableReference>
         </TestableReference>
      </Testables>
   </TestAction>
</Scheme>
EOF
if ! ls "$dir"/dd/Build/Products/*.xctestrun >/dev/null 2>&1; then
	echo "+ xcodebuild build-for-testing -project $proj -scheme Remote -destination id=$udid -allowProvisioningUpdates DEVELOPMENT_TEAM=$team"
	xcodebuild build-for-testing -project "$proj" -scheme Remote -destination "id=$udid" \
		-derivedDataPath "$dir/dd" -allowProvisioningUpdates DEVELOPMENT_TEAM="$team" \
		>"$build_dir/xcui-build.log" 2>&1 ||
		{ echo "the UI-test bundle did not build: see $build_dir/xcui-build.log" >&2; exit 1; }
fi

# pid_of prints the app's process on the Apple TV, or nothing.
pid_of() {
	xcrun devicectl device info processes --device "$udid" --json-output - 2>/dev/null |
		python3 -I -c 'import json,sys
d = json.load(sys.stdin)
for p in d.get("result", {}).get("runningProcesses", []):
    if str(p.get("executable", "")).endswith("/DanceFITVN.app/DanceFITVN"):
        print(p.get("processIdentifier")); break' || true
}

# ---------------------------------------------------------------------
# The app: built, signed, installed, launched with its console in the log
# and the DFV_/GDESK_ variables as its environment; then the wait.
# ---------------------------------------------------------------------
start=$(date +%s)
launcher=""
if [[ -z ${DFV_NOLAUNCH:-} ]]; then
	if [[ -z ${DFV_NOBUILD:-} ]]; then
		scripts/dev-build.sh tvos -device >"$build_dir/dev-build.log" 2>&1 ||
			{ echo "the app did not build: see $build_dir/dev-build.log" >&2; exit 1; }
		scripts/tvos-device-sign.sh "$app"
	fi
	xcrun devicectl device install app --device "$udid" "$app" >"$build_dir/install.log" 2>&1 ||
		{ echo "install failed: see $build_dir/install.log" >&2; cat "$build_dir/install.log" >&2; exit 1; }
	envjson=$(env | python3 -I -c 'import json,sys
own = set(sys.argv[1].split())
e = {}
for line in sys.stdin:
    k, _, v = line.rstrip("\n").partition("=")
    if (k.startswith("DFV_") or k.startswith("GDESK_")) and k not in own:
        e[k] = v
print(json.dumps(e))' "$own")
	# devicectl refuses an empty object.
	envarg=()
	[[ $envjson != "{}" ]] && envarg=(--environment-variables "$envjson")
	echo "+ xcrun devicectl device process launch --terminate-existing --console ${envarg[*]:-} $bundle_id > $log"
	xcrun devicectl device process launch --device "$udid" --terminate-existing --console \
		"${envarg[@]}" "$bundle_id" >"$log" 2>&1 &
	launcher=$!
	ready=${DFV_READY:-}
	[[ -z $ready && -n ${DFV_STATE:-} ]] && ready="dfv: shot ready"
	if [[ -n $ready ]]; then
		n=0 limit=$((${DFV_READY_S:-90} * 4))
		until grep -qE "$ready" "$log" 2>/dev/null; do
			((n++ < limit)) || { echo "the app was not ready (\"$ready\") after ${DFV_READY_S:-90} s: see $log" >&2; break; }
			kill -0 "$launcher" 2>/dev/null || { echo "the launcher ended: see $log" >&2; break; }
			sleep 0.25
		done
	else
		sleep 4
	fi
fi

# ---------------------------------------------------------------------
# The steps, on the Apple TV.
# ---------------------------------------------------------------------
status=0
xctestrun=$(ls "$dir"/dd/Build/Products/*.xctestrun | head -1)
echo "+ xcodebuild test-without-building -xctestrun $xctestrun -destination id=$udid > $xlog"
TEST_RUNNER_DFV_SHOT="attach:$tag" TEST_RUNNER_DFV_STEPS="$steps" TEST_RUNNER_DFV_APP="${DFV_APP:-}" \
	xcodebuild test-without-building -xctestrun "$xctestrun" -destination "id=$udid" \
	-resultBundlePath "$out/$tag.xcresult" -parallel-testing-enabled NO >"$xlog" 2>&1 ||
	{ status=$?; echo "xcodebuild test failed: see $xlog"; }
grep -E "dfvremote:|error:|TEST (EXECUTE )?(SUCCEEDED|FAILED)" "$xlog" | sed -E 's/^.*(dfvremote:)/\1/' || true

# The shots: the attachments named "<tag>-<shot>".
if [[ -d $out/$tag.xcresult ]]; then
	att=$build_dir/att-$tag
	rm -rf "$att"
	mkdir -p "$att"
	if xcrun xcresulttool export attachments --path "$out/$tag.xcresult" --output-path "$att" >/dev/null 2>&1; then
		python3 -I - "$att" "$out" <<'PY'
import json, os, shutil, sys
att, out = sys.argv[1:3]
for test in json.load(open(os.path.join(att, "manifest.json"))):
    for a in test.get("attachments", []):
        name = a.get("suggestedHumanReadableName", "")
        base = name.split("_0_")[0] if "_0_" in name else os.path.splitext(name)[0]
        shutil.copy(os.path.join(att, a["exportedFileName"]), os.path.join(out, base + ".png"))
        print("shot", os.path.join(out, base + ".png"))
PY
	fi
fi

# ---------------------------------------------------------------------
# End: the app stops unless DFV_KEEPAPP=1; crash logs since the start are
# kept.
# ---------------------------------------------------------------------
if [[ -n $launcher && -z ${DFV_KEEPAPP:-} ]]; then
	pid=$(pid_of)
	[[ -n $pid ]] && xcrun devicectl device process terminate --device "$udid" --pid "$pid" >/dev/null 2>&1 || true
	sleep 1
	kill "$launcher" 2>/dev/null || true
	wait "$launcher" 2>/dev/null || true
fi
crashes=$out/$tag.crashes
if xcrun devicectl device info files --device "$udid" --domain-type systemCrashLogs --json-output "$build_dir/crashlist.json" >/dev/null 2>&1; then
	python3 -I - "$build_dir/crashlist.json" "$exe" "$start" <<'PY' | while read -r f; do
import json, sys, datetime
d = json.load(open(sys.argv[1]))
exe, start = sys.argv[2], float(sys.argv[3])
for f in d.get("result", {}).get("files", []):
    p = f.get("relativePath") or f.get("name") or ""
    if not p.split("/")[-1].startswith(exe):
        continue
    t = (f.get("metadata") or {}).get("lastModDate") or ""
    try:
        ts = datetime.datetime.fromisoformat(t.replace("Z", "+00:00")).timestamp()
    except ValueError:
        ts = start
    if ts >= start:
        print(p)
PY
		mkdir -p "$crashes"
		xcrun devicectl device copy from --device "$udid" --domain-type systemCrashLogs \
			--source "$f" --destination "$crashes/$(basename "$f")" >/dev/null 2>&1 && echo "crash log: $crashes/$(basename "$f")"
	done
fi
exit "$status"
