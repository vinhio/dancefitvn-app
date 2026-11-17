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

// SetLoop turns the A–B loop on or off; on, it jumps to A ($P:608, on 08 too).
func (p *Player) SetLoop(on bool) {
	if p.loop == on {
		return
	}
	p.loop = on
	p.emit(ChangeOptions)
	if on && p.lesson.ID != "" {
		p.seek(max(0, p.a))
	}
}

// SetA puts A at sec, kept in [0, dur]. If A then lies past B the two swap, so
// that A <= B always holds.
func (p *Player) SetA(sec float64) { p.setPoint(sec, true) }

// SetB puts B at sec, kept in [0, dur]; before A the two swap.
func (p *Player) SetB(sec float64) { p.setPoint(sec, false) }

// ClearAB restores A = round(dur·0.33) and B = A + 28 for the length now.
func (p *Player) ClearAB() {
	p.a, p.b = defaultAB(p.dur)
	if p.dur > 0 {
		p.b = min(p.b, p.dur)
	}
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
