package player_test

import (
	"dancefitvn-app/internal/media"
	"dancefitvn-app/internal/media/fake"
	"dancefitvn-app/internal/player"
	"math"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSetRate(t *testing.T) {
	t.Parallel()

	for _, rate := range player.SheetRates {
		t.Run("", func(t *testing.T) {
			t.Parallel()

			r := newRig(t, fullLength())
			r.playVideo(0)
			r.p.SetRate(rate)

			assert.InDelta(t, rate, r.p.Rate(), 0)
			assert.InDelta(t, rate, r.eng.Rate(), 0)
			assert.True(t, r.has(player.ChangeRate))
		})
	}
	assert.Equal(t, [5]float64{0.5, 0.75, 1, 1.25, 1.5}, player.SheetRates)

	// nonsense is ignored
	r := newRig(t, fullLength())
	for _, bad := range []float64{0, -1, math.NaN()} {
		r.p.SetRate(bad)
	}
	assert.InDelta(t, 0.75, r.p.Rate(), 0)
}

func TestSetRateBeforeAnyLoadIsKept(t *testing.T) {
	t.Parallel()

	r := newRig(t, fullLength())
	r.p.SetRate(1.5)
	r.playVideo(0)

	assert.InDelta(t, 1.5, r.eng.Rate(), 0)
}

func TestCycleRate(t *testing.T) {
	t.Parallel()

	tests := []struct {
		from, want float64
	}{
		{0.75, 1},
		{1, 1.25},
		{1.25, 0.75},
		{0.5, 0.75},
		{1.5, 0.75},
		{2, 0.75},
	}
	for _, tc := range tests {
		t.Run("", func(t *testing.T) {
			t.Parallel()

			r := newRig(t, fullLength())
			r.playVideo(0)
			r.p.SetRate(tc.from)
			r.p.CycleRate()

			assert.InDelta(t, tc.want, r.p.Rate(), 0)
			assert.InDelta(t, tc.want, r.eng.Rate(), 0)
		})
	}

	r := newRig(t, fullLength())
	var seen []float64
	for range 4 {
		seen = append(seen, r.p.Rate())
		r.p.CycleRate()
	}
	assert.Equal(t, []float64{0.75, 1, 1.25, 0.75}, seen)
}

func TestToggles(t *testing.T) {
	t.Parallel()

	r := newRig(t, fullLength())
	r.p.SetMirror(false)
	r.p.SetMirror(false) // no change, no event
	r.p.SetBeat(false)
	assert.False(t, r.p.Mirror())
	assert.False(t, r.p.Beat())
	assert.Equal(t, []player.ChangeKind{player.ChangeOptions, player.ChangeOptions}, r.kinds())
	r.p.SetMirror(true)
	r.p.SetBeat(true)
	assert.True(t, r.p.Mirror())
	assert.True(t, r.p.Beat())
}

func TestABSetAndClear(t *testing.T) {
	t.Parallel()

	type step struct {
		fn string // "A", "B", "clear"
		v  float64
	}
	tests := []struct {
		name  string
		steps []step
		a, b  float64
	}{
		{"default", nil, 73, 101},
		{"A only", []step{{"A", 50}}, 50, 101},
		{"A then B", []step{{"A", 50}, {"B", 80}}, 50, 80},
		{"B before A swaps", []step{{"B", 60}}, 60, 73},
		{"A after B swaps", []step{{"A", 150}}, 101, 150},
		{"A below 0 clamps", []step{{"A", -5}}, 0, 101},
		{"B past the end clamps", []step{{"B", 999}}, 73, 220},
		{"A past the end clamps", []step{{"A", 999}}, 101, 220},
		{"clear restores", []step{{"A", 10}, {"B", 20}, {"clear", 0}}, 73, 101},
		{"equal points", []step{{"A", 30}, {"B", 30}}, 30, 30},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			r := newRig(t, fullLength())
			r.playVideo(0)
			for _, s := range tc.steps {
				switch s.fn {
				case "A":
					r.p.SetA(s.v)
				case "B":
					r.p.SetB(s.v)
				default:
					r.p.ClearAB()
				}
			}

			assert.InDelta(t, tc.a, r.p.A(), 0)
			assert.InDelta(t, tc.b, r.p.B(), 0)
			assert.LessOrEqual(t, r.p.A(), r.p.B())
		})
	}
}

func TestClearABUsesTheEngineLength(t *testing.T) {
	t.Parallel()

	r := newRig(t, rigOpts{eng: fake.Options{VideoDuration: 30}})
	r.playVideo(0)
	r.p.SetA(2)
	r.p.SetB(3)
	r.p.ClearAB()

	assert.InDelta(t, 10, r.p.A(), 0)
	assert.InDelta(t, 30, r.p.B(), 0, "B does not pass the end")
}

func TestLoop(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		audio     bool
		loop      bool
		ticks     int
		wantSeeks int
		wantPos   float64
	}{
		{"on: jumps to A at once", false, true, 0, 1, 73},
		{"on: just before B", false, true, 111, 1, 73 + 111*0.25},
		{"on: at B back to A", false, true, 112, 2, 73},
		{"on: second round", false, true, 112 + 56, 2, 73 + 56*0.25},
		{"off: no jump", false, false, 112, 0, 28},
		{"on then off: runs past B", false, true, -1, 1, 73 + 112*0.25},
		{"audio never loops past B", true, true, 112, 1, 101},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			r := newRig(t, fullLength())
			if tc.audio {
				r.playAudio(0)
			} else {
				r.playVideo(0)
			}
			r.p.SetRate(1)
			if tc.loop {
				r.p.SetLoop(true)
			}
			ticks := tc.ticks
			if ticks < 0 { // the loop is turned off before B
				r.p.SetLoop(false)
				ticks = 112
			}
			r.tick(ticks)

			assert.Equal(t, tc.wantSeeks, r.count(fake.OpSeek))
			assert.InDelta(t, tc.wantPos, r.p.Pos(), 1e-9)
			assert.Equal(t, tc.loop && tc.ticks >= 0, r.p.Loop())
		})
	}
}

func TestLoopWhilePausedDoesNotSeek(t *testing.T) {
	t.Parallel()

	r := newRig(t, fullLength())
	r.playVideo(0)
	r.p.SetRate(1)
	r.p.SetLoop(true)
	r.p.Pause()
	r.eng.Emit(media.Event{Kind: media.EventTime, Token: r.eng.Token(), Time: 150})

	assert.Equal(t, 1, r.count(fake.OpSeek))
}

func TestBeatIndex(t *testing.T) {
	t.Parallel()

	tests := []struct {
		pos  float64
		want int
	}{
		{0, 0},
		{0.49, 0},
		{0.5, 1},
		{3.99, 7},
		{4.0, 0},
		{4.5, 1},
		{7.5, 7},
		{8, 0},
		{219.75, 7}, // 439.5 -> 439 mod 8
	}
	for _, tc := range tests {
		t.Run("", func(t *testing.T) {
			t.Parallel()

			r := newRig(t, fullLength())
			r.playVideo(0)
			r.p.Pause()
			r.p.Seek(tc.pos)

			assert.Equal(t, tc.want, r.p.BeatIndex())
		})
	}
}

func TestBeatIndexFollowsTheMediaTimeOverRateChanges(t *testing.T) {
	t.Parallel()

	r := newRig(t, fullLength())
	r.playVideo(0)
	steps := []struct {
		rate  float64
		ticks int
		pos   float64
		beat  int
	}{
		{0.75, 8, 1.5, 3},  // 8 ticks * .25 s * .75
		{1.5, 4, 3.0, 6},   // + 1.5
		{0.5, 4, 3.5, 7},   // + .5
		{1, 2, 4.0, 0},     // + .5, the bar wraps
		{1.25, 16, 9.0, 2}, // + 5
	}
	for _, s := range steps {
		r.p.SetRate(s.rate)
		r.tick(s.ticks)
		require.InDelta(t, s.pos, r.p.Pos(), 1e-9)
		assert.Equal(t, s.beat, r.p.BeatIndex(), "at %.2f s", s.pos)
	}
}
