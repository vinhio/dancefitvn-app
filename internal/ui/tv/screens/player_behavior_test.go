//go:build tvos

package screens_test

import (
	"testing"
	"time"

	"dancefitvn-app/internal/player"
	"dancefitvn-app/internal/ui/tv/env"
	"dancefitvn-app/internal/ui/tv/nav"
	"dancefitvn-app/internal/ui/tv/tvtest"

	"github.com/gdeskdev/desk/foundation/core/geom"
	"github.com/gdeskdev/desk/foundation/core/keys"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ====================================================================
// ========================== Main functions ==========================
// ====================================================================

// TestPlayerIsOpenedByHomeWithP1Focused: `h1` plays the lesson to continue and opens
// TV07 with the focus on `p1`, the first control of the screen.
func TestPlayerIsOpenedByHomeWithP1Focused(t *testing.T) {
	t.Parallel()
	svc := tvtest.NewPlayerServices(t)
	svc.PB = &tvtest.Playback{} // nothing loaded: the home's lesson is started
	r := openFast(t, svc, env.RouteHome, false)
	r.settle()
	require.Equal(t, "h1", r.key())
	require.True(t, r.press(keys.KeyEnter))
	r.settle()
	assert.Equal(t, 1, svc.PB.(*tvtest.Playback).Calls["PlayVideo"])
	assert.Equal(t, env.RoutePlayer, r.sh.Stack().Screen())
	assert.Equal(t, 1, r.sh.Stack().Depth())
	assert.Equal(t, "p1", r.key())
	r.player()
}

// TestPlayerButtons: `p1` toggles the playback and shows Phát or Tạm dừng as the
// player says; the Play/Pause key does the same, on this screen only.
func TestPlayerButtons(t *testing.T) {
	t.Parallel()
	svc := tvtest.NewPlayerServices(t)
	r := inPlayer(t, svc, true)
	pb := r.playback()
	require.Equal(t, "p1", r.key())
	lay := r.layout()
	assert.Equal(t, "Phát", find(t, lay, "p1").Text)

	require.True(t, r.press(keys.KeyEnter))
	assert.Equal(t, 1, pb.Calls["Toggle"])
	pb.Publish(player.ChangeState)
	r.tt.Elapse(tvtest.Frame)
	assert.Equal(t, "Tạm dừng", find(t, r.layout(), "p1").Text)

	assert.True(t, r.press(keys.KeyMediaPlayPause))
	assert.Equal(t, 2, pb.Calls["Toggle"])
	pb.Publish(player.ChangeState)
	r.tt.Elapse(tvtest.Frame)
	assert.Equal(t, "Phát", find(t, r.layout(), "p1").Text)
	assert.True(t, r.press(keys.KeySpace))
	assert.Equal(t, 3, pb.Calls["Toggle"])
	pb.Publish(player.ChangeState)
	r.tt.Elapse(tvtest.Frame)
	assert.Equal(t, "Tạm dừng", find(t, r.layout(), "p1").Text)
	assert.Empty(t, r.tt.Leaks())

	require.True(t, r.press(keys.KeyEscape)) // back to the home: the key is the system's
	assert.False(t, r.press(keys.KeyMediaPlayPause))
}

// TestPlayerFollowsTheClock: the 4 Hz tick moves the clock, the bar and the lit
// beat; the options show as status lines and hide again.
func TestPlayerFollowsTheClock(t *testing.T) {
	t.Parallel()
	svc := tvtest.NewPlayerServices(t)
	r := inPlayer(t, svc, false)
	pb := r.playback()
	p := r.player()

	pb.BeatV, pb.BeatIdx = true, 3
	pb.Publish(player.ChangeOptions)
	pb.Tick(100)
	r.tt.Elapse(tvtest.Frame)
	lay := r.layout()
	assert.Equal(t, "01:40", find(t, lay, "tv.player.pos").Text)
	assert.Equal(t, 3, p.Beats().Lit())
	e := find(t, lay, "div.scrub")
	assert.InDelta(t, 520, e.W, .1)

	pb.RateV, pb.MirrorV, pb.LoopV = 1.25, true, true
	pb.Publish(player.ChangeRate)
	pb.Publish(player.ChangeOptions)
	r.tt.Elapse(tvtest.Frame)
	lay = r.layout()
	assert.Equal(t, "Tốc độ 1.25x", find(t, lay, "tv.player.speed").Text)
	assert.Equal(t, "Lật gương", find(t, lay, "tv.player.mirror").Text)
	assert.Equal(t, "Lặp 01:13–01:41", find(t, lay, "tv.player.loop").Text)
	y := func(k string) float64 { return find(t, lay, k).Y }
	assert.Less(t, y("tv.player.speed"), y("tv.player.mirror"), "the lines are stacked in order")
	assert.Less(t, y("tv.player.mirror"), y("tv.player.loop"))

	pb.RateV, pb.MirrorV, pb.LoopV, pb.BeatV = 1, false, false, false
	pb.Publish(player.ChangeRate)
	pb.Publish(player.ChangeOptions)
	r.tt.Elapse(tvtest.Frame)
	lay = r.layout()
	for _, k := range []string{"tv.player.speed", "tv.player.mirror", "tv.player.loop", "div.beats"} {
		_, ok := lay.Find(k)
		assert.False(t, ok, k+" is hidden")
	}
	assert.Equal(t, 760.0, find(t, lay, "tv.player.pos").Y, "without the bar the clock starts the block")
}

// TestPlayerPanel: `p2` opens the panel on `o0`; a speed asks the player and the
// check follows its answer; the toggles ask for the option and show Bật or Tắt;
// `oc` closes it with the focus on `p2`. The arrows never leave the panel.
func TestPlayerPanel(t *testing.T) {
	t.Parallel()
	svc := tvtest.NewPlayerServices(t)
	r := inPlayer(t, svc, true)
	pb := r.playback()
	p := r.player()
	require.True(t, r.press(keys.KeyRight))
	require.Equal(t, "p2", r.key())
	require.True(t, r.press(keys.KeyEnter))
	r.settle()
	require.True(t, r.sh.Stack().Panel())
	assert.True(t, p.PanelOpen())
	assert.Equal(t, "o0", r.key())

	// the speeds: o2 (1x) has the check
	checks := func() (out []string) {
		lay := r.layout()
		for _, k := range []string{"o0", "o1", "o2", "o3"} {
			if find(t, lay, k).Text != "" {
				out = append(out, k)
			}
		}

		return out
	}
	_ = checks
	require.True(t, r.press(keys.KeyDown))
	assert.Equal(t, "o1", r.key())
	require.True(t, r.press(keys.KeyEnter))
	assert.Equal(t, 1, pb.Calls["SetRate"])
	assert.InDelta(t, .75, pb.RateV, 0)
	pb.Publish(player.ChangeRate)
	r.tt.Elapse(tvtest.Frame)
	lay := r.layout()
	assert.Equal(t, "0.75x", find(t, lay, "o1").Text)
	assert.Equal(t, "Tốc độ 0.75x", find(t, lay, "tv.player.speed").Text, "the status line follows")

	// the toggles
	for range 3 {
		require.True(t, r.press(keys.KeyDown))
	}
	assert.Equal(t, "ot0", r.key())
	require.True(t, r.press(keys.KeyEnter))
	assert.Equal(t, 1, pb.Calls["SetMirror"])
	require.True(t, r.press(keys.KeyDown))
	require.True(t, r.press(keys.KeyEnter))
	assert.Equal(t, 1, pb.Calls["SetLoop"])
	require.True(t, r.press(keys.KeyDown))
	require.True(t, r.press(keys.KeyEnter))
	assert.Equal(t, 1, pb.Calls["SetBeat"])
	assert.True(t, pb.MirrorV && pb.LoopV && pb.BeatV)
	pb.Publish(player.ChangeOptions)
	r.tt.Elapse(tvtest.Frame)
	lay = r.layout()
	for _, k := range []string{"ot0", "ot1", "ot2"} {
		assert.Equal(t, "Bật", lastLine(find(t, lay, k).Text), k)
	}

	// close
	require.True(t, r.press(keys.KeyDown))
	assert.Equal(t, "oc", r.key())
	require.True(t, r.press(keys.KeyEnter))
	r.settle()
	assert.False(t, r.sh.Stack().Panel())
	assert.False(t, p.PanelOpen())
	assert.Equal(t, "p2", r.key())
	assert.Equal(t, env.RoutePlayer, r.sh.Stack().Screen())
	assert.Empty(t, r.tt.Leaks())
}

// lastLine returns the text after the last newline.
func lastLine(s string) string {
	for i := len(s) - 1; i >= 0; i-- {
		if s[i] == '\n' {
			return s[i+1:]
		}
	}

	return s
}

// TestPlayerPanelKeepsTheArrowsAndMenuClosesIt: with the panel open the arrows stay
// in it; Menu closes it with the focus on `p2` and only the next Menu goes back
// (index.html:731).
func TestPlayerPanelKeepsTheArrowsAndMenuClosesIt(t *testing.T) {
	t.Parallel()
	r := inPlayer(t, tvtest.NewPlayerServices(t), true)
	require.True(t, r.press(keys.KeyRight))
	require.True(t, r.press(keys.KeyEnter))
	r.settle()
	in := map[string]bool{"o0": true, "o1": true, "o2": true, "o3": true, "ot0": true, "ot1": true, "ot2": true, "oc": true}
	for _, k := range []keys.Key{keys.KeyLeft, keys.KeyRight, keys.KeyUp, keys.KeyUp, keys.KeyDown, keys.KeyDown, keys.KeyDown, keys.KeyDown,
		keys.KeyDown, keys.KeyDown, keys.KeyDown, keys.KeyDown, keys.KeyDown, keys.KeyLeft, keys.KeyRight} {
		r.press(k)
		assert.True(t, in[r.key()], "the focus stays in the panel: %q", r.key())
	}
	assert.True(t, r.press(keys.KeyEscape))
	assert.False(t, r.sh.Stack().Panel())
	assert.Equal(t, "p2", r.key())
	assert.Equal(t, env.RoutePlayer, r.sh.Stack().Screen())
	assert.False(t, r.player().PanelOpen())
	assert.True(t, r.press(keys.KeyEscape))
	assert.Equal(t, env.RouteHome, r.sh.Stack().Screen())
	assert.Equal(t, 1, r.playback().Calls["Leave"], "leaving the screen tells the player")
}

// TestPlayerNextRowPlaysTheLesson: a next-lesson row plays that lesson and takes
// the screen's place (no push), with the focus on `p1`.
func TestPlayerNextRowPlaysTheLesson(t *testing.T) {
	t.Parallel()
	svc := tvtest.NewPlayerServices(t)
	r := inPlayer(t, svc, true)
	pb := r.playback()
	depth := r.sh.Stack().Depth()
	require.True(t, r.press(keys.KeyRight))
	require.True(t, r.press(keys.KeyRight))
	require.Equal(t, "pn3", r.key(), "the row level with the buttons is the nearest at the right")
	require.True(t, r.press(keys.KeyUp))
	require.True(t, r.press(keys.KeyUp))
	require.Equal(t, "pn1", r.key())
	require.True(t, r.press(keys.KeyEnter))
	r.settle()
	assert.Equal(t, 1, pb.Calls["PlayVideo"])
	assert.Equal(t, "l_3505", pb.LessonV.ID)
	assert.Equal(t, depth, r.sh.Stack().Depth(), "replaced, not pushed")
	assert.Equal(t, env.RoutePlayer, r.sh.Stack().Screen())
	assert.Equal(t, "p1", r.key())
	assert.Zero(t, pb.Calls["Leave"], "a lesson taking this screen's place is not leaving it")
	assert.True(t, pb.PlayingV)
}

// TestPlayerEndsInUpNext: a video that plays to its end gives way to TV10, which
// takes the player's place; a sound does not.
func TestPlayerEndsInUpNext(t *testing.T) {
	t.Parallel()
	r := inPlayer(t, tvtest.NewPlayerServices(t), false)
	depth := r.sh.Stack().Depth()
	r.playback().End()
	r.settle()
	assert.Equal(t, env.RouteUpNext, r.sh.Stack().Screen())
	assert.Equal(t, depth, r.sh.Stack().Depth())
	assert.Empty(t, r.tt.Leaks())

	svc := tvtest.NewPlayerServices(t)
	svc.PB.(*tvtest.Playback).KindV = player.KindAudio
	r = inPlayer(t, svc, false)
	r.playback().End()
	r.settle()
	assert.Equal(t, env.RoutePlayer, r.sh.Stack().Screen(), "only a video gives way")
}

// TestPlayerOfflinePausesAndShowsTheError: the network going away pauses and puts
// TV11 over the player (index.html:812); Back returns to it.
func TestPlayerOfflinePausesAndShowsTheError(t *testing.T) {
	t.Parallel()
	svc := tvtest.NewPlayerServices(t)
	r := inPlayer(t, svc, false)
	pb := r.playback()
	pb.Start()
	svc.SetOnline(false)
	r.settle()
	assert.False(t, pb.PlayingV)
	assert.Equal(t, env.RouteError, r.sh.Stack().Screen())
	assert.Equal(t, "e1", r.key())

	svc.SetOnline(true)
	require.True(t, r.press(keys.KeyEscape))
	r.settle()
	assert.Equal(t, env.RoutePlayer, r.sh.Stack().Screen())
	assert.Empty(t, r.tt.Leaks())
}

// TestPlayerPlaneFollowsTheScreen: the plane is at 656,0, 608 by 1080 while TV07
// shows, hidden when it goes; until the first frame the stage covers the hole, and
// once the picture is on the plane the hole is transparent, with #0B0B0C at both
// sides of it (K5, R-P7-3).
func TestPlayerPlaneFollowsTheScreen(t *testing.T) {
	t.Parallel()
	svc := tvtest.NewPlayerServices(t)
	r := openFast(t, svc, env.RouteHome, false)
	r.settle()
	pb := r.playback()
	tvtest.AttachPlane(t, r.tt, pb)
	r.sh.Go(nav.Route{Screen: env.RoutePlayer}, false)
	r.settle()
	us := r.tt.Underlays()
	require.Len(t, us, 1)
	assert.Equal(t, geom.XYWH(656, 0, 608, 1080), us[0].Frame)
	assert.False(t, us[0].Hidden)
	assert.Equal(t, geom.XYWH(656, 0, 608, 1080), r.player().Hole())

	px := func(x, y int) (a, rr, g, b uint8) {
		img := r.pixels()
		o := y*img.Stride + x*4

		return img.Pix[o+3], img.Pix[o], img.Pix[o+1], img.Pix[o+2]
	}
	a, _, _, _ := px(960, 540)
	assert.Equal(t, uint8(255), a, "the K5 stage covers the hole until the first frame")

	pb.Picture = true
	pb.Publish(player.ChangeState)
	r.tt.Elapse(tvtest.Frame)
	a, _, _, _ = px(960, 540)
	assert.Zero(t, a, "the picture is on the plane: nothing is painted over it")
	a, rr, g, b := px(300, 50)
	assert.Equal(t, [4]uint8{255, 0x0b, 0x0b, 0x0c}, [4]uint8{a, rr, g, b}, "left of the hole")
	a, rr, g, b = px(1600, 1070)
	assert.Equal(t, [4]uint8{255, 0x0b, 0x0b, 0x0c}, [4]uint8{a, rr, g, b}, "right of the hole")

	require.True(t, r.press(keys.KeyEscape))
	r.settle()
	us = r.tt.Underlays()
	require.Len(t, us, 1)
	assert.True(t, us[0].Hidden, "hidden after Back")
}

// TestPlayerWithoutAPlayerHasP1: a Services with no player gives a bare TV07 that
// Menu still leaves.
func TestPlayerWithoutAPlayerHasP1(t *testing.T) {
	t.Parallel()
	svc := tvtest.NewPlayerServices(t)
	svc.PB = nil
	r := inPlayer(t, svc, false)
	assert.Equal(t, "p1", r.key())
	assert.True(t, r.press(keys.KeyEscape))
	assert.Equal(t, env.RouteHome, r.sh.Stack().Screen())
}

// TestPlayerReadsTheCourseWhenNobodyDid: the next lessons come from a read of the
// course when no screen read it before.
func TestPlayerReadsTheCourseWhenNobodyDid(t *testing.T) {
	t.Parallel()
	svc := tvtest.NewPlayerServices(t)
	r := openAt(t, svc, nav.Route{Screen: env.RoutePlayer}, false)
	r.settle()
	p := r.player()
	assert.Len(t, p.Rows(), 4)
	_, ok := r.sh.Env().CachedCourse("set-35")
	assert.True(t, ok, "the course is kept for the screens that follow")
	assert.Equal(t, "p1", r.key())

	// the last lessons of the course have fewer than four
	svc2 := tvtest.NewPlayerServices(t)
	svc2.PB = tvtest.NewLessonPlayback(svc2.CourseData, 9)
	r2 := openAt(t, svc2, nav.Route{Screen: env.RoutePlayer}, false)
	r2.settle()
	assert.Len(t, r2.player().Rows(), 1)
}

// TestPlayerTickDoesNotAllocate: a tick of the player while it plays (the clock,
// the bar, the beat) and its paint allocate nothing, once the seconds of the clock
// have been shown.
func TestPlayerTickDoesNotAllocate(t *testing.T) { //nolint:paralleltest // AllocsPerRun needs the process to itself
	skipUnderRace(t)
	svc := playerState(t)
	r := inPlayer(t, svc, false)
	pb := r.playback()
	pos := 0.0
	step := func() {
		pos += .25
		if pos >= 220 {
			pos = 0
		}
		pb.BeatIdx = int(pos*2) % 8
		pb.Tick(pos)
		r.tt.Elapse(16 * time.Millisecond)
	}
	for range 4 * 221 { // every second's text is made and shaped once
		step()
	}
	assert.Zero(t, testing.AllocsPerRun(100, step), "a tick")
	r.tt.Elapse(time.Second)
	assert.Zero(t, testing.AllocsPerRun(50, func() { r.tt.Elapse(16 * time.Millisecond) }), "a steady frame")
}
