//go:build tvos

// Adapted from dancefitvn-app/internal/ui/mobile/screens/bg.go (radialBG): the
// `background: radial-gradient(...), var(--bg)` of a `.scr`, for the TV's
// 1920x1080 screens (index.html:645, :651, :694, :713).

package tvkit

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

// backdropScale is how many times smaller than the box the gradient is rendered
// before it is drawn stretched: a whole number, so that a circle stays round
// (kit.Radial), and large enough that the smooth ramp shows no steps.
const backdropScale = 4

// Backdrop is the background of a screen: the radial layers over --bg, filling
// the box. The image is rendered when the size changes, never per frame.
type Backdrop struct {
	ui.Base

	// Layers are the gradients, the topmost first.
	Layers []kit.Radial

	img  *scene.Image
	w, h int
}

// ====================================================================
// ========================== Main functions ==========================
// ====================================================================

// Layout fills the room.
func (b *Backdrop) Layout(_ *ui.Context, cs ui.Constraints) geom.Size { return cs.Constrain(cs.Max) }

// Paint draws the gradient over --bg.
func (b *Backdrop) Paint(c *ui.Context, p *ui.Painter) {
	r := b.Bounds()
	w, h := int(math.Round(float64(r.W)/backdropScale)), int(math.Round(float64(r.H)/backdropScale))
	if w <= 0 || h <= 0 {
		return
	}
	if b.img == nil || b.w != w || b.h != h {
		b.img = kit.RenderRadials(theme.From(c).T.Bg, b.Layers, w, h, b.img)
		b.w, b.h = w, h
	}
	p.Image(b.img, r, 1)
}
