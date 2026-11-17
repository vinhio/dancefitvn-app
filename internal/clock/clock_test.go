// Adapted from jplayer-mobile/internal/clock/clock_test.go at 49de3c2.

package clock

import (
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var t0 = time.Date(2026, 11, 4, 12, 0, 0, 0, time.UTC)

func TestFakeAdvance(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		setup func(f *Fake, log *[]string)
		adv   time.Duration
		want  []string
		left  int
	}{
		{"none due", func(f *Fake, log *[]string) {
			f.After(time.Second, func() { *log = append(*log, "a") })
		}, 999 * time.Millisecond, nil, 1},
		{"due exactly", func(f *Fake, log *[]string) {
			f.After(time.Second, func() { *log = append(*log, "a") })
		}, time.Second, []string{"a"}, 0},
		{"time order then id order", func(f *Fake, log *[]string) {
			f.After(2*time.Second, func() { *log = append(*log, "late") })
			f.After(time.Second, func() { *log = append(*log, "x") })
			f.After(time.Second, func() { *log = append(*log, "y") })
		}, 3 * time.Second, []string{"x", "y", "late"}, 0},
		{"timer set by callback fires in same advance", func(f *Fake, log *[]string) {
			f.After(time.Second, func() {
				*log = append(*log, "a")
				f.After(time.Second, func() { *log = append(*log, "b") })
			})
		}, 2 * time.Second, []string{"a", "b"}, 0},
		{"callback timer beyond the end waits", func(f *Fake, log *[]string) {
			f.After(time.Second, func() {
				f.After(5*time.Second, func() { *log = append(*log, "b") })
			})
		}, 2 * time.Second, nil, 1},
		{"negative delay is now", func(f *Fake, log *[]string) {
			f.After(-time.Hour, func() { *log = append(*log, "a") })
		}, 0, []string{"a"}, 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			f := NewFake(t0)
			var log []string
			tt.setup(f, &log)
			f.Advance(tt.adv)
			assert.Equal(t, tt.want, log)
			assert.Equal(t, tt.left, f.Pending())
			assert.Equal(t, t0.Add(tt.adv), f.Now())
		})
	}
}

func TestFakeNowInsideCallback(t *testing.T) {
	t.Parallel()

	f := NewFake(t0)
	var seen time.Time
	f.After(3*time.Second, func() { seen = f.Now() })
	f.Advance(10 * time.Second)
	assert.Equal(t, t0.Add(3*time.Second), seen)
	assert.Equal(t, t0.Add(10*time.Second), f.Now())
}

func TestFakeStop(t *testing.T) {
	t.Parallel()

	f := NewFake(t0)
	fired := 0
	a := f.After(time.Second, func() { fired++ })
	b := f.After(time.Second, func() { fired += 10 })
	assert.True(t, f.Stop(a))
	assert.False(t, f.Stop(a), "second stop")
	assert.False(t, f.Stop(0))
	f.Advance(time.Second)
	assert.Equal(t, 10, fired)
	assert.False(t, f.Stop(b), "already fired")
}

func TestFakeAfterStopDoesNotAllocate(t *testing.T) {
	f := NewFake(t0)
	fn := func() {}
	f.Stop(f.After(time.Second, fn)) // grow the slice once
	allocs := testing.AllocsPerRun(100, func() {
		f.Stop(f.After(time.Second, fn))
	})
	assert.Zero(t, allocs)
}

func TestSystem(t *testing.T) {
	t.Parallel()

	t.Run("fires through dispatch", func(t *testing.T) {
		t.Parallel()
		got := make(chan func(), 1)
		c := System(func(fn func()) { got <- fn })
		done := make(chan struct{})
		id := c.After(time.Millisecond, func() { close(done) })
		require.NotZero(t, id)
		fn := <-got
		fn()
		<-done
		assert.False(t, c.Stop(id))
		assert.WithinDuration(t, time.Now(), c.Now(), time.Minute)
	})
	t.Run("stop prevents", func(t *testing.T) {
		t.Parallel()
		var mu sync.Mutex
		fired := false
		c := System(func(fn func()) { mu.Lock(); fired = true; mu.Unlock() })
		id := c.After(time.Hour, func() {})
		assert.True(t, c.Stop(id))
		assert.False(t, c.Stop(id))
		mu.Lock()
		assert.False(t, fired)
		mu.Unlock()
	})
	t.Run("nil dispatch runs on the timer goroutine", func(t *testing.T) {
		t.Parallel()
		done := make(chan struct{})
		c := System(nil)
		c.After(time.Millisecond, func() { close(done) })
		<-done
	})
}
