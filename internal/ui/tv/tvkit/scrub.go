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
)

// scrubTrackInk is `rgba(255,255,255,.3)`.
var scrubTrackInk = color.RGBA(255, 255, 255, .3)

// Scrub is the player's position bar: a track of white at .3 and a --pr fill as
// wide as Value of it. It does not allocate when the value moves.
type Scrub struct {
	ui.Base
	// Value is how much of the track the fill covers, 0 to 1; change it with
	// SetValue.
	Value float32
	// Label names the bar for a screen reader ("Vị trí phát"). The TV's bar takes
	// no focus and no seeking: it is a progress bar, not a slider.
	Label string
	// Spoken is the reading of the bar ("01:12 / 03:40"), made by the screen when
	// the clock moves to another second; empty reads as the percent of Value.
	Spoken string
}

// ====================================================================
// ============================= Creation =============================
// ====================================================================

// NewScrub returns a bar with the fill at 0.
func NewScrub() *Scrub { return &Scrub{} }

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

// Describe describes the bar as a progress bar at Value of its length, valued by
// Spoken, else by the percent (kit.DescribeBar).
func (s *Scrub) Describe(_ *ui.Context, n *platform.SemanticNode) {
	kit.DescribeBar(n, s.Label, s.Spoken, s.Value)
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
	if w := r.W * s.Value; w > 0 {
		p.Fill(geom.XYWH(0, scrubTop, w, scrubTrack), scene.Solid(theme.From(c).T.Primary), scrubRadius)
	}
}
