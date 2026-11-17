// Adapted from jplayer-mobile/internal/media/enginekit/ticker.go at 49de3c2.

package enginekit

import (
	"dancefitvn-app/internal/clock"
	"time"
)

// ====================================================================
// ============================ Contracts =============================
// ====================================================================

// TickEvery is the period of an engine's time events (04 §2.1).
const TickEvery = 250 * time.Millisecond

// Ticker calls a func every period on a clock, while it runs and the app is
// in the foreground. Use it on the UI thread only.
type Ticker struct {
	clk     clock.Clock
	every   time.Duration
	fn      func()
	tickFn  func()
	timer   clock.Timer
	running bool
	back    bool
}

// ====================================================================
// ============================= Creation =============================
// ====================================================================

// NewTicker returns a stopped Ticker that calls fn every period on c. Pass an
// fn made once: a tick then allocates nothing.
func NewTicker(c clock.Clock, every time.Duration, fn func()) *Ticker {
	t := &Ticker{clk: c, every: every, fn: fn}
	t.tickFn = t.onTick

	return t
}

// ====================================================================
// ========================== Main functions ==========================
// ====================================================================

// Start makes the ticker run: fn is called one period from now, and every
// period after. It does nothing for a ticker that runs.
func (t *Ticker) Start() {
	if t.running {
		return
	}
	t.running = true
	t.schedule()
}

// Stop stops the ticker. It does nothing for one that is stopped.
func (t *Ticker) Stop() {
	t.running = false
	t.cancel()
}

// SetBackground pauses the ticks while the app is in the background, and
// resumes them, from a full period, when it is back, if the ticker runs. It
// does not call fn: the engine tells the time itself when it is back.
func (t *Ticker) SetBackground(on bool) {
	if t.back == on {
		return
	}
	t.back = on
	if on {
		t.cancel()

		return
	}
	if t.running {
		t.schedule()
	}
}

// Running tells that the ticker was started and not stopped, in the
// background as well.
func (t *Ticker) Running() bool { return t.running }

// Tick calls fn now, without moving the next tick.
func (t *Ticker) Tick() { t.fn() }

// ====================================================================
// ========================= Helper Functions =========================
// ====================================================================

// schedule arranges the next tick unless one is waiting or the app is in the
// background.
func (t *Ticker) schedule() {
	if t.timer != 0 || t.back {
		return
	}
	t.timer = t.clk.After(t.every, t.tickFn)
}

// cancel drops the tick that waits.
func (t *Ticker) cancel() {
	if t.timer != 0 {
		t.clk.Stop(t.timer)
		t.timer = 0
	}
}

// onTick arranges the next tick, then calls fn, which may stop the ticker.
func (t *Ticker) onTick() {
	t.timer = 0
	if !t.running || t.back {
		return
	}
	t.schedule()
	t.fn()
}
