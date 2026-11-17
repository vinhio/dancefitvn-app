//go:build !tvos

package screens

import (
	"math"

	"dancefitvn-app/internal/ui/kit"
	"dancefitvn-app/internal/ui/theme"

	"github.com/gdeskdev/desk/foundation/core/geom"
	"github.com/gdeskdev/desk/foundation/scene"
	"github.com/gdeskdev/desk/framework/ui"
)

// ====================================================================
// ============================ Contracts =============================
// ====================================================================

// radialScale is how many times smaller than the box the gradient is rendered
// before it is drawn stretched: a whole number, so that a circle stays round
// (kit.Radial), and large enough that the smooth ramp shows no steps.
const radialScale = 4

// radialBG is the `background: radial-gradient(...), var(--bg)` of a `.scr`
// (index.html:328, :339): the layers over --bg, filling the box. The image is
// rendered when the size changes, never per frame.
type radialBG struct {
	ui.Base

	layers []kit.Radial
	img    *scene.Image
	w, h   int
}

// ====================================================================
// ========================== Main functions ==========================
// ====================================================================

// Layout fills the room.
func (b *radialBG) Layout(_ *ui.Context, cs ui.Constraints) geom.Size { return cs.Constrain(cs.Max) }

// Paint draws the gradient over --bg.
func (b *radialBG) Paint(c *ui.Context, p *ui.Painter) {
	r := b.Bounds()
	w, h := int(math.Round(float64(r.W)/radialScale)), int(math.Round(float64(r.H)/radialScale))
	if w <= 0 || h <= 0 {
		return
	}
	if b.img == nil || b.w != w || b.h != h {
		b.img = kit.RenderRadials(theme.From(c).T.Bg, b.layers, w, h, b.img)
		b.w, b.h = w, h
	}
	p.Image(b.img, r, 1)
}
