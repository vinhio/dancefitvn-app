// Adapted from jplayer-mobile/internal/app/media_other.go at 49de3c2.

//go:build !tvos && !(ios && cgo) && !(android && cgo)

package app

import "dancefitvn-app/internal/media"

// ====================================================================
// ============================ Contracts =============================
// ====================================================================

// mediaLate makes the system's playback after the first frame
// (lateMedia): the fake engine costs nothing.
const mediaLate = false

// qualityLimited stands for the iPhone's: the fake plays in its place on
// the Mac, where the screens are compared with the prototype, which has
// the quality row.
const qualityLimited = true

// ====================================================================
// ============================= Creation =============================
// ====================================================================

// newPlatformMedia reports core.ErrUnsupported: the system's engines are
// iOS's and Android's; elsewhere the window plays on the fake engine.
func newPlatformMedia(media.Config, func(string, ...any)) (platformMedia, error) {
	return platformMedia{}, errNoEngine
}

// ====================================================================
// ========================== Main functions ==========================
// ====================================================================

// warmMedia has nothing to warm; ready runs at once (posted).
func warmMedia(cfg media.Config, ready func(), _ func(string, ...any)) {
	if ready != nil {
		cfg.Dispatch(ready)
	}
}

// limitResolution has no engine to limit off the phones. It reports false.
func limitResolution(media.Engine, int, int) bool { return false }
