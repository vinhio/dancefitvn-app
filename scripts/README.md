# scripts

Helper scripts for builds and shots of the mobile app. Adapted from
`jplayer-mobile/scripts` at 49de3c2 (`dev-build.sh`, `mobile-shots.sh`).

- `dev-build.sh ios|android [gdesk build flags]`: a developer's build (the
  `dancefitvndev` tag), the only one that reads the variables of a script:
  `DFV_STATE` (a state of `internal/app/states_mobile.go`), `DFV_SETTLE`
  (ms of rest before the layout is written, 1500), `DFV_OFFLINE=1` (every
  request fails as a dead network does), `DFV_SHOT_LOG=1` (the layout is
  also logged as base64 lines). iOS reads them from the environment
  (`SIMCTL_CHILD_DFV_STATE=M02 xcrun simctl launch …`), Android from the
  launch intent's string extras (`am start … --es DFV_STATE M02`). An iOS
  build then runs `ios-faceid-plist.sh` (gdesk build already writes
  `NSFaceIDUsageDescription` from gdesk.json, G3a). Plain `gdesk build`
  ships without the variables. `DFV_TAGS=a,b` adds tags. The downloads' variables are
  below (`net.sh`).
- `shot.sh <ID[,ID…]> ios|android|tester [-compare]`: draws states and
  writes `<ID>.png` and `<ID>.layout.json` (docs/plan/07-testing.md §3.3):
  - `ios`: builds with `dev-build.sh`, installs on the simulator
    "DanceFITVN iPhone 16", pins its status bar to 9:41, launches each state
    and waits for the app to write
    `Library/Caches/com.jivecode.dancefitvn/dfv-shots/<ID>.layout.json` in
    its data container (`simctl get_app_container … data`), then
    `simctl io screenshot`. Output: `local/shots/app/ios/`.
  - `android`: the same on the emulator (`ANDROID_SERIAL`, default
    `emulator-5554`; `ANDROID_HOME` must name the SDK): demo-mode status bar
    at 9:41, `am start --es DFV_STATE <ID>`, the path of the layout from
    logcat (`dfv: shot ready <ID> <path>`), read with `adb root`; without
    root, from the base64 lines (`DFV_SHOT_LOG`). Output:
    `local/shots/app/android/`.
  - `tester`: in process on both phones (Core Text for iOS, gotext for
    Android): `go test ./internal/app -run TestShots`. Output:
    `local/shots/app-tester/{ios,android}/`.
  - `-compare`: `cmd/shotcmp` against `local/shots/prototype-kit/mobile`;
    iOS at 3x against `mobile@3x` (layout and full screen, the masks of
    local/plans/P3.md §4), Android on the layout (stretch rule). Reports
    `local/shots/app-report-<mode>.md`, diff pictures
    `local/shots/diff-app/<mode>/`; the exit status is shotcmp's.
  - `DFV_NOBUILD=1` skips the build and the install.

  Every state is reached through the real flow over the mock (D7): a
  sign-in, a logout keeping the biometric copy, a scenario of the mock, a
  route held. Pixel and layout states: `M01 M02 M02B M03B M03 M04 M05
  M05B`. A developer's: `M02-locked M02-device M02-offline M02-expired
  M03-offline M04-offline UPGRADE`, and `LOGIN` (types the mock's
  credentials and taps "Đăng nhập" through the window's touch events, to
  Home) and `FORGOT` (taps "Quên mật khẩu?": the system's browser opens).
- `net.sh ios|android none|wifi|cellular|auto`: the network a developer's
  build sees (P5.1c), the Mac's own untouched. iOS writes the kind into
  `Library/Application Support/com.jivecode.dancefitvn/dev/network` of the
  app's data container (`simctl get_app_container`; `DFV_SIM`, default
  `booted`), which the dev build reads every second (the simulator has no
  airplane mode); `auto` gives the system's kind back. Android uses the
  emulator's switches: `none` = airplane mode, `cellular` = Wi-Fi off and
  mobile data on, `wifi`/`auto` = both on.
- The downloads' variables of a developer's build (P5.1c):
  `DFV_NETWORK=none|wifi|cellular|auto` forces the kind of network from
  the launch (the file of `net.sh` wins while it exists),
  `DFV_DL_KBPS=<n>` reads the downloads at n KiB/s (64: a lesson's 481 KB
  in 7.5 s, so that a cut lands mid-file), `DFV_BADSHA=1` makes the API's
  sha256 of every audio file wrong (the size kept: each download ends in
  "Tải lỗi, thử lại"), `DFV_DL_LESSON=<n>` the lesson of Set 35 the
  download smokes take (5). Smoke states: `SMOKE-DL` (confirms the
  download on the detail of Set 35, logs every event, toast, network and
  online change as `dancefitvn: smoke: DL …`), `SMOKE-M11-RUNNING` (the
  same, then screen 11), `SMOKE-OFFLINE` (waits for screen 13, then plays
  the downloaded file). Shot states of P5: `M09 M11 M12 M13 M14`.
- `ios-faceid-plist.sh [--nosign] <App.app> [sentence]`: writes
  `NSFaceIDUsageDescription` (the key `bio.faceIdUsage` of
  `locales/vi.json`, or `$DFV_FACEID_USAGE`) into an iOS build and signs it
  again (F6). Since gDesk's G3a gdesk build writes the same key from
  gdesk.json's `ios.privacy.faceID`, so the script only sets it again, a
  safety net for an older gdesk; `--nosign` leaves the signature to
  `release.sh`.
- `check-livebuild.sh [ios] [tvos] [android]` (adapted from
  `jplayer-mobile/scripts/check-storebuild.sh` at 49de3c2): the live build
  (`gdesk build -os <sys> -tags live`, the build that ships, built from a
  snapshot so `dist/` is left alone) links no `internal/api/mock` (`go list
  -deps`), and its program holds none of the mock's or a developer's
  markers (the mock's JSON and media, `mock-media://`, the overlay, the shot
  states, the shot writer, `DFV_*`) and does hold `api.DefaultBase`. A
  developer's Mac build is the control (a marker it lacks is stale).
  `--check <app|ipa|apk|aab>…` scans builds made elsewhere;
  `DFV_LIVEBUILD_KEEP=<dir>` keeps the builds.
- The API of a build (`internal/app/apimode.go`): the mock in the process
  by default; `-tags live` talks to `api.DefaultBase` and reads no
  variable. A developer's build: `DFV_API=<base>` (http on the loopback
  only, e.g. `go run ./cmd/mockserve -addr 127.0.0.1:0`) sends everything
  there; with `DFV_LIVE_ROUTES=app.config,auth.login,auth.refresh,me.home`
  only those routes (the hybrid; a list `api.CheckLiveRoutes` refuses is
  logged and the mock answers all). `DFV_TAGS=live scripts/dev-build.sh
  ios|tvos` is the live build with `DFV_API` replacing its base: a state
  that needs a scenario of the mock stops (`dfv: state <ID> stopped`). The
  launch logs `dancefitvn: API: mock|live|hybrid(n routes: …)`, never a
  URL. Android reads both from the launch extras (`--es DFV_API …`; the
  emulator needs `adb reverse`).
- `fonts/`: the font subsetting tools (P2).
- `gen-nfc.py`: writes `internal/format/nfc_table.go` (the composition table of `format.NFC`) and its test vectors `internal/format/testdata/nfc.txt` from Python's `unicodedata`.

## Apple TV

- `tvos-remote.sh "<steps>" [tag]`: drives the developer's tvOS build on
  "DanceFITVN Apple TV" with the Siri Remote through XCUIRemote
  (`e2e/remote/RemoteUITests.swift`, adapted from jplayer-tv at 781bee5):
  `up down left right select menu playpause home`, `<button>@<ms>`,
  `hold:<ms>`, `wait:<ms>`, `since:<ms>`, `shot:<name>`, `front`,
  `state:<name>`, `mark:<text>`. Every `DFV_*` variable but the script's own
  goes to the app (`DFV_STATE`, `DFV_ENGINE=real`, `DFV_FOCUSTRACE=1`,
  `DFV_API`, `DFV_FRAMELOG=1`, …), and every `GDESK_*` one
  (`GDESK_FRAMESTATS=1`); with `DFV_STATE` the steps wait for `dfv: shot ready`.
  Outputs in `DFV_REMOTE_OUT` (default `local/p7/remote`): the app's
  console, the markers, the system's log, the shots.
- `shot.sh <IDs> tvos|tvtester [-compare]`: the TV's states
  (`internal/app/states_tv.go`); `tvos -compare` shoots on the 1080p
  simulator "DanceFITVN Apple TV 1080p" (`DFV_TVSIM1080` names another;
  `DFV_TVSIM` overrides both),
  and `cmd/shotcmp` runs with the masks of local/plans/P7.md P7.7 (QR,
  expiry line, `c1`/`c2`, TV12's photo, medals and version line, the plane,
  the avatar) and the waivers of `local/shots/app-exceptions-tv.txt`.
  TV10 holds its countdown at 5 and TV-toast its toast while the shot is
  taken.
- `tvos-memgate.sh [tag]`: the memory gate R1 (P7.5c) on the 4K simulator:
  `DFV_STATE=TV01-confirm`, 20 toasts on TV01, pairing, 10 TV05 ↔ TV06
  trips, 20 panel open/close on TV07, the footprint sampled every second
  (`footprint`) and the Go heap from the frame log; growth over the three
  phases ≤ 10 MB. `DFV_MEM_TOASTS`, `DFV_MEM_TRIPS`, `DFV_MEM_PANELS`,
  `DFV_MEM_OUT` (default `local/p7/p75c/part2`).
- `tvos-framestats.sh [tag]`: the frames of focus moves on TV05 at 4K
  (`DFV_FRAMELOG=1 GDESK_FRAMESTATS=1`, a snake through the grid and back):
  p50/p95/max, gate p95 ≤ 16.7 ms.
- A developer's build with `DFV_FRAMELOG=1` (and `GDESK_FRAMESTATS=1`, which
  times the frames) writes on the console `fs <unix ms> <n> <layout µs>
  <paint µs> <present µs> <total µs> <gpu> <unchanged> <pixels>` per frame,
  `key <name> down|repeat t=<unix ms>` per press and a `mem` line of the Go
  runtime a second (`internal/app/framelog.go`, from jplayer-tv).
- `tvos-store-check.sh run|app|test|vet`: `run` checks the Mirror of
  NSUserDefaults with the small test app of `e2e/tvstore`; `app` checks the
  app itself (preferences through relaunch, install over, a purge of
  `Library/Caches` and a reinstall; the purge signs the TV out, R-P7-7).
- The tvOS app logs `dancefitvn: app <state>` at each lifecycle change and
  `dancefitvn: first screen <route>`; a developer's build also `dfv: keep-awake …
  idleTimerDisabled=…` (UIApplication read back), `dfv: pair start: …
  device name …` and `dfv: prefs …` (`DFV_PREFS=quality=720p,autonext=off`
  sets them as TV12 does).

## Accessibility and performance (P8.3d, P8.4a)

These run a developer's build (`dev-build.sh`) through
`scripts/lib/target.sh`, which picks the target: the simulator by
default, the iPhone of `DFV_IPHONE_UDID` or the Apple TV of `DFV_TV_UDID`
when set (built `-device`, signed with `e2e/ios-device-sign.sh` or
`tvos-device-sign.sh`, installed and launched with `devicectl`), and on
Android the emulator or phone of `ANDROID_SERIAL` (required, no default).
On a device nothing of the system is changed and only
`com.jivecode.dancefitvn` is installed, launched and stopped; it must be
awake and unlocked. On a simulator the perf scripts turn its
accessibility runtime off for the run and put it back
(`ApplicationAccessibilityEnabled` of the simulator's own defaults: loaded,
it adds about 2 s to a launch). `DFV_NOBUILD=1` installs `dist/` as it
is; outputs go to `DFV_PERF_OUT` (default `local/p8/perf`).

- `a11y-dump.sh ios|tvos [ID…]` (adapted from
  `jplayer-mobile/scripts/m11-a11y.sh` at 49de3c2): simulators only. Each
  state is launched with `DFV_A11Y=1`: once its layout is written the app
  writes `<ID>.a11y.json` beside it, the content's tree and the elements
  the uikit bridge hands VoiceOver (label, value, traits, frame;
  `internal/sys.AccessibilityDump`, `internal/app/devaccess.go`), then
  presses the keys of `DFV_A11Y_MOVE` (`right,down`, `select`, `menu`)
  with a dump after each (`<ID>-m1`, …). The simulator's
  `ApplicationAccessibilityEnabled` is on for the run (UIKit computes the
  frames only then) and put back. Defaults: M03 M05 M06 M06-hidden M07 M15;
  TV04 TV06 TV07 TV08 with their moves. Checks (`report.md`): every element
  has a label and a trait; nothing of M06 is read in M07 or M06-hidden (but
  the stage) nor of TV07 in TV08; on the TV the focused node's element
  carries the selected trait before and after each move. Output
  `local/p8/a11y/<sys>/` (`DFV_A11Y_OUT`); exit 1 on a failed check.
- A developer's build logs `dancefitvn: mark home <ms> t=<unix ms>
  from=process` once, after the first frame that shows Home with its data
  (TV04 on the TV): ms since the system started the process
  (`internal/sys.ProcessStart`: sysctl on iOS and tvOS, `/proc` on
  Android; `from=init` counts from the app package's start where it cannot
  be read). The build that ships has none of it.
- `perf-start.sh ios|android|tvos [N]`: signs in once (`DFV_STATE=M03`,
  TV04), then N cold launches (`DFV_PERF_N`, 10) with no variable, each to
  its mark: median and p90 of process start → Home, the launch command →
  Home on the Mac's clock (simulators, emulator) and Android's `am start
  -W` TotalTime. Gate 1.5 s (iOS, tvOS), 2.5 s (Android).
- `perf-scroll.sh ios|android [courses|detail|both]`: `DFV_STATE=PERF-SCROLL`
  (Courses) and `PERF-SCROLL-M05` (the detail of Set 35) fling the list up
  and down `DFV_PERF_FLINGS` times (20) with synthetic touches timed as a
  finger's, the frame log on (`DFV_FRAMELOG=1 GDESK_FRAMESTATS=1`),
  analysed by `scripts/lib/perf_scroll.py`: per list the glides and the
  gaps from every frame drawn, unchanged ones included (a fling's crawl
  below a device pixel a frame presents nothing, on time; a gap is two
  drawn frames over 32 ms apart or a jump of the frame count), the glide
  rate and the work over 16.7 and 32 ms from the changed frames. On
  Android the first frame after a pause over 200 ms is reported apart,
  out of the work gate (no touch is injected: no input boost, a cold
  CPU). Gate: no gap and no work over 32 ms, ≥ 58 fps.
  `perf-scroll.sh analyse <label> <list>=<log>...` re-analyses logs
  already taken.
- `perf-video.sh ios|android|tvos [minutes]`: `DFV_STATE=PERF-VIDEO10`
  plays the video of Set 35 on the system's engine in an A–B loop over the
  whole media for `minutes` (`DFV_PERF_MIN`, 10), logging the player every
  30 s and a `mem` line a second (`DFV_FRAMELOG=1`); the process's memory
  is sampled every `DFV_PERF_EVERY` s (simulator: `footprint`, 5 s;
  Android: `dumpsys meminfo` total PSS, 30 s; an iPhone or Apple TV:
  xctrace's Activity Monitor, `scripts/lib/xctrace_footprint.py`). Growth =
  last − first after 30 s of play; gate ≤ 10 MB.
- TV focus frames stay with `tvos-framestats.sh`; the R1 memory gate with
  `tvos-memgate.sh`.

## Devices

The scripts that reach a paired iPhone or Apple TV name it by UDID from
the environment, with no default (no device or team of anyone is written
in the repository); each stops with a message naming the variable when it
is unset:

- `DFV_IPHONE_UDID`: the iPhone (`e2e/devtest-iphone.sh`,
  `e2e/ios-device-sign.sh`; set, it turns `perf-start.sh`,
  `perf-scroll.sh` and `perf-video.sh ios` from the simulator to the
  iPhone).
- `DFV_TV_UDID`: the Apple TV (`tvos-device-sign.sh`,
  `tvos-remote-device.sh`, `tvos-avf-test.sh device-unit|device-e2e`).
  Set, it also turns `tvos-memgate.sh`, `tvos-framestats.sh`,
  `perf-start.sh tvos` and `perf-video.sh tvos` from the simulator to the
  Apple TV: unset it (or `DFV_TV_UDID=`) for the simulator runs
  (`a11y-dump.sh` never reads it).
- `ANDROID_SERIAL`: the emulator or phone of the `perf-*.sh android` runs
  (required by them).
- `DFV_TEAM`: the Apple developer team ID that signs the UI-test bundle of
  `tvos-remote-device.sh` (and the runs above that go through it).

`xcrun devicectl list devices` lists the UDIDs, Xcode › Settings ›
Accounts the team. These variables are never passed on to the app.

`tvos-remote-device.sh` builds its UI-test runner with `xcodebuild
-allowProvisioningUpdates`: that may register the Apple TV with the team
and create or renew the team's development profile in the developer
account (a change to the account, not to the device). Nothing of the
release path uses it.

## Release (P8.2)

- `release.sh ios|tvos|android|all` (adapted from
  `jplayer-mobile/scripts/release.sh` at 49de3c2 and
  `jplayer-tv/scripts/tvos-release.sh` at 781bee5): the store builds, `-tags
  live` (the live API, no mock), made from a snapshot of the tree in
  `local/release/src` whose `go.mod` names gDesk by a relative path (no
  home folder in the programs). Artefacts in `dist/release/` with a line
  each in `SHA256SUMS`, the output of every check (`<artefact>.check.txt`)
  and what each build comes from (`<base>-<sys>.provenance.txt`):
  - iOS, tvOS: `gdesk build -device -release`; iOS then
    `ios-faceid-plist.sh --nosign`, which sets the key gdesk build already
    wrote from gdesk.json (G3a). With gdesk.json's
    `signing.iosIdentity` and `signing.iosProfile`/`tvosProfile` the app is
    signed as `gdesk package` signs (profile embedded, its entitlements) into
    `DanceFITVN-<version>-<build>[-tvos].ipa`; without them
    `…-unsigned.ipa` (layout check only). The dSYM zipped beside it.
  - Android: `gdesk build -os android -release`: the APK, re-signed with
    `signing.androidKeystore` when its password variable is set, else
    `…-debugkey.apk` (testers); the native debug symbols beside it. With
    bundletool (`BUNDLETOOL` or on the PATH) also `gdesk package` → `.aab`;
    bundletool is not downloaded (Q-P8-11), so the sizes are the APK's.
    The release asks clang for 16 KB LOAD segments
    (`CCC_OVERRIDE_OPTIONS`, `-z max-page-size=16384`): gdesk's own link
    gives 4 KB, which Play refuses for Android 15+ targets, until gDesk
    passes the flag.
  - Each artefact passes `release/check-ios.sh` or
    `release/check-android.sh`, then `check-livebuild.sh --check`; a FAIL
    stops the release.
  - `--sim ios|tvos`: the simulator's `-release -tags live` build, into
    `local/release/sim/<sys>/`, to install on the simulators.
  - `doctor`: what a signed release still needs (Apple Distribution
    identity, App Store profiles, upload keystore and its password,
    bundletool, App Store Connect API key); exit 1 while something is
    missing.
  - `DFV_FLAVOUR=demo`: the mock's app for internal testers
    (`DanceFITVN-demo-…`, no live check).
  Nothing is uploaded: App Store Connect, TestFlight and Play Console are
  the account holder's.
- `release/check-ios.sh <app|ipa>` (adapted from jplayer-mobile at
  49de3c2): Info.plist (id, version, build, `vi`, minimum 17.0, encryption
  answer, background audio, DT keys, ATS holding `NSAllowsLocalNetworking`
  alone; iOS: icon names, iPad orientations, `NSFaceIDUsageDescription`;
  tvOS: device family 3, the App Icon stack and Top Shelf images),
  Assets.car, the privacy manifest (the repo's, valid collected data,
  every required reason API the program imports or names declared:
  `release/iosscan.py`), the program (arm64, device platform, minos,
  SDK, no `/Users/`, system libraries only, `go version -m` tags, tvOS's
  prohibited functions), the ObjC selectors against the SDK headers, the
  signature, `DSYM=` UUID, sizes.
- `release/check-android.sh <apk|aab> [--install [serial…]]` (adapted
  from jplayer-mobile at 49de3c2): the manifest (package, version 1000102,
  min 26, target 36, exactly the permissions, not debuggable, cleartext to
  127.0.0.1 alone, Back, portrait, media service), arm64-v8a only, 16 KB
  LOAD alignment and zipalign, signer, sizes (APK download estimate from
  `apkanalyzer`). `--install` puts the APK on devices with `adb install
  -r`, starts it, checks it runs 5 s with no crash since the start (the
  log is never cleared), screenshots into `CHECK_ANDROID_SHOTS`, and
  uninstalls it (`DFV_KEEP=1` keeps it).

Note on API bases: the live build uses `api.DefaultBase`
(`https://dancefitvn.com/mobile/api/v1`). The `http://127.0.0.1:8787/api/v1`
examples above are for `cmd/mockserve`. Against the real backend running
locally the base is `http://127.0.0.1:7790/mobile/api/v1`.
