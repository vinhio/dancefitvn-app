// Adapted from jplayer-mobile/e2e/engine/avf_ios_test.go at 49de3c2 (pullUntil, pullForce, measurePulls, percentiles, cpuTime).

//go:build ios && !tvos

package engine

import (
	"runtime"
	"syscall"
	"testing"
	"time"

	"github.com/gdeskdev/desk/foundation/scene"
	"github.com/stretchr/testify/require"
)

// ====================================================================
// ============================ Contracts =============================
// ====================================================================

// pullReport is what measurePulls saw.
type pullReport struct {
	pulls, frames              int
	frameMallocs, emptyMallocs uint64
	frameDirty, emptyDirty     int // pulls that saw any malloc
	frameCost, emptyCost       []time.Duration
	span                       time.Duration
}

// ====================================================================
// ========================== Main functions ==========================
// ====================================================================

// pullUntil pulls the engine's surface on the UI thread about 60 times a
// second while it is Active, as a view's paint does, until done accepts
// the newest frame (done runs on the UI thread), or within passes.
func pullUntil(t *testing.T, h *harness, within time.Duration, done func(img *scene.Image, seq uint64) bool) bool {
	t.Helper()

	tick := time.NewTicker(pullEvery)
	defer tick.Stop()
	deadline := time.NewTimer(within)
	defer deadline.Stop()
	for {
		select {
		case <-deadline.C:
			return false
		case <-tick.C:
			var ok bool
			h.onUI(t, func() {
				v := h.eng.Video()
				if v.Active() {
					v.Pull()
				}
				img, seq := v.Frame(0)
				ok = done(img, seq)
			})
			if ok {
				return true
			}
		}
	}
}

// pullForce pulls about 60 times a second for d, Active or not, and shows
// each latched frame to seen.
func pullForce(t *testing.T, h *harness, d time.Duration, seen func(img *scene.Image, seq uint64)) {
	t.Helper()

	tick := time.NewTicker(pullEvery)
	defer tick.Stop()
	deadline := time.NewTimer(d)
	defer deadline.Stop()
	for {
		select {
		case <-deadline.C:
			return
		case <-tick.C:
			h.onUI(t, func() {
				v := h.eng.Video()
				v.Pull()
				seen(v.Frame(0))
			})
		}
	}
}

// measurePulls pulls about 60 times a second for d, times each pull and
// counts its allocations (runtime.MemStats around it), apart for the pulls
// that brought a frame. The whole loop runs in one call on the UI thread,
// as a view's paints would, its buffers made before: the test goroutine,
// parked meanwhile, allocates nothing inside the windows. It first pulls
// until the surface's three images exist (Back makes each once).
func measurePulls(t *testing.T, h *harness, d time.Duration) pullReport {
	t.Helper()

	require.True(t, pullUntil(t, h, frameWithin, func(_ *scene.Image, seq uint64) bool { return seq >= 4 }),
		"four frames, so that the surface has made its three images")
	n := int(d/pullEvery) + 16
	r := pullReport{frameCost: make([]time.Duration, 0, n), emptyCost: make([]time.Duration, 0, n)}
	h.onUI(t, func() {
		var before, after runtime.MemStats
		v := h.eng.Video()
		_, last := v.Frame(0)
		tick := time.NewTicker(pullEvery)
		defer tick.Stop()
		<-tick.C // the runtime's timer heap grows once
		start := time.Now()
		for time.Since(start) < d {
			<-tick.C
			runtime.ReadMemStats(&before)
			s := time.Now()
			v.Pull()
			took := time.Since(s)
			runtime.ReadMemStats(&after)
			_, seq := v.Frame(0)
			r.pulls++
			mallocs := after.Mallocs - before.Mallocs
			if seq != last {
				last = seq
				r.frames++
				r.frameMallocs += mallocs
				if mallocs != 0 {
					r.frameDirty++
				}
				r.frameCost = append(r.frameCost, took)

				continue
			}
			r.emptyMallocs += mallocs
			if mallocs != 0 {
				r.emptyDirty++
			}
			r.emptyCost = append(r.emptyCost, took)
		}
		r.span = time.Since(start)
	})

	return r
}

// cpuTime is the user and system time the process used so far.
func cpuTime() time.Duration {
	var ru syscall.Rusage
	if err := syscall.Getrusage(syscall.RUSAGE_SELF, &ru); err != nil {
		return 0
	}

	return time.Duration(ru.Utime.Nano() + ru.Stime.Nano())
}
