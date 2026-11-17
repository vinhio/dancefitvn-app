// Adapted from jplayer-tv/internal/media/avf/limits.go at 781bee5 (the item's preferredMaximumResolution, KT7) and jplayer-tv/internal/media/avf/config.go at 781bee5 (Quality).

//go:build ios && !tvos && cgo

package avf

import (
	"dancefitvn-app/internal/media"
)

// ====================================================================
// ========================== Main functions ==========================
// ====================================================================

// SetMaxResolution limits the video of e, an engine of this package, to w
// by h pixels (AVPlayerItem.preferredMaximumResolution, AVPlayerItem.h:552):
// an HLS stream then picks no variant larger, the app's "Chất lượng video"
// setting (Q-P4-3). 0×0 (or any side not positive) lifts the limit, which
// is AVFoundation's default. It applies to the item loaded now and to
// every later one. It reports false for another engine or a closed one,
// or when the current item refused it. UI thread.
func SetMaxResolution(e media.Engine, w, h int) bool {
	en, ok := e.(*engine)
	if !ok || en.closed {
		return false
	}
	size := cgSize{}
	if w > 0 && h > 0 {
		size = cgSize{W: float64(w), H: float64(h)}
	}
	en.maxRes = size
	if en.cur == nil || en.cur.item == 0 {
		return true
	}

	return en.applyMaxResolution(en.cur.item)
}

// MaxResolution reads the preferredMaximumResolution of the item of e, an
// engine of this package, back from AVFoundation (0×0: no limit); false
// for another engine, nothing loaded, or an error. For device tests only;
// UI thread.
func MaxResolution(e media.Engine) (w, h float64, ok bool) {
	en, isAVF := e.(*engine)
	if !isAVF || en.closed || en.cur == nil || en.cur.item == 0 {
		return 0, 0, false
	}
	in := en.inv.maxResolution
	if in.Invoke(en.cur.item) != nil {
		return 0, 0, false
	}
	s, err := in.Result[cgSize]()
	if err != nil {
		return 0, 0, false
	}

	return s.W, s.H, true
}
