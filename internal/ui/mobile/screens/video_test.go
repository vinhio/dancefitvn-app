//go:build !tvos

package screens_test

import (
	"testing"
	"time"

	"dancefitvn-app/internal/player"
	"dancefitvn-app/internal/ui/kit/kittest"
	"dancefitvn-app/internal/ui/mobile/env"
	"dancefitvn-app/internal/ui/mobile/screens"
	"dancefitvn-app/internal/ui/mobile/shell/shelltest"
	"dancefitvn-app/locales"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ====================================================================
// ============================ Contracts =============================
// ====================================================================

// optionsOpen is the "more" button of the top row, which opens the options sheet.
const optionsOpen = "sheet:options"

// ====================================================================
// ========================== Main functions ==========================
// ====================================================================

// TestVideoBoxesMatchPrototype lays screen 06 out with the iPhone's real text
// engine and compares every element of M06, M06-playing and M06-hidden with the
// prototype's within 1 DIP: the back and "more" buttons, the course line, the audio
// file's side button with its icon (the speed, mirror and loop buttons are gone,
// K38), the chips, the title, the transport row. The
// picture is the real video (K5) and is not compared; the controls hidden keep
// their boxes (F-11).
func TestVideoBoxesMatchPrototype(t *testing.T) {
	t.Parallel()

	r, pb, v := atVideo(t, kittest.Phones[0], true)
	pb.Pause()
	r.Settle()
	checkAgainstProtoWithout(t, "M06", r.Dump("M06"), 1, nil, videoSideDrop...)

	restart(r, pb)
	r.Tester.Elapse(kittest.Frame)
	checkAgainstProtoWithout(t, "M06-playing", r.Dump("M06-playing"), 1, nil, videoSideDrop...)
	assert.True(t, v.Shown())

	r.Tester.Elapse(ms3s + 300*time.Millisecond)
	require.False(t, v.Shown(), "hidden")
	assert.Zero(t, v.Opacity())
	checkAgainstProtoWithout(t, "M06-hidden", r.Dump("M06-hidden"), 1, nil, videoSideDrop...)
}

// TestVideoOnTheWiderPhone checks the 412 wide phone against the iPhone's: the top
// row and the side column keep their y and follow the right edge, the panel stays on
// the bottom of the taller screen (the same 34 under it), the stage fills the window.
func TestVideoOnTheWiderPhone(t *testing.T) {
	t.Parallel()

	rI, pbI, _ := atVideo(t, kittest.Phones[0], true)
	rA, pbA, _ := atVideo(t, kittest.Phones[1], true)
	pbI.Pause()
	pbA.Pause()
	a, b := rI.Dump("M06"), rA.Dump("M06")
	require.Len(t, b.Elements, len(a.Elements))
	right := map[string]bool{"sheet:options": true, "svg.i#2": true, "dlTap:set35-3": true, "svg.i#3": true}
	for i, ea := range a.Elements {
		eb := b.Elements[i]
		require.Equal(t, ea.Key, eb.Key)
		switch ea.Key {
		case "div#m", "stageTap", "video":
			assert.InDelta(t, 412, eb.W, .01, ea.Key)
			assert.InDelta(t, 892, eb.H, .01, ea.Key)
		case "span.chip.y", "span.chip.k", "div.d", "div.beats", "scrub", "prevLes", "seek:-10", "svg.i#4", "togglePlay", "seek:10", "svg.i#5", "nextLes":
			assert.InDelta(t, ea.Y+40, eb.Y, .6, ea.Key+": y sits 40 lower, as far from the bottom")
		default:
			assert.InDelta(t, ea.Y, eb.Y, .6, ea.Key+": y")
		}
		if right[ea.Key] {
			assert.InDelta(t, ea.X+19, eb.X, .6, ea.Key+": x follows the right edge")
		}
	}
	d, _ := b.Find("div.d")
	assert.InDelta(t, 372, d.W, .01, "the title spans the 20 DIP margins")
	tp, _ := b.Find("togglePlay")
	assert.InDelta(t, (412-58)/2, tp.X, .6, "the play button is in the middle")
	assert.InDelta(t, 892-34-58, tp.Y, .01, "34 above the bottom")
}

// TestVideoLessonLine checks the texts of the lesson: "Bài 03 / 10" under the course,
// the tag chip, the title and the course line, the prev and next numbers, on both
// phones; and that another lesson changes them.
func TestVideoLessonLine(t *testing.T) {
	t.Parallel()

	kittest.ForEachPhone(t, func(t *testing.T, ph kittest.Phone) {
		r, pb, _ := atVideo(t, ph, false)
		assert.True(t, r.Tester.Shows("Bài 03 / 10"))
		assert.True(t, r.Tester.Shows("DanceFIT Set 35 · “Cùng nhảy cùng hát”"), "the course line")
		assert.True(t, r.Tester.Shows("‹ 02"))
		assert.True(t, r.Tester.Shows("04 ›"))
		assert.True(t, r.Tester.Shows("Video dọc 9:16"))
		assert.False(t, r.Tester.Shows("0.75x"), "the speed is in the options sheet only (K38)")
		assert.True(t, r.Tester.Shows("Đã tải"), "the audio file of lesson 3 is on the device")

		require.NoError(t, r.Tester.Click("‹ 02"))
		r.Settle()
		assert.Equal(t, "Prev", pb.Calls()[len(pb.Calls())-1])
		assert.True(t, r.Tester.Shows("Bài 02 / 10"))
		assert.True(t, r.Tester.Shows("‹ 01"))
		assert.True(t, r.Tester.Shows("03 ›"))
		require.NoError(t, r.Tester.Click("‹ 01"))
		assert.True(t, r.Tester.Shows("Bài 01 / 10"))
		assert.True(t, r.Tester.Shows("‹ 01"), "lesson 1: the prev button still reads 01, and does nothing")
		pb.ClearCalls()
		require.NoError(t, r.Tester.Click("‹ 01"))
		assert.Empty(t, pb.Calls(), "no lesson before the first")
		assert.Equal(t, env.RouteVideo, r.Route())
	})
}

// TestVideoLastLesson checks that on lesson 10 the next button reads 10 and does
// nothing.
func TestVideoLastLesson(t *testing.T) {
	t.Parallel()

	kittest.ForEachPhone(t, func(t *testing.T, ph kittest.Phone) {
		r, pb, _ := atVideo(t, ph, false)
		for range 7 {
			pb.Next()
		}
		r.Settle()
		require.Equal(t, 10, pb.Lesson().No)
		assert.True(t, r.Tester.Shows("Bài 10 / 10"))
		assert.True(t, r.Tester.Shows("10 ›"))
		pb.ClearCalls()
		require.NoError(t, r.Tester.Click("10 ›"))
		assert.Empty(t, pb.Calls())
	})
}

// TestVideoBackLeavesThePlayer checks that Back (the button, and the Android key
// through the Nav) pauses and saves through Leave, once, and shows the screen under.
func TestVideoBackLeavesThePlayer(t *testing.T) {
	t.Parallel()

	kittest.ForEachPhone(t, func(t *testing.T, ph kittest.Phone) {
		r, pb, _ := atVideo(t, ph, false)
		require.NoError(t, r.Tester.Click("Quay lại"))
		assert.Equal(t, "detail", r.Route())
		assert.Equal(t, 1, pb.Count("Leave"))
		assert.False(t, pb.Playing(), "a video pauses when it is left")
		assert.Equal(t, player.StatePaused, pb.State())
	})
}

// noPlayer is Services without a player: Playback is nil and Prefs are the
// defaults.
type noPlayer struct{ env.Services }

// Playback is nil: no player.
func (noPlayer) Playback() env.Playback { return nil }

func (noPlayer) WatchedLocal(string) bool { return false }

// Prefs are nil: Env keeps the defaults in memory.
func (noPlayer) Prefs() env.Prefs { return nil }

// TestVideoWithoutAPlayerIsTheStub checks that services with no player give the
// stub with a back button, so that the route is never a blank.
func TestVideoWithoutAPlayerIsTheStub(t *testing.T) {
	t.Parallel()

	f, err := shelltest.New(shelltest.MockFS(t))
	require.NoError(t, err)
	e := env.New(noPlayer{f}, locales.MustTexts())
	e.Act = &recorder{}
	require.Nil(t, e.Playback())
	_, ok := screens.NewVideo(e).(*screens.Stub)
	assert.True(t, ok)
}
