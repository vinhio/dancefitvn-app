//go:build tvos

// The auto-hide of the video's controls on the TV (K46, owner 2026-11-18). The
// prototype's TV07 does not hide them (docs/plan/03 section 5a); the phone's
// screen does (Q12: `CTL_HIDE_MS`, `.vui{transition:opacity .25s}`), and its rules
// are the ones taken here: it hides while the video plays and nothing counts
// otherwise. What jPlayer TV has tested on a real remote (watch/idle.go at
// 781bee5, Pre of the focus Manager) decides what the first button does.

package screens

import (
	"time"

	"dancefitvn-app/internal/player"
	"dancefitvn-app/internal/ui/tv/focus"

	"github.com/gdeskdev/desk/foundation/platform"
)

// ====================================================================
// ============================ Contracts =============================
// ====================================================================

// The numbers of the auto-hide of TV07.
const (
	// playerChromeIdle is `CTL_HIDE_MS=3000` (index.html:554): the chrome hides
	// this long after the last button while the video plays.
	playerChromeIdle = 3000 * time.Millisecond
	// playerChromeFade is `.vui{transition:opacity .25s}` (index.html:146), CSS
	// `ease`; the system's Reduce Motion makes it a jump (ui.Animated).
	playerChromeFade = 250 * time.Millisecond
)

// ====================================================================
// ========================== Main functions ==========================
// ====================================================================

// Chrome tells whether the controls are shown (not hidden, nor fading out).
func (s *Player) Chrome() bool { return !s.hid }

// ChromeOpacity is the chrome's opacity at this paint, 1 shown to 0 hidden.
func (s *Player) ChromeOpacity() float32 { return s.fade.Value() }

// chromeKey is the hook that sees every button of the remote before the focus
// does (env.Actions.OnKey). Any button restarts the 3 s. When the chrome is
// hidden the button only brings it back and does nothing else (arrows, Select,
// Menu: no move, no press, the screen is not left, and the focus stays where it
// was); Play/Pause brings it back and goes on to toggle, as in jPlayer TV. With the
// chrome shown every button goes on as it did.
func (s *Player) chromeKey(b focus.Button) bool {
	was := s.hid
	s.wake()
	if !was {
		return false
	}

	return b != focus.ButtonPlayPause
}

// ====================================================================
// ========================= Helper Functions =========================
// ====================================================================

// hideable tells whether the chrome may hide now: the video plays (the picture is
// moving), the panel is not open, and no scrub waits to be applied. A pause, a
// load, an end or an error leaves the controls up and nothing counts (03 section
// 5a).
func (s *Player) hideable() bool {
	return s.pb.Playing() && s.pb.State() == player.StatePlaying && s.panel == nil && !s.scrubbing
}

// touch notes a button at this moment and starts the wait, if it is not running:
// a later button only moves the note (the wait looks at it when it ends), so that
// a held arrow makes no timer of its own at each repeat and allocates nothing.
func (s *Player) touch() {
	if s.ctx == nil {
		return
	}
	s.touched = s.ctx.Now()
	s.armChrome(playerChromeIdle)
}

// armChrome waits d for the hide, unless it is waiting already.
func (s *Player) armChrome(d time.Duration) {
	if s.idle.Pending() || s.ctx == nil {
		return
	}
	s.idle = s.ctx.AfterFunc(d, s.idleFn)
}

// wake shows the chrome (fading it in when it was hidden) and restarts the 3 s.
func (s *Player) wake() {
	if s.hid {
		s.hid = false
		s.fade.To(s, 1)
		s.chromeTell()
	}
	s.touch()
}

// chromeDue is the end of the wait: the 3 s have gone since the last button and
// the chrome hides, or a button came meanwhile and it waits for what is left. A
// chrome that may not hide now (a pause, the panel, a scrub waiting) stays and
// waits for what changes that: stateChrome, the panel closing, the scrub applied.
func (s *Player) chromeDue() {
	if s.ctx == nil || s.hid {
		return
	}
	if left := s.touched + playerChromeIdle - s.ctx.Now(); left > 0 {
		s.armChrome(left)

		return
	}
	if !s.hideable() {
		return
	}
	s.hid = true
	s.fade.To(s, 0)
	s.chromeTell()
}

// stateChrome follows the player's state: anything but a video that plays shows
// the chrome and leaves nothing to count (`Playing` is what starts the 3 s; a
// chrome already hidden stays hidden through a stall and its resume, as the phone's
// `State` does).
func (s *Player) stateChrome() {
	if s.hideable() {
		if !s.hid {
			s.touch()
		}

		return
	}
	s.idle.Stop()
	if s.hid {
		s.hid = false
		s.fade.To(s, 1)
		s.chromeTell()
	}
}

// chromeTell tells a screen reader that the controls came or went (they are out of
// its reading from that moment, not after the fade, kit.Hush).
func (s *Player) chromeTell() {
	if s.ctx != nil {
		s.ctx.SemanticsChanged(platform.ChangeLayout)
	}
}
