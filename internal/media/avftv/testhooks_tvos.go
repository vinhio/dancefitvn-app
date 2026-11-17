// Adapted from jplayer-mobile/internal/media/avf/testhooks_ios.go at 49de3c2.

//go:build ios && tvos

package avftv

import (
	"github.com/gdeskdev/desk/foundation/ffi/objc"
)

// ====================================================================
// ========================== Main functions ==========================
// ====================================================================

// Tick runs the time tick of e now, as its ticker does every 250 ms while
// it plays; false for a nil or closed engine. For device measures only
// (the tick's allocations); UI thread.
func Tick(e *Engine) bool {
	if e == nil || e.closed {
		return false
	}
	e.tick()

	return true
}

// PlayerRate reads, from AVFoundation, the rate and the defaultRate of the
// player of e (AVPlayer.h:152, :163): the speed it plays at now (0
// paused) and the one play uses; false for a nil or closed engine or an
// error. For device measures only (04 §2.2: play keeps the speed
// chosen); UI thread. It allocates nothing.
func PlayerRate(e *Engine) (rate, defaultRate float64, ok bool) {
	if e == nil || e.closed || e.inv.rate == nil {
		return 0, 0, false
	}
	if e.inv.rate.Invoke(e.player) != nil || e.inv.defaultRate.Invoke(e.player) != nil {
		return 0, 0, false
	}
	r, err := e.inv.rate.Result[float32]()
	if err != nil {
		return 0, 0, false
	}
	d, err := e.inv.defaultRate.Result[float32]()
	if err != nil {
		return 0, 0, false
	}

	return float64(r), float64(d), true
}

// PitchAlgorithm reads the audioTimePitchAlgorithm of the item of e
// (04 §2.2: TimeDomain); false for a nil engine or nothing loaded. For
// device measures only; UI thread.
func PitchAlgorithm(e *Engine) (string, bool) {
	if e == nil || e.closed || e.item == 0 {
		return "", false
	}
	var s string
	objc.WithPool(func() { s = objc.GoString(objc.Call(e.item, sel.pitchAlgorithm)) })

	return s, true
}

// LayerScaleX reads the transform.scale.x of the AVPlayerLayer of e back
// from Core Animation: -1 mirrored, 1 not; false before the layer exists.
// For device tests only; UI thread.
func LayerScaleX(e *Engine) (float64, bool) {
	if e == nil || e.closed || e.plane == nil {
		return 0, false
	}

	return e.plane.scaleX()
}

// LayerReadyForDisplay reads the readyForDisplay of the AVPlayerLayer of
// e: its player's current item has a frame on the layer; ok false before
// the layer exists. For device tests only; UI thread.
func LayerReadyForDisplay(e *Engine) (ready, ok bool) {
	if e == nil || e.closed || e.plane == nil || e.plane.layer == 0 {
		return false, false
	}

	return e.plane.readyForDisplay(), true
}
