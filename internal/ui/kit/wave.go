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
// `data-scrub="m"` does, and a drag follows the finger to any position, as Scrub
// does (K38): the bars light up to the finger while it drags. Positions are
// shares, 0 to 1.
type Wave struct {
	ui.Base
	seekCore
	// Label names the bar for assistive technologies.
	Label string
	// Reading tells a screen reader the position and the length of the media, in
	// seconds; nil reads as 0 of 0.
	Reading func() (pos, dur float64)
	// OnStep is called with +-SeekStep when a screen reader increments or
	// decrements the bar.
	OnStep func(delta float64)

	pos   float32
	heard int // how many bars are lit (the player's)
}

var (
	_ SeekBar           = (*Wave)(nil)
	_ ui.PointerHandler = (*Wave)(nil)
	_ ui.Labeler        = (*Wave)(nil)
	_ ui.Describer      = (*Wave)(nil)
	_ ui.SemanticActor  = (*Wave)(nil)
)

// NewWave returns a wave with no bar lit but the first (`0 <= 0`).
func NewWave(label string) *Wave {
	w := &Wave{Label: label, heard: 1}
	w.Name = nameWave

	return w
}

// ====================================================================
// ========================== Main functions ==========================
// ====================================================================

// SetPos lights the bars up to share of the length: bar i when i / 70 <= share.
// A position that lights the same bars repaints nothing; nothing allocates.
// While a finger drags the bar it only remembers the share: the finger's lights
// the bars.
func (w *Wave) SetPos(share float32) {
	share = clamp01(share)
	n := litBars(share)
	w.pos = share
	if n != w.heard {
		w.heard = n
		if !w.seekCore.on {
			w.Repaint()
		}
	}
}

// Pos returns the share the bars were last set to (the player's, not the
// finger's).
func (w *Wave) Pos() float32 { return w.pos }

// Lit returns how many bars are lit (the first Lit of them): up to the finger
// while it drags.
func (w *Wave) Lit() int {
	if w.seekCore.on {
		return litBars(w.share)
	}

	return w.heard
}

// SeekBegin starts the drag at share.
func (w *Wave) SeekBegin(share float32) {
	w.begin(share)
	w.Repaint()
}

// SeekMove follows the finger to share; it repaints when another bar lights.
func (w *Wave) SeekMove(share float32) {
	was := w.Lit()
	if w.move(share) && w.Lit() != was {
		w.Repaint()
	}
}

// SeekEnd ends the drag: OnSeek once for commit, and the bars stay where the
// finger let go until the player tells its position.
func (w *Wave) SeekEnd(commit bool) {
	if share, was := w.end(commit); was {
		if commit {
			w.pos, w.heard = share, litBars(share)
		}
		w.Repaint()
	}
}

// AccessibleLabel returns the bar's label.
func (w *Wave) AccessibleLabel() string { return w.Label }

// Describe describes the bar as a slider over the length of the media.
func (w *Wave) Describe(_ *ui.Context, n *platform.SemanticNode) { describeSeek(n, w.Label, w.Reading) }

// PerformAction moves the position by SeekStep, back or on.
func (w *Wave) PerformAction(_ *ui.Context, a platform.SemanticAction) bool {
	return performSeek(a, w.OnStep)
}

// HandlePointer is the press, the drag and the release on the bars themselves (a
// screen puts a SeekPad over them for the touch target).
func (w *Wave) HandlePointer(_ *ui.Context, e *ui.PointerEvent) {
	handleSeek(e, w, w.Frame().W)
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
	lit := w.Lit()
	for i := range WaveBars {
		h := waveShare[i] * r.H
		paint := rest
		if i < lit {
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
