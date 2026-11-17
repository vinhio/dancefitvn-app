// Adapted from jplayer-mobile/internal/media/enginekit/tapwait.go at 49de3c2.

package enginekit

import "time"

// ====================================================================
// ============================ Contracts =============================
// ====================================================================

// TapStarve is how long a media whose audio tap is wanted may play with
// the tap giving nothing before its analyser gives up (false: the
// visualizer draws its synthetic bars). A tap gives its first buffer
// 0.1–0.3 s after the media starts; it gives none at all when the system
// plays elsewhere (AirPlay) or skips it (risk R4 of local/plans/M10.md).
const TapStarve = 1500 * time.Millisecond

// TapWait tells, for one load whose audio tap is wanted, whether it played
// TapStarve without the tap giving anything: until then its analyser
// answers silence (the prototype's analyser is real from the load on,
// player.js:70, :302-303). The zero TapWait has not played. UI thread; it
// allocates nothing.
type TapWait struct {
	played time.Duration
	from   time.Time // when the span playing now began, zero when paused
}

// ====================================================================
// ========================== Main functions ==========================
// ====================================================================

// SetPlaying tells that the media plays from now on (true) or no longer
// (false): only the time it plays counts.
func (w *TapWait) SetPlaying(playing bool, now time.Time) {
	switch {
	case playing && w.from.IsZero():
		w.from = now
	case !playing && !w.from.IsZero():
		w.played += max(now.Sub(w.from), 0)
		w.from = time.Time{}
	default:
	}
}

// TapAnswers tells whether the analyser of a load whose audio tap is
// wanted answers (true: the bytes of the tap, silence until it gives
// any) or gives up (false: synthetic bars): it answers before the item is
// ready, and once ready while the tap is attached (attached, false when
// the item has no audio track) and either gave samples (written) or the
// media did not play TapStarve without any (starved).
func TapAnswers(ready, attached bool, written uint64, starved bool) bool {
	switch {
	case !ready:
		return true
	case !attached:
		return false
	default:
		return written > 0 || !starved
	}
}

// Starved tells that the media played TapStarve or more by now: a tap
// that gave nothing by then gives nothing.
func (w *TapWait) Starved(now time.Time) bool {
	played := w.played
	if !w.from.IsZero() {
		played += max(now.Sub(w.from), 0)
	}

	return played >= TapStarve
}
