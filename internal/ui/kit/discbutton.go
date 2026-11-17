package kit

import (
	"dancefitvn-app/internal/ui/icon"
	"dancefitvn-app/internal/ui/theme"

	"github.com/gdeskdev/desk/foundation/core/geom"
	"github.com/gdeskdev/desk/foundation/scene"
	"github.com/gdeskdev/desk/framework/ui"
)

// ====================================================================
// ============================ Contracts =============================
// ====================================================================

// DiscButton is a round button of one color with a centered glyph, as the play
// button of screen 06 is: `style="width:58px;height:58px;border-radius:50%;
// background:var(--pr);color:var(--on);padding:16px"` (index.html:429). Its
// glyph is the class-less PLAY or PAUSE svg, which layout.json does not record,
// so it has no name unless GlyphName says one.
type DiscButton struct {
	ui.Pressable
	// Icon is the glyph shown.
	Icon icon.Name
	// Size is the diameter; 0 reads as 58.
	Size float32
	// Padding is the room between the disc and the glyph on each side: the glyph
	// is Size - 2 Padding wide.
	Padding float32
	// Fill is the disc's color, Ink the glyph's.
	Fill, Ink theme.Ink
	// Label names the button for assistive technologies.
	Label string

	glyph    Icon
	attached bool
}

var _ ui.Labeler = (*DiscButton)(nil)

// The play button's numbers (index.html:429).
const (
	// discSize is `width:58px;height:58px`.
	discSize float32 = 58
	// discPadding is `padding:16px`.
	discPadding float32 = 16
)

// ====================================================================
// ============================= Creation =============================
// ====================================================================

// NewDiscButton returns the play button of screen 06 showing glyph: --pr under
// --on, 58 wide, padded 16.
func NewDiscButton(glyph icon.Name, label string) *DiscButton {
	return &DiscButton{Icon: glyph, Size: discSize, Padding: discPadding, Fill: theme.InkPrimary, Ink: theme.InkOnPrimary, Label: label}
}

// ====================================================================
// ========================== Main functions ==========================
// ====================================================================

// SetIcon changes the glyph; the same one changes nothing. The icon child takes
// it at once: its size does not change, so no layout follows, and a child left
// with the old glyph until the next layout drew PLAY as PAUSE on a screen where
// nothing else moves (a video paused, a load that failed).
func (b *DiscButton) SetIcon(n icon.Name) {
	if n != b.Icon {
		b.Icon = n
		b.glyph.Glyph = n
		b.Repaint()
	}
}

// AccessibleLabel returns the button's label.
func (b *DiscButton) AccessibleLabel() string { return b.Label }

// Layout takes the diameter and centers the glyph.
func (b *DiscButton) Layout(c *ui.Context, cs ui.Constraints) geom.Size {
	if !b.attached {
		b.attached = true
		b.Look = noLook
		b.Add(&b.glyph)
	}
	d := b.Size
	if d <= 0 {
		d = discSize
	}
	g := max(d-2*b.Padding, 0)
	b.glyph.Glyph, b.glyph.Size, b.glyph.Ink = b.Icon, g, b.Ink
	size := cs.Constrain(geom.Sz(d, d))
	c.Layout(&b.glyph, ui.Tight(geom.Sz(g, g)))
	c.Place(&b.glyph, geom.Pt(centeredY(size.W, g), centeredY(size.H, g)))

	return size
}

// Paint draws the disc; the glyph is its child.
func (b *DiscButton) Paint(c *ui.Context, p *ui.Painter) {
	look := theme.From(c)
	r := b.Bounds()
	p.Fill(r, scene.Solid(b.Fill.Of(look)), r.W/2)
	if b.FocusVisible() {
		focusRing(p, r, r.W/2, look.T.Primary)
	}
	b.Base.Paint(c, p)
}
