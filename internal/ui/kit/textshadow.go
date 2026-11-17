// Adapted from jplayer-tv/internal/ui/kit/textshadow.go at 781bee5.

package kit

import (
	"github.com/gdeskdev/desk/bridge/text"
	"github.com/gdeskdev/desk/foundation/core/color"
	"github.com/gdeskdev/desk/foundation/core/geom"
	"github.com/gdeskdev/desk/framework/ui"
)

// ====================================================================
// ============================ Contracts =============================
// ====================================================================

// TextShadow is a CSS `text-shadow: <x> <y> <blur> <color>`.
type TextShadow struct {
	// Offset moves the shadow from the text.
	Offset geom.Point
	// Blur is the CSS blur radius in points; as for box-shadow, the Gaussian's
	// standard deviation is half of it, which is what [ui.Painter.Layer] takes.
	Blur float32
	// Color is the shadow's colour; its alpha is the strength.
	Color color.Color
}

// PosterTitleShadow is `.poster .ttl{text-shadow:0 2px 12px rgba(0,0,0,.18)}`
// (index.html:67), the title on a poster.
var PosterTitleShadow = TextShadow{Offset: geom.Pt(0, 2), Blur: 12, Color: color.RGBA(0, 0, 0, 0.18)}

// ====================================================================
// ========================== Main functions ==========================
// ====================================================================

// DrawTextShadow draws l at at in colour c over its shadow s. gDesk has no
// text shadow (GT18 of jPlayer TV), so the shadow is the text in the shadow's
// colour inside a blurred layer, drawn first: the picture is the CSS one, at
// the cost of a layer per call. It does not allocate beyond the painter's
// own recording.
func DrawTextShadow(p *ui.Painter, l *text.Layout, at geom.Point, s TextShadow, c color.Color) {
	if l == nil {
		return
	}
	drawTextShadow(p, l, snapTextShadow(p, l, at), s, c)
}

// ====================================================================
// ========================= Helper Functions =========================
// ====================================================================

// snapTextShadow is where Chrome draws the text asked for at at: shifted by the
// text shift, its first baseline on a whole CSS px.
func snapTextShadow(p *ui.Painter, l *text.Layout, at geom.Point) geom.Point {
	at.X += textShiftX()
	if len(l.Lines) > 0 {
		// Chrome puts the baseline of the title's 1x line box on a whole CSS px.
		base, off := l.Lines[0].Baseline, p.Offset()
		at.Y = snapAxis(off.Y+at.Y+base, iconSnapUnit) - base - off.Y
	}

	return at
}

// drawTextShadow draws l at at, which is placed already, over its shadow as a
// blurred layer.
func drawTextShadow(p *ui.Painter, l *text.Layout, at geom.Point, s TextShadow, c color.Color) {
	if s.Color.A > 0 {
		shadowAt := at.Add(s.Offset)
		p.Layer(1, s.Blur, func() { p.Text(l, shadowAt, s.Color) })
	}
	p.Text(l, at, c)
}
