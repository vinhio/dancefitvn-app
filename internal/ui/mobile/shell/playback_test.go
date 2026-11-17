//go:build !tvos

package shell_test

import (
	"fmt"
	"testing"

	"dancefitvn-app/internal/player"
	"dancefitvn-app/internal/store"
	"dancefitvn-app/internal/ui/kit/kittest"
	"dancefitvn-app/internal/ui/mobile/env"
	"dancefitvn-app/internal/ui/mobile/shell"
	"dancefitvn-app/internal/ui/mobile/shell/shelltest"
	"dancefitvn-app/locales"

	"github.com/gdeskdev/desk/foundation/core/keys"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// toastLoad is player.err.load.
const toastLoad = "Không phát được bài này. Thử lại sau"

// atCourse signs in, boots and opens the detail of set-35, so that the course is
// read (the lessons the player is given); it returns the rig and the fake player
// the Fake carries.
func atCourse(t *testing.T, ph kittest.Phone) (*shelltest.Rig, *shelltest.FakePlayback) {
	t.Helper()
	f, err := shelltest.New(shelltest.MockFS(t))
	require.NoError(t, err)
	require.NoError(t, f.SeedSession())
	r := shelltest.MountRig(t, ph, false, f, shell.Options{})
	r.Boot()
	require.Equal(t, "home", r.Route())
	r.Shell.Detail("set-35")
	r.Tester.Elapse(kittest.Frame) // the screen reads the course on its first frame
	r.Settle()
	require.Equal(t, "detail", r.Route())
	_, ok := r.Shell.Env().CachedCourse("set-35")
	require.True(t, ok, "the course is read")

	return r, f.FakePlayback()
}

// TestPlayVideoOpensScreen06 checks `playVideo` (03 section 3) from the detail of
// a course and from Home with the course not read yet: the lesson is on the
// player with its course data (number, resume position, length, the list) and
// screen 06 is on top of the stack.
func TestPlayVideoOpensScreen06(t *testing.T) {
	t.Parallel()

	kittest.ForEachPhone(t, func(t *testing.T, ph kittest.Phone) {
		t.Run("from the detail", func(t *testing.T) {
			r, pb := atCourse(t, ph)
			depth := r.Shell.Nav().Depth()
			r.Shell.PlayVideo("set-35", "l_3503", 3)
			r.Settle()
			assert.Equal(t, env.RouteVideo, r.Route())
			assert.Equal(t, depth+1, r.Shell.Nav().Depth())
			assert.Equal(t, []string{"PlayVideo:l_3503"}, pb.Calls())
			l := pb.Lesson()
			assert.Equal(t, "l_3503", l.ID)
			assert.Equal(t, 3, l.No)
			assert.Equal(t, 10, l.Count)
			assert.Equal(t, 72, l.ResumeSec)
			assert.Equal(t, 220, l.DurationSec)
			assert.Equal(t, "DanceFIT Set 35", l.CourseName)
			assert.Equal(t, "SET 35", l.CourseShort)
			assert.Equal(t, player.KindVideo, pb.Kind())
			assert.Equal(t, 72.0, pb.Pos(), "the resume position of the course data")
			assert.True(t, pb.HasPrev())
			assert.True(t, pb.HasNext())
		})
		t.Run("from Home, the course not read", func(t *testing.T) {
			f, err := shelltest.New(shelltest.MockFS(t))
			require.NoError(t, err)
			require.NoError(t, f.SeedSession())
			r := shelltest.MountRig(t, ph, false, f, shell.Options{})
			r.Boot()
			require.Equal(t, "home", r.Route())
			r.Shell.PlayVideo("set-35", "l_3503", 3)
			assert.Equal(t, "home", r.Route(), "the course is on its way")
			r.Settle()
			assert.Equal(t, env.RouteVideo, r.Route())
			assert.Equal(t, 1, r.Shell.Nav().Depth())
			assert.Equal(t, []string{"PlayVideo:l_3503"}, f.FakePlayback().Calls())
		})
		t.Run("a lesson the course lacks", func(t *testing.T) {
			r, pb := atCourse(t, ph)
			r.Shell.PlayVideo("set-35", "l_nope", 99)
			r.Tester.Elapse(100 * ms)
			assert.Equal(t, "detail", r.Route())
			assert.Empty(t, pb.Calls())
			assert.True(t, r.Tester.Shows(toastLoad))
		})
	})
}

// TestPlayAudioAndOffTap checks `playAudio` and `offTap`: the sound starts and 08
// opens on the stack; the offline screen's lesson with its file plays, one without
// toasts that the video needs the network.
func TestPlayAudioAndOffTap(t *testing.T) {
	t.Parallel()

	kittest.ForEachPhone(t, func(t *testing.T, ph kittest.Phone) {
		r, pb := atCourse(t, ph)
		depth := r.Shell.Nav().Depth()
		r.Shell.PlayAudio("set-35", "l_3503", 3)
		assert.Equal(t, env.RouteAudio, r.Route())
		assert.Equal(t, depth+1, r.Shell.Nav().Depth())
		assert.Equal(t, []string{"PlayAudio:l_3503"}, pb.Calls())
		assert.Equal(t, player.KindAudio, pb.Kind())
		assert.Equal(t, 0.0, pb.Pos(), "a sound starts at 0")

		r.Shell.Back()
		require.Equal(t, "detail", r.Route())
		assert.Equal(t, 0, pb.Count("Leave"), "a sound goes on playing when 08 is left")
		assert.True(t, pb.Playing())

		pb.ClearCalls()
		r.Shell.OffTap("set-35", "l_3504", 4) // the demo has the files of 1, 2, 3, 4 and 6
		assert.Equal(t, env.RouteAudio, r.Route(), "a downloaded lesson plays its sound")
		assert.Equal(t, []string{"PlayAudio:l_3504"}, pb.Calls())
		r.Shell.Back()
		r.Shell.OffTap("set-35", "l_3509", 9)
		r.Tester.Elapse(100 * ms)
		assert.Equal(t, "detail", r.Route())
		assert.True(t, r.Tester.Shows("Video bài 09 cần có mạng để xem"))
		assert.Equal(t, 0, pb.Count("PlayAudio:l_3509"))
	})
}

// TestBackFromVideoLeavesThePlayer checks that leaving 06 by Back and by the Back key
// tells the player Leave once, and returns to the screen
// under it.
func TestBackFromVideoLeavesThePlayer(t *testing.T) {
	t.Parallel()

	kittest.ForEachPhone(t, func(t *testing.T, ph kittest.Phone) {
		for _, how := range []string{"Back", "key"} {
			t.Run(how, func(t *testing.T) {
				r, pb := atCourse(t, ph)
				r.Shell.PlayVideo("set-35", "l_3503", 3)
				require.Equal(t, env.RouteVideo, r.Route())
				require.True(t, pb.Playing())
				assert.Equal(t, 0, pb.Count("Leave"))
				if how == "Back" {
					r.Shell.Back()
				} else {
					require.True(t, r.Tester.Key(keys.KeyBack, 0))
				}
				assert.Equal(t, "detail", r.Route())
				assert.Equal(t, 1, pb.Count("Leave"))
				assert.False(t, pb.Playing(), "a video pauses when 06 is left")
			})
		}
	})
}

// TestVideoEndsOnScreen10 checks the player's ChangeEnded: a video ending on 06
// puts 10 in its place (the stack as it was); a sound ending on 08 changes
// nothing; `playVideo` from 10 puts 06 in 10's place; PlayNext does that for the
// next lesson, when the preference is on.
func TestVideoEndsOnScreen10(t *testing.T) {
	t.Parallel()

	kittest.ForEachPhone(t, func(t *testing.T, ph kittest.Phone) {
		t.Run("end, then a lesson from 10", func(t *testing.T) {
			r, pb := atCourse(t, ph)
			r.Shell.PlayVideo("set-35", "l_3503", 3)
			depth := r.Shell.Nav().Depth()
			pb.End()
			assert.Equal(t, env.RouteDone, r.Route())
			assert.Equal(t, depth, r.Shell.Nav().Depth(), "Replace: the stack stays")
			r.Shell.PlayVideo("set-35", "l_3504", 4)
			assert.Equal(t, env.RouteVideo, r.Route())
			assert.Equal(t, depth, r.Shell.Nav().Depth(), "Replace again")
			assert.Equal(t, "l_3504", pb.Lesson().ID)
			assert.Equal(t, 0.0, pb.Pos(), "lesson 4 has no resume position")
			assert.Equal(t, 1, pb.Count("Leave"), "06 told the player when 10 took its place")
		})
		t.Run("a sound ending changes nothing", func(t *testing.T) {
			r, pb := atCourse(t, ph)
			r.Shell.PlayAudio("set-35", "l_3503", 3)
			pb.End()
			assert.Equal(t, env.RouteAudio, r.Route())
		})
		t.Run("PlayNext", func(t *testing.T) {
			r, pb := atCourse(t, ph)
			r.Shell.PlayVideo("set-35", "l_3503", 3)
			pb.End()
			require.Equal(t, env.RouteDone, r.Route())
			depth := r.Shell.Nav().Depth()
			r.Shell.PlayNext()
			assert.Equal(t, env.RouteVideo, r.Route())
			assert.Equal(t, depth, r.Shell.Nav().Depth())
			assert.Equal(t, "l_3504", pb.Lesson().ID)
			assert.True(t, pb.Playing())
		})
		t.Run("PlayNext with auto next off", func(t *testing.T) {
			r, pb := atCourse(t, ph)
			r.Fake.FakePrefs().SetAutoNext(false)
			r.Shell.PlayVideo("set-35", "l_3503", 3)
			pb.End()
			r.Shell.PlayNext()
			assert.Equal(t, env.RouteDone, r.Route())
			assert.Equal(t, "l_3503", pb.Lesson().ID)
		})
		t.Run("PlayNext at the last lesson", func(t *testing.T) {
			r, pb := atCourse(t, ph)
			r.Shell.PlayVideo("set-35", "l_3510", 10)
			pb.End()
			require.Equal(t, env.RouteDone, r.Route())
			r.Shell.PlayNext()
			assert.Equal(t, env.RouteDone, r.Route())
			assert.Equal(t, "l_3510", pb.Lesson().ID)
		})
	})
}

// TestPlayerErrors checks 03 section 9: a lesson that fails to load toasts "Không
// phát được bài này. Thử lại sau" and stays on the screen; the server saying
// not_enrolled opens the course's detail in the place of 06.
func TestPlayerErrors(t *testing.T) {
	t.Parallel()

	kittest.ForEachPhone(t, func(t *testing.T, ph kittest.Phone) {
		t.Run("load failed", func(t *testing.T) {
			r, pb := atCourse(t, ph)
			r.Shell.PlayVideo("set-35", "l_3503", 3)
			pb.Fail(fmt.Errorf("engine: load: %w", assert.AnError))
			r.Tester.Elapse(100 * ms)
			assert.Equal(t, env.RouteVideo, r.Route())
			assert.True(t, r.Tester.Shows(toastLoad))
			r.Tester.Elapse(2300 * ms)
			assert.False(t, r.Tester.Shows(toastLoad))
		})
		t.Run("not enrolled", func(t *testing.T) {
			r, pb := atCourse(t, ph)
			r.Shell.PlayVideo("set-35", "l_3503", 3)
			depth := r.Shell.Nav().Depth()
			pb.Fail(fmt.Errorf("resolve: %w", player.ErrNotEnrolled))
			assert.Equal(t, "detail", r.Route())
			assert.Equal(t, "set-35", r.Shell.Nav().Current().Params)
			assert.Equal(t, depth, r.Shell.Nav().Depth())
			assert.Equal(t, 1, pb.Count("Leave"))
		})
		t.Run("an error on another screen", func(t *testing.T) {
			r, pb := atCourse(t, ph)
			pb.Fail(assert.AnError)
			r.Tester.Elapse(100 * ms)
			assert.Equal(t, "detail", r.Route())
			assert.False(t, r.Tester.Shows(toastLoad))
		})
	})
}

// TestToDetailBuildsTheStack checks `toDetail` of 10: the detail of the course with
// [courses] under it, wherever the user came from.
func TestToDetailBuildsTheStack(t *testing.T) {
	t.Parallel()

	kittest.ForEachPhone(t, func(t *testing.T, ph kittest.Phone) {
		r, pb := atCourse(t, ph)
		r.Shell.PlayVideo("set-35", "l_3503", 3)
		pb.End()
		require.Equal(t, env.RouteDone, r.Route())
		r.Shell.ToDetail("set-35")
		r.Settle()
		assert.Equal(t, "detail", r.Route())
		assert.Equal(t, "set-35", r.Shell.Nav().Current().Params)
		stack := r.Shell.Nav().Stack()
		require.Len(t, stack, 1)
		assert.Equal(t, "courses", stack[0].Screen)
		r.Shell.Back()
		assert.Equal(t, "courses", r.Route())
	})
}

// noPlayer is services without a player: Playback is nil and Prefs are the
// defaults.
type noPlayer struct{ env.Services }

// Playback is nil: no player.
func (noPlayer) Playback() env.Playback { return nil }

func (noPlayer) WatchedLocal(string) bool { return false }

// Prefs are nil: Env keeps the defaults in memory.
func (noPlayer) Prefs() env.Prefs { return nil }

// TestPlaybackActionsWithoutAPlayer checks that services that have no player leave
// playVideo, playAudio and PlayNext doing nothing (the app before P4.6a), and give
// the screens the default preferences.
func TestPlaybackActionsWithoutAPlayer(t *testing.T) {
	t.Parallel()

	kittest.ForEachPhone(t, func(t *testing.T, ph kittest.Phone) {
		f, err := shelltest.New(shelltest.MockFS(t))
		require.NoError(t, err)
		require.NoError(t, f.SeedSession())
		sh := shell.New(noPlayer{f}, locales.MustTexts(), shell.Options{})
		tt := kittest.Mount(t, ph, sh)
		tt.RunUntilIdle()
		tt.Elapse(600 * ms)
		tt.RunUntilIdle()
		require.Equal(t, "home", sh.Nav().Current().Screen)
		assert.Nil(t, sh.Env().Playback())

		sh.PlayVideo("set-35", "l_3503", 3)
		sh.PlayAudio("set-35", "l_3503", 3)
		sh.PlayNext()
		tt.RunUntilIdle()
		assert.Equal(t, "home", sh.Nav().Current().Screen)

		p := sh.Env().Prefs()
		assert.True(t, p.AutoNext())
		assert.Equal(t, store.QualityAuto, p.Quality())
		p.CycleQuality()
		assert.Equal(t, store.QualityP1080, sh.Env().Prefs().Quality(), "kept in memory")
	})
}
