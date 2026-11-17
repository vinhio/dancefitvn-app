// Adapted from jplayer-mobile/internal/ui/kit/switch.go at 49de3c2.

package kit

import (
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

// The numbers of .sw (index.html:118-120).
const (
	// switchW, switchH are `.sw{width:51px;height:31px}` (:118).
	switchW, switchH float32 = 51, 31
	// switchRadius is `.sw{border-radius:16px}` (:118).
	switchRadius float32 = 16
	// switchBorder is `.sw{border:1px solid var(--bd)}` (:118).
	switchBorder float32 = 1
	// switchKnob is `.sw:after{width:24px;height:24px}` (:119).
	switchKnob float32 = 24
	// switchKnobTop and switchKnobOff are `.sw:after{top:2.5px;left:3px}` (:119)
	// and switchKnobOn is `.sw.on:after{left:23px}` (:120): offsets from the
	// padding box, which lies inside the 1 px border.
	switchKnobTop, switchKnobOff, switchKnobOn float32 = 2.5, 3, 23
)

// The names layout.json gives the switch: tag and classes (`span.sw`,
// `span.sw.on`).
const (
	nameSwitch   = "span.sw"
	nameSwitchOn = "span.sw.on"
)

// ====================================================================
// ============================= Creation =============================
// ====================================================================

// Switch is `.sw` (index.html:118-120): a 51 x 31 pill of --sf2 with a --bd
// border and a white knob that, when on, turns --pr with a --on knob 20 DIPs to
// the right. It shows On and nothing else: the Row it is in takes the press and
// the page flips On. Colors and knob move together over `transition:.2s`, CSS
// `ease`.
type Switch struct {
	ui.Base
	// On is the state; change it with SetOn.
	On bool

	k ui.Animated[float32] // how far the switch has gone toward on, 0 to 1
}

// ====================================================================
// ========================== Main functions ==========================
// ====================================================================

// NewSwitch returns a switch in the state on, shown at once (not sliding in).
func NewSwitch(on bool) *Switch {
	s := &Switch{On: on}
	s.k.Set(flag(on))

	return s
}

// SetOn turns the switch on or off; the knob and the colors move over 0.2 s.
// The same state changes nothing.
func (s *Switch) SetOn(on bool) {
	if on != s.On {
		s.On = on
		s.autoName()
		s.Repaint()
	}
}

// Knob returns where the knob's left edge and top are and its size, in the
// switch's coordinates, as of the last paint: a test reads it.
func (s *Switch) Knob() (x, y, size float32) {
	k := s.k.Value()

	return switchBorder + switchKnobOff + k*(switchKnobOn-switchKnobOff), switchBorder + switchKnobTop, switchKnob
}

// Describe describes the switch as a switch, on or off.
func (s *Switch) Describe(_ *ui.Context, n *platform.SemanticNode) {
	n.Role = platform.RoleSwitch
	n.Toggle = platform.ToggledOff
	if s.On {
		n.Toggle = platform.ToggledOn
	}
}

// Layout takes the fixed size, and the name `span.sw` or `span.sw.on` unless the
// caller gave one.
func (s *Switch) Layout(_ *ui.Context, cs ui.Constraints) geom.Size {
	s.autoName()

	return cs.Constrain(geom.Sz(switchW, switchH))
}

// Paint draws the track and the knob, which move together toward the state.
func (s *Switch) Paint(c *ui.Context, p *ui.Painter) {
	look := theme.From(c)
	t := &look.T
	s.k.Duration, s.k.Ease = theme.SwitchDur, theme.CSSEase
	k := s.k.Track(s, flag(s.On))
	r := s.Bounds()
	p.Fill(r, scene.Solid(t.Surface2.Mix(t.Primary, k)), switchRadius)
	p.Border(r, geom.Uniform(switchBorder), t.Border.Mix(t.Primary, k), switchRadius)
	x, y, d := s.Knob()
	p.Fill(geom.XYWH(x, y, d, d), scene.Solid(color.White.Mix(t.OnPrimary, k)), d/2)
}

// ====================================================================
// ========================= Helper Functions =========================
// ====================================================================

// autoName keeps the name in step with the state while the caller set none.
func (s *Switch) autoName() {
	if s.Name == "" || s.Name == nameSwitch || s.Name == nameSwitchOn {
		if s.On {
			s.Name = nameSwitchOn
		} else {
			s.Name = nameSwitch
		}
	}
}
