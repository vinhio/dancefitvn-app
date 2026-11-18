//go:build tvos

package screens

import (
	"time"

	"dancefitvn-app/internal/ui/kit"
	"dancefitvn-app/internal/ui/tv/focus"
)

// ====================================================================
// ============================ Contracts =============================
// ====================================================================

// The scrub of the position bar (K43; the prototype's TV bar does not seek).
const (
	// keyPlayerScrub is the key of the bar, which takes the focus above `p1`.
	keyPlayerScrub = "ps"
	// playerScrubStep is what a Left or a Right moves the position by: the 10 s of
	// the phone's ‹10 and 10› buttons (`data-v`, index.html:430). A swipe on the
	// remote's touch surface is an arrow for each quarter of the surface it
	// crosses, so it moves by as many steps; a held arrow repeats every 125 ms.
	playerScrubStep = kit.SeekStep
	// playerScrubIdle is how long the arrows stop before the player is asked for
	// the position they reached: one seek for a swipe or a held arrow, not one for
	// each step. It is longer than the 400 ms the driver waits to repeat a held
	// arrow, so a hold is one scrub.
	playerScrubIdle = 700 * time.Millisecond
)

// ====================================================================
// ========================== Main functions ==========================
// ====================================================================

// scrubMove is Left and Right on the bar: they step the position back and on.
// Up and Down are left to the focus.
func (s *Player) scrubMove(d focus.Dir) bool {
	switch d {
	case focus.DirLeft:
		s.scrubBy(-playerScrubStep)
	case focus.DirRight:
		s.scrubBy(playerScrubStep)
	case focus.DirUnknown, focus.DirUp, focus.DirDown:
		return false
	default:
		return false
	}

	return true
}

// scrubStep is a screen reader's increment or decrement: the step, applied at
// once.
func (s *Player) scrubStep(dir int) {
	s.scrubBy(float64(dir) * playerScrubStep)
	s.commitScrub()
}

// scrubSelect is Select on the bar: it applies the position that is being
// scrubbed to, and with none under way it is the Play/Pause button.
func (s *Player) scrubSelect() {
	if s.scrubbing {
		s.commitScrub()

		return
	}
	s.pb.Toggle()
}

// scrubBy moves the position being scrubbed to by d seconds (from where the
// player is when none is), shows it on the clock and the bar, and arms the seek
// for when the arrows stop. It allocates nothing.
func (s *Player) scrubBy(d float64) {
	dur := s.length()
	if dur <= 0 {
		return // nothing to scrub in before the length is known
	}
	if !s.scrubbing {
		s.scrubbing, s.target = true, s.pb.Pos()
	}
	s.target = min(max(s.target+d, 0), max(dur-1, 0)) // Player.SeekBy keeps one second before the end
	s.syncLive()
	if s.ctx == nil {
		s.commitScrub()

		return
	}
	s.last = s.ctx.Now()
	s.arm(playerScrubIdle)
}

// arm waits d for the seek, unless it is waiting already: a later arrow only
// notes its time (last), and scrubDue waits for what is left, so that a held arrow
// makes no timer of its own at each repeat.
func (s *Player) arm(d time.Duration) {
	if s.timer.Pending() {
		return
	}
	if s.commit == nil {
		s.commit = s.scrubDue
	}
	s.timer = s.ctx.AfterFunc(d, s.commit)
}

// scrubDue is the timer of the seek: the arrows have stopped for playerScrubIdle
// and the position is asked for, or they went on and it waits for the rest.
func (s *Player) scrubDue() {
	if !s.scrubbing {
		return
	}
	if left := s.last + playerScrubIdle - s.ctx.Now(); left > 0 {
		s.arm(left)

		return
	}
	s.commitScrub()
}

// commitScrub asks the player for the position that was scrubbed to, once.
func (s *Player) commitScrub() {
	s.timer.Stop()
	if !s.scrubbing {
		return
	}
	s.scrubbing = false
	s.pb.Seek(s.target)
	s.syncLive() // the player's own, when it did not say
	s.touch()    // K46: the 3 s of the chrome begin from the seek
}

// dropScrub forgets a scrub under way: its lesson is over or gone.
func (s *Player) dropScrub() {
	s.timer.Stop()
	if s.scrubbing {
		s.scrubbing = false
		s.sec = -1
	}
}

// length is the length of the lesson in seconds as the clock shows it.
func (s *Player) length() float64 {
	if dur := s.pb.Dur(); dur > 0 {
		return dur
	}

	return float64(s.pb.Lesson().DurationSec)
}
