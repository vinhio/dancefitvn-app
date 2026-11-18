package player

import "math"

// ====================================================================
// ========================== Main functions ==========================
// ====================================================================

// SetRate sets the speed (the sheet's 0.5 to 1.5, $P:609). The engine hears it
// at once and again at each Play.
func (p *Player) SetRate(r float64) {
	if p.closed || math.IsNaN(r) || r <= 0 {
		return
	}
	p.syncTime() // the position the engine reached at the old speed
	p.rate = r
	if p.loaded {
		_ = p.o.Engine.SetRate(r)
	}
	p.emit(ChangeRate)
}

// CycleRate goes 0.75, 1, 1.25 and back to 0.75; any other speed goes to
// 0.75 ($P:609, the button of 08).
func (p *Player) CycleRate() {
	next := CycleRates[0]
	for i, r := range CycleRates {
		if r == p.rate {
			next = CycleRates[(i+1)%len(CycleRates)]
		}
	}
	p.SetRate(next)
}

// SetMirror flips the picture or not; the view draws it.
func (p *Player) SetMirror(on bool) {
	if p.mirror != on {
		p.mirror = on
		p.emit(ChangeOptions)
	}
}

// SetBeat shows the 8-beat bar or not.
func (p *Player) SetBeat(on bool) {
	if p.beat != on {
		p.beat = on
		p.emit(ChangeOptions)
	}
}

// SetLoop turns the A–B loop on or off (K38, on 08 and on TV too). On, the loop
// begins at the position now: A is where the media is, B is LoopLength s on (or
// the end, when fewer remain), and nothing seeks, the position being A. The loop
// then plays from A to B and goes back to A (onTime, onEnded). Off leaves A and B
// as they are, for the band the screens draw to come back at the next time.
func (p *Player) SetLoop(on bool) {
	if p.loop == on {
		return
	}
	p.loop = on
	if on {
		p.repeatOne = false // one repeat at a time
	}
	if on && p.lesson.ID != "" {
		p.syncTime()
		p.a = max(0, p.pos)
		if p.dur > 0 {
			p.a = min(p.a, p.dur)
		}
		p.b = p.a + LoopLength
		if p.dur > 0 {
			p.b = min(p.b, p.dur)
		}
	}
	p.emit(ChangeOptions)
}

// SetRepeat sets the mode (K38): RepeatAB is SetLoop(true), which begins the loop
// at the position; RepeatOne starts the lesson again each time it ends, with no
// A–B loop; RepeatOff (and the zero) ends both.
func (p *Player) SetRepeat(m RepeatMode) {
	switch m {
	case RepeatAB:
		p.SetLoop(true)
	case RepeatOne:
		if p.repeatOne && !p.loop {
			return
		}
		p.loop, p.repeatOne = false, true
		p.emit(ChangeOptions)
	case RepeatUnknown, RepeatOff:
		if !p.loop && !p.repeatOne {
			return
		}
		p.loop, p.repeatOne = false, false
		p.emit(ChangeOptions)
	default:
	}
}

// CycleRepeat goes off, one, A–B and back to off: the audio screen's button.
func (p *Player) CycleRepeat() {
	switch p.Repeat() {
	case RepeatUnknown, RepeatAB:
		p.SetRepeat(RepeatOff)
	case RepeatOff:
		p.SetRepeat(RepeatOne)
	case RepeatOne:
		p.SetRepeat(RepeatAB)
	default:
	}
}

// SetA puts A at sec, kept in [0, dur]. If A then lies past B the two swap, so
// that A <= B always holds. The screens do not move A (K38): the loop begins
// where it was turned on; this is for a state set up whole (see SetAB).
func (p *Player) SetA(sec float64) { p.setPoint(sec, true) }

// SetB moves B to sec, as a finger dragging the B marker on the position bar or
// the buttons of the options sheet ask (K38): kept in [A + LoopLength, dur], or
// at dur when fewer than LoopLength s are left after A, so the loop is never
// shorter than min(LoopLength, dur - A). With the loop on, a B that now lies
// before the position takes the media back to A (the position is past the loop).
// It never seeks otherwise.
func (p *Player) SetB(sec float64) {
	if p.closed || math.IsNaN(sec) {
		return
	}
	hi := sec
	if p.dur > 0 {
		hi = p.dur
	}
	lo := min(p.a+LoopLength, hi)
	b := max(lo, min(sec, hi))
	if b == p.b {
		return
	}
	p.b = b
	p.emit(ChangeOptions)
	if p.loop && p.lesson.ID != "" {
		p.syncTime()
		if p.pos >= p.b {
			p.seek(max(0, p.a))
		}
	}
}

// SetAB sets A and B whole, each kept in [0, dur] with A <= B, with no least
// length: for a state that must show an exact loop (a screenshot, a test, a dev
// script), not for the user.
func (p *Player) SetAB(a, b float64) {
	if math.IsNaN(a) || math.IsNaN(b) {
		return
	}
	p.a, p.b = max(0, a), max(0, b)
	if p.dur > 0 {
		p.a, p.b = min(p.a, p.dur), min(p.b, p.dur)
	}
	if p.a > p.b {
		p.a, p.b = p.b, p.a
	}
	p.emit(ChangeOptions)
}

// ClearAB restores the loop of a lesson that has none: the first LoopLength s of
// the length now.
func (p *Player) ClearAB() {
	p.a, p.b = defaultAB(p.dur)
	p.emit(ChangeOptions)
}

// ====================================================================
// ========================= Helper Functions =========================
// ====================================================================

// setPoint moves A or B and keeps A <= B.
func (p *Player) setPoint(sec float64, isA bool) {
	if math.IsNaN(sec) {
		return
	}
	sec = max(0, sec)
	if p.dur > 0 {
		sec = min(sec, p.dur)
	}
	if isA {
		p.a = sec
	} else {
		p.b = sec
	}
	if p.a > p.b {
		p.a, p.b = p.b, p.a
	}
	p.emit(ChangeOptions)
}
