// Adapted from jplayer-mobile/internal/app/uiclock.go at 49de3c2.

package app

import (
	"slices"
	"time"

	"dancefitvn-app/internal/clock"

	"github.com/gdeskdev/desk/foundation/platform"
	"github.com/gdeskdev/desk/framework/ui"
)

// ====================================================================
// ============================= Creation =============================
// ====================================================================

// uiFirstSlots is how many timers a uiClock tracks before its ring first
// grows: the core keeps a handful at once (the store's two debounces, the
// player's sleep and skip timers).
const uiFirstSlots = 16

// uiClock is the core's clock (clock.Clock) on a view's: its timers are
// the view's (ui.Context.AfterFunc), so that they fire on the UI thread
// between frames, and a ui.Tester's Elapse drives the store and the
// player as it drives the widgets. Now is the wall time at Bind plus the
// view's time since; Resync puts it back on the wall clock, which the
// view's monotonic clock leaves behind while the phone sleeps.
//
// Before Bind, After queues its calls with their due wall time; Bind
// arranges each on the view for the time left. A uiClock is for the UI
// thread only, as the view is.
type uiClock struct {
	wall  func() time.Time
	view  *ui.View
	base  time.Time  // the wall time at the view's time 0
	slots []uiSlot   // by id modulo len (a power of two)
	early []uiQueued // the calls arranged before Bind
	next  clock.Timer
}

// uiSlot ties a clock.Timer to the view's timer for it.
type uiSlot struct {
	id clock.Timer // 0 for a free slot
	t  ui.Timer
}

// uiQueued is a call arranged before Bind.
type uiQueued struct {
	at time.Time
	fn func()
	id clock.Timer
}

// newUIClock returns a uiClock reading the wall time from wall (time.Now
// when nil), unbound: its timers wait for Bind.
func newUIClock(wall func() time.Time) *uiClock {
	if wall == nil {
		wall = time.Now
	}

	return &uiClock{wall: wall, slots: make([]uiSlot, uiFirstSlots)}
}

// ====================================================================
// ========================== Main functions ==========================
// ====================================================================

// Bind puts the clock on v's time and arranges on it the calls queued
// before, for the time each has left (none for those already due, which
// fire at the view's next turn, in the order they were arranged). Call it
// once, on the UI thread, after mounting the view.
func (k *uiClock) Bind(v *ui.View) {
	k.view = v
	k.base = k.wall().Round(0).Add(-v.Context().Now())
	now := k.Now()
	for _, q := range k.early {
		k.put(q.id, v.Context().AfterFunc(q.at.Sub(now), q.fn))
	}
	clear(k.early)
	k.early = k.early[:0]
}

// Resync puts Now back on the wall clock, never backwards: call it when
// the app returns to the foreground, as the view's clock does not count
// the time the phone slept.
func (k *uiClock) Resync() {
	if k.view == nil {
		return
	}
	if base := k.wall().Round(0).Add(-k.view.Context().Now()); base.After(k.base) {
		k.base = base
	}
}

// Expose tells the view that nothing it shows can be trusted, as a driver does when
// its window is made again: the next frame draws every pixel, even if no widget
// changed. A TV idle on a menu draws nothing for minutes, and what the system kept of
// its picture may be gone when the screen saver ends (the uikit driver tells such a
// return as a visibility change, which draws only what changed meanwhile); a screen
// that changed nothing then stayed black. Call it on the UI thread, when the app is
// active again.
func (k *uiClock) Expose() {
	if k.view == nil {
		return
	}
	k.view.HandleEvent(platform.Event{Kind: platform.EventExpose})
}

// Now returns the wall time: by the view's clock once bound (the frame's
// time during a frame), the wall clock's before.
func (k *uiClock) Now() time.Time {
	if k.view == nil {
		return k.wall()
	}

	return k.base.Add(k.view.Context().Now())
}

// After calls fn on the UI thread once d has passed, by the view's
// timers. It allocates nothing in steady state when fn is made once.
// A nil fn arranges nothing and returns 0.
func (k *uiClock) After(d time.Duration, fn func()) clock.Timer {
	if fn == nil {
		return 0
	}
	k.next++
	id := k.next
	if k.view == nil {
		k.early = append(k.early, uiQueued{at: k.wall().Add(max(d, 0)), fn: fn, id: id})

		return id
	}
	k.put(id, k.view.Context().AfterFunc(d, fn))

	return id
}

// Stop cancels the call of t and reports whether it was still pending.
func (k *uiClock) Stop(t clock.Timer) bool {
	if t == 0 {
		return false
	}
	if k.view == nil {
		i := slices.IndexFunc(k.early, func(q uiQueued) bool { return q.id == t })
		if i < 0 {
			return false
		}
		k.early = slices.Delete(k.early, i, i+1)

		return true
	}
	s := &k.slots[int(t)&(len(k.slots)-1)]
	if s.id != t {
		return false
	}
	ok := s.t.Stop()
	*s = uiSlot{}

	return ok
}

// ====================================================================
// ========================= Helper Functions =========================
// ====================================================================

// put records the view's timer of id in the ring. A slot still held by a
// pending timer makes the ring grow, so that no pending timer is lost.
func (k *uiClock) put(id clock.Timer, t ui.Timer) {
	for {
		s := &k.slots[int(id)&(len(k.slots)-1)]
		if s.id == 0 || !s.t.Pending() {
			*s = uiSlot{id: id, t: t}

			return
		}
		k.grow()
	}
}

// grow doubles the ring until its pending timers fit one to a slot, and
// drops the fired ones.
func (k *uiClock) grow() {
	n := len(k.slots) * 2
	for {
		next := make([]uiSlot, n)
		fits := true
		for _, s := range k.slots {
			if s.id == 0 || !s.t.Pending() {
				continue
			}
			d := &next[int(s.id)&(n-1)]
			if d.id != 0 {
				fits = false

				break
			}
			*d = s
		}
		if fits {
			k.slots = next

			return
		}
		n *= 2
	}
}

// pending counts the calls waiting: for tests.
func (k *uiClock) pending() int {
	n := len(k.early)
	for _, s := range k.slots {
		if s.id != 0 && s.t.Pending() {
			n++
		}
	}

	return n
}
