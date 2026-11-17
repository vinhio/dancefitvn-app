//go:build tvos

package tvkit

import (
	"time"

	"dancefitvn-app/internal/ui/theme"

	"github.com/gdeskdev/desk/foundation/core/color"
	"github.com/gdeskdev/desk/foundation/core/geom"
	"github.com/gdeskdev/desk/foundation/scene"
	"github.com/gdeskdev/desk/framework/ui"
)

// ====================================================================
// ============================ Contracts =============================
// ====================================================================

// The numbers of `.tpanel` (index.html:197).
const (
	// PanelW, PanelInset are `.tpanel{right:60px;top:60px;bottom:60px;
	// width:640px}`.
	PanelW, PanelInset float32 = 640, 60
	// panelRadius, panelPad are `border-radius:36px;padding:44px`.
	panelRadius, panelPad float32 = 36, 44
	// panelDropY, panelDropBlur, panelDropAlpha are `box-shadow:0 30px 80px
	// rgba(0,0,0,.6)`.
	panelDropY, panelDropBlur, panelDropAlpha float32 = 30, 80, .6
	// PanelFade is `animation:fi .2s` (CSS `ease`); the panel plays it when it
	// opens only (K17).
	PanelFade = 200 * time.Millisecond
)

// namePanel is the key layout.json gives the panel: `div.tpanel`.
const namePanel = "div.tpanel"

// block is a child of the panel's flow with the margins CSS gives it.
type block struct {
	w                ui.Widget
	top, bottom, ind float32 // margin-top, margin-bottom, margin-left
}

// Panel is `.tpanel` (index.html:197): --sf, 36 rounded, 44 of padding, a drop
// shadow of 0 30 80, over everything. Its children flow as blocks, one under the
// other, with the vertical margins of CSS block layout: the margin between two
// siblings is the larger of the two, not their sum. The panel fades in over
// 200 ms when it joins the screen and has no fade out (K17: the prototype's
// replay of the animation at each re-render is not kept).
//
// The screen gives the panel its size (640 by the height between the 60 insets)
// and its place.
type Panel struct {
	ui.Base

	blocks []block
	alpha  ui.Animated[float32]

	// dimmed, c and p paint the panel under its layer while it fades in: made
	// once, so that a frame allocates nothing.
	dimmed func()
	c      *ui.Context
	p      *ui.Painter
}

var _ ui.Mounter = (*Panel)(nil)

// ====================================================================
// ============================= Creation =============================
// ====================================================================

// NewPanel returns an empty panel named `div.tpanel`.
func NewPanel() *Panel {
	p := &Panel{}
	p.Name = namePanel

	return p
}

// ====================================================================
// ========================== Main functions ==========================
// ====================================================================

// Put puts w at the end of the flow with the margins top, bottom and left
// (`margin:top 0 bottom left`).
func (p *Panel) Put(w ui.Widget, top, bottom, left float32) {
	p.blocks = append(p.blocks, block{w: w, top: top, bottom: bottom, ind: left})
	p.Base.Add(w)
}

// Mounted starts the fade in.
func (p *Panel) Mounted(*ui.Context) {
	p.alpha.Set(0)
	p.alpha.Duration, p.alpha.Ease = PanelFade, theme.CSSEase
	p.alpha.To(p, 1)
}

// Layout takes the size it is given and flows the blocks in the padding.
func (p *Panel) Layout(c *ui.Context, cs ui.Constraints) geom.Size {
	size := cs.Constrain(cs.Max)
	inner := size.W - 2*panelPad
	y, prev := panelPad, float32(0)
	for i, b := range p.blocks {
		gap := b.top
		if i > 0 {
			gap = max(prev, b.top) // adjoining margins collapse
		}
		y += gap
		w := inner - b.ind
		s := c.Layout(b.w, ui.Constraints{Min: geom.Sz(w, 0), Max: geom.Sz(w, ui.Unbounded)})
		c.Place(b.w, geom.Pt(panelPad+b.ind, y))
		y += s.H
		prev = b.bottom
	}

	return size
}

// Paint draws the panel, under a layer while it fades in.
func (p *Panel) Paint(c *ui.Context, pt *ui.Painter) {
	if !p.alpha.Running() || p.alpha.Value() >= 1 {
		p.paint(c, pt)

		return
	}
	if p.dimmed == nil {
		p.dimmed = func() { p.paint(p.c, p.p) }
	}
	p.c, p.p = c, pt
	pt.Layer(p.alpha.Value(), 0, p.dimmed)
	p.c, p.p = nil, nil
}

// ====================================================================
// ========================= Helper Functions =========================
// ====================================================================

// paint draws the shadow, the fill and the children.
func (p *Panel) paint(c *ui.Context, pt *ui.Painter) {
	r := p.Bounds()
	pt.Shadow(r, panelRadius, ui.Shadow{
		Offset: geom.Pt(0, panelDropY), Blur: panelDropBlur, Color: color.RGBA(0, 0, 0, panelDropAlpha),
	})
	pt.Fill(r, scene.Solid(theme.From(c).T.Surface), panelRadius)
	p.Base.Paint(c, pt)
}
