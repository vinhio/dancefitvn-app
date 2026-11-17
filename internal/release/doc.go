// Adapted from jplayer-mobile/internal/release/doc.go at 49de3c2.

// Package release holds the release gates of DanceFITVN: tests that keep
// gdesk.json, the icon and the two privacy manifests (iOS, tvOS)
// consistent with each other, with the code that reaches Apple's required
// reason APIs and with the stores' rules (local/plans/P8.md P8.2). It has
// no code; scripts/release.sh and scripts/release/check-*.sh check the
// builds themselves.
package release
