//go:build !tvos

// End-to-end flows of the playback (P4.7): the real shell and screens over
// the real services, the mock API through the script transport, the fake
// engine and the virtual clock of the Tester, on both phones. The shape
// (a flow that waits on the services between steps of virtual time, a gate
// that holds a request) is adapted from jplayer-mobile/internal/ui/views/
// flow_videos_test.go and flow_test.go at 49de3c2.

package app

import (
	"context"
	"encoding/json"
	"net/http"
	"os"
	"runtime"
	"sync/atomic"
	"testing"
	"time"

	"dancefitvn-app/internal/download"
	"dancefitvn-app/internal/media"
	"dancefitvn-app/internal/media/fake"
	"dancefitvn-app/internal/model"
	"dancefitvn-app/internal/player"
	"dancefitvn-app/internal/sys"
	"dancefitvn-app/internal/ui/icon"
	"dancefitvn-app/internal/ui/kit"
	"dancefitvn-app/internal/ui/kit/kittest"
	"dancefitvn-app/internal/ui/mobile/env"
	"dancefitvn-app/internal/ui/mobile/screens"

	"github.com/gdeskdev/desk/foundation/core/geom"
	"github.com/gdeskdev/desk/foundation/platform"
	"github.com/gdeskdev/desk/framework/ui"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ====================================================================
// ============================ Contracts =============================
// ====================================================================

// playFlow is a flow over a signed-in phone whose player runs on the fake
// engine (its length is the one the test asks for), with a recording
// keep-awake and the lifecycle listener at hand.
type playFlow struct {
	*flow
	engine atomic.Pointer[fake.Engine]
	awake  *fakeAwake
	life   func(platform.LifecycleState)
}

// ====================================================================
// ============================= Creation =============================
// ====================================================================

// newPlayFlow returns a flow of a signed-in coach whose engine plays media
// videoDur seconds long (the mock's files are 12 s, the lessons' data say
// 220 s).
func newPlayFlow(t *testing.T, ph kittest.Phone, videoDur float64) *playFlow {
	t.Helper()

	return newPlayFlowOn(t, ph, videoDur, nil, "", nil, wireKnobs{})
}

// newPlayFlowOn is newPlayFlow on the phone's vault and folders (nil and ""
// are new ones, with a session seeded), with more changes to the services'
// dependencies and the knobs of a developer's build.
func newPlayFlowOn(t *testing.T, ph kittest.Phone, videoDur float64, secrets *memSecrets, dir string,
	more func(d *mobileDeps), knobs wireKnobs) *playFlow {
	t.Helper()
	pf := &playFlow{awake: &fakeAwake{}}
	if secrets == nil {
		secrets = &memSecrets{vals: map[string][]byte{}}
		dir = t.TempDir()
		seedSession(t, secrets, dir, sys.BiometryNone)
	}
	pf.flow = newFlow(t, ph, flowOpts{secrets: secrets, dir: dir, knobs: knobs, edit: func(d *mobileDeps) {
		d.NewMedia = func(cfg media.Config) (platformMedia, error) {
			e := fake.New(cfg, fake.Options{VideoDuration: videoDur, AudioDuration: videoDur})
			pf.engine.Store(e)

			return platformMedia{name: "fake", eng: e, sess: &fake.Session{}}, nil
		}
		d.Awake = pf.awake
		d.OnLifecycle = func(fn func(platform.LifecycleState)) func() {
			pf.life = fn

			return func() { pf.life = nil }
		}
		if more != nil {
			more(d)
		}
	}})

	return pf
}

// ====================================================================
// ========================== Main functions ==========================
// ====================================================================

// TestFlowPlayVideoControls follows a user through 05 -> 06 over the real
// wire (03 sections 3, 5, 5a): the lesson resumes where the course says,
// the controls hide 3 s after the last touch, +10 and -10 move by ten
// seconds, the speed sheet changes the engine's rate, the mirror flips the
// picture, the A-B loop seeks to A and wraps at B, and Back pauses.
func TestFlowPlayVideoControls(t *testing.T) {
	t.Parallel()

	kittest.ForEachPhone(t, func(t *testing.T, ph kittest.Phone) {
		pf := newPlayFlow(t, ph, 220)
		pb := pf.startLesson3()
		v, ok := pf.sh.Nav().Screen().(*screens.Video)
		require.True(t, ok)

		// 03 section 3: the resume position of the course, from the stream
		loads := pf.loads()
		require.Len(t, loads, 1)
		assert.Contains(t, loads[0].Src.URL, "/hls/l_3503/master.m3u8")
		assert.InDelta(t, 72, loads[0].Load.Start, 0)
		assert.Equal(t, 1, pf.st.calledN("lessons.stream"))
		assert.Equal(t, []bool{false, true}, pf.awake.told(), "the screen is kept on")
		assert.Equal(t, "l_3503", pb.Lesson().ID)
		assert.InDelta(t, 72, pb.Pos(), 1)

		// 03 section 5a: hidden at 3000 ms of the last touch, not at 2999
		pf.showControls(v)
		before := pf.eng().Position()
		pf.tap("seek:10", 1)
		assert.InDelta(t, before+10, pf.eng().Position(), 0.3, "+10")
		pf.tt.Elapse(2999 * time.Millisecond)
		assert.True(t, v.Shown(), "2999 ms after the touch")
		pf.tt.Elapse(time.Millisecond)
		assert.False(t, v.Shown(), "3000 ms: hidden")
		assert.True(t, pb.Playing(), "the video plays on")

		// -10
		pf.showControls(v)
		before = pf.eng().Position()
		pf.tap("seek:-10", 1)
		assert.InDelta(t, before-10, pf.eng().Position(), 0.3, "-10")

		// the rate sheet and the mirror (03 section 5)
		pf.showControls(v)
		pf.tap("sheet:options", 1)
		pf.tt.Elapse(300 * time.Millisecond)
		require.True(t, pf.sh.Nav().Sheet().IsOpen())
		wasMirror := pb.Mirror()
		pf.tap("speed:1.25", 1)
		assert.InDelta(t, 1.25, pf.eng().Rate(), 0)
		assert.InDelta(t, 1.25, pb.Rate(), 0)
		pf.tap("mirror", 1) // the sheet's row: the column has no mirror button (K38)
		assert.Equal(t, !wasMirror, pb.Mirror())
		view, ok := widgetNth(pf.sh, kit.NameVideo, 1).(*kit.VideoView)
		require.True(t, ok)
		assert.Equal(t, !wasMirror, view.Mirror, "the picture is flipped")
		require.NoError(t, pf.tt.Click("Xong"))
		pf.tt.Elapse(300 * time.Millisecond)
		assert.False(t, pf.sh.Nav().Sheet().IsOpen())
		pos := pf.eng().Position()
		pf.tt.Elapse(4 * time.Second)
		assert.InDelta(t, pos+5, pf.eng().Position(), 0.5, "4 s at 1.25x")

		// the A-B loop, from the sheet (K38): begins at the position (no seek), 30 s long,
		// wraps at B (checked at the 250 ms tick)
		pf.showControls(v)
		pf.tap("sheet:options", 1)
		pf.tt.Elapse(300 * time.Millisecond)
		pf.tap("loop", 1)
		require.NoError(t, pf.tt.Click("Xong"))
		pf.tt.Elapse(300 * time.Millisecond)
		require.True(t, pb.Loop())
		a, b := pb.A(), pb.B()
		require.Less(t, a, b)
		assert.InDelta(t, 30, b-a, 0.01, "30 s from where it was turned on")
		assert.InDelta(t, a, pf.eng().Position(), 1, "A is the position it was turned on at")
		pf.tt.Elapse(time.Duration((b-a)/1.25*float64(time.Second)) + 2*time.Second)
		assert.GreaterOrEqual(t, pf.seeksTo(a), 1, "back to A at B")
		assert.GreaterOrEqual(t, pf.eng().Position(), a-0.5)
		assert.Less(t, pf.eng().Position(), b+0.5, "it never plays past B")

		// Back: the video pauses, the screen is let go, the detail shows
		require.True(t, pf.back())
		pf.until(env.RouteDetail)
		assert.Equal(t, player.StatePaused, pb.State())
		told := pf.awake.told()
		assert.False(t, told[len(told)-1])
	})
}

// TestFlowPlayerSavesProgress checks the saves of the lesson over the wire
// (03 section 3, 05 section 3): one at every 10 s of playing (3 in 35 s),
// one at the pause, none while paused, one when the screen is left with the
// position of the engine in its body.
func TestFlowPlayerSavesProgress(t *testing.T) {
	t.Parallel()

	kittest.ForEachPhone(t, func(t *testing.T, ph kittest.Phone) {
		pf := newPlayFlow(t, ph, 220)
		pb := pf.startLesson3()
		v, ok := pf.sh.Nav().Screen().(*screens.Video)
		require.True(t, ok)
		base := pf.st.calledN("lessons.progress")

		pf.play(35 * time.Second)
		assert.Equal(t, 3, pf.saves()-base, "10 s, 20 s, 30 s")

		pf.showControls(v)
		pf.tap("togglePlay", 1)
		require.Equal(t, player.StatePaused, pb.State())
		pf.drive("the save of the pause", func() bool { return pf.saves()-base >= 4 })
		pf.play(30 * time.Second)
		assert.Equal(t, 4, pf.saves()-base, "nothing is saved while paused")

		pf.showControls(v)
		pf.tap("togglePlay", 1)
		require.Equal(t, player.StatePlaying, pb.State())
		// the first tick after a long pause is past the interval since the
		// last save: the new position is saved at once
		pf.drive("the save of the resume", func() bool { return pf.saves()-base >= 5 })
		pf.tt.Elapse(2 * time.Second)
		pos := pf.eng().Position()
		require.True(t, pf.back())
		pf.drive("the save of the leave", func() bool { return pf.saves()-base >= 6 })
		bodies := pf.st.bodiesOf("lessons.progress")
		var last model.ProgressRequest
		require.NoError(t, json.Unmarshal(bodies[len(bodies)-1], &last))
		assert.InDelta(t, pos, float64(last.PositionSec), 1, "the position of the leave")
		assert.Equal(t, 220, last.DurationSec, "the engine's length")
	})
}

// TestFlowPlayToTheEnd plays lesson 3 on the mock's 12 s media (the course
// says 220 s, resume 72: past the end, so it starts at 0), to its end: 06
// gives way to 10, which counts 5 to 0 and plays lesson 4; the lesson ends
// past 90 % so it was saved as watched.
func TestFlowPlayToTheEnd(t *testing.T) {
	t.Parallel()

	kittest.ForEachPhone(t, func(t *testing.T, ph kittest.Phone) {
		t.Run("countdown plays the next lesson", func(t *testing.T) {
			pf := newPlayFlow(t, ph, 12)
			pb := pf.startLesson3()
			assert.InDelta(t, 12, pb.Dur(), 0, "the engine's length replaces the API's")
			assert.InDelta(t, 0, pb.Pos(), 1, "resume 72 is past 12 s: it starts at 0")
			assert.Equal(t, 1, pf.seeksTo(0), "the one seek to 0")

			pf.tt.Elapse(14 * time.Second) // 12 s at .75 is 16 s
			assert.Equal(t, env.RouteVideo, pf.route())
			pf.until(env.RouteDone)
			pf.shows("Xong bài 03!")
			pf.shows("TIẾP THEO · TỰ PHÁT SAU 5 GIÂY")
			pf.shows("Về khoá học")
			pf.shows("Đã xem 3/10 bài")
			assert.Equal(t, player.StateEnded, pb.State())
			assert.True(t, pf.w.WatchedLocal("l_3503"), "past 90 % of the engine's length")

			// the last save is the end: 12 of 12 s
			var got model.ProgressRequest
			pf.await("a save with the position of the end", func() bool {
				bodies := pf.st.bodiesOf("lessons.progress")

				return len(bodies) > 0 && json.Unmarshal(bodies[len(bodies)-1], &got) == nil && got.PositionSec >= 11
			})
			assert.Equal(t, 12, got.DurationSec)

			pf.tt.Elapse(4 * time.Second)
			assert.Equal(t, env.RouteDone, pf.route(), "4 s of 5")
			pf.tt.Elapse(time.Second)
			pf.until(env.RouteVideo)
			assert.Equal(t, 4, pb.Lesson().No, "the next lesson")
			pf.drive("lesson 4 plays", func() bool { return pb.State() == player.StatePlaying })
			assert.InDelta(t, 0, pb.Pos(), 1, "from its start")
			pf.shows("Bài 04 / 10")
		})
		t.Run("to the course shows lesson 3 watched", func(t *testing.T) {
			pf := newPlayFlow(t, ph, 12)
			pf.startLesson3()
			pf.tt.Elapse(14 * time.Second)
			pf.until(env.RouteDone)
			pf.tapName("toDetail:set35")
			pf.until(env.RouteDetail)
			pf.drive("the lessons", func() bool { return widgetNth(pf.sh, "playVideo:set35-5", 1) != nil })
			pf.settleFor(2)
			assert.Equal(t, 3, pf.watchedRows(), "lessons 1 and 2 of the data, and 3 watched in this run")
		})
	})
}

// TestFlowAudioAndBackground checks 04 section 2.5: a sound goes on when
// the app goes to the background and when 08 is left, a video pauses and
// saves, and the screen is let go.
func TestFlowAudioAndBackground(t *testing.T) {
	t.Parallel()

	kittest.ForEachPhone(t, func(t *testing.T, ph kittest.Phone) {
		t.Run("audio plays on", func(t *testing.T) {
			pf := newPlayFlow(t, ph, 220)
			pf.openDetail()
			pf.sh.PlayAudio(demoCourseID, "l_3504", 4)
			pf.until(env.RouteAudio)
			pb := pf.sh.Env().Playback()
			pf.drive("the sound plays", func() bool { return pb.State() == player.StatePlaying })
			assert.Equal(t, player.KindAudio, pb.Kind())
			assert.Contains(t, pf.loads()[0].Src.URL, "/audio/l_3504.mp3")
			assert.True(t, pf.loads()[0].Src.AudioOnly)
			assert.Equal(t, []bool{false}, pf.awake.told(), "no screen kept on for a sound")

			// leaving 08 does not stop it
			require.True(t, pf.back())
			pf.until(env.RouteDetail)
			pos := pb.Pos()
			pf.tt.Elapse(4 * time.Second)
			assert.Equal(t, player.StatePlaying, pb.State())
			assert.Greater(t, pb.Pos(), pos+2, "08 is gone, the sound goes on")

			// and the background does not either
			require.NotNil(t, pf.life)
			pf.life(platform.LifecycleBackground)
			pf.q.drain()
			assert.Equal(t, player.StatePlaying, pb.State())
			assert.Equal(t, []bool{true}, pf.backgrounds())
			pos = pf.eng().Position()
			pf.tt.Elapse(4 * time.Second)
			assert.Greater(t, pf.eng().Position(), pos+2, "the engine keeps the time in the background")
			pf.life(platform.LifecycleActive)
			assert.Equal(t, player.StatePlaying, pb.State())
		})
		t.Run("video pauses and saves", func(t *testing.T) {
			pf := newPlayFlow(t, ph, 220)
			pb := pf.startLesson3()
			pf.tt.Elapse(2 * time.Second)
			saves := pf.saves()
			pf.life(platform.LifecycleBackground)
			pf.q.drain()
			assert.Equal(t, player.StatePaused, pb.State())
			assert.Equal(t, []bool{true}, pf.backgrounds())
			pf.drive("the save of the background", func() bool { return pf.saves() > saves })
			told := pf.awake.told()
			assert.False(t, told[len(told)-1], "the screen let go")
			pf.life(platform.LifecycleActive)
			assert.Equal(t, player.StatePaused, pb.State(), "coming back does not play it")
			assert.Equal(t, env.RouteVideo, pf.route())
		})
	})
}

// TestFlowAudioFromOffline plays a downloaded lesson from the offline
// screen, whose file is on the device (03 section 2).
func TestFlowAudioFromOffline(t *testing.T) {
	t.Parallel()

	kittest.ForEachPhone(t, func(t *testing.T, ph kittest.Phone) {
		pf := newPlayFlow(t, ph, 220)
		pf.homeLoaded()
		pf.tab(1)
		pf.coursesLoaded() // the list is cached before the network goes
		pf.tab(0)
		pf.openDetail()
		c, ok := pf.sh.Env().CachedCourse(demoCourseID)
		require.True(t, ok)
		m := pf.w.downloads()
		require.NotNil(t, m)
		require.NoError(t, m.Start(c.Lessons[2], download.CourseInfo{ID: demoCourseID}))
		var path string
		pf.drive("the file", func() bool {
			p, done := m.Path("l_3503")
			path = p

			return done
		})
		require.True(t, pf.back())
		pf.homeLoaded()

		pf.w.tr.setOffline(true)
		pf.tab(1)
		pf.drive("offline", func() bool { return !pf.w.Online() })
		pf.coursesLoaded()
		require.NoError(t, pf.tt.Click("go.detail:set35"))
		pf.until(env.RouteOffline)
		pf.sh.PlayVideo(demoCourseID, "l_3505", 5)
		pf.shows("Video bài 05 cần có mạng để xem")
		pf.sh.OffTap(demoCourseID, "l_3505", 5)
		audioCalls := pf.st.calledN("lessons.audio") // the download's
		pf.sh.OffTap(demoCourseID, "l_3503", 3)
		pf.until(env.RouteAudio)
		pb := pf.sh.Env().Playback()
		pf.drive("the file plays", func() bool { return pb.State() == player.StatePlaying })
		loads := pf.loads()
		assert.Equal(t, path, loads[len(loads)-1].Src.URL, "the file on the device, no request")
		assert.True(t, loads[len(loads)-1].Src.AudioOnly)
		assert.Equal(t, audioCalls, pf.st.calledN("lessons.audio"))
		require.True(t, pf.back())
		pf.tt.Elapse(3 * time.Second)
		assert.Equal(t, player.StatePlaying, pb.State(), "the sound goes on after 08")
		_, statErr := os.Stat(path)
		require.NoError(t, statErr)
	})
}

// TestFlowProgressOffline checks the saves of a lesson played while the
// network is gone: the video stops and screen 13 takes its place (03
// section 2, `$P:810-813`), its position waits in the queue, is sent in one
// batch when the network comes back, and a logout sends what is left once
// and empties the queue (05 section 3, Q-P4.6a-1).
func TestFlowProgressOffline(t *testing.T) {
	t.Parallel()

	kittest.ForEachPhone(t, func(t *testing.T, ph kittest.Phone) {
		pf := newPlayFlow(t, ph, 220)
		pf.startLesson3()
		q := pf.w.queue.Load()
		require.NotNil(t, q)

		pf.w.tr.setOffline(true)
		pf.tt.Elapse(12 * time.Second)
		pf.drive("a save queued", func() bool { return q.Len() >= 1 })
		assert.Zero(t, pf.st.calledN("progress.batch"), "offline: nothing is sent")
		pf.until(env.RouteOffline)
		assert.False(t, pf.sh.Env().Playback().Playing(), "the video stops offline")

		pf.w.tr.setOffline(false)
		pf.clk.Advance(probeSteps[0])
		pf.drive("the network back", func() bool { return pf.w.Online() })
		pf.drive("the flush", func() bool { return q.Len() == 0 && pf.st.calledN("progress.batch") == 1 })
		var batch model.BatchRequest
		bodies := pf.st.bodiesOf("progress.batch")
		require.Len(t, bodies, 1)
		require.NoError(t, json.Unmarshal(bodies[0], &batch))
		require.NotEmpty(t, batch.Items)
		assert.Equal(t, "l_3503", batch.Items[0].LessonID)
		assert.Greater(t, batch.Items[0].PositionSec, 72)

		// what is queued when the user logs out is sent first, once; online
		// again, 13 gave its place back to the detail, which plays the video
		pf.until(env.RouteDetail)
		pf.drive("the lessons", func() bool { return widgetNth(pf.sh, "playVideo:set35-3", 1) != nil })
		pf.tapName("playVideo:set35-3")
		pf.until(env.RouteVideo)
		pf.drive("the video plays", func() bool { return pf.sh.Env().Playback().Playing() })
		pf.w.tr.setOffline(true)
		pf.tt.Elapse(12 * time.Second)
		pf.drive("a save queued again", func() bool { return q.Len() >= 1 })
		pf.w.tr.setOffline(false)
		require.NoError(t, logoutNow(pf.rig))
		pf.q.drain()
		assert.Zero(t, q.Len(), "the queue is empty after the logout")
		assert.Equal(t, 2, pf.st.calledN("progress.batch"), "sent before the token went")
	})
}

// TestFlowExpiredStreamRetriesOnce checks 03 section 9 for a stream whose
// address stops working while it plays: the engine's network error asks for
// a new address once and plays on from the position; a second request that
// fails is an error toast, not a loop.
func TestFlowExpiredStreamRetriesOnce(t *testing.T) {
	t.Parallel()

	kittest.ForEachPhone(t, func(t *testing.T, ph kittest.Phone) {
		pf := newPlayFlow(t, ph, 220)
		pb := pf.startLesson3()
		pf.tt.Elapse(8 * time.Second)
		pos := pf.eng().Position()
		require.Equal(t, 1, pf.st.calledN("lessons.stream"))

		pf.eng().Fail(media.ErrNetwork)
		pf.drive("the new address plays", func() bool {
			return pf.st.calledN("lessons.stream") == 2 && pb.State() == player.StatePlaying
		})
		loads := pf.loads()
		require.Len(t, loads, 2)
		assert.InDelta(t, pos, loads[1].Load.Start, 0.5, "from where it was")
		assert.Equal(t, env.RouteVideo, pf.route())
		pf.gone("Không phát được bài này. Thử lại sau")

		// the retry's address is refused: one answer, an error
		pf.st.failNext("lessons.stream", 1, http.StatusInternalServerError, "errors/server_error.json")
		pf.eng().Fail(media.ErrNetwork)
		pf.drive("the error", func() bool { return pb.State() == player.StateError })
		pf.shows("Không phát được bài này. Thử lại sau")
		assert.Equal(t, 3, pf.st.calledN("lessons.stream"), "one request per failure, no loop")
	})
}

// TestFlowFailedPlaylistShowsPlayAndRetries checks a lesson whose stream answers
// but whose playlist is gone (the engine cannot open it, twice): the toast, the
// button of screen 06 drawn as PLAY (a load under way draws PAUSE, and no tick
// follows the failure to draw it again), and a tap on it fetching the stream again
// and playing once the playlist is back.
func TestFlowFailedPlaylistShowsPlayAndRetries(t *testing.T) {
	t.Parallel()

	kittest.ForEachPhone(t, func(t *testing.T, ph kittest.Phone) {
		var eng atomic.Pointer[fake.Engine]
		var gone atomic.Bool
		gone.Store(true)
		pf := newPlayFlowOn(t, ph, 220, nil, "", func(d *mobileDeps) {
			d.NewMedia = func(cfg media.Config) (platformMedia, error) {
				e := fake.New(cfg, fake.Options{VideoDuration: 220, Fail: map[string]error{missingPlaylist: media.ErrSource}})
				eng.Store(e)

				return platformMedia{name: "fake", eng: &playlistGone{Engine: e, gone: &gone}, sess: &fake.Session{}}, nil
			}
		}, wireKnobs{})
		pf.engine.Store(eng.Load())
		pf.openDetail()
		pf.tapName("playVideo:set35-3")
		pf.until(env.RouteVideo)
		pb := pf.sh.Env().Playback()
		require.NotNil(t, pb)
		pf.drive("the error", func() bool { return pb.State() == player.StateError })
		assert.Equal(t, 2, pf.st.calledN("lessons.stream"), "one retry with a new address, then the end")
		pf.shows("Không phát được bài này. Thử lại sau")
		assert.False(t, pb.Playing())
		toggle := widgetNth(pf.sh, "togglePlay", 1).(*kit.DiscButton)
		assert.Equal(t, icon.Play, toggle.Icon)
		assert.Equal(t, icon.Play, drawnGlyph(t, toggle), "PLAY is what the person sees")
		assert.Equal(t, env.RouteVideo, pf.route(), "the screen stays")

		gone.Store(false) // the playlist is back
		pf.tapName("togglePlay")
		pf.drive("the lesson plays", func() bool { return pb.State() == player.StatePlaying })
		assert.Equal(t, 3, pf.st.calledN("lessons.stream"), "the tap fetched the stream again")
		assert.Equal(t, icon.Pause, drawnGlyph(t, widgetNth(pf.sh, "togglePlay", 1)))
	})
}

// TestFlowNotEnrolledGoesToTheDetail checks 03 section 9: a stream the
// server refuses with not_enrolled leaves 06 for the course's detail.
func TestFlowNotEnrolledGoesToTheDetail(t *testing.T) {
	t.Parallel()

	kittest.ForEachPhone(t, func(t *testing.T, ph kittest.Phone) {
		pf := newPlayFlow(t, ph, 220)
		pf.openDetail()
		pf.st.failNext("lessons.stream", 1, http.StatusForbidden, "errors/not_enrolled.json")
		pf.tapName("playVideo:set35-5")
		pf.drive("the player's error", func() bool {
			pb := pf.sh.Env().Playback()

			return pb != nil && pb.State() == player.StateError
		})
		pf.until(env.RouteDetail)
		assert.Equal(t, demoCourseID, pf.sh.Nav().Current().Params)
		assert.Zero(t, pf.eng().Position())
		pf.gone("Không phát được bài này. Thử lại sau")
	})
}

// ====================================================================
// ========================= Helper Functions =========================
// ====================================================================

// eng is the engine the services made.
func (pf *playFlow) eng() *fake.Engine {
	e := pf.engine.Load()
	require.NotNil(pf.t, e, "the engine is not made yet")

	return e
}

// openDetail waits for Home, opens the detail of Set 35 and waits until its
// lessons are on screen.
func (pf *playFlow) openDetail() {
	pf.t.Helper()
	pf.homeLoaded()
	pf.sh.Detail(demoCourseID)
	pf.until(env.RouteDetail)
	pf.drive("the lessons", func() bool {
		_, ok := pf.sh.Env().CachedCourse(demoCourseID)

		return ok && widgetNth(pf.sh, "playVideo:set35-5", 1) != nil
	})
}

// startLesson3 taps lesson 3's card of the detail (the lesson to go on
// with, resume 72) and waits until it plays on 06.
func (pf *playFlow) startLesson3() env.Playback {
	pf.t.Helper()
	pf.openDetail()
	pf.tapName("playVideo:set35-3")
	pf.until(env.RouteVideo)
	pb := pf.sh.Env().Playback()
	require.NotNil(pf.t, pb)
	pf.drive("the video plays", func() bool { return pb.State() == player.StatePlaying })

	return pb
}

// tapName taps the first widget named name, as a finger does.
func (pf *playFlow) tapName(name string) {
	pf.t.Helper()
	pf.tap(name, 1)
}

// tap taps the middle of the n-th visible widget named name.
func (pf *playFlow) tap(name string, n int) {
	pf.t.Helper()
	w := widgetNth(pf.sh, name, n)
	require.NotNil(pf.t, w, "no visible widget %q #%d", name, n)
	pf.tt.ClickAt(w.Node().WindowFrame().Center())
	pf.q.drain()
}

// showControls shows the controls of 06 when they are hidden: a tap on the
// picture, away from the buttons.
func (pf *playFlow) showControls(v *screens.Video) {
	pf.t.Helper()
	if !v.Shown() {
		pf.tt.ClickAt(geom.Pt(150, 400))
	}
	require.True(pf.t, v.Shown())
}

// play lets d of virtual time pass one second at a time, as the app lives it:
// the results of the saves come back to the UI thread between the steps,
// and the next step waits until the saves under way have returned (the
// tracker sends one at a time and holds the others back).
func (pf *playFlow) play(d time.Duration) {
	pf.t.Helper()
	for left := d; left > 0; left -= time.Second {
		pf.tt.Elapse(min(left, time.Second))
		pf.quiet()
	}
}

// await runs the queued work of the UI thread until cond holds, without
// letting any virtual time pass: for what a test must see before a timer of
// the app (the countdown of 10) goes on.
func (pf *playFlow) await(what string, cond func() bool) {
	pf.t.Helper()
	end := time.Now().Add(driveLimit)
	for time.Now().Before(end) {
		pf.q.drain()
		if cond() {
			return
		}
		runtime.Gosched()
	}
	pf.t.Fatalf("%s did not happen in %v; log: %v", what, driveLimit, pf.log.all())
}

// quiet waits until every save the tracker started has returned from the
// transport, and its result has run on the UI thread.
func (pf *playFlow) quiet() {
	pf.t.Helper()
	end := time.Now().Add(driveLimit)
	for pf.st.doneN("lessons.progress") < pf.saves() {
		require.True(pf.t, time.Now().Before(end), "a save did not return")
		runtime.Gosched()
	}
	for range 4 { // the result is posted right after the return
		runtime.Gosched()
		pf.q.drain()
	}
}

// saves is how many saves of the position reached the transport.
func (pf *playFlow) saves() int { return pf.st.calledN("lessons.progress") }

// loads returns the sources the engine was asked to load.
func (pf *playFlow) loads() []fake.Call {
	var out []fake.Call
	for _, c := range pf.eng().Calls() {
		if c.Op == fake.OpLoad {
			out = append(out, c)
		}
	}

	return out
}

// seeksTo counts the seeks of the engine to position v.
func (pf *playFlow) seeksTo(v float64) int {
	n := 0
	for _, c := range pf.eng().Calls() {
		if c.Op == fake.OpSeek && c.Value > v-0.5 && c.Value < v+0.5 {
			n++
		}
	}

	return n
}

// backgrounds returns the values of the engine's SetBackground calls.
func (pf *playFlow) backgrounds() []bool {
	var out []bool
	for _, c := range pf.eng().Calls() {
		if c.Op == fake.OpSetBackground {
			out = append(out, c.Flag)
		}
	}

	return out
}

// watchedRows counts the lines of the screen that end in "· Đã xem": the
// rows of the lessons shown as watched.
func (pf *playFlow) watchedRows() int {
	n := 0
	for _, s := range pf.tt.Shown() {
		if len(s) > len("· Đã xem") && s[len(s)-len("· Đã xem"):] == "· Đã xem" {
			n++
		}
	}

	return n
}

// widgetNth returns the n-th (from 1) visible widget named name in tree
// order, nil when there is none.
func widgetNth(root ui.Widget, name string, n int) ui.Widget {
	var found ui.Widget
	count := 0
	var walk func(w ui.Widget)
	walk = func(w ui.Widget) {
		b := w.Node()
		if b.Hidden || found != nil {
			return
		}
		if b.Name == name {
			count++
			if count == n {
				found = w

				return
			}
		}
		for c := range b.Children() {
			walk(c)
		}
	}
	walk(root)

	return found
}

// missingPlaylist is the address the engine cannot open (the playlist answers 404).
const missingPlaylist = "http://playlist.invalid/master.m3u8"

// playlistGone is an engine whose loads fail while gone is set, as an HLS playlist
// that answers 404 does: it opens a source that is not there.
type playlistGone struct {
	*fake.Engine
	gone *atomic.Bool
}

// Load opens the source, or the missing one while the playlist is gone.
func (p *playlistGone) Load(ctx context.Context, src media.Source, o media.LoadOptions) (uint64, error) {
	if p.gone.Load() {
		src.URL = missingPlaylist
	}

	return p.Engine.Load(ctx, src, o)
}

// drawnGlyph is the symbol the button's icon child draws.
func drawnGlyph(t *testing.T, w ui.Widget) icon.Name {
	t.Helper()
	for ch := range w.Node().Children() {
		if ic, ok := ch.(*kit.Icon); ok {
			return ic.Glyph
		}
	}
	require.Fail(t, "the button has no icon child")

	return icon.NameUnknown
}
