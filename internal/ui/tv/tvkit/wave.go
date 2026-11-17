//go:build tvos

package tvkit

import (
	"math"

	"dancefitvn-app/internal/ui/kit"
	"dancefitvn-app/internal/ui/theme"

	"github.com/gdeskdev/desk/foundation/core/geom"
	"github.com/gdeskdev/desk/foundation/platform"
	"github.com/gdeskdev/desk/foundation/scene"
	"github.com/gdeskdev/desk/framework/ui"
)

// ====================================================================
// ============================ Contracts =============================
// ====================================================================

// The numbers of the music screen's wave (index.html:696, `#tWave`): 120 bars in
// a row of `gap:2px;height:80px`, each `flex:1;border-radius:3px`.
const (
	// WaveBars is `Array.from({length:120}, ...)`.
	WaveBars = 120
	// waveHeight, waveGap, waveRadius are `height:80px`, `gap:2px` and
	// `border-radius:3px`.
	waveHeight, waveGap, waveRadius float32 = 80, 2, 3
)

// nameWave is the key layout.json gives the wave: `div.wave` is not recorded for
// the TV's, whose row has no class.
const nameWave = "tv.music.wave"

// waveShare is the height of bar i as a share of the row's, `25 + 75 *
// |sin(i * .55) * cos(i * .23 + 1)|` per cent, made once.
var waveShare = func() (s [WaveBars]float32) {
	for i := range s {
		x := float64(i)
		s[i] = float32((25 + 75*math.Abs(math.Sin(x*.55)*math.Cos(x*.23+1))) / 100)
	}

	return s
}()

// Wave is the music screen's waveform: 120 bars 80 tall at most, 2 apart, as wide
// as the room shares them, centred on the row. A bar is --pr once it is heard
// (`i / 120 <= pos / dur`, `liveTV`), --sf2 before. It is not a control: the
// prototype's has no seek.
type Wave struct {
	ui.Base

	pos float32
	on  int // how many bars are lit
	// Label names the wave for a screen reader ("Vị trí phát"); the prototype's
	// has no seek, so it is a progress bar.
	Label string
	// Spoken is the reading of the wave ("01:12 / 03:40"), made by the screen when
	// the clock moves to another second; empty reads as the percent played.
	Spoken string
}

// ====================================================================
// ============================= Creation =============================
// ====================================================================

// NewWave returns a wave with the first bar lit (`0 <= 0`).
func NewWave() *Wave {
	w := &Wave{on: 1}
	w.Name = nameWave

	return w
}

// ====================================================================
// ========================== Main functions ==========================
// ====================================================================

// SetPos lights the bars up to share of the length: bar i when i / 120 <= share.
// A position that lights the same bars repaints nothing; nothing allocates.
func (w *Wave) SetPos(share float32) {
	share = min(max(share, 0), 1)
	n := min(int(math.Floor(float64(share)*WaveBars))+1, WaveBars)
	w.pos = share
	if n != w.on {
		w.on = n
		w.Repaint()
	}
}

// SetSpoken changes the reading of the wave and tells assistive technologies; the
// same text changes nothing.
func (w *Wave) SetSpoken(txt string) {
	if txt != w.Spoken {
		w.Spoken = txt
		w.ValueChanged()
	}
}

// Describe describes the wave as a progress bar at the share played, valued by
// Spoken, else by the percent (kit.DescribeBar).
func (w *Wave) Describe(_ *ui.Context, n *platform.SemanticNode) {
	kit.DescribeBar(n, w.Label, w.Spoken, w.pos)
}

// Pos returns the share the bars were last set to.
func (w *Wave) Pos() float32 { return w.pos }

// Lit returns how many bars are lit (the first Lit of them).
func (w *Wave) Lit() int { return w.on }

// Layout is as wide as it is given and 80 tall.
func (w *Wave) Layout(_ *ui.Context, cs ui.Constraints) geom.Size {
	width := cs.Max.W
	if width >= ui.Unbounded {
		width = cs.Min.W
	}

	return cs.Constrain(geom.Sz(width, waveHeight))
}

// Paint draws the bars, each centred on the row: the heard ones in --pr.
func (w *Wave) Paint(c *ui.Context, p *ui.Painter) {
	t := &theme.From(c).T
	r := w.Bounds()
	bar := (r.W - float32(WaveBars-1)*waveGap) / WaveBars // `flex:1`
	heard, rest := scene.Solid(t.Primary), scene.Solid(t.Surface2)
	x := float32(0)
	for i := range WaveBars {
		h := waveShare[i] * r.H
		paint := rest
		if i < w.on {
			paint = heard
		}
		p.Fill(geom.XYWH(x, (r.H-h)/2, bar, h), paint, waveRadius)
		x += bar + waveGap
	}
}
