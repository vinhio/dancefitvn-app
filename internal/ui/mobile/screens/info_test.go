//go:build !tvos

package screens_test

import (
	"testing"
	"time"

	"dancefitvn-app/internal/model"
	"dancefitvn-app/internal/ui/kit"
	"dancefitvn-app/internal/ui/kit/kittest"
	"dancefitvn-app/internal/ui/mobile/env"
	"dancefitvn-app/internal/ui/mobile/nav"
	"dancefitvn-app/internal/ui/mobile/screens"
	"dancefitvn-app/internal/ui/mobile/shell/shelltest"

	"github.com/gdeskdev/desk/foundation/core/geom"
	"github.com/gdeskdev/desk/foundation/core/keys"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// The texts of the toasts of 15B (locales/vi.json).
const (
	toastInfoRequired = "Tên và email là bắt buộc"
	toastInfoSaved    = "Đã cập nhật thông tin tài khoản"
	toastInfoInvalid  = "Thông tin chưa hợp lệ"
	toastOffline      = "Không kết nối được, thử lại sau"
	toastBusy         = "Hệ thống đang bận"
)

// typeInto focuses the field and types s after what it holds.
func typeInto(r *shelltest.Rig, in *kit.Input, s string) {
	r.Tester.View().Focus(in.Field())
	r.Tester.Type(s)
}

// press taps the button, as a finger does.
func press(r *shelltest.Rig, b *kit.Button) {
	tap(r, b)
	r.Settle()
}

// TestInfoShowsTheUser checks screen 15B over the user of GET /me, on both phones:
// the four fields, the phone grouped as the prototype shows it, the texts of the
// form, the button.
func TestInfoShowsTheUser(t *testing.T) {
	t.Parallel()

	kittest.ForEachPhone(t, func(t *testing.T, ph kittest.Phone) {
		r, in := atInfo(t, ph, false)
		assert.Equal(t, "Thảo Vy", in.First().Text())
		assert.Equal(t, "Nguyễn", in.Last().Text())
		assert.Equal(t, meEmail, in.Email().Text())
		assert.Equal(t, "0905 123 456", in.Phone().Text(), "grouped 4-3-3 (Q-P6-7)")
		assert.Equal(t, -1, in.Tab())
		for _, text := range []string{"Thông tin tài khoản", "Thông tin", "tài khoản", "Thông tin tài khoản và khoá học của bạn.",
			"Tên", "Họ", "Email", "Số điện thoại", "Đổi mật khẩu",
			"Thay đổi được đồng bộ với tài khoản trên website dancefitvn.com.", "Cập nhật"} {
			assert.Truef(t, r.Tester.Shows(text), "%q is on the screen", text)
		}
		assert.False(t, in.Busy())
		assert.Zero(t, r.Fake.Account().Count("UpdateMe"))
	})
}

// TestInfoBackDiscards checks that Back leaves what was typed behind: nothing is
// sent and screen 15 shows the user as it was.
func TestInfoBackDiscards(t *testing.T) {
	t.Parallel()

	kittest.ForEachPhone(t, func(t *testing.T, ph kittest.Phone) {
		r, in := atInfo(t, ph, false)
		typeInto(r, in.First(), " Mới")
		assert.Equal(t, "Thảo Vy Mới", in.First().Text())
		tapKey(t, r, "back")
		assert.Equal(t, env.RouteAccount, r.Route())
		assert.Equal(t, "Nguyễn Thảo Vy", accountOf(t, r).UserName())
		assert.Zero(t, r.Fake.Account().Count("UpdateMe"))

		_, in = toInfo(t, r)
		assert.Equal(t, "Thảo Vy", in.First().Text(), "a screen built again starts from the user")
	})
}

// TestInfoLocalChecks checks `$P:564`: no name or an email with no "@" is the toast
// "Tên và email là bắt buộc" and nothing is sent; a name that is only spaces counts
// as none.
func TestInfoLocalChecks(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name         string
		first, email string
	}{
		{"no name", "", meEmail},
		{"a name of spaces", "   ", meEmail},
		{"no email", "Vy", ""},
		{"an email with no at", "Vy", "thaovy.hlv.gmail.com"},
	}
	kittest.ForEachPhone(t, func(t *testing.T, ph kittest.Phone) {
		for _, tc := range tests {
			t.Run(tc.name, func(t *testing.T) {
				r, in := atInfo(t, ph, false)
				in.First().SetText(tc.first)
				in.Email().SetText(tc.email)
				press(r, in.Submit())
				assert.True(t, shows(r, toastInfoRequired))
				assert.Zero(t, r.Fake.Account().Count("UpdateMe"), "nothing is sent")
				assert.Equal(t, env.RouteInfo, r.Route(), "the screen stays")
				assert.False(t, in.Busy())
			})
		}
	})
}

// TestInfoSaves checks the save: what is typed is sent trimmed, the phone as digits,
// then the screen goes back to 15, which shows the new name at once, with the toast
// "Đã cập nhật thông tin tài khoản" (`$P:564`).
func TestInfoSaves(t *testing.T) {
	t.Parallel()

	kittest.ForEachPhone(t, func(t *testing.T, ph kittest.Phone) {
		r, in := atInfo(t, ph, false)
		in.First().SetText("  Lan Anh ")
		in.Last().SetText("Trần")
		in.Email().SetText(" lan@example.com ")
		in.Phone().SetText("0912 345 678")
		press(r, in.Submit())

		assert.Equal(t, []model.MeUpdateRequest{
			{FirstName: "Lan Anh", LastName: "Trần", Email: "lan@example.com", Phone: "0912345678"},
		}, r.Fake.Account().Updates())
		require.Equal(t, env.RouteAccount, r.Route(), "back() to screen 15")
		assert.Equal(t, "Trần Lan Anh", accountOf(t, r).UserName())
		assert.Equal(t, "lan@example.com", accountOf(t, r).Email())
		assert.True(t, shows(r, toastInfoSaved))
		r.Tester.Elapse(2200 * time.Millisecond)
		assert.False(t, r.Tester.Shows(toastInfoSaved), "the toast goes after 2200 ms")

		_, in = toInfo(t, r)
		assert.Equal(t, "Lan Anh", in.First().Text(), "the form starts from the saved user")
	})
}

// TestInfoSavesOnce checks that a second tap while the save is out is ignored.
func TestInfoSavesOnce(t *testing.T) {
	t.Parallel()

	kittest.ForEachPhone(t, func(t *testing.T, ph kittest.Phone) {
		r, in := atInfo(t, ph, false)
		release := r.Fake.Gate("me.update")
		t.Cleanup(release)
		tap(r, in.Submit())
		r.WaitFor(in.Busy, "the save under way")
		tap(r, in.Submit())
		tap(r, in.Submit())
		release()
		r.Settle()
		assert.Equal(t, 1, r.Fake.Calls("me.update"))
		assert.Equal(t, env.RouteAccount, r.Route())
	})
}

// TestInfoEmailPending checks the answer with `email_change_pending`: the screen goes
// back, and the toast tells to look in the mailbox of the new address (Q-P6-6) instead
// of "Đã cập nhật thông tin tài khoản".
func TestInfoEmailPending(t *testing.T) {
	t.Parallel()

	kittest.ForEachPhone(t, func(t *testing.T, ph kittest.Phone) {
		files := map[string]string{"me/me.patch.json": `{"user":{"id":"u_412","first_name":"Thảo Vy","last_name":"Nguyễn",
			"full_name":"Nguyễn Thảo Vy","email":"thaovy.hlv@gmail.com","phone":"0905123456","avatar_url":"","member_since":"2024-03-12","role":"coach"},
			"email_change_pending":"moi@example.com"}`}
		r := atAccountOver(t, ph, files)
		_, in := toInfo(t, r)
		in.Email().SetText("moi@example.com")
		press(r, in.Submit())
		require.Equal(t, env.RouteAccount, r.Route())
		assert.True(t, shows(r, "Kiểm tra hộp thư moi@example.com để xác nhận email mới"))
		assert.False(t, r.Tester.Shows(toastInfoSaved))
	})
}

// TestInfoServerErrors runs the scenarios of PATCH /me and the failures of the network:
// the fields the server names go red and the toast says what is wrong, the screen
// stays with what was typed; typing in a red field takes the red back.
func TestInfoServerErrors(t *testing.T) {
	t.Parallel()

	kittest.ForEachPhone(t, func(t *testing.T, ph kittest.Phone) {
		t.Run("validation_failed", func(t *testing.T) {
			r, in := atInfo(t, ph, false)
			require.NoError(t, r.Fake.SetScenario("me.update", "validation_failed"))
			in.First().SetText("Lan")
			press(r, in.Submit())
			assert.True(t, shows(r, toastInfoInvalid))
			assert.True(t, in.First().Error, "first_name is red")
			assert.True(t, in.Email().Error, "email is red")
			assert.False(t, in.Last().Error)
			assert.False(t, in.Phone().Error)
			assert.Equal(t, env.RouteInfo, r.Route())
			assert.Equal(t, "Lan", in.First().Text(), "what was typed stays")
			assert.False(t, in.Busy())

			typeInto(r, in.First(), "x")
			assert.False(t, in.First().Error, "typing takes the red back")
			assert.True(t, in.Email().Error)
		})
		t.Run("offline", func(t *testing.T) {
			r, in := atInfo(t, ph, false)
			r.Fake.SetOnline(false)
			press(r, in.Submit())
			assert.True(t, shows(r, toastOffline))
			assert.Equal(t, env.RouteInfo, r.Route())
			assert.False(t, in.First().Error)
			r.Fake.SetOnline(true)
			press(r, in.Submit())
			assert.Equal(t, env.RouteAccount, r.Route(), "saves when the network is back")
		})
		t.Run("server error", func(t *testing.T) {
			r, in := atInfo(t, ph, false)
			r.Fake.Fail("me.update", 500, "errors/server_error.json")
			press(r, in.Submit())
			assert.True(t, shows(r, toastBusy))
			assert.Equal(t, env.RouteInfo, r.Route())
		})
		t.Run("upgrade required", func(t *testing.T) {
			r, in := atInfo(t, ph, false)
			r.Fake.Fail("me.update", 426, "errors/upgrade_required.json")
			press(r, in.Submit())
			assert.Equal(t, env.RouteUpgrade, r.Route())
		})
	})
}

// TestInfoEnterMovesOn checks the Enter of each field (Q-P6 decision 4): Next to the
// following field, Done on the last one saves.
func TestInfoEnterMovesOn(t *testing.T) {
	t.Parallel()

	kittest.ForEachPhone(t, func(t *testing.T, ph kittest.Phone) {
		r, in := atInfo(t, ph, false)
		r.Tester.View().Focus(in.First().Field())
		for _, next := range []*kit.Input{in.Last(), in.Email(), in.Phone()} {
			require.True(t, r.Tester.Key(keys.KeyEnter, 0))
			assert.Same(t, next.Field(), focused(r), "Enter moves to the next field")
		}
		assert.Zero(t, r.Fake.Account().Count("UpdateMe"))
		require.True(t, r.Tester.Key(keys.KeyEnter, 0))
		r.Settle()
		assert.Equal(t, 1, r.Fake.Account().Count("UpdateMe"), "Done on the last field saves")
		assert.Equal(t, env.RouteAccount, r.Route())
	})
}

// TestInfoPasswordRow checks that the row "Đổi mật khẩu" of the form opens SH.password.
func TestInfoPasswordRow(t *testing.T) {
	t.Parallel()

	kittest.ForEachPhone(t, func(t *testing.T, ph kittest.Phone) {
		r, _ := atInfo(t, ph, false)
		tapKey(t, r, "sheet:password")
		slideIn(r)
		_, ok := r.Shell.Nav().Sheet().Content().(*screens.PasswordSheet)
		assert.True(t, ok)
	})
}

// TestInfoReadsTheUserWhenNoneIsKept checks that a form opened with no kept account
// view (the network was down at launch) is filled from GET /me when it answers, and
// that what the user typed meanwhile is not overwritten.
func TestInfoReadsTheUserWhenNoneIsKept(t *testing.T) {
	t.Parallel()

	kittest.ForEachPhone(t, func(t *testing.T, ph kittest.Phone) {
		t.Run("fills", func(t *testing.T) {
			f, err := shelltest.New(shelltest.MockFS(t))
			require.NoError(t, err)
			require.NoError(t, f.SeedSession())
			r := shelltest.MountRig(t, ph, false, f, shellOptions())
			r.Boot()
			release := f.Gate("me.get")
			t.Cleanup(release)
			r.Shell.Go(nav.Route{Screen: env.RouteInfo}, false)
			r.Tester.Elapse(kittest.Frame)
			in := r.Shell.Nav().Screen().(*screens.Info)
			assert.Empty(t, in.Phone().Text(), "the session has no phone")
			release()
			r.Settle()
			assert.Equal(t, "0905 123 456", in.Phone().Text())
		})
		t.Run("keeps what was typed", func(t *testing.T) {
			f, err := shelltest.New(shelltest.MockFS(t))
			require.NoError(t, err)
			require.NoError(t, f.SeedSession())
			r := shelltest.MountRig(t, ph, false, f, shellOptions())
			r.Boot()
			release := f.Gate("me.get")
			t.Cleanup(release)
			r.Shell.Go(nav.Route{Screen: env.RouteInfo}, false)
			r.Tester.Elapse(kittest.Frame)
			in := r.Shell.Nav().Screen().(*screens.Info)
			typeInto(r, in.Phone(), "0911")
			release()
			r.Settle()
			assert.Equal(t, "0911", in.Phone().Text())
		})
	})
}

// TestInfoKeyboardLiftsTheFields checks, on a short phone, that the field that has the
// focus rises above the keyboard (P6 decision 4): the page's bottom padding follows the
// keyboard and 250 ms after it rose the field is scrolled over it. The fixed button
// stays where it is, under the keyboard.
func TestInfoKeyboardLiftsTheFields(t *testing.T) {
	t.Parallel()

	for _, ph := range kittest.Phones {
		t.Run(ph.Name, func(t *testing.T) {
			t.Parallel()

			r, in := atInfo(t, ph, false)
			const w, h, kb = 360, 600, 280
			r.Tester.SetSize(w, h) // a short phone: the phone field is under the keyboard
			r.Tester.Elapse(kittest.Frame)
			phoneBottom := func() float32 { return in.Phone().Field().WindowFrame().Max().Y }
			require.Greater(t, phoneBottom(), float32(h-kb), "the field starts under where the keyboard will be")
			button := in.Submit().WindowFrame()

			r.Tester.View().Focus(in.Phone().Field())
			r.Tester.SetKeyboard(geom.Insets{Bottom: kb})
			r.Tester.Elapse(kittest.Frame)
			assert.Greater(t, phoneBottom(), float32(h-kb), "not yet: the keyboard is rising")
			r.Tester.Elapse(300 * time.Millisecond)
			assert.LessOrEqual(t, phoneBottom(), float32(h-kb), "the field is above the keyboard")
			assert.Greater(t, in.Phone().Field().WindowFrame().Y, float32(54), "and on the page")
			assert.Equal(t, button, in.Submit().WindowFrame(), "the button stays where it is")

			r.Tester.SetKeyboard(geom.Insets{})
			r.Tester.Elapse(kittest.Frame)
			sc := in.Scroll()
			assert.LessOrEqual(t, sc.Offset().Y, sc.MaxOffset().Y, "the page is within its end again")
		})
	}
}

// TestInfoSteadyFrameDoesNotAllocate checks that a frame of 15B standing still
// allocates nothing, with the keyboard down and up, on both phones.
func TestInfoSteadyFrameDoesNotAllocate(t *testing.T) { //nolint:paralleltest // AllocsPerRun needs the process to itself
	if kittest.Race {
		t.Skip("allocation counts are not stable under the race detector")
	}
	for _, ph := range kittest.Phones {
		r, in := atInfo(t, ph, false)
		r.Tester.View().Focus(in.Email().Field())
		for range 40 { // the caret's blink and the ring settle
			r.Tester.Elapse(kittest.Frame)
		}
		assert.Zero(t, testing.AllocsPerRun(50, r.Tester.Repaint), ph.Name)

		r.Tester.SetKeyboard(geom.Insets{Bottom: 336})
		for range 40 {
			r.Tester.Elapse(kittest.Frame)
		}
		assert.Zero(t, testing.AllocsPerRun(50, r.Tester.Repaint), ph.Name+": keyboard up")
	}
}
