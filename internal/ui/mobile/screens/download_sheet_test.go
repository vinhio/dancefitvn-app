//go:build !tvos

package screens_test

import (
	"fmt"
	"slices"
	"testing"

	"dancefitvn-app/internal/api"
	"dancefitvn-app/internal/format"
	"dancefitvn-app/internal/ui/kit"
	"dancefitvn-app/internal/ui/kit/kittest"
	"dancefitvn-app/internal/ui/mobile/env"
	"dancefitvn-app/internal/ui/mobile/screens"
	"dancefitvn-app/internal/ui/mobile/shell/shelltest"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// The texts of sheet 09 over the demo downloads of Set 35 (M09): lesson 5 has no
// file, 1 to 4 and 6 have one and 7 is under way, so "Cả khoá" has 5, 8, 9 and 10
// left.
const (
	freeLine    = "Bộ nhớ trống: 18,2 GB · Nghe được cả khi offline"
	confirmOne  = "Tải xuống · 8,4 MB"
	confirmAll  = "Tải xuống · 35,4 MB"
	oneSub      = "05 · Hãy Hát Lên · 03:30"
	allSub      = "4 file còn thiếu · đã có 6"
	allTitle    = "Cả khoá Set 35"
	oneTitle    = "Chỉ bài này"
	wifiOnlyRow = "Chỉ tải qua Wi‑Fi"
)

// openDownloadSheet opens sheet 09 from the detail of Set 35 as `dlTap` of lesson 5
// does, and lets it slide in.
func openDownloadSheet(t testing.TB, ph kittest.Phone, real bool, setups ...setup) *shelltest.Rig {
	t.Helper()
	r := atDetail(t, ph, real, "set-35", setups...)
	r.Shell.DlTap("set-35", "l_3505", 5)
	r.Settle()
	slideIn(r)
	require.True(t, r.Shell.Nav().Sheet().IsOpen())

	return r
}

// downloadSheetOf returns the content of the sheet 09 that is open.
func downloadSheetOf(t testing.TB, r *shelltest.Rig) *screens.DownloadSheet {
	t.Helper()
	s, ok := r.Shell.Nav().Sheet().Content().(*screens.DownloadSheet)
	require.True(t, ok, "sheet 09 is open")

	return s
}

// confirmText returns the text of the button of the sheet.
func confirmText(t testing.TB, r *shelltest.Rig) string {
	t.Helper()
	b, ok := named(r.Shell, "dlConfirm", 1).(*kit.Button)
	require.True(t, ok)

	return b.Text
}

// radio returns the radio of the sheet called name.
func radio(t testing.TB, r *shelltest.Rig, name string) *kit.Radio {
	t.Helper()
	x, ok := named(r.Shell, name, 1).(*kit.Radio)
	require.True(t, ok, name)

	return x
}

// ====================================================================
// ========================== Main functions ==========================
// ====================================================================

// TestDownloadSheetBoxesMatchPrototype lays sheet 09 out over the detail with the
// iPhone's real text engine and compares every element of M09 with the prototype's
// within 1 DIP: the dim, the panel and its grab, the header, the two radios with
// their dots, the Wi-Fi row with its switch, the line of the free space, the
// button. The detail under it is M05, whose rows are as accepted. The exceptions
// are boxes of text that Core Text sets up to 1,3 narrower than Chrome (the
// kerning row of PROGRESS section 5).
func TestDownloadSheetBoxesMatchPrototype(t *testing.T) {
	t.Parallel()

	r := openDownloadSheet(t, kittest.Phones[0], true)
	lay := r.Dump("M09")
	skip := map[string]bool{"playVideo:set35-3": true, "div.muted#2": true, "dlAll:set35": true, "svg.i#3": true, "span.d": true, "svg.i#17": true}
	checkAgainstProto(t, "M09", lay, 1, skip)
	want := shelltest.ProtoLayout(t, "M09")
	for key := range skip {
		w, _ := want.Find(key)
		g, ok := lay.Find(key)
		require.True(t, ok, key)
		assert.InDelta(t, w.Y, g.Y, 1, key+": y")
		assert.InDelta(t, w.H, g.H, 1, key+": h")
		assert.InDelta(t, w.X, g.X, 1.5, key+": x")
		assert.InDelta(t, w.W, g.W, 1.5, key+": w")
	}
}

// TestDownloadSheetOnTheWiderPhone checks the 412 wide phone's sheet against the
// iPhone's: the panel is as wide as the window and rests on its bottom, the parts
// keep their order and heights, what spans the 20 DIP margins is 372, the switch
// and the cross are at the right edge.
func TestDownloadSheetOnTheWiderPhone(t *testing.T) {
	t.Parallel()

	a := openDownloadSheet(t, kittest.Phones[0], true).Dump("M09")
	b := openDownloadSheet(t, kittest.Phones[1], true).Dump("M09")
	pa, _ := a.Find("div.sheet")
	pb, _ := b.Find("div.sheet")
	assert.InDelta(t, 412, pb.W, .01)
	assert.InDelta(t, 892, pb.Y+pb.H, .01, "on the bottom of the window")
	assert.InDelta(t, pa.H, pb.H, .6, "as tall as on the iPhone")
	for _, k := range []string{"div.sh", "dlChoice:one", "dlChoice:all", "pref:wifi", "p.muted", "dlConfirm"} {
		ea, _ := a.Find(k)
		eb, ok := b.Find(k)
		require.True(t, ok, k)
		assert.InDelta(t, 372, eb.W, .6, k+": spans the margins")
		assert.InDelta(t, ea.H, eb.H, .6, k+": height")
		assert.InDelta(t, ea.Y-pa.Y, eb.Y-pb.Y, .6, k+": place in the sheet")
	}
	for _, k := range []string{"closeSheet#2", "span.sw.on"} {
		ea, _ := a.Find(k)
		eb, _ := b.Find(k)
		assert.InDelta(t, ea.X+19, eb.X, .6, k+": at the right edge")
	}
}

// TestDownloadSheetShowsTheChoices checks the texts of the radios, the button and
// the line of the free space for the demo (M09), and that the choice moves the
// button's size and the radios' state, on both phones.
func TestDownloadSheetShowsTheChoices(t *testing.T) {
	t.Parallel()

	kittest.ForEachPhone(t, func(t *testing.T, ph kittest.Phone) {
		r := openDownloadSheet(t, ph, false)
		s := downloadSheetOf(t, r)
		assert.Equal(t, env.DownloadChoiceOne, s.Choice())
		assert.Equal(t, "l_3505", s.Params.LessonID)
		one, all := radio(t, r, "dlChoice:one"), radio(t, r, "dlChoice:all")
		assert.Equal(t, oneTitle, one.Title)
		assert.Equal(t, oneSub, one.Sub)
		assert.Equal(t, "8,4 MB", one.Trailing)
		assert.Equal(t, allTitle, all.Title)
		assert.Equal(t, allSub, all.Sub)
		assert.Equal(t, "35,4 MB", all.Trailing)
		assert.True(t, one.On)
		assert.False(t, all.On)
		assert.Equal(t, confirmOne, confirmText(t, r))
		assert.True(t, r.Tester.Shows(freeLine))
		n, bytes := s.Missing()
		assert.Equal(t, 4, n)
		assert.Equal(t, "35,4 MB", format.Size(bytes))

		tap(r, all)
		assert.Equal(t, env.DownloadChoiceAll, s.Choice())
		assert.True(t, all.On)
		assert.False(t, one.On)
		assert.Equal(t, confirmAll, confirmText(t, r))
		assert.Equal(t, env.DownloadChoiceAll, s.Choice())

		tap(r, one)
		assert.True(t, one.On)
		assert.Equal(t, confirmOne, confirmText(t, r))
		assert.Empty(t, r.Fake.Dl().Started(), "the radios start nothing")
	})
}

// TestDownloadAllOpensOnTheWholeCourse checks `dlAll`: the sheet opens on "Cả khoá"
// with the size of the course in its button.
func TestDownloadAllOpensOnTheWholeCourse(t *testing.T) {
	t.Parallel()

	kittest.ForEachPhone(t, func(t *testing.T, ph kittest.Phone) {
		r := atDetail(t, ph, false, "set-35")
		r.Shell.DlAll("set-35")
		r.Settle()
		s := downloadSheetOf(t, r)
		assert.Equal(t, env.DownloadChoiceAll, s.Choice())
		assert.Equal(t, "l_3505", s.Params.LessonID)
		assert.True(t, radio(t, r, "dlChoice:all").On)
		assert.Equal(t, confirmAll, confirmText(t, r))
	})
}

// TestDownloadSheetWiFiSwitch checks the row "Chỉ tải qua Wi‑Fi": it is on at first
// (the prototype's default), a tap turns the preference off and the switch with it,
// another turns it back, and the preference is the services' (it persists).
func TestDownloadSheetWiFiSwitch(t *testing.T) {
	t.Parallel()

	kittest.ForEachPhone(t, func(t *testing.T, ph kittest.Phone) {
		r := openDownloadSheet(t, ph, false)
		row, ok := named(r.Shell, "pref:wifi", 1).(*kit.Row)
		require.True(t, ok)
		sw, ok := row.Trailing.(*kit.Switch)
		require.True(t, ok)
		assert.Equal(t, wifiOnlyRow, row.Title)
		assert.True(t, sw.On)
		assert.True(t, r.Fake.FakePrefs().WiFiOnly())

		tap(r, row)
		assert.False(t, sw.On)
		assert.False(t, r.Fake.FakePrefs().WiFiOnly(), "the preference is kept")
		tap(r, row)
		assert.True(t, sw.On)
		assert.True(t, r.Fake.FakePrefs().WiFiOnly())

		// the sheet opened again shows the preference
		r.Fake.FakePrefs().SetWiFiOnly(false)
		r.Shell.Nav().Sheet().Close()
		r.Shell.DlTap("set-35", "l_3505", 5)
		r.Settle()
		row, _ = named(r.Shell, "pref:wifi", 1).(*kit.Row)
		assert.False(t, row.Trailing.(*kit.Switch).On)
	})
}

// TestDownloadSheetFreeSpace checks the line of the free space: filled with what
// the device says, filled again when the space is read again, left out while it is
// not known (Q-P5-9), and the sheet asks for the reading when it opens.
func TestDownloadSheetFreeSpace(t *testing.T) {
	t.Parallel()

	kittest.ForEachPhone(t, func(t *testing.T, ph kittest.Phone) {
		r := openDownloadSheet(t, ph, false)
		_, _, refreshes := r.Fake.Dl().Calls()
		assert.GreaterOrEqual(t, refreshes, 1, "asked on open")
		assert.True(t, r.Tester.Shows(freeLine))

		r.Fake.Dl().SetSpace(4_300_000_000, 64_000_000_000)
		r.Settle()
		assert.True(t, r.Tester.Shows("Bộ nhớ trống: 4,3 GB · Nghe được cả khi offline"))
		assert.False(t, r.Tester.Shows(freeLine))

		panel := func() float64 { e, _ := r.Dump("x").Find("div.sheet"); return e.H }
		withLine := panel()
		r.Fake.Dl().ClearSpace()
		r.Settle()
		assert.False(t, r.Tester.Shows("Bộ nhớ trống: 4,3 GB · Nghe được cả khi offline"))
		_, ok := r.Dump("x").Find("p.muted")
		assert.False(t, ok, "no line")
		assert.InDelta(t, 17, withLine-panel(), .6, "the sheet is the line's 17 shorter, its 4 and 16 of room kept")
		r.Fake.Dl().SetSpace(4_300_000_000, 64_000_000_000)
		r.Settle()
		assert.True(t, r.Tester.Shows("Bộ nhớ trống: 4,3 GB · Nghe được cả khi offline"), "back")
		assert.InDelta(t, withLine, panel(), .01)
	})
}

// TestDownloadSheetConfirmStartsTheChoice checks `dlConfirm`: "Chỉ bài này" queues
// the lesson, "Cả khoá" the lessons with no file and none under way, in the order
// of the course; the sheet closes and "Đang tải file âm thanh…" shows.
func TestDownloadSheetConfirmStartsTheChoice(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		all   bool
		start []string
	}{
		{name: "one", start: []string{"l_3505"}},
		{name: "all", all: true, start: []string{"l_3505", "l_3508", "l_3509", "l_3510"}},
	}
	kittest.ForEachPhone(t, func(t *testing.T, ph kittest.Phone) {
		for _, tc := range tests {
			t.Run(tc.name, func(t *testing.T) {
				r := openDownloadSheet(t, ph, false)
				if tc.all {
					tap(r, radio(t, r, "dlChoice:all"))
				}
				tap(r, named(r.Shell, "dlConfirm", 1))
				r.Settle()
				assert.Equal(t, tc.start, r.Fake.Dl().Started())
				assert.False(t, r.Shell.Nav().Sheet().IsOpen())
				assert.True(t, shows(r, toastStarted))
				for _, id := range tc.start {
					st, _ := r.Fake.Dl().State(id)
					assert.Equal(t, kit.DlIng, st, id)
				}
			})
		}
	})
}

// TestDownloadSheetCountsOnlyLessonsWithAudio checks Q-P5-10: a course whose lessons
// 8 and 9 have no audio has two files left to fetch, not four, and "Cả khoá"
// starts those two.
func TestDownloadSheetCountsOnlyLessonsWithAudio(t *testing.T) {
	t.Parallel()

	kittest.ForEachPhone(t, func(t *testing.T, ph kittest.Phone) {
		r := atDetail(t, ph, false, "set-35")
		c, ok := r.Shell.Env().CachedCourse("set-35")
		require.True(t, ok)
		c.Lessons = slices.Clone(c.Lessons)
		c.Lessons[7].HasAudio, c.Lessons[8].HasAudio = false, false
		sh := r.Shell.Nav().Sheet()
		sh.Open(screens.NewDownloadSheet(r.Shell.Env(), env.DownloadSheetParams{Course: c, LessonID: "l_3505", Choice: env.DownloadChoiceAll}, sh.Close), kit.SheetOptions{})
		r.Settle()
		slideIn(r)
		s := downloadSheetOf(t, r)
		n, bytes := s.Missing()
		assert.Equal(t, 2, n)
		assert.Equal(t, c.Lessons[4].AudioSizeBytes+c.Lessons[9].AudioSizeBytes, bytes)
		all := radio(t, r, "dlChoice:all")
		assert.Equal(t, "2 file còn thiếu · đã có 6", all.Sub)
		assert.Equal(t, format.Size(bytes), all.Trailing)
		assert.Equal(t, "Tải xuống · "+format.Size(bytes), confirmText(t, r))

		tap(r, named(r.Shell, "dlConfirm", 1))
		r.Settle()
		assert.Equal(t, []string{"l_3505", "l_3510"}, r.Fake.Dl().Started())
	})
}

// TestDownloadSheetStartRefusals checks what the confirm does when the download
// cannot start: with less free space than the files need the toast "Không đủ bộ
// nhớ" shows, nothing starts and the sheet stays; on mobile data with "Chỉ tải qua
// Wi‑Fi" the lessons are queued and the sheet closes with "Đang chờ Wi‑Fi để tải".
func TestDownloadSheetStartRefusals(t *testing.T) {
	t.Parallel()

	kittest.ForEachPhone(t, func(t *testing.T, ph kittest.Phone) {
		t.Run("no space", func(t *testing.T) {
			r := openDownloadSheet(t, ph, false)
			r.Fake.Dl().SetSpace(1_000_000, 64_000_000_000)
			r.Settle()
			tap(r, named(r.Shell, "dlConfirm", 1))
			r.Settle()
			assert.Empty(t, r.Fake.Dl().Started())
			assert.True(t, r.Shell.Nav().Sheet().IsOpen(), "the sheet stays")
			assert.True(t, shows(r, toastNoSpace))
		})
		t.Run("wifi only on mobile data", func(t *testing.T) {
			r := openDownloadSheet(t, ph, false)
			r.Fake.Dl().SetOnWiFi(false)
			tap(r, named(r.Shell, "dlConfirm", 1))
			r.Settle()
			assert.Equal(t, []string{"l_3505"}, r.Fake.Dl().Started(), "queued, paused")
			assert.False(t, r.Shell.Nav().Sheet().IsOpen())
			assert.True(t, shows(r, toastWaitWiFi))
		})
		t.Run("wifi only turned off on mobile data", func(t *testing.T) {
			r := openDownloadSheet(t, ph, false)
			r.Fake.Dl().SetOnWiFi(false)
			tap(r, named(r.Shell, "pref:wifi", 1))
			tap(r, named(r.Shell, "dlConfirm", 1))
			r.Settle()
			assert.Equal(t, []string{"l_3505"}, r.Fake.Dl().Started())
			assert.True(t, shows(r, toastStarted))
		})
		t.Run("network gone under the sheet", func(t *testing.T) {
			r := openDownloadSheet(t, ph, false)
			r.Fake.Dl().SetStartErr(fmt.Errorf("test: %w", api.ErrOffline))
			tap(r, named(r.Shell, "dlConfirm", 1))
			r.Settle()
			assert.True(t, shows(r, toastNoNet))
		})
	})
}

// TestDownloadSheetFollowsTheDownloads checks that the counts of the sheet follow
// the downloads while it is open: a lesson of the course that finishes or starts
// leaves "Cả khoá"; the sheet stops following when it closes.
func TestDownloadSheetFollowsTheDownloads(t *testing.T) {
	t.Parallel()

	kittest.ForEachPhone(t, func(t *testing.T, ph kittest.Phone) {
		r := openDownloadSheet(t, ph, false)
		s := downloadSheetOf(t, r)
		all := radio(t, r, "dlChoice:all")
		r.Fake.Dl().SetDownload("l_3508", kit.DlDone, 1)
		r.Settle()
		assert.Equal(t, "3 file còn thiếu · đã có 7", all.Sub)
		n, _ := s.Missing()
		assert.Equal(t, 3, n)
		r.Fake.Dl().SetDownload("l_3509", kit.DlIng, .3)
		r.Settle()
		assert.Equal(t, "2 file còn thiếu · đã có 8", all.Sub)
		assert.Equal(t, "Tải xuống · 8,4 MB", confirmText(t, r))

		r.Shell.Nav().Sheet().Close()
		r.Settle()
		r.Fake.Dl().SetDownload("l_3510", kit.DlDone, 1)
		r.Settle()
		assert.Equal(t, "2 file còn thiếu · đã có 8", all.Sub, "a closed sheet is not followed")
	})
}

// TestDownloadSheetCross checks that the cross of the header closes the sheet
// without starting anything.
func TestDownloadSheetCross(t *testing.T) {
	t.Parallel()

	kittest.ForEachPhone(t, func(t *testing.T, ph kittest.Phone) {
		r := openDownloadSheet(t, ph, false)
		tap(r, named(r.Shell, "closeSheet", 2))
		r.Settle()
		assert.False(t, r.Shell.Nav().Sheet().IsOpen())
		assert.Empty(t, r.Fake.Dl().Started())
	})
}

// TestDownloadSheetSteadyFrameDoesNotAllocate checks that a frame of the open sheet
// standing still allocates nothing, on both phones, and that a progress event of a
// download under way, which the sheet hears, does not either.
func TestDownloadSheetSteadyFrameDoesNotAllocate(t *testing.T) { //nolint:paralleltest // AllocsPerRun needs the process to itself
	if kittest.Race {
		t.Skip("allocation counts are not stable under the race detector")
	}
	for _, ph := range kittest.Phones {
		r := openDownloadSheet(t, ph, false)
		warmDownloads(r)
		assert.Zero(t, testing.AllocsPerRun(50, r.Tester.Repaint), ph.Name)
		ev := env.DownloadEvent{Kind: env.DownloadEventProgress, LessonID: "l_3507", No: 7, Done: 6_272_000, Total: 9_800_000}
		topic := r.Fake.Dl().Events()
		assert.Zero(t, testing.AllocsPerRun(50, func() { topic.Publish(ev) }), ph.Name+": progress event")
	}
}
