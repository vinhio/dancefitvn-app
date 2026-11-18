//go:build !tvos

package app

import (
	"context"
	"net/http"
	"os"
	"strings"
	"testing"
	"time"

	"dancefitvn-app/internal/api"
	"dancefitvn-app/internal/download"
	"dancefitvn-app/internal/media"
	"dancefitvn-app/internal/media/fake"
	"dancefitvn-app/internal/model"
	"dancefitvn-app/internal/player"
	"dancefitvn-app/internal/progress"
	"dancefitvn-app/internal/store"
	"dancefitvn-app/internal/ui/kit/kittest"
	"dancefitvn-app/internal/ui/mobile/env"

	"github.com/gdeskdev/desk/foundation/platform"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ====================================================================
// ============================ Contracts =============================
// ====================================================================

// demoCourseID is the course of the playback's tests: Set 35, whose
// lesson 3 (l_3503, 220 s) is at 72 s.
const demoCourseID = "set-35"

// playRig is the phone's services with the player on the fake engine,
// bound to a blank Tester whose Elapse drives the engine and the saves,
// the script transport in the way of the requests, a recording keep-awake
// and the lifecycle listener at hand.
type playRig struct {
	*rig
	st    *scriptTransport
	eng   *fake.Engine
	made  int
	awake *fakeAwake
	life  func(platform.LifecycleState)
}

// ====================================================================
// ============================= Creation =============================
// ====================================================================

// newPlayRig returns the services over fresh folders, not signed in;
// edit, when set, changes the deps first.
func newPlayRig(t *testing.T, dir string, edit func(*mobileDeps)) *playRig {
	t.Helper()
	r := newRig(t, nil, dir)
	pr := &playRig{rig: r, st: newScriptTransport(), awake: &fakeAwake{}}
	r.deps.NewMedia = func(cfg media.Config) (platformMedia, error) {
		pr.made++
		pr.eng = fake.New(cfg, fake.Options{VideoDuration: 220, AudioDuration: 220})

		return platformMedia{name: "fake", eng: pr.eng, sess: &fake.Session{}}, nil
	}
	r.deps.Awake = pr.awake
	r.deps.OnLifecycle = func(fn func(platform.LifecycleState)) func() {
		pr.life = fn

		return func() { pr.life = nil }
	}
	if edit != nil {
		edit(&r.deps)
	}
	r.tt = newBlankTester(t)
	r.w = newMobileWire(r.deps, wireKnobs{prepare: func(_ context.Context, w *mobileWire) error {
		pr.st.inner = w.tr.inner
		w.tr.inner = pr.st

		return nil
	}})
	t.Cleanup(r.w.close) // before the Tester's close: the last in, the first out
	r.w.attach(r.tt.View())

	return pr
}

// ====================================================================
// ========================== Main functions ==========================
// ====================================================================

// TestWirePlaysVideoAndSavesProgress plays lesson 3 through the wire: the
// address is the stream's (GET /lessons/{id}/stream), the load starts at
// the lesson's resume position, the screen is kept on, the position is
// saved every 10 s of playing (PUT /lessons/{id}/progress) and when the
// screen of the video is left.
func TestWirePlaysVideoAndSavesProgress(t *testing.T) {
	t.Parallel()

	pr := newPlayRig(t, "", nil)
	pr.signIn()
	list := pr.lessons()
	p := pr.player()
	p.PlayVideo(list[2], list)
	pr.state(player.StatePlaying)
	loads := pr.loads()
	require.Len(t, loads, 1)
	assert.Contains(t, loads[0].Src.URL, "/hls/l_3503/master.m3u8")
	assert.False(t, loads[0].Src.AudioOnly)
	assert.InDelta(t, 72, loads[0].Load.Start, 0, "the course's resume position (Q-P4-13)")
	assert.Equal(t, 1, pr.st.calledN("lessons.stream"))
	assert.Equal(t, []bool{false, true}, pr.awake.told(), "the screen is kept on")

	pr.drive("a save while playing", func() bool { return pr.st.calledN("lessons.progress") >= 1 })
	assert.Greater(t, p.Pos(), 72.0+7, "10 s at the speed .75")
	saves := pr.st.calledN("lessons.progress")
	pr.tt.Elapse(2 * time.Second) // a position the last save did not send
	p.Leave()
	assert.Equal(t, player.StatePaused, p.State(), "a video pauses when left")
	pr.drive("the save of the leave", func() bool { return pr.st.calledN("lessons.progress") > saves })
	assert.Equal(t, []bool{false, true, false}, pr.awake.told())
}

// TestWireResolver checks the addresses of the lessons: the stream's and
// the audio's from the API, a downloaded file from the device, and the
// errors as they come (not_enrolled, offline), whose refused session
// signs out.
func TestWireResolver(t *testing.T) {
	t.Parallel()

	pr := newPlayRig(t, "", nil)
	pr.signIn()
	res := wireResolver{w: pr.w}
	ctx := t.Context()

	u, err := res.VideoURL(ctx, "l_3503")
	require.NoError(t, err)
	assert.True(t, strings.HasPrefix(u, "http://127.0.0.1:"), u)
	assert.Contains(t, u, "/hls/l_3503/master.m3u8")

	u, err = res.AudioURL(ctx, "l_3504")
	require.NoError(t, err)
	assert.Contains(t, u, "/audio/l_3504.mp3", "not downloaded: the API's")

	pr.st.failNext("lessons.stream", 1, http.StatusForbidden, "errors/not_enrolled.json")
	_, err = res.VideoURL(ctx, "l_3503")
	var ae *api.Error
	require.ErrorAs(t, err, &ae)
	assert.Equal(t, "not_enrolled", ae.Code)

	pr.w.tr.setOffline(true)
	_, err = res.AudioURL(ctx, "l_3504")
	require.ErrorIs(t, err, api.ErrOffline)
	pr.w.tr.setOffline(false)
}

// TestWireNotEnrolled checks the player tells the shell the user is not in
// the course when the server refuses the stream (03 section 9, 05B).
func TestWireNotEnrolled(t *testing.T) {
	t.Parallel()

	pr := newPlayRig(t, "", nil)
	pr.signIn()
	list := pr.lessons()
	var got []error
	stop := pr.player().Changes().Subscribe(func(c player.Change) {
		if c.Kind == player.ChangeError {
			got = append(got, c.Err)
		}
	})
	defer stop()
	pr.st.failNext("lessons.stream", 1, http.StatusForbidden, "errors/not_enrolled.json")
	pr.player().PlayVideo(list[3], list)
	pr.state(player.StateError)
	require.Len(t, got, 1)
	require.ErrorIs(t, got[0], player.ErrNotEnrolled)
}

// TestWireAudioFromDevice checks a downloaded lesson plays its file, and
// the API's address when the file has gone (Q-P4.3-2).
func TestWireAudioFromDevice(t *testing.T) {
	t.Parallel()

	pr := newPlayRig(t, "", nil)
	pr.signIn()
	c, err := pr.w.Course(t.Context(), demoCourseID)
	require.NoError(t, err)
	m := pr.w.downloads()
	require.NotNil(t, m)
	require.NoError(t, m.Start(c.Lessons[2], download.CourseInfo{ID: demoCourseID}))
	var path string
	pr.drive("the download", func() bool {
		p, ok := m.Path("l_3503")
		path = p

		return ok
	})
	res := wireResolver{w: pr.w}
	u, err := res.AudioURL(t.Context(), "l_3503")
	require.NoError(t, err)
	assert.Equal(t, path, u, "the file on the device")
	pr.w.local.played("l_3503") // asking for the address is no play: the file is not under suspicion

	list := env.PlayerLessons(c)
	pr.player().PlayAudio(list[2], list)
	pr.state(player.StatePlaying)
	loads := pr.loads()
	require.NotEmpty(t, loads)
	assert.Equal(t, path, loads[len(loads)-1].Src.URL)
	assert.True(t, loads[len(loads)-1].Src.AudioOnly)
	assert.Equal(t, []bool{false}, pr.awake.told(), "no screen kept on for a sound")

	require.NoError(t, os.Remove(path))
	u, err = res.AudioURL(t.Context(), "l_3503")
	require.NoError(t, err)
	assert.Contains(t, u, "/audio/l_3503.mp3", "the file gone: the API's")
}

// TestReconnectFlushesProgressQueue checks the positions saved offline
// are sent (POST /progress/batch) when the network comes back (P3.6
// note), and not while it is away.
func TestReconnectFlushesProgressQueue(t *testing.T) {
	t.Parallel()

	pr := newPlayRig(t, "", nil)
	pr.signIn()
	ctx := t.Context()
	q := pr.w.queue.Load()
	require.NotNil(t, q)
	pr.w.tr.setOffline(true)
	_, err := pr.w.Config(ctx)
	require.ErrorIs(t, err, api.ErrOffline)
	require.NoError(t, q.Add(queueItem("l_3503", 120)))
	pr.q.drain()
	assert.Zero(t, pr.st.calledN("progress.batch"), "offline: nothing sent")

	pr.w.tr.setOffline(false)
	_, err = pr.w.Config(ctx)
	require.NoError(t, err)
	pr.drive("the flush", func() bool { return q.Len() == 0 })
	assert.Equal(t, 1, pr.st.calledN("progress.batch"))
}

// TestForegroundFlushesProgressQueue checks the queue is sent when the app
// comes back to the foreground, only for a signed-in user.
func TestForegroundFlushesProgressQueue(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		signed bool
		sent   int
	}{
		{"signed in", true, 1},
		{"signed out", false, 0},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			pr := newPlayRig(t, "", nil)
			if tc.signed {
				pr.signIn()
			} else {
				_, err := pr.w.Restore(t.Context())
				require.NoError(t, err)
			}
			q := pr.w.queue.Load()
			require.NotNil(t, q)
			require.NoError(t, q.Add(queueItem("l_3503", 120)))
			require.NotNil(t, pr.life)
			pr.life(platform.LifecycleActive)
			pr.drive("the flush ended", func() bool { return !tc.signed || q.Len() == 0 })
			pr.w.bg.Wait()
			assert.Equal(t, tc.sent, pr.st.calledN("progress.batch"))
			assert.Equal(t, 1-tc.sent, q.Len())
		})
	}
}

// TestWireLifecycle checks the background pauses a video and saves its
// position, lets a sound play on, tells the engine both ways, and lets
// the screen go (Q-P4-17, 04 section 2.5).
func TestWireLifecycle(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		kind  player.Kind
		after player.State
	}{
		{"video pauses", player.KindVideo, player.StatePaused},
		{"audio plays on", player.KindAudio, player.StatePlaying},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			pr := newPlayRig(t, "", nil)
			pr.signIn()
			list := pr.lessons()
			p := pr.player()
			if tc.kind == player.KindVideo {
				p.PlayVideo(list[2], list)
			} else {
				p.PlayAudio(list[2], list)
			}
			pr.state(player.StatePlaying)
			pr.tt.Elapse(2 * time.Second)
			saves := pr.st.calledN("lessons.progress")

			pr.life(platform.LifecycleBackground)
			pr.q.drain()
			assert.Equal(t, tc.after, p.State())
			assert.Equal(t, []bool{true}, pr.backgrounds())
			pr.settle("the save of the background", func() bool { return pr.st.calledN("lessons.progress") > saves })
			told := pr.awake.told()
			assert.False(t, told[len(told)-1], "the screen let go")

			pr.life(platform.LifecycleActive)
			assert.Equal(t, []bool{true, false}, pr.backgrounds())
			assert.Equal(t, tc.after, p.State(), "the foreground does not play a paused video")
		})
	}
}

// TestWirePrefs checks the preferences: the defaults before the services
// are made, the stored ones after, a quality cycled saved on disk for
// the next launch, and the Wi-Fi default of the server taken on a first
// launch only.
func TestWirePrefs(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	pr := newPlayRig(t, dir, nil)
	p := pr.w.Prefs()
	assert.True(t, p.AutoNext())
	assert.Equal(t, store.QualityAuto, p.Quality())
	pr.signIn()
	assert.True(t, pr.w.prefsFresh.Load(), "nothing stored yet")
	pr.w.configured(model.AppConfig{Download: model.DownloadConfig{WiFiOnlyDefault: false}})
	assert.False(t, pr.w.prefStore.Load().Get().WiFiOnly, "the server's default on a first launch")
	pr.w.configured(model.AppConfig{Download: model.DownloadConfig{WiFiOnlyDefault: true}})
	assert.False(t, pr.w.prefStore.Load().Get().WiFiOnly, "taken once")

	p.CycleQuality()
	p.CycleQuality()
	assert.Equal(t, store.QualityP720, p.Quality())
	pr.w.close()

	again := newPlayRig(t, dir, nil)
	_, err := again.w.Restore(t.Context())
	require.NoError(t, err)
	again.q.drain()
	assert.Equal(t, store.QualityP720, again.w.Prefs().Quality(), "kept for the next launch")
	assert.False(t, again.w.prefsFresh.Load())
}

// TestWireMakesMediaLate checks the media made late (the iPhone's): none
// before the first frame, made at the turn after it, the quality of the
// preferences put on them; and made at once by what cannot wait.
func TestWireMakesMediaLate(t *testing.T) {
	t.Parallel()

	pr := newPlayRig(t, "", func(d *mobileDeps) { d.MediaLate = true })
	require.NotNil(t, pr.w.pb.late)
	pr.q.drain()
	assert.Zero(t, pr.made, "before the first frame")
	pr.w.pb.firstFrame()
	pr.w.pb.firstFrame()
	assert.Zero(t, pr.made, "made at the next turn, not in the frame")
	pr.q.drain()
	assert.Equal(t, 1, pr.made)
	assert.Same(t, pr.eng, pr.w.pb.sys.eng)

	now := newPlayRig(t, "", func(d *mobileDeps) { d.MediaLate = true })
	require.NotNil(t, now.w.Playback().Video(), "the screen of the video makes them")
	assert.Equal(t, 1, now.made)
}

// TestPlayVideoFlow plays a lesson from its course on the phone with the
// real shell and services: 06 opens on the stack and plays, Back
// returns to the detail, the video pauses and its position is saved.
func TestPlayVideoFlow(t *testing.T) {
	t.Parallel()

	kittest.ForEachPhone(t, func(t *testing.T, ph kittest.Phone) {
		f := newFlow(t, ph, flowOpts{})
		f.until(env.RouteLogin)
		f.signIn("coach@example.invalid", "secret1")
		f.homeLoaded()
		f.sh.Detail(demoCourseID)
		f.until(env.RouteDetail)
		f.drive("the detail's content", func() bool {
			_, ok := f.sh.Env().CachedCourse(demoCourseID)

			return ok
		})
		f.sh.PlayVideo(demoCourseID, "l_3503", 3)
		f.until(env.RouteVideo)
		pb := f.sh.Env().Playback()
		require.NotNil(t, pb)
		f.drive("the video plays", func() bool { return pb.State() == player.StatePlaying })
		assert.Equal(t, "l_3503", pb.Lesson().ID)
		f.drive("a save while playing", func() bool { return f.st.calledN("lessons.progress") >= 1 })
		saves := f.st.calledN("lessons.progress")
		f.tt.Elapse(2 * time.Second) // a position the last save did not send
		require.True(t, f.back())
		f.until(env.RouteDetail)
		assert.Equal(t, player.StatePaused, pb.State())
		f.drive("the save of the leave", func() bool { return f.st.calledN("lessons.progress") > saves })
	})
}

// TestWireSaverUpdateDoesNotAllocate checks the tick of the player through
// the wire's saver (the tracker made) allocates nothing.
func TestWireSaverUpdateDoesNotAllocate(t *testing.T) { //nolint:paralleltest // AllocsPerRun needs the process to itself
	pr := newPlayRig(t, "", nil)
	pr.signIn()
	s := pr.w.pb.saver
	s.Update("l_3503", demoCourseID, 72, 220, false)
	require.NotNil(t, s.t)
	pos := 72.0
	allocs := testing.AllocsPerRun(200, func() {
		pos += 0.25
		s.Update("l_3503", demoCourseID, pos, 220, false)
	})
	assert.Zero(t, allocs)
}

// TestFirstFrameHookDoesNotAllocate checks the hook every frame runs
// allocates nothing after the first.
func TestFirstFrameHookDoesNotAllocate(t *testing.T) { //nolint:paralleltest // AllocsPerRun needs the process to itself
	pr := newPlayRig(t, "", func(d *mobileDeps) { d.MediaLate = true })
	pr.w.pb.firstFrame()
	allocs := testing.AllocsPerRun(200, pr.w.pb.firstFrame)
	assert.Zero(t, allocs)
}

// BenchmarkWireSaverUpdate measures the player's tick through the wire's
// saver.
func BenchmarkWireSaverUpdate(b *testing.B) {
	w := &mobileWire{deps: mobileDeps{Logf: b.Logf}}
	w.ui = newUIClock(func() time.Time { return t0 })
	q, err := progress.NewQueue(store.NewMemory(), wireSender{w: w})
	if err != nil {
		b.Fatal(err)
	}
	w.queue.Store(q)
	w.dispatch = func(fn func()) {}
	s := &wireSaver{w: w}
	s.Update("l_3503", demoCourseID, 72, 220, false)
	pos := 72.0
	b.ReportAllocs()
	for b.Loop() {
		pos += 0.25
		s.Update("l_3503", demoCourseID, pos, 220, false)
	}
}

// TestWireKeepsPausedFrame checks the frame of a video is kept when it
// pauses and when the app goes to the background (Q-P4.4-2), so that
// screen 06 shows it while the engine has none, and let go when another
// lesson loads.
func TestWireKeepsPausedFrame(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		stop func(pr *playRig)
	}{
		{"a pause", func(pr *playRig) { pr.player().Pause() }},
		{"the leave of 06", func(pr *playRig) { pr.player().Leave() }},
		{"the background", func(pr *playRig) { pr.life(platform.LifecycleBackground) }},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			pr := newPlayRig(t, "", nil)
			pr.signIn()
			list := pr.lessons()
			p := pr.player()
			p.PlayVideo(list[2], list)
			pr.state(player.StatePlaying)
			s := p.Video()
			unwatch := s.Watch()
			defer unwatch()
			paint := uint64(0)
			pr.drive("a frame on screen", func() bool {
				paint++
				img, _ := s.Frame(paint)

				return img != nil
			})
			kept, _ := s.Kept()
			assert.Nil(t, kept, "playing: nothing kept")

			tc.stop(pr)
			pr.drive("the frame kept", func() bool {
				img, _ := s.Kept()

				return img != nil
			})

			p.PlayVideo(list[3], list)
			kept, _ = s.Kept()
			assert.Nil(t, kept, "another lesson: let go")
		})
	}
}

// ====================================================================
// ========================= Helper Functions =========================
// ====================================================================

// signIn makes the services and signs a coach in.
func (pr *playRig) signIn() {
	pr.t.Helper()
	ctx := pr.t.Context()
	_, err := pr.w.Restore(ctx)
	require.NoError(pr.t, err)
	require.NoError(pr.t, pr.w.Login(ctx, "coach@example.invalid", "secret1"))
	pr.q.drain()
}

// lessons returns the lessons of Set 35 as the shell gives them to the
// player.
func (pr *playRig) lessons() []player.Lesson {
	pr.t.Helper()
	c, err := pr.w.Course(pr.t.Context(), demoCourseID)
	require.NoError(pr.t, err)

	return env.PlayerLessons(c)
}

// player is the window's player.
func (pr *playRig) player() *player.Player { return pr.w.pb.player }

// state drives until the player is in s.
func (pr *playRig) state(s player.State) {
	pr.t.Helper()
	pr.drive("the player's state", func() bool { return pr.player().State() == s })
}

// loads returns the sources the engine was asked to load.
func (pr *playRig) loads() []fake.Call {
	var out []fake.Call
	for _, c := range pr.eng.Calls() {
		if c.Op == fake.OpLoad {
			out = append(out, c)
		}
	}

	return out
}

// backgrounds returns the values of the engine's SetBackground calls.
func (pr *playRig) backgrounds() []bool {
	var out []bool
	for _, c := range pr.eng.Calls() {
		if c.Op == fake.OpSetBackground {
			out = append(out, c.Flag)
		}
	}

	return out
}

// queueItem is a position saved offline.
func queueItem(id string, pos int) progress.Item {
	return progress.Item{LessonID: id, PositionSec: pos, DurationSec: 220, ClientTime: demoNow}
}

// TestLogoutSendsThenClearsProgressQueue checks Q-P4.6a-1: a logout the user
// asks for sends the queue once with the token still valid (when the network
// is there), a forced sign-out cannot send, and both leave the queue empty so
// that nothing goes out with the next user's token.
func TestLogoutSendsThenClearsProgressQueue(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		offline bool
		forced  bool
		batches int
	}{
		{"online logout flushes first", false, false, 1},
		{"offline logout drops", true, false, 0},
		{"forced sign-out drops", false, true, 0},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			pr := newPlayRig(t, "", nil)
			pr.signIn()
			q := pr.w.queue.Load()
			require.NotNil(t, q)
			require.NoError(t, q.Add(queueItem("l_3503", 120)))
			pr.w.tr.setOffline(tc.offline)

			if tc.forced {
				pr.w.mgr.SignOut(nil)
			} else {
				require.NoError(t, logoutNow(pr.rig))
			}
			pr.q.drain()

			assert.Zero(t, q.Len())
			assert.Equal(t, tc.batches, pr.st.calledN("progress.batch"))
			_, signed := pr.w.mgr.Session()
			assert.False(t, signed)
		})
	}
}
