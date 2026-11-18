//go:build !tvos

package screens_test

import (
	"strings"
	"testing"

	"dancefitvn-app/internal/player"
	"dancefitvn-app/internal/ui/kit"
	"dancefitvn-app/internal/ui/kit/kittest"
	"dancefitvn-app/internal/ui/mobile/shell/shelltest"

	"github.com/gdeskdev/desk/foundation/core/geom"
	"github.com/gdeskdev/desk/framework/ui"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// longTopCourse is the name of a production course, wider than the room between
// the top row's two buttons once it is upper-cased and letter-spaced.
const longTopCourse = `DanceFIT® Set 35: "Cùng nhảy cùng hát và một tên rất dài nữa cho đủ dài"`

// topRowGap is the least air between the course lines and a round button.
const topRowGap = 8

// withLongCourseLesson makes the playback play lesson l_3505 of longTopCourse.
func withLongCourseLesson(r *shelltest.Rig, pb *shelltest.FakePlayback, audio bool) {
	l := pb.Lesson()
	l.ID, l.No, l.CourseName, l.Tagline = "l_3505", 5, longTopCourse, "Cùng nhảy cùng hát"
	if audio {
		r.Fake.Dl().SetDownload(l.ID, kit.DlDone, 1) // the file is on the device: the "…" shows
		pb.PlayAudio(l, []player.Lesson{l})
	} else {
		pb.PlayVideo(l, []player.Lesson{l})
	}
}

// drawnWidth is the width of the text the label w draws: with an ellipsis it is what
// fits its box, without one it is the whole line, drawn past the box.
func drawnWidth(t testing.TB, w ui.Widget) float32 {
	t.Helper()
	l, ok := w.(*kit.Label)
	require.True(t, ok, "a label")

	return l.TextLayout().Size.W
}

// frameOfText returns the window frame of the first widget showing s.
func frameOfText(t testing.TB, r *shelltest.Rig, s string) geom.Rect {
	t.Helper()
	w, _, ok := r.Tester.Find(s)
	require.Truef(t, ok, "no widget shows %q", s)

	return w.Node().WindowFrame()
}

// TestVideoTopRowKeepsTheCourseBetweenTheButtons is the top row of screen 06 with a
// course name that does not fit: the eyebrow starts right of the back button and
// ends left of the "more" button, each with a gap, ends in "…", stays centred, the
// whole name is still its accessibility label, and the "more" button still takes
// its tap.
func TestVideoTopRowKeepsTheCourseBetweenTheButtons(t *testing.T) {
	t.Parallel()

	kittest.ForEachPhone(t, func(t *testing.T, ph kittest.Phone) {
		r, pb, _ := atVideo(t, ph, true)
		withLongCourseLesson(r, pb, false)
		r.Settle()
		r.Tester.Elapse(kittest.Frame)

		w, _, ok := r.Tester.Find(longTopCourse)
		require.True(t, ok, "the eyebrow, whole in its accessibility label")
		eb := w.Node().WindowFrame()
		assert.LessOrEqual(t, drawnWidth(t, w), eb.W+.5, "the text drawn is cut to its box, not drawn past it")
		back := named(r.Shell, "back", 1).Node().WindowFrame()
		more := named(r.Shell, optionsOpen, 1).Node().WindowFrame()
		assert.GreaterOrEqual(t, eb.X, back.X+back.W+topRowGap, "starts right of the back button")
		assert.LessOrEqual(t, eb.X+eb.W, more.X-topRowGap, "ends left of the more button")
		assert.InDelta(t, (back.X+back.W+more.X)/2, eb.X+eb.W/2, 1, "centred between the buttons")
		for _, e := range r.Dump("long").Elements {
			if strings.HasPrefix(e.Key, "div.eb") {
				assert.LessOrEqual(t, e.X+e.W, float64(more.X)-topRowGap+.5, "the dump's box too")
			}
		}

		// the button next to the long text still takes its tap
		tap(r, named(r.Shell, optionsOpen, 1))
		r.Tester.Elapse(300 * kittest.Frame)
		assert.True(t, r.Shell.Nav().Sheet().IsOpen(), "the more button opens the options")
	})
}

// TestAudioTopRowKeepsTheCourseBetweenTheButtons is the same for screen 08, whose
// course name is the bold line under "ĐANG PHÁT TỪ".
func TestAudioTopRowKeepsTheCourseBetweenTheButtons(t *testing.T) {
	t.Parallel()

	kittest.ForEachPhone(t, func(t *testing.T, ph kittest.Phone) {
		r, pb, _ := atAudio(t, ph, true)
		withLongCourseLesson(r, pb, true)
		r.Settle()
		r.Tester.Elapse(kittest.Frame)

		nw, _, _ := r.Tester.Find(longTopCourse)
		require.NotNil(t, nw)
		if l, ok := nw.(*kit.Label); ok {
			assert.LessOrEqual(t, drawnWidth(t, l), l.Node().WindowFrame().W+.5, "the text drawn is cut to its box")
		}
		name := frameOfText(t, r, longTopCourse)
		back := named(r.Shell, "back", 1).Node().WindowFrame()
		more := named(r.Shell, "toast", 1).Node().WindowFrame()
		assert.GreaterOrEqual(t, name.X, back.X+back.W+topRowGap, "starts right of the back button")
		assert.LessOrEqual(t, name.X+name.W, more.X-topRowGap, "ends left of the more button")
		assert.InDelta(t, (back.X+back.W+more.X)/2, name.X+name.W/2, 1, "centred between the buttons")
	})
}
