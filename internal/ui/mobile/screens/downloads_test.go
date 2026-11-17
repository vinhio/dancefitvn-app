//go:build !tvos

package screens_test

import (
	"testing"
	"time"

	"dancefitvn-app/internal/ui/kit"
	"dancefitvn-app/internal/ui/kit/kittest"
	"dancefitvn-app/internal/ui/kit/layoutdump"
	"dancefitvn-app/internal/ui/mobile/env"
	"dancefitvn-app/internal/ui/mobile/screens"
	"dancefitvn-app/internal/ui/mobile/shell/shelltest"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ====================================================================
// ============================ Contracts =============================
// ====================================================================

// shows runs the toast in (a frame) and tells whether msg shows.
func shows(r *shelltest.Rig, msg string) bool {
	r.Tester.Elapse(100 * time.Millisecond)

	return r.Tester.Shows(msg)
}

// ====================================================================
// ========================== Main functions ==========================
// ====================================================================

// TestDownloadsBoxesMatchPrototype lays screen 11 out with the iPhone's real text
// engine over the demo downloads and compares every element of M11 with the
// prototype's within 1 DIP: the title, the summary, the card of the space with its
// bar, the card of the download under way (poster, bar, text, cross), the group's
// eyebrow, the five rows with their posters, sizes and buttons, the tab bar. The
// text of the free space and the bar's fill are what the device says (K11); in the
// demo they equal the prototype's.
func TestDownloadsBoxesMatchPrototype(t *testing.T) {
	t.Parallel()

	r := atDownloads(t, kittest.Phones[0], true)
	lay := r.Dump("M11")
	// the value's box is the width of "Trống 18,2 GB" as Core Text sets it, 0,2 less
	skip := map[string]bool{"span.muted": true}
	checkAgainstProto(t, "M11", lay, 1, skip)
	want := shelltest.ProtoLayout(t, "M11")
	w, _ := want.Find("span.muted")
	g, ok := lay.Find("span.muted")
	require.True(t, ok)
	assert.InDelta(t, w.X, g.X, .5)
	assert.InDelta(t, w.W, g.W, .5)
	assert.InDelta(t, w.Y, g.Y, .01)
	assert.InDelta(t, w.H, g.H, .01)
}

// TestDownloadsEmptyBoxesMatchPrototype compares screen 12 with M12 within 1 DIP:
// the title, the dashed disc's headphones, the title, the two lines, the button
// "Xem khoá học", the tab bar.
func TestDownloadsEmptyBoxesMatchPrototype(t *testing.T) {
	t.Parallel()

	r := atDownloads(t, kittest.Phones[0], true, emptied)
	d := downloadsOf(t, r)
	require.True(t, d.Empty())
	checkAgainstProto(t, "M12", r.Dump("M12"), 1, nil)
}

// TestDownloadsOnTheWiderPhone checks the 412 wide phone's screens 11 and 12
// against the iPhone's: the same column from the top, what runs the width 19
// wider, what sits at the right edge moved by the 19, what is centered by 9,5.
func TestDownloadsOnTheWiderPhone(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		id     string
		setups []func(*shelltest.FakeDownloads)
		full   []string
		right  []string
	}{
		{id: "M11", full: []string{"div.h1", "p.muted", "div.card", "div.card#2", "playAudio:set35-1", "playAudio:set35-6", "div.eb#2"},
			right: []string{"dlTap:set35-7", "askDelete:set35-1", "askDelete:set35-6", "span.val", "span.val#5", "span.muted"}},
		{id: "M12", setups: []func(*shelltest.FakeDownloads){emptied}, full: []string{"div.h1", "div.d", "p.muted"}},
	} {
		a := atDownloads(t, kittest.Phones[0], true, tc.setups...).Dump(tc.id)
		b := atDownloads(t, kittest.Phones[1], true, tc.setups...).Dump(tc.id)
		for _, ea := range a.Elements {
			if ea.Y >= 760 || ea.Key == "div#m" || ea.Key == "div.scr" || ea.Key == "div.scroll" { // the tab bar, on the bottom of the taller screen; the frames
				continue
			}
			eb, ok := b.Find(ea.Key)
			require.True(t, ok, tc.id+" "+ea.Key)
			assert.InDelta(t, ea.Y, eb.Y, 1.5, tc.id+" "+ea.Key+": y")
			assert.InDelta(t, ea.H, eb.H, 1.5, tc.id+" "+ea.Key+": h")
		}
		for _, k := range tc.full {
			e, ok := b.Find(k)
			require.True(t, ok, k)
			assert.InDelta(t, 20, e.X, .01, k+": x")
			assert.InDelta(t, 372, e.W, .01, k+": width")
		}
		if tc.id == "M11" { // the bar is inside the card's 14 padding
			e, _ := b.Find("div.prog")
			assert.InDelta(t, 34, e.X, .01, "div.prog: x")
			assert.InDelta(t, 344, e.W, .01, "div.prog: width")
		}
		for _, k := range tc.right {
			ea, _ := a.Find(k)
			eb, ok := b.Find(k)
			require.True(t, ok, k)
			assert.InDelta(t, ea.X+19, eb.X, 2, k+": x moves with the edge") // 2: gotext sets the text 1,2 wider than Core Text
		}
	}
}

// TestDownloadsShowsTheDemo checks what screen 11 says of the demo state: the
// summary, the space, the running count and the group's eyebrow, the rows in the
// order of the downloads, on both phones.
func TestDownloadsShowsTheDemo(t *testing.T) {
	t.Parallel()

	kittest.ForEachPhone(t, func(t *testing.T, ph kittest.Phone) {
		r := atDownloads(t, ph, false)
		d := downloadsOf(t, r)
		assert.Equal(t, 5, d.Files())
		assert.Equal(t, 1, d.Running())
		assert.False(t, d.Empty())
		assert.Equal(t, 2, d.Tab())
		for _, s := range []string{
			"5 file âm thanh · 48,4 MB · nghe khi offline", "Trống 18,2 GB", "07 · Viva La Vida", "6,3 / 9,8 MB · 64%",
			"01 · DanceFIT Warm Up 13", "12,8 MB", "03 · Stink and Dutty",
		} {
			assert.True(t, r.Tester.Shows(s), s)
		}
		assert.NotNil(t, named(r.Shell, "dlTap:set35-7", 1))
		for _, k := range []string{"playAudio:set35-1", "playAudio:set35-2", "playAudio:set35-3", "playAudio:set35-4", "playAudio:set35-6"} {
			assert.NotNil(t, named(r.Shell, k, 1), k)
		}
		assert.Nil(t, named(r.Shell, "playAudio:set35-7", 1), "a download under way has no row of a file")
		e, ok := r.Dump("M11").Find("div.eb")
		require.True(t, ok)
		assert.Equal(t, "ĐANG TẢI · 1", e.Text)
		e, ok = r.Dump("M11").Find("div.eb#2")
		require.True(t, ok)
		assert.Equal(t, "DANCEFIT SET 35 · 5/10 BÀI", e.Text)
	})
}

// TestDownloadsProgressEventMovesTheCard checks that a progress event moves the bar
// and the text of the running card and builds nothing: the card is the same widget
// after it.
func TestDownloadsProgressEventMovesTheCard(t *testing.T) {
	t.Parallel()

	kittest.ForEachPhone(t, func(t *testing.T, ph kittest.Phone) {
		r := atDownloads(t, ph, true)
		before := named(r.Shell, "dlTap:set35-7", 1)
		bar := func() layoutdump.Element {
			e, ok := r.Dump("M11").Find("i#2")
			require.True(t, ok)

			return e
		}
		w0 := bar().W
		assert.InDelta(t, .64*(ph.Size.W-40-24-44-12-12-40), w0, .6)

		r.Fake.Dl().SetDownload("l_3507", kit.DlIng, .5)
		r.Settle()
		assert.InDelta(t, w0*.5/.64, bar().W, .6, "the bar moves to 50 %")
		assert.True(t, r.Tester.Shows("4,9 / 9,8 MB · 50%"))
		assert.False(t, r.Tester.Shows("6,3 / 9,8 MB · 64%"))
		assert.Same(t, before, named(r.Shell, "dlTap:set35-7", 1), "nothing is built")
	})
}

// TestDownloadsTaps checks the taps of screen 11: the cross of a running card is
// `dlTap` (forgotten, the toast, the card gone), a row plays the audio, the button
// of a row opens sheet 14.
func TestDownloadsTaps(t *testing.T) {
	t.Parallel()

	kittest.ForEachPhone(t, func(t *testing.T, ph kittest.Phone) {
		t.Run("cancel", func(t *testing.T) {
			r := atDownloads(t, ph, false)
			tap(r, named(r.Shell, "dlTap:set35-7", 1))
			r.Settle()
			assert.Equal(t, []string{"l_3507"}, r.Fake.Dl().Cancelled())
			assert.True(t, shows(r, toastCanceled))
			d := downloadsOf(t, r)
			assert.Equal(t, 0, d.Running())
			assert.Equal(t, 5, d.Files())
			assert.False(t, r.Tester.Shows("07 · Viva La Vida"))
		})
		t.Run("play", func(t *testing.T) {
			r := atDownloads(t, ph, false)
			tap(r, named(r.Shell, "playAudio:set35-3", 1))
			r.Settle()
			assert.Equal(t, env.RouteAudio, r.Route())
			assert.Contains(t, r.Fake.FakePlayback().Calls(), "PlayAudio:l_3503")
		})
		t.Run("options", func(t *testing.T) {
			r := atDownloads(t, ph, false)
			tap(r, named(r.Shell, "askDelete:set35-3", 1))
			r.Settle()
			require.True(t, r.Shell.Nav().Sheet().IsOpen())
			s, ok := r.Shell.Nav().Sheet().Content().(*screens.DeleteSheet)
			require.True(t, ok, "sheet 14")
			assert.Equal(t, env.DeleteSheetParams{CourseID: "set-35", LessonID: "l_3503", No: 3}, s.Params)
			assert.Equal(t, env.RouteDownloads, r.Route(), "a tap on the button is not a tap on the row")
		})
		t.Run("delete from the sheet", func(t *testing.T) {
			r := atDownloads(t, ph, false)
			tap(r, named(r.Shell, "askDelete:set35-3", 1))
			r.Settle()
			slideIn(r)
			tap(r, named(r.Shell, "delConfirm", 1))
			r.Settle()
			assert.Equal(t, []string{"l_3503"}, r.Fake.Dl().Deleted())
			assert.False(t, r.Shell.Nav().Sheet().IsOpen())
			assert.True(t, shows(r, toastDeleted))
			d := downloadsOf(t, r)
			assert.Equal(t, 4, d.Files(), "the row is gone")
			assert.True(t, r.Tester.Shows("4 file âm thanh · 39,6 MB · nghe khi offline"))
			assert.Nil(t, named(r.Shell, "playAudio:set35-3", 1))
		})
	})
}

// TestDownloadsFollowsTheDownloads checks that the lists are made again on the
// events that change them: a download that finishes moves from the cards to the
// rows, a new one adds a card, the last one deleted leaves screen 12.
func TestDownloadsFollowsTheDownloads(t *testing.T) {
	t.Parallel()

	kittest.ForEachPhone(t, func(t *testing.T, ph kittest.Phone) {
		r := atDownloads(t, ph, false)
		dl := r.Fake.Dl()
		d := downloadsOf(t, r)

		dl.FinishDownload("l_3507")
		r.Settle()
		assert.Equal(t, 0, d.Running())
		assert.Equal(t, 6, d.Files())
		assert.True(t, r.Tester.Shows("07 · Viva La Vida"))
		assert.NotNil(t, named(r.Shell, "playAudio:set35-7", 1))
		assert.True(t, shows(r, "Đã tải xong bài 07"), "the Shell's toast")

		dl.SetDownload("l_3505", kit.DlIng, .1)
		r.Settle()
		assert.Equal(t, 1, d.Running())
		assert.NotNil(t, named(r.Shell, "dlTap:set35-5", 1))
		assert.True(t, r.Tester.Shows("0,8 / 8,4 MB · 10%"))

		dl.Clear()
		r.Settle()
		assert.True(t, d.Empty(), "screen 12")
		assert.True(t, r.Tester.Shows("Chưa có file âm thanh"))
		assert.False(t, r.Tester.Shows("Bộ nhớ máy"))

		dl.SetDownload("l_3501", kit.DlDone, 1)
		r.Settle()
		assert.False(t, d.Empty(), "a file brings the list back")
		assert.True(t, r.Tester.Shows("1 file âm thanh · 12,8 MB · nghe khi offline"))
	})
}

// TestDownloadsRefreshesWhenReturnedTo checks that a screen 11 that was left (it is
// under the player, unmounted, deaf to the events) shows what changed meanwhile when
// Back returns to it.
func TestDownloadsRefreshesWhenReturnedTo(t *testing.T) {
	t.Parallel()

	kittest.ForEachPhone(t, func(t *testing.T, ph kittest.Phone) {
		r := atDownloads(t, ph, false)
		tap(r, named(r.Shell, "playAudio:set35-3", 1))
		r.Settle()
		require.Equal(t, env.RouteAudio, r.Route())
		r.Fake.Dl().FinishDownload("l_3507")
		r.Fake.Dl().SetDownload("l_3501", kit.DlNone, 0)
		r.Settle()
		r.Shell.Back()
		r.Settle()
		require.Equal(t, env.RouteDownloads, r.Route())
		d := downloadsOf(t, r)
		assert.Equal(t, 0, d.Running())
		assert.Equal(t, 5, d.Files(), "lesson 7 in, lesson 1 out")
		assert.NotNil(t, named(r.Shell, "playAudio:set35-7", 1))
		assert.Nil(t, named(r.Shell, "playAudio:set35-1", 1))
	})
}

// TestDownloadsFailedJobIsForgotten checks the Failed event the Shell answers: the
// toast, the job forgotten, its card gone from screen 11.
func TestDownloadsFailedJobIsForgotten(t *testing.T) {
	t.Parallel()

	kittest.ForEachPhone(t, func(t *testing.T, ph kittest.Phone) {
		r := atDownloads(t, ph, false)
		r.Fake.Dl().FailDownload("l_3507", env.ErrChecksum)
		r.Settle()
		assert.True(t, shows(r, toastFailed))
		assert.Equal(t, 0, downloadsOf(t, r).Running())
		assert.Equal(t, []string{"l_3507"}, r.Fake.Dl().Cancelled())
	})
}

// TestDownloadsStorageCard checks the card of the device's space: the free space
// read again moves the text and the bar (the share used), an unknown space shows
// no text and an empty bar (Q-P5-9).
func TestDownloadsStorageCard(t *testing.T) {
	t.Parallel()

	kittest.ForEachPhone(t, func(t *testing.T, ph kittest.Phone) {
		r := atDownloads(t, ph, true)
		fill := func() (float64, bool) {
			e, ok := r.Dump("M11").Find("i")

			return e.W, ok
		}
		w, ok := fill()
		require.True(t, ok)
		track, _ := r.Dump("M11").Find("div.prog")
		assert.InDelta(t, .71*track.W, w, .5, "71 % used")

		r.Fake.Dl().SetSpace(32_000_000_000, 64_000_000_000)
		r.Settle()
		assert.True(t, r.Tester.Shows("Trống 32,0 GB"))
		w, _ = fill()
		assert.InDelta(t, .5*track.W, w, .5)

		r.Fake.Dl().ClearSpace()
		r.Settle()
		assert.False(t, r.Tester.Shows("Trống 32,0 GB"))
		assert.False(t, r.Tester.Shows("Trống 18,2 GB"))
		_, ok = r.Dump("M11").Find("i#2")
		assert.False(t, ok, "the storage bar has no fill: the only fill left is the running card's")
		assert.True(t, r.Tester.Shows("Bộ nhớ máy"), "the card stays")
	})
}

// TestDownloadsAsksForTheSpaceOnShow checks that the screen asks for the free
// space to be read again when it opens.
func TestDownloadsAsksForTheSpaceOnShow(t *testing.T) {
	t.Parallel()

	r := atDownloads(t, kittest.Phones[0], false)
	_, _, refreshes := r.Fake.Dl().Calls()
	assert.GreaterOrEqual(t, refreshes, 1)
}

// TestDownloadsEmptyButtonOpensCourses checks the button of screen 12: `go.courses`
// opens the list of courses as the root of its tab.
func TestDownloadsEmptyButtonOpensCourses(t *testing.T) {
	t.Parallel()

	kittest.ForEachPhone(t, func(t *testing.T, ph kittest.Phone) {
		r := atDownloads(t, ph, false, emptied)
		for _, s := range []string{
			"Chưa có file âm thanh", "Tải file âm thanh của bài học để tập luyện\nvà dạy lớp cả khi không có mạng.", "Xem khoá học",
		} {
			assert.True(t, r.Tester.Shows(s), s)
		}
		tap(r, named(r.Shell, "go.courses", 1))
		r.Settle()
		assert.Equal(t, env.RouteCourses, r.Route())
		assert.Equal(t, 0, r.Shell.Nav().Depth(), "a root")
	})
}

// TestDownloadsSteadyFrameDoesNotAllocate checks that a frame of screen 11 standing
// still allocates nothing, with the card of the space, a running card and the rows
// on it, and that a frame of screen 12 does not either.
func TestDownloadsSteadyFrameDoesNotAllocate(t *testing.T) { //nolint:paralleltest // AllocsPerRun needs the process to itself
	if kittest.Race {
		t.Skip("allocation counts are not stable under the race detector")
	}
	for _, ph := range kittest.Phones {
		for name, setups := range map[string][]func(*shelltest.FakeDownloads){"list": nil, "empty": {emptied}} {
			r := atDownloads(t, ph, false, setups...)
			warmDownloads(r)
			assert.Zero(t, testing.AllocsPerRun(50, r.Tester.Repaint), ph.Name+" "+name)
		}
	}
}

// TestDownloadProgressEventDoesNotAllocate checks that a progress event that does
// not change what the card shows (the same bytes, or bytes that make the same text
// and the same bar) allocates nothing, with the screen showing.
func TestDownloadProgressEventDoesNotAllocate(t *testing.T) { //nolint:paralleltest // AllocsPerRun needs the process to itself
	if kittest.Race {
		t.Skip("allocation counts are not stable under the race detector")
	}
	for _, ph := range kittest.Phones {
		r := atDownloads(t, ph, false)
		warmDownloads(r)
		ev := env.DownloadEvent{Kind: env.DownloadEventProgress, LessonID: "l_3507", No: 7, Done: 6_272_000, Total: 9_800_000}
		topic := r.Fake.Dl().Events()
		topic.Publish(ev)
		r.Tester.Repaint()
		assert.Zero(t, testing.AllocsPerRun(100, func() { topic.Publish(ev) }), ph.Name+": same bytes")
		ev.Done++ // 6,3 MB and 64 % still
		assert.Zero(t, testing.AllocsPerRun(100, func() { topic.Publish(ev) }), ph.Name+": same text")
	}
}

// BenchmarkDownloadsSteadyFrame is a frame of screen 11 standing still.
func BenchmarkDownloadsSteadyFrame(b *testing.B) {
	r := atDownloads(b, kittest.Phones[0], false)
	warmDownloads(r)
	b.ReportAllocs()
	for b.Loop() {
		r.Tester.Repaint()
	}
}

// BenchmarkDownloadProgressEvent is a progress event of the running card whose text
// does not change.
func BenchmarkDownloadProgressEvent(b *testing.B) {
	r := atDownloads(b, kittest.Phones[0], false)
	warmDownloads(r)
	ev := env.DownloadEvent{Kind: env.DownloadEventProgress, LessonID: "l_3507", No: 7, Done: 6_272_000, Total: 9_800_000}
	topic := r.Fake.Dl().Events()
	b.ReportAllocs()
	for b.Loop() {
		topic.Publish(ev)
	}
}

// ====================================================================
// ========================= Helper Functions =========================
// ====================================================================

// warmDownloads lets the frames run until what screen 11 draws is laid out and
// shaped once.
func warmDownloads(r *shelltest.Rig) {
	for range 8 {
		r.Tester.Elapse(kittest.Frame)
	}
	r.Tester.Repaint()
}
