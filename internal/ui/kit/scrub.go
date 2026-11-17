package kit

import (
	"dancefitvn-app/internal/ui/theme"

	"github.com/gdeskdev/desk/foundation/core/geom"
	"github.com/gdeskdev/desk/foundation/platform"
	"github.com/gdeskdev/desk/foundation/scene"
	"github.com/gdeskdev/desk/framework/ui"
)

// ====================================================================
// ============================ Contracts =============================
// ====================================================================

// The numbers of `.scrub` (index.html:151-155), the position bar of screen 06.
const (
	// scrubHeight is `.scrub{height:18px}` (:151).
	scrubHeight float32 = 18
	// scrubTrackTop, scrubTrackH, scrubTrackRadius are `.scrub .tr{top:7px;
	// height:4px;border-radius:2px}` (:152) and `.fl` (:153): the fill has the
	// same top, height and radius.
	scrubTrackTop, scrubTrackH, scrubTrackRadius float32 = 7, 4, 2
	// scrubBandTop, scrubBandH, scrubBandRadius are `.scrub .ab{top:5px;
	// height:8px;border-radius:2px}` (:154): the A-B band.
	scrubBandTop, scrubBandH, scrubBandRadius float32 = 5, 8, 2
	// scrubKnobTop, scrubKnob are `.scrub .kn{top:2px;width:14px;height:14px;
	// margin-left:-7px;border-radius:50%}` (:155).
	scrubKnobTop, scrubKnob float32 = 2, 14
	// scrubTrackAlpha is `.scrub .tr{background:rgba(255,255,255,.3)}` (:152) and
	// scrubBandAlpha `.scrub .ab{background:rgba(229,56,59,.7)}` (:154): --red at .7.
	scrubTrackAlpha, scrubBandAlpha float32 = .3, .7
)

// nameScrub is the key of the bar: `data-scrub="m"`.
const nameScrub = "scrub"

// ====================================================================
// ============================= Creation =============================
// ====================================================================

// Scrub is `.scrub`: a 4 DIP track of white at .3, filled in --pr up to the
// position, with a 14 DIP knob on the end of the fill and, when the A-B loop is
// on, the band of red at .7 between A and B. A press on it, anywhere, asks for
// that position through OnSeek, as the prototype's `pointerdown` does (there is
// no drag, Q-P4-15). Positions are shares of the length, 0 to 1.
type Scrub struct {
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

	pos  float32
	a, b float32
	band bool
}

var (
	_ ui.PointerHandler = (*Scrub)(nil)
	_ ui.Labeler        = (*Scrub)(nil)
	_ ui.Describer      = (*Scrub)(nil)
	_ ui.SemanticActor  = (*Scrub)(nil)
)

// NewScrub returns a bar at 0 with no band.
func NewScrub(label string) *Scrub {
	s := &Scrub{Label: label}
	s.Name = nameScrub

	return s
}

// ====================================================================
// ========================== Main functions ==========================
// ====================================================================

// SetPos moves the fill and the knob to share of the width; the same position
// repaints nothing. It does not allocate.
func (s *Scrub) SetPos(share float32) {
	share = clamp01(share)
	if s.pos != share {
		s.pos = share
		s.Repaint()
	}
}

// Pos returns the share the fill is at.
func (s *Scrub) Pos() float32 { return s.pos }

// SetBand shows the A-B band from a to b (shares), or hides it when on is false.
func (s *Scrub) SetBand(on bool, a, b float32) {
	a, b = clamp01(a), clamp01(b)
	if s.band != on || (on && (s.a != a || s.b != b)) {
		s.band, s.a, s.b = on, a, b
		s.Repaint()
	}
}

// Band returns whether the band shows and its ends.
func (s *Scrub) Band() (on bool, a, b float32) { return s.band, s.a, s.b }

// AccessibleLabel returns the bar's label.
func (s *Scrub) AccessibleLabel() string { return s.Label }

// Describe describes the bar as a slider over the length of the media.
func (s *Scrub) Describe(_ *ui.Context, n *platform.SemanticNode) {
	describeSeek(n, s.Label, s.Reading)
}

// PerformAction moves the position by SeekStep, back or on.
func (s *Scrub) PerformAction(_ *ui.Context, a platform.SemanticAction) bool {
	return performSeek(a, s.OnStep)
}

// HandlePointer asks for a position on a press, takes the press and every
// event after it, but does not follow the finger.
func (s *Scrub) HandlePointer(_ *ui.Context, e *ui.PointerEvent) {
	switch e.Kind {
	case ui.PointerDown:
		if e.Button != platform.ButtonPrimary && e.Button != platform.ButtonNone {
			return
		}
		e.Handle()
		if w := s.Frame().W; w > 0 && s.OnSeek != nil {
			s.OnSeek(clamp01(e.Pos.X / w))
		}
	case ui.PointerUp, ui.PointerMove:
		e.Handle()
	case ui.PointerUnknown, ui.PointerEnter, ui.PointerLeave, ui.PointerCancel, ui.PointerScroll:
	default:
	}
}

// Layout is as wide as it is given and 18 tall.
func (s *Scrub) Layout(_ *ui.Context, cs ui.Constraints) geom.Size {
	w := cs.Max.W
	if w >= ui.Unbounded {
		w = cs.Min.W
	}

	return cs.Constrain(geom.Sz(w, scrubHeight))
}

// Paint draws the track, the band, the fill and the knob, in that order.
func (s *Scrub) Paint(c *ui.Context, p *ui.Painter) {
	look := theme.From(c)
	r := s.Bounds()
	p.Fill(geom.XYWH(0, scrubTrackTop, r.W, scrubTrackH), scene.Solid(theme.InkWhite.Of(look).WithAlpha(scrubTrackAlpha)), scrubTrackRadius)
	if s.band && s.b > s.a {
		p.Fill(geom.XYWH(s.a*r.W, scrubBandTop, (s.b-s.a)*r.W, scrubBandH), scene.Solid(look.T.Red.WithAlpha(scrubBandAlpha)), scrubBandRadius)
	}
	fw := s.pos * r.W
	if fw > 0 {
		p.Fill(geom.XYWH(0, scrubTrackTop, fw, scrubTrackH), scene.Solid(look.T.Primary), scrubTrackRadius)
	}
	p.Fill(geom.XYWH(fw-scrubKnob/2, scrubKnobTop, scrubKnob, scrubKnob), scene.Solid(look.T.Primary), scrubKnob/2)
}

// ====================================================================
// ========================= Helper Functions =========================
// ====================================================================

// clamp01 keeps v in [0, 1]; NaN is 0.
func clamp01(v float32) float32 {
	if !(v > 0) {
		return 0
	}

	return min(v, 1)
}
