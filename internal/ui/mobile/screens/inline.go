//go:build !tvos

package screens

import (
	"dancefitvn-app/internal/ui/kit"
	"dancefitvn-app/internal/ui/theme"

	"github.com/gdeskdev/desk/bridge/text"
	"github.com/gdeskdev/desk/foundation/core/geom"
	"github.com/gdeskdev/desk/framework/ui"
)

// ====================================================================
// ============================ Contracts =============================
// ====================================================================

// bodyStrut is the size of the page's own text, the 16 px strut every line of
// inline text in a block sits on (kit's bodyStrut).
const bodyStrut float32 = 16

// inlineLine is a run of inline text in a block of the page's 16 px text: a `<b>`
// in a `<div>`. Its line box is at least the strut's, with the baselines
// together, so that a 14 px title is 20 high and not its own 17.
type inlineLine struct {
	ui.Base

	label *kit.Label
}

// ====================================================================
// ============================= Creation =============================
// ====================================================================

// newInline returns the line holding label.
func newInline(label *kit.Label) *inlineLine {
	l := &inlineLine{label: label}
	l.Add(label)

	return l
}

// ====================================================================
// ========================== Main functions ==========================
// ====================================================================

// Layout puts the label on the strut's baseline.
func (l *inlineLine) Layout(c *ui.Context, cs ui.Constraints) geom.Size {
	ls := c.Layout(l.label, ui.Loose(geom.Sz(cs.Max.W, ui.Unbounded)))
	s := strut(c)
	lb := l.label.Baseline()
	asc := max(s.Baseline, lb)
	desc := max(s.Height-s.Baseline, ls.H-lb)
	c.Place(l.label, geom.Pt(0, asc-lb))

	return cs.Constrain(geom.Sz(ls.W, asc+desc))
}

// ====================================================================
// ========================= Helper Functions =========================
// ====================================================================

// strut returns the line box of the page's 16 px body text.
func strut(c *ui.Context) theme.LineBox {
	face := theme.From(c).Faces.Face(theme.RoleBody, bodyStrut, 0)
	m, err := c.Shaper().Metrics(face)
	if err != nil {
		m = text.Metrics{Ascent: bodyStrut * .96875, Descent: bodyStrut * .2421875}
	}

	return theme.Chrome(m, 0)
}
