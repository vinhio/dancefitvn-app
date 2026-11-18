// Adapted from jplayer-mobile/internal/ui/kit/anim.go at 49de3c2 (Spinner: a
// ring with an accent arc that turns, the phase an ui.Animated that the widget
// keeps going from Paint). Changed: the colors are the app's tokens (--pr arc
// over white at .22), Size and Border are the owner's choice of K47 (the
// prototype has no spinner), and it draws a resting arc under Reduce Motion.

package kit

import (
	"math"
	"time"

	"dancefitvn-app/internal/ui/theme"

	"github.com/gdeskdev/desk/foundation/core/geom"
	"github.com/gdeskdev/desk/foundation/scene"
	"github.com/gdeskdev/desk/framework/ui"
)

// ====================================================================
// ============================ Contracts =============================
// ====================================================================

// The numbers of the loading ring (K47: the prototype has none; jPlayer's
// `.spinner` is `animation:spin .8s linear infinite` with a quarter arc).
const (
	// SpinnerPeriod is one turn.
	SpinnerPeriod = 800 * time.Millisecond
	// spinnerDefaultSize and spinnerDefaultBorder are what Size and Border of 0
	// read as.
	spinnerDefaultSize, spinnerDefaultBorder float32 = 18, 2.5
	// spinnerArc is the quarter of the ring the accent paints.
	spinnerArc = math.Pi / 2
	// spinnerKappa is 4/3 tan(pi/8): the control distance of a quarter circle
	// drawn as one cubic curve.
	spinnerKappa = .5522847498
	// spinnerTrack is the alpha of the white ring the arc runs on.
	spinnerTrack = .22
)

// Spinner is a ring of --pr's arc turning over a faint white ring: the loading
// mark. It asks for a frame while it is painted, and none when motion is
// reduced (the arc stands at the top). It is for the UI thread.
type Spinner struct {
	ui.Base
	// Decor: a mark that something loads is not an element of a reading.
	Decor
	// Size is the side of the ring and Border the width of its line; 0 reads as
	// 18 and 2.5.
	Size, Border float32

	phase ui.Animated[float32]
	held  float32
	arc   scene.Path
}

// ====================================================================
// ============================= Creation =============================
// ====================================================================

// NewSpinner returns a ring of the side and the line.
func NewSpinner(size, border float32) *Spinner { return &Spinner{Size: size, Border: border} }

// ====================================================================
// ========================== Main functions ==========================
// ====================================================================

// Layout takes the fixed square.
func (s *Spinner) Layout(_ *ui.Context, cs ui.Constraints) geom.Size {
	side, _ := s.measures()

	return cs.Constrain(geom.Sz(side, side))
}

// Paint draws the ring and the arc, turned clockwise by the phase.
func (s *Spinner) Paint(c *ui.Context, p *ui.Painter) {
	look := theme.From(c)
	r := s.Bounds()
	_, bw := s.measures()
	p.Border(r, geom.Uniform(bw), theme.InkWhite.Of(look).WithAlpha(spinnerTrack), r.W/2)
	turn := s.turn(c)
	mid := -math.Pi/2 + float64(turn)*2*math.Pi // the top, turned
	a0, a1 := mid-spinnerArc/2, mid+spinnerArc/2
	rad := float64(r.W-bw) / 2
	cx, cy := float64(r.W)/2, float64(r.H)/2
	s0, c0 := math.Sincos(a0)
	s1, c1 := math.Sincos(a1)
	k := spinnerKappa * rad
	s.arc.Reset()
	s.arc.MoveTo(float32(cx+rad*c0), float32(cy+rad*s0))
	s.arc.CubicTo(
		float32(cx+rad*c0-k*s0), float32(cy+rad*s0+k*c0),
		float32(cx+rad*c1+k*s1), float32(cy+rad*s1-k*c1),
		float32(cx+rad*c1), float32(cy+rad*s1))
	p.StrokePath(&s.arc, scene.Stroke{Width: bw}, scene.Solid(look.T.Primary))
}

// turn returns where the arc is in its turn, from 0 to 1; 0 under Reduce Motion.
// It moves the phase on, which keeps the frames coming.
func (s *Spinner) turn(c *ui.Context) float32 {
	if c.ReduceMotion() {
		s.held = 0
		s.phase.Set(0)

		return 0
	}
	if !s.phase.Running() {
		from := s.held
		if from >= 1 || s.phase.Value() >= 1 {
			from = 0
		}
		s.phase.Set(from)
		s.phase.Duration, s.phase.Ease = max(time.Duration(float64(SpinnerPeriod)*float64(1-from)), time.Millisecond), ui.Linear
		s.phase.To(s, 1)
	}
	s.held = s.phase.Value()

	return s.held
}

// ====================================================================
// ========================= Helper Functions =========================
// ====================================================================

// measures returns the side and the line, the defaults for 0.
func (s *Spinner) measures() (side, border float32) {
	side, border = s.Size, s.Border
	if side <= 0 {
		side = spinnerDefaultSize
	}
	if border <= 0 {
		border = spinnerDefaultBorder
	}

	return side, border
}
