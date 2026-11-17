#!/bin/sh
# Adapted from jplayer-mobile/e2e/devtest-android.sh at 49de3c2.
# Changed: the app's id and name, the AVD's name, DFV_E2E_BG for
# JPLAYER_E2E_BG, the driver's commands trimmed to the generic ones and
# the network's (airplane on|off added), the output saved in local/p4/logs.
#
# Builds the tests of one package of the app into an APK, runs it on the
# device adb sees (an emulator: emulator -avd dfv-412x892 -no-snapshot)
# and prints its output, read from logcat:
#   e2e/devtest-android.sh [pkg]
# pkg is a path from the repository's top (default ./e2e/engine). An app
# gets no command line, so every test of the package runs and prints its
# name unless DEVTEST_RUN=<pattern> is set: it is written to run.txt in the
# media folder and the app uses it as -test.run. DEVTEST_TIMEOUT is the
# seconds to wait for the verdict (default 300). The app's own process is
# watched: a crash or exit without a verdict fails at once.
# ANDROID_HOME names the SDK (default ~/Library/Android/sdk, else
# Homebrew's android-commandlinetools); JAVA_HOME the JDK (default
# Homebrew's openjdk@21). Modelled on gDesk's
# bridge/backend/android/emulator/test.sh: gDesk's Java classes compiled
# with javac and d8, GDeskActivity, the media service declared as
# android.mediaPlayback does, GDeskFiles as gdesk build declares it (left
# out with DEVTEST_NOFILES=1), no cleartext traffic but to 127.0.0.1, as
# the app's android.cleartextHosts makes it. The media comes from
# e2e/prepare-media.sh and is pushed into the app's external files folder.
# The tests drive what only adb can through lines "devtest: <n> <cmd>" in
# their log, which the script runs as it reads them and answers in
# devtest-<n>.txt of that folder: top (the resumed activity), back (the
# Back key), wifi on|off and data on|off (adb shell svc wifi|data
# enable|disable), airplane on|off (adb shell cmd connectivity
# airplane-mode enable|disable), for e2e/sys/network_android_test.go
# (whatever the outcome, airplane mode off and both networks on again
# after a run that stopped half-way), home (the Home key), front (the
# activity started again: singleTask brings it back), sleep and wake (the
# screen off and on, then the keyguard dismissed). See
# e2e/sys/platform_android_test.go. DFV_E2E_BG gives the seconds of each
# phase of the background measures (pushed as bg.txt).
# On a real phone (adb sees no emulator: ro.kernel.qemu is not 1) the
# driver changes nothing of the system: wifi, data, airplane, sleep and
# wake are answered "refused: real device" without running, and phone.txt
# is pushed into the media folder so that the tests skip those steps (they
# are checked by hand there). shot-NAME (a screencap into
# local/p4/shots/android-NAME.png, pushed back as shot-NAME.png) works on
# both.
set -eu
here=$(cd "$(dirname "$0")" && pwd)
root=$(cd "$here/.." && pwd)
sdk=${ANDROID_HOME:-$HOME/Library/Android/sdk}
[ -d "$sdk" ] || sdk=/opt/homebrew/share/android-commandlinetools
ndk=${ANDROID_NDK_HOME:-$(ls -d "$sdk"/ndk/* | sort -V | tail -1)}
tools=$(ls -d "$sdk"/build-tools/* | sort -V | tail -1)
platform=$(ls -d "$sdk"/platforms/android-* | sort -V | tail -1)
host=$(uname -s | tr '[:upper:]' '[:lower:]')-x86_64
adb="$sdk/platform-tools/adb"
: "${JAVA_HOME:=/opt/homebrew/opt/openjdk@21/libexec/openjdk.jdk/Contents/Home}"
export JAVA_HOME
PATH="$JAVA_HOME/bin:$PATH"
pkg=${1:-./e2e/engine}
appid=com.jivecode.dancefitvn.e2e
activity=dev.gdesk.android.GDeskActivity
out=$(mktemp -d)
trap 'rm -rf "$out"' EXIT

"$adb" get-state >/dev/null 2>&1 || { echo "devtest-android: no device: start the AVD first (emulator -avd dfv-412x892 -no-snapshot)" >&2; exit 1; }

mkdir -p "$out/apk/lib/arm64-v8a"
(cd "$root" && GOOS=android GOARCH=arm64 CGO_ENABLED=1 CGO_CFLAGS= CGO_LDFLAGS= \
	CC="$ndk/toolchains/llvm/prebuilt/$host/bin/aarch64-linux-android26-clang" \
	go test -c -buildmode=c-shared -o "$out/apk/lib/arm64-v8a/libmain.so" "$pkg")

# gDesk's Java, for the oldest Android the app runs on (API 26).
gdroid=$(cd "$root" && go list -m -f '{{.Dir}}' github.com/gdeskdev/desk/bridge/backend/android)
mkdir -p "$out/classes"
find "$gdroid/java" -name '*.java' >"$out/sources"
javac -source 8 -target 8 -bootclasspath "$platform/android.jar" -encoding UTF-8 -Xlint:all,-options \
	-d "$out/classes" @"$out/sources"
"$tools/d8" --release --min-api 26 --lib "$platform/android.jar" --output "$out/apk" \
	$(find "$out/classes" -name '*.class')

provider='    <provider android:name="dev.gdesk.android.GDeskFiles" android:authorities="'$appid'.gdesk.files" android:exported="false" android:grantUriPermissions="true"/>'
[ "${DEVTEST_NOFILES:-0}" = 1 ] && provider='    <!-- no GDeskFiles (DEVTEST_NOFILES=1) -->'
cat >"$out/AndroidManifest.xml" <<MANIFEST
<?xml version="1.0" encoding="utf-8"?>
<manifest xmlns:android="http://schemas.android.com/apk/res/android" package="$appid" android:versionCode="1" android:versionName="1">
  <uses-permission android:name="android.permission.INTERNET"/>
  <uses-permission android:name="android.permission.ACCESS_NETWORK_STATE"/>
  <permission android:name="$appid.permission.GDESK_RECEIVER" android:protectionLevel="signature"/>
  <uses-permission android:name="$appid.permission.GDESK_RECEIVER"/>
  <uses-permission android:name="android.permission.FOREGROUND_SERVICE"/>
  <uses-permission android:name="android.permission.FOREGROUND_SERVICE_MEDIA_PLAYBACK"/>
  <uses-permission android:name="android.permission.WAKE_LOCK"/>
  <application android:label="DanceFITVN e2e" android:networkSecurityConfig="@xml/gdesk_network_security" android:hasCode="true" android:extractNativeLibs="true">
    <activity android:name="$activity" android:exported="true" android:launchMode="singleTask" android:configChanges="orientation|screenSize|screenLayout|smallestScreenSize|keyboard|keyboardHidden|navigation|density" android:windowSoftInputMode="adjustResize">
      <meta-data android:name="android.app.lib_name" android:value="main"/>
      <intent-filter><action android:name="android.intent.action.MAIN"/><category android:name="android.intent.category.LAUNCHER"/></intent-filter>
    </activity>
    <service android:name="dev.gdesk.android.GDeskMediaService" android:exported="false" android:foregroundServiceType="mediaPlayback"/>
$provider
  </application>
</manifest>
MANIFEST
# Cleartext to 127.0.0.1 alone, as gdesk build writes it for the app's
# android.cleartextHosts (kits/cli/androidres.go).
mkdir -p "$out/res/xml"
cat >"$out/res/xml/gdesk_network_security.xml" <<XML
<?xml version="1.0" encoding="utf-8"?>
<network-security-config>
  <base-config cleartextTrafficPermitted="false"/>
  <domain-config cleartextTrafficPermitted="true">
    <domain includeSubdomains="false">127.0.0.1</domain>
  </domain-config>
</network-security-config>
XML
"$tools/aapt2" compile --dir "$out/res" -o "$out/res.zip"
"$tools/aapt2" link -o "$out/base.apk" -I "$platform/android.jar" --manifest "$out/AndroidManifest.xml" --min-sdk-version 26 --target-sdk-version "${platform##*-}" "$out/res.zip"
(cd "$out/apk" && zip -q -r ../base.apk .)
"$tools/zipalign" -P 16 -f 4 "$out/base.apk" "$out/aligned.apk"
keystore="$out/test.keystore"
keytool -genkeypair -keystore "$keystore" -storepass android -alias t -keypass android -keyalg RSA -keysize 2048 -validity 1 -dname CN=test >/dev/null 2>&1
"$tools/apksigner" sign --ks "$keystore" --ks-pass pass:android --out "$out/tests.apk" "$out/aligned.apk"

phone=0
[ "$("$adb" shell getprop ro.kernel.qemu | tr -d '\r')" = 1 ] || [ "$("$adb" shell getprop ro.boot.qemu | tr -d '\r')" = 1 ] || phone=1
"$here/prepare-media.sh" "$out/media"
[ "$phone" = 1 ] && printf 'phone\n' >"$out/media/phone.txt"
"$adb" install -r "$out/tests.apk" >/dev/null
files=/sdcard/Android/data/$appid/files
"$adb" shell "rm -rf $files; mkdir -p $files"
"$adb" push "$out"/media/. "$files/" >/dev/null
# The folders adb push makes are the shell's, 0770 with ext_data_rw: on a
# phone (Android 16) the app cannot go into them. Opened to all, in the
# app's own folder.
"$adb" shell "chmod -R a+rwX $files"
if [ -n "${DFV_E2E_BG:-}" ]; then
	printf '%s\n' "$DFV_E2E_BG" >"$out/bg.txt"
	"$adb" push "$out/bg.txt" "$files/bg.txt" >/dev/null
fi
if [ -n "${DEVTEST_RUN:-}" ]; then
	printf '%s\n' "$DEVTEST_RUN" >"$out/run.txt"
	"$adb" push "$out/run.txt" "$files/run.txt" >/dev/null
fi
# drive runs the driver's commands the log asks for and has not run yet.
drive() {
	grep -E '^devtest: [0-9]+ [a-z0-9 -]+$' "$out/log" 2>/dev/null | while read -r _ n cmd; do
		[ -e "$out/driven-$n" ] && continue
		reply=
		if [ "$phone" = 1 ]; then
			case "$cmd" in
			wifi* | data* | airplane* | sleep | wake) reply="refused: real device" ;;
			esac
		fi
		[ -n "$reply" ] || case "$cmd" in
		top) reply=$("$adb" shell dumpsys activity activities </dev/null | grep -m1 -E 'topResumedActivity|mResumedActivity' | tr -d '\r') ;;
		back) "$adb" shell input keyevent 4 </dev/null && reply=ok ;;
		"wifi on") "$adb" shell svc wifi enable </dev/null && reply=ok ;;
		"wifi off") "$adb" shell svc wifi disable </dev/null && reply=ok ;;
		"data on") "$adb" shell svc data enable </dev/null && reply=ok ;;
		"data off") "$adb" shell svc data disable </dev/null && reply=ok ;;
		"airplane on") "$adb" shell cmd connectivity airplane-mode enable </dev/null && reply=ok ;;
		"airplane off") "$adb" shell cmd connectivity airplane-mode disable </dev/null && reply=ok ;;
		home) "$adb" shell input keyevent KEYCODE_HOME </dev/null && reply=ok ;;
		front) "$adb" shell am start -n "$appid/$activity" </dev/null >/dev/null && reply=ok ;;
		sleep) "$adb" shell input keyevent KEYCODE_SLEEP </dev/null && reply=ok ;;
		wake) "$adb" shell input keyevent KEYCODE_WAKEUP </dev/null && "$adb" shell wm dismiss-keyguard </dev/null && reply=ok ;;
		shot-*)
			mkdir -p "$root/local/p4/shots"
			"$adb" exec-out screencap -p >"$root/local/p4/shots/android-${cmd#shot-}.png" </dev/null &&
				"$adb" push "$root/local/p4/shots/android-${cmd#shot-}.png" "$files/${cmd}.png" </dev/null >/dev/null && reply=ok ;;
		*) reply="unknown command: $cmd" ;;
		esac
		printf '%s\n' "${reply:-none}" >"$out/reply"
		"$adb" push "$out/reply" "$files/devtest-$n.txt" </dev/null >/dev/null
		touch "$out/driven-$n"
	done
}
"$adb" shell am force-stop "$appid"
# A phone's log is the system's: it is read by the app's pid, never cleared.
[ "$phone" = 1 ] || "$adb" logcat -c
"$adb" shell am start -W -n "$appid/$activity" >/dev/null

# Only the app's own process counts: logcat is read with --pid, so another
# app's FAIL cannot end the wait. The verdict is the final PASS or FAIL line
# the tests print; the app ending without one, or restarting (a new pid), is a
# crash and fails at once.
pid=
for _ in $(seq 1 10); do
	pid=$("$adb" shell pidof -s "$appid" | tr -d '\r') || true
	[ -n "$pid" ] && break
	sleep 1
done
verdict=
crash=
if [ -z "$pid" ]; then
	# Gone before pidof saw it: ActivityManager logged its pid at the start.
	pid=$("$adb" logcat -d -s ActivityManager:I | sed -n "s/.*Start proc \([0-9][0-9]*\):$appid\/.*/\1/p" | tail -1)
	[ -n "$pid" ] || crash="the app did not start"
fi
if [ -z "$crash" ]; then
	first=$pid
	for _ in $(seq 1 ${DEVTEST_TIMEOUT:-300}); do
		"$adb" logcat -d --pid="$first" >"$out/raw" 2>/dev/null || true
		sed -n 's/^.* gdesk *: //p' "$out/raw" >"$out/log"
		drive
		if grep -qE '^(PASS|FAIL)$' "$out/log"; then verdict=1; break; fi
		if grep -qE 'FATAL EXCEPTION|Fatal signal|^panic:|runtime: |SIGSEGV' "$out/raw"; then crash="the app crashed (see log)"; break; fi
		now=$("$adb" shell pidof -s "$appid" | tr -d '\r') || true
		if [ -z "$now" ]; then
			# gone: the tests end the process after printing the verdict, so
			# read once more for the verdict before calling it a crash.
			"$adb" logcat -d --pid="$first" >"$out/raw" 2>/dev/null || true
			sed -n 's/^.* gdesk *: //p' "$out/raw" >"$out/log"
			if grep -qE '^(PASS|FAIL)$' "$out/log"; then verdict=1; else crash="the app exited without a verdict"; fi
			break
		fi
		if [ "$now" != "$first" ]; then crash="the app restarted (pid $first -> $now)"; break; fi
		sleep 1
	done
	[ -n "$verdict$crash" ] || crash="timeout: no verdict in ${DEVTEST_TIMEOUT:-300} s"
fi
if [ -n "$pid" ]; then
	"$adb" logcat -d --pid="$pid" >"$out/raw" 2>/dev/null || true
	sed -n 's/^.* gdesk *: //p' "$out/raw" >"$out/log"
	cat "$out/log"
	logs="$root/local/p4/logs"
	mkdir -p "$logs"
	cp "$out/log" "$logs/$(basename "$pkg")-android-$(date +%Y%m%d-%H%M%S).log"
fi
if [ -n "$crash" ]; then
	echo "FAIL: devtest-android: $crash" >&2
	grep -E 'FATAL EXCEPTION|Fatal signal|panic:|AndroidRuntime' "$out/raw" 2>/dev/null | head -20 >&2 || true
fi
"$adb" shell am force-stop "$appid"
"$adb" uninstall "$appid" >/dev/null
[ -z "$crash" ] && grep -q '^PASS$' "$out/log"
