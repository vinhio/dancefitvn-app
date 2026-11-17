// Adapted from jplayer-mobile/internal/app/uiclock_test.go at 49de3c2
// (TestUIClockDrivesStore dropped: the app has no store debounce).

package app

import (
	"testing"
	"time"

	"dancefitvn-app/internal/clock"

	"github.com/gdeskdev/desk/framework/ui"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// t0 is the wall time of the clock's tests.
var t0 = time.Date(2026, 11, 14, 9, 41, 0, 0, time.UTC)

// newBlankTester returns a tester of an empty screen, for the clock's
// tests.
func newBlankTester(t testing.TB) *ui.Tester {
	t.Helper()
	tt, err := ui.NewTester(ui.Spacer(0), 393, 852)
	require.NoError(t, err)
	t.Cleanup(tt.Close)

	return tt
}

// newClockTester returns a uiClock reading wall from *at, bound to a
// tester's view whose time Elapse drives.
func newClockTester(t testing.TB, at *time.Time) (*ui.Tester, *uiClock) {
	t.Helper()
	tt := newBlankTester(t)
	k := newUIClock(func() time.Time { return *at })
	k.Bind(tt.View())

	return tt, k
}

// TestUIClockElapse checks the view's time drives the clock: Now moves
// with Elapse, calls fire when due, in the order arranged for equal
// times, and Stop cancels.
func TestUIClockElapse(t *testing.T) {
	t.Parallel()

	at := t0
	tt, k := newClockTester(t, &at)
	assert.Equal(t, t0, k.Now(), "bound at the wall time")

	var got []string
	k.After(300*time.Millisecond, func() { got = append(got, "a") })
	k.After(300*time.Millisecond, func() { got = append(got, "b") })
	late := k.After(time.Second, func() { got = append(got, "late") })
	stopped := k.After(100*time.Millisecond, func() { got = append(got, "stopped") })
	require.NotZero(t, late)
	assert.True(t, k.Stop(stopped))
	assert.False(t, k.Stop(stopped), "a second stop")
	assert.False(t, k.Stop(0))

	tt.Elapse(290 * time.Millisecond)
	assert.Empty(t, got)
	tt.Elapse(20 * time.Millisecond)
	assert.Equal(t, []string{"a", "b"}, got)
	assert.Equal(t, 1, k.pending())
	assert.Equal(t, t0.Add(310*time.Millisecond), k.Now())

	assert.True(t, k.Stop(late))
	tt.Elapse(time.Second)
	assert.Equal(t, []string{"a", "b"}, got)
	assert.Zero(t, k.pending())
	assert.False(t, k.Stop(late), "stopped already")
	assert.Zero(t, k.After(time.Second, nil), "a nil call arranges nothing")
}

// TestUIClockFiredStop checks Stop of a call that fired reports false.
func TestUIClockFiredStop(t *testing.T) {
	t.Parallel()

	at := t0
	tt, k := newClockTester(t, &at)
	fired := 0
	id := k.After(50*time.Millisecond, func() { fired++ })
	tt.Elapse(100 * time.Millisecond)
	assert.Equal(t, 1, fired)
	assert.False(t, k.Stop(id))
}

// TestUIClockBeforeBind checks the calls arranged before Bind wait for
// it and fire for the time they had left, those already due at the
// view's next turn, and that Stop reaches them before Bind.
func TestUIClockBeforeBind(t *testing.T) {
	t.Parallel()

	at := t0
	k := newUIClock(func() time.Time { return at })
	assert.Equal(t, t0, k.Now(), "the wall time before Bind")
	var got []string
	k.After(500*time.Millisecond, func() { got = append(got, "half") })
	k.After(100*time.Millisecond, func() { got = append(got, "due") })
	gone := k.After(200*time.Millisecond, func() { got = append(got, "gone") })
	assert.True(t, k.Stop(gone))
	assert.False(t, k.Stop(gone))
	assert.Equal(t, 2, k.pending())

	at = t0.Add(200 * time.Millisecond) // the window opens 200 ms later
	tt := newBlankTester(t)
	k.Bind(tt.View())
	assert.Equal(t, at, k.Now())
	assert.Equal(t, 2, k.pending())

	tt.Elapse(time.Second / 60)
	assert.Equal(t, []string{"due"}, got, "overdue fires at the next turn")
	tt.Elapse(280 * time.Millisecond)
	assert.Equal(t, []string{"due"}, got)
	tt.Elapse(20 * time.Millisecond)
	assert.Equal(t, []string{"due", "half"}, got, "300 ms were left of 500")
}

// TestUIClockResync checks Resync moves Now up to the wall clock after
// the phone slept, and never back.
func TestUIClockResync(t *testing.T) {
	t.Parallel()

	at := t0
	tt, k := newClockTester(t, &at)
	tt.Elapse(time.Second)
	assert.Equal(t, t0.Add(time.Second), k.Now())

	at = t0.Add(time.Hour) // the phone slept an hour, the view's clock did not count it
	k.Resync()
	assert.Equal(t, t0.Add(time.Hour), k.Now())

	at = t0 // a wall clock set back does not take Now back
	k.Resync()
	assert.Equal(t, t0.Add(time.Hour), k.Now())
	tt.Elapse(time.Second)
	assert.Equal(t, t0.Add(time.Hour+time.Second), k.Now())
}

// TestUIClockRingGrows checks more pending calls than the ring's first
// slots all fire and stay stoppable.
func TestUIClockRingGrows(t *testing.T) {
	t.Parallel()

	at := t0
	tt, k := newClockTester(t, &at)
	const n = 3*uiFirstSlots + 5
	fired := make([]bool, n)
	ids := make([]clock.Timer, n)
	for i := range n {
		ids[i] = k.After(time.Duration(i+1)*time.Millisecond, func() { fired[i] = true })
	}
	assert.Equal(t, n, k.pending())
	assert.GreaterOrEqual(t, len(k.slots), n)
	assert.True(t, k.Stop(ids[n-1]))
	tt.Elapse(time.Second)
	for i := range n - 1 {
		assert.True(t, fired[i], "call %d", i)
	}
	assert.False(t, fired[n-1], "the stopped call")
	assert.Zero(t, k.pending())
}

// TestUIClockAfterStopDoesNotAllocate checks arranging and stopping a
// call made once allocates nothing in steady state.
func TestUIClockAfterStopDoesNotAllocate(t *testing.T) { //nolint:paralleltest // AllocsPerRun needs the process to itself
	at := t0
	_, k := newClockTester(t, &at)
	fn := func() {}
	k.Stop(k.After(time.Second, fn)) // grow the view's timers once
	allocs := testing.AllocsPerRun(100, func() {
		k.Stop(k.After(time.Second, fn))
		_ = k.Now()
	})
	assert.Zero(t, allocs)
}

// TestUIClockFireDoesNotAllocate checks a call firing through Elapse
// allocates nothing in steady state, as the player's sleep tick does.
func TestUIClockFireDoesNotAllocate(t *testing.T) { //nolint:paralleltest // AllocsPerRun needs the process to itself
	at := t0
	tt, k := newClockTester(t, &at)
	n := 0
	var tick func()
	tick = func() { n++; k.After(time.Second/60, tick) }
	k.After(time.Second/60, tick)
	tt.Elapse(time.Second)
	allocs := testing.AllocsPerRun(50, func() { tt.Elapse(time.Second / 60) })
	assert.Zero(t, allocs)
	assert.Positive(t, n)
}

// BenchmarkUIClockAfterStop times arranging and stopping a call.
func BenchmarkUIClockAfterStop(b *testing.B) {
	at := t0
	_, k := newClockTester(b, &at)
	fn := func() {}
	b.ReportAllocs()
	for b.Loop() {
		k.Stop(k.After(time.Second, fn))
	}
}
