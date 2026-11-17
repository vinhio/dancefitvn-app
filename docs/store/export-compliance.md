# Export compliance — DanceFITVN

*Adapted from jplayer-mobile/docs/store/export-compliance.md and jplayer-tv/docs/release.md ("Export compliance") at 49de3c2 / 781bee5.*

## The answer

`usesNonExemptEncryption` is **false** for both platforms in `gdesk.json`:

- `ios.usesNonExemptEncryption: false` → Info.plist `ITSAppUsesNonExemptEncryption` = false.
- `tvos.usesNonExemptEncryption: false` → the same key in the tvOS bundle.

Android has no equivalent key; the Play export declaration is answered in Play Console (see the last section).

## Why

The app uses only standard encryption that is built into the platform or is the standard TLS of the Go library:

- All network traffic is HTTPS (TLS) to DanceFITVN's own API and to the media URLs the API returns (`https://dancefitvn.com/mobile/api/v1`, HLS and audio over HTTPS). The TLS is done by the system (AVFoundation, the OS network stack) or by Go's standard `net/http` and `crypto/tls`.
- Tokens are stored in the Keychain or the OS secret store (`bridge/system/secret`), not in a custom cipher.
- The app does not implement its own cryptography. It has no VPN, no messaging, no end-to-end encryption, no custom key exchange, and no encryption for the user's content beyond the TLS in transit.
- The app does not use proprietary or non-standard algorithms. Hashes (SHA-256 for the lesson-audio checksum) are for integrity checks, not for hiding content.

Under the standard-encryption exemption, an app that uses only standard TLS/HTTPS and OS-provided crypto answers "No" to "contains non-exempt encryption". That is why the flag is false.

## Caveats (what must be confirmed)

- **TBD (DanceFITVN):** a legal or compliance person confirms the answer. This note is not legal advice (the same caveat as jPlayer TV, docs/release.md).
- The Go TLS stack is part of the Go standard library (since the app uses Go's `net/http` for the API). Same question as jPlayer TV's T13 note: confirm against Apple's export compliance documentation before a public release. The flag stays false for the trial build (internal TestFlight).
- If the app ever adds its own encryption (for example, encrypting the local progress queue with a custom key, or a non-standard protocol), the answer must change to "true" and the export documents must be filed. Re-check at every new feature that touches crypto.

## Google Play

Play asks the export question in the Console, in App content or the release form, not in the bundle. The answer follows the same facts: standard HTTPS only, no non-standard cryptography. **TBD (DanceFITVN):** confirm the Play answer when the first upload is made.
