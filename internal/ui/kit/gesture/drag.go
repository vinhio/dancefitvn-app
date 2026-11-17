// Adapted from jplayer-mobile/internal/ui/gesture/drag.go at 49de3c2.

package gesture

import (
	"time"

	"github.com/gdeskdev/desk/foundation/core/geom"
)

// ====================================================================
// ============================ Contracts =============================
// ====================================================================

// The numbers of the drag that dismisses a sheet (docs/plan/03 section 1: the
// app adds the system's pull-down to the prototype, whose sheet closes by its
// buttons and the dim only; jPlayer's 28 % of the height became an absolute
// distance).
const (
	// SheetDistance is how far down, in DIPs, a finger must pull a sheet to
	// dismiss it by position: 80 DIPs.
	SheetDistance float32 = 80
	// SheetFling is the speed, in DIPs a millisecond, that dismisses it by
	// itself: 0.6 DIP/ms (600 DIPs a second).
	SheetFling float32 = .6
	// SheetResist is how far a pull up follows the finger: 0, the sheet does
	// not leave its place upward (its bottom would show the screen under it).
	SheetResist float32 = 0
)

// ====================================================================
// ============================= Creation =============================
// ====================================================================

// Drag follows a sheet pulled down to dismiss it. The sheet takes the press
// (a widget that wants it checks Exempted first), feeds the moves and the
// lifting, and draws itself Offset down.
type Drag struct {
	// Distance is how far down a drag must pass to dismiss, in DIPs.
	Distance float32
	// Fling is the speed, in DIPs a millisecond, that dismisses by itself.
	Fling float32
	// Resist is how far a pull up follows the finger; 0 holds the page.
	Resist float32

	track  Track
	y      float32
	active bool
}

// SheetDrag returns the numbers of a sheet's drag.
func SheetDrag() Drag {
	return Drag{Distance: SheetDistance, Fling: SheetFling, Resist: SheetResist}
}

// ====================================================================
// ========================== Main functions ==========================
// ====================================================================

// Press notes a press that may become a drag of the page.
func (d *Drag) Press(at geom.Point, now time.Duration) {
	d.track.Want = AxisY
	d.track.Press(at, now)
	d.y, d.active = 0, false
}

// PressAt is Press for a press whose event carries its time evTime (0 when the
// driver does not tell): the drag measures the finger's speed by the events'
// times, or by now, the view's clock, when they are 0 (Track.PressAt).
func (d *Drag) PressAt(at geom.Point, evTime, now time.Duration) {
	d.track.Want = AxisY
	d.track.PressAt(at, evTime, now)
	d.y, d.active = 0, false
}

// MoveAt is Move for a drag begun with PressAt.
func (d *Drag) MoveAt(at geom.Point, evTime, now time.Duration) Step {
	return d.follow(d.track.MoveAt(at, evTime, now))
}

// Move follows the finger. After StepStart and StepMove the page is at Offset:
// `max(0, dy) + min(0, dy) * Resist`. StepRefuse tells that the finger went
// sideways first: the press is not the page's.
func (d *Drag) Move(at geom.Point, now time.Duration) Step {
	return d.follow(d.track.Move(at, now))
}

// Release lifts the finger and tells whether to dismiss the page: `dy >
// Distance || vy > Fling`, the dy measured to the place the finger lifted, the
// speed that of the last step. Offset keeps where the page was, for it to go
// from there, to the end or back to 0, until the next Press.
func (d *Drag) Release(at geom.Point) (dismiss bool) {
	dd, v, started := d.track.Release(at)

	return d.lift(dd, v, started)
}

// ReleaseAt is Release for a drag begun with PressAt, lifted at evTime by the
// system's clock (0 when unknown) and now by the view's: a finger that rested
// before it lifted does not fling (Track.ReleaseAt).
func (d *Drag) ReleaseAt(at geom.Point, evTime, now time.Duration) (dismiss bool) {
	dd, v, started := d.track.ReleaseAt(at, evTime, now)

	return d.lift(dd, v, started)
}

// Cancel takes the drag away; Offset keeps where the page was, for it to
// return from there.
func (d *Drag) Cancel() {
	d.track.Cancel()
	d.active = false
}

// Offset returns how far the page is pulled down now, in DIPs; negative for a
// pull up with resistance. After Release or Cancel it is where the page was.
func (d *Drag) Offset() float32 { return d.y }

// Active tells whether the page is being dragged.
func (d *Drag) Active() bool { return d.active }

// Armed tells whether a press is followed that may still become a drag.
func (d *Drag) Armed() bool { return d.track.Armed() }

// ====================================================================
// ========================= Helper Functions =========================
// ====================================================================

// follow takes what the track made of a move, and sets the offset.
func (d *Drag) follow(step Step) Step {
	if step != StepStart && step != StepMove {
		return step
	}
	d.active = true
	dy := d.track.Delta().Y
	d.y = max(0, dy) + min(0, dy)*d.Resist

	return step
}

// lift ends the drag and tells whether it dismisses the page.
func (d *Drag) lift(dd, v geom.Point, started bool) bool {
	d.active = false
	if !started {
		d.y = 0

		return false
	}

	return dd.Y > d.Distance || v.Y > d.Fling
}
