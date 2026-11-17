// Adapted from jplayer-mobile/internal/ui/gesture/velocity_test.go at 49de3c2.

package gesture

import (
	"testing"
	"time"

	"github.com/gdeskdev/desk/foundation/core/geom"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// The velocity of a Track begun with PressAt is a fit of the finger's latest
// samples timed by the events' times (gDesk's ui.Scroll tracker): Android
// delivers the samples of a slow frame together, microseconds apart in the
// app's time.

// hz240 is the time between the samples of a 240 Hz touch screen.
const hz240 = time.Second / 240

// samples is a finger moving down along y at speed DIPs a millisecond,
// reported every step: n samples after the press.
type samples struct {
	speed float32
	step  time.Duration
	n     int
	// burst is how many samples are handled at one time of the view's clock.
	burst int
	exact bool
}

// play presses a Track at the origin, moves it through the samples and
// returns it with the time of the last one.
func (s samples) play(tr *Track) (last time.Duration) {
	const sys = time.Hour // the system's clock, unlike the view's
	view := time.Duration(0)
	stamp := func(i int) time.Duration {
		if s.exact {
			return sys + s.step*time.Duration(i)
		}

		return 0
	}
	tr.PressAt(geom.Pt(0, 0), stamp(0), view)
	for i := 1; i <= s.n; i++ {
		if i%s.burst == 1 || s.burst == 1 {
			view += s.step * time.Duration(s.burst)
		}
		ms := float32(s.step*time.Duration(i)) / float32(time.Millisecond)
		tr.MoveAt(geom.Pt(0, s.speed*ms), stamp(i), view)
	}

	return view
}

func TestTrackFitVelocity(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		s    samples
		want float32 // DIPs a millisecond
	}{
		{"240 Hz on time", samples{.3, hz240, 24, 1, true}, .3},
		{"240 Hz in bursts of 8, one time of the view's clock", samples{.3, hz240, 24, 8, true}, .3},
		{"120 Hz in bursts of 3", samples{.9, time.Second / 120, 12, 3, true}, .9},
		{"a hard throw is held to 8", samples{12, hz240, 8, 4, true}, fitMax},
		{"no times, 60 Hz, the view's clock", samples{.3, time.Second / 60, 8, 1, false}, .3},
		{"no times, 60 Hz, a fast swipe", samples{1.2, time.Second / 60, 8, 1, false}, 1.2},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			var tr Track
			tr.Want = AxisY
			tc.s.play(&tr)
			assert.InDelta(t, tc.want, tr.Velocity().Y, float64(tc.want)*.01+.001)
			assert.Zero(t, tr.Velocity().X)
		})
	}
}

// TestTrackLastStepReadsABurstAsSpeed shows what PressAt fixes: the step
// alone divides the move by the view's clock, a microsecond or the floor of a
// millisecond for the samples that came together.
func TestTrackLastStepReadsABurstAsSpeed(t *testing.T) {
	t.Parallel()

	var old, fit Track
	old.Want, fit.Want = AxisY, AxisY
	const sys = time.Hour
	old.Press(geom.Pt(0, 0), 0)
	fit.PressAt(geom.Pt(0, 0), sys, 0)
	for i := 1; i <= 8; i++ {
		// Eight samples at .3 DIP/ms, 240 Hz, all handled at 33 ms.
		p := geom.Pt(0, .3*float32(hz240*time.Duration(i))/float32(time.Millisecond))
		old.Move(p, 33*time.Millisecond)
		fit.MoveAt(p, sys+hz240*time.Duration(i), 33*time.Millisecond)
	}
	assert.Greater(t, old.Velocity().Y, float32(1), "the last step over the floor of 1 ms")
	assert.InDelta(t, .3, fit.Velocity().Y, .005)
}

func TestTrackFitRelease(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		exact bool
		rest  time.Duration // from the last sample to the release
		want  bool          // flings
	}{
		{"times, 30 ms after the last sample", true, 30 * time.Millisecond, true},
		{"times, 50 ms after: the finger stopped", true, 50 * time.Millisecond, false},
		{"view's clock, 90 ms after", false, 90 * time.Millisecond, true},
		{"view's clock, 110 ms after: the finger stopped", false, 110 * time.Millisecond, false},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			var tr Track
			tr.Want = AxisY
			s := samples{1, time.Second / 60, 8, 1, tc.exact}
			last := s.play(&tr)
			end := geom.Pt(0, 1*float32(s.step*8)/float32(time.Millisecond))
			var evTime time.Duration
			if tc.exact {
				evTime = time.Hour + s.step*8 + tc.rest
			}
			d, v, started := tr.ReleaseAt(end, evTime, last+tc.rest)
			assert.True(t, started)
			assert.InDelta(t, end.Y, d.Y, 1e-3)
			if tc.want {
				assert.InDelta(t, 1, v.Y, .01)
			} else {
				assert.Zero(t, v.Y)
			}
		})
	}
}

// TestTrackClockIsDecidedAtPress checks a gesture never mixes its clocks: one
// pressed with a time keeps to the events' and takes a move without one to be
// at the last time; one pressed without keeps to the view's, whatever times
// its moves carry.
func TestTrackClockIsDecidedAtPress(t *testing.T) {
	t.Parallel()

	t.Run("exact, a move without a time", func(t *testing.T) {
		t.Parallel()

		var tr Track
		tr.Want = AxisY
		tr.PressAt(geom.Pt(0, 0), time.Hour, 0)
		tr.MoveAt(geom.Pt(0, 10), time.Hour+10*time.Millisecond, 10*time.Millisecond)
		require.Equal(t, StepMove, tr.MoveAt(geom.Pt(0, 10.2), 0, 20*time.Millisecond), "no time: at the last sample")
		tr.MoveAt(geom.Pt(0, 20), time.Hour+20*time.Millisecond, 30*time.Millisecond)
		assert.InDelta(t, 1, tr.Velocity().Y, .05, "the view's clock did not enter")
	})
	t.Run("not exact, the moves carry times", func(t *testing.T) {
		t.Parallel()

		var tr Track
		tr.Want = AxisY
		tr.PressAt(geom.Pt(0, 0), 0, 0)
		for i := 1; i <= 6; i++ {
			// The view's clock says 10 ms a step; a stray time says 1 s.
			tr.MoveAt(geom.Pt(0, float32(10*i)), time.Duration(i)*time.Second, time.Duration(i)*10*time.Millisecond)
		}
		assert.InDelta(t, 1, tr.Velocity().Y, .01)
	})
}

func TestTrackFitSlowsBeforeLifting(t *testing.T) {
	t.Parallel()

	var tr Track
	tr.Want = AxisY
	const sys = time.Hour
	tr.PressAt(geom.Pt(0, 0), sys, 0)
	// 2 DIP/ms for 40 ms, then crawling at .1 for 40 ms.
	y := float32(0)
	for i := 1; i <= 16; i++ {
		y += 2 * float32(hz240.Seconds()*1000)
		if i > 8 {
			y -= 1.9 * float32(hz240.Seconds()*1000)
		}
		tr.MoveAt(geom.Pt(0, y), sys+hz240*time.Duration(i), 0)
	}
	assert.Less(t, tr.Velocity().Y, float32(.5), "the slowdown is what the fit reports at the end")
	assert.GreaterOrEqual(t, tr.Velocity().Y, float32(0))
}

// TestDragReleaseAtDecides checks the page drags' dismissal by the fit: the
// thresholds are the prototype's, the speed is the finger's.
func TestDragReleaseAtDecides(t *testing.T) {
	t.Parallel()

	drags := []struct {
		name string
		new  func() Drag
		fast float32 // DIPs a millisecond, over the fling
		slow float32 // under it
	}{
		{"sheet", SheetDrag, .9, .3},
	}
	for _, dg := range drags {
		for _, tc := range []struct {
			name    string
			speed   float32
			rest    time.Duration
			dismiss bool
		}{
			{"slow", dg.slow, 4 * time.Millisecond, false},
			{"fast", dg.fast, 4 * time.Millisecond, true},
			{"fast, rested", dg.fast, 60 * time.Millisecond, false},
		} {
			t.Run(dg.name+" "+tc.name, func(t *testing.T) {
				t.Parallel()

				d := dg.new()
				const sys = time.Hour
				d.PressAt(geom.Pt(0, 0), sys, 0)
				var end geom.Point
				for i := 1; i <= 12; i++ { // bursts of 8 at one view time (45 DIPs: under the distance)
					end = geom.Pt(0, tc.speed*float32(hz240*time.Duration(i))/float32(time.Millisecond))
					d.MoveAt(end, sys+hz240*time.Duration(i), time.Duration(i/8)*33*time.Millisecond)
				}
				assert.Equal(t, tc.dismiss, d.ReleaseAt(end, sys+12*hz240+tc.rest, 99*time.Millisecond))
			})
		}
	}
}

func TestTrackFitMoveDoesNotAllocate(t *testing.T) { //nolint:paralleltest // AllocsPerRun
	for _, exact := range []bool{true, false} {
		tr := Track{Want: AxisX}
		tr.PressAt(geom.Pt(0, 0), touchtestBase(exact), 0)
		x := float32(0)
		now := time.Duration(0)
		assert.Zero(t, testing.AllocsPerRun(200, func() {
			x += 3
			now += hz240
			tr.MoveAt(geom.Pt(x, 1), touchtestBase(exact)+now, now)
			_ = tr.Velocity()
			_, _, _ = tr.ReleaseAt(geom.Pt(x, 1), touchtestBase(exact)+now, now)
			tr.armed, tr.started = true, true // keep following after the release
		}), "exact times: %v", exact)
	}
}

// touchtestBase is the first event time of a gesture, 0 when it has none.
func touchtestBase(exact bool) time.Duration {
	if exact {
		return time.Hour
	}

	return 0
}
