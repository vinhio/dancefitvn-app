// Adapted from jplayer-mobile/internal/player/transport.go at 49de3c2.

package player

import (
	"dancefitvn-app/internal/media"
	"math"
)

// ====================================================================
// ========================== Main functions ==========================
// ====================================================================

// Play starts or resumes. The speed is given to the engine first: AVPlayer's
// play resets it to 1. A failed load is made again from where it was; a
// media at its end plays from the start.
func (p *Player) Play() {
	p.interrupted = false
	if p.closed || p.lesson.ID == "" {
		return
	}
	if p.background && p.kind == KindVideo {
		return // a remote Play cannot show a video nobody sees; a sound plays on
	}
	p.detached = false
	switch {
	case p.state == StateError:
		p.reload(true)
		p.retried = false
	case !p.loaded: // the address is on its way: play when it came
		p.want = true
		if p.state == StatePaused {
			p.setState(StateLoading)
		}
	default:
		p.want = true
		if p.state == StateEnded {
			p.pos = 0
			p.emit(ChangeTime)
		}
		_ = p.o.Engine.SetRate(p.rate)
		_ = p.o.Engine.Play()
	}
}

// Pause pauses, keeping the position. It ends the waiting for the end of an
// interruption.
func (p *Player) Pause() {
	was := p.interrupted
	p.interrupted = false
	p.pause()
	if was && !p.closed {
		p.emit(ChangeState) // the state is the same, but the hold of the interruption ended
	}
}

// pause is Pause without ending the wait for the end of an interruption.
func (p *Player) pause() {
	if p.closed || p.lesson.ID == "" || p.state == StateError {
		return
	}
	p.want = false
	if !p.loaded { // the address is on its way: it will load paused
		p.setState(StatePaused)

		return
	}
	_ = p.o.Engine.Pause()
}

// Toggle is the play button ($P:589).
func (p *Player) Toggle() {
	if p.want {
		p.Pause()
	} else {
		p.Play()
	}
}

// Seek moves to sec, kept in [0, dur] (a tap on the scrub bar, $P:627).
func (p *Player) Seek(sec float64) {
	if p.closed || p.lesson.ID == "" || math.IsNaN(sec) {
		return
	}
	hi := p.dur
	if hi <= 0 {
		hi = sec
	}
	p.seek(max(0, min(sec, hi)))
}

// SeekBy moves by d seconds, kept in [0, dur - 1] (the ‹10 and 10› buttons,
// $P:607). The position is read from the engine first.
func (p *Player) SeekBy(d float64) {
	if p.closed || p.lesson.ID == "" {
		return
	}
	p.syncTime()
	t := max(0, p.pos+d)
	if p.dur > 0 {
		t = min(p.dur-1, t)
	}
	p.seek(max(0, t))
}

// Leave is the shell closing the screen of the lesson: a video pauses, a
// sound plays on; the position is saved and the Saver forgets the lesson.
func (p *Player) Leave() {
	if p.closed || p.lesson.ID == "" {
		return
	}
	if p.kind == KindVideo {
		p.Pause()
	}
	p.saveLeave()
	p.detached = p.kind == KindVideo
}

// Background follows the app's lifecycle (Q-P4-17): going to the background
// pauses a video, a sound plays on; the position is saved either way, as the
// system may end the app there. The engine hears of it too.
func (p *Player) Background(on bool) {
	if p.closed {
		return
	}
	p.background = on
	if on && p.lesson.ID != "" {
		if p.kind == KindVideo {
			p.Pause()
		}
		p.syncTime()
		p.saveUpdate()
		p.savePause()
	}
	if b, ok := p.o.Engine.(media.Backgrounder); ok {
		b.SetBackground(on)
	}
}

// ====================================================================
// ========================= Helper Functions =========================
// ====================================================================

// seek moves the media to t, which is already kept inside it.
func (p *Player) seek(t float64) {
	p.pos = t
	if p.loaded {
		_ = p.o.Engine.Seek(t)
	} else { // the address is on its way: the load begins there
		p.start = t
	}
	if p.state == StateEnded {
		p.setState(StatePaused)
	}
	p.emit(ChangeTime)
	p.emit(ChangeSeek)
}
