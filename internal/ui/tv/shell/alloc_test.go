//go:build tvos

package shell_test

import (
	"testing"
	"time"

	"dancefitvn-app/internal/ui/kit/kittest"
	"dancefitvn-app/internal/ui/tv/env"

	"github.com/gdeskdev/desk/foundation/core/keys"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ====================================================================
// ========================== Main functions ==========================
// ====================================================================

// skipUnderRace skips an allocation test in a -race build: the detector
// instruments every access (07 section 2.7); the plain build asserts the zero.
func skipUnderRace(tb testing.TB) {
	tb.Helper()
	if kittest.Race {
		tb.Skip("allocation counts are asserted in the build without -race")
	}
}

// walker returns a func that moves the focus along the first row of a screen, Right then Left, with the frames of the focus animation
// after each press.
func walker(tb testing.TB, route, first string) (step func(), r *rig) {
	tb.Helper()
	rg := open(tb, route, false)
	require.Equal(tb, first, rg.key())
	right := true
	step = func() {
		k := keys.KeyRight
		if !right {
			k = keys.KeyLeft
		}
		right = !right
		rg.tt.Key(k, 0)
		rg.tt.Elapse(40 * time.Millisecond) // the frames of the scale transition
	}
	for range 8 {
		step()
	}

	return step, rg
}

// TestTVFocusMoveDoesNotAllocate: an arrow press between two nav items, with the
// focus animation's frames after it, allocates nothing (07 section 4).
func TestTVFocusMoveDoesNotAllocate(t *testing.T) { //nolint:paralleltest // AllocsPerRun needs the process to itself
	skipUnderRace(t)
	nav, _ := walker(t, env.RouteSettings, "nav0")
	assert.InDelta(t, 0, testing.AllocsPerRun(50, nav), 0, "Right/Left along the nav bar, animated")
	buttons, _ := walker(t, env.RouteHome, "h1")
	assert.InDelta(t, 0, testing.AllocsPerRun(50, buttons), 0, "Right/Left along the buttons, animated")
}

// TestTVKeysDoNotAllocate: a Select tap, a Menu nobody takes, a Play/Pause with
// a handler and a steady frame allocate nothing.
func TestTVKeysDoNotAllocate(t *testing.T) { //nolint:paralleltest // AllocsPerRun needs the process to itself
	skipUnderRace(t)
	r := open(t, env.RouteWelcome, false)
	calls := 0
	r.sh.OnPlayPause(func() { calls++ })
	r.welcome().Item("w1").OnSelect = func() { calls++ }
	require.Equal(t, "w1", r.key())

	assert.InDelta(t, 0, testing.AllocsPerRun(50, func() { r.tt.Key(keys.KeyEnter, 0) }), 0, "Select tap")
	assert.InDelta(t, 0, testing.AllocsPerRun(50, func() { r.tt.Key(keys.KeyEscape, 0) }), 0, "Menu at a root")
	assert.InDelta(t, 0, testing.AllocsPerRun(50, func() { r.tt.Key(keys.KeyMediaPlayPause, 0) }), 0, "Play/Pause")
	assert.InDelta(t, 0, testing.AllocsPerRun(50, func() { r.tt.Elapse(16 * time.Millisecond) }), 0, "a steady frame")
	assert.Positive(t, calls)
}

// TestTVHoldDoesNotAllocate: a Select held to 550 ms and released allocates
// nothing.
func TestTVHoldDoesNotAllocate(t *testing.T) { //nolint:paralleltest // AllocsPerRun needs the process to itself
	skipUnderRace(t)
	r := open(t, env.RouteWelcome, false)
	n := 0
	r.welcome().Item("w1").OnHold = func() { n++ }
	step := func() {
		r.tt.KeyDown(keys.KeyEnter, 0)
		r.tt.Elapse(600 * time.Millisecond)
		r.tt.KeyUp(keys.KeyEnter, 0)
	}
	step()
	assert.InDelta(t, 0, testing.AllocsPerRun(20, step), 0)
	assert.Positive(t, n)
}

// BenchmarkTVShellFocusMove is a Right/Left pair along the nav bar.
func BenchmarkTVShellFocusMove(b *testing.B) {
	step, _ := walker(b, env.RouteSettings, "nav0")
	b.ReportAllocs()
	for b.Loop() {
		step()
	}
}
