//go:build tvos

package tvkit

import (
	"dancefitvn-app/internal/ui/kit"
	"dancefitvn-app/internal/ui/theme"

	"github.com/gdeskdev/desk/foundation/core/color"
	"github.com/gdeskdev/desk/foundation/core/geom"
	"github.com/gdeskdev/desk/foundation/platform"
	"github.com/gdeskdev/desk/foundation/scene"
	"github.com/gdeskdev/desk/framework/ui"
)

// ====================================================================
// ============================ Contracts =============================
// ====================================================================

// The numbers of the player's position bar (index.html:685, `.scrub` with
// `style="height:24px;margin-top:10px"`, its `.tr` and `.fl` at `height:8px`,
// and `.scrub .tr{top:7px;border-radius:2px;background:rgba(255,255,255,.3)}`,
// `.scrub .fl{top:7px;border-radius:2px;background:var(--pr)}`, :152-154). The
// TV's has no knob, no loop span and no seeking.
const (
	// ScrubHeight is `.scrub{height:24px}`.
	ScrubHeight float32 = 24
	// scrubTop, scrubTrack, scrubRadius are `top:7px`, `height:8px` and
	// `border-radius:2px`.
	scrubTop, scrubTrack, scrubRadius float32 = 7, 8, 2
	// scrubKnob, scrubKnobRing are the focused bar's knob (K43), as tall as the
	// bar's box, and the --pr ring around it.
	scrubKnob, scrubKnobRing float32 = ScrubHeight, 4
)

// scrubTrackInk is `rgba(255,255,255,.3)`.
var scrubTrackInk = color.RGBA(255, 255, 255, .3)

// Scrub is the player's position bar: a track of white at .3 and a --pr fill as
// wide as Value of it. It is a control (K43): the remote's Left and Right and a
// swipe on its touch surface (each a quarter of the surface is an arrow) reach
// OnMove, with which the screen scrubs; Item.Label names the bar for a screen
// reader ("Vị trí phát"). Focused it has a knob on the fill's end. It does not
// allocate when the value moves.
type Scrub struct {
	Item
	// Value is how much of the track the fill covers, 0 to 1; change it with
	// SetValue.
	Value float32
	// Spoken is the reading of the bar ("01:12 / 03:40"), made by the screen when
	// the clock moves to another second; empty reads as the percent of Value.
	Spoken string
	// OnStep is a screen reader's increment (+1) or decrement (-1): the screen
	// seeks by its step and applies it at once.
	OnStep func(dir int)
	// OnBlur is called when the focus leaves the bar: the screen applies a scrub
	// that is under way.
	OnBlur func()
}

// ====================================================================
// ============================= Creation =============================
// ====================================================================

// NewScrub returns a bar with the fill at 0.
func NewScrub() *Scrub { return &Scrub{} }

// FocusChanged shows the knob and tells OnBlur when the focus goes.
func (s *Scrub) FocusChanged(c *ui.Context, focused bool) {
	s.Item.FocusChanged(c, focused)
	if !focused && s.OnBlur != nil {
		s.OnBlur()
	}
}

// PerformAction is a screen reader's increment or decrement through OnStep, any
// other action Item's.
func (s *Scrub) PerformAction(c *ui.Context, a platform.SemanticAction) bool {
	switch {
	case a == platform.ActIncrement && s.OnStep != nil:
		s.OnStep(1)
	case a == platform.ActDecrement && s.OnStep != nil:
		s.OnStep(-1)
	default:
		return s.Item.PerformAction(c, a)
	}

	return true
}

// ====================================================================
// ========================== Main functions ==========================
// ====================================================================

// SetValue moves the fill to v of the track, clamped to 0 and 1; the same value
// changes nothing.
func (s *Scrub) SetValue(v float32) {
	v = min(max(v, 0), 1)
	if v != s.Value {
		s.Value = v
		s.Repaint()
	}
}

// SetSpoken changes the reading of the bar and tells assistive technologies; the
// same text changes nothing.
func (s *Scrub) SetSpoken(txt string) {
	if txt != s.Spoken {
		s.Spoken = txt
		s.ValueChanged()
	}
}

// Describe describes the bar as an adjustable slider at Value of its length,
// valued by Spoken, else by the percent: it is a control now, so that VoiceOver
// reads it as the one it adjusts with a swipe.
func (s *Scrub) Describe(c *ui.Context, n *platform.SemanticNode) {
	s.Item.Describe(c, n)
	kit.DescribeBar(n, s.Item.Label, s.Spoken, s.Value)
	n.Role = platform.RoleSlider
	n.Actions |= platform.ActIncrement | platform.ActDecrement
}

// Layout takes the width it gets and 24.
func (s *Scrub) Layout(_ *ui.Context, cs ui.Constraints) geom.Size {
	w := cs.Max.W
	if w >= ui.Unbounded {
		w = cs.Min.W
	}

	return cs.Constrain(geom.Sz(w, ScrubHeight))
}

// Paint draws the track and the fill.
func (s *Scrub) Paint(c *ui.Context, p *ui.Painter) {
	r := s.Bounds()
	p.Fill(geom.XYWH(0, scrubTop, r.W, scrubTrack), scene.Solid(scrubTrackInk), scrubRadius)
	primary := theme.From(c).T.Primary
	w := r.W * s.Value
	if w > 0 {
		p.Fill(geom.XYWH(0, scrubTop, w, scrubTrack), scene.Solid(primary), scrubRadius)
	}
	if b := s.Blend(); b > 0 { // focused: the knob on the end of the fill
		half := scrubKnob / 2
		knob := geom.XYWH(min(max(w, half), r.W-half)-half, 0, scrubKnob, scrubKnob)
		ring := primary
		ring.A = uint8(float32(ring.A) * b)
		p.Shadow(knob, half, ui.Shadow{Spread: scrubKnobRing * b, Color: ring})
		p.Fill(knob, scene.Solid(color.RGB(255, 255, 255)), half)
	}
}
