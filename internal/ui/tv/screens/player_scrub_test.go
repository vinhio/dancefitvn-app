//go:build tvos

package screens_test

import (
	"image/color"
	"testing"
	"time"

	"dancefitvn-app/internal/player"
	"dancefitvn-app/internal/ui/tv/tvtest"

	"github.com/gdeskdev/desk/foundation/core/keys"
	"github.com/gdeskdev/desk/foundation/platform"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ====================================================================
// ============================ Contracts =============================
// ====================================================================

// scrubIdle is the quiet time after the last arrow that the seek waits for
// (playerScrubIdle).
const scrubIdle = 700 * time.Millisecond

// ====================================================================
// ========================== Main functions ==========================
// ====================================================================

// onScrub opens TV07 (72 of 220 s, paused) with the focus on the position bar.
func onScrub(t testing.TB) *rig {
	t.Helper()
	r := inPlayer(t, tvtest.NewPlayerServices(t), false)
	require.Equal(t, "p1", r.key())
	require.True(t, r.press(keys.KeyUp))
	require.Equal(t, "ps", r.key(), "Up from the buttons reaches the bar")

	return r
}

// TestPlayerScrubSeeksOnceAfterTheArrowsStop: Right and Left on the focused bar
// move the clock and the fill at once and ask the player for the position one
// time, when the arrows have stopped (a swipe on the touch surface is one arrow for
// each quarter of it).
func TestPlayerScrubSeeksOnceAfterTheArrowsStop(t *testing.T) {
	t.Parallel()
	r := onScrub(t)
	pb := r.playback()
	pos := func() string { return find(t, r.layout(), "tv.player.pos").Text }
	assert.Equal(t, "01:12", pos())

	for range 3 {
		require.True(t, r.press(keys.KeyRight), "the bar uses Right")
	}
	assert.Equal(t, "01:42", pos(), "the clock follows the arrows")
	assert.InDelta(t, 102./220, r.player().Scrub().Value, 1e-3, "and the bar")
	assert.Zero(t, pb.Calls["Seek"], "nothing is asked of the player yet")
	assert.Equal(t, "ps", r.key(), "the focus stays")

	require.True(t, r.press(keys.KeyLeft))
	assert.Equal(t, "01:32", pos())
	r.tt.Elapse(scrubIdle - 100*time.Millisecond)
	assert.Zero(t, pb.Calls["Seek"], "still waiting for the arrows to stop")
	pb.Tick(72) // the 4 Hz tick of the player does not undo the scrub
	r.tt.Elapse(tvtest.Frame)
	assert.Equal(t, "01:32", pos())

	r.tt.Elapse(200 * time.Millisecond)
	assert.Equal(t, []float64{92}, pb.Seeks, "one seek, at the position reached")
	r.tt.Elapse(tvtest.Frame)
	assert.Equal(t, "01:32", pos(), "the player's own position now")
	r.tt.Elapse(2 * scrubIdle)
	assert.Equal(t, 1, pb.Calls["Seek"], "and no other")
}

// TestPlayerScrubHeldArrowIsOneSeek: a held Right (the driver repeats it every
// 125 ms after 400 ms) is one scrub: one seek, after the key is up.
func TestPlayerScrubHeldArrowIsOneSeek(t *testing.T) {
	t.Parallel()
	r := onScrub(t)
	pb := r.playback()
	r.tt.KeyDown(keys.KeyRight, 0)
	r.tt.Elapse(400 * time.Millisecond)
	for range 8 {
		r.tt.KeyRepeat(keys.KeyRight, 0)
		r.tt.Elapse(125 * time.Millisecond)
	}
	r.tt.KeyUp(keys.KeyRight, 0)
	assert.Zero(t, pb.Calls["Seek"], "no seek while the key is held")
	assert.Equal(t, "ps", r.key())
	r.tt.Elapse(scrubIdle + tvtest.Frame)
	assert.Equal(t, []float64{72 + 90}, pb.Seeks)
}

// TestPlayerScrubStaysInTheVideo: the arrows stop at the start and one second
// before the end, as the buttons of the phone do.
func TestPlayerScrubStaysInTheVideo(t *testing.T) {
	t.Parallel()
	r := onScrub(t)
	pb := r.playback()
	for range 12 {
		r.press(keys.KeyLeft)
	}
	r.tt.Elapse(scrubIdle + tvtest.Frame)
	for range 30 {
		r.press(keys.KeyRight)
	}
	r.tt.Elapse(scrubIdle + tvtest.Frame)
	assert.Equal(t, []float64{0, 219}, pb.Seeks)
	assert.Equal(t, "03:39", find(t, r.layout(), "tv.player.pos").Text)
}

// TestPlayerScrubSelectAppliesAtOnce: Select on the bar seeks now, and with no
// scrub under way it plays and pauses as `p1` does.
func TestPlayerScrubSelectAppliesAtOnce(t *testing.T) {
	t.Parallel()
	r := onScrub(t)
	pb := r.playback()
	r.press(keys.KeyRight)
	require.True(t, r.press(keys.KeyEnter))
	assert.Equal(t, []float64{82}, pb.Seeks)
	r.tt.Elapse(2 * scrubIdle)
	assert.Equal(t, 1, pb.Calls["Seek"], "the timer is gone")
	assert.Zero(t, pb.Calls["Toggle"])
	require.True(t, r.press(keys.KeyEnter))
	assert.Equal(t, 1, pb.Calls["Toggle"], "nothing to apply: Play/Pause")
}

// TestPlayerScrubAppliesWhenTheFocusLeaves: Down to the buttons applies the scrub
// at once.
func TestPlayerScrubAppliesWhenTheFocusLeaves(t *testing.T) {
	t.Parallel()
	r := onScrub(t)
	pb := r.playback()
	r.press(keys.KeyRight)
	r.press(keys.KeyRight)
	require.True(t, r.press(keys.KeyDown))
	assert.Equal(t, "p2", r.key(), "the nearest button under the bar's middle")
	assert.Equal(t, []float64{92}, pb.Seeks)
}

// TestPlayerScrubDroppedWithItsLesson: a lesson that gives way to another takes
// the pending scrub with it.
func TestPlayerScrubDroppedWithItsLesson(t *testing.T) {
	t.Parallel()
	r := onScrub(t)
	pb := r.playback()
	r.press(keys.KeyRight)
	pb.LessonV.ID = "other"
	pb.Publish(player.ChangeLesson)
	r.tt.Elapse(2 * scrubIdle)
	assert.Zero(t, pb.Calls["Seek"])
}

// TestPlayerScrubBarIsASlider: a screen reader finds an adjustable bar whose
// increment and decrement step the position and apply it.
func TestPlayerScrubBarIsASlider(t *testing.T) {
	t.Parallel()
	r := onScrub(t)
	pb := r.playback()
	bar, ok := findNode(r.tt.Semantics(), "Vị trí phát", platform.RoleSlider)
	require.True(t, ok)
	assert.True(t, bar.Actions.Has(platform.ActIncrement|platform.ActDecrement))
	assert.True(t, r.tt.PerformAction("Vị trí phát", platform.ActIncrement))
	assert.True(t, r.tt.PerformAction("Vị trí phát", platform.ActDecrement))
	assert.Equal(t, []float64{82, 72}, pb.Seeks)
}

// TestPlayerScrubFocusShowsAKnob: the focused bar has a knob on the end of the fill,
// white, which the bar at rest does not.
func TestPlayerScrubFocusShowsAKnob(t *testing.T) {
	t.Parallel()
	r := inPlayer(t, tvtest.NewPlayerServices(t), false)
	at := func() color.RGBA {
		bar := find(t, r.layout(), "div.scrub")
		x, y := int(bar.X+bar.W*72/220), int(bar.Y+bar.H/2)

		return r.pixels().RGBAAt(x, y+8)
	}
	rest := at()
	assert.NotEqual(t, color.RGBA{255, 255, 255, 255}, rest)
	r.press(keys.KeyUp)
	r.tt.Elapse(200 * time.Millisecond)
	assert.Equal(t, color.RGBA{255, 255, 255, 255}, at(), "the knob is under the end of the fill")
}

// TestPlayerScrubNeedsALength: a bar with no length known does nothing.
func TestPlayerScrubNeedsALength(t *testing.T) {
	t.Parallel()
	svc := tvtest.NewPlayerServices(t)
	pb := svc.PB.(*tvtest.Playback)
	pb.DurV, pb.LessonV.DurationSec = 0, 0
	r := inPlayer(t, svc, false)
	require.True(t, r.press(keys.KeyUp))
	require.Equal(t, "ps", r.key())
	r.press(keys.KeyRight)
	r.tt.Elapse(2 * scrubIdle)
	assert.Zero(t, pb.Calls["Seek"])
}

// TestPlayerRowsLeftDoesNotLandOnTheBar: Left from a next-lesson row goes to the
// buttons as before, not to the bar, whose Left would seek.
func TestPlayerRowsLeftDoesNotLandOnTheBar(t *testing.T) {
	t.Parallel()
	r := inPlayer(t, tvtest.NewPlayerServices(t), false)
	for _, k := range []string{"pn0", "pn1", "pn2", "pn3"} {
		r.focusKey(k)
		require.True(t, r.press(keys.KeyLeft))
		assert.Contains(t, []string{"p1", "p2"}, r.key(), "from %s", k)
	}
}

// TestPlayerScrubDoesNotAllocate: the frames with the knob showing, with a scrub
// pending, and the ticks of the player that come while it is, allocate nothing (an
// arrow writes the new clock, as a tick that crosses a second does).
func TestPlayerScrubDoesNotAllocate(t *testing.T) { //nolint:paralleltest // AllocsPerRun needs the process to itself
	skipUnderRace(t)
	r := onScrub(t)
	pb := r.playback()
	r.tt.Elapse(time.Second)
	r.tt.Repaint()
	assert.Zero(t, testing.AllocsPerRun(20, r.tt.Repaint), "the steady frame with the knob")
	r.press(keys.KeyRight)
	r.tt.Repaint()
	assert.Zero(t, testing.AllocsPerRun(20, r.tt.Repaint), "the frame of a pending scrub")
	tick := func() {
		pb.Tick(72)
		r.tt.Elapse(tvtest.Frame)
	}
	tick()
	assert.Zero(t, testing.AllocsPerRun(20, tick), "a tick of the player during the scrub")
}
