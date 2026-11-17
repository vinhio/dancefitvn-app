package kit

import (
	"math"

	"dancefitvn-app/internal/ui/theme"

	"github.com/gdeskdev/desk/foundation/core/geom"
	"github.com/gdeskdev/desk/foundation/scene"
	"github.com/gdeskdev/desk/framework/ui"
)

// ====================================================================
// ============================ Contracts =============================
// ====================================================================

// The numbers of the progress ring of MS.done (index.html:445).
const (
	// RingSize is the ring's `width:64px;height:64px`.
	RingSize float32 = 64
	// ringInner is the hole's `width:48px;height:48px`.
	ringInner float32 = 48
	// ringText is the hole's `font-size:13px;font-weight:800`.
	ringText float32 = 13
	// ringArc is the widest arc one cubic makes, a quarter circle: the control
	// distance of an arc of angle a is 4/3 tan(a/4) of the radius.
	ringArc = math.Pi / 2
)

// Ring is `<div style="width:64px;height:64px;border-radius:50%;
// background:conic-gradient(var(--pr) Xdeg, var(--sf2) 0)">` with the 48 DIP
// disc of --sf and its text in the middle: the share of the circle from the top,
// clockwise, in --pr over --sf2. gDesk has no conic gradient (F-16 of
// local/plans/P4.md), so the share is a pie wedge filled from cubic arcs, built
// when the share changes.
type Ring struct {
	ui.Base

	label Label
	share float32
	wedge scene.Path
	built bool
	attch bool
}

// NewRing returns a ring at share 0 (all --sf2) with the text in its hole.
func NewRing(text string) *Ring {
	r := &Ring{}
	r.label = Label{Size: ringText, Weight: 800, Align: ui.TextAlignCenter}
	r.label.SetText(text)

	return r
}

// ====================================================================
// ========================== Main functions ==========================
// ====================================================================

// SetShare sets the share of the circle in --pr, 0 to 1; the same one changes
// nothing.
func (r *Ring) SetShare(s float32) {
	s = clamp01(s)
	if s != r.share {
		r.share = s
		r.built = false
		r.Repaint()
	}
}

// Share returns the share in --pr.
func (r *Ring) Share() float32 { return r.share }

// SetText changes the text in the hole; the same one changes nothing.
func (r *Ring) SetText(s string) {
	if r.label.Text != s {
		r.label.SetText(s)
		r.Relayout()
	}
}

// Text returns the text in the hole.
func (r *Ring) Text() string { return r.label.Text }

// Layout makes the ring 64 square and centers the text in the 48 hole.
func (r *Ring) Layout(c *ui.Context, cs ui.Constraints) geom.Size {
	if !r.attch {
		r.attch = true
		r.Add(&r.label)
	}
	size := cs.Constrain(geom.Sz(RingSize, RingSize))
	ls := c.Layout(&r.label, ui.Loose(geom.Sz(ringInner, ui.Unbounded)))
	c.Place(&r.label, geom.Pt(centeredY(size.W, ls.W), centeredY(size.H, ls.H)))

	return size
}

// Paint draws the --sf2 disc, the --pr wedge over it, the --sf hole, and the text
// as the child.
func (r *Ring) Paint(c *ui.Context, p *ui.Painter) {
	t := &theme.From(c).T
	b := r.Bounds()
	p.Fill(b, scene.Solid(t.Surface2), b.W/2)
	switch {
	case r.share >= 1:
		p.Fill(b, scene.Solid(t.Primary), b.W/2)
	case r.share > 0:
		if !r.built {
			r.build(b.W / 2)
		}
		p.FillPath(&r.wedge, scene.Solid(t.Primary), scene.NonZero)
	default: // nothing done: the disc alone
	}
	hole := geom.XYWH((b.W-ringInner)/2, (b.H-ringInner)/2, ringInner, ringInner)
	p.Fill(hole, scene.Solid(t.Surface), ringInner/2)
	r.Base.Paint(c, p)
}

// ====================================================================
// ========================= Helper Functions =========================
// ====================================================================

// build makes the wedge of the share: from the center to the top, along the
// circle of radius rad clockwise through share x 360 degrees, back to the
// center. The path's memory is reused.
func (r *Ring) build(rad float32) {
	r.built = true
	r.wedge.Reset()
	cx, cy := rad, rad
	r.wedge.MoveTo(cx, cy)
	r.wedge.LineTo(cx, cy-rad)
	total := float64(r.share) * 2 * math.Pi
	from := -math.Pi / 2 // the top: conic-gradient starts there and goes clockwise
	for left := total; left > 1e-9; {
		step := min(left, ringArc)
		to := from + step
		k := 4 / 3 * math.Tan(step/4) * float64(rad)
		sx, sy := math.Cos(from), math.Sin(from)
		ex, ey := math.Cos(to), math.Sin(to)
		fr := float64(rad)
		r.wedge.CubicTo(
			float32(float64(cx)+fr*sx-k*sy), float32(float64(cy)+fr*sy+k*sx),
			float32(float64(cx)+fr*ex+k*ey), float32(float64(cy)+fr*ey-k*ex),
			float32(float64(cx)+fr*ex), float32(float64(cy)+fr*ey))
		from, left = to, left-step
	}
	r.wedge.Close()
}
