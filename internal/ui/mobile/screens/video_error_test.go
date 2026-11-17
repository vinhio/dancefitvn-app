//go:build !tvos

package screens_test

import (
	"fmt"
	"io/fs"
	"testing"

	"dancefitvn-app/internal/api"
	"dancefitvn-app/internal/player"
	"dancefitvn-app/internal/ui/icon"
	"dancefitvn-app/internal/ui/kit"
	"dancefitvn-app/internal/ui/kit/kittest"
	"dancefitvn-app/internal/ui/mobile/env"
	"dancefitvn-app/internal/ui/mobile/shell/shelltest"

	"github.com/gdeskdev/desk/foundation/core/geom"
	"github.com/gdeskdev/desk/framework/ui"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// painted returns the glyph the button's child icon draws: the symbol a person
// sees, which the Icon field only promises until the next layout.
func painted(t testing.TB, w ui.Widget) icon.Name {
	t.Helper()
	for ch := range w.Node().Children() {
		if ic, ok := ch.(*kit.Icon); ok {
			return ic.Glyph
		}
	}
	require.Fail(t, "the button has no icon child")

	return icon.NameUnknown
}

// TestVideoFailedLoadShowsPlay checks that a lesson that fails to load (the stream
// answers 404, the playlist is gone) leaves screen 06 showing the paused state: the
// play symbol drawn (not the pause one it showed while it loaded, with no tick to lay
// the button out again), and a tap on it asks the player again.
func TestVideoFailedLoadShowsPlay(t *testing.T) {
	t.Parallel()

	kittest.ForEachPhone(t, func(t *testing.T, ph kittest.Phone) {
		r, pb, _ := atVideo(t, ph, false)
		p := partsOf(t, r)
		pb.SetState(player.StateLoading)
		r.Settle()
		require.Equal(t, icon.Pause, painted(t, p.toggle), "a load under way shows pause, as playing does")

		pb.Fail(&api.Error{Status: 404, Code: "not_found"})
		r.Settle()
		assert.Equal(t, icon.Play, p.toggle.Icon, "the state of the button")
		assert.Equal(t, icon.Play, painted(t, p.toggle), "the symbol drawn")
		assert.Equal(t, "Phát", p.toggle.Label)
		assert.True(t, r.Tester.Shows("Không phát được bài này. Thử lại sau"))

		pb.ClearCalls()
		tap(r, p.toggle)
		assert.Equal(t, 1, pb.Count("Toggle"), "the tap asks the player, which fetches the stream again")
		assert.Equal(t, player.StateLoading, pb.State())
		r.Settle()
		assert.Equal(t, icon.Pause, painted(t, p.toggle), "loading again")

		pb.SetState(player.StatePlaying)
		r.Settle()
		assert.Equal(t, icon.Pause, painted(t, p.toggle))
	})
}

// TestVideoPauseDrawsPlay checks that a pause with no tick after it (the video
// is paused, nothing else moves) draws the play symbol too.
func TestVideoPauseDrawsPlay(t *testing.T) {
	t.Parallel()

	kittest.ForEachPhone(t, func(t *testing.T, ph kittest.Phone) {
		r, pb, _ := atVideo(t, ph, false)
		p := partsOf(t, r)
		require.Equal(t, icon.Pause, painted(t, p.toggle))
		pb.SetState(player.StatePaused)
		r.Settle()
		assert.Equal(t, icon.Play, painted(t, p.toggle))
		pb.SetState(player.StatePlaying)
		r.Settle()
		assert.Equal(t, icon.Pause, painted(t, p.toggle))
	})
}

// atVideoOver is atVideo over a mock folder whose course data are patched.
func atVideoOver(t *testing.T, ph kittest.Phone, fsys fs.FS) (*shelltest.Rig, *shelltest.FakePlayback) {
	t.Helper()
	r := detailOver(t, ph, fsys)
	r.Fake.Dl().Clear() // lesson 3 has a file in the demo state, which a button of its own shows
	r.Shell.PlayVideo("set-35", "l_3503", 3)
	r.Settle()
	require.Equal(t, env.RouteVideo, r.Route())
	r.Tester.Elapse(kittest.Frame)

	return r, r.Fake.FakePlayback()
}

// TestVideoAudioButtonOfALessonWithoutAudio checks the side column of 06 for a lesson
// the course data give no audio file (has_audio false): the button "Âm thanh" is gone
// (it is the last, so the other three keep their places), and one that finds out in the
// run (lessons.audio answers not_found) goes as well.
func TestVideoAudioButtonOfALessonWithoutAudio(t *testing.T) {
	t.Parallel()

	kittest.ForEachPhone(t, func(t *testing.T, ph kittest.Phone) {
		normal, _, _ := atVideo(t, ph, false)
		want := map[string]geom.Rect{}
		for _, n := range []string{"sheet:options", "mirror", "loop"} {
			want[n] = named(normal.Shell, n, 1).Node().WindowFrame()
		}
		require.NotNil(t, named(normal.Shell, "dlTap:set35-3", 1), "a lesson with an audio has the button")

		without, _ := atVideoOver(t, ph, patchedCourse(t, func(lessons []any) { lessonAt(lessons, 3)["has_audio"] = false }))
		assert.False(t, has(without, "dlTap:set35-3"), "no button for a lesson with no audio")
		for n, f := range want {
			assert.Equal(t, f, named(without.Shell, n, 1).Node().WindowFrame(), n+" stays where it is")
		}

		// in the run: not_found answered for the audio of the lesson
		r, _, _ := atVideo(t, ph, false)
		r.Fake.Dl().SetDownload("l_3503", kit.DlIng, .1)
		r.Settle()
		require.True(t, has(r, "dlTap:set35-3"))
		r.Fake.Dl().FailDownload("l_3503", fmt.Errorf("download: %w", env.ErrNoAudio))
		r.Settle()
		assert.False(t, has(r, "dlTap:set35-3"))
		assert.True(t, r.Tester.Shows(toastNoAudio))
	})
}

// has tells whether a visible widget is called name.
func has(r *shelltest.Rig, name string) bool { return named(r.Shell, name, 1) != nil }
