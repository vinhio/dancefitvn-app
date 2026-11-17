package app

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"dancefitvn-app/internal/clock"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ====================================================================
// ============================ Contracts =============================
// ====================================================================

// probeStart is the fake clock's start.
var probeStart = time.Date(2026, time.November, 14, 9, 41, 0, 0, time.UTC)

// ====================================================================
// ========================== Main functions ==========================
// ====================================================================

// TestOnlineProbeBacksOff checks the waits between probes while they fail
// (5, 10, 20, 40, 60, 60 s), the end at the first success, and that start
// while running changes nothing.
func TestOnlineProbeBacksOff(t *testing.T) {
	t.Parallel()

	clk := clock.NewFake(probeStart)
	var mu sync.Mutex
	calls, okAt := 0, 7
	p := newOnlineProbe(clk, func(context.Context) error {
		mu.Lock()
		defer mu.Unlock()
		calls++
		if calls == okAt {
			return nil
		}

		return errors.New("offline")
	})
	p.spawn = func(fn func()) { fn() }
	p.start()
	p.start()
	require.Equal(t, 1, clk.Pending())
	for i, d := range []int{5, 10, 20, 40, 60, 60} {
		clk.Advance(secs(d) - secs(1))
		mu.Lock()
		before := calls
		mu.Unlock()
		assert.Equal(t, i, before, "no probe before %d s", d)
		clk.Advance(secs(1))
		mu.Lock()
		assert.Equal(t, i+1, calls, "probe %d after %d s", i+1, d)
		mu.Unlock()
	}
	clk.Advance(secs(60))
	mu.Lock()
	assert.Equal(t, okAt, calls, "the success ended the probes")
	mu.Unlock()
	assert.Zero(t, clk.Pending())
}

// TestOnlineProbeStopAndClose checks that stop disarms the probe, start
// arms it again from the first step, and close ends it for good.
func TestOnlineProbeStopAndClose(t *testing.T) {
	t.Parallel()

	clk := clock.NewFake(probeStart)
	calls := 0
	p := newOnlineProbe(clk, func(context.Context) error { calls++; return errors.New("offline") })
	p.spawn = func(fn func()) { fn() }
	p.start()
	p.stop()
	assert.Zero(t, clk.Pending())
	clk.Advance(secs(60))
	assert.Zero(t, calls)

	p.start()
	clk.Advance(probeSteps[0])
	assert.Equal(t, 1, calls)
	p.close()
	assert.Zero(t, clk.Pending())
	p.start()
	assert.Zero(t, clk.Pending(), "a closed probe does not start")
}

// TestOnlineProbeStoppedWhileProbing checks that a probe whose stop came
// while it ran arms nothing after it.
func TestOnlineProbeStoppedWhileProbing(t *testing.T) {
	t.Parallel()

	clk := clock.NewFake(probeStart)
	var p *onlineProbe
	p = newOnlineProbe(clk, func(context.Context) error {
		p.stop() // the network came back as the probe ran

		return errors.New("late")
	})
	p.spawn = func(fn func()) { fn() }
	p.start()
	clk.Advance(probeSteps[0])
	assert.Zero(t, clk.Pending())
}

// TestOnlineProbeNow checks the probe at once of a network that came back:
// it probes without waiting (the first step's probe), and when it fails the
// back-off goes on from its second step; a closed probe does nothing.
func TestOnlineProbeNow(t *testing.T) {
	t.Parallel()

	clk := clock.NewFake(probeStart)
	calls := 0
	p := newOnlineProbe(clk, func(context.Context) error { calls++; return errors.New("offline") })
	p.spawn = func(fn func()) { fn() }
	p.start()
	clk.Advance(probeSteps[0])
	clk.Advance(probeSteps[1])
	require.Equal(t, 2, calls, "the back-off is at its third step")

	p.now()
	assert.Equal(t, 3, calls, "at once")
	assert.Equal(t, 1, clk.Pending())
	clk.Advance(probeSteps[1])
	assert.Equal(t, 4, calls, "the next one after the second step: the back-off started again")

	p.close()
	p.now()
	assert.Equal(t, 4, calls, "closed: no probe")
	assert.Zero(t, clk.Pending())
}

// ====================================================================
// ========================= Helper Functions =========================
// ====================================================================

// secs is n seconds.
func secs(n int) time.Duration { return time.Duration(n) * time.Second }
