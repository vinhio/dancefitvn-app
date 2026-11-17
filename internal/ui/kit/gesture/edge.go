// Adapted from jplayer-mobile/internal/ui/nav/edge.go at 49de3c2 (the strip,
// the axis lock and the velocity of the finger; the interactive slide of the
// screen is not taken: the prototype has no transitions).

package gesture

import (
	"time"

	"github.com/gdeskdev/desk/foundation/core/geom"
)

// ====================================================================
// ============================ Contracts =============================
// ====================================================================

// The numbers of the swipe back from the left edge (Q-P2-4: the prototype has
// none; docs/plan/03 section 1 says Back and the iOS edge swipe are back()).
const (
	// EdgeStrip is how wide, in DIPs, the strip at the left edge is where a
	// press may start a swipe back; jPlayer Mobile's style.EdgeBackWidth.
	EdgeStrip float32 = 26
	// EdgeDistance is how far right, in DIPs, the finger must go to commit
	// the swipe by position.
	EdgeDistance float32 = 80
	// EdgeFling is the speed, in DIPs a millisecond, that commits it by
	// itself.
	EdgeFling float32 = .6
)

// ====================================================================
// ============================= Creation =============================
// ====================================================================

// Edge follows a finger that pressed in the strip at the left edge: whether it
// goes sideways, and whether its lifting commits a swipe back. It draws
// nothing and moves nothing; the navigation asks it and pops the stack.
type Edge struct {
	// Strip, Distance and Fling are the numbers above; EdgeBack sets them.
	Strip, Distance, Fling float32

	track Track
	x0    float32
	armed bool
	taken bool
}

// EdgeBack returns the numbers of the swipe back.
func EdgeBack() Edge { return Edge{Strip: EdgeStrip, Distance: EdgeDistance, Fling: EdgeFling} }

// ====================================================================
// ========================== Main functions ==========================
// ====================================================================

// InStrip tells whether x, in the coordinates of the left edge (0 on it), is in
// the strip.
func (e *Edge) InStrip(x float32) bool { return x >= 0 && x < e.Strip }

// Press notes a press at at, whose left edge is at x = left (the window's
// coordinates), at event time evTime (0 when the driver gives none) and view
// time now. It tells whether the press is in the strip and is followed.
func (e *Edge) Press(at geom.Point, left float32, evTime, now time.Duration) bool {
	e.armed, e.taken = false, false
	if !e.InStrip(at.X - left) {
		return false
	}
	e.armed, e.x0 = true, at.X
	e.track.Want = AxisX
	e.track.PressAt(at, evTime, now)

	return true
}

// MoveAt follows the finger and tells what the move did: StepStart when it
// locks to the horizontal axis and goes right (the swipe is taken, with this
// move), StepMove after that, StepRefuse when it went vertical first or left,
// StepWait under the slop.
func (e *Edge) MoveAt(at geom.Point, evTime, now time.Duration) Step {
	if !e.armed {
		return StepUnknown
	}
	step := e.track.MoveAt(at, evTime, now)
	switch step {
	case StepStart:
		if at.X < e.x0 { // a swipe back goes right
			e.armed = false
			e.track.Cancel()

			return StepRefuse
		}
		e.taken = true
	case StepRefuse:
		e.armed = false
	case StepUnknown, StepWait, StepMove:
	default:
	}

	return step
}

// ReleaseAt lifts the finger at at, at event time evTime and view time now,
// and tells whether to go back: the swipe was taken, and the finger went at
// least Distance right of where it pressed or faster than Fling (the fit of
// its latest samples; none when it rested before it lifted).
func (e *Edge) ReleaseAt(at geom.Point, evTime, now time.Duration) (commit bool) {
	if !e.armed || !e.taken {
		e.Cancel()

		return false
	}
	d, v, started := e.track.ReleaseAt(at, evTime, now)
	e.armed, e.taken = false, false

	return started && (d.X >= e.Distance || v.X > e.Fling)
}

// Cancel forgets the press: the system took the touch away.
func (e *Edge) Cancel() {
	e.track.Cancel()
	e.armed, e.taken = false, false
}

// Armed tells whether a press in the strip is followed and may still become the
// swipe.
func (e *Edge) Armed() bool { return e.armed }

// Taken tells whether the swipe was taken: the finger went sideways, right,
// from the strip.
func (e *Edge) Taken() bool { return e.taken }
