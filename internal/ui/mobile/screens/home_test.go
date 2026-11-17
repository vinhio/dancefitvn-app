//go:build !tvos

// Pattern from jplayer-mobile/internal/ui/views/alloc_spec_test.go and home_test.go
// at 49de3c2 (steady: warm up, then AllocsPerRun; a scroll step within what is
// laid out; the strip dragged by a finger).

package screens_test

import (
	"context"
	"strconv"
	"testing"
	"time"

	"dancefitvn-app/internal/model"
	"dancefitvn-app/internal/ui/kit/kittest"
	"dancefitvn-app/internal/ui/mobile/env"
	"dancefitvn-app/internal/ui/mobile/nav"
	"dancefitvn-app/internal/ui/mobile/screens"
	"dancefitvn-app/internal/ui/mobile/shell/shelltest"
	"dancefitvn-app/locales"

	"github.com/gdeskdev/desk/foundation/core/geom"
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

// TestHomeBoxesMatchPrototype lays Home out with the iPhone's real text engine
// over the mock's `me.home` and compares every element of M03 with the
// prototype's within 1 DIP: the greeting, the continue card and what is over it,
// the three numbers, the section heads, the six cards of the strip with their
// poster and title, the update, the tab bar. The one exception is a width: the
// text "MỚI CẬP NHẬT" is 1.24 narrower in Core Text than in Chrome (a shaping
// difference of the Vietnamese capitals' marks); its position and height hold.
func TestHomeBoxesMatchPrototype(t *testing.T) {
	t.Parallel()

	r := atHome(t, kittest.Phones[0], true)
	lay := r.Dump("M03")
	checkAgainstProto(t, "M03", lay, 1, map[string]bool{"span.d#2": true})

	want := shelltest.ProtoLayout(t, "M03")
	w, _ := want.Find("span.d#2")
	g, ok := lay.Find("span.d#2")
	require.True(t, ok)
	assert.InDelta(t, w.X, g.X, 1)
	assert.InDelta(t, w.Y, g.Y, 1)
	assert.InDelta(t, w.H, g.H, 1)
	assert.InDelta(t, w.W, g.W, 1.5)
}

// TestHomeOnTheWiderPhone checks the 412 wide phone's Home against the iPhone's:
// the same column from the top (y of every element within 1.5 DIPs), what runs the
// width 19 wider, the numbers sharing the 372 between them, the avatar and the link
// moved by the 19, the strip's cards where they were.
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
		assert.InDelta(t, ea.Y, eb.Y, 1.5, ea.Key+": y")
	}
	full := []string{"playVideo:set35-3", "div.poster", "div.sec", "div.sec#2", "go.detail:set36"}
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
	for i, key := range []string{"go.detail:set35", "go.detail:set34", "go.detail:hotextra", "go.detail:set33", "go.detail:set32", "go.detail:set31"} {
		e, ok := b.Find(key)
		require.True(t, ok, key)
		assert.InDelta(t, 20+float64(i)*168, e.X, 0.01, key)
		assert.InDelta(t, 156, e.W, 0.01, key)
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
			"DanceFIT Hot Extra", "“Hot Extra”", "Đặc biệt · 10 BÀI",
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
// continue card plays the lesson, a card of the strip and the update open their
// course, the link and the avatar go to their tabs as roots.
func TestHomeTapsAskTheActions(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		key  string
		want string
	}{
		{"continue card", "playVideo:set35-3", "playVideo set-35 l_3503 3"},
		{"first card of the strip", "go.detail:set35", "detail set-35"},
		{"special course", "go.detail:hotextra", "detail hot-extra"},
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
// strip, and the numbers 18 under the header (the header's margin and the numbers'
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
		strip, ok := lay.Find("go.detail:intro")
		require.True(t, ok)
		assert.InDelta(t, 20, strip.X, 0.01)
		_, more := lay.Find("go.detail:set35")
		assert.False(t, more)
	})
}

// TestHomeStripScrollsSideways checks the strip: a finger across it moves it
// sideways and not the page, a press in the left edge strip scrolls it too (Home is
// a root: no swipe back can start there, R-P2-9), the last card ends 20 from the
// right edge, and a tap on the part of a card that peeks in from the right (over the
// 20 of the margin) opens that card.
func TestHomeStripScrollsSideways(t *testing.T) {
	t.Parallel()

	kittest.ForEachPhone(t, func(t *testing.T, ph kittest.Phone) {
		r := atHome(t, ph, false)
		h := homeOf(t, r)
		strip := h.Content().Strip()
		lay := r.Dump("M03")
		card, _ := lay.Find("go.detail:set35")
		y := float32(card.Y) + 60
		w := ph.Size.W

		r.Tester.Swipe(geom.Pt(w-40, y), geom.Pt(40, y), 200*time.Millisecond)
		r.Tester.Elapse(2 * time.Second)
		assert.Greater(t, strip.Offset().X, float32(200), "the strip moved")
		assert.InDelta(t, 0, h.Scroll().Offset().Y, 0.01, "the page did not")

		r.Tester.Swipe(geom.Pt(w-40, y), geom.Pt(40, y), 200*time.Millisecond)
		r.Tester.Elapse(2 * time.Second)
		maxX := strip.MaxOffset().X
		assert.InDelta(t, maxX, strip.Offset().X, 0.5, "the end of the strip")
		assert.InDelta(t, 6*156+5*12+40-float64(w), float64(maxX), 0.01, "six cards, five gaps, 20 padding each side")

		back := strip.Offset().X
		r.Tester.Swipe(geom.Pt(8, y), geom.Pt(w-60, y), 200*time.Millisecond)
		r.Tester.Elapse(2 * time.Second)
		assert.Less(t, strip.Offset().X, back, "a press at the left edge scrolls the strip")
		assert.Equal(t, "home", r.Route(), "and goes nowhere")
		assert.Equal(t, 0, r.Shell.Nav().Depth())
		assert.False(t, r.Shell.Nav().EdgeArmed(r.Tester.View().Context()), "Home has nothing to go back to")

		strip.ScrollTo(geom.Pt(0, 0), false)
		r.Tester.Elapse(kittest.Frame)
		r.Tester.ClickAt(geom.Pt(w-8, y)) // over the third card's visible part, in the right margin
		r.Settle()
		assert.Equal(t, "detail", r.Route())
		assert.Equal(t, "hot-extra", r.Shell.Nav().Current().Params)
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

// TestHomeStripScrollFrameDoesNotAllocate checks that a frame of the strip moving a
// DIP, and one of the page moving, allocate nothing.
func TestHomeStripScrollFrameDoesNotAllocate(t *testing.T) { //nolint:paralleltest // AllocsPerRun needs the process to itself
	if kittest.Race {
		t.Skip("allocation counts are not stable under the race detector")
	}
	for _, ph := range kittest.Phones {
		r := atHome(t, ph, false)
		h := homeOf(t, r)
		strip := h.Content().Strip()
		warm(r)
		d := float32(1)
		step := func() {
			d = -d
			strip.ScrollBy(geom.Pt(d, 0))
			r.Tester.Frame()
		}
		for range 16 {
			step()
		}
		assert.Zero(t, testing.AllocsPerRun(50, step), ph.Name+": the strip")

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
		assert.Zero(t, testing.AllocsPerRun(50, page), ph.Name+": the page")
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

// BenchmarkHomeStripScrollFrame is a frame of the strip moving a DIP.
func BenchmarkHomeStripScrollFrame(b *testing.B) {
	r := atHome(b, kittest.Phones[0], false)
	strip := homeOf(b, r).Content().Strip()
	warm(r)
	d := float32(1)
	b.ReportAllocs()
	for b.Loop() {
		d = -d
		strip.ScrollBy(geom.Pt(d, 0))
		r.Tester.Frame()
	}
}

// ====================================================================
// ========================= Helper Functions =========================
// ====================================================================

// reveal scrolls Home so that the widget named key shows whole above the tab bar:
// the strip to the left for a card of it, the page to its end for the update.
func reveal(r *shelltest.Rig, h *screens.Home, key string) {
	switch key {
	case "go.detail:hotextra":
		h.Content().Strip().ScrollTo(geom.Pt(200, 0), false)
	case "go.detail:set36":
		h.Scroll().ScrollTo(h.Scroll().MaxOffset(), false)
	default: // other cards are already whole
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
