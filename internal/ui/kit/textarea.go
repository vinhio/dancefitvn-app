// Adapted from jplayer-mobile/internal/ui/kit/field.go at 49de3c2 (TextArea: the
// several-line field on headless.TextArea, its frame painted, its text, selection,
// composition and caret from the area's accessors, `DragExempt`, the label of the
// Field around it). The look is DanceFITVN's: the `<textarea>` of the coach
// profile (index.html:489), painted as Input paints `.inp`.

package kit

import (
	"math"

	"dancefitvn-app/internal/ui/theme"

	"github.com/gdeskdev/desk/foundation/core/geom"
	"github.com/gdeskdev/desk/foundation/platform"
	"github.com/gdeskdev/desk/foundation/scene"
	"github.com/gdeskdev/desk/framework/components/headless"
	"github.com/gdeskdev/desk/framework/ui"
)

// ====================================================================
// ============================ Contracts =============================
// ====================================================================

// The numbers of the `<textarea id="bio">` (index.html:489).
const (
	// textareaMinHeight is `min-height:80px`; the box is border-box, and the
	// textarea's `rows` give less, so the height is 80.
	textareaMinHeight float32 = 80
	// textareaRadius is `border-radius:14px`.
	textareaRadius float32 = 14
	// textareaPadV, textareaPadH are `padding:12px 14px`; the 1 px border is
	// inside the box, so the text starts one further (`border:1px solid var(--bd)`).
	textareaPadV, textareaPadH float32 = 12, 14
	// textareaSize is `font-size:14px`.
	textareaSize float32 = 14
	// TextAreaGap is the room under a textarea in a `.field`: the 4 px of the
	// line the inline-block sits on (its descender), then `.field{margin-bottom:16px}`.
	TextAreaGap float32 = 4
)

// nameTextArea is the key of a text area in layout.json: the `<textarea>` is not
// recorded by the prototype's capture, so no key of it is compared.
const nameTextArea = "textarea"

// TextArea is the `<textarea>` of screen 15C (index.html:489): a box of --sf, 14
// rounded, 80 tall, with a 1 px --bd border (--pr while it has the focus, as
// `.inp`), padded 12 14, 14 px text, several lines broken at its width, scrolling
// to keep the caret in view. Its Name is `textarea`.
type TextArea struct {
	headless.TextArea
	// Label names the field for assistive technologies; the placeholder does when
	// it is empty.
	Label string
}

var (
	_ ui.Labeler   = (*TextArea)(nil)
	_ ui.Describer = (*TextArea)(nil)
)

// ====================================================================
// ========================== Main functions ==========================
// ====================================================================

// NewTextArea returns an empty text area with a placeholder.
func NewTextArea(placeholder string) *TextArea {
	return &TextArea{TextArea: headless.TextArea{Placeholder: placeholder}}
}

// AccessibleLabel returns the area's label, or its placeholder.
func (a *TextArea) AccessibleLabel() string {
	if a.Label != "" {
		return a.Label
	}

	return a.Placeholder
}

// Describe describes the area as a text area named by its label.
func (a *TextArea) Describe(c *ui.Context, n *platform.SemanticNode) {
	a.TextArea.Describe(c, n)
	if a.Label != "" {
		n.Label = a.Label
	}
}

// DragExempt tells that a press on the area never starts the drag of a sheet
// around it.
func (a *TextArea) DragExempt() bool { return true }

// Layout takes the room's width and 80 DIPs.
func (a *TextArea) Layout(c *ui.Context, cs ui.Constraints) geom.Size {
	if a.Name == "" {
		a.Name = nameTextArea
	}
	a.Font = theme.From(c).Faces.Face(theme.RoleBody, textareaSize, 400)
	a.Padding = geom.Insets{
		Left: borderInset + textareaPadH, Right: borderInset + textareaPadH,
		Top: borderInset + textareaPadV, Bottom: borderInset + textareaPadV,
	}
	cs.Min.H, cs.Max.H = textareaMinHeight, textareaMinHeight

	return a.TextArea.Layout(c, cs)
}

// Paint draws the frame as `.inp` does, then the lines, the selection, the
// composition's underline and the caret.
func (a *TextArea) Paint(c *ui.Context, p *ui.Painter) {
	look := theme.From(c)
	t := &look.T
	r := SnapBox(p, a.Bounds())
	radius := capRadius(r, textareaRadius)
	border, width := t.Border, inputBorder
	if a.FocusWithin() {
		border, width = t.Primary, inputRing
	}
	p.Fill(r, scene.Solid(t.Surface), radius)
	// Chrome floors a border to whole CSS px, as for `.inp`
	p.Border(r, geom.Uniform(float32(math.Floor(float64(width)))), border, radius)
	p.Clip(a.ContentRect(), 0, func() {
		o := a.TextOrigin()
		o.X += textShiftX()
		focused := a.Focused()
		if focused {
			for _, s := range a.SelectionRects() {
				p.Fill(s, scene.Solid(t.Primary), 0)
			}
		}
		if a.Len() == 0 && a.Composing() == "" {
			if a.Placeholder != "" {
				p.Text(c.TextLayout(a.Placeholder, a.FontFace(c.Theme()), a.ContentRect().W), o, t.Text3)
			}
		} else {
			for _, ln := range a.VisibleLines() {
				at := geom.Pt(o.X, o.Y+ln.Y)
				p.Text(ln.Layout, at, t.Text)
				if focused {
					for _, s := range a.SelectionRects() {
						p.Clip(s, 0, func() { p.Text(ln.Layout, at, t.OnPrimary) })
					}
				}
			}
		}
		for _, u := range a.CompositionRects() {
			p.Fill(geom.XYWH(u.X, u.Y+u.H-1, u.W, 1), scene.Solid(t.Text), 0)
		}
		if a.CaretVisible() && !a.HasSelection() {
			p.Fill(a.CaretRect(), scene.Solid(t.Text), 0)
		}
	})
}
