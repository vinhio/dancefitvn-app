//go:build !tvos

package screens_test

import (
	"testing"
	"time"

	"dancefitvn-app/internal/model"
	"dancefitvn-app/internal/ui/kit"
	"dancefitvn-app/internal/ui/kit/kittest"
	"dancefitvn-app/internal/ui/mobile/env"
	"dancefitvn-app/internal/ui/mobile/screens"
	"dancefitvn-app/internal/ui/mobile/shell/shelltest"

	"github.com/gdeskdev/desk/foundation/core/geom"
	"github.com/gdeskdev/desk/foundation/core/keys"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// The texts of the toasts of 15C (locales/vi.json).
const (
	toastCoachSaved = "Đã cập nhật hồ sơ HLV"
	toastBadSocial  = "Chỉ hỗ trợ Facebook, Instagram, YouTube, TikTok, Zalo, Threads"
	toastMaxSocials = "Tối đa 6 liên kết"
	toastMaxPlaces  = "Tối đa 5 địa điểm"
	coachBioMock    = "Huấn luyện viên DanceFIT & StepX tại Đà Nẵng."
	coachPublicMock = "https://dancefitvn.com/huan-luyen-vien/u_412"
	coachSocialFB   = "https://www.facebook.com/thaovy.dancefit"
	coachSocialTikT = "https://www.tiktok.com/@thaovy.hlv"
	coachSocialYT   = "https://www.youtube.com/@ThaoVyDance"
	coachPlaceMock  = "Nhà văn hoá P. Hoà Thuận, Đà Nẵng"
)

// atCoach opens screen 15C from screen 15, as a tap on its row does.
func atCoach(tb testing.TB, ph kittest.Phone, real bool, setups ...func(*shelltest.Fake)) (*shelltest.Rig, *screens.Coach) {
	tb.Helper()
	r := atAccount(tb, ph, real, setups...)

	return toCoach(tb, r)
}

// toCoach taps the row "Hồ sơ huấn luyện viên" of screen 15.
func toCoach(tb testing.TB, r *shelltest.Rig) (*shelltest.Rig, *screens.Coach) {
	tb.Helper()
	tapKey(tb, r, "go.coach")
	r.Tester.Elapse(kittest.Frame)
	r.Settle()
	require.Equal(tb, env.RouteCoach, r.Route())
	co, ok := r.Shell.Nav().Screen().(*screens.Coach)
	require.True(tb, ok, "the screen showing is Coach")

	return r, co
}

// inputTexts are the texts of the inputs.
func inputTexts(in []*kit.Input) []string {
	out := make([]string, len(in))
	for i, x := range in {
		out[i] = x.Text()
	}

	return out
}

func TestCoachShowsTheProfile(t *testing.T) {
	t.Parallel()

	kittest.ForEachPhone(t, func(t *testing.T, ph kittest.Phone) {
		r, co := atCoach(t, ph, false)
		assert.Equal(t, -1, co.Tab())
		assert.Equal(t, coachBioMock, co.Bio().Text())
		assert.Equal(t, []string{"DANCEFIT", "STEPX", "BODYX"}, co.Subjects())
		assert.Equal(t, []string{coachSocialFB, coachSocialTikT, coachSocialYT}, inputTexts(co.Socials()))
		assert.Equal(t, []string{coachPlaceMock}, inputTexts(co.Places()))
		for i, p := range []string{"facebook", "tiktok", "youtube"} {
			assert.Equal(t, p, co.Platform(i), "the platform in front of the link")
		}
		for _, text := range []string{"Hồ sơ huấn luyện viên", "Thông tin bên dưới sẽ hiển thị công khai trên trang Huấn luyện viên.",
			"Xem trang Huấn luyện viên ↗", "Giới thiệu bản thân", "Bộ môn giảng dạy", "Bộ môn giảng dạy do quản trị viên cập nhật.",
			"Trang mạng xã hội", "+ Thêm liên kết", "Chỉ hỗ trợ Facebook, Instagram, YouTube, TikTok, Zalo và Threads.",
			"Địa điểm dạy", "+ Thêm địa điểm", "Cập nhật hồ sơ HLV"} {
			assert.Truef(t, r.Tester.Shows(text), "%q is on the screen", text)
		}
		assert.Equal(t, 1, r.Fake.Account().Count("Coach"))
		assert.Zero(t, r.Fake.Account().Count("UpdateCoach"))
		assert.False(t, co.Busy())
	})
}

// TestCoachDrawsTheKeptViewFirst checks that a screen built after a read draws at
// once, before its own read answers.
func TestCoachDrawsTheKeptViewFirst(t *testing.T) {
	t.Parallel()

	kittest.ForEachPhone(t, func(t *testing.T, ph kittest.Phone) {
		r, _ := atCoach(t, ph, false)
		tapKey(t, r, "back")
		require.Equal(t, env.RouteAccount, r.Route())
		at := keyPoint(t, r, "go.coach")
		release := r.Fake.Gate("me.coach.get")
		t.Cleanup(release)
		r.Tester.ClickAt(at)
		r.Tester.Elapse(kittest.Frame)
		co, ok := r.Shell.Nav().Screen().(*screens.Coach)
		require.True(t, ok)
		assert.Equal(t, coachBioMock, co.Bio().Text(), "the kept view, the read is held")
		assert.Len(t, co.Socials(), 3)
		release()
		r.Settle()
	})
}

// tapIn scrolls the page of the screen until the element key is clear of the bar of
// the button and of the top, then taps it, as a finger does.
func tapIn(tb testing.TB, r *shelltest.Rig, co *screens.Coach, key string) {
	tb.Helper()
	e := element(tb, r.Dump("reveal"), key)
	room := float64(r.Phone.Size.H) - 120
	switch {
	case e.Y+e.H > room:
		co.Scroll().ScrollBy(geom.Pt(0, float32(e.Y+e.H-room)))
		r.Tester.Elapse(kittest.Frame)
	case e.Y < 60:
		co.Scroll().ScrollBy(geom.Pt(0, float32(e.Y-60)))
		r.Tester.Elapse(kittest.Frame)
	}
	tapKey(tb, r, key)
}

// keyPoint returns the middle of the element key of the dump, to tap while a request
// is held (a Dump would wait for it).
func keyPoint(tb testing.TB, r *shelltest.Rig, key string) geom.Point {
	tb.Helper()
	e, ok := r.Dump("point").Find(key)
	require.Truef(tb, ok, "no element %q", key)

	return geom.Pt(float32(e.X+e.W/2), float32(e.Y+e.H/2))
}

func TestCoachSubjectsEditableHidesTheNote(t *testing.T) {
	t.Parallel()

	files := map[string]string{"me/coach-profile.json": `{"bio":"","subjects":["DanceFIT"],"subjects_editable":true,"socials":[],
		"allowed_platforms":["facebook"],"max_socials":2,"places":[],"max_places":1,"public_url":""}`}
	kittest.ForEachPhone(t, func(t *testing.T, ph kittest.Phone) {
		r := atAccountOver(t, ph, files)
		r, co := toCoach(t, r)
		assert.Equal(t, []string{"DANCEFIT"}, co.Subjects())
		assert.False(t, r.Tester.Shows("Bộ môn giảng dạy do quản trị viên cập nhật."), "the administrator's note is for a locked list")
		assert.Empty(t, co.Socials())
		// the limits are the API's
		tapIn(t, r, co, "addSoc")
		tapIn(t, r, co, "addSoc")
		tapIn(t, r, co, "addSoc")
		assert.Len(t, co.Socials(), 2)
		assert.True(t, shows(r, "Tối đa 2 liên kết"))
		tapIn(t, r, co, "addPlace")
		tapIn(t, r, co, "addPlace")
		assert.Len(t, co.Places(), 1)
		assert.True(t, shows(r, "Tối đa 1 địa điểm"))
	})
}

func TestCoachAddAndRemoveLinks(t *testing.T) {
	t.Parallel()

	kittest.ForEachPhone(t, func(t *testing.T, ph kittest.Phone) {
		r, co := atCoach(t, ph, false)
		tapIn(t, r, co, "addSoc")
		require.Len(t, co.Socials(), 4)
		assert.Empty(t, co.Socials()[3].Text(), "a new link is empty")
		assert.Equal(t, "link", co.Platform(3), "and names no platform")
		// typing changes the platform when the screen is drawn again (Q-P6-18): an add does
		co.Socials()[3].SetText("https://zalo.me/thaovy")
		assert.Equal(t, "link", co.Platform(3))
		tapIn(t, r, co, "addSoc")
		assert.Equal(t, "zalo", co.Platform(3))
		// the first link goes: the others move up, each with its own platform
		tapIn(t, r, co, "rmSoc")
		assert.Equal(t, []string{coachSocialTikT, coachSocialYT, "https://zalo.me/thaovy", ""}, inputTexts(co.Socials()))
		assert.Equal(t, []string{"tiktok", "youtube", "zalo", "link"}, []string{co.Platform(0), co.Platform(1), co.Platform(2), co.Platform(3)})
		assert.Zero(t, r.Fake.Account().Count("UpdateCoach"), "nothing is sent before the button")
	})
}

func TestCoachAddAndRemovePlaces(t *testing.T) {
	t.Parallel()

	kittest.ForEachPhone(t, func(t *testing.T, ph kittest.Phone) {
		r, co := atCoach(t, ph, false)
		tapIn(t, r, co, "addPlace")
		require.Len(t, co.Places(), 2)
		co.Places()[1].SetText("CLB Thể thao – Quận 1")
		tapIn(t, r, co, "rmPlace")
		assert.Equal(t, []string{"CLB Thể thao – Quận 1"}, inputTexts(co.Places()))
		tapIn(t, r, co, "rmPlace")
		assert.Empty(t, co.Places())
		tapIn(t, r, co, "addPlace")
		assert.Len(t, co.Places(), 1)
	})
}

// TestCoachLimits checks both limits: the sixth link and the fifth place go in, the
// next ones are the toasts "Tối đa 6 liên kết" and "Tối đa 5 địa điểm" and add nothing.
func TestCoachLimits(t *testing.T) {
	t.Parallel()

	kittest.ForEachPhone(t, func(t *testing.T, ph kittest.Phone) {
		r, co := atCoach(t, ph, false)
		for range 3 {
			tapIn(t, r, co, "addSoc")
		}
		require.Len(t, co.Socials(), 6)
		assert.False(t, r.Tester.Shows(toastMaxSocials))
		tapIn(t, r, co, "addSoc")
		assert.Len(t, co.Socials(), 6, "the limit of the API (max_socials)")
		assert.True(t, shows(r, toastMaxSocials))
		r.Tester.Elapse(2300 * time.Millisecond) // the toast covers the link under it

		for range 4 {
			tapIn(t, r, co, "addPlace")
		}
		require.Len(t, co.Places(), 5)
		tapIn(t, r, co, "addPlace")
		assert.Len(t, co.Places(), 5, "the limit of the API (max_places)")
		assert.True(t, shows(r, toastMaxPlaces))
	})
}

// TestCoachBadLink checks the prototype's local rule: a link that names none of the
// platforms is the toast only (no red border, as `saveCoach` `$P:617`) and nothing
// is sent.
func TestCoachBadLink(t *testing.T) {
	t.Parallel()

	kittest.ForEachPhone(t, func(t *testing.T, ph kittest.Phone) {
		r, co := atCoach(t, ph, false)
		tapIn(t, r, co, "addSoc")
		co.Socials()[3].SetText("https://example.com/thaovy")
		press(r, co.Save())
		assert.True(t, shows(r, toastBadSocial))
		assert.Zero(t, r.Fake.Account().Count("UpdateCoach"), "nothing is sent")
		assert.False(t, co.Socials()[3].Error, "the local check shows the toast only, no red border")
		assert.False(t, co.Socials()[0].Error)
		assert.Equal(t, env.RouteCoach, r.Route())
	})
}

// TestCoachLinkPatternIsCaseInsensitive checks `/(facebook|…)/i`.
func TestCoachLinkPatternIsCaseInsensitive(t *testing.T) {
	t.Parallel()

	kittest.ForEachPhone(t, func(t *testing.T, ph kittest.Phone) {
		r, co := atCoach(t, ph, false)
		tapIn(t, r, co, "addSoc")
		co.Socials()[3].SetText("HTTPS://WWW.INSTAGRAM.COM/thaovy")
		press(r, co.Save())
		assert.Equal(t, 1, r.Fake.Account().Count("UpdateCoach"))
	})
}

// TestCoachSaves checks success: the empty links and places are dropped, the rest goes
// as typed, the screen goes back with the toast, and the profile built again shows the
// edits (the overlay keeps them).
func TestCoachSaves(t *testing.T) {
	t.Parallel()

	kittest.ForEachPhone(t, func(t *testing.T, ph kittest.Phone) {
		r, co := atCoach(t, ph, false)
		co.Bio().SetText(coachBioMock + " Mới.")
		tapIn(t, r, co, "addSoc")
		tapIn(t, r, co, "addSoc")
		co.Socials()[4].SetText("  https://www.instagram.com/thaovy ")
		tapIn(t, r, co, "addPlace")
		co.Places()[1].SetText("   ")
		tapIn(t, r, co, "addPlace")
		co.Places()[2].SetText("CLB Thể thao – Quận 1")
		press(r, co.Save())
		ups := r.Fake.Account().CoachUpdates()
		require.Len(t, ups, 1)
		assert.Equal(t, coachBioMock+" Mới.", ups[0].Bio)
		assert.Equal(t, []model.SocialURL{{URL: coachSocialFB}, {URL: coachSocialTikT}, {URL: coachSocialYT}, {URL: "https://www.instagram.com/thaovy"}}, ups[0].Socials,
			"the empty link is dropped, the typed one trimmed")
		assert.Equal(t, []string{coachPlaceMock, "CLB Thể thao – Quận 1"}, ups[0].Places)
		assert.Equal(t, env.RouteAccount, r.Route(), "back()")
		assert.True(t, shows(r, toastCoachSaved))
		r.Tester.Elapse(2200 * time.Millisecond)
		assert.False(t, r.Tester.Shows(toastCoachSaved), "the toast goes after 2200 ms")

		_, again := toCoach(t, r)
		assert.Equal(t, coachBioMock+" Mới.", again.Bio().Text())
		assert.Equal(t, 4, len(again.Socials()))
		assert.Equal(t, []string{coachPlaceMock, "CLB Thể thao – Quận 1"}, inputTexts(again.Places()))
	})
}

// TestCoachInvalidSocial runs the scenario `invalid_social`: the screen stays, the
// toast is the prototype's, and the input of the link the server names goes red: it
// names it by its place in the request, with the empty links dropped.
func TestCoachInvalidSocial(t *testing.T) {
	t.Parallel()

	kittest.ForEachPhone(t, func(t *testing.T, ph kittest.Phone) {
		r, co := atCoach(t, ph, false)
		require.NoError(t, r.Fake.SetScenario("me.coach.update", "invalid_social"))
		co.Socials()[1].SetText("") // the request is [facebook, youtube]: `socials[1]` is youtube
		press(r, co.Save())
		assert.True(t, shows(r, toastBadSocial))
		assert.Equal(t, env.RouteCoach, r.Route(), "the screen stays")
		assert.Equal(t, []bool{false, false, true}, []bool{co.Socials()[0].Error, co.Socials()[1].Error, co.Socials()[2].Error})
		assert.False(t, co.Busy())
		assert.Equal(t, coachSocialYT, co.Socials()[2].Text(), "what was typed stays")

		require.NoError(t, r.Fake.SetScenario("me.coach.update", "default"))
		typeInto(r, co.Socials()[2], "x")
		assert.False(t, co.Socials()[2].Error)
		press(r, co.Save())
		assert.Equal(t, env.RouteAccount, r.Route(), "a corrected profile goes through")
	})
}

// TestCoachServerErrors runs the failures of the network and the server.
func TestCoachServerErrors(t *testing.T) {
	t.Parallel()

	kittest.ForEachPhone(t, func(t *testing.T, ph kittest.Phone) {
		t.Run("offline", func(t *testing.T) {
			r, co := atCoach(t, ph, false)
			r.Fake.SetOnline(false)
			press(r, co.Save())
			assert.True(t, shows(r, toastOffline))
			assert.Equal(t, env.RouteCoach, r.Route())
			assert.False(t, co.Busy())
		})
		t.Run("server error", func(t *testing.T) {
			r, co := atCoach(t, ph, false)
			r.Fake.Fail("me.coach.update", 500, "errors/server_error.json")
			press(r, co.Save())
			assert.True(t, shows(r, toastBusy))
			assert.Equal(t, env.RouteCoach, r.Route())
		})
		t.Run("upgrade required", func(t *testing.T) {
			r, co := atCoach(t, ph, false)
			r.Fake.Fail("me.coach.update", 426, "errors/upgrade_required.json")
			press(r, co.Save())
			assert.Equal(t, env.RouteUpgrade, r.Route())
		})
	})
}

func TestCoachSavesOnce(t *testing.T) {
	t.Parallel()

	kittest.ForEachPhone(t, func(t *testing.T, ph kittest.Phone) {
		r, co := atCoach(t, ph, false)
		release := r.Fake.Gate("me.coach.update")
		t.Cleanup(release)
		tap(r, co.Save())
		r.WaitFor(co.Busy, "the request under way")
		tap(r, co.Save())
		release()
		r.Settle()
		assert.Equal(t, 1, r.Fake.Calls("me.coach.update"))
	})
}

// TestCoachBackDiscardsAndDropsTheSave checks that Back leaves the edits behind, and
// that leaving while the request is out shows nothing of its answer.
func TestCoachBackDiscards(t *testing.T) {
	t.Parallel()

	kittest.ForEachPhone(t, func(t *testing.T, ph kittest.Phone) {
		r, co := atCoach(t, ph, false)
		r.Tester.View().Focus(co.Bio())
		r.Tester.Type(" Mới.")
		tapIn(t, r, co, "rmSoc")
		tapKey(t, r, "back")
		require.Equal(t, env.RouteAccount, r.Route())
		assert.Zero(t, r.Fake.Account().Count("UpdateCoach"))
		_, again := toCoach(t, r)
		assert.Equal(t, coachBioMock, again.Bio().Text())
		assert.Len(t, again.Socials(), 3)
	})
	kittest.ForEachPhone(t, func(t *testing.T, ph kittest.Phone) {
		r, co := atCoach(t, ph, false)
		release := r.Fake.Gate("me.coach.update")
		t.Cleanup(release)
		tap(r, co.Save())
		r.WaitFor(co.Busy, "the request under way")
		require.NoError(t, r.Tester.Click("Quay lại")) // a Dump would wait for the held request
		release()
		r.Settle()
		assert.False(t, shows(r, toastCoachSaved), "the answer of a screen left shows nothing")
	})
}

// TestCoachReadDoesNotOverwriteEdits checks that what GET answers late does not wipe
// what the user typed meanwhile.
func TestCoachReadDoesNotOverwriteEdits(t *testing.T) {
	t.Parallel()

	kittest.ForEachPhone(t, func(t *testing.T, ph kittest.Phone) {
		r := atAccount(t, ph, false)
		at := keyPoint(t, r, "go.coach")
		release := r.Fake.Gate("me.coach.get")
		t.Cleanup(release)
		r.Tester.ClickAt(at)
		r.Tester.Elapse(kittest.Frame)
		co, ok := r.Shell.Nav().Screen().(*screens.Coach)
		require.True(t, ok)
		require.Empty(t, co.Bio().Text(), "nothing is kept: the form is empty while the read is out")
		require.Empty(t, co.Socials())
		r.Tester.View().Focus(co.Bio())
		r.Tester.Type("Mới")
		release()
		r.Settle()
		assert.Equal(t, "Mới", co.Bio().Text(), "the read did not write over the form")
		assert.Empty(t, co.Socials())
	})
}

func TestCoachOpensThePublicPage(t *testing.T) {
	t.Parallel()

	kittest.ForEachPhone(t, func(t *testing.T, ph kittest.Phone) {
		r, _ := atCoach(t, ph, false)
		tapKey(t, r, "a.acc")
		assert.Equal(t, []string{coachPublicMock}, r.Fake.OpenedURLs())
	})
}

func TestCoachEnterEndsTheEdit(t *testing.T) {
	t.Parallel()

	kittest.ForEachPhone(t, func(t *testing.T, ph kittest.Phone) {
		r, co := atCoach(t, ph, false)
		r.Tester.View().Focus(co.Places()[0].Field())
		require.True(t, r.Tester.Key(keys.KeyEnter, 0))
		assert.Nil(t, focused(r), "Done hides the keyboard")
	})
}

// TestCoachKeyboardRevealsTheField checks that the place field, at the foot of the page,
// comes above the keyboard when it is raised.
func TestCoachKeyboardRevealsTheField(t *testing.T) {
	t.Parallel()

	kittest.ForEachPhone(t, func(t *testing.T, ph kittest.Phone) {
		r, co := atCoach(t, ph, false)
		in := co.Places()[0]
		r.Tester.View().Focus(in.Field())
		r.Tester.SetKeyboard(geom.Insets{Bottom: 336})
		r.Tester.Elapse(400 * time.Millisecond)
		assert.LessOrEqual(t, in.WindowFrame().Max().Y, ph.Size.H-336, "the field is above the keyboard")
		assert.GreaterOrEqual(t, in.WindowFrame().Y, float32(0))
	})
}

// TestCoachBoxesMatchPrototype lays screen 15C out as M15C (393 wide) within 1 DIP.
func TestCoachBoxesMatchPrototype(t *testing.T) {
	t.Parallel()

	r, _ := atCoach(t, kittest.Phones[0], true)
	checkAgainstProto(t, "M15C", r.Dump("M15C"), 1, nil)
}

// TestCoachOnTheWiderPhone checks screen 15C on the 412 wide phone: the same column
// from the top, 372 wide, the rows and the button at the gutters.
func TestCoachOnTheWiderPhone(t *testing.T) {
	t.Parallel()

	ra, _ := atCoach(t, kittest.Phones[0], true)
	rb, _ := atCoach(t, kittest.Phones[1], true)
	a, b := ra.Dump("M15C"), rb.Dump("M15C")
	require.Len(t, b.Elements, len(a.Elements))
	for i, ea := range a.Elements {
		eb := b.Elements[i]
		require.Equal(t, ea.Key, eb.Key)
		switch ea.Key {
		case "div#m", "div.scr", "div.scroll":
			assert.InDelta(t, ea.H+40, eb.H, 1.5, ea.Key+": h")
		case "saveCoach", "svg.i#8": // `bottom` of the window
			assert.InDelta(t, ea.Y+40, eb.Y, 1.5, ea.Key+": y from the bottom")
		default:
			assert.InDelta(t, ea.Y, eb.Y, 1.5, ea.Key+": y")
			assert.InDelta(t, ea.H, eb.H, 1.5, ea.Key+": h")
		}
	}
	for _, key := range []string{"p.muted", "label", "label#2", "div.muted", "label#3", "div.muted#2", "label#4", "saveCoach"} {
		e := element(t, b, key)
		assert.InDelta(t, 20, e.X, 0.01, key+": left")
		assert.InDelta(t, 372, e.W, 0.01, key+": width")
	}
	for _, key := range []string{"div.inp", "div.inp#2", "div.inp#3", "div.inp#4"} {
		assert.InDelta(t, 320, element(t, b, key).W, 0.01, key+": the room less the button and its gap")
	}
	for _, key := range []string{"rmSoc", "rmSoc#2", "rmSoc#3", "rmPlace"} {
		assert.InDelta(t, 412-20-44, element(t, b, key).X, 0.01, key+": at the right edge")
	}
	assert.InDelta(t, 892-38-54, element(t, b, "saveCoach").Y, 0.01, "bottom:38px")
}

// TestCoachSteadyFrameDoesNotAllocate checks that a frame of the screen standing still
// allocates nothing, on both phones.
func TestCoachSteadyFrameDoesNotAllocate(t *testing.T) { //nolint:paralleltest // AllocsPerRun needs the process to itself
	if kittest.Race {
		t.Skip("allocation counts are not stable under the race detector")
	}
	for _, ph := range kittest.Phones {
		r, co := atCoach(t, ph, false)
		r.Tester.View().Focus(co.Bio())
		for range 40 {
			r.Tester.Elapse(kittest.Frame)
		}
		assert.Zero(t, testing.AllocsPerRun(50, r.Tester.Repaint), ph.Name)
	}
}
