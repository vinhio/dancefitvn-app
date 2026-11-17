package kit

import (
	"math"

	"github.com/gdeskdev/desk/foundation/core/color"
	"github.com/gdeskdev/desk/foundation/core/geom"
	"github.com/gdeskdev/desk/foundation/scene"
	"github.com/gdeskdev/desk/framework/ui"
)

// ====================================================================
// ============================ Contracts =============================
// ====================================================================

// The numbers of `.vstage` (index.html:139-142), the K5 stage of screen 06: the
// prototype's stand-in for the video, which the app draws, without the dancer
// (K5), until the first frame of the real video.
const (
	// stageAngle is `.vstage{background:linear-gradient(160deg,...)}` (:139), in
	// degrees clockwise from up.
	stageAngle = 160
	// stageFloorShare is `.vstage .floor{height:28%}` (:141), and the wall's
	// `inset:0 0 28% 0` (:142) is the rest.
	stageFloorShare float32 = .28
	// stageStripeStart, stageStripeEnd, stagePeriod are `.vstage .wall{
	// background:repeating-linear-gradient(90deg,transparent 0 15.5%,
	// rgba(255,255,255,.05) 15.5% 16%)}` (:142): a stripe from 15.5 % to 16 % of
	// the width, every 16 %.
	stageStripeStart, stageStripeEnd, stagePeriod float32 = .155, .16, .16
	// stageStripes is how many stripes the 16 % period puts in the width: at
	// 15.5, 31.5, 47.5, 63.5, 79.5 and 95.5 %.
	stageStripes = 6
)

var (
	// stageTop and stageBottom are `#3A1A1C` and `#0E0E10` of the gradient (:139).
	stageTop    = color.RGB(0x3a, 0x1a, 0x1c)
	stageBottom = color.RGB(0x0e, 0x0e, 0x10)
	// stageStripe is `rgba(255,255,255,.05)` of the wall (:142).
	stageStripe = color.RGBA(255, 255, 255, .05)
	// stageFloor is `.vstage .floor{background:rgba(0,0,0,.4)}` (:141).
	stageFloor = color.RGBA(0, 0, 0, .4)
	// stageGlow is `.vstage:before{background:radial-gradient(ellipse at 50% 35%,
	// rgba(255,208,40,.32),transparent 60%)}` (:140).
	stageGlow = []Radial{{Shape: RadialEllipse, CX: .5, CY: .35, Color: color.RGBA(255, 208, 40, .32), End: .6}}
)

// stageGlowScale is how many times smaller than the box the glow is rendered
// before it is drawn stretched: a whole number (kit.Radial).
const stageGlowScale = 4

// ====================================================================
// ============================= Creation =============================
// ====================================================================

// Stage is `.vstage` (index.html:139-142) as the video's stand-in: the gradient
// of 160 degrees, the yellow glow, the wall's six stripes over the top 72 % and
// the floor's black over the rest. It has no dancer (K5). The paints are made
// when the size changes, never per frame.
type Stage struct {
	ui.Base

	grad  scene.Paint
	gradW float32
	gradH float32

	glow   *scene.Image
	glowW  int
	glowH  int
	glowOK bool
}

// NewStage returns the stage; set the Name to the key layout.json knows the
// box by (`stageTap`).
func NewStage() *Stage { return &Stage{} }

// ====================================================================
// ========================== Main functions ==========================
// ====================================================================

// Layout fills the room.
func (s *Stage) Layout(_ *ui.Context, cs ui.Constraints) geom.Size { return cs.Constrain(cs.Max) }

// Paint draws the gradient, the glow, the stripes and the floor.
func (s *Stage) Paint(_ *ui.Context, p *ui.Painter) {
	r := s.Bounds()
	if r.Empty() {
		return
	}
	s.prepare(r.W, r.H)
	p.Fill(r, s.grad, 0)
	if s.glow != nil {
		p.Image(s.glow, r, 1)
	}
	wallH := r.H * (1 - stageFloorShare)
	for i := range stageStripes {
		x := r.W * (stageStripeStart + float32(i)*stagePeriod)
		p.Fill(geom.XYWH(x, 0, r.W*(stageStripeEnd-stageStripeStart), wallH), scene.Solid(stageStripe), 0)
	}
	p.Fill(geom.XYWH(0, wallH, r.W, r.H-wallH), scene.Solid(stageFloor), 0)
}

// ====================================================================
// ========================= Helper Functions =========================
// ====================================================================

// prepare makes the gradient and the glow for a w by h box, once per size.
func (s *Stage) prepare(w, h float32) {
	if s.gradW != w || s.gradH != h || s.grad.Gradient == nil {
		s.gradW, s.gradH = w, h
		// CSS: the gradient line runs through the middle at the angle, and is as
		// long as |w sin a| + |h cos a|, so that the corners get the end colors.
		a := float64(stageAngle) * math.Pi / 180
		dx, dy := float32(math.Sin(a)), float32(-math.Cos(a))
		half := (float32(math.Abs(float64(w*dx))) + float32(math.Abs(float64(h*dy)))) / 2
		cx, cy := w/2, h/2
		s.grad = scene.Linear(geom.Pt(cx-dx*half, cy-dy*half), geom.Pt(cx+dx*half, cy+dy*half),
			scene.Stop{Color: stageTop, Offset: 0}, scene.Stop{Color: stageBottom, Offset: 1})
	}
	gw, gh := int(math.Round(float64(w)/stageGlowScale)), int(math.Round(float64(h)/stageGlowScale))
	if gw > 0 && gh > 0 && (!s.glowOK || s.glowW != gw || s.glowH != gh) {
		s.glow = RenderRadialsOver(stageGlow, gw, gh, s.glow)
		s.glowW, s.glowH, s.glowOK = gw, gh, true
	}
}

var _ ui.Widget = (*Stage)(nil)
