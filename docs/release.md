# Releasing DanceFITVN (App Store, TestFlight, Google Play)

*Adapted from jplayer-tv/docs/release.md at 781bee5 (structure, the "What you need" and "Export compliance" sections) and jplayer-mobile/scripts/release.sh at 49de3c2. The scripts are DanceFITVN's own: `scripts/release.sh`.*

How to make the store builds of DanceFITVN, check them, and what the account holder still has to do. Run every command from the repository root. Nothing here uploads anything: App Store Connect, TestFlight and Play Console are the account holder's steps.

Status (2026-11-15): the scripts are written and the unsigned and simulator builds run on the development Mac. **No signed artefact exists yet.** Every step that needs the Apple or Google account is unverified until the first real run. Items marked **TBD (DanceFITVN)** need a decision or a file from DanceFITVN; the blocked items are listed in the last section and in `docs/BLOCKERS.md`.

## Commands

All of them are in `scripts/release.sh` (run `scripts/release.sh -h` for the full text).

```sh
scripts/release.sh ios            # iPhone: gdesk build -os ios -device -release -tags live → .ipa
scripts/release.sh tvos           # Apple TV: gdesk build -os tvos -device -release -tags live → .ipa
scripts/release.sh android        # gdesk build -os android -release -tags live → APK (+ .aab with bundletool)
scripts/release.sh all            # ios, tvos and android
scripts/release.sh --sim ios      # simulator's -release -tags live build, copied to local/release/sim/ios/ (not shipped)
scripts/release.sh --sim tvos     # same for the Apple TV simulator
scripts/release.sh doctor         # lists what a signed release still needs (exit 1 while anything is missing)
scripts/check-livebuild.sh --check <app|ipa|apk|aab|binary>...   # no mock, no developer markers, live API base present
scripts/check-livebuild.sh        # the same check; it builds a fresh snapshot of the working tree (tag live) per platform, not the last build
```

What each device build does:

- **iOS** (`ios`): builds with `-device -release -tags live`. Then `scripts/ios-faceid-plist.sh --nosign` writes `NSFaceIDUsageDescription` (gDesk G3a is pending, and `gdesk package` would ship without it, so Face ID would crash). If `gdesk.json` holds `signing.iosIdentity` and `signing.iosProfile`, the script signs the app as `gdesk package` does (the profile embedded, its entitlements, codesign with the identity) and writes `dist/release/DanceFITVN-<version>-<build>.ipa`. Without them it writes `DanceFITVN-<version>-<build>-unsigned.ipa`, which checks the layout only; App Store Connect refuses it.
- **tvOS** (`tvos`): the same, without the Face ID step, with `signing.tvosProfile`. The artefact name carries `-tvos`.
- **Android** (`android`): an APK, re-signed with `signing.androidKeystore` (apksigner) when set and its password variable (`signing.androidPasswordEnv`) holds the password. Without them it is signed with gDesk's debug key and named `-debugkey.apk` (testers only). With `bundletool` (the `BUNDLETOOL` variable or on the `PATH`) it also writes the `.aab` that Play takes. Without bundletool no `.aab` is made and the sizes are the APK's (Q-P8-11: bundletool is not downloaded by the script).
- **Simulator** (`--sim`): the simulator build with the Face ID key, ad-hoc signed, for installing on the simulators to see that the shipped build starts. Not for upload.

Every build is made from a snapshot of the working tree in `local/release/src` (so `dist/` and other builds are left alone), and its `go.mod` uses a relative path for gDesk (so the home folder is not in the binary). Each artefact is checked before the script finishes:

- `scripts/release/check-ios.sh` (iOS and tvOS) or `scripts/release/check-android.sh` (Android): the privacy manifest, the binary's API use, the signing, the 16 KB page alignment on Android.
- `scripts/check-livebuild.sh --check`: no mock package, no mock JSON or media, no `mock-media://` scheme, no developer variable or shot state.

A failed check stops the release. An unsigned IPA or a debug key is a warning, not a failure.

### Outputs

All in `dist/release/` (`DFV_RELEASE_DIR` changes it):

| File | What |
|---|---|
| `DanceFITVN-<version>-<build>.ipa` (or `-unsigned.ipa`) | iPhone build |
| `DanceFITVN-<version>-<build>-tvos.ipa` (or `-unsigned.ipa`) | Apple TV build |
| `DanceFITVN-<version>-<build>.apk`, `-debugkey.apk` | Android build |
| `DanceFITVN-<version>-<build>.aab` | Android bundle for Play (only with bundletool) |
| `<artefact>.check.txt` | output of each check |
| `<base>-<sys>.provenance.txt` | what the build comes from (commit, tags, versions) |
| `SHA256SUMS` | one line per artefact |
| dSYM zip | the native debug symbols, beside each device build |

Names are from the script's header; **check the first real output** (TBD: exact names after the first run).

### Demo flavour (internal testers only)

```sh
DFV_FLAVOUR=demo scripts/release.sh ios
```

Builds the mock app (no `-tags live`: the mock and its sample media are inside), named `DanceFITVN-demo-…`. `check-livebuild.sh` is not run on it. It is not for the App Store or Play. Q-P8-22.

## Signing prerequisites (account holder)

Nothing secret goes in the repository. The certificate's private key stays in the login keychain; the profiles and the API key live in gitignored or home folders.

1. **Apple Distribution certificate** in the login keychain of the build Mac, under the team (team identifier: TBD (DanceFITVN)). Check: `security find-identity -v -p codesigning` lists `Apple Distribution: <name> (<TEAM>)`. It must exist before packaging: `gdesk package` signs with it directly.
2. **Explicit App ID** `com.jivecode.dancefitvn` (done 2026-11-15, BLOCKERS P8.2). No wildcard.
3. **App Store provisioning profiles**, two of them, both type App Store Connect, for the App ID above and the certificate in step 1:
   - iOS → `local/…/appstore-ios.mobileprovision` (path TBD)
   - tvOS → `local/…/appstore-tvos.mobileprovision` (path TBD)
   The profile's entitlements are copied verbatim into the signature; a wildcard profile would sign `<TEAM>.*`.
4. **`gdesk.json` signing block**, once steps 1 and 3 exist. The block is absent today; `scripts/release.sh` reads it:

   ```json
   "signing": {
     "iosIdentity": "Apple Distribution: <name> (<TEAM>)",
     "iosProfile": "local/…/appstore-ios.mobileprovision",
     "tvosProfile": "local/…/appstore-tvos.mobileprovision",
     "androidKeystore": "…",
     "androidAlias": "…",
     "androidPasswordEnv": "DFV_ANDROID_PASSWORD"
   }
   ```

   The identity's common name is not a secret. The profile and keystore paths are under `local/` (gitignored). **TBD (DanceFITVN):** the team identifier, and whether the same certificate signs tvOS (it does for gDesk, `iosIdentity` covers tvOS too).
5. **App Store Connect app record**: New App; platforms iOS and tvOS; name "DanceFITVN" (availability of the name TBD); primary language Vietnamese; bundle ID `com.jivecode.dancefitvn`; SKU TBD (DanceFITVN).
6. **App Store Connect API key** (Users and Access → Integrations → App Store Connect API, role App Manager), or the Transporter app with the Apple ID. Keep the `.p8` out of the repository:

   ```sh
   mkdir -p ~/.appstoreconnect/private_keys
   mv ~/Downloads/AuthKey_<KEYID>.p8 ~/.appstoreconnect/private_keys/
   export ASC_KEY_ID=<KEYID> ASC_ISSUER_ID=<issuer UUID>
   ```

   `ASC_KEY_FILE` can point to the file instead. `doctor` fails if a `.p8` is inside the repository.
7. **Google Play**: a Play Console developer account (TBD (DanceFITVN): personal or organisation); the app record with package name `com.jivecode.dancefitvn`; the **upload keystore** (created once, kept safe; **TBD (DanceFITVN):** confirm Play App Signing enrolment, which holds the app's signing key); the keystore password in the environment variable named by `signing.androidPasswordEnv`. Never commit the keystore or its password.
8. **bundletool** (optional for the `.aab`): the official release jar from github.com/google/bundletool, with its sha256 recorded in `local/tools/`. Refused → BLOCKERS; the sizes then come from the APK (Q-P8-11).
9. **Toolchain**: Xcode with the iOS and tvOS SDKs; the gDesk CLI of `../gdesk-dancefitvn-app` installed (`cd ../gdesk-dancefitvn-app && go install ./kits/cli/cmd/gdesk`); for Android, `JAVA_HOME` and `ANDROID_HOME` as in CLAUDE.md §6.

Check what is still missing at any time:

```sh
scripts/release.sh doctor
```

It prints one line per item and exits 1 while anything is missing: Xcode and SDKs, `gdesk`, `gdesk.json`'s version, build and icon, the Apple Distribution identity, the App Store profiles, the Android upload keystore and its password, bundletool, the App Store Connect API key.

## Version and build

- `gdesk.json` `version` (`CFBundleShortVersionString`, the user-visible version) and `build` (`CFBundleVersion`, the build number). Currently `1.0.0` and `1`.
- App Store Connect takes each build number of a version once: **bump `build` before every upload** (gDesk allows 1–99 per version). Android's `versionCode` is `code*100+build` (gDesk).
- **TBD (gDesk owner / developer, not DanceFITVN):** whether `gdesk package` refuses a version and build already packaged (jPlayer TV's script did, with `--force`). `scripts/release.sh` does not check it, so compare `dist/release/SHA256SUMS` with earlier runs by hand.
- Do not change `version` for a re-upload of the same build; bump `build`.
- **TBD (DanceFITVN):** the version of the first public release.

## Checks before each upload

1. `scripts/release.sh ios` (and `tvos`, `android`); no FAIL, except "unsigned" or "debug key" while signing is missing.
2. `scripts/check-livebuild.sh --check` on each artefact: the live build has no mock or developer marker, and the live API base is present.
3. Privacy manifests: `assets/ios/PrivacyInfo.xcprivacy` and `assets/tvos/PrivacyInfo.xcprivacy` match the data list in `docs/store/app-privacy.md`; the check reports every category the binary uses and fails on an undeclared one.
4. Brand: `go run ./cmd/brandcheck` passes (icon.png opaque RGB 1024², ITMS-90717; iOS dark/tinted, Android layers, splash and notification icons; tvOS brand assets and Top Shelf; `gdesk.json` wiring). The pictures are DanceFITVN's logo laid out by `docs/brand/generate.sh` (`docs/brand/BRAND.md`); the owner's sign-off on the redraw, motto and ® is BLOCKERS D1. Store pictures for the Play listing: `docs/brand/mobile/out/play/` (icon 512, feature graphic 1024x500); store screenshots: `docs/brand/mobile/store-frame.html`, checked with `go run ./cmd/brandcheck -shots DIR`.
5. Export: `usesNonExemptEncryption` false in `gdesk.json` (`docs/store/export-compliance.md`).
6. Store texts: `docs/store/app-privacy.md`, `docs/store/data-safety.md`, `docs/store/review-notes.en.md` match the build and have no TBD left for the release.
7. `SHA256SUMS` lists every artefact; record the sizes for the store listing (P8.4).

## Privacy manifests

Two files, both copied to the bundle root by gDesk after `checkPrivacyManifest` (`gdesk.json` `ios.privacyManifest`, `tvos.privacyManifest`). Tracking false, no tracking domains.

Required-reason APIs (categories and reason codes from Xcode's own definitions):

| Category | Used by | Reason | iOS | tvOS |
|---|---|---|---|---|
| FileTimestamp | Go `os.Stat` on files in the app container (`internal/sys`, the cache, the progress queue) | `C617.1` | yes | yes |
| SystemBootTime | Go monotonic clock (`mach_absolute_time`) | `35F9.1` | yes | yes |
| DiskSpace | free-space check before a download (`internal/sys/freespace_ios.go`, volume keys and `statfs`) | `E174.1`, `85F4.1` | yes | only if the tvOS binary uses it (Q-P8-7) |
| UserDefaults | settings store on tvOS (`internal/store/tvstore`) | `CA92.1` | no | yes |

Collected data types: as in `docs/store/app-privacy.md` (iOS: name, email, phone, photos, other user content, user id, device id, product interaction; tvOS: the subset without phone, photos and other user content).

## Sizes

Record the sizes of the IPA, the APK and the AAB for P8.4 (07 §5). The script does not print a size table; read them from `dist/release/` by hand. Per-ABI sizes need bundletool (`get-size total --dimensions=ABI`; not run by the script). Without bundletool the APK size is the only Android number.

## What is blocked (BLOCKERS)

| Item | Owner | Fallback now | Where |
|---|---|---|---|
| Apple Distribution certificate, App Store profiles (iOS, tvOS), App Store Connect record | account holder | unsigned IPA (layout only) and simulator builds | BLOCKERS P8.2 |
| Play upload keystore, Play Console app | account holder | APK signed with gDesk's debug key (testers only) | BLOCKERS P8.2 |
| bundletool jar (sha256) | account holder / developer | sizes from the APK | Q-P8-11 |
| Sign-off of the brand pictures (mark redraw, traced wordmark, motto, ®) | DanceFITVN | DanceFITVN logo laid out in `docs/brand/` (real icon, tvOS brand assets, Top Shelf) | BLOCKERS D1 |
| Privacy policy URL, support URL, data list confirmation | DanceFITVN | draft in `docs/store/` | BLOCKERS P8.2 |
| Demo account for App Review | DanceFITVN | placeholder in `docs/store/review-notes.en.md` | BLOCKERS P8.2 |
| Live server, its base URL, a test account | DanceFITVN | mock app (demo flavour) | BLOCKERS P8.1 |
| Face ID key in the release (gDesk G3a) | gDesk owner | `scripts/release.sh` writes the key before signing | P8.2 F |
| Legal confirmation of export compliance | DanceFITVN | flag false, see `docs/store/export-compliance.md` | TBD |
| Targeting Android SDK (targetSdk 36) | DanceFITVN / developer | 36 in `gdesk.json`; verify against Play's current rule | Q-P8-6 |

## Where things are

| What | Where |
|---|---|
| Release script | `scripts/release.sh`, with `scripts/release/check-ios.sh`, `check-android.sh`, `iosscan.py` |
| Live-build check | `scripts/check-livebuild.sh` |
| Release artefacts | `dist/release/` (gitignored) |
| Simulator builds | `local/release/sim/<sys>/` (gitignored, not shipped) |
| Build snapshot | `local/release/src/` (gitignored) |
| Signing profiles and keystore | `local/…` (gitignored); API key `~/.appstoreconnect/private_keys/` |
| Privacy manifests | `assets/ios/PrivacyInfo.xcprivacy`, `assets/tvos/PrivacyInfo.xcprivacy` |
| Store texts | `docs/store/` |
| Brand (icons, tvOS brand assets, Top Shelf, Play pictures, store frame) | `docs/brand/` (`BRAND.md`, `generate.sh`), `icon.png`, `assets/brand/`, `assets/tvos/DanceFITVN.brandassets/`; check `cmd/brandcheck` |
