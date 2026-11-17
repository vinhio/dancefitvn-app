package kit

import (
	"math"

	"dancefitvn-app/internal/ui/theme"

	"github.com/gdeskdev/desk/foundation/core/geom"
	"github.com/gdeskdev/desk/foundation/platform"
	"github.com/gdeskdev/desk/foundation/scene"
	"github.com/gdeskdev/desk/framework/ui"
)

// ====================================================================
// ============================ Contracts =============================
// ====================================================================

// The numbers of `.wave` (index.html:164-166) and of the bars MS.audio makes
// (index.html:437), the waveform of screen 08.
const (
	// WaveBars is `Array.from({length:70}, ...)`.
	WaveBars = 70
	// waveGap, waveHeight are `.wave{gap:2px;height:40px}` (:164).
	waveGap, waveHeight float32 = 2, 40
	// waveRadius is `.wave i{border-radius:2px}` (:165).
	waveRadius float32 = 2
)

// nameWave is the class of the bar, `div.wave`.
const nameWave = "div.wave"

// waveShare is the height of bar i as a share of the bar's, `25 + 75 *
// |sin(i * .55) * cos(i * .23 + 1)|` per cent (index.html:437), made once.
var waveShare = func() (s [WaveBars]float32) {
	for i := range s {
		x := float64(i)
		s[i] = float32((25 + 75*math.Abs(math.Sin(x*.55)*math.Cos(x*.23+1))) / 100)
	}

	return s
}()

// ====================================================================
// ============================= Creation =============================
// ====================================================================

// Wave is `.wave`: 70 bars 40 DIPs tall at most, 2 apart, as wide as the room
// shares them, each as high as the formula of the prototype says, centered on the
// row. A bar is --pr once it is heard (`i / 70 <= pos / dur`, `liveM`), --sf2
// before. A press on it, anywhere, asks for that position through OnSeek, as
// `data-scrub="m"` does (no drag, as Scrub). Positions are shares, 0 to 1.
type Wave struct {
	ui.Base
	// OnSeek is called with the share of the width the press fell on.
	OnSeek func(share float32)
	// Label names the bar for assistive technologies.
	Label string
	// Reading tells a screen reader the position and the length of the media, in
	// seconds; nil reads as 0 of 0.
	Reading func() (pos, dur float64)
	// OnStep is called with +-SeekStep when a screen reader increments or
	// decrements the bar.
	OnStep func(delta float64)

	pos float32
	on  int // how many bars are lit
}

var (
	_ ui.PointerHandler = (*Wave)(nil)
	_ ui.Labeler        = (*Wave)(nil)
	_ ui.Describer      = (*Wave)(nil)
	_ ui.SemanticActor  = (*Wave)(nil)
)

// NewWave returns a wave with no bar lit but the first (`0 <= 0`).
func NewWave(label string) *Wave {
	w := &Wave{Label: label, on: 1}
	w.Name = nameWave

	return w
}

// ====================================================================
// ========================== Main functions ==========================
// ====================================================================

// SetPos lights the bars up to share of the length: bar i when i / 70 <= share.
// A position that lights the same bars repaints nothing; nothing allocates.
func (w *Wave) SetPos(share float32) {
	share = clamp01(share)
	n := litBars(share)
	w.pos = share
	if n != w.on {
		w.on = n
		w.Repaint()
	}
}

// Pos returns the share the bars were last set to.
func (w *Wave) Pos() float32 { return w.pos }

// Lit returns how many bars are lit (the first Lit of them).
func (w *Wave) Lit() int { return w.on }

// AccessibleLabel returns the bar's label.
func (w *Wave) AccessibleLabel() string { return w.Label }

// Describe describes the bar as a slider over the length of the media.
func (w *Wave) Describe(_ *ui.Context, n *platform.SemanticNode) { describeSeek(n, w.Label, w.Reading) }

// PerformAction moves the position by SeekStep, back or on.
func (w *Wave) PerformAction(_ *ui.Context, a platform.SemanticAction) bool {
	return performSeek(a, w.OnStep)
}

// HandlePointer asks for a position on a press, takes the press and every event
// after it, but does not follow the finger.
func (w *Wave) HandlePointer(_ *ui.Context, e *ui.PointerEvent) {
	switch e.Kind {
	case ui.PointerDown:
		if e.Button != platform.ButtonPrimary && e.Button != platform.ButtonNone {
			return
		}
		e.Handle()
		if fw := w.Frame().W; fw > 0 && w.OnSeek != nil {
			w.OnSeek(clamp01(e.Pos.X / fw))
		}
	case ui.PointerUp, ui.PointerMove:
		e.Handle()
	case ui.PointerUnknown, ui.PointerEnter, ui.PointerLeave, ui.PointerCancel, ui.PointerScroll:
	default:
	}
}

// Layout is as wide as it is given and 40 tall.
func (w *Wave) Layout(_ *ui.Context, cs ui.Constraints) geom.Size {
	width := cs.Max.W
	if width >= ui.Unbounded {
		width = cs.Min.W
	}

	return cs.Constrain(geom.Sz(width, waveHeight))
}

// Paint draws the bars, each centered on the row: the heard ones in --pr.
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

// ====================================================================
// ========================= Helper Functions =========================
// ====================================================================

// litBars returns how many bars `i / 70 <= share` lights: bar i is heard when
// i <= share * 70, so floor(share * 70) + 1 of them, at most all 70.
func litBars(share float32) int {
	return min(int(math.Floor(float64(share)*WaveBars))+1, WaveBars)
}
