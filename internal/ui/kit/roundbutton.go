package kit

import (
	"dancefitvn-app/internal/ui/icon"
	"dancefitvn-app/internal/ui/theme"

	"github.com/gdeskdev/desk/foundation/core/color"
	"github.com/gdeskdev/desk/foundation/core/geom"
	"github.com/gdeskdev/desk/foundation/scene"
	"github.com/gdeskdev/desk/framework/ui"
)

// ====================================================================
// ============================ Contracts =============================
// ====================================================================

// The numbers of .rb (index.html:84-85).
const (
	// roundSize is `.rb{width:40px;height:40px}` (:84); the close button of a
	// sheet is 32 (:506).
	roundSize float32 = 40
	// ringWidth is the width of the Ring option's light ring (K35).
	ringWidth float32 = 1
)

// ringInk is the Ring option's colour: white at .3 (K35).
var ringInk = color.RGBA(255, 255, 255, .3)

// The names layout.json gives a round button without a data attribute.
const (
	nameRound     = "button.rb"
	nameRoundDark = "button.rb.dk"
)

// ====================================================================
// ============================= Creation =============================
// ====================================================================

// RoundButton is `.rb` (index.html:84-85): a 40 DIP circle of --sf with a
// centered icon (22 by default), or `.rb.dk`, black at .4 with a white icon.
// The back button of a screen and the close button of a sheet (32, --sf2, icon
// 18, :506) are RoundButtons. Set the Name to the data key (`back`,
// `closeSheet`) the prototype knows it by.
type RoundButton struct {
	ui.Pressable
	// Icon is the symbol shown.
	Icon icon.Name
	// Size is the diameter; 0 reads as 40.
	Size float32
	// IconSize is the symbol's size; 0 reads as 22.
	IconSize float32
	// IconStroke is the symbol's stroke; 0 is the default.
	IconStroke float32
	// Dark is `.dk`: black at .4 with a white icon.
	Dark bool
	// Ring adds a thin light ring (white at .3, 1 DIP, inside the disc) so a
	// dark button stays visible over black, as over a letterboxed video.
	// Not in the prototype: the owner's choice (K35).
	Ring bool
	// Bg replaces the background when not zero (--sf2 of the sheet's close).
	Bg color.Color
	// Label names the button for assistive technologies (the aria-label).
	Label string

	icon     Icon
	attached bool
}

var _ ui.Labeler = (*RoundButton)(nil)

// ====================================================================
// ========================== Main functions ==========================
// ====================================================================

// NewRoundButton returns a round button with the icon icn, `.dk` when dark.
func NewRoundButton(icn icon.Name, dark bool, label string) *RoundButton {
	return &RoundButton{Icon: icn, Dark: dark, Label: label}
}

// AccessibleLabel returns the button's label.
func (b *RoundButton) AccessibleLabel() string { return b.Label }

// Layout takes the diameter and centers the icon.
func (b *RoundButton) Layout(c *ui.Context, cs ui.Constraints) geom.Size {
	if !b.attached {
		b.attached = true
		b.Look = noLook
		b.icon.Name = iconName
		b.Add(&b.icon)
	}
	if b.Name == "" {
		b.Name = nameRound
		if b.Dark {
			b.Name = nameRoundDark
		}
	}
	d := b.Size
	if d <= 0 {
		d = roundSize
	}
	ink := theme.InkText
	if b.Dark {
		ink = theme.InkWhite
	}
	b.icon.Glyph, b.icon.Size, b.icon.Stroke, b.icon.Ink = b.Icon, b.IconSize, b.IconStroke, ink
	size := cs.Constrain(geom.Sz(d, d))
	is := c.Layout(&b.icon, ui.Loose(geom.Sz(ui.Unbounded, ui.Unbounded)))
	c.Place(&b.icon, geom.Pt(centeredY(size.W, is.W), centeredY(size.H, is.H)))

	return size
}

// Paint draws the disc; the icon is its child.
func (b *RoundButton) Paint(c *ui.Context, p *ui.Painter) {
	look := theme.From(c)
	bg := look.T.Surface
	if b.Dark {
		bg = look.C.RoundDark
	}
	r := b.Bounds()
	p.Fill(r, scene.Solid(orColor(b.Bg, bg)), r.W/2)
	if b.Ring {
		p.Border(r, geom.Uniform(ringWidth), ringInk, r.W/2)
	}
	if b.FocusVisible() {
		focusRing(p, r, r.W/2, look.T.Primary)
	}
	b.Base.Paint(c, p)
}
