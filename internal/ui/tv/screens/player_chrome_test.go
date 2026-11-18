//go:build tvos

package screens_test

import (
	"testing"
	"time"

	"dancefitvn-app/internal/player"
	"dancefitvn-app/internal/ui/tv/env"
	"dancefitvn-app/internal/ui/tv/nav"
	"dancefitvn-app/internal/ui/tv/tvtest"

	"github.com/gdeskdev/desk/foundation/core/keys"
	"github.com/gdeskdev/desk/foundation/platform"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ====================================================================
// ============================ Contracts =============================
// ====================================================================

// chromeIdle is the quiet time after which the chrome hides (playerChromeIdle,
// `CTL_HIDE_MS`, K46).
const chromeIdle = 3 * time.Second

// ====================================================================
// ========================== Main functions ==========================
// ====================================================================

// playingPlayer opens TV07 with the video playing (the state the auto-hide counts
// in) and the motion as the test asks for it: reduced makes the fade a jump.
func playingPlayer(t testing.TB, reduced bool) *rig {
	t.Helper()
	svc := tvtest.NewPlayerServices(t)
	var r *rig
	if reduced {
		r = openFast(t, svc, env.RouteHome, false)
	} else {
		r = open(t, svc, env.RouteHome, false)
	}
	r.settle()
	pb := r.playback() // after the home: leaving a screen pauses the playback
	pb.PlayingV, pb.StateV = true, player.StatePlaying
	r.sh.Go(nav.Route{Screen: env.RoutePlayer}, false) // over the home, as `h1` does: Menu goes back
	r.settle()
	require.Equal(t, "p1", r.key())
	require.True(t, r.player().Chrome(), "the chrome is up when the screen opens")

	return r
}

// TestPlayerChromeHidesAfterThreeSeconds: the video plays and no button comes: the
// chrome (lesson block, transport, list) is up at 2.9 s and gone at 3 s, and what
// is left is the video's plane and the page behind it.
func TestPlayerChromeHidesAfterThreeSeconds(t *testing.T) {
	t.Parallel()
	r := playingPlayer(t, true)
	p := r.player()
	r.tt.Elapse(chromeIdle - 100*time.Millisecond)
	assert.True(t, p.Chrome(), "up at 2.9 s")
	assert.Equal(t, float32(1), p.ChromeOpacity())
	r.tt.Elapse(200 * time.Millisecond)
	assert.False(t, p.Chrome(), "hidden at 3 s")
	assert.Equal(t, float32(0), p.ChromeOpacity(), "with the motion reduced, at once")

	img := r.pixels()
	for _, pt := range [][2]int{{150, 160}, {200, 960}, {1500, 200}, {1500, 520}, {1700, 1000}} {
		o := pt[1]*img.Stride + pt[0]*4
		assert.Equal(t, [3]uint8{0x0b, 0x0b, 0x0c}, [3]uint8{img.Pix[o], img.Pix[o+1], img.Pix[o+2]}, "nothing but the page at %v", pt)
	}
	assert.Equal(t, "p1", r.key(), "the focus stays where it was")
}

// TestPlayerChromeFades: with the motion on, the chrome fades over 250 ms (CSS
// `ease`), and fades back in over the same time.
func TestPlayerChromeFades(t *testing.T) {
	t.Parallel()
	r := playingPlayer(t, false)
	p := r.player()
	r.tt.Elapse(chromeIdle + tvtest.Frame)
	assert.False(t, p.Chrome())
	r.tt.Elapse(100 * time.Millisecond)
	op := p.ChromeOpacity()
	assert.Greater(t, op, float32(0), "still fading out at 100 ms")
	assert.Less(t, op, float32(1))
	r.tt.Elapse(200 * time.Millisecond)
	assert.Equal(t, float32(0), p.ChromeOpacity())

	require.True(t, r.press(keys.KeyDown))
	assert.True(t, p.Chrome())
	r.tt.Elapse(100 * time.Millisecond)
	op = p.ChromeOpacity()
	assert.Greater(t, op, float32(0))
	assert.Less(t, op, float32(1), "fading in")
	r.tt.Elapse(200 * time.Millisecond)
	assert.Equal(t, float32(1), p.ChromeOpacity())
}

// TestPlayerChromeStaysWhilePaused: nothing counts while the video is paused (the
// phone's rule, 03 section 5a): the chrome is up however long it waits.
func TestPlayerChromeStaysWhilePaused(t *testing.T) {
	t.Parallel()
	r := openFast(t, tvtest.NewPlayerServices(t), env.RoutePlayer, false)
	r.settle()
	r.tt.Elapse(10 * chromeIdle)
	assert.True(t, r.player().Chrome())
}

// TestPlayerChromeButtonWakesAndIsTaken: a button while the chrome is hidden only
// brings it back (arrow, Select, Menu: no move, no press, the screen stays), and
// the next one acts.
func TestPlayerChromeButtonWakesAndIsTaken(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name string
		key  keys.Key
	}{
		{"right", keys.KeyRight},
		{"up", keys.KeyUp},
		{"down", keys.KeyDown},
		{"left", keys.KeyLeft},
		{"select", keys.KeyEnter},
		{"menu", keys.KeyEscape},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			r := playingPlayer(t, true)
			p := r.player()
			pb := r.playback()
			depth := r.sh.Stack().Depth()
			r.tt.Elapse(chromeIdle + tvtest.Frame)
			require.False(t, p.Chrome())

			assert.True(t, r.press(tc.key), "the button is used")
			assert.True(t, p.Chrome(), "and the chrome is back")
			assert.Equal(t, "p1", r.key(), "the focus did not move")
			assert.Zero(t, pb.Calls["Toggle"], "nothing was pressed")
			assert.Equal(t, depth, r.sh.Stack().Depth(), "the screen was not left")
			assert.Equal(t, env.RoutePlayer, r.sh.Stack().Screen())
			assert.Zero(t, pb.Calls["Leave"])
		})
	}
}

// TestPlayerChromeSecondButtonActs: with the chrome back the next button does
// what it does: an arrow moves the focus, Select presses, Menu goes back.
func TestPlayerChromeSecondButtonActs(t *testing.T) {
	t.Parallel()
	r := playingPlayer(t, true)
	pb := r.playback()
	r.tt.Elapse(chromeIdle + tvtest.Frame)
	r.press(keys.KeyRight)
	require.True(t, r.player().Chrome())
	require.True(t, r.press(keys.KeyRight))
	assert.Equal(t, "p2", r.key(), "the focus memory is kept: p1, then Right")
	r.tt.Elapse(chromeIdle + tvtest.Frame)
	assert.False(t, r.player().Chrome())
	r.press(keys.KeyLeft) // wakes
	require.True(t, r.press(keys.KeyLeft))
	require.Equal(t, "p1", r.key())
	require.True(t, r.press(keys.KeyEnter))
	assert.Equal(t, 1, pb.Calls["Toggle"], "Select on p1 pauses")
	r.tt.Elapse(chromeIdle + tvtest.Frame)
	assert.True(t, r.player().Chrome(), "paused: it stays")
	require.True(t, r.press(keys.KeyEscape))
	assert.Equal(t, env.RouteHome, r.sh.Stack().Screen(), "Menu with the chrome up goes back")
}

// TestPlayerChromePlayPauseWakesAndActs: Play/Pause is not taken by the first
// press: it brings the chrome back and toggles, as the key does on the remote of
// jPlayer TV.
func TestPlayerChromePlayPauseWakesAndActs(t *testing.T) {
	t.Parallel()
	r := playingPlayer(t, true)
	pb := r.playback()
	r.tt.Elapse(chromeIdle + tvtest.Frame)
	require.False(t, r.player().Chrome())
	assert.True(t, r.press(keys.KeySpace))
	assert.True(t, r.player().Chrome())
	assert.Equal(t, 1, pb.Calls["Toggle"])
	assert.False(t, pb.PlayingV)
}

// TestPlayerChromeButtonsRestartTheCount: a button while the chrome is up starts
// the 3 s again.
func TestPlayerChromeButtonsRestartTheCount(t *testing.T) {
	t.Parallel()
	r := playingPlayer(t, true)
	p := r.player()
	r.tt.Elapse(2500 * time.Millisecond)
	r.press(keys.KeyRight)
	r.tt.Elapse(2500 * time.Millisecond)
	assert.True(t, p.Chrome(), "2.5 s after the button")
	r.tt.Elapse(600 * time.Millisecond)
	assert.False(t, p.Chrome(), "3 s after it")

	r.press(keys.KeyDown) // wakes
	r.tt.Elapse(2900 * time.Millisecond)
	assert.True(t, p.Chrome(), "the count starts at the wake")
	r.tt.Elapse(200 * time.Millisecond)
	assert.False(t, p.Chrome())
}

// TestPlayerChromePendingScrubKeepsIt: while arrows keep a scrub waiting (one every
// 600 ms, never the 700 ms of quiet it needs) the chrome stays however long that
// is, and the 3 s begin when the seek is applied.
func TestPlayerChromePendingScrubKeepsIt(t *testing.T) {
	t.Parallel()
	r := playingPlayer(t, true)
	p := r.player()
	pb := r.playback()
	require.True(t, r.press(keys.KeyUp))
	require.Equal(t, "ps", r.key())
	for range 10 { // 6 s of arrows
		require.True(t, r.press(keys.KeyRight))
		r.tt.Elapse(600*time.Millisecond - tvtest.Frame)
		assert.True(t, p.Chrome())
	}
	assert.Zero(t, pb.Calls["Seek"], "the scrub is still pending")
	r.tt.Elapse(scrubIdle) // the seek comes 700 ms after the last arrow, 584 ms ago + this
	assert.Equal(t, 1, pb.Calls["Seek"], "applied")
	assert.True(t, p.Chrome())
	r.tt.Elapse(2300 * time.Millisecond) // 3.58 s after the last arrow, 2.88 s after the seek
	assert.True(t, p.Chrome(), "the 3 s count from the seek")
	r.tt.Elapse(300 * time.Millisecond)
	assert.False(t, p.Chrome())
}

// TestPlayerChromeScrubWhenHiddenDoesNotSeek: the first arrow after the chrome
// hid scrubs nothing.
func TestPlayerChromeScrubWhenHiddenDoesNotSeek(t *testing.T) {
	t.Parallel()
	r := playingPlayer(t, true)
	pb := r.playback()
	require.True(t, r.press(keys.KeyUp))
	require.Equal(t, "ps", r.key())
	r.tt.Elapse(chromeIdle + tvtest.Frame)
	require.False(t, r.player().Chrome())
	r.press(keys.KeyRight)
	r.tt.Elapse(2 * scrubIdle)
	assert.Zero(t, pb.Calls["Seek"])
	assert.True(t, r.player().Chrome())
}

// TestPlayerChromePanelHoldsIt: the panel is never hidden under, and the 3 s begin
// when it closes.
func TestPlayerChromePanelHoldsIt(t *testing.T) {
	t.Parallel()
	r := playingPlayer(t, true)
	p := r.player()
	require.True(t, r.press(keys.KeyRight))
	require.True(t, r.press(keys.KeyEnter))
	r.settle()
	require.True(t, p.PanelOpen())
	r.tt.Elapse(3 * chromeIdle)
	assert.True(t, p.Chrome(), "up behind the panel")
	require.True(t, r.press(keys.KeyEscape))
	require.False(t, p.PanelOpen())
	r.tt.Elapse(chromeIdle - 200*time.Millisecond)
	assert.True(t, p.Chrome())
	r.tt.Elapse(400 * time.Millisecond)
	assert.False(t, p.Chrome())
}

// TestPlayerChromeStateShowsIt: a pause shows the chrome that was hidden and
// stops the count; playing again starts it.
func TestPlayerChromeStateShowsIt(t *testing.T) {
	t.Parallel()
	r := playingPlayer(t, true)
	p := r.player()
	pb := r.playback()
	r.tt.Elapse(chromeIdle + tvtest.Frame)
	require.False(t, p.Chrome())
	pb.PlayingV, pb.StateV = false, player.StatePaused
	pb.Publish(player.ChangeState)
	r.tt.Elapse(tvtest.Frame)
	assert.True(t, p.Chrome(), "paused from elsewhere: shown")
	r.tt.Elapse(2 * chromeIdle)
	assert.True(t, p.Chrome())
	pb.PlayingV, pb.StateV = true, player.StatePlaying
	pb.Publish(player.ChangeState)
	r.tt.Elapse(chromeIdle + 2*tvtest.Frame)
	assert.False(t, p.Chrome(), "playing: the count runs")
}

// TestPlayerChromeLoadingWaits: a video still loading keeps the chrome up, and it
// hides 3 s after it plays.
func TestPlayerChromeLoadingWaits(t *testing.T) {
	t.Parallel()
	svc := tvtest.NewPlayerServices(t)
	pb := svc.PB.(*tvtest.Playback)
	pb.PlayingV, pb.StateV = true, player.StateLoading
	r := openFast(t, svc, env.RoutePlayer, false)
	r.settle()
	r.tt.Elapse(2 * chromeIdle)
	assert.True(t, r.player().Chrome(), "loading")
	pb.StateV = player.StatePlaying
	pb.Publish(player.ChangeState)
	r.tt.Elapse(chromeIdle + 2*tvtest.Frame)
	assert.False(t, r.player().Chrome())
}

// TestPlayerChromeIsHushedWhileHidden: a screen reader reads nothing of the hidden
// chrome, is told when it goes and when it returns, and reads it again after.
func TestPlayerChromeIsHushedWhileHidden(t *testing.T) {
	t.Parallel()
	r := playingPlayer(t, true)
	assert.True(t, hasLabel(r.tt.Semantics(), "Tạm dừng"))
	changes := r.tt.SemanticsChanges(platform.ChangeLayout)
	r.tt.Elapse(chromeIdle + 200*time.Millisecond)
	assert.Greater(t, r.tt.SemanticsChanges(platform.ChangeLayout), changes, "told it went")
	sem := r.tt.Semantics()
	for _, l := range []string{"Tạm dừng", "Tuỳ chọn", "Stink and Dutty", "Vị trí phát", "04 · Em Yêu Người Ta"} {
		assert.Falsef(t, hasLabel(sem, l), "%q is read while hidden", l)
	}
	changes = r.tt.SemanticsChanges(platform.ChangeLayout)
	r.press(keys.KeyDown)
	r.tt.Elapse(200 * time.Millisecond) // layout changes are told at most every 100 ms
	assert.Greater(t, r.tt.SemanticsChanges(platform.ChangeLayout), changes, "told it is back")
	assert.True(t, hasLabel(r.tt.Semantics(), "Tạm dừng"))
}

// TestPlayerChromeUnmountTakesTheHookAway: leaving the screen takes the hook away: the
// home that follows has its arrows, and a player opened again has the hook once.
func TestPlayerChromeUnmountTakesTheHookAway(t *testing.T) {
	t.Parallel()
	r := playingPlayer(t, true)
	r.tt.Elapse(chromeIdle + tvtest.Frame)
	r.press(keys.KeyEscape) // wakes
	require.True(t, r.press(keys.KeyEscape))
	require.Equal(t, env.RouteHome, r.sh.Stack().Screen())
	r.tt.Elapse(3 * chromeIdle) // no timer of the old screen runs on
	first := r.key()
	r.press(keys.KeyDown)
	r.press(keys.KeyRight)
	assert.NotEqual(t, first, r.key(), "the home takes its arrows")
}

// TestPlayerChromeDoesNotAllocate: the steady frames with the chrome up and with it
// hidden, a tick of the player in each, and a frame of the fade allocate nothing.
func TestPlayerChromeDoesNotAllocate(t *testing.T) { //nolint:paralleltest // AllocsPerRun needs the process to itself
	skipUnderRace(t)
	r := playingPlayer(t, false)
	pb := r.playback()
	pos := 72.0
	tick := func() {
		if pos += .25; pos >= 80 {
			pos = 72
		}
		pb.Tick(pos)
		r.tt.Elapse(16 * time.Millisecond)
	}
	for range 4 * 8 * 2 { // every second's text is made and shaped once
		tick()
	}
	assert.Zero(t, testing.AllocsPerRun(50, tick), "a tick with the chrome up")
	assert.Zero(t, testing.AllocsPerRun(50, func() { r.tt.Elapse(16 * time.Millisecond) }), "a steady frame with it up")

	r.tt.Elapse(chromeIdle)
	r.tt.Elapse(time.Second)
	require.False(t, r.player().Chrome())
	for range 4 * 8 * 2 {
		tick()
	}
	assert.Zero(t, testing.AllocsPerRun(50, tick), "a tick with the chrome hidden")
	assert.Zero(t, testing.AllocsPerRun(50, func() { r.tt.Elapse(16 * time.Millisecond) }), "a steady frame with it hidden")
}

// TestPlayerChromeKeyDoesNotAllocate: a button that only restarts the count (the
// chrome is up) allocates nothing of its own: Left and Right between the buttons,
// and a button that wakes the hidden chrome.
func TestPlayerChromeKeyDoesNotAllocate(t *testing.T) { //nolint:paralleltest // AllocsPerRun needs the process to itself
	skipUnderRace(t)
	r := playingPlayer(t, true)
	right := true
	key := func() {
		if right {
			r.tt.Key(keys.KeyRight, 0)
		} else {
			r.tt.Key(keys.KeyLeft, 0)
		}
		right = !right
		r.tt.Elapse(tvtest.Frame)
	}
	for range 20 {
		key()
	}
	base := testing.AllocsPerRun(50, func() { r.tt.Elapse(tvtest.Frame) })
	assert.Zero(t, base, "a frame")
	t.Logf("allocations of an arrow with the chrome up: %v", testing.AllocsPerRun(50, key))
}
