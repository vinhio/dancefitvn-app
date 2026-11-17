package kit

import (
	"dancefitvn-app/internal/format"

	"github.com/gdeskdev/desk/foundation/platform"
)

// ====================================================================
// ============================ Contracts =============================
// ====================================================================

// SeekStep is the 10 s that a screen reader's increment and decrement move a
// position bar by: the 10 of `data-v="-10"` and `data-v="10"` (index.html:430).
const SeekStep = 10

// ====================================================================
// ========================= Helper Functions =========================
// ====================================================================

// describeSeek describes a position bar as a slider named label, from 0 to the
// length of the media, at the position, which it speaks as a clock ("01:12"). A
// bar with no reading is a slider at 0 of 0.
func describeSeek(n *platform.SemanticNode, label string, reading func() (pos, dur float64)) {
	n.Role, n.Label = platform.RoleSlider, label
	n.Actions |= platform.ActIncrement | platform.ActDecrement
	if reading == nil {
		return
	}
	pos, dur := reading()
	n.Min, n.Max, n.Now = 0, dur, pos
	n.Value = format.Clock(pos)
}

// performSeek is the SemanticActor of a position bar: an increment moves the
// position SeekStep s on, a decrement as many back, through step; it tells
// whether it took the action.
func performSeek(a platform.SemanticAction, step func(delta float64)) bool {
	if step == nil {
		return false
	}
	switch a {
	case platform.ActIncrement:
		step(SeekStep)
	case platform.ActDecrement:
		step(-SeekStep)
	default:
		return false
	}

	return true
}
