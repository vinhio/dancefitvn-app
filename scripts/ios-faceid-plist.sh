#!/usr/bin/env bash
# Writes NSFaceIDUsageDescription into an iOS app's Info.plist and signs
# the app again, so that Face ID works in it (F6). Since gDesk's G3a,
# gdesk build writes the key itself from gdesk.json's ios.privacy.faceID
# (the same sentence, locales/vi.json's bio.faceIdUsage, which
# internal/app's TestFaceIDUsageIsGdeskJSONs holds equal): this script
# then sets the value it already has, a safety net for a gdesk older
# than G3a. Without the key iOS refuses Face ID, and internal/sys hides
# the Face ID button (Biometry answers BiometryNone) rather than failing.
#
#   scripts/ios-faceid-plist.sh [--nosign] <App.app> [sentence]
#
# Run it after every `gdesk build -os ios` (scripts/dev-build.sh ios does):
#
#   gdesk build -os ios && scripts/ios-faceid-plist.sh dist/ios-simulator-arm64/DanceFITVN.app
#
# The sentence is the second argument, else $DFV_FACEID_USAGE, else the
# key bio.faceIdUsage of locales/vi.json (one place for the app's
# Vietnamese texts). The app is signed ad
# hoc, keeping its entitlements: what the simulator needs. A device build
# signed with an identity is signed again with $DFV_SIGN_IDENTITY, which
# it then requires. --nosign writes the key and leaves the signature
# alone: scripts/release.sh signs the release itself afterwards (with the
# App Store identity and profile), or ships it unsigned.
set -euo pipefail

nosign=0
if [ "${1:-}" = --nosign ]; then
	nosign=1
	shift
fi
app=${1:?usage: scripts/ios-faceid-plist.sh [--nosign] <App.app> [sentence]}
locale="$(cd "$(dirname "$0")/.." && pwd)/locales/vi.json"
text=${2:-${DFV_FACEID_USAGE:-}}
if [ -z "$text" ]; then
	text=$(python3 -I -c 'import json, sys; print(json.load(open(sys.argv[1], encoding="utf-8"))["bio.faceIdUsage"])' "$locale") || {
		echo "ios-faceid-plist: no bio.faceIdUsage in $locale" >&2
		exit 1
	}
fi
plist="$app/Info.plist"
key=NSFaceIDUsageDescription

if [ ! -f "$plist" ]; then
	echo "ios-faceid-plist: no $plist" >&2
	exit 1
fi
if /usr/libexec/PlistBuddy -c "Print :$key" "$plist" >/dev/null 2>&1; then
	/usr/libexec/PlistBuddy -c "Set :$key $text" "$plist"
else
	/usr/libexec/PlistBuddy -c "Add :$key string $text" "$plist"
fi
plutil -lint "$plist" >/dev/null
if [ $nosign = 1 ]; then
	echo "$app: $key set, not signed (--nosign)"
	exit 0
fi

authority=$(codesign -dvv "$app" 2>&1 | sed -n 's/^Authority=//p' | head -1 || true)
identity=-
if [ -n "$authority" ]; then
	identity=${DFV_SIGN_IDENTITY:?the app is signed by "$authority": set DFV_SIGN_IDENTITY to sign it again}
fi
codesign --force --sign "$identity" --preserve-metadata=entitlements --timestamp=none "$app" 2>/dev/null
codesign --verify "$app"
echo "$app: $key set, signed with ${identity/#-/ad hoc}"
