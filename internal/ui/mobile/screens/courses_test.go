//go:build !tvos

// Pattern from jplayer-mobile/internal/ui/views/alloc_spec_test.go at 49de3c2
// (steady: warm up, then AllocsPerRun) and from home_test.go.

package screens_test

import (
	"context"
	"strconv"
	"testing"

	"dancefitvn-app/internal/model"
	"dancefitvn-app/internal/ui/kit/kittest"
	"dancefitvn-app/internal/ui/mobile/env"
	"dancefitvn-app/internal/ui/mobile/nav"
	"dancefitvn-app/internal/ui/mobile/screens"
	"dancefitvn-app/internal/ui/mobile/shell/shelltest"
	"dancefitvn-app/locales"

	"github.com/gdeskdev/desk/foundation/core/geom"
	"github.com/gdeskdev/desk/foundation/core/keys"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ====================================================================
// ============================ Contracts =============================
// ====================================================================

// The ids of the mock's 15 courses in the order of the list.
var mockCourses = []string{
	"set-35", "set-34", "hot-extra", "set-33", "set-32", "set-31", "set-30", "set-29", "set-28",
	"set-27", "set-26", "set-25", "set-24", "hot-extra-2", "intro",
}

// The filter buttons' names and what they let through (counts of me/courses.json).
var filters = []struct {
	key   string
	count int
}{
	{"filter:Tất cả", 15}, {"filter:Thường", 12}, {"filter:Đặc biệt", 2}, {"filter:Giới thiệu", 1},
}

// ====================================================================
// ========================== Main functions ==========================
// ====================================================================

// atCourses boots a signed-in app and takes the Courses tab, with its list in.
func atCourses(tb testing.TB, ph kittest.Phone, real bool, setups ...setup) *shelltest.Rig {
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
	goCourses(r)
	require.Equal(tb, "courses", r.Route())
	require.Equal(tb, len(mockCourses), coursesOf(tb, r).Content().Rows(), "the list is in")

	return r
}

// coursesOf returns the Courses screen showing.
func coursesOf(t testing.TB, r *shelltest.Rig) *screens.Courses {
	t.Helper()
	c, ok := r.Shell.Nav().Screen().(*screens.Courses)
	require.True(t, ok, "the screen showing is Courses")

	return c
}

// atCoursesDirect mounts Courses alone over a Fake with a recording Actions.
func atCoursesDirect(t *testing.T, ph kittest.Phone, svc func(*shelltest.Fake) env.Services) (*screens.Courses, *recorder, *shelltest.Rig) {
	t.Helper()
	f, err := shelltest.New(shelltest.MockFS(t))
	require.NoError(t, err)
	require.NoError(t, f.SeedSession())
	signed, err := f.Restore(context.Background())
	require.NoError(t, err)
	require.True(t, signed)
	var s env.Services = f
	if svc != nil {
		s = svc(f)
	}
	rec := &recorder{}
	e := env.New(s, locales.MustTexts())
	e.Act = rec
	c := screens.NewCourses(e)
	tt := kittest.MountReal(t, ph, c)
	kittest.PinMotion(tt)
	tt.RunUntilIdle()

	return c, rec, &shelltest.Rig{Phone: ph, Fake: f, Tester: tt}
}

// dupCourses is Services whose Courses answers what the app's page loop makes of the
// mock's `paged` scenario: its first page twice, the cursor c_6 coming back.
type dupCourses struct {
	*shelltest.Fake
}

// Courses returns the 6 courses of the first page twice.
func (d dupCourses) Courses(ctx context.Context) (model.CoursePage, error) {
	p, err := d.Fake.Courses(ctx)
	if err != nil {
		return p, err
	}
	first := append([]model.Course(nil), p.Items[:6]...)
	p.Items = append(first, first...)

	return p, nil
}

// TestCoursesBoxesMatchPrototype lays Courses out with the iPhone's real text engine
// over the mock's `me.courses` and compares every element of M04, of M04-special (the
// filter on Đặc biệt) and of M04-empty (the search "zzz") with the prototype's within
// 1 DIP: the title, the filter, the field, the count, the 15 cards with their posters,
// names, quotes and chips, the empty text, the tab bar.
func TestCoursesBoxesMatchPrototype(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		id     string
		filter int
		query  string
		kept   int
	}{
		{"M04", 0, "", 15},
		{"M04-special", 2, "", 2},
		{"M04-empty", 0, "zzz", 0},
	} {
		t.Run(tc.id, func(t *testing.T) {
			t.Parallel()
			r := atCourses(t, kittest.Phones[0], true)
			c := coursesOf(t, r).Content()
			c.SetFilter(tc.filter)
			c.SetQuery(tc.query)
			r.Tester.RunUntilIdle()
			require.Equal(t, tc.kept, c.ShownRows())
			checkAgainstProto(t, tc.id, r.Dump(tc.id), 1, nil)
		})
	}
}

// TestCoursesOnTheWiderPhone checks the 412 wide phone's Courses against the iPhone's:
// the same column from the top (y of every element within 1.5 DIPs), what runs the
// width 19 wider, the tab bar at the bottom of the taller screen.
func TestCoursesOnTheWiderPhone(t *testing.T) {
	t.Parallel()

	rI := atCourses(t, kittest.Phones[0], true)
	rA := atCourses(t, kittest.Phones[1], true)
	a, b := rI.Dump("M04"), rA.Dump("M04")
	require.Len(t, b.Elements, len(a.Elements))
	for i, ea := range a.Elements {
		eb := b.Elements[i]
		require.Equal(t, ea.Key, eb.Key)
		if ea.Key == "div.tabs" {
			assert.InDelta(t, 892, eb.Y+eb.H, 0.01)

			break
		}
		assert.InDelta(t, ea.Y, eb.Y, 1.5, ea.Key+": y")
	}
	for _, key := range []string{"div.h1", "div.segc", "div.inp", "div.muted", "go.detail:set35", "go.detail:intro"} {
		e, ok := b.Find(key)
		require.True(t, ok, key)
		assert.InDelta(t, 20, e.X, 0.01, key+": left")
		assert.InDelta(t, 412-40, e.W, 0.01, key+": width")
	}
	for i, f := range filters {
		e, ok := b.Find(f.key)
		require.True(t, ok, f.key)
		assert.InDelta(t, 24+float64(i)*(412-40-8)/4, e.X, 0.01, f.key)
		assert.InDelta(t, (412.0-40-8)/4, e.W, 0.01, f.key)
	}
}

// TestCoursesRendersTheMockList checks the texts the mock's `me.courses` makes the
// screen show: the title, the filter with the number on "Tất cả" only, the field's
// hint, the count, a card's name, lessons, quote and chips, and the dash of a course
// with no tagline.
func TestCoursesRendersTheMockList(t *testing.T) {
	t.Parallel()

	kittest.ForEachPhone(t, func(t *testing.T, ph kittest.Phone) {
		r := atCourses(t, ph, false)
		for _, s := range []string{
			"Khoá học của tôi", "Tất cả 15", "Thường", "Đặc biệt", "Giới thiệu",
			"Tìm khoá học theo tên…", "Hiển thị 15 khoá học",
			"DanceFIT Set 35", "10 bài", "“Cùng nhảy cùng hát”", "THƯỜNG", "MỚI",
			"DanceFIT Hot Extra", "“Hot Extra”", "ĐẶC BIỆT", "Giới thiệu DanceFIT", "3 bài", "GIỚI THIỆU", "—",
		} {
			assert.True(t, r.Tester.Shows(s), s)
		}
		assert.False(t, r.Tester.Shows("Thường · 12"), "the number is on the first button only (F-9)")
		assert.False(t, r.Tester.Shows("Đang đồng bộ khoá học…"), "the sync line is gone")
		assert.False(t, r.Tester.Shows("Không tìm thấy khoá học “”"))
		assert.False(t, r.Shell.Nav().TabBar().Hidden, "the tab bar shows")
		assert.Equal(t, 1, r.Shell.Nav().TabBar().Current)
		assert.Equal(t, 1, coursesOf(t, r).Tab())
		assert.Equal(t, 1, r.Fake.Calls("me.courses"))
		assert.Same(t, coursesOf(t, r).Scroll(), r.Shell.Nav().Screen().(nav.Scroller).Scroll())
	})
}

// TestCoursesFilterButtons checks the four filters: a tap on each shows its category,
// the count under the field follows, and the numbers on the buttons stay those of the
// answer's `counts`.
func TestCoursesFilterButtons(t *testing.T) {
	t.Parallel()

	kittest.ForEachPhone(t, func(t *testing.T, ph kittest.Phone) {
		r := atCourses(t, ph, false)
		c := coursesOf(t, r).Content()
		for i, f := range filters {
			require.NoError(t, r.Tester.Click(f.key), f.key)
			r.Tester.RunUntilIdle()
			assert.Equal(t, i, c.Filter(), f.key)
			assert.Equal(t, i, c.Segmented().Selected, f.key)
			assert.Equal(t, f.count, c.ShownRows(), f.key)
			assert.True(t, r.Tester.Shows("Hiển thị "+strconv.Itoa(f.count)+" khoá học"), f.key)
			assert.True(t, r.Tester.Shows("Tất cả 15"), "the first button counts the answer, not the filter")
			assert.Equal(t, 15, c.Count(0))
			assert.Equal(t, 12, c.Count(1))
			assert.Equal(t, 2, c.Count(2))
			assert.Equal(t, 1, c.Count(3))
			assert.Equal(t, i <= 1, r.Tester.Shows("DanceFIT Set 24"), "%s: Set 24 is a regular course", f.key)
		}
		require.NoError(t, r.Tester.Click("filter:Đặc biệt"))
		r.Tester.RunUntilIdle()
		assert.True(t, r.Tester.Shows("DanceFIT Hot Extra"))
		assert.True(t, r.Tester.Shows("DanceFIT Hot Extra 2"))
		assert.False(t, r.Tester.Shows("DanceFIT Set 35"))
		assert.False(t, r.Tester.Shows("Giới thiệu DanceFIT"))
	})
}

// TestCoursesSearchKeepsTheFocus checks that typing into the field filters as the
// letters come, never builds a widget (the field is the same one, still focused), a
// clear brings the 15 back, and Enter takes the focus away (the keyboard goes).
func TestCoursesSearchKeepsTheFocus(t *testing.T) {
	t.Parallel()

	kittest.ForEachPhone(t, func(t *testing.T, ph kittest.Phone) {
		r := atCourses(t, ph, false)
		c := coursesOf(t, r).Content()
		in := c.Input()
		tap(r, in)
		require.True(t, in.Focused(), "a tap on the field focuses it")

		require.True(t, r.Tester.Type("set 3"))
		r.Tester.RunUntilIdle()
		assert.Equal(t, "set 3", in.Text())
		assert.Equal(t, "set 3", c.Query())
		assert.Equal(t, 6, c.ShownRows())
		assert.True(t, r.Tester.Shows("Hiển thị 6 khoá học"))
		assert.True(t, in.Focused(), "still the focus")

		require.True(t, r.Tester.Type("5"))
		r.Tester.RunUntilIdle()
		assert.Equal(t, 1, c.ShownRows())
		assert.True(t, r.Tester.Shows("DanceFIT Set 35"))
		assert.False(t, r.Tester.Shows("DanceFIT Set 34"))
		assert.True(t, in.Focused())

		require.True(t, r.Tester.Type("9"))
		r.Tester.RunUntilIdle()
		assert.Equal(t, 0, c.ShownRows())
		assert.True(t, r.Tester.Shows("Không tìm thấy khoá học “set 359”"))
		assert.True(t, r.Tester.Shows("Hiển thị 0 khoá học"))
		assert.True(t, in.Focused())

		for range 7 { // "set 359" cleared key by key
			require.True(t, r.Tester.Key(keys.KeyBackspace, 0))
		}
		r.Tester.RunUntilIdle()
		assert.Equal(t, "", in.Text())
		assert.Equal(t, 15, c.ShownRows())
		assert.True(t, r.Tester.Shows("Hiển thị 15 khoá học"))
		assert.False(t, r.Tester.Shows("Không tìm thấy"))
		assert.True(t, in.Focused())

		require.True(t, r.Tester.Key(keys.KeyEnter, 0))
		assert.False(t, in.Focused(), "Enter blurs the field")
		assert.Equal(t, 15, c.ShownRows())
	})
}

// TestCoursesEmptyState checks M04-empty: the text with the raw query (not trimmed)
// between the curly quotes, the count 0, the cards gone, the filter kept; and that a
// filter alone can empty the list too ("Giới thiệu" with a query only Thường has).
func TestCoursesEmptyState(t *testing.T) {
	t.Parallel()

	kittest.ForEachPhone(t, func(t *testing.T, ph kittest.Phone) {
		r := atCourses(t, ph, false)
		c := coursesOf(t, r).Content()
		c.SetQuery("  zzz ")
		r.Tester.RunUntilIdle()
		assert.True(t, r.Tester.Shows("Không tìm thấy khoá học “  zzz ”"), "q as typed (index.html:378)")
		assert.True(t, r.Tester.Shows("Hiển thị 0 khoá học"))
		assert.False(t, r.Tester.Shows("DanceFIT Set 35"))
		assert.True(t, r.Tester.Shows("filter:Tất cả"), "the filter stays")

		c.SetQuery("set 35")
		c.SetFilter(3)
		r.Tester.RunUntilIdle()
		assert.True(t, r.Tester.Shows("Không tìm thấy khoá học “set 35”"))
		c.SetFilter(1)
		r.Tester.RunUntilIdle()
		assert.False(t, r.Tester.Shows("Không tìm thấy"))
		assert.True(t, r.Tester.Shows("DanceFIT Set 35"))
	})
}

// TestCoursesCardOpensTheCourse checks a tap on a card: the Actions get the course's id
// (with its dash), through the Shell the route is the detail with the id as its
// params and Courses under it, and Back returns to the list with the filter and the
// search where they were left (`S.m.filter`, `S.m.q`).
func TestCoursesCardOpensTheCourse(t *testing.T) {
	t.Parallel()

	kittest.ForEachPhone(t, func(t *testing.T, ph kittest.Phone) {
		c, rec, r := atCoursesDirect(t, ph, nil)
		require.NoError(t, r.Tester.Click("go.detail:hotextra"))
		assert.Equal(t, []string{"detail hot-extra"}, rec.calls)
		require.NotNil(t, c)

		sr := atCourses(t, ph, false)
		cc := coursesOf(t, sr).Content()
		cc.SetFilter(1)
		cc.SetQuery("set 3")
		sr.Tester.RunUntilIdle()
		require.NoError(t, sr.Tester.Click("go.detail:set34"))
		sr.Settle()
		assert.Equal(t, "detail", sr.Route())
		assert.Equal(t, "set-34", sr.Shell.Nav().Current().Params)
		assert.Equal(t, 1, sr.Shell.Nav().Depth())

		assert.True(t, sr.Shell.Nav().Back())
		sr.Tester.Elapse(kittest.Frame)
		sr.Settle()
		assert.Equal(t, "courses", sr.Route())
		back := coursesOf(t, sr).Content()
		assert.NotSame(t, cc, back, "the screen was built again")
		assert.Equal(t, 1, back.Filter())
		assert.Equal(t, "set 3", back.Query())
		assert.Equal(t, "set 3", back.Input().Text())
		assert.Equal(t, 6, back.ShownRows())
		assert.True(t, sr.Tester.Shows("Hiển thị 6 khoá học"))
		assert.True(t, sr.Tester.Shows("filter:Thường"))
		assert.Equal(t, 1, back.Segmented().Selected)
	})
}

// TestCoursesTabBarGoesToTheTabs checks the bar under Courses: the tabs are found by
// place, their names being words the screen has as well.
func TestCoursesTabBarGoesToTheTabs(t *testing.T) {
	t.Parallel()

	kittest.ForEachPhone(t, func(t *testing.T, ph kittest.Phone) {
		for col, route := range map[float32]string{0: "home", 2: "downloads", 3: "account"} {
			r := atCourses(t, ph, false)
			bar := r.Shell.Nav().TabBar().Node().WindowFrame()
			r.Tester.ClickAt(geom.Pt(bar.X+(col+.5)*bar.W/4, bar.Y+bar.H*.4))
			r.Settle()
			assert.Equal(t, route, r.Route())
		}
	})
}

// TestCoursesOfflineBar checks 03 section 2: the bar appears when the network goes
// with the page under it (everything 40 lower, scroll top 54 to 94), the cards on
// show stay and the search works on them, a card opens the offline screen of its
// course, and when the network is back the bar goes and the list is read again with
// the filter and the text kept.
func TestCoursesOfflineBar(t *testing.T) {
	t.Parallel()

	kittest.ForEachPhone(t, func(t *testing.T, ph kittest.Phone) {
		r := atCourses(t, ph, true)
		c := coursesOf(t, r).Content()
		before := r.Dump("M04")
		assert.False(t, r.Tester.Shows("Không có mạng · Đang dùng chế độ ngoại tuyến"))

		r.Fake.SetOnline(false)
		r.Settle()
		off := r.Dump("M04-offline")
		assert.True(t, r.Tester.Shows("Không có mạng · Đang dùng chế độ ngoại tuyến"))
		assert.True(t, r.Tester.Shows("DanceFIT Set 35"), "the list on show stays")
		bar, ok := off.Find("div.offbar")
		require.True(t, ok)
		assert.InDelta(t, 54, bar.Y, 0.01)
		assert.InDelta(t, 40, bar.H, 0.01)
		top, _ := off.Find("div.scroll")
		assert.InDelta(t, 94, top.Y, 0.01, "`.scroll` at top:94px (:374)")
		for _, key := range []string{"div.h1", "div.segc", "filter:Thường", "div.inp", "div.muted", "go.detail:set35", "go.detail:intro"} {
			a, _ := before.Find(key)
			b, ok := off.Find(key)
			require.True(t, ok, key)
			assert.InDelta(t, a.Y+40, b.Y, 0.01, key+": 40 lower under the bar")
			assert.InDelta(t, a.H, b.H, 0.01, key)
		}

		c.SetFilter(2)
		c.SetQuery("extra 2")
		r.Tester.RunUntilIdle()
		assert.Equal(t, 1, c.ShownRows(), "the search works offline")
		require.NoError(t, r.Tester.Click("go.detail:hotextra2"))
		r.Settle()
		assert.Equal(t, "offline", r.Route(), "a course opens its offline screen")
		assert.Equal(t, "hot-extra-2", r.Shell.Nav().Current().Params)

		r = atCourses(t, ph, true)
		c = coursesOf(t, r).Content()
		r.Fake.SetOnline(false)
		r.Settle()
		c.SetFilter(1)
		c.SetQuery("set 3")
		r.Tester.RunUntilIdle()
		r.Fake.SetOnline(true)
		r.Settle()
		assert.False(t, r.Tester.Shows("Không có mạng · Đang dùng chế độ ngoại tuyến"))
		on := r.Dump("M04")
		top, _ = on.Find("div.scroll")
		assert.InDelta(t, 54, top.Y, 0.01)
		assert.Equal(t, 2, r.Fake.Calls("me.courses"), "the list is read again when the network is back")
		assert.Equal(t, 6, c.ShownRows(), "with the filter and the text kept")
		assert.Equal(t, "set 3", c.Query())
	})
}

// TestCoursesColdStartOffline checks a first read with no network and no cache: the
// title, the filter and the field with "Thử lại" under them, a retry while still
// offline fails the same way, typing before the list is there shows nothing odd, and
// the network coming back reads the list and shows it.
func TestCoursesColdStartOffline(t *testing.T) {
	t.Parallel()

	kittest.ForEachPhone(t, func(t *testing.T, ph kittest.Phone) {
		f, err := shelltest.New(shelltest.MockFS(t))
		require.NoError(t, err)
		require.NoError(t, f.SeedSession())
		r := shelltest.MountRig(t, ph, false, f, shellOptions())
		r.Boot()
		f.SetOnline(false)
		r.Settle()
		goCourses(r)
		c := coursesOf(t, r).Content()
		assert.Equal(t, 0, c.Rows())
		assert.True(t, r.Tester.Shows("Khoá học của tôi"))
		assert.True(t, r.Tester.Shows("Thử lại"))
		assert.False(t, r.Tester.Shows("Đang đồng bộ khoá học…"))
		assert.False(t, r.Tester.Shows("Không tìm thấy"), "no list is not an empty one")

		c.SetQuery("set")
		r.Tester.RunUntilIdle()
		assert.False(t, r.Tester.Shows("Không tìm thấy"))

		require.NoError(t, r.Tester.Click("retry"))
		r.Settle()
		assert.Equal(t, 0, c.Rows(), "still offline")
		assert.True(t, r.Tester.Shows("Thử lại"))
		assert.Equal(t, 2, f.Calls("me.courses"))

		f.SetOnline(true)
		r.Settle()
		assert.Equal(t, 15, c.Rows())
		assert.False(t, r.Tester.Shows("Thử lại"))
		assert.Equal(t, 12, c.ShownRows(), `the text typed meanwhile applies: "set" is in 12 names`)
	})
}

// TestCoursesServerErrorShowsRetry checks a failing GET /me/courses: "Thử lại", and a
// tap, the server well again, shows the list.
func TestCoursesServerErrorShowsRetry(t *testing.T) {
	t.Parallel()

	kittest.ForEachPhone(t, func(t *testing.T, ph kittest.Phone) {
		f, err := shelltest.New(shelltest.MockFS(t))
		require.NoError(t, err)
		require.NoError(t, f.SeedSession())
		r := shelltest.MountRig(t, ph, false, f, shellOptions())
		r.Boot()
		f.Fail("me.courses", 500, "errors/server_error.json")
		goCourses(r)
		require.Equal(t, "courses", r.Route())
		assert.True(t, r.Tester.Shows("Thử lại"))
		assert.Equal(t, 0, coursesOf(t, r).Content().Rows())

		f.Fail("me.courses", 0, "")
		require.NoError(t, r.Tester.Click("retry"))
		r.Settle()
		assert.Equal(t, 15, coursesOf(t, r).Content().Rows())
		assert.False(t, r.Tester.Shows("Thử lại"))
	})
}

// TestCoursesLeftWhileLoadingShowsNothing checks that an answer arriving after the
// screen was left is dropped: no error, no cards, and the screen now showing is not
// touched.
func TestCoursesLeftWhileLoadingShowsNothing(t *testing.T) {
	t.Parallel()

	kittest.ForEachPhone(t, func(t *testing.T, ph kittest.Phone) {
		f, err := shelltest.New(shelltest.MockFS(t))
		require.NoError(t, err)
		require.NoError(t, f.SeedSession())
		r := shelltest.MountRig(t, ph, false, f, shellOptions())
		r.Boot()
		release := f.Gate("me.courses")
		t.Cleanup(release)
		r.Shell.Nav().Go(nav.Route{Screen: "courses"}, true)
		r.Tester.Elapse(kittest.Frame)
		c := coursesOf(t, r)
		assert.True(t, r.Tester.Shows("Đang đồng bộ khoá học…"), "the sync line while the list is read")
		r.Shell.Nav().Go(nav.Route{Screen: "account"}, true)
		release()
		r.Settle()
		r.Tester.Elapse(kittest.Frame)
		assert.Equal(t, "account", r.Route())
		assert.Equal(t, 0, c.Content().Rows())
	})
}

// TestCoursesPagesOverlapShowEachCourseOnce checks the list the page loop makes of the
// mock's `paged` scenario, whose cursor c_6 comes back: the same 6 courses twice show
// 6 cards, and the count under the field says 6.
func TestCoursesPagesOverlapShowEachCourseOnce(t *testing.T) {
	t.Parallel()

	kittest.ForEachPhone(t, func(t *testing.T, ph kittest.Phone) {
		c, _, r := atCoursesDirect(t, ph, func(f *shelltest.Fake) env.Services { return dupCourses{f} })
		assert.Equal(t, 6, c.Content().Rows())
		assert.Equal(t, 6, c.Content().ShownRows())
		assert.True(t, r.Tester.Shows("Hiển thị 6 khoá học"))
		assert.True(t, r.Tester.Shows("Tất cả 15"), "the button counts the answer's `counts`")
	})
}

// TestCoursesKeepsTheScrollWhenFiltered checks that the scroll offset stays where it
// is when a filter leaves a shorter list, clamped to its end, and that a list that
// fits ends it at the top.
func TestCoursesKeepsTheScrollWhenFiltered(t *testing.T) {
	t.Parallel()

	kittest.ForEachPhone(t, func(t *testing.T, ph kittest.Phone) {
		r := atCourses(t, ph, false)
		c := coursesOf(t, r)
		sc := c.Scroll()
		sc.ScrollTo(geom.Pt(0, 400), false)
		r.Tester.Elapse(kittest.Frame)
		assert.InDelta(t, 400, sc.Offset().Y, 0.01)

		c.Content().SetQuery("set 3") // 6 cards: 6*116 + 197 + 120 less the window
		r.Tester.Elapse(kittest.Frame)
		assert.InDelta(t, sc.MaxOffset().Y, sc.Offset().Y, 0.01, "clamped to the end")
		assert.Less(t, sc.Offset().Y, float32(400))

		c.Content().SetQuery("zzz")
		r.Tester.Elapse(kittest.Frame)
		assert.InDelta(t, 0, sc.Offset().Y, 0.01, "the empty text fits")
		assert.Equal(t, float32(0), sc.MaxOffset().Y)

		c.Content().SetQuery("")
		r.Tester.Elapse(kittest.Frame)
		assert.Equal(t, 15, c.Content().ShownRows())
	})
}

// TestCoursesSteadyFrameDoesNotAllocate checks that a frame of Courses standing still
// allocates nothing, with the list as it is and with a search that filters it, on
// both phones.
func TestCoursesSteadyFrameDoesNotAllocate(t *testing.T) { //nolint:paralleltest // AllocsPerRun needs the process to itself
	if kittest.Race {
		t.Skip("allocation counts are not stable under the race detector")
	}
	for _, ph := range kittest.Phones {
		r := atCourses(t, ph, false)
		warmCourses(r)
		assert.Zero(t, testing.AllocsPerRun(50, r.Tester.Repaint), ph.Name)

		coursesOf(t, r).Content().SetQuery("set 3")
		warmCourses(r)
		assert.Zero(t, testing.AllocsPerRun(50, r.Tester.Repaint), ph.Name+": filtered")
	}
}

// TestCoursesScrollFrameDoesNotAllocate checks that a frame of the page moving allocates
// nothing.
func TestCoursesScrollFrameDoesNotAllocate(t *testing.T) { //nolint:paralleltest // AllocsPerRun needs the process to itself
	if kittest.Race {
		t.Skip("allocation counts are not stable under the race detector")
	}
	for _, ph := range kittest.Phones {
		r := atCourses(t, ph, false)
		sc := coursesOf(t, r).Scroll()
		warmCourses(r)
		y := float32(0)
		for ; y < 1500; y += 40 { // every card is on screen once: laid out, shaped, drawn
			sc.ScrollTo(geom.Pt(0, y), false)
			r.Tester.Elapse(kittest.Frame)
		}
		dir := float32(-1)
		page := func() {
			if y > 1500 || y < 1000 {
				dir = -dir
			}
			y += 3 * dir
			sc.ScrollTo(geom.Pt(0, y), false)
			r.Tester.Frame()
		}
		for range 16 {
			page()
		}
		assert.Zero(t, testing.AllocsPerRun(50, page), ph.Name)
	}
}

// BenchmarkCoursesSteadyFrame is a frame of Courses standing still.
func BenchmarkCoursesSteadyFrame(b *testing.B) {
	r := atCourses(b, kittest.Phones[0], false)
	warmCourses(r)
	b.ReportAllocs()
	for b.Loop() {
		r.Tester.Repaint()
	}
}

// ====================================================================
// ========================= Helper Functions =========================
// ====================================================================

// goCourses opens the Courses tab and lets its read run: the screen starts it with the
// frame after it is mounted.
func goCourses(r *shelltest.Rig) {
	r.Shell.Nav().Go(nav.Route{Screen: "courses"}, true)
	r.Tester.Elapse(kittest.Frame)
	r.Settle()
}

// warmCourses lets the frames run until what Courses draws is laid out and shaped
// once.
func warmCourses(r *shelltest.Rig) {
	for range 8 {
		r.Tester.Elapse(kittest.Frame)
	}
	r.Tester.Repaint()
}
