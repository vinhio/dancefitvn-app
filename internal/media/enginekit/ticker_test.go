// Adapted from jplayer-mobile/internal/media/enginekit/ticker_test.go at 49de3c2.

package enginekit

import (
	"dancefitvn-app/internal/clock"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

var t0 = time.Date(2026, 11, 5, 9, 0, 0, 0, time.UTC)

func TestTicker(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		run  func(tk *Ticker, clk *clock.Fake)
		want int
	}{
		{"idle until started", func(_ *Ticker, clk *clock.Fake) { clk.Advance(time.Second) }, 0},
		{"a tick per period", func(tk *Ticker, clk *clock.Fake) { tk.Start(); clk.Advance(time.Second) }, 4},
		{"not before the period", func(tk *Ticker, clk *clock.Fake) { tk.Start(); clk.Advance(249 * time.Millisecond) }, 0},
		{"start twice is one", func(tk *Ticker, clk *clock.Fake) { tk.Start(); tk.Start(); clk.Advance(time.Second) }, 4},
		{"stop", func(tk *Ticker, clk *clock.Fake) {
			tk.Start()
			clk.Advance(500 * time.Millisecond)
			tk.Stop()
			clk.Advance(time.Second)
		}, 2},
		{"stop twice", func(tk *Ticker, clk *clock.Fake) { tk.Start(); tk.Stop(); tk.Stop(); clk.Advance(time.Second) }, 0},
		{"restart", func(tk *Ticker, clk *clock.Fake) {
			tk.Start()
			clk.Advance(250 * time.Millisecond)
			tk.Stop()
			tk.Start()
			clk.Advance(500 * time.Millisecond)
		}, 3},
		{"fn is not called by Tick's schedule", func(tk *Ticker, _ *clock.Fake) { tk.Tick() }, 1},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			clk := clock.NewFake(t0)
			n := 0
			tk := NewTicker(clk, TickEvery, func() { n++ })
			tt.run(tk, clk)
			assert.Equal(t, tt.want, n)
		})
	}
}

func TestTickerBackground(t *testing.T) {
	t.Parallel()

	clk := clock.NewFake(t0)
	n := 0
	tk := NewTicker(clk, TickEvery, func() { n++ })
	tk.Start()
	clk.Advance(250 * time.Millisecond)
	assert.Equal(t, 1, n)

	tk.SetBackground(true)
	assert.True(t, tk.Running(), "it still runs, it is only quiet")
	assert.Zero(t, clk.Pending(), "no timer in the background")
	clk.Advance(5 * time.Second)
	assert.Equal(t, 1, n)

	tk.SetBackground(true) // again: nothing
	tk.SetBackground(false)
	assert.Equal(t, 1, n, "fn is not called on return, the engine tells the time")
	clk.Advance(250 * time.Millisecond)
	assert.Equal(t, 2, n)

	// Stopped in the background stays stopped on return.
	tk.SetBackground(true)
	tk.Stop()
	tk.SetBackground(false)
	assert.Zero(t, clk.Pending())
	assert.False(t, tk.Running())

	// Started in the background waits for the foreground.
	tk.SetBackground(true)
	tk.Start()
	clk.Advance(time.Second)
	assert.Equal(t, 2, n)
	tk.SetBackground(false)
	clk.Advance(250 * time.Millisecond)
	assert.Equal(t, 3, n)
}

func TestTickerFnMayStop(t *testing.T) {
	t.Parallel()

	clk := clock.NewFake(t0)
	n := 0
	var tk *Ticker
	tk = NewTicker(clk, TickEvery, func() {
		n++
		if n == 2 {
			tk.Stop()
		}
	})
	tk.Start()
	clk.Advance(5 * time.Second)

	assert.Equal(t, 2, n)
	assert.Zero(t, clk.Pending())
}

func TestTickerDoesNotAllocate(t *testing.T) { //nolint:paralleltest // AllocsPerRun needs the process to itself
	clk := clock.NewFake(t0)
	tk := NewTicker(clk, TickEvery, func() {})
	tk.Start()
	clk.Advance(time.Second) // warm the clock's slice
	allocs := testing.AllocsPerRun(100, func() { clk.Advance(TickEvery) })
	assert.Zero(t, allocs)
}

func BenchmarkTicker(b *testing.B) {
	clk := clock.NewFake(t0)
	tk := NewTicker(clk, TickEvery, func() {})
	tk.Start()
	clk.Advance(time.Second)
	b.ReportAllocs()
	for b.Loop() {
		clk.Advance(TickEvery)
	}
}
