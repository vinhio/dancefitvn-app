//go:build tvos

package tvkit

import (
	"dancefitvn-app/internal/ui/icon"
	"dancefitvn-app/internal/ui/kit"
	"dancefitvn-app/internal/ui/theme"

	"github.com/gdeskdev/desk/bridge/text"
	"github.com/gdeskdev/desk/foundation/core/color"
	"github.com/gdeskdev/desk/foundation/core/geom"
	"github.com/gdeskdev/desk/foundation/scene"
	"github.com/gdeskdev/desk/framework/ui"
)

// ====================================================================
// ============================ Contracts =============================
// ====================================================================

// ButtonSize is the size of a Button: the default `.tbtn` or one of the two
// inline overrides the prototype writes.
type ButtonSize uint8

// The sizes. ButtonSizeUnknown reads as ButtonLarge.
const (
	ButtonSizeUnknown ButtonSize = iota
	// ButtonLarge is `.tbtn`: height 88, padding 0 44, 30 px (:189).
	ButtonLarge
	// ButtonSmall is `style="height:76px;padding:0 34px;font-size:26px"` of
	// `p1` and `p2` (:687).
	ButtonSmall
	// ButtonTiny is `style="font-size:24px;height:70px;padding:0 30px"` of
	// `c1` and `c2` (:658, :659).
	ButtonTiny
)

// The numbers of `.tbtn` (index.html:189-191).
const (
	// btnGap is `gap:14px`.
	btnGap float32 = 14
	// btnRadius is `border-radius:44px`, which the smaller sizes keep: the
	// radius is capped at half the height, a pill.
	btnRadius float32 = 44
	// btnWeight is `font-weight:700`.
	btnWeight text.Weight = 700
	// btnIcon is `.tbtn svg.i{width:34px;height:34px}`.
	btnIcon float32 = 34
)

// btnSpec is what a size sets: the inline styles of the prototype.
type btnSpec struct {
	height, padX, font float32
}

// btnSpecs are the three sizes: (:189), (:687), (:658).
var btnSpecs = [...]btnSpec{
	ButtonSizeUnknown: {88, 44, 30},
	ButtonLarge:       {88, 44, 30},
	ButtonSmall:       {76, 34, 26},
	ButtonTiny:        {70, 30, 24},
}

// Colors of `.tbtn`: the translucent white of the rest state (`:189`).
var btnRest = color.RGBA(255, 255, 255, .12)

// Button is `.f.tbtn` (index.html:189-191): an inline-flex pill, its optional
// icon and its text 14 apart, as wide as they are with the padding. At rest it
// is rgba(255,255,255,.12) with --tx text; focused it is --pr with --on text,
// a shadow of 0 26 50 rgba(0,0,0,.6) with no ring, scaled to 1.06.
//
// Set Key to the `data-fid` (it is also the Name layout.json knows the button
// by) and OnSelect to the `data-tact`.
type Button struct {
	Item
	// Text is the label.
	Text string
	// Icon is the symbol before the text; icon.NameUnknown for none.
	Icon icon.Name
	// IconSize is the symbol's size; 0 reads as 34 (`.tbtn svg.i`).
	IconSize float32
	// Size picks the height, padding and font.
	Size ButtonSize
	// PlainIcon is an icon that is a class-less `<span style="width:30px">`
	// holding the PLAY svg (`h1`, `d1`, :665, :677): layout.json does not record
	// it, so the glyph carries no name.
	PlainIcon bool

	label    kit.Label
	glyph    kit.Icon
	attached bool
	body     func(c *ui.Context, p *ui.Painter) // paintBody as a method value, made once
}

var _ ui.Labeler = (*Button)(nil)

// ====================================================================
// ============================= Creation =============================
// ====================================================================

// NewButton returns a `.tbtn` with the key, the text and the symbol icn (or
// icon.NameUnknown).
func NewButton(key, txt string, icn icon.Name) *Button {
	b := &Button{Text: txt, Icon: icn}
	b.Key, b.Name, b.Label = key, key, txt
	b.FocusScale = FocusScale

	return b
}

// ====================================================================
// ========================== Main functions ==========================
// ====================================================================

// SetText changes the label, laying the button out again; the same text
// changes nothing.
func (b *Button) SetText(s string) {
	if s != b.Text {
		b.Text, b.Item.Label = s, s
		b.Relayout()
	}
}

// IconRect returns where the icon is, in the button's coordinates, as of the
// last layout; the zero rectangle when there is none.
func (b *Button) IconRect() geom.Rect {
	if b.Icon == icon.NameUnknown {
		return geom.Rect{}
	}

	return b.glyph.Frame()
}

// Layout sizes the button around its content (`display:inline-flex`) and
// centers the icon and the text on its height.
func (b *Button) Layout(c *ui.Context, cs ui.Constraints) geom.Size {
	if !b.attached {
		b.attached = true
		b.glyph.Name = "svg.i"
		b.Add(&b.glyph, &b.label)
	}
	spec := btnSpecs[b.Size]
	b.label.Role, b.label.Weight = theme.RoleBody, btnWeight
	if b.label.Size != spec.font {
		b.label.Size = spec.font
		b.label.Relayout()
	}
	b.label.SetText(b.Text)
	has := b.Icon != icon.NameUnknown
	b.glyph.Glyph, b.glyph.Size = b.Icon, b.iconSize()
	b.glyph.Name = "svg.i"
	if b.PlainIcon {
		b.glyph.Name = ""
	}
	b.glyph.Hidden = !has // inside the button's own Layout: no parent to tell
	content := float32(0)
	var ts, is geom.Size
	if b.Text != "" {
		ts = c.Layout(&b.label, ui.Loose(geom.Sz(ui.Unbounded, ui.Unbounded)))
		content = ts.W
	}
	if has {
		is = c.Layout(&b.glyph, ui.Tight(geom.Sz(b.glyph.Size, b.glyph.Size)))
		content += is.W
		if b.Text != "" {
			content += btnGap
		}
	}
	size := cs.Constrain(geom.Sz(content+2*spec.padX, spec.height))
	x := (size.W - content) / 2
	if has {
		c.Place(&b.glyph, geom.Pt(x, (size.H-is.H)/2))
		x += is.W + btnGap*flag(b.Text != "")
	}
	if b.Text != "" {
		c.Place(&b.label, geom.Pt(x, (size.H-ts.H)/2))
	}

	return size
}

// Paint draws the button at its focus scale.
func (b *Button) Paint(c *ui.Context, p *ui.Painter) {
	if b.body == nil {
		b.body = b.paintBody
	}
	b.Item.Scaled(c, p, b.body)
}

// LayoutStyle is `div.f.tbtn`: the text style, the fill and the radius.
func (b *Button) LayoutStyle(c *ui.Context) (kit.CSS, string) {
	look := theme.From(c)
	s, txt := b.label.LayoutStyle(c)
	s.Color = kit.CSSColor(b.ink().Of(look))
	s.Background = kit.CSSColor(b.fill(look))
	s.Radius = "44px"

	return s, txt
}

// ====================================================================
// ========================= Helper Functions =========================
// ====================================================================

// paintBody draws the shadow, the fill and the content.
func (b *Button) paintBody(c *ui.Context, p *ui.Painter) {
	look := theme.From(c)
	r := b.Bounds()
	radius := min(btnRadius, r.H/2)
	FocusShadow(p, r, radius, b.Blend(), color.Color{})
	p.Fill(r, scene.Solid(b.fill(look)), radius)
	ink := b.ink()
	b.label.Ink, b.glyph.Ink = ink, ink
	b.Base.Paint(c, p)
}

// fill is the background: --pr focused, the translucent white at rest. It
// switches at once with the `.focus` class: `background` is not in the
// transition list (:180).
func (b *Button) fill(look *theme.Look) color.Color {
	if b.IsFocused() {
		return look.T.Primary
	}

	return btnRest
}

// ink is the color of the text and the icon: --on focused, --tx at rest.
func (b *Button) ink() theme.Ink {
	if b.IsFocused() {
		return theme.InkOnPrimary
	}

	return theme.InkText
}

// iconSize is the symbol's size: 34 unless set.
func (b *Button) iconSize() float32 {
	if b.IconSize > 0 {
		return b.IconSize
	}

	return btnIcon
}

// flag is 1 for true and 0 for false.
func flag(v bool) float32 {
	if v {
		return 1
	}

	return 0
}
