//go:build !tvos

package screens

import (
	"dancefitvn-app/internal/ui/kit"

	"github.com/gdeskdev/desk/foundation/core/geom"
	"github.com/gdeskdev/desk/framework/ui"
)

// ====================================================================
// ============================ Contracts =============================
// ====================================================================

// hiddenSeek is the press the video screen carries itself on the position bar
// while the controls are hidden (K38): their buttons take no touch then
// (`.scr.vid.hid .vui{pointer-events:none}`), so the bar cannot take the press;
// the screen watches it instead. A press that stays inside the slop is the tap on
// the picture, as before; one that goes sideways first is a drag of the bar: the
// controls show and the bar follows the finger.
type hiddenSeek struct {
	// armed tells that the press began on the bar's touch target.
	armed bool
	// active tells that the drag of the bar has begun.
	active bool
}

// ====================================================================
// ========================= Helper Functions =========================
// ====================================================================

// seekTarget is the second a position share of the bar means, kept one second
// short of the end: a finger that reaches the end of the bar (or lets go past
// it) lands where the media still has a second to play, not on its end, which
// would end the lesson and its auto next with no one having watched it (the
// ±10 s buttons stop at the length less 1 as well, `$P:607`). A media with no
// length is not sought.
func seekTarget(share float32, dur float64) (sec float64, ok bool) {
	if dur <= 0 {
		return 0, false
	}
	sec = float64(share) * dur
	if dur > 1 {
		sec = min(sec, dur-1)
	}

	return max(sec, 0), true
}

// begin is the press on the bar's touch target while the controls are hidden;
// pos is in the screen's coordinates.
func (h *hiddenSeek) begin(pad *kit.SeekPad, screen ui.Widget, pos geom.Point) {
	h.active = false
	h.armed = pad.Hit(pos.Add(screen.Node().WindowFrame().Min()))
}

// absf returns |v|.
func absf(v float32) float32 {
	if v < 0 {
		return -v
	}

	return v
}
