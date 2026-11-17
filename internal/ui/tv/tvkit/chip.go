//go:build tvos

package tvkit

import (
	"dancefitvn-app/internal/ui/kit"
	"dancefitvn-app/internal/ui/theme"

	"github.com/gdeskdev/desk/bridge/text"
	"github.com/gdeskdev/desk/foundation/core/color"
	"github.com/gdeskdev/desk/foundation/core/geom"
	"github.com/gdeskdev/desk/foundation/platform"
	"github.com/gdeskdev/desk/foundation/scene"
	"github.com/gdeskdev/desk/framework/ui"
)

// ====================================================================
// ============================ Contracts =============================
// ====================================================================

// The numbers of a filter chip `cf*` (TS.courses, index.html:673).
const (
	// chipHeight, chipPadX, chipRadius, chipSize are `style="height:56px;
	// padding:0 28px;border-radius:28px;font-size:26px;font-weight:700"`.
	chipHeight, chipPadX, chipRadius, chipSize float32 = 56, 28, 28, 26
	// chipBorder is `border:2px solid var(--bd)` of a chip that is not selected;
	// the selected one has no border, so it is 4 narrower.
	chipBorder float32 = 2
	// chipWeight is `font-weight:700`.
	chipWeight text.Weight = 700
)

// FilterChip is `.f` of a filter `cf*`: a pill of 56, its text 26/700, padded
// 28. The selected chip is --pr with --on text and no border; the others are
// --sf with --tx text and a 2 px --bd border. The fill is an inline style, so
// it stays when the chip is focused (F-9, `PrototypeInlineFocus`): the focus only
// adds the ring of 6 in --pr, the shadow and the scale (`#tv .f.focus`, :181).
//
// Set Key to the `data-fid` (it is also the Name layout.json knows the chip by),
// Selected to the filter in force and OnSelect to `data-tact="tfilter"`.
type FilterChip struct {
	Item
	// Text is the label, "Tất cả · 15".
	Text string
	// Selected is the chip of the filter in force; change it with SetSelected.
	Selected bool

	label kit.Label
	body  func(c *ui.Context, p *ui.Painter) // paintBody as a method value, made once
}

// ====================================================================
// ============================= Creation =============================
// ====================================================================

// NewFilterChip returns a chip of key showing txt.
func NewFilterChip(key, txt string) *FilterChip {
	c := &FilterChip{Text: txt}
	c.Key, c.Name, c.Item.Label = key, key, txt
	c.FocusScale = FocusScale
	c.label.Role, c.label.Size, c.label.Weight = theme.RoleBody, chipSize, chipWeight
	c.Add(&c.label)

	return c
}

// ====================================================================
// ========================== Main functions ==========================
// ====================================================================

// SetText changes the label; the same text changes nothing.
func (c *FilterChip) SetText(s string) {
	if s != c.Text {
		c.Text, c.Item.Label = s, s
		c.Relayout()
	}
}

// Describe describes the chip as a radio button named by its text and count
// ("Tất cả · 15"), selected while it is the filter in force. It sets no Toggle:
// the tvOS bridge turns a toggle into a "toggle button" that speaks 0 or 1
// (P8.3d dump); a filter is a button that is selected.
func (c *FilterChip) Describe(ctx *ui.Context, n *platform.SemanticNode) {
	c.Item.Describe(ctx, n)
	n.Role = platform.RoleRadio
	if c.Selected {
		n.Flags |= platform.FlagSelected
	}
}

// SetSelected makes the chip the filter in force, or not; the border goes with
// it, so the chip is laid out again.
func (c *FilterChip) SetSelected(on bool) {
	if on != c.Selected {
		c.Selected = on
		c.Relayout()
	}
}

// Layout sizes the chip around its text: 28 each side, and 2 more for the border
// of a chip that is not selected.
func (c *FilterChip) Layout(ctx *ui.Context, cs ui.Constraints) geom.Size {
	c.label.SetText(c.Text)
	ts := ctx.Layout(&c.label, ui.Loose(geom.Sz(ui.Unbounded, ui.Unbounded)))
	border := c.border()
	size := cs.Constrain(geom.Sz(ts.W+2*(chipPadX+border), chipHeight))
	ctx.Place(&c.label, geom.Pt((size.W-ts.W)/2, (size.H-ts.H)/2))

	return size
}

// Paint draws the chip at its focus scale.
func (c *FilterChip) Paint(ctx *ui.Context, p *ui.Painter) {
	if c.body == nil {
		c.body = c.paintBody
	}
	c.Item.Scaled(ctx, p, c.body)
}

// LayoutStyle is `div.f` of the chip: the text style, the fill, the radius and
// the border.
func (c *FilterChip) LayoutStyle(ctx *ui.Context) (kit.CSS, string) {
	look := theme.From(ctx)
	s, txt := c.label.LayoutStyle(ctx)
	s.Color = kit.CSSColor(c.ink().Of(look))
	s.Background = kit.CSSColor(c.fill(look))
	s.Radius = "28px"

	return s, txt
}

// ====================================================================
// ========================= Helper Functions =========================
// ====================================================================

// border is 2 for a chip that is not selected, 0 for the selected one.
func (c *FilterChip) border() float32 {
	if c.Selected {
		return 0
	}

	return chipBorder
}

// paintBody draws the ring and shadow, the fill, the border and the text.
func (c *FilterChip) paintBody(ctx *ui.Context, p *ui.Painter) {
	look := theme.From(ctx)
	r := c.Bounds()
	FocusShadow(p, r, chipRadius, c.Blend(), look.T.Primary)
	p.Fill(r, scene.Solid(c.fill(look)), chipRadius)
	if b := c.border(); b > 0 {
		p.Border(r, geom.Insets{Top: b, Right: b, Bottom: b, Left: b}, look.T.Border, chipRadius)
	}
	c.label.Ink = c.ink()
	c.Base.Paint(ctx, p)
}

// fill is --pr for the selected chip and --sf for the others, at rest and
// focused alike (the inline style beats `.focus`); with PrototypeInlineFocus
// off the focused chip is --pr.
func (c *FilterChip) fill(look *theme.Look) color.Color {
	if c.Selected || (!PrototypeInlineFocus && c.IsFocused()) {
		return look.T.Primary
	}

	return look.T.Surface
}

// ink is --on on --pr and --tx on --sf.
func (c *FilterChip) ink() theme.Ink {
	if c.Selected || (!PrototypeInlineFocus && c.IsFocused()) {
		return theme.InkOnPrimary
	}

	return theme.InkText
}
