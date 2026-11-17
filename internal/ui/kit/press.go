// Adapted from jplayer-mobile/internal/ui/kit/press.go at 49de3c2.

package kit

import (
	"time"

	"dancefitvn-app/internal/ui/theme"

	"github.com/gdeskdev/desk/framework/ui"
)

// ====================================================================
// ============================ Contracts =============================
// ====================================================================

// pressSpec is how a widget looks while a finger is on it (CSS :active): a
// scale, with the time it takes to go to it and back. A zero time jumps.
type pressSpec struct {
	// Scale is the size a pressed widget has; 0 or 1 for none.
	Scale float32
	// In and Out are the times of the scale going to Scale and back to 1.
	In, Out time.Duration
}

// pressButton is `.btn:active{transform:scale(.98)}` (index.html:63): the rule
// has no transition, so the scale applies and goes at once.
var pressButton = pressSpec{Scale: theme.PressScale}

// ====================================================================
// ============================= Creation =============================
// ====================================================================

// pressFX follows a press: the scale a pressSpec describes. A widget keeps one,
// asks it at every paint and draws what it returns.
type pressFX struct {
	scale  ui.Animated[float32]
	placed bool
}

// ====================================================================
// ========================== Main functions ==========================
// ====================================================================

// track moves the scale toward its target for pressed and returns where it is
// now: the scale to draw the widget at (1 at rest). The first call puts it at
// rest at once. A spec without times jumps. It does not allocate.
func (f *pressFX) track(w ui.Widget, pressed bool, s *pressSpec) float32 {
	if !f.placed {
		f.scale.Set(1)
		f.placed = true
	}
	to, dur := float32(1), s.Out
	if pressed {
		dur = s.In
		if s.Scale > 0 {
			to = s.Scale
		}
	}
	if to != f.scale.Target() {
		if dur <= 0 {
			f.scale.Set(to)
		} else {
			f.scale.Duration, f.scale.Ease = dur, theme.CSSEase
			f.scale.To(w, to)
		}
	}

	return f.scale.Value()
}
