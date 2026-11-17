// Adapted from jplayer-mobile/internal/clock/clock.go at 49de3c2.

// Package clock is the UI thread's time for the core: the wall clock and callbacks that run on the UI thread, so that the core never sleeps and tests drive time by hand.
package clock

import (
	"slices"
	"sync"
	"time"
)

// ====================================================================
// ============================ Contracts =============================
// ====================================================================

// Timer identifies a call that After arranged; 0 is none.
type Timer uint64

// Clock is the UI thread's time: wall time, and callbacks on the UI thread.
type Clock interface {
	// Now returns the current wall time.
	Now() time.Time
	// After calls fn on the UI thread once d has passed. Passing a func
	// value made once (a method value kept in a field) must not allocate
	// in steady state.
	After(d time.Duration, fn func()) Timer
	// Stop cancels the call t stands for and reports whether it was still
	// pending. A Timer of 0, or one already fired, stops nothing.
	Stop(t Timer) bool
}

// ====================================================================
// ============================= Creation =============================
// ====================================================================

// Fake is a Clock driven by hand, for tests. It is safe for concurrent
// use (workers read Now while the test advances it); callbacks run on the
// goroutine that calls Advance, without the lock held.
type Fake struct {
	mu     sync.Mutex
	now    time.Time
	timers []fakeTimer
	next   Timer
}

// fakeTimer is one pending call of a Fake.
type fakeTimer struct {
	at time.Time
	fn func()
	id Timer
}

// NewFake returns a Fake whose time is start.
func NewFake(start time.Time) *Fake { return &Fake{now: start} }

// system is the Clock on the wall time that System returns.
type system struct {
	dispatch func(func())
	timers   map[Timer]*time.Timer
	next     Timer
	mu       sync.Mutex
}

// System returns a Clock on the real time: After starts a [time.Timer]
// whose expiry hands fn to dispatch, which runs it on the UI thread. It is
// the fallback for programs that have no UI clock; a nil dispatch runs fn
// on the timer's goroutine.
func System(dispatch func(func())) Clock {
	if dispatch == nil {
		dispatch = func(fn func()) { fn() }
	}

	return &system{dispatch: dispatch, timers: make(map[Timer]*time.Timer)}
}

// ====================================================================
// ========================== Main functions ==========================
// ====================================================================

// Now returns the Fake's time.
func (f *Fake) Now() time.Time {
	f.mu.Lock()
	defer f.mu.Unlock()

	return f.now
}

// After arranges fn for d after the Fake's time. A negative d counts as 0.
func (f *Fake) After(d time.Duration, fn func()) Timer {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.next++
	f.timers = append(f.timers, fakeTimer{at: f.now.Add(max(d, 0)), fn: fn, id: f.next})

	return f.next
}

// Stop cancels the call of t and reports whether it was pending.
func (f *Fake) Stop(t Timer) bool {
	f.mu.Lock()
	defer f.mu.Unlock()
	i := slices.IndexFunc(f.timers, func(x fakeTimer) bool { return x.id == t })
	if i < 0 {
		return false
	}
	f.timers = slices.Delete(f.timers, i, i+1)

	return true
}

// Advance moves the Fake's time forward by d and runs the calls that fall
// due, earliest first and in the order they were arranged for equal
// times. A call arranged by a callback runs in the same Advance when it
// falls due before the end. While a callback runs, Now is its due time.
func (f *Fake) Advance(d time.Duration) {
	f.mu.Lock()
	end := f.now.Add(max(d, 0))
	for {
		best := -1
		for i := range f.timers {
			t := &f.timers[i]
			if t.at.After(end) {
				continue
			}
			if best < 0 || t.at.Before(f.timers[best].at) || (t.at.Equal(f.timers[best].at) && t.id < f.timers[best].id) {
				best = i
			}
		}
		if best < 0 {
			break
		}
		t := f.timers[best]
		f.timers = slices.Delete(f.timers, best, best+1)
		if t.at.After(f.now) {
			f.now = t.at
		}
		f.mu.Unlock()
		t.fn()
		f.mu.Lock()
	}
	f.now = end
	f.mu.Unlock()
}

// Pending returns how many calls wait.
func (f *Fake) Pending() int {
	f.mu.Lock()
	defer f.mu.Unlock()

	return len(f.timers)
}

// Now returns the wall time.
func (s *system) Now() time.Time { return time.Now() }

// After starts a timer that dispatches fn after d.
func (s *system) After(d time.Duration, fn func()) Timer {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.next++
	id := s.next
	s.timers[id] = time.AfterFunc(d, func() {
		s.mu.Lock()
		_, live := s.timers[id]
		delete(s.timers, id)
		s.mu.Unlock()
		if live {
			s.dispatch(fn)
		}
	})

	return id
}

// Stop cancels the timer of t.
func (s *system) Stop(t Timer) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	tm, ok := s.timers[t]
	if !ok {
		return false
	}
	delete(s.timers, t)

	return tm.Stop()
}
