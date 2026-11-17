// Adapted from jplayer-mobile/internal/ui/gesture/exempt.go at 49de3c2.

package gesture

import (
	"github.com/gdeskdev/desk/foundation/core/geom"
	"github.com/gdeskdev/desk/framework/ui"
)

// ====================================================================
// ============================ Contracts =============================
// ====================================================================

// Exempt is a widget a press on which never starts a sheet's drag: the
// grip, the text inputs, the range (and a row being swiped), which the
// prototype lists in `.grip, input, textarea, .range, .row.sliding`
// (js/gestures.js:76).
type Exempt interface {
	// DragExempt tells whether the widget is exempt now (a row only while it
	// slides).
	DragExempt() bool
}

// ====================================================================
// ========================== Main functions ==========================
// ====================================================================

// Exempted tells whether the deepest widget of root's subtree at p, in the
// window's coordinates, or one of its ancestors up to root, is Exempt. It
// skips hidden widgets and honours HitTester as the view's hit test does, and
// allocates nothing.
func Exempted(root ui.Widget, p geom.Point) bool {
	if root == nil {
		return false
	}
	b := root.Node()
	off := b.Parent()
	var origin geom.Point
	if off != nil {
		origin = off.Node().WindowFrame().Min()
	}

	return exempted(root, p.Sub(origin))
}

// ====================================================================
// ========================= Helper Functions =========================
// ====================================================================

// exempted walks the subtree of w, p being in the coordinates of w's parent,
// and tells whether the widget hit or a widget around it up to w is Exempt.
func exempted(w ui.Widget, p geom.Point) bool {
	b := w.Node()
	if b.Hidden || !b.Frame().Contains(p) {
		return false
	}
	local := p.Sub(b.Frame().Min())
	// The last child is the topmost: the last one that catches p wins, as
	// the view's hitTest takes the first from the end.
	var found ui.Widget
	for ch := range b.Children() {
		if inside(ch, local) {
			found = ch
		}
	}
	if found != nil && exempted(found, local) {
		return true
	}
	if found == nil {
		if t, ok := w.(ui.HitTester); ok && !t.HitTest(local) {
			return false
		}
	}
	x, ok := w.(Exempt)

	return ok && x.DragExempt()
}

// inside tells whether the point, in the parent's coordinates, is on the
// widget or a descendant that catches it: the test the view's hitTest makes.
func inside(w ui.Widget, p geom.Point) bool {
	b := w.Node()
	if b.Hidden || !b.Frame().Contains(p) {
		return false
	}
	local := p.Sub(b.Frame().Min())
	for ch := range b.Children() {
		if inside(ch, local) {
			return true
		}
	}
	if t, ok := w.(ui.HitTester); ok && !t.HitTest(local) {
		return false
	}

	return true
}
