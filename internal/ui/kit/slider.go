// Adapted from jplayer-mobile/internal/ui/kit/range.go at 49de3c2 (a bar a finger
// takes at once: `DragExempt` so that no sheet or scroll around it takes the drag,
// the label, the semantics). The behaviour is gDesk's headless.Slider (the value,
// the step, the pointer and the keys); the look is `input[type=range]` of the
// prototype (index.html:171-173).

package kit

import (
	"dancefitvn-app/internal/ui/theme"

	"github.com/gdeskdev/desk/foundation/core/geom"
	"github.com/gdeskdev/desk/foundation/scene"
	"github.com/gdeskdev/desk/framework/components/headless"
	"github.com/gdeskdev/desk/framework/ui"
)

// ====================================================================
// ============================ Contracts =============================
// ====================================================================

// The numbers of `input[type=range]` (index.html:171-173).
const (
	// sliderTrack, sliderTrackRadius are `height:6px;border-radius:3px` of the
	// input: the track, which is the box layout.json records as `input`.
	sliderTrack, sliderTrackRadius float32 = 6, 3
	// sliderThumb is `::-webkit-slider-thumb{width:20px;height:20px;border-radius:50%}`;
	// the thumb's middle stops one radius from each end of the track.
	sliderThumb float32 = 20
	// sliderThumbRise is how far the thumb's top stands over the track's: Chrome puts
	// the 20 px thumb of a 6 px track at y 660 where the track is at 669 (M15D, the
	// thumb's red pixels from 660.3 to 679.3), not centred on it.
	sliderThumbRise float32 = 9
)

// The names layout.json gives the slider: its track is the `<input>`.
const (
	nameSlider      = "zoom"  // `<input type="range" id="zoom">`, the whole touch area
	nameSliderTrack = "input" // the input's own box
)

// Slider is `input[type=range]` of the avatar sheet (index.html:171-173, :523): a
// 6 tall track of --sf2, 3 rounded, and a 20 round thumb of --red whose middle runs
// from 10 in from the track's start to 10 in from its end. The value is a Min to
// Max in steps of Step, set by a press or a drag along the track, or by the arrow
// keys; OnChange hears the user's changes (SetValue's are silent). The widget is as
// tall as the thumb, so that a finger finds it; the track is the 6 DIPs the
// prototype's input is, 9 down from the top of that. A press on it never starts the
// drag of a sheet around it (DragExempt).
type Slider struct {
	headless.Slider
	// Label names the slider for assistive technologies.
	Label string

	track    sliderBar
	attached bool
}

// sliderBar is the track: the `<input>` box.
type sliderBar struct {
	ui.Base
}

var (
	_ ui.Labeler = (*Slider)(nil)
)

// ====================================================================
// ============================= Creation =============================
// ====================================================================

// NewSlider returns a slider from min to max in steps of step, at value.
func NewSlider(minV, maxV, step, value float64) *Slider {
	s := &Slider{}
	s.Min, s.Max, s.Step = minV, maxV, step
	s.Value = value

	return s
}

// ====================================================================
// ========================== Main functions ==========================
// ====================================================================

// AccessibleLabel returns the slider's label.
func (s *Slider) AccessibleLabel() string { return s.Label }

// DragExempt tells that a press on the slider never starts the drag of a sheet
// around it.
func (s *Slider) DragExempt() bool { return true }

// Layout takes the room's width and the thumb's height, and puts the track in it.
func (s *Slider) Layout(c *ui.Context, cs ui.Constraints) geom.Size {
	if !s.attached {
		s.attached = true
		s.track.Name = nameSliderTrack
		s.Add(&s.track)
	}
	if s.Name == "" {
		s.Name = nameSlider
	}
	s.Inset = sliderThumb / 2
	w := cs.Max.W
	if w >= ui.Unbounded {
		w = 160
	}
	size := cs.Constrain(geom.Sz(w, sliderThumb))
	c.Layout(&s.track, ui.Tight(geom.Sz(size.W, sliderTrack)))
	c.Place(&s.track, geom.Pt(0, sliderThumbRise))

	return size
}

// Paint draws the thumb over the track.
func (s *Slider) Paint(c *ui.Context, p *ui.Painter) {
	look := theme.From(c)
	s.Base.Paint(c, p) // the track
	r := s.Bounds()
	x := s.Inset + float32(s.Fraction())*(r.W-2*s.Inset)
	thumb := geom.XYWH(x-sliderThumb/2, 0, sliderThumb, sliderThumb)
	p.Fill(thumb, scene.Solid(look.T.Red), sliderThumb/2)
	if s.FocusVisible() {
		focusRing(p, thumb, sliderThumb/2, look.T.Primary)
	}
}

// Paint draws the track.
func (b *sliderBar) Paint(c *ui.Context, p *ui.Painter) {
	r := b.Bounds()
	p.Fill(r, scene.Solid(theme.From(c).T.Surface2), capRadius(r, sliderTrackRadius))
}
