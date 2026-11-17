// Adapted from jplayer-mobile/internal/ui/nav/edge.go at 49de3c2 (the pattern: a
// press noted in the strip, taken after the axis lock, held by HandlePointer, the
// gate on SystemGestures().Left). The strip, the lock and the velocity are
// kit/gesture.Edge's (Q-P2.5-1); the screens do not follow the finger.

package nav

import (
	"dancefitvn-app/internal/ui/kit/gesture"
	"github.com/gdeskdev/desk/foundation/platform"
	"github.com/gdeskdev/desk/framework/ui"
)

// ====================================================================
// ========================== Main functions ==========================
// ====================================================================

// EdgeArmed tells whether a swipe back can start in the strip at the left edge now:
// the system does not take swipes from that edge (SystemGestures().Left is 0: iOS,
// Android with three-button navigation; with gesture navigation the system's Back
// arrives as the key instead), and Back has something to do. A widget with a drag
// of its own across the strip declines presses in it while this is true.
func (n *Nav) EdgeArmed(c *ui.Context) bool {
	return c.SystemGestures().Left <= 0 && n.CanBack()
}

// InEdgeStrip tells whether x, in the window's coordinates, is in the strip where a
// press starts a swipe back now.
func (n *Nav) InEdgeStrip(c *ui.Context, x float32) bool {
	return n.EdgeArmed(c) && n.edge.InStrip(x-n.WindowFrame().X)
}

// InterceptPointer notes a press that starts in the strip, and takes it once the
// finger has gone 8 DIPs and mostly sideways, to the right; a finger that goes up
// or down first is a scroll's. The scroll inside is asked first and declines a
// sideways move, so the Nav is asked next.
func (n *Nav) InterceptPointer(c *ui.Context, e *ui.PointerEvent) bool {
	switch e.Kind {
	case ui.PointerDown:
		n.press(c, e)
	case ui.PointerMove:
		if !n.edge.Armed() || n.edge.Taken() {
			return false
		}

		return n.edge.MoveAt(e.Pos.Add(n.WindowFrame().Min()), e.Time, c.Now()) == gesture.StepStart
	case ui.PointerUnknown, ui.PointerUp, ui.PointerEnter, ui.PointerLeave, ui.PointerCancel, ui.PointerScroll:
	default:
	}

	return false
}

// HandlePointer holds a press in the strip that nothing else held, so that its moves
// reach InterceptPointer, follows the finger of a swipe the Nav took, and goes Back
// when it lifts far or fast enough.
func (n *Nav) HandlePointer(c *ui.Context, e *ui.PointerEvent) {
	switch e.Kind {
	case ui.PointerDown:
		if n.press(c, e) {
			e.Handle()
		}
	case ui.PointerMove:
		if n.edge.Taken() {
			e.Handle()
			n.edge.MoveAt(e.Pos.Add(n.WindowFrame().Min()), e.Time, c.Now())
		}
	case ui.PointerUp:
		if n.edge.Taken() {
			e.Handle()
			if n.edge.ReleaseAt(e.Pos.Add(n.WindowFrame().Min()), e.Time, c.Now()) {
				n.Back()
			}
		}
		n.edge.Cancel()
	case ui.PointerCancel:
		n.edge.Cancel()
	case ui.PointerUnknown, ui.PointerEnter, ui.PointerLeave, ui.PointerScroll:
	default:
	}
}

// ====================================================================
// ========================= Helper Functions =========================
// ====================================================================

// press notes a press and tells whether it is in the strip and followed: a primary
// press, with the edge open to the app.
func (n *Nav) press(c *ui.Context, e *ui.PointerEvent) bool {
	n.edge.Cancel()
	if e.Button != platform.ButtonPrimary && e.Button != platform.ButtonNone {
		return false
	}
	if !n.EdgeArmed(c) {
		return false
	}
	w := n.WindowFrame()

	return n.edge.Press(e.Pos.Add(w.Min()), w.X, e.Time, c.Now())
}
