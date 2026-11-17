# e2e

End-to-end tests that run on the iOS simulator and the Apple TV simulator,
inside a real gDesk app (docs/plan/04-media-engine.md, 07 §1). Adapted from
jPlayer Mobile's `e2e/` (49de3c2) and jPlayer TV's `e2e/avfcheck` and
`scripts/tvos-avf-test.sh` (781bee5). Test files only, plus one small
package; on the Mac the device tests skip, so `go test ./...` stays green
without a device.

| Path | What |
|---|---|
| `engine/` | the iPhone engine (`internal/media/avf`): `TestMain` opens an app, the tests call the engine on the UI thread (HLS and MP3, speeds, seeks ±10, A–B loop, end, stale events, expired links, max resolution, mirror by screenshot, background hooks and the real background, session, media reset, `ReadTime`, 0 allocations per tick and per pull, no leak over loads) |
| `engine/` on Android | the Android engine (`internal/media/amp`, `*_android_test.go`, `measure_test.go` and `view_test.go` shared with the iPhone): HLS and MP3, speeds of the sheet (`PlaybackParams` read back, pitch 1), a long window of the HLS's speed, rate set paused, ±10 seeks paused, A–B, end, stale events, expired and revoked links, mirror by `adb screencap`, the paused start's frame, the state machine, 0 allocations per tick, `ReadTime`, volume, pause/seek and frame, the YUV conversion, audio focus and the noisy receiver, keep-up at 1× and 1.5× in a painting view, no leak over 20 switches, the background hooks, the Wi-Fi and hand-off locks; last (`zz_`) the real background through Home (the screen off on an emulator only) |
| `tvengine/` | the Apple TV engine (`internal/media/avftv`) with its native plane under a transparent view: the same groups, plus `Place`, the layer's mirror read back, and the UI-thread time of `Load` (`aa_loadtime_tvos_test.go`, first in the run) |
| `sys/` | the system services of `internal/sys` (P5.1): the network's kind (`nw_path_monitor`; on the emulator Wi-Fi, mobile data and airplane mode switched by the driver), `Cellular` 0 allocations on the UI thread, `FreeSpace` against `statfs`, the audio folder (made, out of backups, the folder of before moved in; tvOS: none). The photo picker (P6.3): present, a second one `core.ErrDuplicate`, cancel `ErrCanceled`; answers of the item provider (JPEG/PNG as they are, past 4096 px through ImageIO, upright); a pick by the user through the real PHPicker, tapped by XCUITest (`local/p6/pick-tap.sh`, `DFV_PICK_TAP`); tvOS `ErrUnsupported`. Runs on the iPhone simulator and the Android emulator; the Apple TV simulator takes it built with `-tags tvos` by hand (no script yet) |
| `e2emedia/` | the tests' media server: the mock's media through `internal/api/mock`'s `Server` on 127.0.0.1, and a gate in front of it that plays the CDN (signed links with `exp`, 403 once expired or revoked; a long HLS playlist of the mock's segments). Unit-tested on the Mac |
| `devtest-ios.sh [pkg] [-test.run X]` | builds the package's tests for the iPhone simulator, installs and runs them, prints the output, saves it in `local/p4/logs/`, exits non-zero unless `PASS` |
| `devtest-android.sh [pkg]` | the same on the Android emulator (`emulator -avd dfv-412x892 -no-snapshot`) or a phone (`ANDROID_SERIAL`): the package's tests in an APK with gDesk's Java, the output read from logcat; `DEVTEST_RUN=<pattern>` for `-test.run`. On a phone the driver changes no system setting (wifi, data, airplane, sleep, wake answer `refused: real device`; `phone.txt` in the media folder tells the tests), the log is never cleared, and the media folders are opened (`chmod -R a+rwX`: on API 36 the app cannot enter the folders `adb push` makes) |
| `devtest-iphone.sh [pkg] [-test.run X]`, `ios-device-sign.sh <App.app>` | the same on a paired iPhone (`DFV_IPHONE_UDID`, required): built for `iphoneos`, signed with the Apple Development identity and Xcode's wildcard team profile listing the phone, installed and run by `devicectl`, uninstalled at the end (`DFV_KEEP=1` keeps it). See `scripts/README.md`, "Devices" |
| `prepare-media.sh <dir>` | copies the mock's lesson `l_3503` (HLS, 12 s) and `audio/_sample.mp3` (30 s) into the bundle (both scripts call it) |
| `../scripts/tvos-avf-test.sh unit\|e2e\|vet` | the Apple TV runner: `e2e` runs `tvengine/`, `unit` runs `internal/media/avftv`'s own tests on the simulator |

## Running

```sh
e2e/devtest-ios.sh                                  # simulator "DanceFITVN iPhone 16" (GDESK_SIMULATOR to change)
e2e/devtest-ios.sh ./e2e/engine -test.run TestAVFRates
e2e/devtest-android.sh ./e2e/engine                 # emulator, or a phone: ANDROID_SERIAL=<serial>
DEVTEST_RUN=TestAMPRates e2e/devtest-android.sh ./e2e/engine
scripts/tvos-avf-test.sh e2e                        # simulator "DanceFITVN Apple TV"
scripts/tvos-avf-test.sh e2e "DanceFITVN Apple TV" -test.run TestTVRates
```

No media comes from the network and none is real DanceFITVN content: the
tests play what `prepare-media.sh` copied from `mock/`, served by the test
app on a loopback port of its own.

## The driver

The tests ask for what only the Mac can do by printing a line the script
reads as it runs:

- iPhone (`engine/driver_test.go`): `devtest: <n> <cmd>`, answered in
  `devtest-<n>.txt` of the app's tmp folder; `home` (Settings in front: the
  app goes to the background), `front`, `shot-NAME` (a screenshot into
  `local/p4/shots/ios-NAME.png`, copied to the app's tmp).
- Android (`sys/platform_android_test.go`, `engine/driver_test.go`):
  `devtest: <n> <cmd>`, answered in `devtest-<n>.txt` of the app's
  external files folder; `top`, `back`, `wifi on|off`, `data on|off`,
  `airplane on|off`, `home`, `front`, `sleep`, `wake` (the last five
  refused on a phone), `shot-NAME` (`adb exec-out screencap` into
  `local/p4/shots/android-NAME.png`, pushed back as `shot-NAME.png`).
- Apple TV (`tvengine/main_tvos_test.go`): `avf-test: <request>`, answered
  by the file `avf-ack-<request>`; `screenshot NAME`, `background` (TV
  Settings in front), `foreground`.

## Writing a test

- `newHarness(t)`: the engine on the platform, its events in a channel,
  closed at cleanup; `h.load`, `h.await`, `h.awaitOr`, `h.expectNone`,
  `h.drain`, `h.readTime`, `h.tickTime`, `state(s)`, `kind(k)`,
  `mediaURL(name)`.
- Tests are not parallel: one app a process. Waits are on the wall clock
  (the engines play in real time); a file named `zz_…` runs last (the real
  background), `aa_…` first (the first `Load` of the process).

| Path | What |
|---|---|
| `remote/` | `RemoteUITests.swift`, the XCUIRemote UI test `scripts/tvos-remote.sh` builds (a UI-test bundle with no host app) and runs against the app on the Apple TV simulator (adapted from jplayer-tv `e2e/remote` at 781bee5) |
| `tvstore/` | the small app of `scripts/tvos-store-check.sh run`: the Mirror of NSUserDefaults and files across relaunch, purge, spoilt slots, reboot, install over and reinstall |
