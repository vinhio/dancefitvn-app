#!/usr/bin/env bash
# Adapted from jplayer-mobile/scripts/release/check-android.sh at 49de3c2.
#
# Checks an Android release build of DanceFITVN before it goes to Play
# Console or to testers, and optionally installs it on devices.
#
#   scripts/release/check-android.sh <DanceFITVN.apk | DanceFITVN.aab> [--install [serial...]]
#   scripts/release/check-android.sh -h | --help
#
# An .aab needs bundletool (BUNDLETOOL or on the PATH: validate, dump
# manifest); an .apk is read with the SDK's apkanalyzer, apksigner and
# zipalign. The manifest: package and versionName of gdesk.json,
# versionCode (EXPECT_VERSION_CODE, default 1000001: 1.0.0 build 1),
# minSdk 26, targetSdk 36 (EXPECT_TARGET_SDK), exactly the permissions
# the app asks for (gdesk.json's USE_BIOMETRIC and ACCESS_NETWORK_STATE,
# gDesk's INTERNET, its media service's and its receiver's), not
# debuggable, no usesCleartextTraffic, the network security config
# letting http reach 127.0.0.1 alone (gdesk.json's cleartextHosts, Q-P8-23;
# read from an .apk only), Back kept as before
# (enableOnBackInvokedCallback false), the media service of type
# mediaPlayback, portrait, an icon and a theme (no round icon is a
# warning: gdesk.json has no android.icon until DanceFITVN's art).
# Native code for arm64-v8a only, every library's LOAD segments aligned to
# 16 KB (Play, Android 15+), the .apk zip-aligned for 16 KB pages. Prints
# the signer (gDesk's debug key is a warning) and the sizes (an .apk's
# download size as apkanalyzer estimates it).
#
# --install (an .apk; an .aab needs bundletool build-apks) installs it on
# each serial (default: every device adb lists) with adb install -r,
# starts it, checks it is still running 5 s later with no crash in the
# crash log since the start (the log is never cleared: a phone's is the
# user's), takes a screenshot into CHECK_ANDROID_SHOTS (when set) and
# uninstalls it unless DFV_KEEP=1. Only com.jivecode.dancefitvn is
# installed or removed; no setting of the device is touched.
#
# Lines read "ok", "WARN" or "FAIL"; exit status 1 when any FAIL, 2 on
# usage. JAVA_HOME and ANDROID_HOME default to the ones of CLAUDE.md §6.
set -uo pipefail

usage() { awk 'NR > 1 && /^#/ { sub(/^# ?/, ""); print; next } NR > 1 { exit }' "$0"; }

case "${1:-}" in
-h | --help) usage; exit 0 ;;
"") usage >&2; exit 2 ;;
esac
pkgfile=$1
shift
install=0
serials=()
if [ "${1:-}" = --install ]; then
	install=1
	shift
	serials=("$@")
elif [ $# -gt 0 ]; then
	usage >&2
	exit 2
fi
[ -f "$pkgfile" ] || { echo "check-android: $pkgfile does not exist" >&2; exit 2; }
case $pkgfile in
*.apk) kind=apk ;;
*.aab) kind=aab ;;
*) echo "check-android: $pkgfile is neither an .apk nor an .aab" >&2; exit 2 ;;
esac

here="$(cd "$(dirname "$0")" && pwd)"
root="$(cd "$here/../.." && pwd)"
export JAVA_HOME="${JAVA_HOME:-/opt/homebrew/opt/openjdk@21/libexec/openjdk.jdk/Contents/Home}"
export ANDROID_HOME="${ANDROID_HOME:-/opt/homebrew/share/android-commandlinetools}"
adb="$ANDROID_HOME/platform-tools/adb"
bt="${BUNDLETOOL:-$(command -v bundletool || true)}"
readelf=$(ls "$ANDROID_HOME"/ndk/*/toolchains/llvm/prebuilt/*/bin/llvm-readelf 2>/dev/null | tail -1)
analyzer=$(ls "$ANDROID_HOME"/cmdline-tools/*/bin/apkanalyzer 2>/dev/null | head -1)
btools=$(ls -d "$ANDROID_HOME"/build-tools/* 2>/dev/null | sort -V | tail -1)
tmp=$(mktemp -d)
trap 'rm -rf "$tmp"' EXIT

bundletool() {
	case $bt in
	*.jar) "$JAVA_HOME/bin/java" -jar "$bt" "$@" ;;
	*) "$bt" "$@" ;;
	esac
}

fails=0
warns=0
ok() { echo "ok    $*"; }
warn() { echo "WARN  $*"; warns=$((warns + 1)); }
fail() { echo "FAIL  $*"; fails=$((fails + 1)); }
tally() {
	while IFS= read -r l; do
		echo "$l"
		case $l in WARN*) warns=$((warns + 1)) ;; FAIL*) fails=$((fails + 1)) ;; esac
	done
}
cfg() { plutil -extract "$1" raw -o - "$root/gdesk.json" 2>/dev/null; }

echo "check-android: $pkgfile ($kind)"
pkg=$(cfg id)

# ---- bundle and manifest -----------------------------------------------
manifest_ok=0
if [ $kind = aab ]; then
	if [ -z "$bt" ]; then
		fail "no bundletool (BUNDLETOOL or on the PATH): the .aab is not checked"
	else
		bundletool validate --bundle="$pkgfile" >"$tmp/validate.txt" 2>&1 && ok "bundletool validate" ||
			fail "bundletool validate: $(tail -3 "$tmp/validate.txt" | tr '\n' ' ')"
		bundletool dump manifest --bundle="$pkgfile" >"$tmp/manifest.xml" 2>"$tmp/dump.err" && manifest_ok=1 ||
			fail "bundletool dump manifest: $(tail -2 "$tmp/dump.err" | tr '\n' ' ')"
	fi
	: >"$tmp/nsc.xml"
else
	if [ -z "$analyzer" ]; then
		fail "no apkanalyzer in $ANDROID_HOME/cmdline-tools"
	else
		"$analyzer" manifest print "$pkgfile" >"$tmp/manifest.xml" 2>"$tmp/dump.err" && manifest_ok=1 ||
			fail "apkanalyzer manifest print: $(tail -2 "$tmp/dump.err" | tr '\n' ' ')"
		"$analyzer" resources xml --file res/xml/gdesk_network_security.xml "$pkgfile" >"$tmp/nsc.xml" 2>/dev/null || : >"$tmp/nsc.xml"
	fi
fi
cat >"$tmp/manifest.py" <<'PY'
import sys
import xml.etree.ElementTree as ET

path, nsc, pkg, version, code, target = sys.argv[1:]
A = "{http://schemas.android.com/apk/res/android}"
root = ET.parse(path).getroot()


def check(cond, good, bad, level="FAIL"):
    print(("ok    " + good) if cond else (f"{level:<6}" + bad))


def attr(el, name):
    return None if el is None else el.get(A + name)


# Enums come as names from a binary manifest, as hex from apkanalyzer and
# the bundle's protocol buffers: signature is 0x2, mediaPlayback 0x2,
# portrait 1 (R.attr).
def flags(v):
    try:
        return int(v, 16) if v and v.startswith("0x") else int(v)
    except (TypeError, ValueError):
        return None


check(root.get("package") == pkg, f"package {pkg}", f"package {root.get('package')}, want {pkg}")
check(attr(root, "versionName") == version, f"versionName {version}", f"versionName {attr(root, 'versionName')}, want {version}")
check(attr(root, "versionCode") == code, f"versionCode {code}", f"versionCode {attr(root, 'versionCode')}, want {code}")
sdk = root.find("uses-sdk")
check(attr(sdk, "minSdkVersion") == "26", "minSdk 26", f"minSdk {attr(sdk, 'minSdkVersion')}, want 26")
check(attr(sdk, "targetSdkVersion") == target, f"targetSdk {target}", f"targetSdk {attr(sdk, 'targetSdkVersion')}, want {target}")

rp = pkg + ".permission.GDESK_RECEIVER"
want = {"android.permission.INTERNET", "android.permission.ACCESS_NETWORK_STATE", "android.permission.USE_BIOMETRIC",
        "android.permission.FOREGROUND_SERVICE", "android.permission.FOREGROUND_SERVICE_MEDIA_PLAYBACK",
        "android.permission.WAKE_LOCK", rp}
have = {attr(p, "name") for p in root.findall("uses-permission")}
check(have == want, f"permissions: {len(have)}, as listed",
      f"permissions: extra {sorted(have - want)}, missing {sorted(want - have)}")
declared = {attr(p, "name"): attr(p, "protectionLevel") for p in root.findall("permission")}
check(declared.get(rp) == "signature" or flags(declared.get(rp)) == 2, f"{rp} declared, signature",
      f"{rp} declared as {declared.get(rp)!r}, want signature")

app = root.find("application")
check(attr(app, "debuggable") not in ("true", "0xffffffff"), "not debuggable", "android:debuggable true")
check(attr(app, "usesCleartextTraffic") not in ("true", "0xffffffff"), "no usesCleartextTraffic", "android:usesCleartextTraffic true")
check(attr(app, "networkSecurityConfig") is not None, "networkSecurityConfig set (gdesk.json cleartextHosts)", "no networkSecurityConfig")
check(attr(app, "enableOnBackInvokedCallback") == "false", "enableOnBackInvokedCallback false",
      f"enableOnBackInvokedCallback {attr(app, 'enableOnBackInvokedCallback')}, want false (Back as KEYCODE_BACK)")
check(attr(app, "icon") is not None, f"icon {attr(app, 'icon')}", "no android:icon")
check(attr(app, "theme") is not None, f"theme {attr(app, 'theme')}", "no android:theme")
check(attr(app, "roundIcon") is not None, f"roundIcon {attr(app, 'roundIcon')}",
      "no android:roundIcon (no adaptive icon until DanceFITVN's art: gdesk.json android.icon)", level="WARN")
acts = app.findall("activity")
orient = [attr(a, "screenOrientation") for a in acts]
check(any(o in ("portrait", "1") for o in orient), "activity portrait", f"activities' screenOrientation {orient}, want portrait")
fgs = [attr(s, "foregroundServiceType") for s in app.findall("service")]
check(any(t == "mediaPlayback" or (flags(t) or 0) & 2 for t in fgs), "service foregroundServiceType mediaPlayback",
      f"services' foregroundServiceType {fgs}")

try:
    n = ET.parse(nsc).getroot()
except (ET.ParseError, OSError):
    n = None
if n is None:
    print("WARN  network security config not read (an .aab's resources are protocol buffers)")
else:
    base = n.find("base-config")
    doms = [(d.text or "").strip() for c in n.findall("domain-config") if c.get("cleartextTrafficPermitted") == "true"
            for d in c.findall("domain")]
    check(base is not None and base.get("cleartextTrafficPermitted") == "false" and doms == ["127.0.0.1"],
          "network security config: cleartext to 127.0.0.1 alone",
          f"network security config: base cleartext {None if base is None else base.get('cleartextTrafficPermitted')}, cleartext domains {doms}")
PY
if [ $manifest_ok = 1 ]; then
	tally < <(python3 -I "$tmp/manifest.py" "$tmp/manifest.xml" "$tmp/nsc.xml" "$pkg" "$(cfg version)" "${EXPECT_VERSION_CODE:-1000001}" "${EXPECT_TARGET_SDK:-36}")
fi

# ---- native code -------------------------------------------------------
if [ $kind = aab ]; then libdir='base/lib' depth=3; else libdir='lib' depth=2; fi
abis=$(unzip -Z1 "$pkgfile" | awk -F/ -v d=$depth -v l="${libdir##*/}" '$(d-1) == l && NF > d { print $d }' | sort -u | tr '\n' ' ')
[ "$abis" = "arm64-v8a " ] && ok "native code: arm64-v8a only" || fail "native code ABIs: ${abis:-none}, want arm64-v8a"
unzip -Z1 "$pkgfile" | grep -q "^$libdir/arm64-v8a/libmain.so\$" || fail "no $libdir/arm64-v8a/libmain.so"
if [ -z "$readelf" ]; then
	warn "no llvm-readelf in the NDK: 16 KB alignment not checked"
else
	for so in $(unzip -Z1 "$pkgfile" | grep "^$libdir/.*\.so\$"); do
		unzip -p "$pkgfile" "$so" >"$tmp/lib.so"
		bad=""
		for a in $("$readelf" -lW "$tmp/lib.so" | awk '$1 == "LOAD" { print $NF }'); do
			[ $((a)) -ge 16384 ] || bad="$bad $a"
		done
		[ -z "$bad" ] && ok "$so: LOAD segments aligned to 16 KB or more" || fail "$so: LOAD alignment $bad < 16 KB"
	done
fi
if [ $kind = apk ]; then
	if [ -x "$btools/zipalign" ]; then
		"$btools/zipalign" -c -P 16 4 "$pkgfile" >/dev/null 2>&1 && ok "zipalign -c -P 16: aligned for 16 KB pages" || fail "zipalign -c -P 16 fails"
	else
		warn "no zipalign in $ANDROID_HOME/build-tools"
	fi
fi

# ---- signature and size ------------------------------------------------
if [ $kind = apk ]; then
	signer=$("$btools/apksigner" verify --print-certs "$pkgfile" 2>/dev/null | awk -F': ' '/certificate DN:/ { print $2; exit }')
else
	signer=$("$JAVA_HOME/bin/keytool" -printcert -jarfile "$pkgfile" 2>/dev/null | awk -F': ' '/^Owner:/ { print $2; exit }')
fi
case $signer in
"") fail "the $kind is not signed (or its signature does not verify)" ;;
*"GDesk Debug"* | *"gDesk Debug"*) warn "signed with gDesk's debug key ($signer): testers only; Play needs the upload key (signing.androidKeystore)" ;;
*) ok "signed by $signer" ;;
esac
size="size  .$kind $(du -h "$pkgfile" | cut -f1 | tr -d ' ') ($(stat -f %z "$pkgfile") bytes)"
if [ $kind = apk ] && [ -n "$analyzer" ]; then
	dl=$("$analyzer" apk download-size "$pkgfile" 2>/dev/null || true)
	[ -n "$dl" ] && size="$size, download $dl bytes (apkanalyzer estimate)"
fi
lib=$(unzip -Zl "$pkgfile" 2>/dev/null | awk '/libmain.so$/ { print $4; exit }')
[ -n "$lib" ] && size="$size, libmain.so $lib bytes uncompressed"
echo "$size"

# ---- install -----------------------------------------------------------
if [ $install -eq 1 ]; then
	[ $kind = apk ] || { fail "--install takes an .apk (an .aab goes through bundletool build-apks)"; serials=(); }
	[ ${#serials[@]} -gt 0 ] || [ $kind != apk ] || serials=($("$adb" devices | awk 'NR > 1 && $2 == "device" { print $1 }'))
	[ ${#serials[@]} -gt 0 ] || [ $kind != apk ] || fail "--install: no device"
	for s in "${serials[@]}"; do
		api=$("$adb" -s "$s" shell getprop ro.build.version.sdk </dev/null | tr -d '\r')
		model=$("$adb" -s "$s" shell getprop ro.product.model </dev/null | tr -d '\r')
		if ! "$adb" -s "$s" install -r "$pkgfile" </dev/null >"$tmp/install.txt" 2>&1; then
			fail "$s ($model, API $api): adb install: $(grep -m1 -oE 'INSTALL_[A-Z_]+|Failure.*' "$tmp/install.txt")"
			continue
		fi
		since=$("$adb" -s "$s" shell date +%s </dev/null | tr -d '\r')
		"$adb" -s "$s" shell am force-stop "$pkg" </dev/null
		"$adb" -s "$s" shell am start -W -n "$pkg/dev.gdesk.android.GDeskActivity" </dev/null >"$tmp/start.txt" 2>&1
		sleep 5
		pid=$("$adb" -s "$s" shell pidof "$pkg" </dev/null | tr -d '\r')
		crash=$("$adb" -s "$s" logcat -d -b crash -T "$since.000" </dev/null 2>/dev/null | grep -m1 -E "FATAL EXCEPTION|Fatal signal|$pkg" || true)
		total=$(awk -F': ' '/TotalTime/ { print $2 }' "$tmp/start.txt" | tr -d '\r')
		if [ -n "${CHECK_ANDROID_SHOTS:-}" ]; then
			mkdir -p "$CHECK_ANDROID_SHOTS"
			"$adb" -s "$s" exec-out screencap -p </dev/null >"$CHECK_ANDROID_SHOTS/$(echo "$s" | tr -c 'A-Za-z0-9.\n-' _).png" 2>/dev/null || true
		fi
		if [ -n "$pid" ] && [ -z "$crash" ]; then
			ok "$s ($model, API $api): installed, started in ${total:-?} ms, running (pid $pid)"
		else
			fail "$s ($model, API $api): installed but not running after 5 s${crash:+: $crash}"
		fi
		if [ "${DFV_KEEP:-}" != 1 ]; then
			"$adb" -s "$s" uninstall "$pkg" </dev/null >/dev/null 2>&1 && ok "$s: $pkg uninstalled" || warn "$s: could not uninstall $pkg"
		fi
	done
fi

echo "check-android: $fails FAIL, $warns WARN"
[ $fails -eq 0 ]
