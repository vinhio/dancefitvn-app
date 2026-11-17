// Adapted from jplayer-mobile/internal/ui/gesture/track_test.go at 49de3c2.

package gesture

import (
	"testing"
	"time"

	"dancefitvn-app/internal/ui/kit/kittest"
	"github.com/gdeskdev/desk/foundation/core/geom"
	"github.com/stretchr/testify/assert"
)

// ms is a time in milliseconds.
func ms(n int) time.Duration { return time.Duration(n) * time.Millisecond }

func TestTrackLocks(t *testing.T) {
	t.Parallel()

	press := geom.Pt(100, 100)
	tests := []struct {
		name   string
		want   Axis
		to     geom.Point
		step   Step
		locked Axis
	}{
		{"7 across is under the slop", AxisUnknown, geom.Pt(107, 100), StepWait, AxisUnknown},
		{"7 and 7 is under the slop", AxisUnknown, geom.Pt(107, 107), StepWait, AxisUnknown},
		{"8 across locks x", AxisUnknown, geom.Pt(108, 100), StepStart, AxisX},
		{"6 across 8 down locks y", AxisUnknown, geom.Pt(106, 108), StepStart, AxisY},
		{"8 and 8 lock y", AxisUnknown, geom.Pt(108, 108), StepStart, AxisY},
		{"9 across 8 down lock x", AxisUnknown, geom.Pt(109, 108), StepStart, AxisX},
		{"x wanted, x locked", AxisX, geom.Pt(92, 100), StepStart, AxisX},
		{"x wanted, y locked", AxisX, geom.Pt(104, 110), StepRefuse, AxisY},
		{"y wanted, x locked", AxisY, geom.Pt(120, 102), StepRefuse, AxisX},
		{"y wanted, y locked", AxisY, geom.Pt(102, 80), StepStart, AxisY},
		{"y wanted, 8 and 8", AxisY, geom.Pt(108, 108), StepStart, AxisY},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			tr := Track{Want: tc.want}
			tr.Press(press, 0)
			assert.True(t, tr.Armed())
			assert.Equal(t, tc.step, tr.Move(tc.to, ms(16)))
			assert.Equal(t, tc.locked, tr.Locked())
			assert.Equal(t, tc.step == StepStart, tr.Started())
			assert.Equal(t, tc.step != StepRefuse, tr.Armed(), "a refused press is no longer followed")
			if tc.step == StepRefuse {
				assert.Equal(t, StepRefuse, tr.Move(tc.to, ms(32)), "it stays refused")
			}
		})
	}
}

func TestTrackVelocity(t *testing.T) {
	t.Parallel()

	var tr Track
	tr.Press(geom.Pt(100, 100), ms(1000))
	// Under the slop the velocity is left alone: the step that locks is
	// measured from the press (js/gestures.js:20).
	assert.Equal(t, StepWait, tr.Move(geom.Pt(105, 100), ms(1010)))
	assert.Equal(t, StepStart, tr.Move(geom.Pt(110, 100), ms(1020)))
	assert.InDelta(t, .5, tr.Velocity().X, 1e-6, "10 DIPs in 20 ms from the press")
	assert.Equal(t, StepMove, tr.Move(geom.Pt(114, 100), ms(1030)))
	assert.InDelta(t, .4, tr.Velocity().X, 1e-6, "the last step alone: 4 DIPs in 10 ms")
	assert.Equal(t, StepMove, tr.Move(geom.Pt(114, 100), ms(1040)))
	assert.InDelta(t, 0, tr.Velocity().X, 1e-6, "a rest is a velocity of 0")
	// Under a millisecond counts as a millisecond.
	assert.Equal(t, StepMove, tr.Move(geom.Pt(116, 103), ms(1040)))
	assert.InDelta(t, 2, tr.Velocity().X, 1e-6)
	assert.InDelta(t, 3, tr.Velocity().Y, 1e-6)
	assert.Equal(t, geom.Pt(16, 3), tr.Delta(), "from the press")
}

func TestTrackRelease(t *testing.T) {
	t.Parallel()

	var tr Track
	tr.Press(geom.Pt(100, 100), 0)
	tr.Move(geom.Pt(120, 101), ms(10))
	tr.Move(geom.Pt(150, 101), ms(20))
	d, v, started := tr.Release(geom.Pt(160, 102))
	assert.True(t, started)
	assert.Equal(t, geom.Pt(60, 2), d, "from the press to the place of the lifting")
	assert.InDelta(t, 3, v.X, 1e-6, "the velocity of the last move, not of the lifting")
	assert.False(t, tr.Armed())
	assert.Equal(t, StepUnknown, tr.Move(geom.Pt(170, 102), ms(30)), "nothing follows a released press")

	tr.Press(geom.Pt(100, 100), 0)
	tr.Move(geom.Pt(103, 100), ms(10))
	_, _, started = tr.Release(geom.Pt(103, 100))
	assert.False(t, started, "under the slop the press was a tap")

	tr.Press(geom.Pt(100, 100), 0)
	tr.Move(geom.Pt(120, 100), ms(10))
	assert.True(t, tr.Cancel())
	assert.False(t, tr.Cancel())
}

func TestTrackDoesNotAllocate(t *testing.T) { //nolint:paralleltest // AllocsPerRun
	if kittest.Race {
		t.Skip("allocation counts are not stable under the race detector")
	}
	var tr Track
	tr.Press(geom.Pt(10, 10), 0)
	now := time.Duration(0)
	x := float32(10)
	assert.Zero(t, testing.AllocsPerRun(100, func() {
		now += frame
		x += 3
		tr.Move(geom.Pt(x, 10), now)
	}))
}

func BenchmarkTrackMove(b *testing.B) {
	var tr Track
	tr.Press(geom.Pt(10, 10), 0)
	now := time.Duration(0)
	x := float32(10)
	b.ReportAllocs()
	for b.Loop() {
		now += frame
		x += .5
		tr.Move(geom.Pt(x, 10), now)
	}
}
