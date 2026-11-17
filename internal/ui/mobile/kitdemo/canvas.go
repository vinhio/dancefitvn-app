//go:build !tvos

package kitdemo

import (
	"dancefitvn-app/internal/tokens"
	"dancefitvn-app/internal/ui/kit"
	"dancefitvn-app/internal/ui/theme"

	"github.com/gdeskdev/desk/foundation/core/color"
	"github.com/gdeskdev/desk/foundation/core/geom"
	"github.com/gdeskdev/desk/foundation/scene"
	"github.com/gdeskdev/desk/framework/ui"
)

// ====================================================================
// ============================ Contracts =============================
// ====================================================================

// The prototype's phone, 393 x 852 (`#m`), the box every coordinate of a page
// is written in.
const (
	protoW float32 = 393
	protoH float32 = 852
)

// stretchMargin is the 07 section 3.3 rule's margin and cmd/shotcmp's default
// `-margin`: an edge this close to the screen's edge is anchored to it.
const stretchMargin float32 = 24

// placement is a widget and the prototype box it is at.
type placement struct {
	w   ui.Widget
	r   geom.Rect
	how how
	// mid centres the box in what holds it (an icon in a line of centred text).
	mid bool
}

// how a placement gets its size and where it follows the phone's size.
type how uint8

const (
	// howUnknown is the zero value: a placement nobody sized.
	howUnknown how = iota
	// block is as wide as the box and as tall as it needs.
	block
	// fit is as large as it needs (an inline element, a flex item).
	fit
	// sized is exactly the box.
	sized
)

// canvas is a screen as the kitdemo draws it: --bg, and widgets at the boxes
// the prototype puts them, in document order (layout.json numbers repeated
// names in that order). On a phone wider or taller than 393 x 852 the boxes
// follow the 07 section 3.3 rule: an element anchored to both sides stretches,
// one anchored to the right moves, a centred one moves by half; what the
// prototype pins to the bottom edge moves down.
type canvas struct {
	ui.Base

	items []placement
	// pinned holds the indexes of the items that move with the bottom edge.
	pinned map[int]bool
	// design is the size the boxes are written for: the prototype's phone, or the
	// TV stage.
	design geom.Size
}

// ====================================================================
// ============================= Creation =============================
// ====================================================================

// newCanvas returns an empty screen.
func newCanvas() *canvas {
	return &canvas{pinned: map[int]bool{}, design: geom.Sz(protoW, protoH)}
}

// ====================================================================
// ========================== Main functions ==========================
// ====================================================================

// at puts w as wide as the prototype box x..x+wd at y, as tall as it needs
// (a block-level element).
func (cv *canvas) at(w ui.Widget, x, y, wd float32) ui.Widget {
	return cv.add(w, geom.XYWH(x, y, wd, 0), block)
}

// fit puts w at x, y with the size it asks for; wd is the width the prototype
// gives the box, which tells where it is anchored on a wider phone.
func (cv *canvas) fit(w ui.Widget, x, y, wd float32) ui.Widget {
	return cv.add(w, geom.XYWH(x, y, wd, 0), fit)
}

// centre is fit for what sits in a line of centred text: it moves by half of
// what the container grows.
func (cv *canvas) centre(w ui.Widget, x, y, wd float32) ui.Widget {
	cv.add(w, geom.XYWH(x, y, wd, 0), fit)
	cv.items[len(cv.items)-1].mid = true

	return w
}

// box puts w exactly at the box.
func (cv *canvas) box(w ui.Widget, x, y, wd, h float32) ui.Widget {
	return cv.add(w, geom.XYWH(x, y, wd, h), sized)
}

// pin is at, for what the prototype pins to the bottom of the screen: it
// follows the bottom edge of a taller phone.
func (cv *canvas) pin(w ui.Widget, x, y, wd float32) ui.Widget {
	cv.add(w, geom.XYWH(x, y, wd, 0), block)
	cv.pinned[len(cv.items)-1] = true

	return w
}

// add appends a placement and makes w a child.
func (cv *canvas) add(w ui.Widget, r geom.Rect, h how) ui.Widget {
	cv.items = append(cv.items, placement{w: w, r: r, how: h})
	cv.Add(w)

	return w
}

// Layout puts every widget at its box, on the phone's size.
func (cv *canvas) Layout(c *ui.Context, cs ui.Constraints) geom.Size {
	room := cs.Max
	dw, dh := room.W-cv.design.W, room.H-cv.design.H
	// The heights of the boxes the page leaves open (`at`, `fit`) are what the
	// widgets measure at the design width: the rule needs them to tell which
	// elements share a line.
	boxes, labelled, mid := make([]span, len(cv.items)), make([]bool, len(cv.items)), make([]bool, len(cv.items))
	for i, it := range cv.items {
		h := it.r.H
		if h == 0 {
			switch it.how {
			case fit:
				h = c.Layout(it.w, ui.Loose(geom.Sz(ui.Unbounded, ui.Unbounded))).H
			case block:
				h = c.Layout(it.w, ui.Constraints{Min: geom.Sz(it.r.W, 0), Max: geom.Sz(it.r.W, ui.Unbounded)}).H
			case sized:
			default:
			}
		}
		boxes[i] = span{it.r.X, it.r.W, it.r.Y, h}
		_, labelled[i] = it.w.(*kit.Button)
		mid[i] = it.mid
	}
	xs, ws := planX(boxes, labelled, mid, dw, cv.design.W)
	for i, it := range cv.items {
		x, w := xs[i], ws[i]
		y := it.r.Y
		if cv.pinned[i] {
			y += dh
		}
		switch it.how {
		case fit:
			c.Layout(it.w, ui.Loose(geom.Sz(ui.Unbounded, ui.Unbounded)))
		case sized:
			c.Layout(it.w, ui.Tight(geom.Sz(w, it.r.H)))
		case block:
			c.Layout(it.w, ui.Constraints{Min: geom.Sz(w, 0), Max: geom.Sz(w, ui.Unbounded)})
		default:
			// howUnknown: nothing to lay out
		}
		c.Place(it.w, geom.Pt(x, y))
	}

	return room
}

// Paint fills the screen with --bg and paints the widgets.
func (cv *canvas) Paint(c *ui.Context, p *ui.Painter) {
	p.Fill(cv.Bounds(), scene.Solid(theme.From(c).T.Bg), 0)
	cv.Base.Paint(c, p)
}

// ====================================================================
// ========================= Helper Functions =========================
// ====================================================================

// abs32 returns |v|.
func abs32(v float32) float32 {
	if v < 0 {
		return -v
	}

	return v
}

// plate is a filled rounded box with no content: the card, the banner and the
// coloured square of the prototype that the kit has no widget for yet (a Card
// arrives with the screens of P3).
type plate struct {
	ui.Base

	fill   color.Color
	radius float32
}

// newPlate returns a plate of the color with the radius, named name (empty for
// none).
func newPlate(name string, fill color.Color, radius float32) *plate {
	p := &plate{fill: fill, radius: radius}
	p.Name = name

	return p
}

// newCard returns `.card` (index.html: --sf, radius 16) named name.
func newCard(name string) *plate { return newPlate(name, tokens.Dark.Surface, 16) }

// Layout takes the box it is given.
func (p *plate) Layout(_ *ui.Context, cs ui.Constraints) geom.Size {
	return cs.Constrain(cs.Max)
}

// Paint fills the box.
func (p *plate) Paint(_ *ui.Context, pt *ui.Painter) {
	pt.Fill(p.Bounds(), scene.Solid(p.fill), p.radius)
}
