// Adapted from jplayer-mobile/internal/ui/kit/common.go at 49de3c2.

package kit

import (
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

// focusOffset and focusWidth are the browser's default `:focus-visible`
// outline, which the prototype leaves unstyled: 2 px, 2 px away from the box.
const (
	focusOffset float32 = 2
	focusWidth  float32 = 2
)

// borderInset is the 1 px border the layout of an `.inp` and a `.radio` keeps
// in every state: Chrome rounds a 1.2 or 1.6 px border down to the device pixel
// when it lays out (layout.json of M02B and M09 reports border 1px and the
// content at the same x as in the resting state), so the content does not move
// when the border is thicker.
const borderInset float32 = 1

// bodyStrut is the size of the page's text, `html,body` with no font-size: the
// 16 px strut every inline run of text in a block sits on.
const bodyStrut float32 = 16

// ====================================================================
// ============================= Creation =============================
// ====================================================================

// labelStyle is the font of a Label the way CSS writes it: size, weight,
// line height, letter-spacing, text-transform and color.
type labelStyle struct {
	Role       theme.Role
	Size       float32
	Weight     text.Weight
	LineHeight float32
	Spacing    float32
	Upper      bool
	Ink        theme.Ink
	Ellipsis   bool
	Wrap       bool
	MaxLines   int
}

// ====================================================================
// ========================== Main functions ==========================
// ====================================================================

// apply sets the label's font to s, laying it out again only when something
// changed. Call it before laying the label out.
func (s labelStyle) apply(l *Label) {
	if l.Role == s.Role && l.Size == s.Size && l.Weight == s.Weight && l.LineHeight == s.LineHeight &&
		l.Spacing == s.Spacing && l.Upper == s.Upper && l.Ink == s.Ink && l.Ellipsis == s.Ellipsis &&
		l.Wrap == s.Wrap && l.MaxLines == s.MaxLines {
		return
	}
	l.Role, l.Size, l.Weight, l.LineHeight, l.Spacing, l.Upper = s.Role, s.Size, s.Weight, s.LineHeight, s.Spacing, s.Upper
	l.Ink, l.Ellipsis, l.Wrap, l.MaxLines = s.Ink, s.Ellipsis, s.Wrap, s.MaxLines
	l.Relayout()
}

// SetHidden hides or shows w, and lays its parents out again when that changes:
// a flip of ui.Base.Hidden alone leaves the parents' cached layout as it was, so
// a widget shown again would keep the zero frame it was left with (or none at
// all), and one hidden would leave its room. Setting the state it has already
// changes nothing and costs nothing, so a steady frame allocates nothing.
// It reports whether the state changed.
func SetHidden(w ui.Widget, hidden bool) bool {
	b := w.Node()
	if b.Hidden == hidden {
		return false
	}
	b.Hidden = hidden
	b.Relayout()

	return true
}

// ====================================================================
// ========================= Helper Functions =========================
// ====================================================================

// flag is 1 for true and 0 for false: the target of a state's animation.
func flag(b bool) float32 {
	if b {
		return 1
	}

	return 0
}

// focusRing draws the focus outline round r, rounded by radius.
func focusRing(p *ui.Painter, r geom.Rect, radius float32, col color.Color) {
	g := focusOffset + focusWidth
	p.Border(r.Inset(geom.Uniform(-g)), geom.Uniform(focusWidth), col, radius+g)
}

// scaleAbout returns the map scaling by s about the point at.
func scaleAbout(s float32, at geom.Point) scene.Affine {
	return scene.Affine{XX: s, YY: s, X0: at.X * (1 - s), Y0: at.Y * (1 - s)}
}

// scaled paints fn scaled by s about the middle of r; at s = 1 it just calls
// fn, as Painter.Transform turns off the culling of children.
func scaled(p *ui.Painter, r geom.Rect, s float32, fn func()) {
	if s == 1 {
		fn()

		return
	}
	p.Transform(scaleAbout(s, r.Center()), fn)
}

// centeredY returns the y that puts a box of height h in the middle of a box
// of height outer.
func centeredY(outer, h float32) float32 { return (outer - h) / 2 }

// capRadius brings a corner radius within half the shorter side of r, as CSS
// does: `border-radius: 999px` is a pill.
func capRadius(r geom.Rect, radius float32) float32 { return min(radius, min(r.W, r.H)/2) }

// strut returns the line box of the page's 16 px body text. A run of inline
// text (a `<span>` after a block, a `<b>` inside a `<span>`) sits in a line box
// at least that tall with its baseline on the strut's, so a 12 px sub line is
// 20 px high and a 14 px title 20, not their own 15 and 17.
func strut(c *ui.Context) theme.LineBox {
	face := theme.From(c).Faces.Face(theme.RoleBody, bodyStrut, 0)
	m, err := c.Shaper().Metrics(face)
	if err != nil {
		m = text.Metrics{Ascent: bodyStrut * .96875, Descent: bodyStrut * .2421875}
	}

	return theme.Chrome(m, 0)
}

// noLook is the Look of a Pressable that draws itself: nothing.
func noLook(*ui.Context, ui.Interaction) ui.Decoration { return ui.Decoration{} }

// orColor returns col, or def when col is the zero color.
func orColor(col, def color.Color) color.Color {
	if col.A == 0 {
		return def
	}

	return col
}
