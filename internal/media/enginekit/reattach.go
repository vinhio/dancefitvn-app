// Adapted from jplayer-mobile/internal/media/enginekit/reattach.go at 49de3c2.

package enginekit

import "time"

// ====================================================================
// ============================ Contracts =============================
// ====================================================================

// ReattachSlack is how far from the position a player was given its
// video Surface back at its exact seek may land before the engine seeks
// there again, in seconds, beyond what it played since: about a frame,
// more than an exact seek's own error (an audio frame, about 20 ms behind
// at most; the Galaxy A17 landed 5 to 20 ms ahead once it played on),
// less than the shortest jump measured (148 ms back, to the key frame
// just before).
const ReattachSlack = 0.05

// ReattachSeek guards the exact seek a player is given with its video
// Surface back from the background (internal/media/amp). Samsung's
// NuPlayer (Galaxy A17, c2.mtk.avc.decoder) seeks on its own as it makes
// its video decoder again (its "SecSeek", told to nobody), to a key frame
// near where it was: the one before as it plays, the one after, seconds
// on, when paused (2026-11-09: 11.485 s to 10.426 s playing, 5.436 s to
// 10.426 s paused). That seek may come after the engine's: the engine's
// is then told done after it, so its completion tells whether the player
// landed elsewhere, and the engine seeks there again, once. The zero
// ReattachSeek guards nothing. UI thread.
type ReattachSeek struct {
	seek int       // the number of the guarded seek among those asked, 0 for none
	at   float64   // where it went, seconds
	rate float64   // the speed it plays at from there, 0 when paused
	from time.Time // when it was asked
}

// ====================================================================
// ========================== Main functions ==========================
// ====================================================================

// Arm guards the seek numbered seek among those asked of a player (its
// count once asked), to at seconds, asked at now; the player plays on
// from there at rate (0 when it does not play).
func (r *ReattachSeek) Arm(seek int, at, rate float64, now time.Time) {
	*r = ReattachSeek{seek: seek, at: at, rate: max(rate, 0), from: now}
}

// Disarm guards nothing (the Surface taken away again).
func (r *ReattachSeek) Disarm() { *r = ReattachSeek{} }

// Check is told each seek completion: done the seeks the player told
// done, this one counted, asked those asked, pos the position it tells at
// now, seconds. It returns where to seek again and true when the
// completion is the guarded seek's, no seek followed it, and the player
// is more than ReattachSlack behind its target, or ahead of it by more
// than it played since it was asked and ReattachSlack; it disarms once
// the guarded seek is done or another followed it (the user's: it goes
// elsewhere). It allocates nothing.
func (r *ReattachSeek) Check(done, asked int, pos float64, now time.Time) (float64, bool) {
	if r.seek == 0 || done < r.seek {
		return 0, false
	}
	g := *r
	r.Disarm()
	if asked > done {
		return 0, false
	}
	played := max(now.Sub(g.from).Seconds(), 0) * g.rate
	if pos >= g.at-ReattachSlack && pos <= g.at+played+ReattachSlack {
		return 0, false
	}

	return g.at, true
}
