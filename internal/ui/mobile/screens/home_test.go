//go:build !tvos

// Pattern from jplayer-mobile/internal/ui/views/alloc_spec_test.go and home_test.go
// at 49de3c2 (steady: warm up, then AllocsPerRun; a scroll step within what is
// laid out; the page dragged by a finger).

package screens_test

import (
	"context"
	"strconv"
	"strings"
	"testing"
	"time"

	"dancefitvn-app/internal/model"
	"dancefitvn-app/internal/ui/kit"
	"dancefitvn-app/internal/ui/kit/kittest"
	"dancefitvn-app/internal/ui/mobile/env"
	"dancefitvn-app/internal/ui/mobile/nav"
	"dancefitvn-app/internal/ui/mobile/screens"
	"dancefitvn-app/internal/ui/mobile/shell/shelltest"
	"dancefitvn-app/locales"

	"github.com/gdeskdev/desk/foundation/core/geom"
	"github.com/gdeskdev/desk/foundation/scene"
	"github.com/gdeskdev/desk/framework/ui"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ====================================================================
// ============================ Contracts =============================
// ====================================================================

// recorder is env.Actions that writes down what the screen asked.
type recorder struct {
	calls []string
}

var _ env.Actions = (*recorder)(nil)

func (r *recorder) note(s string) { r.calls = append(r.calls, s) }
func (r *recorder) Go(rt nav.Route, root bool) {
	r.note("go " + rt.Screen + " " + strconv.FormatBool(root))
}
func (r *recorder) Back()            { r.note("back") }
func (r *recorder) Toast(msg string) { r.note("toast " + msg) }
func (r *recorder) SignedIn()        { r.note("signedIn") }
func (r *recorder) Detail(id string) { r.note("detail " + id) }
func (r *recorder) PlayVideo(c, l string, no int) {
	r.note("playVideo " + c + " " + l + " " + strconv.Itoa(no))
}
func (r *recorder) PlayAudio(c, l string, no int) {
	r.note("playAudio " + c + " " + l + " " + strconv.Itoa(no))
}
func (r *recorder) OffTap(c, l string, no int) {
	r.note("offTap " + c + " " + l + " " + strconv.Itoa(no))
}
func (r *recorder) DlTap(c, l string, no int) {
	r.note("dlTap " + c + " " + l + " " + strconv.Itoa(no))
}
func (r *recorder) DlAll(c string) { r.note("dlAll " + c) }
func (r *recorder) AskDelete(c, l string, no int) {
	r.note("askDelete " + c + " " + l + " " + strconv.Itoa(no))
}
func (r *recorder) StartDownloads(c model.Course, ls []model.Lesson) {
	r.note("startDownloads " + c.ID + " " + strconv.Itoa(len(ls)))
}
func (r *recorder) DeleteDownload(l string) { r.note("deleteDownload " + l) }
func (r *recorder) PlayNext()               { r.note("playNext") }
func (r *recorder) AskPassword()            { r.note("askPassword") }
func (r *recorder) AskLogout()              { r.note("askLogout") }
func (r *recorder) AskAvatar()              { r.note("askAvatar") }
func (r *recorder) AskDevices(p env.DevicesSheetParams) {
	r.note("askDevices " + p.Code + " " + strconv.Itoa(len(p.Devices)))
}
func (r *recorder) Logout() { r.note("logout") }
func (r *recorder) ToDetail(c string) {
	r.note("toDetail " + c)
}

// clock puts the device's clock at t before the boot.
func clock(t time.Time) setup {
	return func(_ *testing.T, f *shelltest.Fake) { f.SetNow(t) }
}

// scenario picks the mock's scenario of a route before the boot.
func scenario(route, name string) setup {
	return func(t *testing.T, f *shelltest.Fake) { require.NoError(t, f.SetScenario(route, name)) }
}

// atHome boots a signed-in app to Home with its data in.
func atHome(tb testing.TB, ph kittest.Phone, real bool, setups ...setup) *shelltest.Rig {
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
	require.NotNil(tb, homeOf(tb, r).Content(), "the data is in")

	return r
}

// homeOf returns the Home screen showing.
func homeOf(t testing.TB, r *shelltest.Rig) *screens.Home {
	t.Helper()
	h, ok := r.Shell.Nav().Screen().(*screens.Home)
	require.True(t, ok, "the screen showing is Home")

	return h
}

// atHomeDirect mounts Home alone over a Fake with a recording Actions, the data in.
func atHomeDirect(t *testing.T, ph kittest.Phone, setups ...setup) (*screens.Home, *shelltest.Fake, *recorder, *shelltest.Rig) {
	t.Helper()
	f, err := shelltest.New(shelltest.MockFS(t))
	require.NoError(t, err)
	require.NoError(t, f.SeedSession())
	signed, err := f.Restore(context.Background()) // what the boot does: the vault's session is the client's
	require.NoError(t, err)
	require.True(t, signed)
	for _, s := range setups {
		s(t, f)
	}
	rec := &recorder{}
	e := env.New(f, locales.MustTexts())
	e.Act = rec
	h := screens.NewHome(e)
	tt := kittest.MountReal(t, ph, h)
	kittest.PinMotion(tt)
	tt.RunUntilIdle()
	require.NotNil(t, h.Content(), "the data is in")

	return h, f, rec, &shelltest.Rig{Phone: ph, Fake: f, Tester: tt}
}

// ====================================================================
// ========================== Main functions ==========================
// ====================================================================

// homeGridKeys are the keys of M03 that K39 moves: the six cards of the strip with
// their posters, titles and quotes, and the update with what is in it (the grid is
// two columns and the update sits under it and the button). The rest of the capture
// stays as it is.
var homeGridKeys = func() map[string]bool {
	keys := []string{
		"go.detail:set35", "go.detail:set34", "go.detail:hotextra", "go.detail:set33", "go.detail:set32", "go.detail:set31",
		"div.sec#2", "span.d#2", "go.detail:set36", "div.poster#8", "div.ttl#7", "s#7", "b#7", "div.muted#10", "span.chip.c-th",
	}
	keys = append(keys, "div.ttl", "s", "b") // the first card's title (the continue card's poster has none)
	for i := 2; i <= 7; i++ {
		n := "#" + strconv.Itoa(i)
		keys = append(keys, "div.poster"+n, "div.ttl"+n, "s"+n, "b"+n)
	}
	for i := 4; i <= 9; i++ {
		keys = append(keys, "div.muted#"+strconv.Itoa(i))
	}
	m := map[string]bool{}
	for _, k := range keys {
		m[k] = true
	}

	return m
}()

// TestHomeBoxesMatchPrototype lays Home out with the iPhone's real text engine
// over the mock's `me.home` and compares every element of M03 that K39 leaves with
// the prototype's within 1 DIP: the greeting, the continue card and what is over
// it, the three numbers, the section head of the courses, the tab bar. The grid, the
// button under it and the update that K39 moves are checked in
// TestHomeCourseGridGeometry and TestHomeUpdateFollowsTheGrid. The text "MỚI CẬP
// NHẬT" is 1.24 narrower in Core Text than in Chrome (a shaping difference of the
// Vietnamese capitals' marks), checked there.
func TestHomeBoxesMatchPrototype(t *testing.T) {
	t.Parallel()

	r := atHome(t, kittest.Phones[0], true)
	lay := r.Dump("M03")
	checkAgainstProto(t, "M03", lay, 1, homeGridKeys)
}

// TestHomeOnTheWiderPhone checks the 412 wide phone's Home against the iPhone's:
// the same column from the top (y of every element within 1.5 DIPs), what runs the
// width 19 wider, the numbers sharing the 372 between them, the avatar and the link
// moved by the 19 (K39: the grid's columns share the 372).
func TestHomeOnTheWiderPhone(t *testing.T) {
	t.Parallel()

	rI := atHome(t, kittest.Phones[0], true)
	rA := atHome(t, kittest.Phones[1], true)
	a, b := rI.Dump("M03"), rA.Dump("M03")
	require.Len(t, b.Elements, len(a.Elements))
	for i, ea := range a.Elements {
		eb := b.Elements[i]
		require.Equal(t, ea.Key, eb.Key)
		if ea.Key == "div.tabs" { // the bar sits at the bottom of the taller screen
			assert.InDelta(t, 892, eb.Y+eb.H, 0.01)

			break
		}
		if homeGridKeys[ea.Key] { // K39: the grid is wider, so what is under it moves
			break
		}
		assert.InDelta(t, ea.Y, eb.Y, 1.5, ea.Key+": y")
	}
	full := []string{"playVideo:set35-3", "div.poster", "div.sec", "div.sec#2", "go.detail:set36", "go.courses.all"}
	for _, key := range full {
		e, ok := b.Find(key)
		require.True(t, ok, key)
		assert.InDelta(t, 20, e.X, 0.01, key+": left")
		assert.InDelta(t, 412-40, e.W, 0.01, key+": width")
	}
	third := (412.0 - 40 - 16) / 3
	for i, key := range []string{"div.card", "div.card#2", "div.card#3"} {
		e, _ := b.Find(key)
		assert.InDelta(t, third, e.W, 0.05, key+": a third of the room")
		assert.InDelta(t, 20+float64(i)*(third+8), e.X, 0.05, key)
	}
	for key, dx := range map[string]float64{"go.account": 19, "go.courses": 19} {
		ea, _ := a.Find(key)
		eb, _ := b.Find(key)
		assert.InDelta(t, ea.X+dx, eb.X, 0.5, key+": moves with the right edge")
	}
	// the continue card's text and disc follow the card's right edge
	for _, key := range []string{"div.d#2", "div.prog"} {
		e, _ := b.Find(key)
		assert.InDelta(t, 412-40-16-80, e.W, 0.01, key+": the text block is 96 less than the card")
	}
}

// TestHomeRendersTheMockHome checks the texts the mock's `me.home` makes Home show:
// the greeting, the continue card's line, title and chip, the three numbers with
// their labels, the section heads, the update's line and chip.
func TestHomeRendersTheMockHome(t *testing.T) {
	t.Parallel()

	kittest.ForEachPhone(t, func(t *testing.T, ph kittest.Phone) {
		r := atHome(t, ph, false)
		for _, s := range []string{
			"Thứ bảy, 14/11 · Chào buổi sáng",
			"Xin chào, Thảo Vy",
			"TIẾP TỤC XEM",
			"DanceFIT Set 35 · Bài 03 · Bụng · còn 2 phút",
			"Stink and Dutty",
			"15", "Khoá học", "10", "Bài · Set 35", "5", "Audio offline",
			"Khoá học của tôi", "Xem tất cả",
			"Mới cập nhật", "DanceFIT Set 36", "Khoá mới · bạn chưa tham gia", "THƯỜNG · 10 BÀI",
			"DanceFIT Hot Extra", "Đặc biệt · 10 BÀI",
		} {
			assert.True(t, r.Tester.Shows(s), s)
		}
		assert.True(t, r.Tester.Shows("go.account"), "the avatar")
		_, ok := r.Shell.Nav().Screen().(*screens.Home)
		assert.True(t, ok)
		assert.False(t, r.Shell.Nav().TabBar().Hidden, "the Home tab's bar shows")
		assert.False(t, r.Tester.Shows("Đang đồng bộ khoá học…"), "the skeleton is gone")
		assert.Equal(t, 1, r.Fake.Calls("me.home"))
	})
}

// TestHomeEyebrowByWeekdayAndHour checks the eyebrow's template: the weekday of the
// clock, dd/MM, and the greeting by the hour (before 12 morning, before 18
// afternoon, else evening, Q-P3-15).
func TestHomeEyebrowByWeekdayAndHour(t *testing.T) {
	t.Parallel()

	zone := time.FixedZone("+07", 7*3600)
	tests := []struct {
		at   time.Time
		want string
	}{
		{time.Date(2026, 11, 15, 0, 0, 0, 0, zone), "Chủ nhật, 15/11 · Chào buổi sáng"},
		{time.Date(2026, 11, 16, 11, 59, 0, 0, zone), "Thứ hai, 16/11 · Chào buổi sáng"},
		{time.Date(2026, 11, 17, 12, 0, 0, 0, zone), "Thứ ba, 17/11 · Chào buổi chiều"},
		{time.Date(2026, 11, 18, 17, 59, 0, 0, zone), "Thứ tư, 18/11 · Chào buổi chiều"},
		{time.Date(2026, 11, 19, 18, 0, 0, 0, zone), "Thứ năm, 19/11 · Chào buổi tối"},
		{time.Date(2026, 11, 20, 23, 59, 0, 0, zone), "Thứ sáu, 20/11 · Chào buổi tối"},
		{shelltest.DemoNow, "Thứ bảy, 14/11 · Chào buổi sáng"},
	}
	for _, tc := range tests {
		t.Run(tc.want, func(t *testing.T) {
			t.Parallel()
			kittest.ForEachPhone(t, func(t *testing.T, ph kittest.Phone) {
				r := atHome(t, ph, false, clock(tc.at))
				assert.True(t, r.Tester.Shows(tc.want), tc.want)
			})
		})
	}
}

// TestHomeTapsAskTheActions checks what every tap on Home asks of the Actions: the
// continue card plays the lesson, a card of the grid and the update open their
// course, the link, the button under the grid and the avatar go to their tabs as
// roots.
func TestHomeTapsAskTheActions(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		key  string
		want string
	}{
		{"continue card", "playVideo:set35-3", "playVideo set-35 l_3503 3"},
		{"first card of the grid", "go.detail:set35", "detail set-35"},
		{"second column", "go.detail:set34", "detail set-34"},
		{"special course", "go.detail:hotextra", "detail hot-extra"},
		{"last card", "go.detail:set31", "detail set-31"},
		{"all courses button", "go.courses.all", "go courses true"},
		{"update (not enrolled)", "go.detail:set36", "detail set-36"},
		{"see all", "go.courses", "go courses true"},
		{"avatar", "go.account", "go account true"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			kittest.ForEachPhone(t, func(t *testing.T, ph kittest.Phone) {
				h, _, rec, r := atHomeDirect(t, ph)
				reveal(r, h, tc.key)
				require.NoError(t, r.Tester.Click(tc.key))
				assert.Equal(t, []string{tc.want}, rec.calls)
			})
		})
	}
}

// TestHomeTapsOpenTheRoutes runs the taps through the Shell: a card goes to the
// detail route of its course (id with the dash, K10), the update of a course the
// user is not in goes to the same route (05B is the detail's), the link and the
// avatar to their tabs with the stack empty, and, offline, a card goes to the
// offline screen and the continue card toasts that the video needs the network.
func TestHomeTapsOpenTheRoutes(t *testing.T) {
	t.Parallel()

	kittest.ForEachPhone(t, func(t *testing.T, ph kittest.Phone) {
		for _, tc := range []struct{ key, route, param string }{
			{"go.detail:set35", "detail", "set-35"},
			{"go.detail:set36", "detail", "set-36"},
			{"go.courses", "courses", ""},
			{"go.account", "account", ""},
		} {
			r := atHome(t, ph, false)
			reveal(r, homeOf(t, r), tc.key)
			require.NoError(t, r.Tester.Click(tc.key), tc.key)
			r.Settle()
			assert.Equal(t, tc.route, r.Route(), tc.key)
			if tc.param != "" {
				assert.Equal(t, tc.param, r.Shell.Nav().Current().Params, tc.key)
				assert.Equal(t, 1, r.Shell.Nav().Depth(), "a card opens over Home")
			} else {
				assert.Equal(t, 0, r.Shell.Nav().Depth(), tc.key+": a root")
			}
		}

		r := atHome(t, ph, false)
		r.Fake.SetOnline(false)
		r.Settle()
		require.NoError(t, r.Tester.Click("go.detail:set35"))
		assert.Equal(t, "offline", r.Route(), "offline a course opens its offline screen")

		r = atHome(t, ph, false)
		r.Fake.SetOnline(false)
		r.Settle()
		require.NoError(t, r.Tester.Click("playVideo:set35-3"))
		assert.True(t, r.Tester.Shows("Video bài 03 cần có mạng để xem"))
		assert.Equal(t, "home", r.Route())
	})
}

// TestHomeTabBarGoesToTheTabs checks the bar under Home: Khoá học and Tài khoản are
// found by place, as the names of the link and the avatar are the same words.
func TestHomeTabBarGoesToTheTabs(t *testing.T) {
	t.Parallel()

	kittest.ForEachPhone(t, func(t *testing.T, ph kittest.Phone) {
		for col, route := range map[float32]string{1: "courses", 2: "downloads", 3: "account"} {
			r := atHome(t, ph, false)
			bar := r.Shell.Nav().TabBar().Node().WindowFrame()
			r.Tester.ClickAt(geom.Pt(bar.X+(col+.5)*bar.W/4, bar.Y+bar.H*.4))
			r.Settle()
			assert.Equal(t, route, r.Route())
		}
	})
}

// TestHomeOfflineBar checks the offline bar: it appears when the network goes, with
// the page under it (scroll top 54 to 94, everything 40 lower), the data on show
// stays, and it goes when the network is back, the data read again.
func TestHomeOfflineBar(t *testing.T) {
	t.Parallel()

	kittest.ForEachPhone(t, func(t *testing.T, ph kittest.Phone) {
		r := atHome(t, ph, true)
		before := r.Dump("M03")
		top, _ := before.Find("div.scroll")
		assert.InDelta(t, 54, top.Y, 0.01)
		assert.False(t, r.Tester.Shows("Không có mạng · Đang dùng chế độ ngoại tuyến"))

		r.Fake.SetOnline(false)
		r.Settle()
		off := r.Dump("M03-offline")
		assert.True(t, r.Tester.Shows("Không có mạng · Đang dùng chế độ ngoại tuyến"))
		assert.True(t, r.Tester.Shows("Xin chào, Thảo Vy"), "the data on show stays")
		assert.Len(t, homeOf(t, r).Content().Cards(), 6, "the grid stays as it was (cached home)")
		assert.NotNil(t, homeOf(t, r).Content().AllButton(), "and the button under it")
		bar, ok := off.Find("div.offbar")
		require.True(t, ok)
		assert.InDelta(t, 54, bar.Y, 0.01)
		assert.InDelta(t, 40, bar.H, 0.01)
		top, _ = off.Find("div.scroll")
		assert.InDelta(t, 94, top.Y, 0.01, "`#m .scroll` at top:94px (:362)")
		for _, key := range []string{"div.eb", "playVideo:set35-3", "div.card", "div.sec", "go.detail:set35", "go.detail:set36"} {
			a, _ := before.Find(key)
			b, ok := off.Find(key)
			require.True(t, ok, key)
			assert.InDelta(t, a.Y+40, b.Y, 0.01, key+": 40 lower under the bar")
			assert.InDelta(t, a.H, b.H, 0.01, key)
		}

		r.Fake.SetOnline(true)
		r.Settle()
		assert.False(t, r.Tester.Shows("Không có mạng · Đang dùng chế độ ngoại tuyến"))
		on := r.Dump("M03")
		top, _ = on.Find("div.scroll")
		assert.InDelta(t, 54, top.Y, 0.01)
		assert.Equal(t, 2, r.Fake.Calls("me.home"), "the data is read again when the network is back")
	})
}

// TestHomeColdStartOffline checks a launch with no network and no cache: the
// skeleton stays with "Thử lại", a retry while still offline fails the same way, and
// the network coming back reads the data and shows the content.
func TestHomeColdStartOffline(t *testing.T) {
	t.Parallel()

	kittest.ForEachPhone(t, func(t *testing.T, ph kittest.Phone) {
		r := newRig(t, ph, false, shellOptions(), signed())
		r.Settle()
		r.Fake.SetOnline(false)
		r.Tester.Elapse(600 * time.Millisecond)
		r.Settle()
		require.Equal(t, "home", r.Route())
		h := homeOf(t, r)
		assert.Nil(t, h.Content())
		assert.True(t, r.Tester.Shows("Thử lại"))
		assert.False(t, r.Tester.Shows("Đang đồng bộ khoá học…"))

		require.NoError(t, r.Tester.Click("retry"))
		r.Settle()
		assert.Nil(t, h.Content(), "still offline")
		assert.True(t, r.Tester.Shows("Thử lại"))
		assert.Equal(t, 2, r.Fake.Calls("me.home"))

		r.Fake.SetOnline(true)
		r.Settle()
		require.NotNil(t, h.Content())
		assert.True(t, r.Tester.Shows("Xin chào, Thảo Vy"))
		assert.False(t, r.Tester.Shows("Thử lại"))
	})
}

// TestHomeServerErrorShowsRetry checks a failing GET /me/home: the skeleton and
// "Thử lại", and a tap, the server well again, shows the content.
func TestHomeServerErrorShowsRetry(t *testing.T) {
	t.Parallel()

	kittest.ForEachPhone(t, func(t *testing.T, ph kittest.Phone) {
		f, err := shelltest.New(shelltest.MockFS(t))
		require.NoError(t, err)
		require.NoError(t, f.SeedSession())
		f.Fail("me.home", 500, "errors/server_error.json")
		r := shelltest.MountRig(t, ph, false, f, shellOptions())
		r.Boot()
		require.Equal(t, "home", r.Route())
		assert.True(t, r.Tester.Shows("Thử lại"))
		assert.Nil(t, homeOf(t, r).Content())

		f.Fail("me.home", 0, "")
		require.NoError(t, r.Tester.Click("retry"))
		r.Settle()
		assert.NotNil(t, homeOf(t, r).Content())
		assert.True(t, r.Tester.Shows("Xin chào, Thảo Vy"))
	})
}

// TestHomeFirstReadRecovers is defect 3.5: after a first read that failed the screen
// reads again by itself (2 s, then 6 s later), the button staying until the content
// is in; the third failure leaves the button alone, and a tap starts the count again.
func TestHomeFirstReadRecovers(t *testing.T) {
	t.Parallel()

	kittest.ForEachPhone(t, func(t *testing.T, ph kittest.Phone) {
		boot := func(t *testing.T) (*shelltest.Fake, *shelltest.Rig) {
			f, err := shelltest.New(shelltest.MockFS(t))
			require.NoError(t, err)
			require.NoError(t, f.SeedSession())
			f.Fail("me.home", 500, "errors/server_error.json")
			r := shelltest.MountRig(t, ph, false, f, shellOptions())
			r.Boot()
			require.Equal(t, "home", r.Route())
			require.True(t, r.Tester.Shows("Thử lại"))

			return f, r
		}
		t.Run("the network was only away", func(t *testing.T) {
			f, r := boot(t)
			assert.Equal(t, 1, f.Calls("me.home"))
			f.Fail("me.home", 0, "")
			r.Tester.Elapse(1900 * time.Millisecond)
			r.Settle()
			assert.Nil(t, homeOf(t, r).Content(), "not before 2 s")
			r.Tester.Elapse(200 * time.Millisecond)
			r.Settle()
			assert.NotNil(t, homeOf(t, r).Content())
			assert.True(t, r.Tester.Shows("Xin chào, Thảo Vy"))
			assert.False(t, r.Tester.Shows("Thử lại"))
			assert.Equal(t, 2, f.Calls("me.home"))
		})
		t.Run("still failing", func(t *testing.T) {
			f, r := boot(t)
			r.Tester.Elapse(2100 * time.Millisecond)
			r.Settle()
			assert.Equal(t, 2, f.Calls("me.home"))
			assert.True(t, r.Tester.Shows("Thử lại"))
			r.Tester.Elapse(6100 * time.Millisecond)
			r.Settle()
			assert.Equal(t, 3, f.Calls("me.home"))
			r.Tester.Elapse(30 * time.Second)
			r.Settle()
			assert.Equal(t, 3, f.Calls("me.home"), "no more by itself")
			assert.True(t, r.Tester.Shows("Thử lại"))

			f.Fail("me.home", 0, "")
			require.NoError(t, r.Tester.Click("retry"))
			r.Settle()
			assert.NotNil(t, homeOf(t, r).Content())
		})
		t.Run("the app comes back to the front", func(t *testing.T) {
			f, r := boot(t)
			f.Fail("me.home", 0, "")
			f.SetForeground(false)
			r.Settle()
			assert.Equal(t, 1, f.Calls("me.home"), "nothing while away")
			f.SetForeground(true)
			r.Settle()
			assert.Equal(t, 2, f.Calls("me.home"), "read at once, without the 2 s")
			assert.NotNil(t, homeOf(t, r).Content())
			assert.False(t, r.Tester.Shows("Thử lại"))

			// with the content on show a return reads nothing
			f.SetForeground(false)
			f.SetForeground(true)
			r.Settle()
			assert.Equal(t, 2, f.Calls("me.home"))
		})
	})
}

// TestHomeReadCutByTheBackgroundIsReadAgain: a read still out when the app is sent
// away and brought back is made again, and its late answer (the system's cut one) is
// dropped without the error.
func TestHomeReadCutByTheBackgroundIsReadAgain(t *testing.T) {
	t.Parallel()

	kittest.ForEachPhone(t, func(t *testing.T, ph kittest.Phone) {
		var release func()
		r := newRig(t, ph, false, shellOptions(), signed(), func(t *testing.T, f *shelltest.Fake) {
			release = f.Gate("me.home")
			t.Cleanup(release)
		})
		r.Settle()
		r.Tester.Elapse(600 * time.Millisecond)
		r.WaitFor(func() bool { return r.Route() == "home" }, "Home")
		h := homeOf(t, r)
		require.Nil(t, h.Content())
		r.Fake.SetForeground(false)
		r.Fake.SetForeground(true)
		release()
		r.Settle()
		r.Tester.Elapse(kittest.Frame)
		assert.NotNil(t, h.Content(), "the content comes")
		assert.False(t, r.Tester.Shows("Thử lại"))
		assert.Equal(t, 2, r.Fake.Calls("me.home"), "the read was made again on the return")
	})
}

// TestHomeLeftWhileLoadingShowsNothing checks that an answer arriving after the
// screen was left (a tab tapped while the skeleton was up) is dropped: no error, no
// content, and the screen now showing is not touched.
func TestHomeLeftWhileLoadingShowsNothing(t *testing.T) {
	t.Parallel()

	kittest.ForEachPhone(t, func(t *testing.T, ph kittest.Phone) {
		var release func()
		r := newRig(t, ph, false, shellOptions(), signed(), func(t *testing.T, f *shelltest.Fake) {
			release = f.Gate("me.home")
			t.Cleanup(release)
		})
		r.Settle()
		r.Tester.Elapse(600 * time.Millisecond)
		r.WaitFor(func() bool { return r.Route() == "home" }, "Home")
		h := homeOf(t, r)
		r.Shell.Nav().Go(nav.Route{Screen: "courses"}, true)
		release() // the answer comes after the screen is gone
		r.Settle()
		r.Tester.Elapse(kittest.Frame)
		assert.Equal(t, "courses", r.Route())
		assert.Nil(t, h.Content())
		assert.False(t, r.Tester.Shows("Thử lại"))
	})
}

// TestHomeNewUserHidesTheContinueCardAndTheUpdates checks the `new_user` answer:
// no continue card, no "Mới cập nhật", the numbers 1 / 0 / n, one course in the
// grid and no button under it, and the numbers 18 under the header (the header's margin and the numbers'
// 16 collapse into the larger).
func TestHomeNewUserHidesTheContinueCardAndTheUpdates(t *testing.T) {
	t.Parallel()

	kittest.ForEachPhone(t, func(t *testing.T, ph kittest.Phone) {
		r := atHome(t, ph, true, scenario("me.home", "new_user"))
		assert.False(t, r.Tester.Shows("TIẾP TỤC XEM"))
		assert.False(t, r.Tester.Shows("Mới cập nhật"))
		assert.True(t, r.Tester.Shows("Giới thiệu DanceFIT"))
		assert.True(t, r.Tester.Shows("Khoá học của tôi"))
		lay := r.Dump("M03-new")
		for _, e := range lay.Elements {
			assert.NotContains(t, e.Key, "playVideo:", "no continue card")
			assert.NotContains(t, e.Key, "go.detail:set36", "no update")
		}
		eb, _ := lay.Find("div.eb")
		card, ok := lay.Find("div.card")
		require.True(t, ok)
		assert.InDelta(t, eb.Y+14+8+28+18, card.Y, 0.01, "the numbers follow the header at 18")
		tile, ok := lay.Find("go.detail:intro")
		require.True(t, ok)
		assert.InDelta(t, 20, tile.X, 0.01)
		assert.InDelta(t, (float64(ph.Size.W)-40-12)/2, tile.W, 0.01, "one column wide, the other empty")
		_, more := lay.Find("go.detail:set35")
		assert.False(t, more)
		_, button := lay.Find("go.courses.all")
		assert.False(t, button, "1 course of 1: nothing more to see")
		assert.Nil(t, homeOf(t, r).Content().AllButton())
	})
}

// TestHomeCourseGridGeometry checks K39 on both phones: the courses of me.home, six at
// most, in two columns of equal width inside the 20 DIP margins with 12 between
// columns and rows, each tile's poster as wide as the tile at the 156:100 aspect,
// the tiles of a row as tall as each other, and the reading order across the rows.
func TestHomeCourseGridGeometry(t *testing.T) {
	t.Parallel()

	kittest.ForEachPhone(t, func(t *testing.T, ph kittest.Phone) {
		r := atHome(t, ph, true)
		cards := homeOf(t, r).Content().Cards()
		require.Len(t, cards, 6, "six courses at most")
		colW := (float64(ph.Size.W) - 40 - 12) / 2
		head, ok := r.Dump("M03").Find("div.sec")
		require.True(t, ok)
		wantKeys := []string{"go.detail:set35", "go.detail:set34", "go.detail:hotextra", "go.detail:set33", "go.detail:set32", "go.detail:set31"}
		var rowTop, rowH float64
		for i, cc := range cards {
			f := cc.Node().WindowFrame()
			assert.Equal(t, wantKeys[i], cc.Node().Name, "reading order, row by row")
			assert.InDelta(t, colW, f.W, 0.01, wantKeys[i]+": column width")
			col, row := i%2, i/2
			assert.InDelta(t, 20+float64(col)*(colW+12), f.X, 0.01, wantKeys[i]+": x")
			if col == 0 {
				if row == 0 {
					rowTop = float64(head.Y+head.H) + 12
				} else {
					rowTop += rowH + 12
				}
				rowH = float64(f.H)
			}
			assert.InDelta(t, rowTop, f.Y, 0.01, wantKeys[i]+": y")
			assert.InDelta(t, rowH, f.H, 0.01, wantKeys[i]+": the row is as tall as its first tile")
			po := cc.Poster.Node().WindowFrame()
			assert.InDelta(t, colW, po.W, 0.01, wantKeys[i]+": poster as wide as the tile")
			assert.InDelta(t, colW*100/156, po.H, 0.01, wantKeys[i]+": poster 156:100")
			assert.InDelta(t, f.X, po.X, 0.01)
			assert.InDelta(t, f.Y, po.Y, 0.01)
			assert.LessOrEqual(t, float64(f.X+f.W), float64(ph.Size.W)-20+0.01, "inside the right margin")
		}
		last := cards[5].Node().WindowFrame()
		assert.InDelta(t, float64(ph.Size.W)-20, last.X+last.W, 0.01, "the right column ends at the margin")
	})
}

// TestHomeCourseTilesTextAndColors checks the text of a tile: the name and the
// category line with its number of lessons, in the mock's order; the tagline is
// left out (K39).
func TestHomeCourseTilesTextAndColors(t *testing.T) {
	t.Parallel()

	kittest.ForEachPhone(t, func(t *testing.T, ph kittest.Phone) {
		r := atHome(t, ph, false)
		cards := homeOf(t, r).Content().Cards()
		assert.Equal(t, "DanceFIT Set 35", cards[0].Name)
		assert.Equal(t, "Thường · 10 BÀI", cards[0].Tag)
		assert.Equal(t, "DanceFIT Hot Extra", cards[2].Name)
		assert.False(t, r.Tester.Shows("“Cùng nhảy cùng hát”"), "no tagline in the grid")
	})
}

// TestHomeAllCoursesButton checks K39's button under the grid: it reads "Xem tất cả
// 15 khoá" (the courses_total of the answer), is as wide as the column and 12
// under the last row, takes the update's section 22 under it, and a tap goes to the
// Courses tab as a root.
func TestHomeAllCoursesButton(t *testing.T) {
	t.Parallel()

	kittest.ForEachPhone(t, func(t *testing.T, ph kittest.Phone) {
		h, _, rec, r := atHomeDirect(t, ph)
		btn := h.Content().AllButton()
		require.NotNil(t, btn)
		assert.Equal(t, "Xem tất cả 15 khoá", btn.Text)
		assert.Equal(t, kit.ButtonS, btn.Kind)
		cards := h.Content().Cards()
		lastRow := cards[len(cards)-1].Node().WindowFrame()
		bf := btn.Node().WindowFrame()
		assert.InDelta(t, 20, bf.X, 0.01)
		assert.InDelta(t, float64(ph.Size.W)-40, bf.W, 0.01)
		assert.InDelta(t, 54, bf.H, 0.01)
		assert.InDelta(t, lastRow.Y+lastRow.H+12, bf.Y, 0.01, "12 under the last row")
		upd, ok := r.Dump("M03").Find("div.sec#2")
		require.True(t, ok)
		assert.InDelta(t, bf.Y+bf.H+22, upd.Y, 0.5, "the update's head, 22 under the button")

		reveal(r, h, "go.courses.all")
		require.NoError(t, r.Tester.Click("go.courses.all"))
		assert.Equal(t, []string{"go courses true"}, rec.calls)
	})
}

// TestHomeAllCoursesButtonFollowsTheTotal checks that the button is there when the
// member has more courses than the grid shows and not when the total is what is
// shown, from the answer of the route.
func TestHomeAllCoursesButtonFollowsTheTotal(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		total int
		my    int
		want  string
	}{
		{"more than shown", 15, 6, "Xem tất cả 15 khoá"},
		{"seven of six shown", 7, 6, "Xem tất cả 7 khoá"},
		{"all shown (six)", 6, 6, ""},
		{"fewer than the grid takes", 3, 3, ""},
		{"none", 0, 0, ""},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			kittest.ForEachPhone(t, func(t *testing.T, ph kittest.Phone) {
				f, err := shelltest.New(shelltest.MockFS(t))
				require.NoError(t, err)
				e := env.New(f, locales.MustTexts())
				e.Act = &recorder{}
				h := model.Home{MyCourses: make([]model.Course, tc.my)}
				h.Stats.CoursesTotal = tc.total
				for i := range h.MyCourses {
					h.MyCourses[i] = model.Course{ID: "set-" + strconv.Itoa(30+i), Name: "DanceFIT Set " + strconv.Itoa(30+i), LessonCount: 10}
				}
				k := screens.NewHomeContentForTest(e, h)
				tt := kittest.MountReal(t, ph, k)
				tt.RunUntilIdle()
				assert.Len(t, k.Cards(), min(tc.my, 6))
				if tc.want == "" {
					assert.Nil(t, k.AllButton(), "no button")

					return
				}
				require.NotNil(t, k.AllButton())
				assert.Equal(t, tc.want, k.AllButton().Text)
			})
		})
	}
}

// TestHomeGridTilesAreButtonsNamedByTheCourse checks the accessibility tree: each
// tile is a button named by its course, valued by its category line.
func TestHomeGridTilesAreButtonsNamedByTheCourse(t *testing.T) {
	t.Parallel()

	kittest.ForEachPhone(t, func(t *testing.T, ph kittest.Phone) {
		r := atHome(t, ph, false)
		for _, cc := range homeOf(t, r).Content().Cards() {
			assert.Equal(t, cc.Name, cc.AccessibleLabel())
			assert.NotEmpty(t, cc.Tag)
		}
	})
}

// TestHomeUpdateFollowsTheGrid checks that "Mới cập nhật" stays below the button, as
// wide as the column (K39), and that its head is the 1.24 narrower shaping of
// Core Text, no more.
func TestHomeUpdateFollowsTheGrid(t *testing.T) {
	t.Parallel()

	r := atHome(t, kittest.Phones[0], true)
	lay := r.Dump("M03")
	want := shelltest.ProtoLayout(t, "M03")
	w, _ := want.Find("span.d#2")
	g, ok := lay.Find("span.d#2")
	require.True(t, ok)
	assert.InDelta(t, w.X, g.X, 1)
	assert.InDelta(t, w.H, g.H, 1)
	assert.InDelta(t, w.W, g.W, 1.5)
	btn, ok := lay.Find("go.courses.all")
	require.True(t, ok)
	sec, _ := lay.Find("div.sec#2")
	upd, _ := lay.Find("go.detail:set36")
	assert.InDelta(t, btn.Y+btn.H+22, sec.Y, 0.5)
	assert.InDelta(t, sec.Y+sec.H+12, upd.Y, 0.5)
	assert.InDelta(t, 96, upd.H, 0.5)
}

// TestHomeGridPostersLoadTheirCovers checks A2.11 in the grid: every tile's poster asks
// for the cover of its course, and a loaded cover covers the whole poster, scaled to
// the column (K39).
func TestHomeGridPostersLoadTheirCovers(t *testing.T) {
	t.Parallel()

	kittest.ForEachPhone(t, func(t *testing.T, ph kittest.Phone) {
		img, err := scene.NewImage(8, 8)
		require.NoError(t, err)
		for i := 0; i < len(img.Pix); i += 4 {
			img.Pix[i], img.Pix[i+1], img.Pix[i+2], img.Pix[i+3] = 0, 255, 0, 255
		}
		r := atHome(t, ph, false)
		r.Settle()
		var url string
		for _, id := range []string{"set-35", "set-34", "hot-extra", "set-33", "set-32", "set-31"} {
			found := ""
			for _, c := range r.Fake.CoverCalls() {
				if strings.HasSuffix(c, "/covers/"+id+".jpg") {
					found = c
				}
			}
			assert.NotEmpty(t, found, id+": the tile asked for its cover")
			if id == "set-35" {
				url = found
			}
		}
		r.Fake.SetCover(url, img)
		r.Shell.Go(nav.Route{Screen: env.RouteCourses}, true) // leave, and come back: Home is built again
		r.Settle()
		r.Shell.Go(nav.Route{Screen: env.RouteHome}, true)
		r.WaitFor(func() bool { return homeOf(t, r).Content() != nil }, "Home with its data")
		r.Settle()
		r.Tester.Elapse(kittest.Frame)
		po := homeOf(t, r).Content().Cards()[0].Poster.Node().WindowFrame()
		px := r.Tester.Snapshot().RGBAAt(int(po.X+po.W/2), int(po.Y+po.H/2))
		assert.Equal(t, uint8(255), px.G, "the loaded cover covers the poster")
		assert.Zero(t, px.R)
		px = r.Tester.Snapshot().RGBAAt(int(po.X+po.W/2), int(po.Y+po.H-3))
		assert.Equal(t, uint8(255), px.G, "and reaches its bottom edge")
		assert.Zero(t, px.R)
	})
}

// TestHomeGridLongNameTakesTwoLinesAndRowsStayEven checks a name too long for one
// line: two lines with "…", the tile taller by a line and its neighbour in the row as
// tall, the category line under the name.
func TestHomeGridLongNameTakesTwoLinesAndRowsStayEven(t *testing.T) {
	t.Parallel()

	kittest.ForEachPhone(t, func(t *testing.T, ph kittest.Phone) {
		f, err := shelltest.New(shelltest.MockFS(t))
		require.NoError(t, err)
		e := env.New(f, locales.MustTexts())
		e.Act = &recorder{}
		h := model.Home{MyCourses: []model.Course{
			{ID: "set-35", Name: "DanceFIT Set 35", LessonCount: 10},
			{ID: "set-34", Name: "DanceFIT Set 34 Một Tên Khoá Học Rất Dài Không Thể Vừa Trong Một Dòng Hay Hai Dòng Của Cột", LessonCount: 10},
		}}
		h.Stats.CoursesTotal = 2
		k := screens.NewHomeContentForTest(e, h)
		tt := kittest.MountReal(t, ph, k)
		tt.RunUntilIdle()
		short, long := k.Cards()[0].Node().WindowFrame(), k.Cards()[1].Node().WindowFrame()
		assert.InDelta(t, long.H, short.H, 0.01, "the row is as tall as its taller tile")
		assert.Greater(t, long.H, float32(109.0+10+17+6+20+12-0.5))
		assert.LessOrEqual(t, long.H, float32(109.4+10+34+6+20+12+0.5+6), "two lines of the name at most")
		assert.Nil(t, k.AllButton())
	})
}

// TestHomeHasNoHorizontalScroll checks that K39 took the strip away: Home's page is the
// only scroll, and a swipe across the courses scrolls it, not a strip.
func TestHomeHasNoHorizontalScroll(t *testing.T) {
	t.Parallel()

	kittest.ForEachPhone(t, func(t *testing.T, ph kittest.Phone) {
		r := atHome(t, ph, false)
		h := homeOf(t, r)
		tile := h.Content().Cards()[0].Node().WindowFrame()
		y := tile.Y + 40
		r.Tester.Swipe(geom.Pt(ph.Size.W-40, y), geom.Pt(40, y), 200*time.Millisecond)
		r.Tester.Elapse(2 * time.Second)
		assert.Equal(t, ui.ScrollVertical, h.Scroll().Axes)
		assert.InDelta(t, 0, h.Scroll().Offset().X, 0.01)
		assert.Equal(t, "home", r.Route(), "a sideways swipe over the grid does nothing")
	})
}

// TestHomeCardTapOpensTheCourse runs a tap on a tile of each column through the Shell.
func TestHomeCardTapOpensTheCourse(t *testing.T) {
	t.Parallel()

	kittest.ForEachPhone(t, func(t *testing.T, ph kittest.Phone) {
		for _, tc := range []struct{ key, id string }{{"go.detail:set35", "set-35"}, {"go.detail:set34", "set-34"}} {
			r := atHome(t, ph, false)
			require.NoError(t, r.Tester.Click(tc.key))
			r.Settle()
			assert.Equal(t, "detail", r.Route())
			assert.Equal(t, tc.id, r.Shell.Nav().Current().Params)
		}
	})
}

// TestHomeKeepsTheScrollWhenTheDataIsReadAgain checks that the content replaced by
// the second read leaves the page where the user had scrolled it.
func TestHomeKeepsTheScrollWhenTheDataIsReadAgain(t *testing.T) {
	t.Parallel()

	kittest.ForEachPhone(t, func(t *testing.T, ph kittest.Phone) {
		r := atHome(t, ph, false)
		h := homeOf(t, r)
		h.Scroll().ScrollTo(geom.Pt(0, 50), false)
		r.Tester.Elapse(kittest.Frame)
		before := h.Content()
		r.Fake.SetOnline(false)
		r.Fake.SetOnline(true)
		r.Settle()
		assert.NotSame(t, before, h.Content(), "the content was made again")
		assert.InDelta(t, 50, h.Scroll().Offset().Y, 0.01)
	})
}

// TestHomeIsTheHomeTabAndAScroller checks what the Nav reads of the screen.
func TestHomeIsTheHomeTabAndAScroller(t *testing.T) {
	t.Parallel()

	kittest.ForEachPhone(t, func(t *testing.T, ph kittest.Phone) {
		r := atHome(t, ph, false)
		h := homeOf(t, r)
		assert.Equal(t, 0, h.Tab())
		assert.NotNil(t, h.Scroll())
		assert.Same(t, h.Scroll(), r.Shell.Nav().Screen().(nav.Scroller).Scroll())
	})
}

// TestHomeSteadyFrameDoesNotAllocate checks that a frame of Home standing still
// allocates nothing, on both phones.
func TestHomeSteadyFrameDoesNotAllocate(t *testing.T) { //nolint:paralleltest // AllocsPerRun needs the process to itself
	if kittest.Race {
		t.Skip("allocation counts are not stable under the race detector")
	}
	for _, ph := range kittest.Phones {
		r := atHome(t, ph, false)
		warm(r)
		assert.Zero(t, testing.AllocsPerRun(50, r.Tester.Repaint), ph.Name)
	}
}

// TestHomePageScrollFrameDoesNotAllocate checks that a frame of the page moving,
// with the grid and the button in it, allocates nothing.
func TestHomePageScrollFrameDoesNotAllocate(t *testing.T) { //nolint:paralleltest // AllocsPerRun needs the process to itself
	if kittest.Race {
		t.Skip("allocation counts are not stable under the race detector")
	}
	for _, ph := range kittest.Phones {
		r := atHome(t, ph, false)
		h := homeOf(t, r)
		warm(r)
		sc := h.Scroll()
		y := float32(0)
		for ; y < 200; y += 7 { // the whole page is laid out and shaped once
			sc.ScrollTo(geom.Pt(0, y), false)
			r.Tester.Elapse(kittest.Frame)
		}
		dir := float32(-1)
		page := func() {
			if y > 200 || y < 100 {
				dir = -dir
			}
			y += 3 * dir
			sc.ScrollTo(geom.Pt(0, y), false)
			r.Tester.Frame()
		}
		for range 16 {
			page()
		}
		assert.Zero(t, testing.AllocsPerRun(50, page), ph.Name+": the page (grid and button)")
	}
}

// BenchmarkHomeSteadyFrame is a frame of Home standing still.
func BenchmarkHomeSteadyFrame(b *testing.B) {
	r := atHome(b, kittest.Phones[0], false)
	warm(r)
	b.ReportAllocs()
	for b.Loop() {
		r.Tester.Repaint()
	}
}

// ====================================================================
// ========================= Helper Functions =========================
// ====================================================================

// reveal scrolls Home so that the widget named key shows whole above the tab bar:
// the page to its end for what is under the fold (the last row, the button, the
// update).
func reveal(r *shelltest.Rig, h *screens.Home, key string) {
	switch key {
	case "go.detail:hotextra", "go.detail:set33", "go.detail:set32", "go.detail:set31", "go.courses.all", "go.detail:set36":
		h.Scroll().ScrollTo(h.Scroll().MaxOffset(), false)
	default: // the first row is whole
	}
	r.Tester.Elapse(kittest.Frame)
}

// warm lets the frames run until what Home draws is laid out and shaped once.
func warm(r *shelltest.Rig) {
	for range 8 {
		r.Tester.Elapse(kittest.Frame)
	}
	r.Tester.Repaint()
}
