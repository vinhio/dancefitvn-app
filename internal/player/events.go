// Adapted from jplayer-mobile/internal/player/events.go at 49de3c2.

package player

import (
	"dancefitvn-app/internal/media"
	"math"
)

// ====================================================================
// ========================== Main functions ==========================
// ====================================================================

// onEvent handles what the engine reports. Events of an older load are
// dropped by their token. EventTime is the 250 ms tick and allocates nothing.
func (p *Player) onEvent(ev media.Event) {
	if p.closed || !p.loaded || ev.Token != p.token {
		return
	}
	switch ev.Kind {
	case media.EventTime:
		p.onTime(ev.Time)
	case media.EventDuration:
		p.onDuration(ev.Duration)
	case media.EventState:
		p.onState(ev.State)
	case media.EventEnded:
		p.onEnded()
	case media.EventError:
		p.onError(ev.Err)
	case media.EventUnknown, media.EventBuffered, media.EventVideoSize:
	default:
	}
}

// onTime is the tick: the position, the A–B loop ($P:779), the Saver.
func (p *Player) onTime(t float64) {
	if p.atEnd() {
		return // a late tick after the end must not lower the saved position
	}
	p.pos = t
	if p.loop && p.kind == KindVideo && p.pos >= p.b && p.state == StatePlaying {
		p.seek(p.a)
	}
	p.saveUpdate()
	p.emit(ChangeTime)
}

// onDuration takes the engine's length for the lesson's: the API's may
// differ (the mock videos last 12 s, the course data say 220). A resume
// position that does not fit it is dropped once, and A and B that lie beyond
// it are made again.
func (p *Player) onDuration(d float64) {
	if math.IsNaN(d) || math.IsInf(d, 0) || d <= 0 {
		return
	}
	p.dur = d
	if p.checkStart {
		p.checkStart = false
		if p.start > 0 && p.start >= WatchedRatio*d {
			p.seek(0)
		}
	}
	if p.a > d || p.b > d {
		p.a = math.Round(d * loopStart)
		p.b = min(p.a+loopLength, d)
		p.emit(ChangeOptions)
	}
	p.emit(ChangeDuration)
}

// onState maps the engine's state onto the player's. A pause at the end is
// not a change of state.
func (p *Player) onState(s media.PlayState) {
	switch s {
	case media.StatePlaying:
		p.want, p.retried = true, false
		p.setState(StatePlaying)
	case media.StatePaused:
		if p.state == StateEnded {
			return
		}
		was := p.state
		p.want = false
		p.syncTime()
		p.setState(StatePaused)
		if was == StatePlaying {
			p.saveUpdate()
			p.savePause()
		}
	case media.StateLoading:
		if p.state != StateEnded {
			p.setState(StateLoading)
		}
	case media.StateUnknown, media.StateEnded, media.StateError:
	default:
	}
}

// onEnded is the media played to its end: the last position is saved (it is
// at or over 90 %, so the lesson counts as watched) and the shell hears
// ChangeEnded. With the A–B loop on, a video wraps to A instead. A video goes to the end-of-lesson screen; a sound stays on 08,
// stopped at its end ($P:780).
func (p *Player) onEnded() {
	if p.checkStart {
		// The end came before the engine's length: the resume position went
		// past a shorter media (the mock streams last 12 s, the course data
		// say 220). It is the start-over case of onDuration, not a lesson
		// watched to its end: begin again, paused when the load was.
		p.checkStart = false
		p.seek(0)
		if p.want {
			p.Play()
		}

		return
	}
	if p.loop && p.kind == KindVideo && (p.dur <= 0 || p.a < p.dur) {
		// The loop is checked before the end ($P:779): a B at the media's
		// end (the mock's 12 s gives B = 12) wraps to A, it is no ending.
		// The engine is at its end, so it is asked to play first (an engine
		// restarts an ended media at 0) and to go to A after. A late Ended
		// after the tick's own wrap lands here too and only repeats it.
		p.want = true
		_ = p.o.Engine.Play()
		p.seek(p.a)
		p.saveUpdate()

		return
	}
	if p.dur > 0 {
		p.pos = p.dur
	}
	p.want = false
	p.state = StateEnded
	p.saveUpdate()
	p.savePause()
	p.emit(ChangeState)
	p.emit(ChangeEnded)
}

// onError retries once with a fresh address when the failure may be an
// address that expired (03 §9): the expiry is never read from the clock, the
// engine's failure is what tells. The second failure, or any other kind, is
// the end of the load.
func (p *Player) onError(err error) {
	if p.state == StateError {
		return // the engine may tell one failure more than once
	}
	if p.retried || !retryable(err) {
		p.fail(err)

		return
	}
	p.syncTime()
	p.logf("player: lesson %s: asking for a new address after %v", p.lesson.ID, err)
	p.reload(p.want || p.state == StatePlaying)
	p.retried = true
}

// onSession handles the audio session: a call or another app taking the
// audio, the headphones pulled out, the system's media services rebuilt.
func (p *Player) onSession(ev media.SessionEvent) {
	if p.closed {
		return
	}
	switch ev.Kind {
	case media.InterruptBegin:
		if p.want {
			p.interrupted = true // before the pause, so its change tells it
			p.pause()
		}
	case media.InterruptEnd:
		resume := ev.ShouldResume && p.interrupted
		p.interrupted = false
		if resume {
			p.Play()
		}
	case media.Noisy:
		p.Pause()
	case media.MediaReset: // the engine's player is gone: load the media again, paused
		if p.loaded && p.url != "" {
			p.syncTime()
			p.start, p.checkStart, p.want = p.pos, false, false
			p.loadURL()
		}
	case media.SessionEventUnknown, media.Duck, media.Unduck:
	default:
	}
}
