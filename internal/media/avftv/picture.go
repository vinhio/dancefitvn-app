package avftv

import "math"

// ====================================================================
// ============================ Contracts =============================
// ====================================================================

// pictureReading is what the engine reads when it looks for the first
// frame of a load on the screen (media.EventVideoSize): pure Go, so the
// rule is tested on a Mac.
type pictureReading struct {
	told     bool    // the load's picture was told already
	video    bool    // the load has a picture (Source.AudioOnly false)
	failed   bool    // the load failed
	attached bool    // the layer under the view shows the engine's player
	ready    bool    // the layer's readyForDisplay
	w, h     float64 // the item's presentationSize, 0 while unknown
}

// ====================================================================
// ========================= Helper Functions =========================
// ====================================================================

// pictureDue tells whether the first frame of the load is on the screen
// and not told yet, and its size in pixels, rounded: once a load, for a
// load with a picture that did not fail, when the layer showing the
// player says it is ready for display and the item knows its size. The
// engine attaches the player to the layer afresh at every load with a
// picture (nativePlane.detach, attach), which turns readyForDisplay back
// to false (tvOS 27 simulator, P7.5a probe), so a true is this load's.
// It allocates nothing.
func pictureDue(r pictureReading) (w, h int, due bool) {
	if r.told || !r.video || r.failed || !r.attached || !r.ready {
		return 0, 0, false
	}
	if !(r.w > 0 && r.h > 0) || math.IsInf(r.w, 0) || math.IsInf(r.h, 0) {
		return 0, 0, false
	}

	return int(math.Round(r.w)), int(math.Round(r.h)), true
}
