//go:build tvos

package shell

import (
	"dancefitvn-app/internal/ui/theme"
	"dancefitvn-app/internal/ui/tv/env"

	"github.com/gdeskdev/desk/foundation/core/geom"
	"github.com/gdeskdev/desk/foundation/scene"
	"github.com/gdeskdev/desk/framework/ui"
)

// ====================================================================
// ============================ Contracts =============================
// ====================================================================

// Stage is the 1920x1080 stage of `#tv` (index.html:178): it paints --bg, then
// the host of the screen and the toast layer over it, each as large as the
// stage (`#tv .scr{position:absolute;inset:0}`). A screen that lets a video
// plane show through (env.Holder, the player) keeps its hole transparent: --bg is
// painted around it, since an opaque fill under the view would hide the plane.
type stage struct {
	ui.Base
}

// host holds the screen of the route showing: nothing, or one widget as large
// as the stage.
type host struct {
	ui.Base
}

// ====================================================================
// ========================== Main functions ==========================
// ====================================================================

// Layout gives every child the whole stage.
func (s *stage) Layout(c *ui.Context, cs ui.Constraints) geom.Size {
	size := cs.Constrain(cs.Max)
	for w := range s.Children() {
		c.Layout(w, ui.Tight(size))
		c.Place(w, geom.Point{})
	}

	return size
}

// Paint fills the stage with --bg, but for the hole of the screen showing, and
// paints the children.
func (s *stage) Paint(c *ui.Context, p *ui.Painter) {
	bg := scene.Solid(theme.From(c).T.Bg)
	r := s.Bounds()
	hole, ok := s.hole()
	if !ok {
		p.Fill(r, bg, 0)
		s.Base.Paint(c, p)

		return
	}
	// the four strips around the hole, in the stage's coordinates
	h := hole.Intersect(r)
	p.Fill(geom.XYWH(r.X, r.Y, r.W, h.Y-r.Y), bg, 0)
	p.Fill(geom.XYWH(r.X, h.Y+h.H, r.W, r.Y+r.H-h.Y-h.H), bg, 0)
	p.Fill(geom.XYWH(r.X, h.Y, h.X-r.X, h.H), bg, 0)
	p.Fill(geom.XYWH(h.X+h.W, h.Y, r.X+r.W-h.X-h.W, h.H), bg, 0)
	s.Base.Paint(c, p)
}

// hole returns the rectangle the screen showing keeps transparent, in the stage's
// coordinates, false when it has none.
func (s *stage) hole() (geom.Rect, bool) {
	for w := range s.Children() {
		h, ok := w.(*host)
		if !ok {
			continue
		}
		for sc := range h.Children() {
			if hr, ok := sc.(env.Holder); ok {
				r := hr.Hole()
				if !r.Empty() {
					o := s.WindowFrame().Min()

					return geom.XYWH(r.X-o.X, r.Y-o.Y, r.W, r.H), true
				}
			}
		}
	}

	return geom.Rect{}, false
}

// Layout gives the screen the whole stage.
func (h *host) Layout(c *ui.Context, cs ui.Constraints) geom.Size {
	size := cs.Constrain(cs.Max)
	for w := range h.Children() {
		c.Layout(w, ui.Tight(size))
		c.Place(w, geom.Point{})
	}

	return size
}
