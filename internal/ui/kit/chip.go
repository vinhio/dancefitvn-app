// Adapted from jplayer-mobile/internal/ui/kit/chip.go at 49de3c2.

package kit

import (
	"dancefitvn-app/internal/ui/icon"
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

// ChipKind is the look of a Chip: the class after `.chip`.
type ChipKind uint8

// The kinds of chip. ChipKindUnknown is the bare `.chip`, which has no color of
// its own: --tx text on nothing, for the overrides to dress.
const (
	ChipKindUnknown ChipKind = iota
	// ChipTh is `.chip.c-th`: the regular courses, #FF6B6E on red at .18.
	ChipTh
	// ChipDb is `.chip.c-db`: the special courses, --info on blue at .18.
	ChipDb
	// ChipGt is `.chip.c-gt`: the introductions, --ok on green at .16.
	ChipGt
	// ChipY is `.chip.y`: --on on --pr.
	ChipY
	// ChipK is `.chip.k`: #fff on black at .55.
	ChipK
	// ChipOkc is `.chip.okc`: --ok on green at .16.
	ChipOkc
)

// The numbers of .chip (index.html:50-56).
const (
	// chipHeight, chipPad, chipRadius, chipGap, chipSize are `.chip{gap:5px;
	// height:22px;padding:0 9px;border-radius:11px;font-size:11px}` (:50).
	chipHeight, chipPad, chipRadius, chipGap, chipSize float32 = 22, 9, 11, 5, 11
	// chipWeight is `.chip{font-weight:700}` (:50).
	chipWeight text.Weight = 700
)

// The names layout.json gives a chip: tag and classes.
const (
	nameChip    = "span.chip"
	nameChipTh  = "span.chip.c-th"
	nameChipDb  = "span.chip.c-db"
	nameChipGt  = "span.chip.c-gt"
	nameChipY   = "span.chip.y"
	nameChipK   = "span.chip.k"
	nameChipOkc = "span.chip.okc"
)

// ====================================================================
// ============================= Creation =============================
// ====================================================================

// Chip is `.chip` (index.html:50-56): an inline pill 22 tall, padded 9, with an
// optional icon and 11/700 text on one line, 5 apart. The inline styles the
// prototype gives some chips are fields: Height, Radius, PadX, Size, Weight,
// Spacing, Bg, Fg. The text is shown as given (the prototype writes THƯỜNG
// upper case in the source, or calls toUpperCase()).
type Chip struct {
	ui.Base
	// Kind picks the colors.
	Kind ChipKind
	// Text is the label.
	Text string
	// Icon is the symbol before the text; icon.NameUnknown for none.
	Icon icon.Name
	// IconSize and IconStroke are `ic(n, size, stroke)`: the size 0 reads as
	// 22, the stroke 0 as the default.
	IconSize, IconStroke float32
	// Height, Radius, PadX, Size, Weight and Spacing override the rule's 22, 11,
	// 9, 11, 700 and 0 when not zero (`height:28px`, `padding:0 14px`,
	// `font-weight:800`, `letter-spacing:1px`, :357, :398, :489).
	Height, Radius, PadX, Size float32
	Weight                     text.Weight
	Spacing                    float32
	// Bg and Fg replace the kind's background and text color when not zero
	// (`background:#2F6FE0;color:#fff`, :489).
	Bg, Fg color.Color

	label    Label
	attached bool
	iconAt   geom.Point
}

var (
	_ ui.Labeler   = (*Chip)(nil)
	_ ui.Describer = (*Chip)(nil)
)

// ====================================================================
// ========================== Main functions ==========================
// ====================================================================

// NewChip returns a chip of kind k showing text.
func NewChip(k ChipKind, text string) *Chip { return &Chip{Kind: k, Text: text} }

// SetText changes the label, laying the chip out again; the same text changes
// nothing.
func (b *Chip) SetText(s string) {
	if s != b.Text {
		b.Text = s
		b.Relayout()
	}
}

// AccessibleLabel returns the chip's text.
func (b *Chip) AccessibleLabel() string { return b.Text }

// Describe describes the chip as text.
func (b *Chip) Describe(_ *ui.Context, n *platform.SemanticNode) {
	n.Role, n.Label = platform.RoleText, b.Text
}

// Layout sizes the chip around its icon and text, 22 tall.
func (b *Chip) Layout(c *ui.Context, cs ui.Constraints) geom.Size {
	if !b.attached {
		b.attached = true
		b.Add(&b.label)
	}
	if b.Name == "" {
		b.Name = b.kindName()
	}
	size, weight := b.Size, b.Weight
	if size <= 0 {
		size = chipSize
	}
	if weight == 0 {
		weight = chipWeight
	}
	labelStyle{Role: theme.RoleBody, Size: size, Weight: weight, Spacing: b.Spacing}.apply(&b.label)
	b.label.SetText(b.Text)
	var ts geom.Size
	content := float32(0)
	if b.Text != "" {
		ts = c.Layout(&b.label, ui.Loose(geom.Sz(ui.Unbounded, ui.Unbounded)))
		content = ts.W
	}
	isz := b.iconSize()
	if b.Icon != icon.NameUnknown {
		content += isz
		if b.Text != "" {
			content += chipGap
		}
	}
	pad, h := b.PadX, b.Height
	if pad <= 0 {
		pad = chipPad
	}
	if h <= 0 {
		h = chipHeight
	}
	out := cs.Constrain(geom.Sz(content+2*pad, h))
	x := (out.W - content) / 2
	if b.Icon != icon.NameUnknown {
		b.iconAt = geom.Pt(x, centeredY(out.H, isz))
		x += isz
		if b.Text != "" {
			x += chipGap
		}
	}
	if b.Text != "" {
		c.Place(&b.label, geom.Pt(x, centeredY(out.H, ts.H)))
	}

	return out
}

// IconRect returns where the icon is, in the chip's coordinates, as of the last
// layout; the zero rectangle when there is none.
func (b *Chip) IconRect() geom.Rect {
	if b.Icon == icon.NameUnknown {
		return geom.Rect{}
	}
	isz := b.iconSize()

	return geom.XYWH(b.iconAt.X, b.iconAt.Y, isz, isz)
}

// Paint draws the pill and the icon; the text is its child.
func (b *Chip) Paint(c *ui.Context, p *ui.Painter) {
	look := theme.From(c)
	bg, fg := b.colors(look)
	b.label.Color = fg
	r := SnapBox(p, b.Bounds()) // `.chip`: whole-CSS-px box (Chrome)
	if bg.A > 0 {
		radius := b.Radius
		if radius <= 0 {
			radius = chipRadius
		}
		p.Fill(r, scene.Solid(bg), capRadius(r, radius))
	}
	if b.Icon != icon.NameUnknown {
		isz := b.iconSize()
		icon.Draw(p, b.Icon, snapOrigin(p, geom.XYWH(b.iconAt.X, b.iconAt.Y, isz, isz)), fg, b.IconStroke) // `svg.i`: whole-CSS-px origin (Chrome)
	}
	b.Base.Paint(c, p)
}

// ====================================================================
// ========================= Helper Functions =========================
// ====================================================================

// iconSize is the size of the symbol: 22 unless set.
func (b *Chip) iconSize() float32 {
	if b.IconSize > 0 {
		return b.IconSize
	}

	return defaultIconSize
}

// colors returns the background (zero for none) and the text color: the kind's,
// or the overrides.
func (b *Chip) colors(look *theme.Look) (bg, fg color.Color) {
	t, k := &look.T, &look.C
	fg = t.Text
	switch b.Kind {
	case ChipTh:
		bg, fg = k.ChipRegularBg, k.ChipRegularText
	case ChipDb:
		bg, fg = k.ChipSpecialBg, t.Info
	case ChipGt, ChipOkc:
		bg, fg = k.ChipIntroBg, t.Success
	case ChipY:
		bg, fg = t.Primary, t.OnPrimary
	case ChipK:
		bg, fg = k.ChipDarkBg, color.White
	case ChipKindUnknown:
	default:
	}

	return orColor(b.Bg, bg), orColor(b.Fg, fg)
}

// kindName is the name layout.json gives the kind.
func (b *Chip) kindName() string {
	switch b.Kind {
	case ChipTh:
		return nameChipTh
	case ChipDb:
		return nameChipDb
	case ChipGt:
		return nameChipGt
	case ChipY:
		return nameChipY
	case ChipK:
		return nameChipK
	case ChipOkc:
		return nameChipOkc
	case ChipKindUnknown:
		return nameChip
	default:
		return nameChip
	}
}
