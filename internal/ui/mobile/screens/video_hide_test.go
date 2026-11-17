//go:build !tvos

package screens_test

import (
	"testing"
	"time"

	"dancefitvn-app/internal/player"
	"dancefitvn-app/internal/ui/kit/kittest"
	"dancefitvn-app/internal/ui/mobile/env"

	"github.com/gdeskdev/desk/foundation/core/geom"
	"github.com/gdeskdev/desk/foundation/core/keys"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestVideoControlsHideAtThreeSeconds checks 03 section 5a: the controls are shown
// at 2999 ms after the last touch, hide at 3000 and are gone at 3250 (the fade of
// 250 ms), with the picture still playing; both phones.
func TestVideoControlsHideAtThreeSeconds(t *testing.T) {
	t.Parallel()

	kittest.ForEachPhone(t, func(t *testing.T, ph kittest.Phone) {
		r, pb, v := atVideo(t, ph, false)
		restart(r, pb)
		require.True(t, v.Shown())
		r.Tester.Elapse(2999 * time.Millisecond)
		assert.True(t, v.Shown(), "2999 ms")
		assert.InDelta(t, 1, v.Opacity(), 0)
		r.Tester.Elapse(time.Millisecond)
		assert.False(t, v.Shown(), "3000 ms: the controls hide")
		r.Tester.Elapse(125 * time.Millisecond)
		assert.Greater(t, v.Opacity(), float32(0), "fading")
		assert.Less(t, v.Opacity(), float32(1), "fading")
		r.Tester.Elapse(125 * time.Millisecond)
		assert.Zero(t, v.Opacity(), "3250 ms: gone")
		assert.True(t, pb.Playing(), "the video goes on")
	})
}

// TestVideoStageTapToggles checks `stageTap`: a tap on the picture while the video
// plays hides the controls at once, a second shows them and starts the 3 s again;
// while it is paused a tap shows them.
func TestVideoStageTapToggles(t *testing.T) {
	t.Parallel()

	kittest.ForEachPhone(t, func(t *testing.T, ph kittest.Phone) {
		r, pb, v := atVideo(t, ph, false)
		restart(r, pb)
		r.Tester.ClickAt(stagePoint())
		assert.False(t, v.Shown(), "a tap hides the shown controls")
		r.Tester.Elapse(300 * time.Millisecond)
		assert.Zero(t, v.Opacity())
		r.Tester.ClickAt(stagePoint())
		assert.True(t, v.Shown(), "a tap shows the hidden controls")
		r.Tester.Elapse(300 * time.Millisecond)
		assert.InDelta(t, 1, v.Opacity(), 0)
		r.Tester.Elapse(2600 * time.Millisecond)
		assert.True(t, v.Shown(), "2900 ms after the tap")
		r.Tester.Elapse(100 * time.Millisecond)
		assert.False(t, v.Shown(), "the count restarted at the tap")
		assert.Zero(t, pb.Count("Toggle"), "a tap on the picture does not pause it")

		pb.Pause()
		r.Settle()
		assert.True(t, v.Shown(), "a pause shows the controls")
		r.Tester.ClickAt(stagePoint())
		assert.True(t, v.Shown(), "paused, a tap shows and never hides")
		r.Tester.Elapse(10 * time.Second)
		assert.True(t, v.Shown(), "nothing counts while paused")
	})
}

// TestVideoHiddenButtonsTakeNoTouch checks that a press on the spot of a button of
// the hidden controls is a tap on the picture: the controls show, and the button
// is not pressed (`pointer-events:none`).
func TestVideoHiddenButtonsTakeNoTouch(t *testing.T) {
	t.Parallel()

	kittest.ForEachPhone(t, func(t *testing.T, ph kittest.Phone) {
		for _, name := range []string{"togglePlay", "seek:10", "mirror", "nextLes", "back"} {
			r, pb, v := atVideo(t, ph, false)
			restart(r, pb)
			r.Tester.Elapse(ms3s + 300*time.Millisecond)
			require.False(t, v.Shown(), name)
			pb.ClearCalls()
			tap(r, named(r.Shell, name, 1))
			assert.True(t, v.Shown(), name+": the controls show")
			assert.Empty(t, pb.Calls(), name+": and the button was not pressed")
			assert.Equal(t, env.RouteVideo, r.Route(), name)
		}
	})
}

// TestVideoButtonsResetTheCount checks that a button of the shown controls starts
// the 3 s again, and the scrub bar does not (index.html:587, :627): it seeks, and
// the controls still hide at 3 s from the touch before.
func TestVideoButtonsResetTheCount(t *testing.T) {
	t.Parallel()

	kittest.ForEachPhone(t, func(t *testing.T, ph kittest.Phone) {
		r, pb, v := atVideo(t, ph, false)
		restart(r, pb)
		r.Tester.Elapse(2 * time.Second)
		tap(r, named(r.Shell, "seek:10", 1))
		r.Tester.Elapse(2999 * time.Millisecond)
		assert.True(t, v.Shown(), "2999 ms after the button")
		r.Tester.Elapse(time.Millisecond)
		assert.False(t, v.Shown(), "3000 ms after the button")

		restart(r, pb)
		r.Tester.Elapse(2 * time.Second)
		sc := named(r.Shell, "scrub", 1)
		require.NotNil(t, sc)
		f := sc.Node().WindowFrame()
		pb.ClearCalls()
		r.Tester.ClickAt(geom.Pt(f.X+f.W/4, f.Y+f.H/2))
		assert.Equal(t, []string{"Seek:55"}, pb.Calls(), "a quarter of 220 s")
		r.Tester.Elapse(999 * time.Millisecond)
		assert.True(t, v.Shown(), "2999 ms after the first touch")
		r.Tester.Elapse(time.Millisecond)
		assert.False(t, v.Shown(), "the scrub did not restart the count")
	})
}

// TestVideoDeadZonesSwallowTaps checks F-10: while the controls show, a tap on the
// top band, between the side buttons, or on the panel where no button is does
// nothing; it does not reach the picture, so it does not hide the controls.
func TestVideoDeadZonesSwallowTaps(t *testing.T) {
	t.Parallel()

	kittest.ForEachPhone(t, func(t *testing.T, ph kittest.Phone) {
		r, pb, v := atVideo(t, ph, false)
		restart(r, pb)
		pb.ClearCalls()
		side := named(r.Shell, "mirror", 1).Node().WindowFrame()
		panel := named(r.Shell, "div.d", 1).Node().WindowFrame()
		for name, p := range map[string]geom.Point{
			"top band":             geom.Pt(ph.Size.W/2, 20),
			"between the top row":  geom.Pt(ph.Size.W/2-100, 110),
			"between side buttons": geom.Pt(side.X+side.W/2, side.Y-8),
			"panel title":          geom.Pt(panel.X+50, panel.Y+10),
			"panel gap":            geom.Pt(30, panel.Y-5),
		} {
			r.Tester.ClickAt(p)
			assert.True(t, v.Shown(), name)
			assert.Empty(t, pb.Calls(), name)
		}
	})
}

// TestVideoPauseAndLoadShowTheControls checks that a pause and a load show them, and
// that Loading to Playing starts the 3 s (Q-P4-5): a video slower to load than the
// wait does not leave the controls hidden, nor hide them 3 s after the touch of before.
func TestVideoPauseAndLoadShowTheControls(t *testing.T) {
	t.Parallel()

	kittest.ForEachPhone(t, func(t *testing.T, ph kittest.Phone) {
		r, pb, v := atVideo(t, ph, false)
		restart(r, pb)
		r.Tester.Elapse(2900 * time.Millisecond)
		pb.SetState(player.StateLoading)
		r.Tester.Elapse(10 * time.Second)
		assert.True(t, v.Shown(), "a load keeps the controls shown, and nothing hides them")
		pb.SetState(player.StatePlaying)
		r.Tester.Elapse(2999 * time.Millisecond)
		assert.True(t, v.Shown(), "2999 ms after Loading to Playing")
		r.Tester.Elapse(time.Millisecond)
		assert.False(t, v.Shown(), "3000 ms")

		pb.SetState(player.StatePaused)
		r.Settle()
		assert.True(t, v.Shown(), "a pause shows them")
		r.Tester.Elapse(time.Minute)
		assert.True(t, v.Shown())
		pb.SetState(player.StatePlaying)
		r.Tester.Elapse(2999 * time.Millisecond)
		assert.True(t, v.Shown())
		r.Tester.Elapse(time.Millisecond)
		assert.False(t, v.Shown())

		// the play button: pauses, shows, and plays with a new count
		pb.SetState(player.StatePaused)
		r.Settle()
		tap(r, named(r.Shell, "togglePlay", 1))
		assert.Equal(t, player.StatePlaying, pb.State())
		r.Tester.Elapse(2999 * time.Millisecond)
		assert.True(t, v.Shown())
		r.Tester.Elapse(time.Millisecond)
		assert.False(t, v.Shown())
	})
}

// TestVideoSheetShowsTheControls checks that an open sheet shows the controls and
// stops the count, and that closing it (Xong, the cross, the dim, Back) starts it
// again.
func TestVideoSheetShowsTheControls(t *testing.T) {
	t.Parallel()

	kittest.ForEachPhone(t, func(t *testing.T, ph kittest.Phone) {
		for _, closeBy := range []string{"Xong", "Đóng", "dim", "back"} {
			r, pb, v := atVideo(t, ph, false)
			restart(r, pb)
			r.Tester.Elapse(2 * time.Second)
			tap(r, named(r.Shell, optionsOpen, 2)) // the speed button of the side column
			r.Tester.Elapse(300 * time.Millisecond)
			require.True(t, r.Shell.Nav().Sheet().IsOpen(), closeBy)
			r.Tester.Elapse(time.Minute)
			assert.True(t, v.Shown(), closeBy+": shown while the sheet is open")
			switch closeBy {
			case "dim":
				r.Tester.ClickAt(geom.Pt(ph.Size.W/2, 100))
			case "back":
				r.Shell.Back()
			default:
				require.NoError(t, r.Tester.Click(closeBy))
			}
			require.False(t, r.Shell.Nav().Sheet().IsOpen(), closeBy)
			assert.Equal(t, env.RouteVideo, r.Route(), closeBy)
			r.Tester.Elapse(2999 * time.Millisecond)
			assert.True(t, v.Shown(), closeBy+": 2999 ms after the sheet closed")
			r.Tester.Elapse(time.Millisecond)
			assert.False(t, v.Shown(), closeBy+": 3000 ms")
		}
	})
}

// TestVideoNextLessonShowsTheControls checks that another lesson (the countdown's
// auto-next, or the next button) starts with the controls shown and a new count.
func TestVideoNextLessonShowsTheControls(t *testing.T) {
	t.Parallel()

	kittest.ForEachPhone(t, func(t *testing.T, ph kittest.Phone) {
		r, pb, v := atVideo(t, ph, false)
		restart(r, pb)
		r.Tester.ClickAt(stagePoint())
		require.False(t, v.Shown())
		pb.Next()
		r.Settle()
		assert.True(t, v.Shown())
		r.Tester.Elapse(2999 * time.Millisecond)
		assert.True(t, v.Shown())
		r.Tester.Elapse(time.Millisecond)
		assert.False(t, v.Shown())
	})
}

// TestVideoStopsFollowingWhenLeft checks that the screen lets go of the player when
// it is left: its timers and its subscription end (no leak, no hide of a screen gone).
func TestVideoStopsFollowingWhenLeft(t *testing.T) {
	t.Parallel()

	kittest.ForEachPhone(t, func(t *testing.T, ph kittest.Phone) {
		r, pb, v := atVideo(t, ph, false)
		restart(r, pb)
		r.Shell.Back()
		r.Settle()
		assert.False(t, v.PendingHide(), "the 3 s end with the screen")
		pb.Tick(100)
		pb.SetState(player.StatePlaying)
		r.Tester.Elapse(10 * time.Second)
		assert.Empty(t, r.Tester.Leaks())
	})
}

// TestVideoStageSwipeIsNoTap checks that a finger that wanders more than the slop
// is not a tap on the picture, and one inside it is.
func TestVideoStageSwipeIsNoTap(t *testing.T) {
	t.Parallel()

	kittest.ForEachPhone(t, func(t *testing.T, ph kittest.Phone) {
		r, pb, v := atVideo(t, ph, false)
		restart(r, pb)
		from := stagePoint()
		r.Tester.Drag(from, geom.Pt(from.X+60, from.Y))
		assert.True(t, v.Shown(), "a swipe is not a tap")
		r.Tester.Drag(from, geom.Pt(from.X+5, from.Y+4))
		assert.False(t, v.Shown(), "a press that moved 5 DIPs is a tap")
	})
}

// TestVideoBackKeyClosesTheSheetFirst checks the system's Back (the key the Android
// driver sends, and the iOS edge swipe): an open sheet closes first, and the screen
// stays; the next Back leaves the screen and the player is told once.
func TestVideoBackKeyClosesTheSheetFirst(t *testing.T) {
	t.Parallel()

	kittest.ForEachPhone(t, func(t *testing.T, ph kittest.Phone) {
		r, pb, _ := atVideo(t, ph, false)
		tap(r, named(r.Shell, optionsOpen, 1))
		r.Tester.Elapse(300 * time.Millisecond)
		require.True(t, r.Shell.Nav().Sheet().IsOpen())
		assert.True(t, r.Tester.Key(keys.KeyBack, 0))
		assert.False(t, r.Shell.Nav().Sheet().IsOpen())
		assert.Equal(t, env.RouteVideo, r.Route())
		assert.Zero(t, pb.Count("Leave"))
		assert.True(t, r.Tester.Key(keys.KeyBack, 0))
		assert.Equal(t, "detail", r.Route())
		assert.Equal(t, 1, pb.Count("Leave"))
	})
}
