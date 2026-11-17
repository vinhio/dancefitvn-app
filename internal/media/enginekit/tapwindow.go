// Adapted from jplayer-mobile/internal/media/enginekit/tapwindow.go at 49de3c2.

package enginekit

import "math"

// ====================================================================
// ============================ Contracts =============================
// ====================================================================

// tapBehind is how far the newest sample of a tap may be behind the
// player's time and still stand for it (the tap of iOS processes ahead of
// the output; right after a seek it has not caught up yet).
const tapBehind = 0.1 // seconds

// ====================================================================
// ========================== Main functions ==========================
// ====================================================================

// TapWindow returns where the n samples to analyse start in a ring of
// size samples that holds the count samples written so far (the newest
// at count-1), so that the window ends at the player's time pos: end is
// the media's time just after the newest sample and step the media's time
// one sample stands for (an audio tap runs ahead of what is heard). When
// end or step is unknown (NaN, not positive), the window is the newest. It
// returns false when the ring does not hold the window: fewer than n
// samples yet, the time long gone from the ring, or well past the newest.
func TapWindow(count uint64, end, step, pos float64, n, size int) (uint64, bool) {
	if n <= 0 || n > size || count < uint64(n) {
		return 0, false
	}
	to := count
	if !math.IsNaN(end) && !math.IsInf(end, 0) && step > 0 && !math.IsNaN(pos) {
		lead := (end - pos) / step // samples from the player's time to the newest
		switch {
		case lead < -tapBehind/step:
			return 0, false
		case lead <= 0:
		case lead >= float64(count):
			return 0, false
		default:
			to = count - uint64(math.Round(lead))
		}
	}
	oldest := uint64(0)
	if count > uint64(size) {
		oldest = count - uint64(size)
	}
	if to < uint64(n) || to-uint64(n) < oldest {
		return 0, false
	}

	return to - uint64(n), true
}
