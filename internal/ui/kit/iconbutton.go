package kit

import (
	"dancefitvn-app/internal/ui/icon"
	"dancefitvn-app/internal/ui/theme"

	"github.com/gdeskdev/desk/foundation/core/geom"
	"github.com/gdeskdev/desk/foundation/platform"
	"github.com/gdeskdev/desk/framework/ui"
)

// ====================================================================
// ============================ Contracts =============================
// ====================================================================

// inlineStrut is the descent the 16 px strut adds under an inline `<svg>` in a
// `<span>` or `<button>` that has no flex of its own: round(.2421875 x 16) = 4
// (F-4 of local/plans/P2.md; `pwEye` is 22 x 26, `go.account` 44 x 48).
const inlineStrut float32 = 4

// ====================================================================
// ============================= Creation =============================
// ====================================================================

// IconButton is a pressable icon that is not a flex item, as the eye of the
// password field is (`<span data-act="pwEye" style="cursor:pointer">ic('eye')
// </span>`, index.html:345): Size wide and Size + 4 tall, the icon at the top, the
// 4 being the descent of the line the inline svg sits on. Set the Name to the
// data key (`pwEye`).
type IconButton struct {
	ui.Pressable
	// Icon is the symbol shown.
	Icon icon.Name
	// Size is the symbol's size; 0 reads as 22.
	Size float32
	// Stroke is the symbol's stroke; 0 is the default.
	Stroke float32
	// Ink colors the symbol; InkUnknown reads as --tx2, the color of an `.inp`.
	Ink theme.Ink
	// Label names the button for assistive technologies.
	Label string
	// Value is what the button holds, spoken after its label ("01:12–01:40" of the
	// loop); empty says nothing.
	Value string
	// State is ToggledOn or ToggledOff for a button that reads as a toggle (the
	// loop of screen 08); the zero ToggledUnknown is a plain button.
	State platform.Toggled

	icon     Icon
	attached bool
}

var (
	_ ui.Labeler   = (*IconButton)(nil)
	_ ui.Describer = (*IconButton)(nil)
)

// ====================================================================
// ========================== Main functions ==========================
// ====================================================================

// NewIconButton returns an icon button of the symbol icn.
func NewIconButton(icn icon.Name, label string) *IconButton {
	return &IconButton{Icon: icn, Label: label}
}

// SetIcon changes the symbol; the same one changes nothing. The icon child takes
// it at once, as no layout follows (DiscButton.SetIcon).
func (b *IconButton) SetIcon(n icon.Name) {
	if n != b.Icon {
		b.Icon = n
		b.icon.Glyph = n
		b.Repaint()
	}
}

// AccessibleLabel returns the button's label.
func (b *IconButton) AccessibleLabel() string { return b.Label }

// Describe describes the button as a button named by Label, valued by Value, with
// the State of a toggle.
func (b *IconButton) Describe(c *ui.Context, n *platform.SemanticNode) {
	b.Pressable.Describe(c, n)
	n.Value, n.Toggle = b.Value, b.State
}

// Layout takes the icon's width and its height plus the strut.
func (b *IconButton) Layout(c *ui.Context, cs ui.Constraints) geom.Size {
	if !b.attached {
		b.attached = true
		b.Look = noLook
		b.icon.Name = iconName
		b.Add(&b.icon)
	}
	ink := b.Ink
	if ink == theme.InkUnknown {
		ink = theme.InkText2
	}
	b.icon.Glyph, b.icon.Size, b.icon.Stroke, b.icon.Ink = b.Icon, b.Size, b.Stroke, ink
	is := c.Layout(&b.icon, ui.Loose(geom.Sz(ui.Unbounded, ui.Unbounded)))
	c.Place(&b.icon, geom.Point{})

	return cs.Constrain(geom.Sz(is.W, is.H+inlineStrut))
}
