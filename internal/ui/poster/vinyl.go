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

// The colors of `.vinyl` (index.html:71).
var (
	// vinylHubColor is `#FF6A1A`, the label of the record.
	vinylHubColor = color.RGB(0xff, 0x6a, 0x1a)
	// vinylDark is `#1a1a1a`, the disc and the first and third inset shadow.
	vinylDark = color.RGB(0x1a, 0x1a, 0x1a)
	// vinylGroove is `#3a3a3a`, the second and fourth inset shadow: the thin
	// lines between the grooves.
	vinylGroove = color.RGB(0x3a, 0x3a, 0x3a)
)

// ====================================================================
// ========================== Main functions ==========================
// ====================================================================

// drawVinyl paints the record `.vinyl` of poster() (index.html:71, :294) in
// the box disc, a square. The CSS is a disc with a radial gradient (orange
// hub, then #1a1a1a) under four inset box-shadows, each a band of its spread
// along the edge, the first listed on top; so from the edge in the disc has 6
// px of #1a1a1a, a 1 px #3a3a3a line, #1a1a1a to 16 px, a 1 px line, then the
// hub's field. A band as wide as the disc's radius covers all of it, as an
// inset shadow does: a 44 px poster's small record is dark all through, with the
// one line at 6 px.
func drawVinyl(p *ui.Painter, disc geom.Rect) {
	r := disc.W / 2
	p.Fill(disc, scene.Solid(vinylDark), r)
	hub := vinylHub * r
	p.Fill(geom.XYWH(disc.X+r-hub, disc.Y+r-hub, 2*hub, 2*hub), scene.Solid(vinylHubColor), hub)
	band(p, disc, vinylBandLight2, vinylGroove)
	band(p, disc, vinylBandDark2, vinylDark)
	band(p, disc, vinylBandLight1, vinylGroove)
	band(p, disc, vinylBandDark1, vinylDark)
}

// ====================================================================
// ========================= Helper Functions =========================
// ====================================================================

// band paints `inset 0 0 0 spread c` over the disc: the ring of that width
// along its edge, or the whole disc when the spread reaches its center.
func band(p *ui.Painter, disc geom.Rect, spread float32, c color.Color) {
	r := disc.W / 2
	if spread >= r {
		p.Fill(disc, scene.Solid(c), r)

		return
	}
	p.Border(disc, geom.Insets{Top: spread, Right: spread, Bottom: spread, Left: spread}, c, r)
}
