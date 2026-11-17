package kit_test

import (
	"testing"

	"dancefitvn-app/internal/ui/kit/kittest"
	"dancefitvn-app/internal/ui/theme"

	"github.com/gdeskdev/desk/foundation/core/geom"
	"github.com/gdeskdev/desk/foundation/scene"
	"github.com/gdeskdev/desk/framework/ui"
)

// vitem is a child of a vstack with the room above it.
type vitem struct {
	w      ui.Widget
	before float32
}

// vstack stacks its children, `before` apart from the one above, each as wide as
// the room: a column with margins, as the prototype's block flow is.
type vstack struct {
	ui.Base
	items []vitem
}

// Layout stacks the items across the room.
func (s *vstack) Layout(c *ui.Context, cs ui.Constraints) geom.Size {
	w := cs.Max.W
	y := float32(0)
	for _, it := range s.items {
		y += it.before
		sz := c.Layout(it.w, ui.Constraints{Min: geom.Sz(w, 0), Max: geom.Sz(w, ui.Unbounded)})
		c.Place(it.w, geom.Pt(0, y))
		y += sz.H
	}

	return cs.Constrain(geom.Sz(w, y))
}

// newVStack returns a stack of the items.
func newVStack(items ...vitem) *vstack {
	s := &vstack{}
	s.items = items
	for _, it := range items {
		s.Add(it.w)
	}

	return s
}

// block is a box of a fixed height that draws nothing: a part the kit does not
// own (the 64 circle of the delete sheet, an inline line of text).
type block struct {
	ui.Base
	h float32
}

// Layout takes the width it gets and its height.
func (b *block) Layout(_ *ui.Context, cs ui.Constraints) geom.Size {
	return cs.Constrain(geom.Sz(cs.Max.W, b.h))
}

// overlays is a root that lays every child over the whole screen on --bg, the
// way the screen, the sheet host and the toast host are stacked: a screen under
// the overlays.
type overlays struct {
	ui.Base
}

// newOverlays returns a root of children stacked over the screen.
func newOverlays(children ...ui.Widget) *overlays {
	o := &overlays{}
	o.Add(children...)

	return o
}

// Layout gives every child the whole screen.
func (o *overlays) Layout(c *ui.Context, cs ui.Constraints) geom.Size {
	for ch := range o.Children() {
		c.Layout(ch, ui.Tight(cs.Max))
		c.Place(ch, geom.Point{})
	}

	return cs.Max
}

// Paint fills the screen with --bg and paints the children.
func (o *overlays) Paint(c *ui.Context, p *ui.Painter) {
	p.Fill(o.Bounds(), scene.Solid(theme.From(c).T.Bg), 0)
	o.Base.Paint(c, p)
}

// anchor says how an element of the 393 wide prototype screen follows the wider
// Android phone: the layout is the same, the box is not (07 section 3.3).
type anchor uint8

const (
	// anchorLeft stays where it is.
	anchorLeft anchor = iota
	// anchorRight moves with the right edge.
	anchorRight
	// anchorCenter moves with the middle.
	anchorCenter
	// anchorStretch grows with the width.
	anchorStretch
)

// onPhone returns the box of the 393 x 852 prototype screen on the phone: the
// horizontal anchor moves it; the vertical one is the caller's (bottom and top).
func onPhone(ph kittest.Phone, b geom.Rect, a anchor) geom.Rect {
	dw := ph.Size.W - 393
	switch a {
	case anchorRight:
		b.X += dw
	case anchorCenter:
		b.X += dw / 2
	case anchorStretch:
		b.W += dw
	case anchorLeft:
	}

	return b
}

// fromBottom returns the box moved with the bottom edge of the phone.
func fromBottom(ph kittest.Phone, b geom.Rect) geom.Rect {
	b.Y += ph.Size.H - 852

	return b
}

// requireNoLeaks fails the test when the view reports leaked state.
func requireNoLeaks(t testing.TB, tt *ui.Tester) {
	t.Helper()
	if l := tt.Leaks(); len(l) > 0 {
		t.Errorf("leaks: %v", l)
	}
}

// barAtBottom holds a widget at the bottom of the screen, as wide as the screen
// and as tall as it wants to be: where the navigation puts the tab bar.
type barAtBottom struct {
	ui.Base
	bar ui.Widget
}

// Layout puts the bar at the bottom.
func (b *barAtBottom) Layout(c *ui.Context, cs ui.Constraints) geom.Size {
	if b.ChildCount() == 0 {
		b.Add(b.bar)
	}
	s := c.Layout(b.bar, ui.Constraints{Max: geom.Sz(cs.Max.W, ui.Unbounded)})
	c.Place(b.bar, geom.Pt(0, cs.Max.H-s.H))

	return cs.Max
}
