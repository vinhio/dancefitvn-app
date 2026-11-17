#!/bin/sh
# New for P8 (first real-iPhone run); no jPlayer counterpart (jplayer-mobile
# has no device flow in its scripts).
#
# Signs an iOS device .app with the Apple Development identity and the
# team's wildcard development profile, for install by devicectl:
#   e2e/ios-device-sign.sh <App.app>
# The profile is Xcode's "iOS Team Provisioning Profile: *" (it must list the
# phone's UDID, DFV_IPHONE_UDID, which is required); its entitlements are
# the ones the app is signed with.
# DFV_SIGN_IDENTITY (sha1 or name; default: the first "Apple Development")
# and DFV_PROFILE (a .mobileprovision path) override the lookup.
set -eu
app=${1:?usage: e2e/ios-device-sign.sh <App.app>}
udid=${DFV_IPHONE_UDID:?set DFV_IPHONE_UDID to the UDID of the iPhone, see xcrun devicectl list devices}
identity=${DFV_SIGN_IDENTITY:-$(security find-identity -v -p codesigning | sed -n 's/.*\([0-9A-F]\{40\}\) "Apple Development.*/\1/p' | head -1)}
[ -n "$identity" ] || { echo "ios-device-sign: no Apple Development identity" >&2; exit 1; }
profile=${DFV_PROFILE:-}
if [ -z "$profile" ]; then
	for f in "$HOME/Library/Developer/Xcode/UserData/Provisioning Profiles/"*.mobileprovision; do
		security cms -D -i "$f" >"${TMPDIR:-/tmp}/dfv-pp.plist" 2>/dev/null || continue
		n=$(plutil -extract Name raw "${TMPDIR:-/tmp}/dfv-pp.plist" 2>/dev/null || true)
		case "$n" in "iOS Team Provisioning Profile: *")
			plutil -extract ProvisionedDevices xml1 -o - "${TMPDIR:-/tmp}/dfv-pp.plist" | grep -q "$udid" && profile=$f && break ;;
		esac
	done
fi
[ -n "$profile" ] || { echo "ios-device-sign: no wildcard profile listing $udid" >&2; exit 1; }
tmp=$(mktemp -d)
trap 'rm -rf "$tmp"' EXIT
security cms -D -i "$profile" >"$tmp/pp.plist"
plutil -extract Entitlements xml1 -o "$tmp/ent.plist" "$tmp/pp.plist"
# the wildcard's application-identifier is TEAM.*: name the app's own id
team=$(plutil -extract TeamIdentifier.0 raw "$tmp/pp.plist")
bid=$(plutil -extract CFBundleIdentifier raw "$app/Info.plist")
/usr/libexec/PlistBuddy -c "Set :application-identifier $team.$bid" "$tmp/ent.plist"
# keep only what a wildcard profile allows and the app needs
for k in keychain-access-groups; do /usr/libexec/PlistBuddy -c "Delete :$k" "$tmp/ent.plist" >/dev/null 2>&1 || true; done
cp "$profile" "$app/embedded.mobileprovision"
codesign --force --sign "$identity" --entitlements "$tmp/ent.plist" --timestamp=none "$app"
codesign --verify --strict "$app"
echo "$app: signed with $identity, profile $(plutil -extract Name raw "$tmp/pp.plist")"
