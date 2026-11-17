#!/usr/bin/env bash
# Adapted from jplayer-mobile/scripts/release.sh at 49de3c2 and
# jplayer-tv/scripts/tvos-release.sh at 781bee5 (doctor, check_profile,
# the relative go.mod of modfile, provenance).
#
# Builds DanceFITVN for the stores and checks what was built. Nothing is
# uploaded: App Store Connect, TestFlight and Play Console are the account
# holder's steps (docs/BLOCKERS.md).
#
#   scripts/release.sh ios|tvos|android|all
#   scripts/release.sh --sim ios|tvos
#   scripts/release.sh doctor
#   scripts/release.sh -h | --help
#
# ios, tvos  gdesk build -os <sys> -device -release -tags live; on iOS
#            scripts/ios-faceid-plist.sh --nosign writes
#            NSFaceIDUsageDescription (gDesk G3a pending: gdesk package
#            would ship without it and Face ID would crash). With
#            gdesk.json's signing.iosIdentity and signing.iosProfile
#            (tvOS: signing.tvosProfile) it then signs the app as gdesk
#            package does (kits/cli pack.go: the profile embedded, the
#            profile's entitlements, codesign with the identity) into
#            dist/release/DanceFITVN-<version>-<build>.ipa; without them it
#            zips the unsigned app under Payload/ into
#            DanceFITVN-<version>-<build>[-tvos]-unsigned.ipa, for checking
#            the layout only (App Store Connect refuses it). The dSYM goes
#            beside it, zipped.
# android    gdesk build -os android -release -tags live: an APK, re-signed
#            with signing.androidKeystore (apksigner) when it is set and
#            its password variable (signing.androidPasswordEnv) holds the
#            password, else kept on gDesk's debug key and named
#            -debugkey.apk (testers only). With bundletool (BUNDLETOOL or on
#            the PATH) also gdesk package -os android -tags live: the .aab
#            Play takes. Without it no .aab is made and the sizes are the
#            APK's (Q-P8-11: bundletool is not downloaded). The native
#            debug symbols go beside it.
# all        ios, tvos and android.
# --sim      the simulator's -release -tags live build of ios or tvos (the
#            Face ID key written, signed ad hoc), copied into
#            local/release/sim/<sys>/: what can be installed on the
#            simulators to see the shipped build start. Not shipped.
# doctor     what a signed release still needs (exit 1 while anything
#            is missing): Xcode and its SDKs, gdesk, gdesk.json's version,
#            build and icon, the Apple Distribution identity, the App Store
#            profiles, the Android upload keystore and its password,
#            bundletool, the App Store Connect API key.
#
# Every build is made from a snapshot of the working tree in
# local/release/src (dist/ and other builds are left alone) whose go.mod
# names gDesk by a relative path: go.mod's absolute replace paths would
# otherwise put the user's home folder into the program's build
# information. Each artefact is checked: scripts/release/check-ios.sh or
# check-android.sh, and scripts/check-livebuild.sh --check (none of the
# mock's or a developer's markers, the live API's base present). A failed
# check stops the release (an unsigned .ipa or a debug key is a warning).
# Every artefact gets a line in dist/release/SHA256SUMS; each check's
# output is kept as dist/release/<artefact>.check.txt, and what the build
# comes from as <base>-<sys>.provenance.txt.
#
# Environment
#   DFV_FLAVOUR=demo   the mock's app for internal testers (no -tags live:
#                      the mock and its sample media inside; Q-P8-22),
#                      named DanceFITVN-demo-...; check-livebuild is not run
#   DFV_RELEASE_DIR    where the artefacts go (default dist/release)
#   BUNDLETOOL         bundletool (a program or a .jar), for the .aab
#   ASC_KEY_ID, ASC_ISSUER_ID, ASC_KEY_FILE
#                      the App Store Connect API key doctor looks for (an
#                      upload is not this script's)
#   JAVA_HOME, ANDROID_HOME  default to the ones of CLAUDE.md §6
#
# Android's 16 KB pages: gdesk's link leaves libmain.so's LOAD segments
# aligned to 4 KB (kits/cli android.go sets CGO_LDFLAGS without
# -z max-page-size; Go passes none), which Play refuses for apps that
# target Android 15+. Until gDesk passes it, this script asks clang for it
# through CCC_OVERRIDE_OPTIONS, and check-android.sh checks the result.
#
# Exit status: 0 when every build and check passed, 1 otherwise, 2 on
# usage.
set -euo pipefail

usage() { awk 'NR > 1 && /^#/ { sub(/^# ?/, ""); print; next } NR > 1 { exit }' "$0"; }

root="$(cd "$(dirname "$0")/.." && pwd)"
cd "$root"
export JAVA_HOME="${JAVA_HOME:-/opt/homebrew/opt/openjdk@21/libexec/openjdk.jdk/Contents/Home}"
export ANDROID_HOME="${ANDROID_HOME:-/opt/homebrew/share/android-commandlinetools}"

sim=0
if [ "${1:-}" = --sim ]; then
	sim=1
	shift
fi
case "${1:-}" in
-h | --help) usage; exit 0 ;;
ios | tvos) systems=("$1") ;;
android | all | doctor) [ $sim = 0 ] || { usage >&2; exit 2; }; systems=("$1") ;;
*) usage >&2; exit 2 ;;
esac
[ "${systems[0]}" = all ] && systems=(ios tvos android)

cfg() { plutil -extract "$1" raw -o - gdesk.json 2>/dev/null; }
signing() { plutil -extract "signing.$1" raw -o - gdesk.json 2>/dev/null || true; }
app_name=$(cfg name)
bundle_id=$(cfg id)
version=$(cfg version)
build=$(cfg build || true)
case ${DFV_FLAVOUR:-live} in
live) flavour=live tags=live base="$app_name-$version-${build:-0}" ;;
demo) flavour=demo tags="" base="$app_name-demo-$version-${build:-0}" ;;
*) echo "release: DFV_FLAVOUR is live or demo, not '$DFV_FLAVOUR'" >&2; exit 2 ;;
esac
rel="${DFV_RELEASE_DIR:-$root/dist/release}"
src="$root/local/release/src"
bundletool="${BUNDLETOOL:-$(command -v bundletool || true)}"
tmp=$(mktemp -d "${TMPDIR:-/tmp}/dfv-release.XXXXXX")
trap 'rm -rf "$tmp"' EXIT

# say prints one line of doctor: OK, FAIL, WARN or INFO, a name, a detail.
say() { printf '%-4s %-16s %s\n' "$1" "$2" "$3"; }

# die prints a message and the next step, and exits 1.
die() {
	echo "release: $1" >&2
	[ -n "${2:-}" ] && echo "next step: $2" >&2
	exit 1
}

# last prints the last line of gdesk's output that names an existing path.
last() { local p="" l; while IFS= read -r l; do [ -e "$l" ] && p=$l; done; echo "$p"; }

# ship copies $1 into dist/release as $2 and records its sum.
ship() {
	mkdir -p "$rel"
	cp -f "$1" "$rel/$2"
	(cd "$rel" && { grep -v "  $2\$" SHA256SUMS 2>/dev/null || true; } >SHA256SUMS.new &&
		shasum -a 256 "$2" >>SHA256SUMS.new && mv SHA256SUMS.new SHA256SUMS)
	echo "release: $rel/$2 ($(du -h "$rel/$2" | cut -f1 | tr -d ' '))"
}

# check runs a check (its arguments), keeping its output as
# dist/release/$1.check.txt; a failure stops the release.
check() {
	local name=$1
	shift
	mkdir -p "$rel"
	if "$@" 2>&1 | tee "$rel/$name.check.txt"; then
		return 0
	fi
	die "$* failed (dist/release/$name.check.txt)"
}

# snapshot copies the working tree into $src, with go.mod's absolute
# replace paths made relative to it (no home folder in the program).
snapshot() {
	[ -f "$tmp/snapshot" ] && return
	mkdir -p "$src"
	rsync -a --delete --exclude '/.git' --exclude '/dist' --exclude '/local' --exclude '__pycache__' --exclude '/reference' \
		--exclude '/e2e' --exclude '/dancefitvn-app' --exclude '*.test' --exclude '.DS_Store' \
		--exclude '/build-*.log' "$root/" "$src/"
	python3 -I - "$src" <<'PY'
import os, re, sys
src = sys.argv[1]
path = os.path.join(src, "go.mod")
out = []
for line in open(path, encoding="utf-8"):
    m = re.match(r"^(\s*\S+(?:\s+\S+)?\s+=>\s+)(/\S+)(\s*)$", line)
    if m:
        line = m.group(1) + os.path.relpath(m.group(2), src) + m.group(3)
    out.append(line)
open(path, "w", encoding="utf-8").write("".join(out))
PY
	touch "$tmp/snapshot"
}

# gbuild runs gdesk build in the snapshot with the flavour's tags and
# prints what it built; its log goes to local/release/src/build-<name>.log.
gbuild() {
	local name=$1 out
	shift
	snapshot
	# shellcheck disable=SC2046 # the tags are one word or none
	out=$(cd "$src" && GOFLAGS= gdesk build "$@" $([ -n "$tags" ] && echo "-tags $tags") 2>"$src/build-$name.log" | last) || true
	if [ -z "$out" ] || [ ! -e "$out" ]; then
		tail -20 "$src/build-$name.log" >&2
		die "gdesk build $* failed (local/release/src/build-$name.log)"
	fi
	echo "$out"
}

# provenance prints what the build of $1 comes from.
provenance() {
	local g="$root/../gdesk-dancefitvn-app"
	echo "date            $(date -u +%Y-%m-%dT%H:%M:%SZ)"
	echo "system          $1"
	echo "version         $version (${build:-no build})"
	echo "bundle id       $bundle_id"
	echo "flavour         $flavour (tags: ${tags:-none})"
	echo "go              $(go version)"
	echo "xcode           $(xcodebuild -version 2>/dev/null | tr '\n' ' ')"
	echo "gdesk cli       $(shasum -a 256 "$(command -v gdesk)" | cut -d' ' -f1)"
	echo "gdesk head      $(git -C "$g" rev-parse HEAD 2>/dev/null || echo '?') ($(git -C "$g" rev-parse --abbrev-ref HEAD 2>/dev/null || echo '?'))"
	echo "gdesk changes   $( (git -C "$g" diff HEAD; git -C "$g" status --short) 2>/dev/null | shasum -a 256 | cut -d' ' -f1)"
	echo "app tree        $(cd "$src" && find . -type f ! -path './dist/*' ! -name 'build-*.log' -print0 | LC_ALL=C sort -z | xargs -0 shasum -a 256 | shasum -a 256 | cut -d' ' -f1)"
}

# signid prints the SHA-1 of the valid code-signing identity named $1.
signid() {
	security find-identity -v -p codesigning | awk -v n="\"$1\"" -v h="$1" 'index($0, n) || $2 == h { print $2; f = 1; exit } END { exit !f }'
}

# ---------------------------------------------------------------------
# doctor
# ---------------------------------------------------------------------

# check_profile FILE PLATFORM prints the App Store profile's checks
# (adapted from jplayer-tv's) and returns 1 on any FAIL.
check_profile() {
	local file=$1 platform=$2 team=$3 sha
	if [ ! -f "$file" ]; then
		say FAIL "$platform-profile" "no $file"
		return 1
	fi
	security cms -D -i "$file" >"$tmp/profile.plist" 2>/dev/null || {
		say FAIL "$platform-profile" "$file is not a CMS-signed provisioning profile"
		return 1
	}
	sha=$(signid "$(signing iosIdentity)" 2>/dev/null || true)
	python3 -I - "$tmp/profile.plist" "$bundle_id" "$sha" "$platform" <<'PY'
import datetime, hashlib, plistlib, sys
path, bundle, sha, platform = sys.argv[1:5]
fails = 0
def say(kind, name, detail):
    global fails
    fails += kind == "FAIL"
    print(f"{kind:<4} {name:<16} {detail}")
p = plistlib.load(open(path, "rb"))
team = (p.get("TeamIdentifier") or ["?"])[0]
say("INFO", f"{platform}-profile", f"name {p.get('Name', '?')!r}, team {team}")
want = {"ios": "iOS", "tvos": "tvOS"}[platform]
plat = p.get("Platform", []) or []
say("OK" if want in plat else "FAIL", f"{platform}-platform", ", ".join(plat) or f"none (want {want})")
if p.get("ProvisionedDevices"):
    say("FAIL", f"{platform}-devices", f"lists {len(p['ProvisionedDevices'])} device(s): a development or ad hoc profile, not App Store")
elif p.get("ProvisionsAllDevices"):
    say("FAIL", f"{platform}-devices", "ProvisionsAllDevices: an enterprise profile")
else:
    say("OK", f"{platform}-devices", "none listed (App Store distribution)")
ent = p.get("Entitlements", {}) or {}
appid = ent.get("application-identifier", "")
say("OK" if appid == f"{team}.{bundle}" else "FAIL", f"{platform}-app-id",
    appid + ("" if appid == f"{team}.{bundle}" else f": want the explicit App ID {team}.{bundle}"))
say("FAIL" if ent.get("get-task-allow") is True else "OK", f"{platform}-task-allow", str(ent.get("get-task-allow", "absent")))
exp = p.get("ExpirationDate")
now = datetime.datetime.now(datetime.timezone.utc).replace(tzinfo=None)
if isinstance(exp, datetime.datetime) and exp > now:
    say("OK", f"{platform}-expiry", f"{exp:%Y-%m-%d} ({(exp - now).days} days left)")
else:
    say("FAIL", f"{platform}-expiry", f"expired or missing ({exp})")
certs = [hashlib.sha1(c).hexdigest().upper() for c in (p.get("DeveloperCertificates", []) or [])]
if not sha:
    say("WARN", f"{platform}-certificate", "no signing identity to compare")
else:
    say("OK" if sha in certs else "FAIL", f"{platform}-certificate",
        "holds the signing identity's certificate" if sha in certs else f"the identity's certificate is not among the profile's {len(certs)}")
sys.exit(1 if fails else 0)
PY
}

doctor() {
	local fails=0 id sha p ks env k
	miss() { fails=$((fails + 1)); }
	if xcodebuild -version >/dev/null 2>&1; then say OK xcode "$(xcodebuild -version | tr '\n' ' ')"; else say FAIL xcode "xcodebuild missing"; miss; fi
	for k in iphoneos appletvos; do
		if xcrun --sdk $k --show-sdk-version >/dev/null 2>&1; then say OK "$k-sdk" "$(xcrun --sdk $k --show-sdk-version)"; else say FAIL "$k-sdk" "no $k SDK"; miss; fi
	done
	if command -v gdesk >/dev/null; then say OK gdesk "$(command -v gdesk)"; else say FAIL gdesk "(cd ../gdesk-dancefitvn-app && go install ./kits/cli/cmd/gdesk)"; miss; fi
	say INFO version "$version (build ${build:-none}); flavour $flavour"
	if [ -z "$build" ]; then
		say FAIL build "gdesk.json has no \"build\": App Store Connect takes each build number once"; miss
	elif ls "$rel/$base".ipa "$rel/$base".aab >/dev/null 2>&1; then
		say WARN build "$base is in dist/release already (signed): bump \"build\" before the next upload"
	fi
	if [ "$(sips -g hasAlpha "$(cfg icon)" 2>/dev/null | awk '/hasAlpha/ { print $2 }')" = no ]; then
		say OK icon "$(cfg icon) opaque (ITMS-90717)"
	else
		say FAIL icon "$(cfg icon) has an alpha channel: a device release refuses it (ITMS-90717)"; miss
	fi
	say INFO art "icon.png and the tvOS brand assets come from docs/brand (generate.sh, cmd/brandcheck)"
	if grep -q 'NSFaceIDUsageDescription' "$root/../gdesk-dancefitvn-app/kits/cli/plist.go" 2>/dev/null; then
		say INFO faceid "kits/cli writes NSFaceIDUsageDescription (G3a): ios-faceid-plist.sh may go"
	else
		say INFO faceid "kits/cli writes no NSFaceIDUsageDescription (G3a pending): this script patches it before signing"
	fi
	if security find-identity -v -p codesigning | grep -q '"Apple Distribution: '; then
		say OK distribution "an Apple Distribution identity is in the keychain"
	else
		say FAIL distribution "no \"Apple Distribution\" identity in the keychain (Xcode > Settings > Accounts > Manage Certificates)"; miss
	fi
	id=$(signing iosIdentity)
	if [ -z "$id" ]; then
		say FAIL identity "gdesk.json has no signing.iosIdentity (\"Apple Distribution: <name> (<team>)\")"; miss
	elif sha=$(signid "$id"); then
		say OK identity "$id ($sha)"
	else
		say FAIL identity "\"$id\" is not a valid code-signing identity in the keychain"; miss
	fi
	for p in ios tvos; do
		k=$(signing "${p}Profile")
		if [ -z "$k" ]; then
			say FAIL "$p-profile" "gdesk.json has no signing.${p}Profile (an App Store profile for $bundle_id, kept under local/)"; miss
		else
			case $k in local/*) ;; *) say WARN "$p-profile" "$k is not under local/ (gitignored)" ;; esac
			check_profile "$root/$k" "$p" || miss
		fi
	done
	ks=$(signing androidKeystore)
	env=$(signing androidPasswordEnv)
	if [ -z "$ks" ]; then
		say FAIL keystore "gdesk.json has no signing.androidKeystore (Play's upload key; the builds use gDesk's debug key)"; miss
	elif [ ! -f "$root/$ks" ]; then
		say FAIL keystore "no $ks"; miss
	else
		say OK keystore "$ks, alias $(signing androidAlias)"
		if [ -n "$env" ] && [ -n "${!env:-}" ]; then say OK keystore-pass "\$$env set"; else say FAIL keystore-pass "signing.androidPasswordEnv (${env:-unset}) holds no password"; miss; fi
	fi
	if [ -n "$bundletool" ]; then
		say OK bundletool "$bundletool"
	else
		say FAIL bundletool "no BUNDLETOOL nor bundletool on the PATH: no .aab (Play needs one); sizes come from the APK"; miss
	fi
	if [ -n "${ASC_KEY_ID:-}" ] && [ -n "${ASC_ISSUER_ID:-}" ]; then
		say OK asc-key "ASC_KEY_ID and ASC_ISSUER_ID set (uploads: the account holder's, not this script's)"
	else
		say FAIL asc-key "ASC_KEY_ID and ASC_ISSUER_ID unset (App Store Connect API key, or the Transporter app, for the upload)"; miss
	fi
	say INFO accounts "App Store Connect app record and Play Console app for $bundle_id: the account holder's (docs/BLOCKERS.md)"
	if [ $fails -gt 0 ]; then
		echo "release: doctor: $fails item(s) missing for a signed release; unsigned and debug-key builds still work"
		return 1
	fi
	echo "release: doctor: ready for a signed release"
}

# ---------------------------------------------------------------------
# builds
# ---------------------------------------------------------------------

# apple builds, patches, signs or zips, checks and ships ios or tvos.
apple() {
	local sys=$1 app id profile pf sha ipa name suffix="" dsym
	[ "$sys" = tvos ] && suffix=-tvos
	# check-ios.sh's tags: the platform's, then the flavour's.
	if [ $flavour = demo ]; then
		[ "$sys" = tvos ] && export EXPECT_TAGS=tvos || export EXPECT_TAGS=
	fi
	# The symbols of an earlier build go: dsymutil updates a dSYM in place.
	rm -rf "$src/dist/symbols/$sys-arm64"
	app=$(gbuild "$sys-device" -os "$sys" -device -release)
	if [ "$sys" = ios ]; then
		scripts/ios-faceid-plist.sh --nosign "$app"
	fi
	dsym=$(find "$src/dist/symbols/$sys-arm64" -maxdepth 2 -name "$app_name.app.dSYM" 2>/dev/null | head -1)
	id=$(signing iosIdentity)
	pf=iosProfile
	[ "$sys" = tvos ] && pf=tvosProfile
	profile=$(signing $pf)
	if [ -n "$id" ] && [ -n "$profile" ]; then
		sha=$(signid "$id") || die "signing.iosIdentity \"$id\" is not a valid code-signing identity in the keychain" "scripts/release.sh doctor"
		[ -f "$root/$profile" ] || die "no signing.$pf $profile" "scripts/release.sh doctor"
		# As kits/cli pack.go's ipa: the profile goes in before signing,
		# its entitlements are the signature's.
		cp "$root/$profile" "$app/embedded.mobileprovision"
		security cms -D -i "$root/$profile" >"$tmp/profile.plist"
		plutil -extract Entitlements xml1 -o "$tmp/entitlements.plist" "$tmp/profile.plist"
		codesign --force --sign "$sha" --entitlements "$tmp/entitlements.plist" "$app"
		codesign --verify --strict --deep "$app"
		name="$base$suffix.ipa"
	else
		echo "release: no signing.iosIdentity/signing.$pf in gdesk.json: an UNSIGNED .ipa (layout check only)"
		name="$base$suffix-unsigned.ipa"
	fi
	DSYM="$dsym" check "$name.app" scripts/release/check-ios.sh "$app"
	rm -rf "$tmp/ipa"
	mkdir -p "$tmp/ipa/Payload"
	ditto "$app" "$tmp/ipa/Payload/$(basename "$app")"
	ipa="$tmp/$name"
	(cd "$tmp/ipa" && zip -qry -X "$ipa" Payload)
	check "$name" scripts/release/check-ios.sh "$ipa"
	if [ $flavour = live ]; then
		check "$name.livebuild" scripts/check-livebuild.sh --check "$ipa"
	fi
	ship "$ipa" "$name"
	if [ -n "$dsym" ]; then
		(cd "$(dirname "$dsym")" && ditto -c -k --keepParent "$(basename "$dsym")" "$tmp/dsym.zip")
		ship "$tmp/dsym.zip" "$base$suffix.dSYM.zip"
	fi
	provenance "$sys" >"$rel/$base-$sys.provenance.txt"
}

# android builds the APK (and the .aab with bundletool), checks and ships
# them.
android() {
	local apk ks alias env name syms aab bt
	# 16 KB LOAD segments (see the header): "#" keeps clang quiet about
	# the override, -Qunused-arguments about the linker flag on compiles.
	export CCC_OVERRIDE_OPTIONS='#+-Qunused-arguments +-Wl,-z,max-page-size=16384'
	rm -rf "$src/dist/symbols/android-arm64"
	apk=$(gbuild android -os android -release)
	ks=$(signing androidKeystore)
	alias=$(signing androidAlias)
	env=$(signing androidPasswordEnv)
	bt=$(ls -d "$ANDROID_HOME"/build-tools/* | sort -V | tail -1)
	if [ -n "$ks" ] && [ -n "$env" ] && [ -n "${!env:-}" ]; then
		cp "$apk" "$tmp/signed.apk"
		"$bt/apksigner" sign --ks "$root/$ks" --ks-key-alias "$alias" --ks-pass "env:$env" "$tmp/signed.apk"
		apk="$tmp/signed.apk"
		name="$base.apk"
	else
		echo "release: no signing.androidKeystore (or no password in its variable): the APK keeps gDesk's DEBUG key (testers only)"
		name="$base-debugkey.apk"
	fi
	check "$name" scripts/release/check-android.sh "$apk"
	if [ $flavour = live ]; then
		check "$name.livebuild" scripts/check-livebuild.sh --check "$apk"
	fi
	ship "$apk" "$name"
	syms=$(find "$src/dist/symbols/android-arm64" -maxdepth 2 -name native-debug-symbols.zip 2>/dev/null | head -1)
	[ -n "$syms" ] && ship "$syms" "$base-android-native-debug-symbols.zip"
	if [ -n "$bundletool" ]; then
		export BUNDLETOOL="$bundletool"
		# shellcheck disable=SC2046 # the tags are one word or none
		aab=$(cd "$src" && GOFLAGS= gdesk package -os android $([ -n "$tags" ] && echo "-tags $tags") 2>"$src/build-aab.log" | last) || true
		[ -n "$aab" ] && [ -f "$aab" ] || die "gdesk package -os android made no .aab (local/release/src/build-aab.log)"
		[ -n "$ks" ] && name="$base.aab" || name="$base-debugkey.aab"
		check "$name" scripts/release/check-android.sh "$aab"
		[ $flavour = live ] && check "$name.livebuild" scripts/check-livebuild.sh --check "$aab"
		ship "$aab" "$name"
	else
		echo "release: no bundletool (BUNDLETOOL or on the PATH): no .aab; Play needs one (scripts/release.sh doctor)"
	fi
	provenance android >"$rel/$base-android.provenance.txt"
}

# simulator builds the simulator's release of ios or tvos for a start
# check on the simulators.
simulator() {
	local sys=$1 app out
	app=$(gbuild "$sys-sim" -os "$sys" -release)
	[ "$sys" = ios ] && scripts/ios-faceid-plist.sh "$app"
	if [ $flavour = live ]; then
		scripts/check-livebuild.sh --check "$app"
	fi
	out="$root/local/release/sim/$sys"
	rm -rf "$out"
	mkdir -p "$out"
	ditto "$app" "$out/$(basename "$app")"
	echo "release: $out/$(basename "$app") (simulator, $flavour)"
}

case " ${GOFLAGS:-} " in
*" -tags"*) die "GOFLAGS holds -tags (\"$GOFLAGS\"), which gdesk would drop" "unset GOFLAGS; DFV_FLAVOUR picks the tags" ;;
esac
for sys in "${systems[@]}"; do
	case $sys in
	doctor) doctor ;;
	ios | tvos) if [ $sim = 1 ]; then simulator "$sys"; else apple "$sys"; fi ;;
	android) android ;;
	esac
done
