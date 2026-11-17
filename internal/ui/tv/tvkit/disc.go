//go:build tvos

package tvkit

import (
	"dancefitvn-app/internal/ui/icon"
	"dancefitvn-app/internal/ui/kit"
	"dancefitvn-app/internal/ui/theme"

	"github.com/gdeskdev/desk/foundation/core/geom"
	"github.com/gdeskdev/desk/foundation/platform"
	"github.com/gdeskdev/desk/foundation/scene"
	"github.com/gdeskdev/desk/framework/ui"
)

// ====================================================================
// ============================ Contracts =============================
// ====================================================================

// The numbers of the music screen's play disc `mu1` (index.html:698):
// `style="width:128px;height:128px;border-radius:50%;background:var(--pr);
// color:var(--on);padding:36px"`.
const (
	// DiscSize is the width and the height.
	DiscSize float32 = 128
	// discPad is the padding around the glyph.
	discPad float32 = 36
)

// Disc is `.f` round, --pr under an --on PLAY or PAUSE glyph (`mu1`). It takes
// the generic `.f.focus` look: scaled to 1.06 with the ring of 6 in --pr and the
// drop shadow of 0 26 50 (the ring is the disc's own color, so it reads as a
// larger disc).
type Disc struct {
	Item
	// PlayLabel and PauseLabel name the disc for a screen reader while it shows
	// the PLAY or the PAUSE glyph: it has no text of its own.
	PlayLabel, PauseLabel string

	glyph kit.Icon
	body  func(c *ui.Context, p *ui.Painter) // paintBody as a method value, made once
}

// ====================================================================
// ============================= Creation =============================
// ====================================================================

// NewDisc returns the disc of key showing the glyph icn (icon.Play or
// icon.Pause).
func NewDisc(key string, icn icon.Name) *Disc {
	d := &Disc{}
	d.Key, d.Name, d.Item.Label = key, key, key
	d.FocusScale = FocusScale
	d.glyph.Glyph, d.glyph.Ink = icn, theme.InkOnPrimary
	d.Add(&d.glyph)

	return d
}

// ====================================================================
// ========================== Main functions ==========================
// ====================================================================

// Describe describes the disc as a button named for what a press does: "Tạm
// dừng" while it shows PAUSE, "Phát" otherwise.
func (d *Disc) Describe(c *ui.Context, n *platform.SemanticNode) {
	d.Item.Describe(c, n)
	n.Label = d.PlayLabel
	if d.glyph.Glyph == icon.Pause {
		n.Label = d.PauseLabel
	}
}

// SetGlyph changes the glyph; the same one changes nothing.
func (d *Disc) SetGlyph(icn icon.Name) { d.glyph.SetGlyph(icn) }

// Glyph returns the glyph shown.
func (d *Disc) Glyph() icon.Name { return d.glyph.Glyph }

// Layout is 128 by 128 with the glyph in the padding.
func (d *Disc) Layout(c *ui.Context, cs ui.Constraints) geom.Size {
	size := cs.Constrain(geom.Sz(DiscSize, DiscSize))
	g := size.W - 2*discPad
	d.glyph.Size = g
	c.Layout(&d.glyph, ui.Tight(geom.Sz(g, g)))
	c.Place(&d.glyph, geom.Pt(discPad, discPad))

	return size
}

// Paint draws the disc at its focus scale.
func (d *Disc) Paint(c *ui.Context, p *ui.Painter) {
	if d.body == nil {
		d.body = d.paintBody
	}
	d.Item.Scaled(c, p, d.body)
}

// LayoutStyle is `div.f` of the disc: the fill and the round radius.
func (d *Disc) LayoutStyle(c *ui.Context) (kit.CSS, string) {
	look := theme.From(c)

	return kit.CSS{Background: kit.CSSColor(look.T.Primary), Radius: "50%", Color: kit.CSSColor(look.T.OnPrimary)}, ""
}

// ====================================================================
// ========================= Helper Functions =========================
// ====================================================================

// paintBody draws the ring and the shadow, the disc and its glyph.
func (d *Disc) paintBody(c *ui.Context, p *ui.Painter) {
	look := theme.From(c)
	r := d.Bounds()
	FocusShadow(p, r, r.W/2, d.Blend(), look.T.Primary)
	p.Fill(r, scene.Solid(look.T.Primary), r.W/2)
	d.Base.Paint(c, p)
}
