//go:build !tvos

// Pattern from jplayer-mobile/internal/ui/views/alloc_spec_test.go at 49de3c2
// (steady: warm up, then AllocsPerRun; a scroll step within what is laid out) and
// from courses_test.go (the rig, the proto comparison, the wider phone).

package screens_test

import (
	"context"
	"strconv"
	"testing"
	"time"

	"dancefitvn-app/internal/ui/icon"
	"dancefitvn-app/internal/ui/kit"
	"dancefitvn-app/internal/ui/kit/kittest"
	"dancefitvn-app/internal/ui/mobile/env"
	"dancefitvn-app/internal/ui/mobile/nav"
	"dancefitvn-app/internal/ui/mobile/screens"
	"dancefitvn-app/internal/ui/mobile/shell"
	"dancefitvn-app/internal/ui/mobile/shell/shelltest"
	"dancefitvn-app/internal/ui/theme"
	"dancefitvn-app/locales"

	"github.com/gdeskdev/desk/foundation/core/geom"
	"github.com/gdeskdev/desk/foundation/core/keys"
	"github.com/gdeskdev/desk/framework/ui"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ====================================================================
// ============================ Contracts =============================
// ====================================================================

// ms is a millisecond.
const ms = time.Millisecond

// flip is Services whose Online answer a test can turn off without telling the
// shell's watchers: the detail stays on the screen while the actions see the network
// gone (the window between the network dying and the shell's swap to the offline
// screen).
type flip struct {
	*shelltest.Fake

	off bool
}

// Online is the Fake's, until off.
func (f *flip) Online() bool { return f.Fake.Online() && !f.off }

// ====================================================================
// ========================== Main functions ==========================
// ====================================================================

// TestDetailBoxesMatchPrototype lays the detail of Set 35 out with the iPhone's real
// text engine over the mock's `courses.show` and compares every element of M05 with the
// prototype's within 1 DIP: the cover, the two round buttons, the title block, the card
// with its play square's text and "Tải hết", the tabs, the ten rows (the current one 8
// outside the column, the keys numbered #2 and #3 as the capture does) and the footer's
// button. The exceptions are widths: "Tải hết" is 0.97 narrower in Core Text than in
// Chrome, which makes the card's text that much wider.
func TestDetailBoxesMatchPrototype(t *testing.T) {
	t.Parallel()

	r := atDetail(t, kittest.Phones[0], true, "set-35")
	lay := r.Dump("M05")
	skip := map[string]bool{"playVideo:set35-3": true, "div.muted#2": true, "dlAll:set35": true, "svg.i#3": true}
	checkAgainstProto(t, "M05", lay, 1, skip)

	want := shelltest.ProtoLayout(t, "M05")
	for _, key := range []string{"playVideo:set35-3", "div.muted#2", "dlAll:set35", "svg.i#3"} {
		w, _ := want.Find(key)
		g, ok := lay.Find(key)
		require.True(t, ok, key)
		assert.InDelta(t, w.Y, g.Y, 1, key+": y")
		assert.InDelta(t, w.H, g.H, 1, key+": h")
		assert.InDelta(t, w.X, g.X, 1.5, key+": x")
		assert.InDelta(t, w.W, g.W, 1.5, key+": w")
	}
}

// TestDetailOnTheWiderPhone checks the 412 wide phone's detail against the iPhone's:
// the same column from the top, what runs the width 19 wider, the buttons at the right
// edge moved by the 19, the footer on the bottom of the taller screen.
func TestDetailOnTheWiderPhone(t *testing.T) {
	t.Parallel()

	rI := atDetail(t, kittest.Phones[0], true, "set-35")
	rA := atDetail(t, kittest.Phones[1], true, "set-35")
	a, b := rI.Dump("M05"), rA.Dump("M05")
	require.Len(t, b.Elements, len(a.Elements))
	for i, ea := range a.Elements {
		eb := b.Elements[i]
		require.Equal(t, ea.Key, eb.Key)
		if ea.Key == "playVideo:set35-3#3" { // the footer: on the bottom of the taller screen
			assert.InDelta(t, 892-38, eb.Y+eb.H, 0.01)

			continue
		}
		assert.InDelta(t, ea.Y, eb.Y, 1.5, ea.Key+": y")
	}
	full := map[string]float64{
		"div.poster": 412, "div.eb.acc": 372, "div.d": 372, "div.muted": 372, "div.card": 372,
		"playVideo:set35-1": 372, "playVideo:set35-3#2": 388, "playVideo:set35-10": 372, "playVideo:set35-3#3": 372,
	}
	for key, w := range full {
		e, ok := b.Find(key)
		require.True(t, ok, key)
		assert.InDelta(t, w, e.W, 0.01, key+": width")
	}
	for _, key := range []string{"button.rb.dk", "dlAll:set35", "dlTap:set35-1", "dlTap:set35-7"} {
		ea, _ := a.Find(key)
		eb, _ := b.Find(key)
		// the chip's text is shaped by each phone's own engine: its width differs by a pixel
		assert.InDelta(t, ea.X+19, eb.X, 1.5, key+": moves with the right edge")
	}
	e, _ := b.Find("span.muted")
	ea, _ := a.Find("span.muted")
	assert.InDelta(t, ea.X, e.X, 0.5, "the tabs keep their place")
}

// TestDetailShowsTheMockCourse checks what the mock's Set 35 makes the screen say and
// the lessons' states: the eyebrow, the title, the sub line with its minutes ("39"), the
// card's "Đang xem bài 03" and counts, the tabs, the footer's "Tiếp tục bài 03"; the two
// watched rows, the current one, the downloads (done, in progress, none).
func TestDetailShowsTheMockCourse(t *testing.T) {
	t.Parallel()

	kittest.ForEachPhone(t, func(t *testing.T, ph kittest.Phone) {
		r := atDetail(t, ph, false, "set-35")
		for _, s := range []string{
			"Khoá học Thường · 10 bài học", "DanceFIT Set 35", "“Cùng nhảy cùng hát” · 10 bài · 39 phút",
			"Đang xem bài 03", "2/10 đã xem · 5/10 audio đã tải", "Tải hết",
			"Bài học · 10", "Giới thiệu", "Tiếp tục bài 03",
			"Warm Up · 05:20 · Đã xem", "Tay Vai · 03:05 · Đã xem", "Bụng · đang xem 01:12 / 03:40", "Bachata · 04:10",
		} {
			assert.True(t, r.Tester.Shows(s), s)
		}
		assert.False(t, r.Tester.Shows("Bắt đầu bài 01"))
		assert.False(t, r.Tester.Shows("Đang đồng bộ khoá học…"), "the skeleton is gone")
		assert.True(t, r.Shell.Nav().TabBar().Hidden, "no tab bar")
		d := detailOf(t, r)
		require.NotNil(t, d.Content())
		assert.Nil(t, d.Locked())
		assert.Equal(t, 1, r.Fake.Calls("courses.show"))

		rows := d.Content().Rows()
		require.Len(t, rows, 10)
		states := []kit.LessonState{
			kit.LessonWatched, kit.LessonWatched, kit.LessonCurrent, kit.LessonNormal, kit.LessonNormal,
			kit.LessonNormal, kit.LessonNormal, kit.LessonNormal, kit.LessonNormal, kit.LessonNormal,
		}
		dls := []kit.LessonDownload{
			kit.DlDone, kit.DlDone, kit.DlDone, kit.DlDone, kit.DlNone, kit.DlDone, kit.DlIng, kit.DlNone, kit.DlNone, kit.DlNone,
		}
		for i, row := range rows {
			assert.Equal(t, states[i], row.State, "state of lesson %d", i+1)
			assert.Equal(t, dls[i], row.Download, "download of lesson %d", i+1)
			assert.Contains(t, []kit.LessonMode{kit.LessonModeUnknown, kit.LessonOnline}, row.Mode, "an online row")
		}
		assert.InDelta(t, .64, rows[6].Progress, 1e-6, "the ring")
		assert.Same(t, d.Scroll(), r.Shell.Nav().Screen().(nav.Scroller).Scroll())
	})
}

// TestDetailRowsFollowALiveDownload checks the rows while a download runs (P5.3):
// the row of a lesson whose download starts shows the ring, a progress event
// moves it (and the ring of a download already under way) with no rebuild, the
// finish makes the row "downloaded" and the count of "audio đã tải" follows, and
// a canceled download goes back to the cloud. It starts the download through
// Start (the queue the sheet 09 makes), where TestDetailFollowsDownloads sets the
// states directly.
func TestDetailRowsFollowALiveDownload(t *testing.T) {
	t.Parallel()

	kittest.ForEachPhone(t, func(t *testing.T, ph kittest.Phone) {
		r := atDetail(t, ph, false, "set-35")
		d := detailOf(t, r)
		rows := d.Content().Rows()
		require.Len(t, rows, 10)
		row5, row7 := rows[4], rows[6]
		require.Equal(t, kit.DlNone, row5.Download)
		require.Equal(t, kit.DlIng, row7.Download)
		require.True(t, r.Tester.Shows("2/10 đã xem · 5/10 audio đã tải"), "five files")
		c, ok := r.Shell.Env().CachedCourse("set-35")
		require.True(t, ok)
		dl := r.Fake.Dl()

		// the glyph of the row's download button, as the device paints it
		dlIcon := func(n int) *kit.Icon {
			b := named(r.Tester.View().Root(), "dlTap:set35-"+strconv.Itoa(n), 1)
			require.NotNil(t, b)
			return named(b, "svg.i", 1).(*kit.Icon)
		}
		require.Equal(t, icon.Dl, dlIcon(5).Glyph)

		require.NoError(t, dl.Start(c, c.Lessons[4:5]))
		r.Tester.Elapse(kittest.Frame)
		assert.Equal(t, kit.DlIng, row5.Download, "the ring appears when the download starts")
		assert.Equal(t, icon.X, dlIcon(5).Glyph, "the cross of a running download")
		assert.InDelta(t, 0, row5.Progress, 1e-6)
		assert.Same(t, row5, d.Content().Rows()[4], "the row is not rebuilt")

		for _, p := range []float32{.25, .6} {
			dl.SetDownload("l_3505", kit.DlIng, p) // a DownloadEventProgress
			dl.SetDownload("l_3507", kit.DlIng, p+.3)
			r.Tester.Elapse(kittest.Frame)
			assert.InDelta(t, p, row5.Progress, 1e-3, "the ring of the new download")
			assert.InDelta(t, p+.3, row7.Progress, 1e-3, "and of the one under way")
			assert.Equal(t, kit.DlIng, row5.Download)
		}
		st, share := dl.State("l_3505")
		assert.Equal(t, kit.DlIng, st)
		assert.InDelta(t, .6, share, 1e-3)

		dl.FinishDownload("l_3505")
		r.Tester.Elapse(kittest.Frame)
		assert.Equal(t, kit.DlDone, row5.Download, "the file is in")
		assert.Equal(t, theme.InkSuccess, dlIcon(5).Ink, "the done icon, not the cross, is what is drawn")
		assert.Equal(t, icon.Dl, dlIcon(5).Glyph)
		assert.True(t, r.Tester.Shows("2/10 đã xem · 6/10 audio đã tải"), "the count follows")

		dl.Forget("l_3507") // the cross of the running card
		r.Tester.Elapse(kittest.Frame)
		assert.Equal(t, kit.DlNone, row7.Download, "a canceled download is the cloud again")
		assert.Equal(t, theme.InkText3, dlIcon(7).Ink)
		assert.Equal(t, icon.Dl, dlIcon(7).Glyph)
		assert.InDelta(t, 0, row7.Progress, 1e-6)
		assert.True(t, r.Tester.Shows("2/10 đã xem · 6/10 audio đã tải"))
	})
}

// TestDetailTapsAreActions checks that every control of the screen asks the action of
// 03 section 3 it is the prototype's `data-act` for, with the lesson's course, id and
// number: a row and the card's text and the footer play the lesson, the download button
// of a row downloads it, "Tải hết" downloads the course, Back goes back; the "more"
// button has no action.
func TestDetailTapsAreActions(t *testing.T) {
	t.Parallel()

	kittest.ForEachPhone(t, func(t *testing.T, ph kittest.Phone) {
		d, rec, r := atDetailDirect(t, ph, "set-35")
		root := r.Tester.View().Root()
		for _, tc := range []struct {
			name   string
			n      int
			scroll float32 // the scroll before the tap: the footer covers the rows from 740
			want   string
		}{
			{"playVideo:set35-3", 1, 0, "playVideo set-35 l_3503 3"}, // the card's text
			{"playVideo:set35-5", 1, 0, "playVideo set-35 l_3505 5"}, // a row
			{"playVideo:set35-3", 2, 0, "playVideo set-35 l_3503 3"}, // the current row
			{"playVideo:set35-3", 3, 0, "playVideo set-35 l_3503 3"}, // the footer's button
			{"dlTap:set35-5", 1, 0, "dlTap set-35 l_3505 5"},
			{"dlTap:set35-7", 1, 300, "dlTap set-35 l_3507 7"}, // the one downloading
			{"playVideo:set35-9", 1, 300, "playVideo set-35 l_3509 9"},
			{"dlAll:set35", 1, 0, "dlAll set-35"},
			{"back", 1, 0, "back"},
		} {
			rec.calls = nil
			d.Scroll().ScrollTo(geom.Pt(0, tc.scroll), false)
			r.Tester.Elapse(kittest.Frame)
			w := named(root, tc.name, tc.n)
			require.NotNil(t, w, "%s#%d", tc.name, tc.n)
			tap(r, w)
			assert.Equal(t, []string{tc.want}, rec.calls, "%s#%d", tc.name, tc.n)
		}
		d.Scroll().ScrollTo(geom.Point{}, false)
		r.Tester.Elapse(kittest.Frame)
		rec.calls = nil
		tap(r, named(root, "button.rb.dk", 1))
		assert.Empty(t, rec.calls, "the more button has no action")
	})
}

// TestDetailWithoutResume checks a course the user has not started (Set 34, `resume`
// null): "Bắt đầu bài 01" on the card, "Xem bài 01" on the footer, both playing lesson 1,
// and no row is the current one.
func TestDetailWithoutResume(t *testing.T) {
	t.Parallel()

	kittest.ForEachPhone(t, func(t *testing.T, ph kittest.Phone) {
		d, rec, r := atDetailDirect(t, ph, "set-34")
		assert.True(t, r.Tester.Shows("Bắt đầu bài 01"))
		assert.True(t, r.Tester.Shows("Xem bài 01"))
		assert.False(t, r.Tester.Shows("Đang xem bài 01"))
		assert.False(t, r.Tester.Shows("Tiếp tục bài 01"))
		for i, row := range d.Content().Rows() {
			assert.NotEqual(t, kit.LessonCurrent, row.State, "lesson %d", i+1)
		}
		root := r.Tester.View().Root()
		tap(r, named(root, "playVideo:set34-1", 1))
		tap(r, named(root, "playVideo:set34-1", 2))
		tap(r, named(root, "playVideo:set34-1", 3))
		require.Len(t, rec.calls, 3)
		for _, c := range rec.calls {
			assert.Equal(t, "playVideo set-34 "+firstID(t, r)+" 1", c)
		}
	})
}

// firstID is the id of lesson 1 of Set 34 in the mock.
func firstID(t *testing.T, r *shelltest.Rig) string {
	t.Helper()
	c, err := r.Fake.Course(context.Background(), "set-34")
	require.NoError(t, err)

	return c.Lessons[0].ID
}

// TestDetailLongNameIsSmaller checks the title's size: 36 for a name of up to 18
// letters, 30 for a longer one (`c.name.length>18?30:36`).
func TestDetailLongNameIsSmaller(t *testing.T) {
	t.Parallel()

	for id, h := range map[string]float64{"set-35": 36, "hot-extra": 36, "hot-extra-2": 30} {
		r := atDetail(t, kittest.Phones[0], true, id)
		e, ok := r.Dump("M05").Find("div.d")
		require.True(t, ok, id)
		assert.InDelta(t, h, e.H, 0.01, id)
	}
}

// TestDetailBack checks that Back, the button and the system's key, returns to the
// screen before: Home, or Courses when the card was opened from there.
func TestDetailBack(t *testing.T) {
	t.Parallel()

	kittest.ForEachPhone(t, func(t *testing.T, ph kittest.Phone) {
		r := atDetail(t, ph, false, "set-35")
		require.NoError(t, r.Tester.Click("back"))
		r.Settle()
		assert.Equal(t, "home", r.Route())

		r.Shell.Nav().Go(nav.Route{Screen: "courses"}, true)
		r.Tester.Elapse(kittest.Frame)
		r.Settle()
		require.NoError(t, r.Tester.Click("go.detail:set35"))
		r.Tester.Elapse(kittest.Frame)
		r.Settle()
		require.Equal(t, "detail", r.Route())
		assert.True(t, r.Tester.Key(keys.KeyBack, 0), "the system's Back")
		assert.Equal(t, "courses", r.Route())
	})
}

// TestDetailOfflineActions checks the offline branches of 03 section 2 and 3 through
// the screen's own controls: with the network gone a row's tap toasts "Video bài NN cần
// có mạng để xem", a download button's and "Tải hết" toast "Không có mạng để tải", each
// for 2200 ms.
func TestDetailOfflineActions(t *testing.T) {
	t.Parallel()

	kittest.ForEachPhone(t, func(t *testing.T, ph kittest.Phone) {
		f, err := shelltest.New(shelltest.MockFS(t))
		require.NoError(t, err)
		require.NoError(t, f.SeedSession())
		svc := &flip{Fake: f}
		sh := shell.New(svc, locales.MustTexts(), shell.Options{})
		tt := kittest.Mount(t, ph, sh)
		kittest.PinMotion(tt)
		r := &shelltest.Rig{Phone: ph, Fake: f, Shell: sh, Tester: tt}
		r.Boot()
		sh.Detail("set-35")
		tt.Elapse(kittest.Frame)
		r.Settle()
		require.Equal(t, "detail", r.Route())
		svc.off = true // the actions see no network; the swap to the offline screen is not made
		root := tt.View().Root()

		tap(r, named(root, "playVideo:set35-5", 1))
		tt.Elapse(100 * ms)
		assert.True(t, tt.Shows("Video bài 05 cần có mạng để xem"))
		tt.Elapse(2200 * ms)
		assert.False(t, tt.Shows("Video bài 05 cần có mạng để xem"), "the toast ends after 2200 ms")

		tap(r, named(root, "dlTap:set35-5", 1))
		tt.Elapse(100 * ms)
		assert.True(t, tt.Shows("Không có mạng để tải"))
		tt.Elapse(2300 * ms)
		assert.False(t, tt.Shows("Không có mạng để tải"))

		tap(r, named(root, "dlAll:set35", 1))
		tt.Elapse(100 * ms)
		assert.True(t, tt.Shows("Không có mạng để tải"))
	})
}

// TestDetailSwapsToOfflineAndBack checks 03 section 2 on this screen: the network
// going puts the course's offline screen in its place, the stack as it was, and its
// return puts the detail back with its content.
func TestDetailSwapsToOfflineAndBack(t *testing.T) {
	t.Parallel()

	kittest.ForEachPhone(t, func(t *testing.T, ph kittest.Phone) {
		r := atDetail(t, ph, false, "set-35")
		depth := r.Shell.Nav().Depth()
		r.Fake.SetOnline(false)
		r.Settle()
		assert.Equal(t, "offline", r.Route())
		assert.Equal(t, depth, r.Shell.Nav().Depth())
		r.Fake.SetOnline(true)
		r.Tester.Elapse(kittest.Frame)
		r.Settle()
		require.Equal(t, "detail", r.Route())
		assert.NotNil(t, detailOf(t, r).Content(), "the course is read again")
		assert.True(t, r.Tester.Shows("Tiếp tục bài 03"))
	})
}

// TestDetailLoadingAndRetry checks the state before the answer: the skeleton with the
// back button and the sync line; a failure leaves it with "Thử lại", whose tap reads
// again; the screen takes the answer when it comes.
func TestDetailLoadingAndRetry(t *testing.T) {
	t.Parallel()

	kittest.ForEachPhone(t, func(t *testing.T, ph kittest.Phone) {
		f, err := shelltest.New(shelltest.MockFS(t))
		require.NoError(t, err)
		require.NoError(t, f.SeedSession())
		f.Fail("courses.show", 500, "errors/server_error.json")
		r := shelltest.MountRig(t, ph, false, f, shellOptions())
		r.Boot()
		r.Shell.Detail("set-35")
		r.Tester.Elapse(kittest.Frame)
		r.Settle()
		d := detailOf(t, r)
		assert.Nil(t, d.Content())
		require.NotNil(t, d.Retry())
		assert.False(t, d.Retry().Hidden, "a failed read shows Thử lại")
		assert.True(t, r.Tester.Shows("Thử lại"))
		assert.False(t, r.Tester.Shows("Đang đồng bộ khoá học…"))
		assert.NotNil(t, named(r.Tester.View().Root(), "back", 1), "the back button is on the skeleton")

		f.Fail("courses.show", 0, "")
		require.NoError(t, r.Tester.Click("retry"))
		r.Tester.Elapse(kittest.Frame)
		r.Settle()
		assert.NotNil(t, d.Content(), "the retry reads again")
		assert.True(t, r.Tester.Shows("Tiếp tục bài 03"))
	})
}

// TestDetailSkeletonWhileReading checks the skeleton while the read is held by a gate,
// the back button working on it, and the content replacing it when the gate opens.
func TestDetailSkeletonWhileReading(t *testing.T) {
	t.Parallel()

	kittest.ForEachPhone(t, func(t *testing.T, ph kittest.Phone) {
		f, err := shelltest.New(shelltest.MockFS(t))
		require.NoError(t, err)
		require.NoError(t, f.SeedSession())
		release := f.Gate("courses.show")
		r := shelltest.MountRig(t, ph, false, f, shellOptions())
		r.Boot()
		r.Shell.Detail("set-35")
		r.Tester.Elapse(kittest.Frame)
		d := detailOf(t, r)
		assert.True(t, r.Tester.Shows("Đang đồng bộ khoá học…"))
		assert.Nil(t, d.Content())
		assert.True(t, d.Retry().Hidden)
		release()
		r.Settle()
		assert.False(t, r.Tester.Shows("Đang đồng bộ khoá học…"))
		assert.NotNil(t, d.Content())

		r.Shell.Back()
		r.Settle()
		release2 := f.Gate("courses.show")
		defer release2()
		r.Shell.Detail("set-34")
		r.Tester.Elapse(kittest.Frame)
		require.NoError(t, r.Tester.Click("back"))
		assert.Equal(t, "home", r.Route(), "Back works on the skeleton")
	})
}

// TestDetailSteadyFrameDoesNotAllocate checks that a frame of the detail standing still
// allocates nothing, on both phones.
func TestDetailSteadyFrameDoesNotAllocate(t *testing.T) { //nolint:paralleltest // AllocsPerRun needs the process to itself
	if kittest.Race {
		t.Skip("allocation counts are not stable under the race detector")
	}
	for _, ph := range kittest.Phones {
		for _, id := range []string{"set-35", "set-36"} {
			r := atDetail(t, ph, false, id)
			warmDetail(r)
			assert.Zero(t, testing.AllocsPerRun(50, r.Tester.Repaint), ph.Name+" "+id)
		}
	}
}

// TestDetailListSteadyFrameDoesNotAllocate checks that a frame of the lesson list
// standing still allocates nothing with the current lesson, the download ring and the
// dim of the locked rows on the screen, and that a frame while the list is scrolled
// does not either: every row is laid out, shaped and drawn once first.
func TestDetailListSteadyFrameDoesNotAllocate(t *testing.T) { //nolint:paralleltest // AllocsPerRun needs the process to itself
	if kittest.Race {
		t.Skip("allocation counts are not stable under the race detector")
	}
	for _, ph := range kittest.Phones {
		for _, id := range []string{"set-35", "set-36"} {
			r := atDetail(t, ph, false, id)
			sc := detailOf(t, r).Scroll()
			warmDetail(r)
			y := float32(0)
			for ; y < 600; y += 40 { // every row is on screen once
				sc.ScrollTo(geom.Pt(0, y), false)
				r.Tester.Elapse(kittest.Frame)
			}
			dir := float32(-1)
			page := func() {
				if y > 600 || y < 300 {
					dir = -dir
				}
				y += 3 * dir
				sc.ScrollTo(geom.Pt(0, y), false)
				r.Tester.Frame()
			}
			for range 16 {
				page()
			}
			assert.Zero(t, testing.AllocsPerRun(50, page), ph.Name+" "+id+": scrolling")
		}
	}
}

// BenchmarkDetailSteadyFrame is a frame of the detail standing still.
func BenchmarkDetailSteadyFrame(b *testing.B) {
	r := atDetail(b, kittest.Phones[0], false, "set-35")
	warmDetail(r)
	b.ReportAllocs()
	for b.Loop() {
		r.Tester.Repaint()
	}
}

// ====================================================================
// ========================= Helper Functions =========================
// ====================================================================

// atDetail boots a signed-in app, opens the course id from Home and lets its read
// run: the screen shows its content.
func atDetail(tb testing.TB, ph kittest.Phone, real bool, id string, setups ...setup) *shelltest.Rig {
	tb.Helper()
	f, err := shelltest.New(shelltest.MockFS(tb))
	require.NoError(tb, err)
	require.NoError(tb, f.SeedSession())
	if t, ok := tb.(*testing.T); ok {
		for _, s := range setups {
			s(t, f)
		}
	}
	r := shelltest.MountRig(tb, ph, real, f, shellOptions())
	r.Boot()
	require.Equal(tb, "home", r.Route())
	r.Shell.Detail(id)
	r.Tester.Elapse(kittest.Frame)
	r.Settle()
	require.Equal(tb, "detail", r.Route())
	d, ok := r.Shell.Nav().Screen().(*screens.Detail)
	require.True(tb, ok)
	require.True(tb, d.Content() != nil || d.Locked() != nil, "the course is in")

	return r
}

// atDetailDirect mounts Detail alone over a Fake with a recording Actions, the course
// in and the app config known (the support line of the locked course).
func atDetailDirect(t *testing.T, ph kittest.Phone, id string) (*screens.Detail, *recorder, *shelltest.Rig) {
	t.Helper()
	f, err := shelltest.New(shelltest.MockFS(t))
	require.NoError(t, err)
	require.NoError(t, f.SeedSession())
	signed, err := f.Restore(context.Background())
	require.NoError(t, err)
	require.True(t, signed)
	cfg, err := f.Config(context.Background())
	require.NoError(t, err)
	rec := &recorder{}
	e := env.New(f, locales.MustTexts())
	e.Act = rec
	e.SetConfig(cfg)
	d := screens.NewDetail(e, id)
	tt := kittest.MountReal(t, ph, d)
	kittest.PinMotion(tt)
	tt.Elapse(kittest.Frame)
	tt.RunUntilIdle()
	require.True(t, d.Content() != nil || d.Locked() != nil, "the course is in")

	return d, rec, &shelltest.Rig{Phone: ph, Fake: f, Tester: tt}
}

// detailOf returns the Detail screen showing.
func detailOf(t testing.TB, r *shelltest.Rig) *screens.Detail {
	t.Helper()
	d, ok := r.Shell.Nav().Screen().(*screens.Detail)
	require.True(t, ok, "the screen showing is Detail")

	return d
}

// warmDetail lets the frames run until what the detail draws is laid out and shaped
// once.
func warmDetail(r *shelltest.Rig) {
	for range 8 {
		r.Tester.Elapse(kittest.Frame)
	}
	r.Tester.Repaint()
}

// named returns the n-th (from 1) visible widget called name in tree order, the way
// layout.json numbers `name`, `name#2`...; nil when there is none.
func named(root ui.Widget, name string, n int) ui.Widget {
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
		for ch := range b.Children() {
			walk(ch)
		}
	}
	walk(root)

	return found
}
