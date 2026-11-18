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
	assert.InDelta(t, 1, r.p.Rate(), 0)
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
	assert.Equal(t, []float64{1, 1.25, 0.75, 1}, seen)
}

func TestToggles(t *testing.T) {
	t.Parallel()

	r := newRig(t, fullLength())
	r.p.SetMirror(false) // already off: no change, no event
	r.p.SetBeat(false)
	assert.Empty(t, r.kinds())
	r.p.SetMirror(true)
	r.p.SetMirror(true) // no change, no event
	r.p.SetBeat(true)
	assert.True(t, r.p.Mirror())
	assert.True(t, r.p.Beat())
	assert.Equal(t, []player.ChangeKind{player.ChangeOptions, player.ChangeOptions}, r.kinds())
	r.p.SetMirror(false)
	r.p.SetBeat(false)
	assert.False(t, r.p.Mirror())
	assert.False(t, r.p.Beat())
}

func TestABSetAndClear(t *testing.T) {
	t.Parallel()

	type step struct {
		fn string // "AB", "B", "A", "clear"
		v  float64
		w  float64
	}
	tests := []struct {
		name  string
		steps []step
		a, b  float64
	}{
		{"default is the first 30 s", nil, 0, 30},
		{"explicit loop, no least length", []step{{"AB", 73, 101}}, 73, 101},
		{"explicit loop kept in the media", []step{{"AB", -5, 999}}, 0, 220},
		{"explicit ends given swapped", []step{{"AB", 90, 40}}, 40, 90},
		{"B below A+30 goes to A+30", []step{{"AB", 50, 60}, {"B", 55, 0}}, 50, 80},
		{"B within the media", []step{{"AB", 50, 60}, {"B", 120, 0}}, 50, 120},
		{"B past the end clamps", []step{{"B", 999, 0}}, 0, 220},
		{"B below 0 clamps to A+30", []step{{"B", -5, 0}}, 0, 30},
		{"fewer than 30 s after A: B is the end", []step{{"AB", 200, 210}, {"B", 205, 0}}, 200, 220},
		{"A moves with the swap rule", []step{{"A", 50, 0}}, 30, 50},
		{"clear restores", []step{{"AB", 10, 20}, {"clear", 0, 0}}, 0, 30},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			r := newRig(t, fullLength())
			r.playVideo(0)
			for _, s := range tc.steps {
				switch s.fn {
				case "AB":
					r.p.SetAB(s.v, s.w)
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

	r := newRig(t, rigOpts{eng: fake.Options{VideoDuration: 12}})
	r.playVideo(0)
	r.p.SetAB(2, 3)
	r.p.ClearAB()

	assert.InDelta(t, 0, r.p.A(), 0)
	assert.InDelta(t, 12, r.p.B(), 0, "B does not pass the end")
}

// TestLoopTurnsOnAtThePosition checks K38 rule 1: the loop begins where the
// media is, B is 30 s on or the end, and nothing seeks.
func TestLoopTurnsOnAtThePosition(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		at    float64
		audio bool
		a, b  float64
	}{
		{"at the start", 0, false, 0, 30},
		{"in the middle", 72, false, 72, 102},
		{"30 s from the end", 190, false, 190, 220},
		{"fewer than 30 s from the end: B is the end", 205, false, 205, 220},
		{"at the end", 220, false, 220, 220},
		{"audio the same", 72, true, 72, 102},
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
			r.p.Seek(tc.at)
			seeks := r.count(fake.OpSeek)
			r.p.SetLoop(true)

			assert.True(t, r.p.Loop())
			assert.InDelta(t, tc.a, r.p.A(), 0)
			assert.InDelta(t, tc.b, r.p.B(), 0)
			assert.Equal(t, seeks, r.count(fake.OpSeek), "turning it on does not seek")
			assert.InDelta(t, tc.at, r.p.Pos(), 0)
			r.p.SetLoop(false)
			assert.InDelta(t, tc.a, r.p.A(), 0, "off keeps the ends")
		})
	}
}

// TestLoopSetBKeepsTheLeastLengthAndJumpsToA checks K38 rules 2 and 3: B moves in
// [A + 30, end], and a B that falls at or before the position takes the media
// back to A; a B that stays ahead does not seek.
func TestLoopSetBKeepsTheLeastLengthAndJumpsToA(t *testing.T) {
	t.Parallel()

	r := newRig(t, fullLength())
	r.playVideo(0)
	r.p.SetRate(1)
	r.p.Seek(50)
	r.p.SetLoop(true)
	require.InDelta(t, 80, r.p.B(), 0)
	seeks := r.count(fake.OpSeek)

	r.p.SetB(110) // +30 step
	assert.InDelta(t, 110, r.p.B(), 0)
	r.p.SetB(80) // -30 step
	assert.InDelta(t, 80, r.p.B(), 0)
	r.p.SetB(60) // under A+30
	assert.InDelta(t, 80, r.p.B(), 0)
	assert.Equal(t, seeks, r.count(fake.OpSeek), "no seek: the position 50 is before B")

	r.p.SetB(999)
	assert.InDelta(t, 220, r.p.B(), 0, "the end")
	r.tick(4 * 40) // 40 s on: 90, past the B of 80 asked next
	assert.InDelta(t, 90, r.p.Pos(), 0)
	seeks = r.count(fake.OpSeek)
	r.p.SetB(120)
	assert.Equal(t, seeks, r.count(fake.OpSeek), "B 120 is past 90")
	r.p.SetB(80) // A+30, behind the position 90
	assert.Equal(t, seeks+1, r.count(fake.OpSeek))
	assert.InDelta(t, 50, r.p.Pos(), 0, "the position was past B: back to A")
}

// TestLoopSetBOffTheLoopDoesNotSeek checks that B moves with the loop off and
// the position past it, with no jump.
func TestLoopSetBOffTheLoopDoesNotSeek(t *testing.T) {
	t.Parallel()

	r := newRig(t, fullLength())
	r.playVideo(0)
	r.p.Seek(150)
	seeks := r.count(fake.OpSeek)
	r.p.SetB(60)

	assert.Equal(t, seeks, r.count(fake.OpSeek))
	assert.InDelta(t, 150, r.p.Pos(), 0)
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
		{"on: no jump at once", false, true, 0, 0, 0},
		{"on: just before B", false, true, 119, 0, 119 * 0.25},
		{"on: at B back to A", false, true, 120, 1, 0},
		{"on: second round", false, true, 120 + 60, 1, 60 * 0.25},
		{"off: no jump", false, false, 120, 0, 30},
		{"on then off: runs past B", false, true, -1, 0, 30},
		{"audio loops too", true, true, 120, 1, 0},
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
				ticks = 120
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

	assert.Equal(t, 0, r.count(fake.OpSeek))
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
