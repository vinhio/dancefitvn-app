#!/usr/bin/env bash
# Adapted from jplayer-mobile/scripts/release/check-ios.sh at 49de3c2 and
# jplayer-tv/scripts/tvos-release.sh at 781bee5 (verify: prohibited
# imports, /Users/ strings, linked libraries, go version -m's tags).
#
# Checks an iOS or tvOS release build of DanceFITVN before it goes to App
# Store Connect: what the upload's validation and App Review are known to
# refuse (ITMS-90713, 90704, 90717, 90737, 90534, 90725, 91053).
#
#   scripts/release/check-ios.sh <DanceFITVN.app | DanceFITVN.ipa>
#   scripts/release/check-ios.sh -h | --help
#
# The system is the bundle's (CFBundleSupportedPlatforms: iPhoneOS or
# AppleTVOS); a simulator build fails.
#
# Info.plist: lints, the bundle id, version and build of gdesk.json,
# Vietnamese as the development region, minimum 17.0,
# ITSAppUsesNonExemptEncryption false, background audio, the DT* keys of
# Xcode, App Transport Security holding NSAllowsLocalNetworking alone
# (gdesk.json's localNetworking: loopback http for a developer's server;
# no arbitrary loads). iOS: the icon by name (iPhone and iPad), the four
# iPad orientations, NSFaceIDUsageDescription (scripts/ios-faceid-plist.sh
# until gDesk's G3a). tvOS: UIDeviceFamily [3], the App Icon stack and
# the Top Shelf images (Assets.car). Assets.car (assetutil): the icon, the
# launch color. PrivacyInfo.xcprivacy: at the bundle's top, the same as
# gdesk.json's ios/tvos.privacyManifest, its collected data types valid
# and none for tracking, and declaring every required reason API the
# program imports (dyld_info) or names; a category declared and unused
# is a warning. The program: arm64 only, built for the device with minos
# 17.0 and an SDK of 26 or later, no /Users/ path, only system libraries,
# go version -m's tags EXPECT_TAGS (default "live", "tvos,live" on tvOS;
# set and empty for none),
# never dancefitvndev; tvOS: none of the functions its SDK prohibits
# (fork, system, posix_spawn, popen...; execve a warning, Go's syscall).
# The Objective-C selectors of the Go code built for the system, looked up
# in the SDK's headers (warnings). The signature when there is one (an
# Apple Distribution authority, embedded.mobileprovision, no
# get-task-allow); unsigned is a warning. DSYM=<App.app.dSYM> checks its
# UUID against the program's. Prints the sizes.
#
# Lines read "ok", "WARN" or "FAIL"; exit status 1 when any FAIL, 2 on
# usage. GDESK_DIR is the gDesk worktree (default ../gdesk-dancefitvn-app).
set -uo pipefail

usage() { awk 'NR > 1 && /^#/ { sub(/^# ?/, ""); print; next } NR > 1 { exit }' "$0"; }

case "${1:-}" in
-h | --help) usage; exit 0 ;;
"") usage >&2; exit 2 ;;
esac
target=$1
[ -e "$target" ] || { echo "check-ios: $target does not exist" >&2; exit 2; }

here="$(cd "$(dirname "$0")" && pwd)"
root="$(cd "$here/../.." && pwd)"
scan="$here/iosscan.py"
tmp=$(mktemp -d)
trap 'rm -rf "$tmp"' EXIT

fails=0
warns=0
ok() { echo "ok    $*"; }
warn() { echo "WARN  $*"; warns=$((warns + 1)); }
fail() { echo "FAIL  $*"; fails=$((fails + 1)); }
# tally reads iosscan.py's lines, counting them as ok, warn and fail do.
tally() {
	while IFS= read -r l; do
		echo "$l"
		case $l in WARN*) warns=$((warns + 1)) ;; FAIL*) fails=$((fails + 1)) ;; esac
	done
}

# ---- the bundle --------------------------------------------------------
case $target in
*.ipa)
	ipa=$target
	unzip -q "$ipa" -d "$tmp/ipa" || { echo "check-ios: $ipa is not a zip" >&2; exit 1; }
	tops=$(cd "$tmp/ipa" && ls -A)
	[ "$tops" = Payload ] || fail ".ipa holds $(echo $tops) at its top, not Payload/ alone"
	apps=("$tmp"/ipa/Payload/*.app)
	[ ${#apps[@]} -eq 1 ] && [ -d "${apps[0]}" ] || { fail ".ipa has no single Payload/*.app"; echo "check-ios: $fails FAIL"; exit 1; }
	app=${apps[0]}
	;;
*.app) ipa="" app=${target%/} ;;
*) echo "check-ios: $target is neither an .app nor an .ipa" >&2; exit 2 ;;
esac

plist="$app/Info.plist"
get() { plutil -extract "$1" raw -o - "$plist" 2>/dev/null; }
json() { plutil -extract "$1" json -o - "$plist" 2>/dev/null; }
has() { plutil -extract "$1" xml1 -o /dev/null "$plist" >/dev/null 2>&1; }
want() { # want <key> <value>
	local v
	v=$(get "$1") || { fail "Info.plist has no $1 (want $2)"; return; }
	[ "$v" = "$2" ] && ok "$1 = $v" || fail "$1 = $v, want $2"
}
need() { has "$1" && ok "$1 = $(get "$1" || json "$1" | head -c 80)" || fail "Info.plist has no $1${2:+ ($2)}"; }
cfg() { plutil -extract "$1" raw -o - "$root/gdesk.json" 2>/dev/null; }

case $(get CFBundleSupportedPlatforms.0) in
iPhoneOS) sys=ios plat=IOS dtplat=iphoneos sdkname=iphoneos goflags="" ;;
AppleTVOS) sys=tvos plat=TVOS dtplat=appletvos sdkname=appletvos goflags="-tags tvos" ;;
*)
	echo "check-ios: $target is for $(get CFBundleSupportedPlatforms.0 || echo 'no platform'), not iPhoneOS or AppleTVOS (a simulator build?)" >&2
	exit 1
	;;
esac
echo "check-ios: $target ($sys)"

# ---- Info.plist --------------------------------------------------------
plutil -lint -s "$plist" && ok "Info.plist lints" || fail "Info.plist does not lint"
want CFBundleIdentifier "$(cfg id)"
want CFBundleShortVersionString "$(cfg version)"
want CFBundleVersion "$(cfg build || cfg version)"
want CFBundleDevelopmentRegion vi
want MinimumOSVersion 17.0
want ITSAppUsesNonExemptEncryption false
want DTPlatformName "$dtplat"
for k in DTPlatformVersion DTSDKName DTSDKBuild DTXcode DTXcodeBuild; do need "$k" "ITMS-90534/90737"; done
for k in DTPlatformBuild DTCompiler; do has "$k" && ok "$k = $(get "$k")" || warn "Info.plist has no $k"; done
modes=$(json UIBackgroundModes | tr -d ' \n')
[ "$modes" = '["audio"]' ] && ok "UIBackgroundModes = $modes" || fail "UIBackgroundModes = ${modes:-none}, want [\"audio\"]"
caps=$(json UIRequiredDeviceCapabilities | tr -d ' \n')
[ "$caps" = '["arm64"]' ] && ok "UIRequiredDeviceCapabilities = $caps" || fail "UIRequiredDeviceCapabilities = ${caps:-none}, want [\"arm64\"]"
ats=$(json NSAppTransportSecurity | tr -d ' \n')
case $ats in
'{"NSAllowsLocalNetworking":true}') ok "NSAppTransportSecurity: NSAllowsLocalNetworking alone (loopback http, Q-P8-23)" ;;
"") ok "no NSAppTransportSecurity" ;;
*) fail "NSAppTransportSecurity = $ats (want NSAllowsLocalNetworking alone: the API is https)" ;;
esac
if [ $sys = ios ]; then
	want CFBundleSupportedPlatforms.0 iPhoneOS
	icon=$(get CFBundleIcons.CFBundlePrimaryIcon.CFBundleIconName) && ok "CFBundleIcons CFBundleIconName = $icon" ||
		fail "Info.plist has no CFBundleIcons.CFBundlePrimaryIcon.CFBundleIconName (ITMS-90713)"
	ipadicon=$(get 'CFBundleIcons~ipad.CFBundlePrimaryIcon.CFBundleIconName') && ok "CFBundleIcons~ipad CFBundleIconName = $ipadicon" ||
		fail "Info.plist has no CFBundleIcons~ipad.CFBundlePrimaryIcon.CFBundleIconName"
	fam=$(json UIDeviceFamily | tr -d ' \n')
	[ "$fam" = '[1,2]' ] && ok "UIDeviceFamily = $fam" || fail "UIDeviceFamily = ${fam:-none}, want [1,2]"
	n=0
	for o in Portrait PortraitUpsideDown LandscapeLeft LandscapeRight; do
		json 'UISupportedInterfaceOrientations~ipad' | grep -q "\"UIInterfaceOrientation$o\"" && n=$((n + 1))
	done
	[ $n -eq 4 ] && ok "UISupportedInterfaceOrientations~ipad: the 4 orientations" || fail "UISupportedInterfaceOrientations~ipad has $n of the 4 orientations (ITMS-90474)"
	face=$(get NSFaceIDUsageDescription) && ok "NSFaceIDUsageDescription = $face" ||
		fail "Info.plist has no NSFaceIDUsageDescription (Face ID crashes without it: scripts/ios-faceid-plist.sh, gDesk G3a)"
else
	want CFBundleSupportedPlatforms.0 AppleTVOS
	fam=$(json UIDeviceFamily | tr -d ' \n')
	[ "$fam" = '[3]' ] && ok "UIDeviceFamily = $fam" || fail "UIDeviceFamily = ${fam:-none}, want [3]"
	icon=$(get CFBundleIcons.CFBundlePrimaryIcon) && ok "CFBundleIcons CFBundlePrimaryIcon = $icon" ||
		fail "Info.plist has no CFBundleIcons.CFBundlePrimaryIcon (tvos.brandAssets)"
	shelf=$(get TVTopShelfImage.TVTopShelfPrimaryImage) || fail "Info.plist has no TVTopShelfImage.TVTopShelfPrimaryImage"
	wide=$(get TVTopShelfImage.TVTopShelfPrimaryImageWide) || fail "Info.plist has no TVTopShelfImage.TVTopShelfPrimaryImageWide"
	has NSFaceIDUsageDescription && warn "NSFaceIDUsageDescription on tvOS (no Face ID there)"
fi
color=$(get UILaunchScreen.UIColorName) || { color=""; fail "Info.plist has no UILaunchScreen.UIColorName"; }

# ---- Assets.car --------------------------------------------------------
if [ -f "$app/Assets.car" ]; then
	if xcrun assetutil --info "$app/Assets.car" >"$tmp/assets.json" 2>/dev/null; then
		if [ $sys = ios ]; then
			tally < <(python3 -I "$scan" assets "$tmp/assets.json" "${icon:-AppIcon}" "$color")
		else
			tally < <(python3 -I "$scan" tvassets "$tmp/assets.json" "${icon:-}" "$color" "${shelf:-}" "${wide:-}")
		fi
	else
		fail "assetutil cannot read Assets.car"
	fi
else
	fail "the app has no Assets.car (icon by name, ITMS-90713)"
fi

# ---- PrivacyInfo.xcprivacy and the program -----------------------------
exe="$app/$(get CFBundleExecutable)"
: >"$tmp/imports.txt"
if [ -f "$exe" ]; then
	archs=$(lipo -archs "$exe" 2>/dev/null)
	[ "$archs" = arm64 ] && ok "program arm64" || fail "program architectures '$archs', want arm64"
	build=$(xcrun vtool -show-build "$exe" 2>/dev/null)
	p=$(awk '$1 == "platform" { print $2; exit }' <<<"$build")
	minos=$(awk '$1 == "minos" { print $2; exit }' <<<"$build")
	sdk=$(awk '$1 == "sdk" { print $2; exit }' <<<"$build")
	[ "$p" = "$plat" ] && ok "LC_BUILD_VERSION platform $p" || fail "LC_BUILD_VERSION platform '$p', want $plat (a simulator build?)"
	[ "$minos" = 17.0 ] && ok "LC_BUILD_VERSION minos 17.0" || fail "LC_BUILD_VERSION minos '$minos', want 17.0"
	[ "${sdk%%.*}" -ge 26 ] 2>/dev/null && ok "LC_BUILD_VERSION sdk $sdk" || fail "LC_BUILD_VERSION sdk '$sdk', want 26 or later (ITMS-90725)"
	xcrun dyld_info -imports "$exe" 2>/dev/null | awk '$1 ~ /^0x/ { print $2 }' >"$tmp/imports.txt"
	if [ $sys = tvos ]; then
		n=$(grep -E '^_(fork|vfork|ptrace|system|posix_spawn|posix_spawnp|popen)$' "$tmp/imports.txt" | tr '\n' ' ')
		[ -z "$n" ] && ok "no fork, vfork, ptrace, system, posix_spawn, popen (prohibited on tvOS)" || fail "imports what tvOS prohibits: $n"
		grep -qx _execve "$tmp/imports.txt" && warn "execve imported (Go's syscall; __TVOS_PROHIBITED in the SDK; jPlayer TV shipped with it)"
	fi
	n=$(LC_ALL=C grep -acE '/Users/|/private/var/folders/' "$exe" || true)
	[ "$n" = 0 ] && ok "no /Users/ or /private/var/folders/ path in the program" || fail "$n lines of the program hold /Users/ or /private/var/folders/ (absolute go.mod replaces?)"
	n=$(otool -L "$exe" | tail -n +2 | awk '{print $1}' | grep -vE '^/(System/Library/Frameworks|usr/lib)/' | tr '\n' ' ')
	[ -z "$n" ] && ok "links system frameworks and libraries only" || fail "links $n"
	tags=$(go version -m "$exe" 2>/dev/null | awk '$1 == "build" && $2 ~ /^-tags=/ { sub(/^-tags=/, "", $2); print $2; exit }')
	wanttags=${EXPECT_TAGS-$([ $sys = tvos ] && echo tvos,live || echo live)}
	[ "$tags" = "$wanttags" ] && ok "go version -m: -tags=${tags:-none}" || fail "go version -m: -tags=${tags:-none}, want ${wanttags:-none}"
	case ",$tags," in *,dancefitvndev,*) fail "a developer's build (dancefitvndev)" ;; esac
else
	fail "the app has no program $exe"
	exe=/dev/null
fi
repo_pi="$root/$(cfg "$sys.privacyManifest")"
pi="$app/PrivacyInfo.xcprivacy"
if [ -f "$pi" ]; then
	plutil -lint -s "$pi" && ok "PrivacyInfo.xcprivacy lints" || fail "PrivacyInfo.xcprivacy does not lint"
	plutil -convert json -o "$tmp/privacy.json" "$pi"
	if [ -f "$repo_pi" ]; then
		plutil -convert json -o "$tmp/repo-privacy.json" "$repo_pi"
		cmp -s "$tmp/privacy.json" "$tmp/repo-privacy.json" && ok "PrivacyInfo.xcprivacy is gdesk.json's $sys.privacyManifest" ||
			fail "PrivacyInfo.xcprivacy differs from $repo_pi"
	else
		fail "gdesk.json's $sys.privacyManifest names no file"
	fi
	tally < <(python3 -I "$scan" privacy "$tmp/privacy.json" "$tmp/imports.txt" "$exe")
else
	fail "the app has no PrivacyInfo.xcprivacy at its top (ITMS-91053); checking the repo's against the program"
	if [ -f "$repo_pi" ]; then
		plutil -convert json -o "$tmp/repo-privacy.json" "$repo_pi"
		tally < <(python3 -I "$scan" privacy "$tmp/repo-privacy.json" "$tmp/imports.txt" "$exe")
	fi
fi

# ---- selectors ---------------------------------------------------------
gdir="${GDESK_DIR:-$root/../gdesk-dancefitvn-app}"
# shellcheck disable=SC2086 # goflags is empty or "-tags tvos"
if (cd "$root" && GOOS=ios GOARCH=arm64 CGO_ENABLED=1 GOFLAGS= go list $goflags -deps -f '{{$d := .Dir}}{{range .GoFiles}}{{$d}}/{{.}}{{"\n"}}{{end}}{{range .CgoFiles}}{{$d}}/{{.}}{{"\n"}}{{end}}' . 2>/dev/null) |
	grep -E "^($root|$(cd "$gdir" 2>/dev/null && pwd || echo /nonexistent))/" >"$tmp/gofiles.txt"; then
	tally < <(python3 -I "$scan" selectors "$(xcrun --sdk "$sdkname" --show-sdk-path)" "$tmp/gofiles.txt")
else
	warn "go list found no Go files of the $sys build (selectors not checked)"
fi

# ---- signature, symbols and sizes --------------------------------------
if codesign -dv "$app" >/dev/null 2>&1 && [ -d "$app/_CodeSignature" ]; then
	codesign -dvv "$app" 2>"$tmp/cs.txt" || true
	auth=$(awk -F= '/^Authority=/ { print $2; exit }' "$tmp/cs.txt")
	codesign --verify --strict --deep "$app" 2>/dev/null && ok "signature verifies (${auth:-ad hoc})" || fail "the signature does not verify"
	case $auth in
	"Apple Distribution: "*) ok "signed by $auth" ;;
	"") warn "signed ad hoc (App Store Connect needs an Apple Distribution signature)" ;;
	*) fail "signed by $auth, want Apple Distribution" ;;
	esac
	if [ -n "$auth" ]; then
		[ -f "$app/embedded.mobileprovision" ] && ok "embedded.mobileprovision" || fail "signed, but no embedded.mobileprovision"
		codesign -d --entitlements - --xml "$app" >"$tmp/ents.plist" 2>/dev/null || true
		gta=$(plutil -extract get-task-allow raw -o - "$tmp/ents.plist" 2>/dev/null || echo absent)
		[ "$gta" = true ] && fail "get-task-allow true (a development profile)" || ok "get-task-allow $gta"
		aid=$(plutil -extract application-identifier raw -o - "$tmp/ents.plist" 2>/dev/null || echo none)
		[[ $aid == *".$(cfg id)" ]] && ok "application-identifier $aid" || fail "application-identifier $aid, want <team>.$(cfg id)"
	fi
else
	warn "the app is not signed (App Store Connect needs it signed with signing.iosIdentity and the App Store profile)"
fi
if [ -n "${DSYM:-}" ]; then
	u1=$(dwarfdump --uuid "$exe" 2>/dev/null | awk '{ print $2; exit }')
	u2=$(dwarfdump --uuid "$DSYM" 2>/dev/null | awk '{ print $2; exit }')
	[ -n "$u1" ] && [ "$u1" = "$u2" ] && ok "dSYM UUID $u1" || fail "program UUID ${u1:-?}, dSYM ${u2:-?}"
fi
echo "size  .app $(du -sh "$app" | cut -f1 | tr -d ' '), program $(du -h "$exe" 2>/dev/null | cut -f1 | tr -d ' ')${ipa:+, .ipa $(du -h "$ipa" | cut -f1 | tr -d ' ')}"

echo "check-ios: $fails FAIL, $warns WARN"
[ $fails -eq 0 ]
