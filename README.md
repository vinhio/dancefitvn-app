# DanceFITVN App

A learning app for DanceFITVN coaches, on **iPhone**, **Android** and **Apple TV**.
It is one Go codebase built on **gDesk**, a self-drawn UI framework: every
screen is a widget painted by gDesk, with no web view. The app matches the
prototype in `reference/prototype/index.html` (Night Stage, mobile and TV)
in layout, sizes, colours, text, behaviour and sequences, except for the
differences listed in `docs/plan/01-prototype-mapping.md` section 3 (K1–K29).

The UI strings are Vietnamese, copied from the prototype into
`locales/vi.json`. Code, comments and documentation are English.

The planning kit (phases P0–P8, decisions, the orchestration rules) is in
`docs/plan/`, in Vietnamese: start with `docs/plan/README.vi.md` (the kit's
own guide), then `docs/plan/PLAN.md` and `docs/plan/PROGRESS.md`. The open items for people outside the repository are
in `docs/BLOCKERS.md`.

## Status

Phases P0–P7 are done. P8 (finishing and trial release) is in progress: the
version is 1.0.0 (build 1), the store builds are **not signed** yet, and the
live API has not been checked against the real server. See the limitations
below and `docs/BLOCKERS.md`.

## Workspace

Four folders side by side, under one parent:

```
~/Working/
├── gdesk/                    gDesk main branch (not edited here)
├── gdesk-dancefitvn-app/     gDesk worktree on branch dancefitvn-app-support
├── dancefitvn-app/           this repository
└── jplayer-mobile/, jplayer-tv/   read-only sources of the playback engine and helpers
```

`go.mod` replaces the gDesk modules with paths in the worktree
(`../gdesk-dancefitvn-app`, created with `gdesk init -local`). Changes to gDesk
are made only in that worktree and only after the gDesk owner agrees; the
proposals are in `docs/BLOCKERS.md`. The module path is
`dancefitvn-app`; gDesk's is `github.com/gdeskdev/desk/<layer>/<module>`.

Prerequisites on the Mac: Go (see `go.mod`), Xcode with the iOS and tvOS
simulators, the Android SDK and NDK, OpenJDK 21, and the `gdesk` CLI
(`cd ../gdesk-dancefitvn-app && go install ./kits/cli/cmd/gdesk`). For the
Android tools:

```sh
export JAVA_HOME=/opt/homebrew/opt/openjdk@21/libexec/openjdk.jdk/Contents/Home
export ANDROID_HOME=/opt/homebrew/share/android-commandlinetools
```

## Build, run, test

```sh
go test -count=1 ./...                    # all tests (fake drivers; runs on the Mac)
go test -count=1 -race ./...
go test -count=1 -p 1 -tags tvos ./...        # Apple TV tests (ui/tv, allocation and benchmark tests)
go test -count=1 -tags dancefitvndev ./...    # developer build tag (dev menu and scenarios)
go test -count=1 -tags kitdemo ./...          # kit demo screens
go test -count=1 -tags live ./...             # live build tag (no mock markers)

gdesk build -os ios                       # dist/ios-simulator-arm64/DanceFITVN.app
gdesk build -os android                   # dist/android-arm64/DanceFITVN.apk
gdesk build -os tvos                      # dist/tvos-simulator-arm64/DanceFITVN.app

xcrun simctl boot "DanceFITVN iPhone 16"  # 393×852 @3x
xcrun simctl boot "DanceFITVN Apple TV"   # Apple TV 4K
emulator -avd dfv-412x892 -no-snapshot &  # 412×892 DIP, Android 35

go run ./cmd/contractcheck -self          # checks the contract tool against the mock
```

`go vet` for the Android target needs the NDK's android-26 clang (see
`CLAUDE.md` §6).

Tests use fake drivers and `ui.Tester`, with no `time.Sleep`. Screen tests
run at the sizes of `CLAUDE.md` §5: the two phone sizes (393×852 and 412×892)
and the TV size (1920×1080). The packages with a build tag (`ios`, `android`, `tvos`) have an
`_other.go` file that returns `core.ErrUnsupported`, so `go test ./...` also
runs on the Mac.

Build tags: `tvos` selects the Apple TV shell; `live` builds the store
version against the live API; `dancefitvndev` adds the developer hooks
(state selection, layout dumps, shots, perf marks); `kitdemo` builds the
widget test page. `-tags` goes after the platform tag, as in
`gdesk build -os ios -tags live`.

## Modes

The API layer (`internal/api`) has three transports:

| Mode | Build | Where the data comes from |
|---|---|---|
| **mock** | default (no tag) | `mock/dancefitvn-mock-api`, embedded in the build, with a media server on the loopback address and an overlay for edits |
| **live** | `-tags live` (the store build) | `api.DefaultBase` = `https://dancefitvn.com/mobile/api/v1`; no mock, no overlay, no scenarios |
| **hybrid** | developer build only | a live base from `DFV_API`, with only the routes named in `DFV_LIVE_ROUTES` going live; the rest stays on the mock |

`DFV_API` must be `http` only on the loopback address. The launch logs
`dancefitvn: API: mock|live|hybrid(n routes: …)`, never a URL or a token.
`DFV_LIVE_ROUTES` is checked by `api.CheckLiveRoutes`: an unknown route, or a
login route that would send a mock token to the server, is refused and the
mock answers instead.

To try a live build against the local mock server:

```sh
go run ./cmd/mockserve                    # 127.0.0.1:8787, with the pair relay
DFV_TAGS=live scripts/dev-build.sh ios    # the live build; then, at launch:
SIMCTL_CHILD_DFV_API=http://127.0.0.1:8787/api/v1 xcrun simctl launch … (Android: --es DFV_API)
```

## Scripts

Full details, and every variable, are in `scripts/README.md`. In short:

| Script | Purpose |
|---|---|
| `scripts/dev-build.sh ios\|android` | Developer build (`dancefitvndev` tag). The app reads its developer variables at launch (`DFV_STATE`, `DFV_OFFLINE`, `DFV_API`, …) |
| `scripts/shot.sh <IDs> ios\|android\|tester\|tvos\|tvtester [-compare]` | Screenshots and `layout.json` of each state, optionally compared with the prototype through `cmd/shotcmp` |
| `scripts/net.sh`, `scripts/a11y-dump.sh` | Network kind for a developer build; accessibility tree as VoiceOver receives it (simulators) |
| `scripts/perf-start.sh`, `perf-scroll.sh`, `perf-video.sh` | Launch time, list scrolling and a 10-minute video, on the simulator, a paired device or the emulator |
| `scripts/tvos-remote.sh`, `tvos-framestats.sh`, `tvos-memgate.sh`, `tvos-store-check.sh` | Apple TV: Siri Remote drive, focus frame times, memory gate, Mirror check |
| `scripts/check-livebuild.sh` | The live build contains no mock data, no developer marker and does contain `api.DefaultBase` |
| `scripts/release.sh` | Store builds (see below) |
| `scripts/release/check-ios.sh`, `check-android.sh`, `iosscan.py` | Checks of a built app or package: plist, privacy manifest, required-reason APIs, 16 KB alignment, manifest |
| `scripts/ios-faceid-plist.sh` | Adds `NSFaceIDUsageDescription` to an iOS build (until gDesk writes it) |
| `scripts/fonts/` | Font subsetting (P2) |

Device targets are named by environment variables, never by a value in the
repository: `DFV_IPHONE_UDID`, `DFV_TV_UDID`, `ANDROID_SERIAL` (required by
the Android perf and device steps) and `DFV_TEAM` (the Apple team that signs
the Apple TV UI-test bundle). The scripts stop with a message naming the
variable when it is unset.

## Commands (`cmd/`)

| Tool | Purpose |
|---|---|
| `cmd/contractcheck` | Compares a running API with the mock's response **structure** (keys and kinds, never values). `-self` checks the tool against the mock. Used with `-base` for the backend check, `-write` only on a test account |
| `cmd/mockserve` | Serves the mock on `127.0.0.1` for the live and hybrid modes, including the TV pairing relay. Loopback only |
| `cmd/shotcmp` | Compares screenshots and layouts with the prototype's captures (used by `scripts/shot.sh -compare`) |

## Release

`scripts/release.sh ios|tvos|android|all` builds the store artefacts from a
snapshot of the tree, with `-tags live`, into `dist/release/`, with
`SHA256SUMS` and one check report per artefact. `--sim ios|tvos` builds the
simulator version for local tests. `DFV_FLAVOUR=demo` builds the mock version
for internal testers. Run `scripts/release.sh doctor` to list what a signed
release still needs; it exits 1 while anything is missing. Nothing is
uploaded.

What blocks a signed release (details in `docs/BLOCKERS.md`):

- No Apple Distribution certificate and no App Store profiles for iOS and
  tvOS. Without them the iOS and tvOS packages are unsigned and check the
  layout only.
- No App Store Connect record or API key, no Play Console app and no upload
  keystore. Without a keystore the Android build uses gDesk's debug key.
- `bundletool` is not installed, so the Android `.aab` is not built and the
  download size is taken from the APK.
- The store texts in `docs/store/` and `docs/release.md` have placeholders
  (`TBD (DanceFITVN)`) for the privacy policy and support URLs, the data list,
  the child-audience answer and the App Review demo account.
- Icon and Apple TV brand assets are gDesk placeholders (the icon is flattened
  onto the app background, with no transparent pixels). The real art belongs
  to DanceFITVN.

## Devices used

The developer checks run on the iPhone simulator, the Apple TV 4K and 1080p
simulators, and, where connected, one iPhone, one Apple TV and one Android
phone, for install and test only. The emulator (412×892, Android 35) is the
fallback for the Android numbers and is labelled as such. Device steps never
change system settings (network, lock, Face ID, Wi-Fi).

## Known limitations

- **Android screen reader.** TalkBack does not read gDesk Android apps yet
  (`kits/mobile/README.md`). The accessibility work for VoiceOver is done
  (labels, roles, hidden controls, reading order). Proposal G10 would bridge
  it; without it, 1.0 ships without TalkBack support, while the media
  notification labels are in Vietnamese.
- **Face ID on iOS** needs `NSFaceIDUsageDescription`. gDesk does not write
  it yet (G3a); `scripts/release.sh` adds it before signing.
- **Android 16 KB pages.** The Play release is checked for 16 KB LOAD
  segments. gDesk's own link aligns to 4 KB with an older NDK, so
  `release.sh` passes `CCC_OVERRIDE_OPTIONS` (fallback F9) until G11 lands.
  A plain `gdesk build -os android` is still 4 KB.
- **Fallbacks in use** (gDesk proposals waiting, see `docs/BLOCKERS.md`):
  F1 secret store in a no-backup folder (G1: Keychain and Keystore; on tvOS
  the copy lives in `Caches`, so a purge signs the TV out); F2 photo picker
  through objc and JNI (G2); F4 network kind (G4); F5 and F5a downloads in
  the foreground and the iOS background task (G5a, G5b); F6 biometric
  sign-in (G3); F7 external links (G8); F8 keep-awake on Android (G9); F9
  16 KB alignment (G11). F3 is not needed.
- **Test coverage limits.** The live API has not been run against the real
  server. The Android device checks (network, photo picker, lock screen,
  emulator PIN) are still manual. The checklist for real devices is section
  6 of `docs/plan/07-testing.md`.
- **Known differences from the prototype** are listed as K1–K29 in
  `docs/plan/01-prototype-mapping.md` section 3. Some wait for the user's
  decision (K9a, K17–K29), listed in `docs/BLOCKERS.md`.

## Licences and third-party material

- Fonts: Inter, JetBrains Mono and Barlow Condensed, all under the SIL Open
  Font License 1.1 (`assets/fonts/OFL-*.txt`). They are subset to the
  characters the app uses.
- The gDesk framework and its tools are under gDesk's own licence (its
  repository). Files copied from jPlayer keep a header "Adapted from …" that
  names the source repository and commit.
- The app icon and the Apple TV brand assets are gDesk placeholders. No
  DanceFITVN artwork, video, music or logo is in the repository.
- The licence of this app's own code has not been chosen yet.

## Repository layout

```
main.go                    the app's entry point (live or mock, by build tag)
internal/app               wiring of the mobile and TV shells, states, developer hooks
internal/api               the API client: live, hybrid, mock transports and routes
internal/auth, store, progress, download, pairing, player, qr, format, model
internal/ui/               widgets and screens: kit, mobile/, tv/
internal/media/            playback engines (avf, amp, avftv) copied from jPlayer
internal/sys/              system services: secrets, biometrics, network, downloads, picker
mock/dancefitvn-mock-api/  the mock API data (JSON, the contract)
locales/vi.json            Vietnamese strings
assets/                    fonts, privacy manifests, Apple TV brand assets
scripts/, cmd/, e2e/       tools, see above
reference/prototype/       the prototype (the specification; not edited)
docs/                      the plan (Vietnamese), BLOCKERS, release and store notes
```

The API data contract is `mock/dancefitvn-mock-api/` and API v1; the
backend's own plan lives in the backend repository, not here.
