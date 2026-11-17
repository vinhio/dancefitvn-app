// Adapted from jplayer-tv/internal/ui/theme/lineheight.go at 781bee5.

package theme

import (
	"math"

	"github.com/gdeskdev/desk/bridge/text"
)

// ====================================================================
// ============================ Contracts =============================
// ====================================================================

// LineBox is the line box CSS gives a line of text: its height and where its
// baseline lies in it, both from the top, in DIPs.
type LineBox struct {
	Height   float32
	Baseline float32
}

// ====================================================================
// ========================== Main functions ==========================
// ====================================================================

// Chrome returns the line box Chrome gives text set in a font with metrics m
// when `line-height` is lineHeight DIPs; lineHeight 0 is `line-height: normal`.
//
// Chrome rounds the font's ascent and descent each to whole pixels (and its
// line gap, which `normal` adds), so `normal` is not size x (ascent + descent)
// but a sum of integers: Inter 11 px is 11 + 3 = 14 high, which the layout.json
// of `.eb` records. With a set line height, the leading is split in two and the
// half over the text floors, so the baseline sits at floor(half-leading) plus
// the rounded ascent. The metrics come from the shaper's own, of the shipped
// font files, not from a table of Chrome's values. It does not allocate.
func Chrome(m text.Metrics, lineHeight float32) LineBox {
	asc, desc := rnd(m.Ascent), rnd(m.Descent)
	if lineHeight <= 0 {
		return LineBox{Height: asc + desc + rnd(m.LineGap), Baseline: asc}
	}
	half := float32(math.Floor(float64(lineHeight-(asc+desc)) / 2))

	return LineBox{Height: lineHeight, Baseline: half + asc}
}

// ====================================================================
// ========================= Helper Functions =========================
// ====================================================================

// rnd rounds half away from zero.
func rnd(v float32) float32 { return float32(math.Round(float64(v))) }
