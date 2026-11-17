//go:build !tvos

// Adapted from jplayer-mobile/internal/app/m9_wire_test.go and
// np_interrupted_test.go at 49de3c2: the system's Now Playing is gDesk's
// fake driver (foundation/platform/fake: NowPlaying, PressMediaCommand),
// run on its UI thread as bridge/system/internal/onui does.

package app

import (
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"dancefitvn-app/internal/media"
	"dancefitvn-app/internal/media/fake"
	"dancefitvn-app/internal/player"

	"github.com/gdeskdev/desk/foundation/platform"
	pfake "github.com/gdeskdev/desk/foundation/platform/fake"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ====================================================================
// ============================ Contracts =============================
// ====================================================================

// npHost is the least platform.AppHost the fake driver needs: it runs
// the test's body once launched, on the driver's UI thread.
type npHost struct{ launched func() }

// countingSink is an npSink that counts the calls before handing them
// to the fake driver's Now Playing.
type countingSink struct {
	inner  npSink
	mu     sync.Mutex
	sets   int
	clears int
}

// npRig is the playback's rig with the system's Now Playing on gDesk's
// fake driver, and the fake audio session at hand.
type npRig struct {
	*playRig
	drv  *pfake.Driver
	sink *countingSink
	sess *fake.Session
}

// ====================================================================
// ============================= Creation =============================
// ====================================================================

// onUI starts a fake driver, runs body on its UI thread (the test's
// goroutine) with it, and stops it.
func onUI(t *testing.T, body func(d *pfake.Driver)) {
	t.Helper()
	d := pfake.New()
	require.NoError(t, d.Start(npHost{launched: func() {
		defer d.Stop()
		body(d)
	}}, platform.AppConfig{Name: "test"}))
	require.NoError(t, d.Loop())
}

// newNPRig returns the playback's rig, signed in, with Now Playing on d
// and the platform osName ("ios" or "android"). Call it on d's UI
// thread; the bridge is closed there at the test's end.
func newNPRig(t *testing.T, d *pfake.Driver, osName string) *npRig {
	t.Helper()
	nr := &npRig{drv: d}
	nr.sink = &countingSink{inner: driverNowPlaying{driver: func() platform.Driver { return d }}}
	nr.playRig = newPlayRig(t, "", func(dp *mobileDeps) {
		dp.Platform = osName
		dp.NowPlaying = nr.sink
		made := dp.NewMedia
		dp.NewMedia = func(cfg media.Config) (platformMedia, error) {
			pm, err := made(cfg)
			nr.sess, _ = pm.sess.(*fake.Session)

			return pm, err
		}
	})
	require.NotNil(t, nr.sess)
	require.NotNil(t, nr.w.pb.np)
	nr.signIn()

	return nr
}

// ====================================================================
// ========================== Main functions ==========================
// ====================================================================

// OnLaunched runs the test's body.
func (h npHost) OnLaunched() { h.launched() }

// OnScheduled ignores scheduled work.
func (npHost) OnScheduled() {}

// OnQuitRequest lets the app quit.
func (npHost) OnQuitRequest() bool { return true }

// OnLifecycle ignores the lifecycle.
func (npHost) OnLifecycle(platform.LifecycleState) {}

// Set counts and shows np.
func (s *countingSink) Set(np platform.NowPlaying) error {
	s.mu.Lock()
	s.sets++
	s.mu.Unlock()

	return s.inner.Set(np)
}

// Clear counts and clears.
func (s *countingSink) Clear() error {
	s.mu.Lock()
	s.clears++
	s.mu.Unlock()

	return s.inner.Clear()
}

// Handle sets fn.
func (s *countingSink) Handle(fn func(platform.MediaCommand)) error { return s.inner.Handle(fn) }

// counts returns the sets and the clears.
func (s *countingSink) counts() (sets, clears int) {
	s.mu.Lock()
	defer s.mu.Unlock()

	return s.sets, s.clears
}

// TestNowPlayingShowsLesson plays lesson 3 of Set 35 and checks what the
// system shows: the lesson's title, the course's name, the course's cover
// (the mock's JPEG), the length, the position, the speed and the state,
// the commands of Q-P4-7 and the buttons' Vietnamese names; then that it
// follows a change of speed, a seek, a new length, the next lesson and a
// pause.
func TestNowPlayingShowsLesson(t *testing.T) {
	t.Parallel()

	cover, err := os.ReadFile(filepath.Join(mockDir, "media", "covers", "set-35.jpg"))
	require.NoError(t, err)
	for _, kind := range []player.Kind{player.KindAudio, player.KindVideo} {
		t.Run(kindName(kind), func(t *testing.T) {
			t.Parallel()

			onUI(t, func(d *pfake.Driver) {
				nr := newNPRig(t, d, "ios")
				defer nr.w.pb.np.close()
				list := nr.lessons()
				p := nr.player()
				_, shown := d.NowPlaying()
				assert.False(t, shown, "nothing before the first play")
				nr.play(kind, list[2], list)

				np := nr.shown()
				assert.Equal(t, list[2].Title, np.Title)
				assert.Equal(t, "Stink and Dutty", np.Title)
				assert.Equal(t, "DanceFIT Set 35", np.Artist)
				assert.Empty(t, np.Album)
				assert.Equal(t, 220*time.Second, np.Duration)
				assert.InDelta(t, p.Pos(), np.Position.Seconds(), 0.3)
				assert.InDelta(t, player.InitialRate, np.Rate, 0)
				assert.Equal(t, platform.PlaybackPlaying, np.State)
				assert.False(t, np.Interrupted)
				assert.Equal(t, npCommands, np.Commands)
				for _, k := range []platform.MediaCommandKind{platform.MediaCommandNext, platform.MediaCommandPrevious} {
					assert.False(t, np.Commands.Has(k), "no %s (Q-P4-7)", k)
				}
				assert.Equal(t, platform.MediaLabels{Play: "Phát", Pause: "Tạm dừng"}, np.Labels)

				p.Pause() // the cover is read in real time: the lesson waits for it
				nr.state(player.StatePaused)
				assert.Equal(t, platform.PlaybackPaused, nr.shown().State)
				nr.drive("the cover", func() bool {
					np, _ := d.NowPlaying()

					return len(np.Artwork) > 0
				})
				np = nr.shown()
				assert.Equal(t, cover, np.Artwork, "the course's cover, as the server has it")
				assert.InDelta(t, p.Pos(), np.Position.Seconds(), 0.01, "where it paused")
				p.Play()
				nr.state(player.StatePlaying)
				assert.Equal(t, platform.PlaybackPlaying, nr.shown().State)

				p.SetRate(1.25)
				assert.InDelta(t, 1.25, nr.shown().Rate, 0, "a new speed")

				p.Seek(100)
				assert.InDelta(t, 100, nr.shown().Position.Seconds(), 0.01, "a seek")

				nr.eng.SetDuration(200)
				nr.q.drain()
				assert.Equal(t, 200*time.Second, nr.shown().Duration, "the engine's length")

				p.Next()
				nr.state(player.StatePlaying)
				np = nr.shown()
				assert.Equal(t, list[3].Title, np.Title, "the next lesson")
				assert.Equal(t, "DanceFIT Set 35", np.Artist)
				assert.InDelta(t, 0, np.Position.Seconds(), 0.5)
				assert.Equal(t, cover, np.Artwork, "the same course's cover, not read again")

				p.Pause()
				nr.state(player.StatePaused)
				np = nr.shown()
				assert.Equal(t, platform.PlaybackPaused, np.State)
				assert.InDelta(t, p.Pos(), np.Position.Seconds(), 0.01, "where it paused")
			})
		})
	}
}

// TestNowPlayingNotPerTick checks the system is told again only when
// something changes: playing on for seconds (ticks every 250 ms) sets
// nothing.
func TestNowPlayingNotPerTick(t *testing.T) {
	t.Parallel()

	onUI(t, func(d *pfake.Driver) {
		nr := newNPRig(t, d, "ios")
		defer nr.w.pb.np.close()
		list := nr.lessons()
		nr.play(player.KindAudio, list[2], list)
		nr.player().Pause() // the cover is read in real time: the lesson waits for it
		nr.state(player.StatePaused)
		nr.drive("the cover", func() bool {
			np, _ := d.NowPlaying()

			return len(np.Artwork) > 0
		})
		nr.player().Play()
		nr.state(player.StatePlaying)
		sets, _ := nr.sink.counts()
		pos := nr.player().Pos()
		nr.tt.Elapse(5 * time.Second)
		require.Greater(t, nr.player().Pos(), pos+3, "it played on")
		after, _ := nr.sink.counts()
		assert.Equal(t, sets, after, "no Set at a tick")
	})
}

// TestNowPlayingNotPerTickDoesNotAllocate: the bridge's work at a tick of
// the player (ChangeTime) allocates nothing.
func TestNowPlayingNotPerTickDoesNotAllocate(t *testing.T) { //nolint:paralleltest // AllocsPerRun needs the process to itself
	onUI(t, func(d *pfake.Driver) {
		nr := newNPRig(t, d, "ios")
		defer nr.w.pb.np.close()
		list := nr.lessons()
		nr.play(player.KindAudio, list[2], list)
		n := nr.w.pb.np
		tick := player.Change{Kind: player.ChangeTime}
		assert.Zero(t, testing.AllocsPerRun(200, func() { n.onChange(tick) }))
	})
}

// BenchmarkNowPlayingTick measures the bridge's work at a tick.
func BenchmarkNowPlayingTick(b *testing.B) {
	n := newNowPlaying(npOptions{sink: &countingSink{}})
	tick := player.Change{Kind: player.ChangeTime}
	b.ReportAllocs()
	for b.Loop() {
		n.onChange(tick)
	}
}

// TestNowPlayingCommands gives each command as the system's media controls
// do (the fake driver's PressMediaCommand) and checks what the player did;
// the commands not offered (previous, next) do not reach the app.
func TestNowPlayingCommands(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		paused  bool // pause before the command
		cmd     platform.MediaCommand
		offered bool
		state   player.State
		pos     func(before float64) float64 // where the player is after; nil: not checked
	}{
		{"play", true, platform.MediaCommand{Kind: platform.MediaCommandPlay}, true, player.StatePlaying, nil},
		{"pause", false, platform.MediaCommand{Kind: platform.MediaCommandPause}, true, player.StatePaused, nil},
		{"toggle pauses", false, platform.MediaCommand{Kind: platform.MediaCommandToggle}, true, player.StatePaused, nil},
		{"toggle plays", true, platform.MediaCommand{Kind: platform.MediaCommandToggle}, true, player.StatePlaying, nil},
		{
			"seek to", true, platform.MediaCommand{Kind: platform.MediaCommandSeekTo, Position: 150 * time.Second}, true,
			player.StatePaused, func(float64) float64 { return 150 },
		},
		{
			"skip forward", true, platform.MediaCommand{Kind: platform.MediaCommandSkip, Offset: 10 * time.Second}, true,
			player.StatePaused, func(b float64) float64 { return b + 10 },
		},
		{
			"skip back", true, platform.MediaCommand{Kind: platform.MediaCommandSkip, Offset: -10 * time.Second}, true,
			player.StatePaused, func(b float64) float64 { return b - 10 },
		},
		{"next is not offered", false, platform.MediaCommand{Kind: platform.MediaCommandNext}, false, player.StatePlaying, nil},
		{"previous is not offered", false, platform.MediaCommand{Kind: platform.MediaCommandPrevious}, false, player.StatePlaying, nil},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			for _, osName := range []string{"ios", "android"} {
				onUI(t, func(d *pfake.Driver) {
					nr := newNPRig(t, d, osName)
					defer nr.w.pb.np.close()
					list := nr.lessons()
					p := nr.player()
					nr.play(player.KindAudio, list[2], list)
					p.Seek(50)
					nr.tt.Elapse(2 * time.Second)
					if tc.paused {
						p.Pause()
						nr.state(player.StatePaused)
					}
					before := p.Pos()
					lesson := p.Lesson().ID
					require.Equal(t, tc.offered, d.PressMediaCommand(tc.cmd), "%s: offered", osName)
					if tc.offered {
						d.PumpEvents() // the command reaches the app through the task queue
					}
					nr.state(tc.state)
					assert.Equal(t, lesson, p.Lesson().ID, "%s: the same lesson", osName)
					if tc.pos != nil {
						assert.InDelta(t, tc.pos(before), p.Pos(), 0.3, "%s: the position", osName)
						assert.InDelta(t, tc.pos(before), nr.shown().Position.Seconds(), 0.3, "%s: shown", osName)
					}
				})
			}
		})
	}
}

// TestNowPlayingClearsAfter10Min checks Now Playing goes away after ten
// minutes without playing, stays while playing, and comes back with the
// next play.
func TestNowPlayingClearsAfter10Min(t *testing.T) {
	t.Parallel()

	onUI(t, func(d *pfake.Driver) {
		nr := newNPRig(t, d, "ios")
		defer nr.w.pb.np.close()
		list := nr.lessons()
		p := nr.player()
		nr.play(player.KindAudio, list[2], list)
		nr.clk.Advance(npPausedFor + time.Minute)
		assert.True(t, nr.isShown(), "kept while playing")

		p.Pause()
		nr.state(player.StatePaused)
		nr.clk.Advance(npPausedFor - time.Second)
		assert.True(t, nr.isShown(), "not yet")
		nr.clk.Advance(time.Second)
		assert.False(t, nr.isShown(), "ten minutes paused")
		assert.False(t, d.PressMediaCommand(platform.MediaCommand{Kind: platform.MediaCommandPlay}), "no command reaches a cleared app")

		p.Play()
		nr.state(player.StatePlaying)
		assert.True(t, nr.isShown(), "shown again at the next play")
		assert.Equal(t, platform.PlaybackPlaying, nr.shown().State)
	})
}

// TestNowPlayingInterrupted checks an interruption (a call) of a lesson
// that plays: it shows paused and Interrupted; on Android it is kept past
// ten minutes while the interruption lasts, on iOS it is not; its end
// without playing again shows it paused and arranges the clearing; its
// end with ShouldResume plays again.
func TestNowPlayingInterrupted(t *testing.T) {
	t.Parallel()

	tests := []struct {
		os   string
		kept bool // kept past ten minutes while interrupted
	}{
		{"android", true},
		{"ios", false},
	}
	for _, tc := range tests {
		t.Run(tc.os, func(t *testing.T) {
			t.Parallel()

			onUI(t, func(d *pfake.Driver) {
				nr := newNPRig(t, d, tc.os)
				defer nr.w.pb.np.close()
				list := nr.lessons()
				p := nr.player()
				nr.play(player.KindAudio, list[2], list)

				nr.sess.Emit(media.SessionEvent{Kind: media.InterruptBegin})
				nr.state(player.StatePaused)
				np := nr.shown()
				assert.Equal(t, platform.PlaybackPaused, np.State)
				assert.True(t, np.Interrupted)

				nr.clk.Advance(npPausedFor + time.Second)
				assert.Equal(t, tc.kept, nr.isShown(), "past ten minutes of a call")
				if !tc.kept {
					return
				}
				nr.sess.Emit(media.SessionEvent{Kind: media.InterruptEnd})
				nr.q.drain()
				assert.Equal(t, player.StatePaused, p.State())
				assert.False(t, nr.shown().Interrupted, "the call ended")
				nr.clk.Advance(npPausedFor)
				assert.False(t, nr.isShown(), "ten minutes after the call")
			})
		})
	}

	t.Run("resume", func(t *testing.T) {
		t.Parallel()

		onUI(t, func(d *pfake.Driver) {
			nr := newNPRig(t, d, "android")
			defer nr.w.pb.np.close()
			list := nr.lessons()
			nr.play(player.KindAudio, list[2], list)
			nr.sess.Emit(media.SessionEvent{Kind: media.InterruptBegin})
			nr.state(player.StatePaused)
			nr.sess.Emit(media.SessionEvent{Kind: media.InterruptEnd, ShouldResume: true})
			nr.state(player.StatePlaying)
			np := nr.shown()
			assert.Equal(t, platform.PlaybackPlaying, np.State)
			assert.False(t, np.Interrupted)
		})
	})

	t.Run("in-app pause ends it", func(t *testing.T) {
		t.Parallel()

		onUI(t, func(d *pfake.Driver) {
			nr := newNPRig(t, d, "android")
			defer nr.w.pb.np.close()
			list := nr.lessons()
			nr.play(player.KindAudio, list[2], list)
			nr.sess.Emit(media.SessionEvent{Kind: media.InterruptBegin})
			nr.state(player.StatePaused)
			require.True(t, nr.shown().Interrupted)
			require.True(t, nr.player().Interrupted())
			nr.player().Pause() // the user taps pause on screen during the call
			assert.False(t, nr.shown().Interrupted, "the user paused")
			nr.clk.Advance(npPausedFor)
			assert.False(t, nr.isShown(), "cleared after ten minutes")
		})
	})

	t.Run("pause command ends it", func(t *testing.T) {
		t.Parallel()

		onUI(t, func(d *pfake.Driver) {
			nr := newNPRig(t, d, "android")
			defer nr.w.pb.np.close()
			list := nr.lessons()
			nr.play(player.KindAudio, list[2], list)
			nr.sess.Emit(media.SessionEvent{Kind: media.InterruptBegin})
			nr.state(player.StatePaused)
			require.True(t, nr.shown().Interrupted)
			require.True(t, d.PressMediaCommand(platform.MediaCommand{Kind: platform.MediaCommandPause}))
			d.PumpEvents()
			assert.False(t, nr.shown().Interrupted, "the user paused")
			nr.clk.Advance(npPausedFor)
			assert.False(t, nr.isShown())
		})
	})
}

// TestNowPlayingClearsOnSignOut checks a sign-out (the user's, or the
// server's refusal) stops the lesson and takes Now Playing away, and the
// window's closing does too, dropping the commands' function.
func TestNowPlayingClearsOnSignOut(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		end  func(nr *npRig)
	}{
		{"logout", func(nr *npRig) { require.NoError(nr.t, nr.w.Logout(nr.t.Context())) }},
		{"refused", func(nr *npRig) { nr.w.mgr.SignOut(nil) }},
		{"close", func(nr *npRig) { nr.w.pb.close() }},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			onUI(t, func(d *pfake.Driver) {
				nr := newNPRig(t, d, "android")
				defer nr.w.pb.np.close()
				list := nr.lessons()
				nr.play(player.KindAudio, list[2], list)
				require.True(t, nr.isShown())

				tc.end(nr)
				nr.q.drain()
				assert.False(t, nr.isShown(), "taken away")
				assert.False(t, d.PressMediaCommand(platform.MediaCommand{Kind: platform.MediaCommandPlay}))
				if tc.name != "close" {
					nr.state(player.StatePaused)
					nr.tt.Elapse(time.Second)
					assert.False(t, nr.isShown(), "the pause that follows shows nothing")
				}
			})
		})
	}
}

// TestWatchedLocal checks the lessons this run watched: one the tracker
// saw played to 90 %, one the server answered watched; none of another
// user's after a sign-out.
func TestWatchedLocal(t *testing.T) {
	t.Parallel()

	pr := newPlayRig(t, "", nil)
	pr.signIn()
	w := pr.w
	s := w.pb.saver
	assert.False(t, w.WatchedLocal("l_3503"))

	s.Update("l_3503", demoCourseID, 100, 220, true)
	assert.False(t, w.WatchedLocal("l_3503"), "under 90 %")
	s.Update("l_3503", demoCourseID, 198, 220, true)
	assert.True(t, w.WatchedLocal("l_3503"), "198 of 220")

	require.NoError(t, w.mock.SetScenario("lessons.progress", "watched"))
	saves := pr.st.calledN("lessons.progress")
	s.Update("l_3504", demoCourseID, 30, 250, false)
	s.Pause()
	pr.drive("the save", func() bool { return pr.st.calledN("lessons.progress") > saves && w.WatchedLocal("l_3504") })
	assert.True(t, w.WatchedLocal("l_3504"), "the server says watched")
	assert.False(t, w.WatchedLocal("l_3505"))

	require.NoError(t, w.Logout(t.Context()))
	pr.q.drain()
	assert.False(t, w.WatchedLocal("l_3503"), "forgotten at the sign-out")
	assert.False(t, w.WatchedLocal("l_3504"))
}

// ====================================================================
// ========================= Helper Functions =========================
// ====================================================================

// play plays l of list as kind and drives until it plays.
func (nr *npRig) play(kind player.Kind, l player.Lesson, list []player.Lesson) {
	nr.t.Helper()
	if kind == player.KindVideo {
		nr.player().PlayVideo(l, list)
	} else {
		nr.player().PlayAudio(l, list)
	}
	nr.state(player.StatePlaying)
}

// shown returns what the system shows; the test fails when nothing does.
func (nr *npRig) shown() platform.NowPlaying {
	nr.t.Helper()
	np, ok := nr.drv.NowPlaying()
	require.True(nr.t, ok, "Now Playing shows")

	return np
}

// isShown tells whether the system shows something.
func (nr *npRig) isShown() bool {
	_, ok := nr.drv.NowPlaying()

	return ok
}

// kindName names a kind for a subtest.
func kindName(k player.Kind) string {
	if k == player.KindVideo {
		return "video"
	}

	return "audio"
}

// TestNowPlayingBackground checks the position shown after the engine's
// tick stopped in the background, and that a remote Play cannot start a
// video there while a sound answers.
func TestNowPlayingBackground(t *testing.T) {
	t.Parallel()

	tests := []struct {
		kind    player.Kind
		playing bool // plays after a remote Play of the paused lesson
	}{
		{player.KindVideo, false},
		{player.KindAudio, true},
	}
	for _, tc := range tests {
		t.Run(kindName(tc.kind), func(t *testing.T) {
			t.Parallel()

			onUI(t, func(d *pfake.Driver) {
				nr := newNPRig(t, d, "android")
				defer nr.w.pb.np.close()
				list := nr.lessons()
				p := nr.player()
				nr.play(tc.kind, list[2], list)
				p.Pause()
				nr.state(player.StatePaused)
				nr.life(platform.LifecycleBackground)
				require.True(t, d.PressMediaCommand(platform.MediaCommand{Kind: platform.MediaCommandPlay}))
				d.PumpEvents()
				nr.tt.Elapse(time.Second)
				if tc.playing {
					nr.state(player.StatePlaying)
					assert.Equal(t, platform.PlaybackPlaying, nr.shown().State)
				} else {
					assert.Equal(t, player.StatePaused, p.State(), "a remote Play of a video is ignored")
					assert.False(t, p.Playing())
					assert.Equal(t, platform.PlaybackPaused, nr.shown().State)
				}
			})
		})
	}

	t.Run("position is the engine's", func(t *testing.T) {
		t.Parallel()

		onUI(t, func(d *pfake.Driver) {
			nr := newNPRig(t, d, "android")
			defer nr.w.pb.np.close()
			list := nr.lessons()
			p := nr.player()
			nr.play(player.KindAudio, list[2], list)
			p.Seek(50)
			p.SetRate(1)
			nr.life(platform.LifecycleBackground)
			nr.tt.Elapse(30 * time.Second) // the tick stops, the sound plays on
			require.True(t, d.PressMediaCommand(platform.MediaCommand{Kind: platform.MediaCommandPause}))
			d.PumpEvents()
			nr.state(player.StatePaused)
			assert.InDelta(t, 80, nr.shown().Position.Seconds(), 1.5)
		})
	})
}
