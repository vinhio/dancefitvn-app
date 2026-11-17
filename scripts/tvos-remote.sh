#!/usr/bin/env bash
# Adapted from jplayer-tv/scripts/tvos-app-remote.sh at 781bee5.
#
# tvos-remote.sh drives DanceFITVN on the Apple TV simulator with the Siri
# Remote (local/plans/P7.md P7.5a step 5, section 8.1): it builds the
# developer's app (scripts/dev-build.sh tvos), installs it, launches it with
# its console in a log, waits for it to be ready, then runs
# e2e/remote/RemoteUITests.swift, an XCTest UI-test bundle with no host app
# (its Xcode project is written here) that presses the remote's buttons
# through XCUIRemote and saves screenshots.
#
#   scripts/tvos-remote.sh "<steps>" [tag]
#
# steps  the grammar of RemoteUITests.swift: up down left right select menu
#        playpause home, <button>@<ms>, hold:<ms>, wait:<ms>, since:<ms>,
#        shot:<name>, front, state:<name>, mark:<text>
# tag    names the outputs (default "remote")
#
# Environment (all optional):
#   DFV_TVSIM      the simulator, by name or UDID (default "DanceFITVN
#                  Apple TV"); booted if it is not, and left booted
#   DFV_NOBUILD    1: install dist/tvos-simulator-arm64/DanceFITVN.app as it is
#   DFV_NOLAUNCH   1: neither install nor launch; drive the app running now
#                  (no console log then)
#   DFV_KEEPAPP    1: leave the app running at the end (default: terminated)
#   DFV_REMOTE_OUT the folder of the outputs (default local/p7/remote)
#   DFV_READY      an extended regexp of the console the steps wait for
#                  (default with DFV_STATE: "dfv: shot ready"; without: none,
#                  3 s); DFV_READY_S its time limit (default 90)
#   DFV_*          every other DFV_ variable is passed to the app
#                  (SIMCTL_CHILD_DFV_...): DFV_STATE, DFV_API, DFV_SETTLE,
#                  DFV_OFFLINE, DFV_ENGINE, DFV_FOCUSTRACE, DFV_FRAMELOG ...;
#                  and every GDESK_ one (GDESK_FRAMESTATS=1)
#
# Outputs in DFV_REMOTE_OUT: <tag>.log (the app's console), <tag>.xcodebuild.log
# (the markers "dfvremote: <time> step|shot|state ..."), <tag>.system.log
# (the system's log of the process: lifecycle, audio), <tag>-<shot>.png.
# The app plays the mock's media only; no URL, token or account is passed.
set -euo pipefail

root=$(cd "$(dirname "$0")/.." && pwd)
cd "$root"

steps=${1:?usage: scripts/tvos-remote.sh \"<steps>\" [tag]}
tag=${2:-remote}
device=${DFV_TVSIM:-DanceFITVN Apple TV}
out=${DFV_REMOTE_OUT:-local/p7/remote}
build_dir=local/p7/remote-build
bundle_id=com.jivecode.dancefitvn
exe=DanceFITVN
app=dist/tvos-simulator-arm64/DanceFITVN.app
mkdir -p "$out" "$build_dir"
log=$out/$tag.log
xlog=$out/$tag.xcodebuild.log
slog=$out/$tag.system.log

# The script's own variables, not passed to the app.
own=" DFV_TVSIM DFV_TV_UDID DFV_TEAM DFV_IPHONE_UDID DFV_NOBUILD DFV_NOLAUNCH DFV_KEEPAPP DFV_REMOTE_OUT DFV_READY DFV_READY_S DFV_STEPS DFV_SHOT "

# ---------------------------------------------------------------------
# Simulator: by UDID (another simulator may be booted too).
# ---------------------------------------------------------------------
udid=$(xcrun simctl list devices | grep -F "$device (" | head -1 | grep -oE '[0-9A-F]{8}-[0-9A-F-]{27}' || true)
udid=${udid:-$device}
xcrun simctl list devices | grep -q "$udid" || { echo "no simulator \"$device\"" >&2; exit 1; }
if ! xcrun simctl list devices | grep "$udid" | grep -q "(Booted)"; then
	echo "+ xcrun simctl boot $udid"
	xcrun simctl boot "$udid"
fi
xcrun simctl bootstatus "$udid" -b >/dev/null

# ---------------------------------------------------------------------
# The UI-test bundle, built for testing once (test-without-building
# later, so that the steps start right when the app is ready).
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
			CODE_SIGN_IDENTITY = "-";
			CODE_SIGN_STYLE = Manual;
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
	echo "+ xcodebuild build-for-testing -project $proj -scheme Remote -destination id=$udid"
	xcodebuild build-for-testing -project "$proj" -scheme Remote -destination "id=$udid" \
		-derivedDataPath "$dir/dd" CODE_SIGNING_ALLOWED=YES >"$build_dir/xcui-build.log" 2>&1 ||
		{ echo "the UI-test bundle did not build: see $build_dir/xcui-build.log" >&2; exit 1; }
fi

# ---------------------------------------------------------------------
# The app: built, installed, launched with its console in the log and the
# DFV_ variables given; then the wait for it to be ready.
# ---------------------------------------------------------------------
since=$(date "+%Y-%m-%d %H:%M:%S")
launcher=""
if [[ -z ${DFV_NOLAUNCH:-} ]]; then
	if [[ -z ${DFV_NOBUILD:-} ]]; then
		scripts/dev-build.sh tvos >"$build_dir/dev-build.log" 2>&1 ||
			{ echo "the app did not build: see $build_dir/dev-build.log" >&2; exit 1; }
	fi
	xcrun simctl install "$udid" "$app"
	pass=()
	while IFS='=' read -r k v; do
		[[ ($k == DFV_* || $k == GDESK_*) && $own != *" $k "* ]] && pass+=("SIMCTL_CHILD_$k=$v")
	done < <(env)
	echo "+ ${pass[*]} xcrun simctl launch --console-pty --terminate-running-process $udid $bundle_id > $log"
	env "${pass[@]}" xcrun simctl launch --console-pty --terminate-running-process "$udid" "$bundle_id" >"$log" 2>&1 &
	launcher=$!
	ready=${DFV_READY:-}
	[[ -z $ready && -n ${DFV_STATE:-} ]] && ready="dfv: shot ready"
	if [[ -n $ready ]]; then
		n=0 limit=$((${DFV_READY_S:-90} * 4))
		until grep -qE "$ready" "$log" 2>/dev/null; do
			((n++ < limit)) || { echo "the app was not ready (\"$ready\") after ${DFV_READY_S:-90} s: see $log" >&2; break; }
			sleep 0.25
		done
	else
		sleep 3
	fi
fi

# ---------------------------------------------------------------------
# The steps.
# ---------------------------------------------------------------------
status=0
xctestrun=$(ls "$dir"/dd/Build/Products/*.xctestrun | head -1)
echo "+ xcodebuild test-without-building -xctestrun $xctestrun -destination id=$udid > $xlog"
TEST_RUNNER_DFV_SHOT="$root/$out/$tag" TEST_RUNNER_DFV_STEPS="$steps" \
	xcodebuild test-without-building -xctestrun "$xctestrun" -destination "id=$udid" \
	-parallel-testing-enabled NO >"$xlog" 2>&1 || { status=$?; echo "xcodebuild test failed: see $xlog"; }
grep -E "dfvremote:|error:|TEST (EXECUTE )?(SUCCEEDED|FAILED)" "$xlog" | sed -E 's/^.*(dfvremote:)/\1/' || true

# ---------------------------------------------------------------------
# End: the app stops unless DFV_KEEPAPP=1; its system log and any crash
# report are kept.
# ---------------------------------------------------------------------
if [[ -n $launcher ]]; then
	if [[ -z ${DFV_KEEPAPP:-} ]]; then
		xcrun simctl terminate "$udid" "$bundle_id" 2>/dev/null || true
		wait "$launcher" 2>/dev/null || true
	fi
fi
xcrun simctl spawn "$udid" log show --start "$since" --style compact \
	--predicate "process == \"$exe\" OR eventMessage CONTAINS \"$bundle_id\"" >"$slog" 2>&1 || true
find ~/Library/Logs/DiagnosticReports -name "$exe-*.ips" -newermt "$since" -exec cp {} "$out/" \; -print 2>/dev/null || true
exit "$status"
