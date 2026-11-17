#!/usr/bin/env bash
# Adapted from jplayer-tv/scripts/tvos-device.sh at 781bee5 (sign,
# check_profile) and e2e/ios-device-sign.sh.
#
# tvos-device-sign.sh signs a tvOS device bundle (gdesk build -os tvos
# -device, which leaves it unsigned) with the Apple Development identity and
# the team's tvOS development profile, as gdesk package does for an .ipa
# (kits/cli pack.go): the profile copied in as embedded.mobileprovision, the
# profile's entitlements with application-identifier set to TEAM.<bundle
# id> (a wildcard profile holds TEAM.*), codesign, codesign --verify.
#
#   scripts/tvos-device-sign.sh [App.app]   (default dist/tvos-arm64/DanceFITVN.app)
#
# The profile is the first tvOS development profile in Xcode's folder that
# lists the Apple TV (DFV_TV_UDID, a UDID, required) and whose App ID covers the
# bundle's: Xcode makes "tvOS Team Provisioning Profile: *" when it signs a
# tvOS target automatically (scripts/tvos-remote-device.sh's UI-test bundle
# does, with -allowProvisioningUpdates). DFV_PROFILE (a .mobileprovision)
# and DFV_SIGN_IDENTITY (SHA-1 or name; default the first "Apple
# Development") override the lookup. Nothing of either is written into the
# repository.
set -euo pipefail
root=$(cd "$(dirname "$0")/.." && pwd)
app=${1:-$root/dist/tvos-arm64/DanceFITVN.app}
udid=${DFV_TV_UDID:?set DFV_TV_UDID to the UDID of the Apple TV, see xcrun devicectl list devices}
[[ -d $app ]] || { echo "tvos-device-sign: no $app (gdesk build -os tvos -device)" >&2; exit 1; }
identity=${DFV_SIGN_IDENTITY:-$(security find-identity -v -p codesigning | sed -n 's/.*\([0-9A-F]\{40\}\) "Apple Development.*/\1/p' | head -1)}
[[ -n $identity ]] || { echo "tvos-device-sign: no Apple Development identity" >&2; exit 1; }
bid=$(plutil -extract CFBundleIdentifier raw "$app/Info.plist")

tmp=$(mktemp -d "${TMPDIR:-/tmp}/tvos-sign.XXXXXX")
trap 'rm -rf "$tmp"' EXIT

# fits FILE tells whether FILE is a tvOS development profile, not expired,
# listing the Apple TV, whose App ID is TEAM.* or TEAM.<bid>.
fits() {
	security cms -D -i "$1" >"$tmp/pp.plist" 2>/dev/null || return 1
	python3 -I - "$tmp/pp.plist" "$udid" "$bid" <<'PY'
import datetime, plistlib, sys
p = plistlib.load(open(sys.argv[1], "rb"))
udid, bid = sys.argv[2], sys.argv[3]
ent = p.get("Entitlements", {}) or {}
team = (p.get("TeamIdentifier") or [""])[0]
exp = p.get("ExpirationDate")
ok = ("tvOS" in (p.get("Platform") or []) and udid in (p.get("ProvisionedDevices") or [])
      and ent.get("get-task-allow") is True
      and ent.get("application-identifier") in (f"{team}.*", f"{team}.{bid}")
      and isinstance(exp, datetime.datetime) and exp > datetime.datetime.now(datetime.timezone.utc).replace(tzinfo=None))
sys.exit(0 if ok else 1)
PY
}

profile=${DFV_PROFILE:-}
if [[ -z $profile ]]; then
	for f in "$HOME/Library/Developer/Xcode/UserData/Provisioning Profiles/"*.mobileprovision \
		"$HOME/Library/MobileDevice/Provisioning Profiles/"*.mobileprovision; do
		[[ -f $f ]] && fits "$f" && { profile=$f; break; }
	done
fi
[[ -n $profile ]] || {
	echo "tvos-device-sign: no tvOS development profile lists $udid for $bid" >&2
	echo "next step: scripts/tvos-remote-device.sh builds its UI-test bundle with -allowProvisioningUpdates, which makes one" >&2
	exit 1
}
fits "$profile" || { echo "tvos-device-sign: $profile is not a tvOS development profile for $udid and $bid" >&2; exit 1; }

security cms -D -i "$profile" >"$tmp/pp.plist"
plutil -extract Entitlements xml1 -o "$tmp/ent.plist" "$tmp/pp.plist"
team=$(plutil -extract TeamIdentifier.0 raw "$tmp/pp.plist")
/usr/libexec/PlistBuddy -c "Set :application-identifier $team.$bid" "$tmp/ent.plist"
# Keep only what the app needs of a wildcard's entitlements.
/usr/libexec/PlistBuddy -c "Delete :keychain-access-groups" "$tmp/ent.plist" >/dev/null 2>&1 || true
cp "$profile" "$app/embedded.mobileprovision"
codesign --force --sign "$identity" --entitlements "$tmp/ent.plist" --timestamp=none "$app"
codesign --verify --deep --strict "$app"
echo "$app: signed for $team.$bid with profile \"$(plutil -extract Name raw "$tmp/pp.plist")\""
