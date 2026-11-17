package app

import (
	"context"
	"sync"
	"time"

	"dancefitvn-app/internal/clock"
)

// ====================================================================
// ============================ Contracts =============================
// ====================================================================

// probeSteps are the waits between two probes of the network while the app
// is offline (docs/plan/03 §2, before gDesk's G4): 5, 10, 20, 40 s, then
// every 60 s.
var probeSteps = [...]time.Duration{5 * time.Second, 10 * time.Second, 20 * time.Second, 40 * time.Second, 60 * time.Second}

// probeTimeout bounds one probe.
const probeTimeout = 15 * time.Second

// onlineProbe asks the server, at growing intervals, whether the network
// came back, after a request found it gone: the API client then reports
// it (api.Options.OnNetwork) and the probe is stopped. It is safe for
// concurrent use.
type onlineProbe struct {
	clk   clock.Clock
	check func(ctx context.Context) error
	// spawn runs a probe off the clock's thread: go, or at once in tests.
	spawn func(fn func())

	mu      sync.Mutex
	running bool
	step    int
	timer   clock.Timer
	ctx     context.Context //nolint:containedctx // canceled by close
	cancel  context.CancelFunc
	fire    func() // p.tick, made once
}

// ====================================================================
// ============================= Creation =============================
// ====================================================================

// newOnlineProbe returns a stopped probe calling check (GET /app/config) on
// clk's timers.
func newOnlineProbe(clk clock.Clock, check func(ctx context.Context) error) *onlineProbe {
	p := &onlineProbe{clk: clk, check: check, spawn: func(fn func()) { go fn() }}
	p.ctx, p.cancel = context.WithCancel(context.Background())
	p.fire = p.tick

	return p
}

// ====================================================================
// ========================== Main functions ==========================
// ====================================================================

// start begins the probes, the first one probeSteps[0] from now; a probe
// running already goes on.
func (p *onlineProbe) start() {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.running || p.ctx.Err() != nil {
		return
	}
	p.running, p.step = true, 0
	p.timer = p.clk.After(probeSteps[0], p.fire)
}

// stop ends the probes: the network is back.
func (p *onlineProbe) stop() {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.running = false
	p.clk.Stop(p.timer)
	p.timer = 0
}

// now probes at once, the back-off started again (this probe is its first
// step): the device's network came back (03 section 2), so the answer need
// not wait for the next step. A probe that fails arms the next one as tick
// does.
func (p *onlineProbe) now() {
	p.mu.Lock()
	if p.ctx.Err() != nil {
		p.mu.Unlock()

		return
	}
	p.clk.Stop(p.timer)
	p.timer = 0
	p.running, p.step = true, 0
	p.mu.Unlock()
	p.tick()
}

// close stops the probes for good, a probe under way canceled.
func (p *onlineProbe) close() {
	p.stop()
	p.cancel()
}

// ====================================================================
// ========================= Helper Functions =========================
// ====================================================================

// tick is a timer's end: one probe, off the clock's thread; while it
// fails the next one is armed after the next step.
func (p *onlineProbe) tick() {
	p.mu.Lock()
	if !p.running {
		p.mu.Unlock()

		return
	}
	p.timer = 0
	p.mu.Unlock()
	p.spawn(func() {
		ctx, cancel := context.WithTimeout(p.ctx, probeTimeout)
		err := p.check(ctx)
		cancel()
		p.mu.Lock()
		defer p.mu.Unlock()
		if err == nil || !p.running || p.ctx.Err() != nil {
			p.running = false

			return
		}
		if p.step < len(probeSteps)-1 {
			p.step++
		}
		p.timer = p.clk.After(probeSteps[p.step], p.fire)
	})
}
