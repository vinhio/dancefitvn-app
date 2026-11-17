package poster

import (
	"github.com/gdeskdev/desk/foundation/core/color"
	"github.com/gdeskdev/desk/foundation/core/geom"
	"github.com/gdeskdev/desk/foundation/scene"
	"github.com/gdeskdev/desk/framework/ui"
)

// ====================================================================
// ============================ Contracts =============================
// ====================================================================

// The DANCER of index.html:242, a `viewBox="0 0 100 100"` drawing:
// `<circle cx="52" cy="17" r="9" fill="${c}"/>` and a path of five
// polylines with round caps and joins.
const (
	dancerBox float32 = 100 // the viewBox's side
	headX     float32 = 52  // cx
	headY     float32 = 17  // cy
	headR     float32 = 9   // r
)

// dancerLines are the polylines of the stroked path, as the `d` attribute
// writes them: "M50 30 L46 58  M48 36 L28 22  M48 36 L70 42 L84 28
// M46 58 L30 78 L20 96  M46 58 L62 76 L78 92".
var dancerLines = [...][]struct{ x, y float32 }{
	{{50, 30}, {46, 58}},
	{{48, 36}, {28, 22}},
	{{48, 36}, {70, 42}, {84, 28}},
	{{46, 58}, {30, 78}, {20, 96}},
	{{46, 58}, {62, 76}, {78, 92}},
}

// ====================================================================
// ============================= Creation =============================
// ====================================================================

// dancer is one figure of a poster as paths in the poster's coordinates,
// made once per size, since a poster is redrawn every frame it scrolls.
//
// Both dancers face the same way. poster() asks for the first one flipped
// (DANCER(c.ink,true,7), index.html:295) but then replaces `class="dn"` by a
// second `style` attribute on a tag that already has the flip's `style`; the
// browser keeps the first attribute, `height:100%`, and drops the flip. The
// prototype's screenshots show two unflipped dancers, and the code, not the
// intention, is the specification.
type dancer struct {
	head scene.Path // the filled circle
	body scene.Path // the stroked polylines
}

// ====================================================================
// ========================== Main functions ==========================
// ====================================================================

// build makes d the figure of side side at (x, y) of the poster. It keeps the
// memory d had.
func (d *dancer) build(x, y, side float32) {
	k := side / dancerBox
	at := func(px, py float32) (float32, float32) { return x + px*k, y + py*k }

	d.head.Reset()
	cx, cy := at(headX, headY)
	d.head.Circle(cx, cy, headR*k)

	d.body.Reset()
	for _, line := range dancerLines {
		for i, pt := range line {
			px, py := at(pt.x, pt.y)
			if i == 0 {
				d.body.MoveTo(px, py)
			} else {
				d.body.LineTo(px, py)
			}
		}
	}
}

// stroke is the line of the body for a figure of side side: stroke-width 7
// of the 100 unit box, round caps and joins.
func stroke(side float32) scene.Stroke {
	return scene.Stroke{Width: figStroke * side / dancerBox, Cap: scene.CapRound, Join: scene.JoinRound}
}

// ====================================================================
// ========================= Helper Functions =========================
// ====================================================================

// unitDancer is the figure in its own 100 unit box, made once: DrawDancer
// scales it to the box it is asked for, so that a frame builds no path.
var unitDancer = func() *dancer {
	d := new(dancer)
	d.build(0, 0, dancerBox)

	return d
}()

// DrawDancer draws the DANCER of index.html:242, `DANCER(ink, false, stroke)`,
// in the widget's own coordinates: a square of side min(r.W, r.H) centred in r
// (`<svg viewBox="0 0 100 100" style="height:100%">`), the head a filled circle
// and the body five polylines with round caps and joins, stroke units wide of
// the 100 unit box (the splash's `DANCER('#FFD028',false,9)`, :331). It draws at
// full opacity: the caller sets the group's (the splash's `opacity:.12`) with
// Painter.Layer. Nothing is allocated once the first frame has run.
func DrawDancer(p *ui.Painter, r geom.Rect, ink color.Color, stroke float32) {
	side := min(r.W, r.H)
	if side <= 0 {
		return
	}
	k := side / dancerBox
	at := geom.Pt(r.X+(r.W-side)/2, r.Y+(r.H-side)/2)
	paint := scene.Solid(ink)
	p.Transform(scene.Affine{XX: k, YY: k, X0: at.X, Y0: at.Y}, func() {
		p.FillPath(&unitDancer.head, paint, scene.NonZero)
		p.StrokePath(&unitDancer.body, scene.Stroke{Width: stroke, Cap: scene.CapRound, Join: scene.JoinRound}, paint)
	})
}
