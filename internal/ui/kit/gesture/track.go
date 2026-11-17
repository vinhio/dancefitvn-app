// Adapted from jplayer-mobile/internal/ui/gesture/track.go at 49de3c2.

package gesture

import (
	"math"
	"time"

	"github.com/gdeskdev/desk/foundation/core/geom"
)

// ====================================================================
// ============================ Contracts =============================
// ====================================================================

// Axis is the direction a drag locked to.
type Axis uint8

// The axes. AxisUnknown is "not locked yet" for a Track, and "either" for
// the axis a Track wants.
const (
	AxisUnknown Axis = iota
	AxisX
	AxisY
)

// Slop is how far a finger moves, on either axis, before a drag locks to
// one: SLOP of js/gestures.js:9. It is the same as ui.TouchSlop.
const Slop float32 = 8

// Step tells what a move did to a Track.
type Step uint8

// The steps. StepUnknown is a Track that was never pressed.
const (
	StepUnknown Step = iota
	// StepWait: the finger is within Slop of where it pressed: nothing yet.
	StepWait
	// StepStart: the axis locked to the wanted one: the drag starts with
	// this move, which is also its first.
	StepStart
	// StepMove: a move of a drag that started.
	StepMove
	// StepRefuse: the axis locked to the other one, or the press is over:
	// the drag will not start, and the press is another gesture's.
	StepRefuse
)

// The tunings of the velocity fit a Track gives with PressAt. They are those
// of gDesk's ui.Scroll tracker (framework/ui/velocity.go), which is not
// exported: the app mirrors it so that its drags measure a finger as a scroll
// does, by the touches' own times.
const (
	// fitSamples is how many of the latest samples are kept, as many as
	// Android's VelocityTracker keeps (velocitySamples).
	fitSamples = 20
	// fitHorizon is how far before the newest sample the fit looks, and how
	// long after it a release is taken to come from a finger at rest when
	// the times are not the events' (velocityHorizon).
	fitHorizon = 100 * time.Millisecond
	// fitRecency is the time constant of the samples' weights,
	// e^(-age/10 ms) (velocityRecency): at 60 Hz the samples before the
	// newest weigh 0.19, 0.036, 0.007, so a steady swipe measures its speed
	// and a finger slowing down before it lifts measures the slowdown.
	fitRecency = 10 * time.Millisecond
	// fitPause is how long after the newest sample, with the events' times,
	// a release is taken to come from a finger that stopped
	// (velocityPause, Android's ASSUME_POINTER_STOPPED_TIME).
	fitPause = 40 * time.Millisecond
	// fitMerge is the gap under which a sample takes the place of the one
	// before it (velocityMerge).
	fitMerge = time.Millisecond
	// fitMax caps a velocity, in DIPs a millisecond: flingMax of gDesk,
	// Android's 8000 DIPs a second.
	fitMax = 8
)

// fitRecencyMillis is fitRecency in milliseconds, the unit of the fit's times.
const fitRecencyMillis = float64(fitRecency) / float64(time.Millisecond)

// Track follows one press: where it began, the axis it locked to past Slop,
// and its velocity. After Press that is the velocity of its last step alone,
// as G.track of js/gestures.js:13-48 has it, without the DOM: the widget feeds
// it the events. After PressAt it is the fit of the finger's latest samples,
// timed by the events' own times when the driver gives them (Android emits
// every sample the system batched, microseconds apart in the app's time, and
// the step alone would read them as a burst of speed) and by the view's
// clock otherwise.
type Track struct {
	// Want is the axis the drag needs; AxisUnknown takes either
	// (`o.axis` of G.track, js/gestures.js:26).
	Want Axis

	x0, y0 float32 // where the press began (js/gestures.js:16 x0, y0)
	lx, ly float32 // where the finger was at the last step (lx, ly)
	dx, dy float32 // the distance from the press at the last move
	vx, vy float32 // the velocity of the last step, in DIPs a millisecond
	// lt is when the last step was (js/gestures.js:17 lt).
	lt time.Duration
	// The samples of the fit, a ring: where the finger was at each time,
	// those of the events when exact.
	at      [fitSamples]time.Duration
	pos     [fitSamples]geom.Point
	n       int  // the samples kept
	next    int  // where the next sample goes in the ring
	fit     bool // PressAt: the velocity is the fit's
	exact   bool // the samples carry the events' times, not the view's clock
	locked  Axis
	armed   bool // pressed, not yet released, cancelled or refused
	started bool // the axis locked to the wanted one
	refused bool // the axis locked to the other one
}

// ====================================================================
// ========================== Main functions ==========================
// ====================================================================

// Press begins to follow a finger that went down at at, at time now
// (js/gestures.js:15-17).
func (t *Track) Press(at geom.Point, now time.Duration) {
	*t = Track{Want: t.Want, x0: at.X, y0: at.Y, lx: at.X, ly: at.Y, lt: now, armed: true}
}

// PressAt begins to follow a finger as Press does, and measures its velocity
// by a fit of its latest samples. The press event's time evTime, when not 0, is
// the clock of the whole gesture (MoveAt and ReleaseAt use the events' times);
// when 0 the gesture runs by now, the view's clock. It is decided here and
// kept: a gesture never mixes the two.
func (t *Track) PressAt(at geom.Point, evTime, now time.Duration) {
	t.Press(at, now)
	t.fit, t.exact = true, evTime != 0
	if t.exact {
		now = evTime
	}
	t.lt = now
	t.addSample(now, at)
}

// MoveAt is Move for a gesture begun with PressAt: it takes the event's time
// when the gesture runs by the events' times, and now, the view's clock,
// otherwise.
func (t *Track) MoveAt(at geom.Point, evTime, now time.Duration) Step {
	return t.Move(at, t.stamp(evTime, now))
}

// stamp returns the time of an event of the gesture by its clock. An event
// without a time in a gesture of exact times is taken to be at the last one.
func (t *Track) stamp(evTime, now time.Duration) time.Duration {
	switch {
	case !t.exact:
		return now
	case evTime == 0:
		return t.newest()
	default:
		return evTime
	}
}

// Move follows the finger to at, at time now, and tells what that did. The
// steps under Slop on both axes change nothing, the velocity included: the
// step that locks the axis is measured from the press (js/gestures.js:20,
// the `return` that leaves lx and lt alone). A gesture begun with PressAt
// takes those steps as samples of the fit too, and now is then the time the
// gesture runs by (MoveAt tells which).
func (t *Track) Move(at geom.Point, now time.Duration) Step {
	switch {
	case t.refused:
		return StepRefuse
	case !t.armed:
		return StepUnknown
	default: // a gesture that is on: go on below
	}
	if t.fit {
		t.addSample(now, at)
	}
	t.dx, t.dy = at.X-t.x0, at.Y-t.y0
	first := false
	if t.locked == AxisUnknown {
		if abs(t.dx) < Slop && abs(t.dy) < Slop {
			return StepWait
		}
		// js/gestures.js:21: equal distances lock to y.
		t.locked = AxisY
		if abs(t.dx) > abs(t.dy) {
			t.locked = AxisX
		}
		if t.Want != AxisUnknown && t.locked != t.Want {
			t.armed, t.refused = false, true

			return StepRefuse
		}
		t.started, first = true, true
	}
	if !t.fit {
		// js/gestures.js:27-28: the velocity of the last step alone, over
		// at least a millisecond.
		ms := float32(now-t.lt) / float32(time.Millisecond)
		if ms < 1 {
			ms = 1
		}
		t.vx, t.vy = (at.X-t.lx)/ms, (at.Y-t.ly)/ms
		t.lx, t.ly, t.lt = at.X, at.Y, now
	}
	if first {
		return StepStart
	}

	return StepMove
}

// Release ends the press with the finger lifted at at (js/gestures.js:32-36).
// It returns the distance from the press to at, the velocity of the last
// step, and whether the drag had started; a drag that had not is not one,
// and the press was a tap.
func (t *Track) Release(at geom.Point) (d, v geom.Point, started bool) {
	started = t.started
	d = geom.Pt(at.X-t.x0, at.Y-t.y0)
	v = t.Velocity()
	t.armed, t.started, t.refused = false, false, false

	return d, v, started
}

// ReleaseAt is Release for a gesture begun with PressAt, lifted at evTime by
// the system's clock (0 when it does not tell) and now by the view's. A finger
// that rested before it lifted does not fling: the velocity is 0 when the
// release comes more than 40 ms after the newest sample with the events' times
// (Android's ASSUME_POINTER_STOPPED_TIME), 100 ms with the view's clock.
func (t *Track) ReleaseAt(at geom.Point, evTime, now time.Duration) (d, v geom.Point, started bool) {
	rest := t.fit && t.stale(t.stamp(evTime, now))
	d, v, started = t.Release(at)
	if rest {
		v = geom.Point{}
	}

	return d, v, started
}

// Cancel ends the press taken away from the widget, and tells whether the
// drag had started (js/gestures.js:37-43: `cancel` is called only then).
func (t *Track) Cancel() (started bool) {
	started = t.started
	t.armed, t.started, t.refused = false, false, false

	return started
}

// Delta returns the distance from the press at the last move, the 8 DIPs of
// Slop included (js/gestures.js:19 dx, dy).
func (t *Track) Delta() geom.Point { return geom.Pt(t.dx, t.dy) }

// Velocity returns the velocity, in DIPs a millisecond: that of the last step
// (js/gestures.js:28 vx, vy), or after PressAt the fit's over the latest
// samples, held to 8 DIPs a millisecond.
func (t *Track) Velocity() geom.Point {
	if t.fit {
		return t.fitVelocity()
	}

	return geom.Pt(t.vx, t.vy)
}

// Locked returns the axis the drag locked to, AxisUnknown before Slop.
func (t *Track) Locked() Axis { return t.locked }

// Armed tells whether a press is followed: pressed, and not yet released,
// cancelled or refused.
func (t *Track) Armed() bool { return t.armed }

// Started tells whether the drag started: the axis locked to the wanted one
// and the press is not over.
func (t *Track) Started() bool { return t.started }

// Reset forgets the press; Want stays.
func (t *Track) Reset() { *t = Track{Want: t.Want} }

// ====================================================================
// ========================= Helper Functions =========================
// ====================================================================

// addSample records that the finger was at p at at. A sample older than the
// newest is left out; one within fitMerge of it takes the place of its position.
func (t *Track) addSample(at time.Duration, p geom.Point) {
	if t.n > 0 {
		last := (t.next + fitSamples - 1) % fitSamples
		dt := at - t.at[last]
		if dt < 0 {
			return
		}
		if dt < fitMerge {
			t.pos[last] = p

			return
		}
	}
	t.at[t.next], t.pos[t.next] = at, p
	t.next = (t.next + 1) % fitSamples
	t.n = min(t.n+1, fitSamples)
}

// newest returns the time of the newest sample, 0 when there is none.
func (t *Track) newest() time.Duration {
	if t.n == 0 {
		return 0
	}

	return t.at[(t.next+fitSamples-1)%fitSamples]
}

// stale tells whether a release at at comes long enough after the newest
// sample for the finger to have stopped.
func (t *Track) stale(at time.Duration) bool {
	limit := fitHorizon
	if t.exact {
		limit = fitPause
	}

	return at-t.newest() > limit
}

// fitVelocity returns the finger's velocity at the newest sample, in DIPs a
// millisecond: the slope of the least-squares line through the samples within
// fitHorizon of it, each weighted by e^(-age/fitRecency). Zero with fewer than
// two samples. It allocates nothing.
func (t *Track) fitVelocity() geom.Point {
	if t.n < 2 {
		return geom.Point{}
	}
	newest := (t.next + fitSamples - 1) % fitSamples
	t0, p0 := t.at[newest], t.pos[newest]
	// The weighted sums of the normal equations, t in milliseconds before
	// the newest sample (so at most 0) and positions from the newest, which
	// keeps the numbers near 1.
	var sw, st, stt, sx, sy, sxt, syt float64
	for k := range t.n {
		i := (newest + fitSamples - k) % fitSamples
		age := t0 - t.at[i]
		if age > fitHorizon {
			break
		}
		tm := -float64(age) / float64(time.Millisecond)
		w := math.Exp(tm / fitRecencyMillis)
		dx, dy := float64(t.pos[i].X-p0.X), float64(t.pos[i].Y-p0.Y)
		sw += w
		st += w * tm
		stt += w * tm * tm
		sx += w * dx
		sy += w * dy
		sxt += w * dx * tm
		syt += w * dy * tm
	}
	den := sw*stt - st*st
	if den <= 0 {
		return geom.Point{}
	}
	vx := (sw*sxt - st*sx) / den
	vy := (sw*syt - st*sy) / den

	return geom.Pt(float32(max(-fitMax, min(fitMax, vx))), float32(max(-fitMax, min(fitMax, vy))))
}

// abs returns the absolute value of v.
func abs(v float32) float32 {
	if v < 0 {
		return -v
	}

	return v
}
