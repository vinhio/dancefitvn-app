// Adapted from jplayer-mobile/internal/media/avf/output_ios.go at 49de3c2.

//go:build ios && !tvos && cgo

package avf

import "github.com/gdeskdev/desk/foundation/ffi/objc"

// ====================================================================
// ========================== Main functions ==========================
// ====================================================================

// pull is the surface's pull (Surface.SetPull, D4): a view calls it from its
// Paint while the surface is Active. It copies the output's new frame into
// the back image through the shim, publishes it and tells the views; with no
// new frame it does nothing, so the view draws nothing new. After a seek or
// a pause it stops the pulls once the frame of the new position came, or
// settleFor later. While a next output of another size waits for its first
// frame (followSize), it takes that output's place as soon as it has one.
// UI thread; it allocates nothing once the images have the frame's size.
func (e *engine) pull() {
	e.pulls++
	l := e.cur
	if l == nil || l.output == 0 {
		return
	}
	if l.next != 0 && !e.starveNext && hasNew(l.next) {
		objc.WithPool(func() { e.promote(l) })
	}
	back := e.video.Back(l.fw, l.fh, true)
	if back == nil {
		return
	}
	if !pullInto(l.output, back, e.force) {
		if e.settling && !e.seeks.Pending() && e.cfg.Clock.Now().After(e.settleUntil) {
			e.endSettle()
		}

		return
	}
	e.force = false
	e.frames++
	e.video.Publish()
	e.video.Notify()
	// While a seek is under way the frame may be the one it leaves (a
	// paused load from Start: the item's first frame comes before the
	// seek's, P4.1): the pulls go on until the seek's frame.
	if e.settling && !e.seeks.Pending() {
		e.endSettle()
	}
}
