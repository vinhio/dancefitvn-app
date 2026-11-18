package kit

import (
	"time"

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
	// scrubKnobDrag is how much larger the knob is while a finger drags it. jPlayer
	// Mobile grows it 1.4 times (`.range.drag .r-knob{transform:scale(1.4)}`,
	// components.css:206) over `.2s` (:204, scrubKnobTime); the bar is 18 tall and
	// has no room above and below for 19.6, so 1.25 (17.5). K38: the prototype's
	// bar has no drag.
	scrubKnobDrag float32 = 1.25
	scrubKnobTime         = 200 * time.Millisecond
	// scrubMarkW, scrubMarkH, scrubMarkTop are the marker at the B end of the A-B
	// band, the handle a finger takes to move B (K38; the prototype's band has no
	// handle): 4 wide and 16 tall from 1, in --red; scrubMarkDragW and
	// scrubMarkDragH while it is dragged. scrubMarkReach is how far from it, each
	// side, a press takes it rather than the knob: the 44 DIP touch target.
	scrubMarkW, scrubMarkH, scrubMarkTop float32 = 4, 16, 1
	scrubMarkDragW, scrubMarkDragH       float32 = 6, 18
	scrubMarkReach                       float32 = SeekTouchHeight / 2
)

// nameScrub is the key of the bar: `data-scrub="m"`.
const nameScrub = "scrub"

// ====================================================================
// ============================= Creation =============================
// ====================================================================

// Scrub is `.scrub`: a 4 DIP track of white at .3, filled in --pr up to the
// position, with a 14 DIP knob on the end of the fill and, when the A-B loop is
// on, the band of red at .7 between A and B. A press on it, anywhere, asks for
// that position through OnSeek and a drag follows the finger to any position,
// playing or paused (K38: the prototype's `pointerdown` only jumps, Q-P4-15; the
// drag is jPlayer Mobile's kit/range.go). A press is taken at once, the knob and
// the fill follow the finger (OnDrag says where, for the time label), the knob
// grows a little, and the release asks once for the position (OnSeek). Positions
// are shares of the length, 0 to 1.
type Scrub struct {
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
	// OnBSet is called once, with the share where a finger lets go of the B marker
	// (K38): the screen moves B there; nothing seeks.
	OnBSet func(share float32)

	pos  float32
	a, b float32
	band bool
	knob ui.Animated[float32] // how far the knob has grown, 0 to 1

	// bMin is the least share B can take (A plus the loop's least length); bOn and
	// bShare are a drag of the B marker under way and where it is.
	bMin, bShare float32
	bOn          bool
}

var (
	_ SeekBar           = (*Scrub)(nil)
	_ BMarker           = (*Scrub)(nil)
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
// repaints nothing. While a finger drags the bar it only remembers the share:
// the bar shows the finger's. It does not allocate.
func (s *Scrub) SetPos(share float32) {
	share = clamp01(share)
	if s.pos != share {
		s.pos = share
		if !s.on {
			s.Repaint()
		}
	}
}

// Pos returns the share the player put the fill at (not the finger's).
func (s *Scrub) Pos() float32 { return s.pos }

// Shown returns the share the fill and the knob are drawn at: the finger's
// while it drags, else Pos.
func (s *Scrub) Shown() float32 {
	if s.on {
		return s.share
	}

	return s.pos
}

// SeekBegin starts the drag at share.
func (s *Scrub) SeekBegin(share float32) {
	s.begin(share)
	s.Repaint()
}

// SeekMove follows the finger to share.
func (s *Scrub) SeekMove(share float32) {
	if s.move(share) {
		s.Repaint()
	}
}

// SeekEnd ends the drag: OnSeek once for commit, and the bar stays where the
// finger let go until the player tells its position.
func (s *Scrub) SeekEnd(commit bool) {
	if share, was := s.end(commit); was {
		if commit {
			s.pos = share
		}
		s.Repaint()
	}
}

// SetBand shows the A-B band from a to b (shares), or hides it when on is false.
func (s *Scrub) SetBand(on bool, a, b float32) {
	a, b = clamp01(a), clamp01(b)
	if s.band != on || (on && (s.a != a || s.b != b)) {
		s.band, s.a, s.b = on, a, b
		s.Repaint()
	}
}

// SetBMin sets the least share the B marker can be dragged to: A plus the loop's
// least length, as a share of the length (the player keeps B there too).
func (s *Scrub) SetBMin(share float32) { s.bMin = clamp01(share) }

// BHit tells whether a press at x, in the bar's coordinates, takes the B marker:
// the band shows and the press is within the touch target of the marker, which
// wins over the playhead's knob where they meet.
func (s *Scrub) BHit(x float32) bool {
	w := s.Frame().W
	if !s.band || s.b <= s.a || w <= 0 {
		return false
	}
	d := x - s.b*w
	if d < 0 {
		d = -d
	}

	return d <= scrubMarkReach
}

// BBegin, BMove and BEnd are the drag of the B marker: it follows the finger along
// the bar, never before bMin, and lets go (commit) with one OnBSet. The playhead
// does not move and nothing is asked of the player but that.
func (s *Scrub) BBegin(share float32) {
	s.bOn = true
	s.bShare = max(clamp01(share), s.bMin)
	s.Repaint()
}

// BMove follows the finger to share.
func (s *Scrub) BMove(share float32) {
	if share = max(clamp01(share), s.bMin); s.bOn && share != s.bShare {
		s.bShare = share
		s.Repaint()
	}
}

// BEnd lets go of the marker: OnBSet once for commit; the band stays where the
// finger left it until the screen sets it from the player.
func (s *Scrub) BEnd(commit bool) {
	if !s.bOn {
		return
	}
	s.bOn = false
	if commit {
		s.b = max(s.bShare, s.a)
		if s.OnBSet != nil {
			s.OnBSet(s.bShare)
		}
	}
	s.Repaint()
}

// BDragging tells whether the B marker is being dragged.
func (s *Scrub) BDragging() bool { return s.bOn }

// endB is the B end of the band as drawn: the finger's while it drags.
func (s *Scrub) endB() float32 {
	if s.bOn {
		return s.bShare
	}

	return s.b
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

// HandlePointer is the press, the drag and the release on the bar itself (a
// screen puts a SeekPad over it for the touch target).
func (s *Scrub) HandlePointer(_ *ui.Context, e *ui.PointerEvent) {
	handleSeek(e, s, s.Frame().W)
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
	if b := s.endB(); s.band && b > s.a {
		p.Fill(geom.XYWH(s.a*r.W, scrubBandTop, (b-s.a)*r.W, scrubBandH), scene.Solid(look.T.Red.WithAlpha(scrubBandAlpha)), scrubBandRadius)
	}
	fw := s.Shown() * r.W
	if fw > 0 {
		p.Fill(geom.XYWH(0, scrubTrackTop, fw, scrubTrackH), scene.Solid(look.T.Primary), scrubTrackRadius)
	}
	s.knob.Duration, s.knob.Ease = scrubKnobTime, theme.CSSEase
	if c.ReduceMotion() {
		s.knob.Duration = -1 // the knob jumps
	}
	grow := float32(0)
	if s.on {
		grow = 1
	}
	d := scrubKnob * (1 + (scrubKnobDrag-1)*s.knob.Track(s, grow)) // grows about its center
	p.Fill(geom.XYWH(fw-d/2, scrubKnobTop+scrubKnob/2-d/2, d, d), scene.Solid(look.T.Primary), d/2)
	if b := s.endB(); s.band && b > s.a {
		mw, mh := scrubMarkW, scrubMarkH
		if s.bOn {
			mw, mh = scrubMarkDragW, scrubMarkDragH
		}
		p.Fill(geom.XYWH(b*r.W-mw/2, scrubKnobTop+scrubKnob/2-mh/2, mw, mh), scene.Solid(look.T.Red), mw/2)
	}
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
