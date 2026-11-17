//go:build !tvos

// Pattern from detail_test.go (the rig, the proto comparison, the steady frame)
// and downloads_test.go (the Fake's downloads and their events).

package screens_test

import (
	"testing"
	"time"

	"dancefitvn-app/internal/ui/kit"
	"dancefitvn-app/internal/ui/kit/kittest"
	"dancefitvn-app/internal/ui/mobile/env"
	"dancefitvn-app/internal/ui/mobile/screens"
	"dancefitvn-app/internal/ui/mobile/shell/shelltest"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// atOffline boots a signed-in app, reads Set 35 (so that the course is cached), and
// cuts the network: the shell puts the course's offline screen in the detail's
// place (03 section 2). real asks for the phone's own text engine.
func atOffline(tb testing.TB, ph kittest.Phone, real bool) *shelltest.Rig {
	tb.Helper()
	r := atDetail(tb, ph, real, "set-35")
	r.Fake.SetOnline(false)
	r.Tester.Elapse(kittest.Frame)
	r.Settle()
	require.Equal(tb, env.RouteOffline, r.Route())
	r.Tester.Elapse(kittest.Frame)

	return r
}

// atOfflineCold cuts the network first and opens the course: no screen read it in
// this run, so the offline screen has only the downloaded lessons to show (or the
// client's cache).
func atOfflineCold(tb testing.TB, ph kittest.Phone, setups ...func(*shelltest.FakeDownloads)) *shelltest.Rig {
	tb.Helper()
	f, err := shelltest.New(shelltest.MockFS(tb))
	require.NoError(tb, err)
	require.NoError(tb, f.SeedSession())
	for _, s := range setups {
		s(f.Dl())
	}
	r := shelltest.MountRig(tb, ph, false, f, shellOptions())
	r.Boot()
	require.Equal(tb, "home", r.Route())
	f.SetOnline(false)
	r.Tester.Elapse(kittest.Frame)
	r.Shell.Detail("set-35")
	r.Tester.Elapse(kittest.Frame)
	r.Settle()
	require.Equal(tb, env.RouteOffline, r.Route())

	return r
}

// offlineOf returns screen 13 showing.
func offlineOf(tb testing.TB, r *shelltest.Rig) *screens.Offline {
	tb.Helper()
	o, ok := r.Shell.Nav().Screen().(*screens.Offline)
	require.True(tb, ok, "the screen showing is Offline")

	return o
}

// TestOfflineBoxesMatchPrototype lays screen 13 of Set 35 out with the iPhone's text
// engine and compares every element of M13 with the prototype's within 1 DIP: the
// offline bar and its icon, the back button, the name, the hint and the ten rows
// (the two watched, the current one 8 outside the column, the dimmed ones with the
// cloud, the downloaded ones with the play disc).
func TestOfflineBoxesMatchPrototype(t *testing.T) {
	t.Parallel()

	r := atOffline(t, kittest.Phones[0], true)
	checkAgainstProto(t, "M13", r.Dump("M13"), 1, nil)
}

// TestOfflineOnTheWiderPhone checks that the 412 wide phone's screen 13 is the
// iPhone's with the width 19 wider: the same y everywhere, the bar and the rows as
// wide as the screen allows, the disc at the right edge.
func TestOfflineOnTheWiderPhone(t *testing.T) {
	t.Parallel()

	a, b := atOffline(t, kittest.Phones[0], true).Dump("M13"), atOffline(t, kittest.Phones[1], true).Dump("M13")
	require.Len(t, b.Elements, len(a.Elements))
	for i, ea := range a.Elements {
		eb := b.Elements[i]
		require.Equal(t, ea.Key, eb.Key)
		assert.InDelta(t, ea.Y, eb.Y, 1.5, ea.Key+": y")
		if ea.Key != "div#m" && ea.Key != "div.scr" && ea.Key != "div.scroll" { // the frame is taller
			assert.InDelta(t, ea.H, eb.H, 1.5, ea.Key+": h")
		}
	}
	bar, _ := b.Find("div.offbar")
	assert.InDelta(t, 412, bar.W, 0.01)
	row, _ := b.Find("offTap:set35-1")
	assert.InDelta(t, 372, row.W, 0.01)
	disc, _ := a.Find("playAudio:set35-1")
	discB, _ := b.Find("playAudio:set35-1")
	assert.InDelta(t, disc.X+19, discB.X, 1.5, "the disc moves with the right edge")
}

// TestOfflineRows checks the states of the rows (index.html:382-393): 01 and 02
// watched, 03 current, the files of 01 to 04 and 06 with "Audio đã tải" and the play
// disc, the others with "Cần có mạng", the cloud and the .38 dim; the texts of the
// screen; no tab bar.
func TestOfflineRows(t *testing.T) {
	t.Parallel()

	kittest.ForEachPhone(t, func(t *testing.T, ph kittest.Phone) {
		r := atOffline(t, ph, false)
		for _, s := range []string{
			"Không có mạng · Đang dùng chế độ ngoại tuyến", "DanceFIT Set 35",
			"Video cần mạng · chỉ nghe được file âm thanh đã tải",
			"Warm Up · Audio đã tải", "Bachata · Audio đã tải",
		} {
			assert.True(t, r.Tester.Shows(s), s)
		}
		assert.False(t, r.Tester.Shows("Tiếp tục bài 03"))
		assert.True(t, r.Shell.Nav().TabBar().Hidden, "no tab bar")
		rows := offlineOf(t, r).Rows()
		require.Len(t, rows, 10)
		assert.True(t, r.Tester.Shows(rows[4].Meta), "Cần có mạng")
		assert.Contains(t, rows[4].Meta, "Cần có mạng")
		done := map[int]bool{1: true, 2: true, 3: true, 4: true, 6: true}
		for i, row := range rows {
			no := i + 1
			assert.Equal(t, kit.LessonOffline, row.Mode)
			assert.Equal(t, done[no], row.Download == kit.DlDone, "row %d file", no)
			assert.Equal(t, !done[no], row.Dim, "row %d dim", no)
		}
		assert.Equal(t, kit.LessonWatched, rows[0].State)
		assert.Equal(t, kit.LessonWatched, rows[1].State)
		assert.Equal(t, kit.LessonCurrent, rows[2].State)
		assert.Equal(t, kit.LessonNormal, rows[3].State)
	})
}

// TestOfflineTaps checks `offTap` and `playAudio` (03 section 2): a row with its file
// plays the audio and opens 08, the disc of a row does the same; a row without a
// file toasts "Video bài 05 cần có mạng để xem" for 2200 ms and plays nothing; Back
// leaves for the screen below.
func TestOfflineTaps(t *testing.T) {
	t.Parallel()

	kittest.ForEachPhone(t, func(t *testing.T, ph kittest.Phone) {
		r := atOffline(t, ph, false)
		pb := r.Fake.FakePlayback()
		require.NotNil(t, pb)
		root := r.Tester.View().Root()

		tap(r, named(root, "offTap:set35-5", 1))
		r.Tester.Elapse(100 * time.Millisecond)
		assert.True(t, r.Tester.Shows("Video bài 05 cần có mạng để xem"))
		assert.Equal(t, env.RouteOffline, r.Route())
		assert.Zero(t, pb.Count("PlayAudio")+pb.Count("PlayVideo"))
		r.Tester.Elapse(2200 * time.Millisecond)
		assert.False(t, r.Tester.Shows("Video bài 05 cần có mạng để xem"), "the toast ends after 2200 ms")

		tap(r, named(root, "offTap:set35-3", 1))
		r.Settle()
		assert.Equal(t, env.RouteAudio, r.Route(), "a file plays")
		assert.Equal(t, 1, pb.Count("PlayAudio"))
		assert.Contains(t, pb.Calls(), "PlayAudio:l_3503")

		r.Shell.Back()
		r.Settle()
		require.Equal(t, env.RouteOffline, r.Route())
		tap(r, named(r.Tester.View().Root(), "playAudio:set35-1", 1))
		r.Settle()
		assert.Equal(t, env.RouteAudio, r.Route(), "the disc plays")
		assert.Contains(t, pb.Calls(), "PlayAudio:l_3501")
		r.Shell.Back()
		r.Settle()

		tap(r, named(r.Tester.View().Root(), "back", 1))
		r.Settle()
		assert.NotEqual(t, env.RouteOffline, r.Route(), "Back leaves")
	})
}

// TestOfflineFollowsDownloads checks the live refresh: a lesson that finishes
// downloading gets its disc and "Audio đã tải" in place, one deleted gets the cloud,
// with the rows kept (same widgets) and a progress changing nothing.
func TestOfflineFollowsDownloads(t *testing.T) {
	t.Parallel()

	kittest.ForEachPhone(t, func(t *testing.T, ph kittest.Phone) {
		r := atOffline(t, ph, false)
		o := offlineOf(t, r)
		rows := o.Rows()
		dl := r.Fake.Dl()
		require.Equal(t, kit.DlNone, rows[4].Download)

		dl.SetDownload("l_3505", kit.DlDone, 1)
		r.Tester.Elapse(kittest.Frame)
		assert.Equal(t, kit.DlDone, rows[4].Download)
		assert.False(t, rows[4].Dim)
		assert.Contains(t, rows[4].Meta, "Audio đã tải")
		assert.Same(t, rows[4], o.Rows()[4], "the row is kept")

		require.NoError(t, dl.Delete("l_3501"))
		r.Tester.Elapse(kittest.Frame)
		assert.Equal(t, kit.DlNone, rows[0].Download)
		assert.True(t, rows[0].Dim)
		assert.Contains(t, rows[0].Meta, "Cần có mạng")

		before := rows[6].Meta
		dl.SetDownload("l_3507", kit.DlIng, .5)
		dl.SetDownload("l_3507", kit.DlIng, .7) // a progress
		r.Tester.Elapse(kittest.Frame)
		assert.Equal(t, before, rows[6].Meta, "a download under way is not a file")
		assert.Equal(t, kit.DlNone, rows[6].Download)
	})
}

// TestOfflineFromDownloadsOnly checks the course no screen read: the rows are the
// downloaded lessons of it, from the display data of the downloads (the name
// included); a tap plays; a file deleted takes its row away; with none left the
// screen says so; Back leaves.
func TestOfflineFromDownloadsOnly(t *testing.T) {
	t.Parallel()

	kittest.ForEachPhone(t, func(t *testing.T, ph kittest.Phone) {
		r := atOfflineCold(t, ph)
		o := offlineOf(t, r)
		require.Len(t, o.Rows(), 5, "the five files of the demo")
		assert.True(t, r.Tester.Shows("DanceFIT Set 35"))
		for _, row := range o.Rows() {
			assert.Equal(t, kit.DlDone, row.Download)
			assert.False(t, row.Dim)
		}

		tap(r, named(r.Tester.View().Root(), "offTap:set35-6", 1))
		r.Settle()
		assert.Equal(t, env.RouteAudio, r.Route(), "the file plays with no course read")
		assert.Contains(t, r.Fake.FakePlayback().Calls(), "PlayAudio:l_3506")
		r.Shell.Back()
		r.Settle()
		require.Equal(t, env.RouteOffline, r.Route())

		require.NoError(t, r.Fake.Dl().Delete("l_3506"))
		r.Tester.Elapse(kittest.Frame)
		assert.Len(t, offlineOf(t, r).Rows(), 4)

		r.Fake.Dl().Clear()
		r.Tester.Elapse(kittest.Frame)
		assert.Empty(t, offlineOf(t, r).Rows())
		assert.True(t, r.Tester.Shows("Chưa có file âm thanh"))
		assert.True(t, r.Tester.Shows("Video cần mạng · chỉ nghe được file âm thanh đã tải"))
	})
}

// TestOfflineBackOnline checks that the network coming back puts the detail in the
// place of screen 13, and that the stack is as it was.
func TestOfflineBackOnline(t *testing.T) {
	t.Parallel()

	kittest.ForEachPhone(t, func(t *testing.T, ph kittest.Phone) {
		r := atOffline(t, ph, false)
		depth := r.Shell.Nav().Depth()
		r.Fake.SetOnline(true)
		r.Tester.Elapse(kittest.Frame)
		r.Settle()
		assert.Equal(t, env.RouteDetail, r.Route())
		assert.Equal(t, depth, r.Shell.Nav().Depth())
	})
}

// TestOfflineSteadyFrameDoesNotAllocate checks that a frame of screen 13 standing
// still allocates nothing, on both phones, and that a download event that changes
// nothing it shows (a progress, a change of the space) allocates nothing.
func TestOfflineSteadyFrameDoesNotAllocate(t *testing.T) { //nolint:paralleltest // AllocsPerRun needs the process to itself
	if kittest.Race {
		t.Skip("allocation counts are not stable under the race detector")
	}
	for _, ph := range kittest.Phones {
		r := atOffline(t, ph, false)
		warmDetail(r)
		assert.Zero(t, testing.AllocsPerRun(50, r.Tester.Repaint), ph.Name)
	}
}

// BenchmarkOfflineSteadyFrame is a frame of screen 13 standing still.
func BenchmarkOfflineSteadyFrame(b *testing.B) {
	r := atOffline(b, kittest.Phones[0], false)
	warmDetail(r)
	b.ReportAllocs()
	for b.Loop() {
		r.Tester.Repaint()
	}
}
