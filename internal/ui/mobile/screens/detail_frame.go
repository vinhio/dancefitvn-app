//go:build !tvos

// Pattern from jplayer-mobile/internal/ui/views/coll.go at 49de3c2 (a page of a
// head, rows and a foot in one scroll, the head's buttons riding on the art, the
// page a Scroller whose offset is kept) and from
// jplayer-mobile/internal/ui/kit/hero.go at 49de3c2 (the hero's text block
// anchored to the bottom of the art, 6 above its edge). The frame is `.scr` with
// `.scroll{top:0}` and an absolute footer of MS.detail and MS.locked
// (index.html:396-412).

package screens

import (
	"github.com/gdeskdev/desk/foundation/core/color"
	"github.com/gdeskdev/desk/foundation/core/geom"
	"github.com/gdeskdev/desk/foundation/scene"
	"github.com/gdeskdev/desk/framework/ui"
)

// ====================================================================
// ============================ Contracts =============================
// ====================================================================

// The names layout.json gives the frame: the page's (nav.NewPage's).
const (
	nameFrame  = "div.scr"    // `.scr` (index.html:73)
	nameScroll = "div.scroll" // `.scroll` (:74)
)

// scrFrame is a screen without the tab bar and without the status bar's 54: `.scr`
// with its `.scroll` at `top:0` over the whole window, and the footer of the screen
// pinned over the scroll's foot (`position:absolute;left:0;right:0;bottom:0`).
// The scroll's content is as wide as the window: what must keep the 20 DIP gutters
// keeps them itself (the hero runs the width, `margin:0 -20px`).
type scrFrame struct {
	ui.Base

	scroll ui.Scroll
	body   frameBody
	foot   *scrFoot // nil: no footer (the loading state)
}

// frameBody is what the scroll scrolls: the content, with the padding under it.
type frameBody struct {
	ui.Base

	content ui.Widget
	bottom  float32
}

// scrFoot is the pinned footer: a button (MS.detail) or a line of text (MS.locked,
// K30) with `padding:T 20px 38px` over a gradient
// (`linear-gradient(transparent, var(--bg) 30%)`, MS.detail) or a plain --bg (MS.locked).
type scrFoot struct {
	ui.Base

	btn  ui.Widget // the button, or the text of the locked course
	pad  geom.Insets
	fade bool

	paint   scene.Paint
	paintH  float32     // the height the paint was made for
	paintBg color.Color // the --bg it was made for
}

// ====================================================================
// ============================= Creation =============================
// ====================================================================

// newFrame returns the frame around content with bottom DIPs of padding under it
// (`padding-bottom:110px` of the detail, the `.scroll`'s own 120 of the locked
// course) and the footer foot, which may be nil.
func newFrame(content ui.Widget, bottom float32, foot *scrFoot) *scrFrame {
	f := &scrFrame{foot: foot}
	f.Name = nameFrame
	f.scroll.Name = nameScroll
	f.scroll.Axes, f.scroll.HideBars = ui.ScrollVertical, true
	f.body.content, f.body.bottom = content, bottom
	f.body.Add(content)
	f.scroll.Add(&f.body)
	f.Add(&f.scroll)
	if foot != nil {
		f.Add(foot)
	}

	return f
}

// newFoot returns the footer around btn, padded by pad, with the gradient when
// fade.
func newFoot(btn ui.Widget, pad geom.Insets, fade bool) *scrFoot {
	f := &scrFoot{btn: btn, pad: pad, fade: fade}
	f.Add(btn)

	return f
}

// ====================================================================
// ========================== Main functions ==========================
// ====================================================================

// Scroll returns the frame's scroll.
func (f *scrFrame) Scroll() *ui.Scroll { return &f.scroll }

// Layout puts the scroll over the whole window and the footer on its bottom edge.
func (f *scrFrame) Layout(c *ui.Context, cs ui.Constraints) geom.Size {
	size := cs.Max
	c.Layout(&f.scroll, ui.Tight(size))
	c.Place(&f.scroll, geom.Point{})
	if f.foot != nil {
		fs := c.Layout(f.foot, ui.Constraints{Min: geom.Sz(size.W, 0), Max: geom.Sz(size.W, ui.Unbounded)})
		c.Place(f.foot, geom.Pt(0, size.H-fs.H))
	}

	return size
}

// Layout puts the content as wide as the window with its padding under it.
func (b *frameBody) Layout(c *ui.Context, cs ui.Constraints) geom.Size {
	w := cs.Max.W
	cz := c.Layout(b.content, ui.Constraints{Min: geom.Sz(w, 0), Max: geom.Sz(w, ui.Unbounded)})
	c.Place(b.content, geom.Point{})

	return cs.Constrain(geom.Sz(w, cz.H+b.bottom))
}

// Layout puts the button inside the padding; the footer is as tall as both.
func (f *scrFoot) Layout(c *ui.Context, cs ui.Constraints) geom.Size {
	w := cs.Max.W
	inner := max(w-f.pad.Left-f.pad.Right, 0)
	bs := c.Layout(f.btn, ui.Constraints{Min: geom.Sz(inner, 0), Max: geom.Sz(inner, ui.Unbounded)})
	c.Place(f.btn, geom.Pt(f.pad.Left, f.pad.Top))
	size := cs.Constrain(geom.Sz(w, f.pad.Top+bs.H+f.pad.Bottom))
	bg := scrimBg(c)
	if f.fade && (f.paint.Gradient == nil || f.paintH != size.H || f.paintBg != bg) {
		f.paintH, f.paintBg = size.H, bg
		f.paint = footFade(bg, size.H)
	}

	return size
}

// Paint draws the footer's ground, the button over it.
func (f *scrFoot) Paint(c *ui.Context, p *ui.Painter) {
	if f.fade {
		p.Fill(f.Bounds(), f.paint, 0)
	} else {
		p.Fill(f.Bounds(), scene.Solid(scrimBg(c)), 0)
	}
	f.Base.Paint(c, p)
}

// ====================================================================
// ========================= Helper Functions =========================
// ====================================================================

// footFade is `linear-gradient(transparent, var(--bg) 30%)` over h: --bg from
// nothing at the top to whole at 30 % and below. The start is --bg at no opacity,
// not black: CSS mixes the colors with their alpha premultiplied, so that the ramp
// never passes through grey.
func footFade(bg color.Color, h float32) scene.Paint {
	return scene.Linear(geom.Point{}, geom.Pt(0, h),
		scene.Stop{Color: bg.WithAlpha(0), Offset: 0},
		scene.Stop{Color: bg, Offset: .3},
		scene.Stop{Color: bg, Offset: 1})
}
